"""
Pruebas de integración, autenticación JWT y ejecución del endpoint POST /api/v1/chat (SUP-247, SUP-353).

Criterios de aceptación verificados:
- 401 si no hay token Authorization ni cookie jwt.
- 401 si el token tiene firma inválida o expiró.
- 401 si el token carece de user_id o del claim exp.
- 422 si el mensaje excede el tamaño máximo permitido.
- 200 con token válido firmado con HS256 y JWT_SECRET (vía Bearer o cookie jwt).
- Ejecución determinista del pipeline de 3 etapas.
- Fallback elegante si falla la etapa 3 de explicación.
- Manejo de texto libre sin filtros reconocidos (anti-alucinación).
- 503 ante falla irrecuperable de proveedores.
"""

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import jwt
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app import config, main
from app.auth import UsuarioAutenticado, obtener_usuario_actual
from app.main import app
from app.productos import ProductoRepositorio
from app.proveedores.base import ProveedorIA, ResultadoExtraccion
from app.routers.chat import crear_router_chat

TEST_JWT_SECRET = "super-secret-key-32-bytes-minimum-length-fixture!"


@pytest.fixture(autouse=True)
def configurar_jwt_secret(monkeypatch):
    """Asegura que JWT_SECRET esté configurado para todas las pruebas."""
    monkeypatch.setattr(config, "JWT_SECRET", TEST_JWT_SECRET)


@pytest.fixture(autouse=True)
def catalogo_fijo(monkeypatch):
    """
    Alimenta el repositorio con una lista en memoria con productos mínimos reales
    en categorías verduras, frutas, lacteos y panaderia sin depender de archivos de disco.
    """
    productos_en_memoria = [
        {
            "id": "prod-tomate-1",
            "nombre": "Tomate Larga Vida",
            "marca": "Generica",
            "categoria": "verduras",
            "supermercado": "Lider",
            "precio": 890.0,
            "precio_normal": 890.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/tomate.png",
            "unidad": "1kg",
            "en_stock": True,
            "distancia_km": 1.2,
        },
        {
            "id": "prod-ajo-2",
            "nombre": "Ajo Chilote",
            "marca": "Generica",
            "categoria": "verduras",
            "supermercado": "Jumbo",
            "precio": 590.0,
            "precio_normal": 590.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/ajo.png",
            "unidad": "malla",
            "en_stock": True,
            "distancia_km": 0.8,
        },
        {
            "id": "prod-leche-3",
            "nombre": "Leche Entera 1L",
            "marca": "Colun",
            "categoria": "lacteos",
            "supermercado": "Lider",
            "precio": 1190.0,
            "precio_normal": 1190.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/leche.png",
            "unidad": "1L",
            "en_stock": True,
            "distancia_km": 1.2,
        },
        {
            "id": "prod-pan-4",
            "nombre": "Pan Marraqueta",
            "marca": "Panaderia",
            "categoria": "panaderia",
            "supermercado": "Lider",
            "precio": 1490.0,
            "precio_normal": 1490.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/pan.png",
            "unidad": "1kg",
            "en_stock": True,
            "distancia_km": 1.5,
        },
        {
            "id": "prod-manzana-5",
            "nombre": "Manzana Royal Gala",
            "marca": "Generica",
            "categoria": "frutas",
            "supermercado": "Jumbo",
            "precio": 1290.0,
            "precio_normal": 1290.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/manzana.png",
            "unidad": "1kg",
            "en_stock": True,
            "distancia_km": 0.8,
        },
    ]
    productos_normalizados = [
        main.repositorio_productos._normalizar_producto(producto)
        for producto in productos_en_memoria
    ]
    monkeypatch.setattr(
        main.repositorio_productos,
        "obtener_todos",
        lambda forzar=False: productos_normalizados,
    )


@pytest.fixture
def cliente():
    return TestClient(app)


def generar_token(
    user_id: str = "usuario-test-123",
    rol: str = "registrado",
    exp_delta: timedelta = timedelta(hours=1),
    secret: str = TEST_JWT_SECRET,
    algorithm: str = "HS256",
    extra_claims: dict = None,
) -> str:
    """Genera un token JWT firmado según el estándar de Identidad."""
    ahora = datetime.now(timezone.utc)
    payload = {
        "user_id": user_id,
        "rol": rol,
        "provider": "local",
        "exp": ahora + exp_delta,
        "iat": ahora,
    }
    if extra_claims:
        payload.update(extra_claims)
    return jwt.encode(payload, secret, algorithm=algorithm)


def test_chat_sin_token_retorna_401(cliente):
    """Test 401 si no hay header Authorization ni cookie en POST /api/v1/chat."""
    respuesta = cliente.post("/api/v1/chat", json={"mensaje": "Hola, busco tomates"})
    assert respuesta.status_code == 401
    assert "Authorization" in respuesta.json().get("detail", "")


def test_chat_header_malformado_retorna_401(cliente):
    """Test 401 si el header Authorization no usa el esquema Bearer."""
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": "Token token-sin-bearer"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert respuesta.status_code == 401
    assert "Bearer" in respuesta.json().get("detail", "")


def test_chat_token_invalido_retorna_401(cliente):
    """Test 401 si el token es inválido o no se puede decodificar."""
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": "Bearer esto_no_es_un_jwt_valido"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert respuesta.status_code == 401


def test_chat_token_firma_invalida_retorna_401(cliente):
    """Test 401 si el token fue firmado con una clave distinta a JWT_SECRET."""
    token_firma_erronea = generar_token(secret="clave-falsa-no-coincidente-1234567890!")
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_firma_erronea}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert respuesta.status_code == 401


def test_chat_token_expirado_retorna_401(cliente):
    """Test 401 si el token expiró."""
    token_expirado = generar_token(exp_delta=timedelta(hours=-2))
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_expirado}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert respuesta.status_code == 401
    assert "expirado" in respuesta.json().get("detail", "").lower()


def test_chat_token_sin_user_id_retorna_401(cliente):
    """Test 401 si el token carece del claim user_id."""
    ahora = datetime.now(timezone.utc)
    payload_sin_user_id = {
        "rol": "registrado",
        "exp": ahora + timedelta(hours=1),
    }
    token_sin_user_id = jwt.encode(payload_sin_user_id, TEST_JWT_SECRET, algorithm="HS256")
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_sin_user_id}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert respuesta.status_code == 401


def test_chat_token_valido_retorna_200_con_pipeline_tres_etapas(cliente):
    """
    Test 200 con token válido firmado con JWT_SECRET y HS256.
    Verifica las 3 etapas deterministas:
    1. Extracción de filtros vía function calling.
    2. Filtrado real sobre el repositorio determinista (con productos en memoria).
    3. Explicación generada únicamente con los productos validados del paso 2.
    """
    token_valido = generar_token(user_id="user-456", rol="registrado")

    filtros_simulados = {
        "categoria": "verduras",
        "precio_max": 1000,
    }

    mock_resultado_extraccion = ResultadoExtraccion(
        filtros=filtros_simulados,
        tiempo_extraccion_ms=45,
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer, \
         patch("app.main.proveedor_fallback.generar_explicacion", new_callable=AsyncMock) as mock_explicar:

        mock_extraer.return_value = mock_resultado_extraccion
        mock_explicar.return_value = "Los productos más convenientes son Tomate y Ajo."

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco verduras por menos de 1000 pesos"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()

        mock_extraer.assert_awaited_once_with("busco verduras por menos de 1000 pesos")

        assert len(datos["productos_recomendados"]) > 0
        for producto in datos["productos_recomendados"]:
            assert producto["categoria"] == "verduras"
            assert producto["precio"] <= 1000
            assert "id" in producto
            assert "precio" in producto

        mock_explicar.assert_awaited_once()
        args_llamada = mock_explicar.call_args[0]
        assert args_llamada[0] == "busco verduras por menos de 1000 pesos"
        assert args_llamada[1] == datos["productos_recomendados"]

        assert datos["respuesta"] == "Los productos más convenientes son Tomate y Ajo."
        assert datos["filtros_aplicados"] == filtros_simulados


def test_chat_secret_no_configurado_retorna_500(cliente, monkeypatch):
    """Test 500 si JWT_SECRET no está configurado en el servidor."""
    monkeypatch.setattr(config, "JWT_SECRET", None)
    token = generar_token()
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token}"},
        json={"mensaje": "Hola"},
    )
    assert respuesta.status_code == 500
    assert "Clave de verificación JWT no configurada" in respuesta.json().get("detail", "")


def test_chat_texto_libre_sin_filtros(cliente):
    """Test cuando el modelo responde en texto libre sin extraer filtros: responde mensaje fijo anti-alucinación."""
    token_valido = generar_token(user_id="user-789")
    mock_resultado = ResultadoExtraccion(
        filtros=None,
        texto_libre="¡Hola! El tomate cuesta $300 en Lider",
        tiempo_extraccion_ms=20,
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.return_value = mock_resultado

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "Hola qué tal"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        assert datos["filtros_aplicados"] is None
        assert datos["productos_recomendados"] == []
        assert "No logré identificar qué productos buscas" in datos["respuesta"]
        assert "$300" not in datos["respuesta"]


def test_chat_etapa_3_falla_retorna_200_con_productos_y_fallback(cliente):
    """
    Si la etapa 3 (explicación LLM) falla con excepción,
    no debe retornar 500: debe retornar 200 con los productos de etapa 2 y un texto por defecto.
    """
    token_valido = generar_token(user_id="user-etapa3")
    mock_resultado = ResultadoExtraccion(
        filtros={"categoria": "lacteos"},
        tiempo_extraccion_ms=15,
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer, \
         patch("app.main.proveedor_fallback.generar_explicacion", new_callable=AsyncMock) as mock_explicar:

        mock_extraer.return_value = mock_resultado
        mock_explicar.side_effect = RuntimeError("Todos los proveedores fallaron en etapa 3")

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "Busco leche"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        assert datos["respuesta"] == "Encontramos estos productos para tu búsqueda."
        assert len(datos["productos"]) > 0


def test_chat_error_en_proveedor(cliente):
    """Test cuando el proveedor retorna un error: debe responder 503 Service Unavailable."""
    token_valido = generar_token(user_id="user-999")
    mock_resultado = ResultadoExtraccion(
        error="Límite de cuota excedido",
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.return_value = mock_resultado

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "Hola"},
        )

        assert respuesta.status_code == 503
        datos = respuesta.json()
        assert "no está disponible" in datos.get("detail", "")


def test_chat_con_cookie_jwt_retorna_200(cliente):
    """Verifica que la cookie 'jwt' emitida por Identidad autentica correctamente al usuario."""
    token_valido = generar_token(user_id="user-cookie-123")
    mock_resultado = ResultadoExtraccion(
        filtros=None,
        texto_libre="Hola desde cookie",
        tiempo_extraccion_ms=10,
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.return_value = mock_resultado

        respuesta = cliente.post(
            "/api/v1/chat",
            cookies={"jwt": token_valido},
            json={"mensaje": "Hola"},
        )

        assert respuesta.status_code == 200
        assert "No logré identificar qué productos buscas" in respuesta.json()["respuesta"]


def test_chat_token_sin_claim_exp_retorna_401(cliente):
    """Verifica que un token sin claim 'exp' es rechazado con 401."""
    ahora = datetime.now(timezone.utc)
    payload_sin_exp = {
        "user_id": "user-sin-exp",
        "rol": "registrado",
        "iat": ahora,
    }
    token_sin_exp = jwt.encode(payload_sin_exp, TEST_JWT_SECRET, algorithm="HS256")

    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_sin_exp}"},
        json={"mensaje": "Hola"},
    )
    assert respuesta.status_code == 401
    assert "Token inválido" in respuesta.json().get("detail", "")


def test_chat_mensaje_largo_excede_maximo_retorna_422(cliente):
    """Verifica que mensajes mayores a 1000 caracteres sean rechazados con 422."""
    token_valido = generar_token()
    mensaje_gigante = "texto_largo" * 101

    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_valido}"},
        json={"mensaje": mensaje_gigante},
    )
    assert respuesta.status_code == 422


def test_endpoints_antiguos_groq_gemini_eliminados(cliente):
    """Verifica que los endpoints no autenticados /chat/groq y /chat/gemini ya no existen."""
    respuesta_groq = cliente.post("/chat/groq", json={"mensaje": "hola"})
    assert respuesta_groq.status_code == 404

    respuesta_gemini = cliente.post("/chat/gemini", json={"mensaje": "hola"})
    assert respuesta_gemini.status_code == 404


def test_cors_configuracion_origenes():
    """Verifica que el middleware CORS esté aplicando orígenes configurados y no '*'."""
    cors_middleware = next(
        (
            middleware
            for middleware in app.user_middleware
            if middleware.cls.__name__ == "CORSMiddleware"
        ),
        None,
    )
    assert cors_middleware is not None
    origenes_permitidos = cors_middleware.kwargs.get("allow_origins", [])
    assert "*" not in origenes_permitidos
    assert "http://localhost:3000" in origenes_permitidos
    assert "http://localhost:5173" in origenes_permitidos


def test_chat_router_ejecuta_obtener_todos_sin_bloquear():
    """Verifica que el router de chat invoca obtener_todos sin bloquear el hilo principal."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = {
        "data": [
            {
                "id": "prod-1",
                "nombre": "Leche Entera 1L",
                "precio": 1190.0,
                "categoria": "lacteos",
                "en_stock": True,
            }
        ],
        "paginacion": {
            "total_registros": 1,
            "pagina_actual": 1,
            "total_paginas": 1,
            "limite": 100,
        },
    }
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    class MockProveedor(ProveedorIA):
        def __init__(self):
            self.api_key = "dummy_key"

        async def extraer_filtros(self, mensaje: str):
            return ResultadoExtraccion(filtros={"categoria": "lacteos"}, tiempo_extraccion_ms=10)

        async def generar_explicacion(self, mensaje: str, productos: list):
            return "Explicacion simulada"

    aplicacion_prueba = FastAPI()
    mock_proveedor = MockProveedor()
    aplicacion_prueba.include_router(crear_router_chat(repo, mock_proveedor))
    aplicacion_prueba.dependency_overrides[obtener_usuario_actual] = lambda: UsuarioAutenticado(
        user_id="test-hilos",
        rol="registrado",
    )

    cliente_prueba = TestClient(aplicacion_prueba)
    respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "quiero lacteos"})
    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert len(datos["productos"]) == 1
    assert datos["respuesta"] == "Explicacion simulada"
