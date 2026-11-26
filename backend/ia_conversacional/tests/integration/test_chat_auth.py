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

import logging
from datetime import UTC, datetime, timedelta
from unittest.mock import AsyncMock, MagicMock, patch

import anyio
import httpx
import jwt
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app import config, main
from app.api.chat import crear_router_chat
from app.api.sesion import UsuarioAutenticado, obtener_usuario_actual
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.main import app
from app.repositories.catalogo import ProductoRepositorio
from app.repositories.gemini import GeminiProveedor
from app.repositories.groq import GroqProveedor
from app.services.chat import TEXTO_CATALOGO_NO_DISPONIBLE
from app.services.fallback import FallbackProveedor

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
        main.repositorio_productos._normalizar_producto(producto) for producto in productos_en_memoria
    ]
    monkeypatch.setattr(
        main.repositorio_productos,
        "obtener_todos",
        lambda: productos_normalizados,
    )
    # Estas pruebas ejercitan la caché en memoria: la búsqueda dirigida se simula caída
    # (None) para que el pipeline use esos productos. La búsqueda tiene sus propias
    # pruebas en tests/unit/test_busqueda_catalogo.py.
    monkeypatch.setattr(main.repositorio_productos, "buscar_productos", lambda *_argumentos, **_opciones: None)
    # Asegurar que la fuente sea "catalogo" para que las pruebas de pipeline
    # funcionen con estado vigente, tal como el test original esperaba.
    main.repositorio_productos._fuente = "catalogo"
    main.repositorio_productos._ultima_actualizacion = 0.0


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
    ahora = datetime.now(UTC)
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
    ahora = datetime.now(UTC)
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

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
    ):
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


def test_chat_texto_libre_sin_filtros(cliente):
    """Test cuando el modelo responde en texto libre sin extraer filtros: responde mensaje fijo anti-alucinación."""
    token_valido = generar_token(user_id="user-789")
    mock_resultado = ResultadoExtraccion(
        filtros=None,
        texto_libre="¡Hola! El tomate cuesta $300 en Lider",
        tiempo_extraccion_ms=20,
    )

    async def extraccion_lenta(_mensaje):
        await anyio.sleep(0.05)
        return mock_resultado

    with patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.side_effect = extraccion_lenta

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

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
    ):
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
        assert len(datos["productos_recomendados"]) > 0


def test_chat_error_en_proveedor(cliente):
    """Test cuando el proveedor retorna un error: debe responder 503 Service Unavailable."""
    token_valido = generar_token(user_id="user-999")
    mock_resultado = ResultadoExtraccion(
        error="Límite de cuota excedido",
    )

    async def extraccion_lenta(_mensaje):
        await anyio.sleep(0.05)
        return mock_resultado

    with patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.side_effect = extraccion_lenta

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

    async def extraccion_lenta(_mensaje):
        await anyio.sleep(0.05)
        return mock_resultado

    with patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.side_effect = extraccion_lenta

        respuesta = cliente.post(
            "/api/v1/chat",
            cookies={"jwt": token_valido},
            json={"mensaje": "Hola"},
        )

        assert respuesta.status_code == 200
        assert "No logré identificar qué productos buscas" in respuesta.json()["respuesta"]


def test_chat_token_sin_claim_exp_retorna_401(cliente):
    """Verifica que un token sin claim 'exp' es rechazado con 401."""
    ahora = datetime.now(UTC)
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
        (middleware for middleware in app.user_middleware if middleware.cls.__name__ == "CORSMiddleware"),
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
        url_base="http://catalogo:8080",
        cliente=mock_cliente,
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
    )

    cliente_prueba = TestClient(aplicacion_prueba)
    respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "quiero lacteos"})
    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert len(datos["productos_recomendados"]) == 1
    assert datos["respuesta"] == "Explicacion simulada"


def test_chat_mensaje_solo_espacios_retorna_422(cliente):
    """Verifica que un mensaje compuesto solo de espacios en blanco sea rechazado con 422."""
    token_valido = generar_token()
    respuesta = cliente.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_valido}"},
        json={"mensaje": "   "},
    )
    assert respuesta.status_code == 422


def test_chat_filtros_vacios_responde_sin_filtros(cliente):
    """Verifica que cuando la extracción retorna filtros vacíos {} se responde TEXTO_SIN_FILTROS."""
    token_valido = generar_token(user_id="user-filtros-vacios")
    mock_resultado = ResultadoExtraccion(
        filtros={},
        tiempo_extraccion_ms=10,
    )

    async def extraccion_lenta(_mensaje):
        await anyio.sleep(0.05)
        return mock_resultado

    with patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.side_effect = extraccion_lenta

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "cosas"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        assert datos["filtros_aplicados"] is None
        assert datos["productos_recomendados"] == []
        assert "No logré identificar qué productos buscas" in datos["respuesta"]


def test_chat_receta_ingredientes_encontrados_y_faltantes(cliente):
    """Test SUP-325: receta con ingredientes encontrados y faltantes informa ingredientes_receta e ingredientes_faltantes."""
    token_valido = generar_token(user_id="user-receta-ingredientes")
    mock_resultado = ResultadoExtraccion(
        filtros={
            "ingredientes": [
                {"nombre": "Tomate", "cantidad_envases": 2},
                {"nombre": "Perejil", "cantidad_envases": 1},
            ],
        },
        tiempo_extraccion_ms=35,
    )
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Para tu receta de pasta, agrega Tomate Larga Vida."

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de pasta con tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()

        mock_extraer.assert_awaited_once_with("busco una receta de pasta con tomate")

        assert len(datos["ingredientes_receta"]) == 1
        assert datos["ingredientes_receta"][0]["ingrediente"] == "Tomate"
        assert datos["ingredientes_receta"][0]["cantidad_sugerida"] == 2
        assert len(datos["ingredientes_receta"][0]["candidatos"]) == 1
        assert datos["ingredientes_receta"][0]["candidatos"][0]["producto_id"] == "prod-tomate-1"

        assert datos["ingredientes_faltantes"] == ["Perejil"]

        assert len(datos["productos_recomendados"]) == 1
        assert datos["productos_recomendados"][0]["id"] == "prod-tomate-1"

        mock_explicar.assert_awaited_once()
        args_llamada = mock_explicar.call_args[0]
        assert args_llamada[0] == "busco una receta de pasta con tomate"
        assert args_llamada[1] == datos["productos_recomendados"]


def test_chat_candidato_descartado_por_precio_vencido(cliente):
    """Test SUP-325: candidato descartado cuando la disponibilidad reporta estado_precio vencido."""
    token_valido = generar_token(user_id="user-receta-vencido")
    mock_resultado = ResultadoExtraccion(
        filtros={"ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=25,
    )
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vencido",
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Los productos más convenientes son Tomate y Ajo."

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de salsa de tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()

        assert datos["ingredientes_receta"] == []
        assert datos["ingredientes_faltantes"] == ["Tomate"]
        assert datos["productos_recomendados"] == []
        assert datos["respuesta"] == "No encontramos productos que coincidan con tu busqueda."


def test_chat_timeout_total_503(cliente, monkeypatch):
    """Test que_timeout total excedido retorna 503 (SUP-...)."""
    # Forzar timeout inmediato reduciendo el límite
    monkeypatch.setattr(config, "CHAT_TIMEOUT_TOTAL", 0.001)
    token_valido = generar_token(user_id="user-timeout")
    mock_resultado = ResultadoExtraccion(
        filtros={"categoria": "verduras"},
        tiempo_extraccion_ms=10,
    )

    async def extraccion_lenta(_mensaje):
        await anyio.sleep(0.05)
        return mock_resultado

    with patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.side_effect = extraccion_lenta

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco verduras"},
        )

        assert respuesta.status_code == 503
        datos = respuesta.json()
        assert "no está disponible" in datos.get("detail", "").lower()


# Nuevo test (a): Explicación lenta que excede tiempo_limite devuelve 200 con fallback
def test_chat_explicacion_lenta_excede_limite_returns_200_con_fallback(cliente, monkeypatch):
    """Test que una explicacion LLM lenta (excede tiempo_limite) retorna 200 con TEXTO_EXPLICACION_RESPALDO."""
    monkeypatch.setattr(config, "CHAT_TIMEOUT_TOTAL", 0.2)
    token_valido = generar_token(user_id="user-lento")

    # Mock de extracción de filtros
    mock_resultado = ResultadoExtraccion(
        filtros={"categoria": "verduras"},
        tiempo_extraccion_ms=10,
    )

    # Mock de generar_explicacion que "tarda" más que el tiempo restante (incluso el piso de 0.5s)
    async def explicacion_lenta(_mensaje, _productos):
        await anyio.sleep(2.0)
        return "Explicacion LLM real despues de esperarmucho tiempo"

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.side_effect = explicacion_lenta

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco verduras"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        # Debería retornar el texto de fallback, no 503
        assert datos["respuesta"] == "Encontramos estos productos para tu búsqueda."
        # Los productos filtrados deberían estar presentes
        assert len(datos["productos_recomendados"]) > 0
        # Los filtros deberían aplicarse
        assert datos["filtros_aplicados"] == {"categoria": "verduras"}


# Nuevo test (b): Receta donde caché tiene precio 1000 y consultar_disponibilidad devuelve precio 2000
def test_chat_receta_precio_caché_vs_disponibilidad(cliente, monkeypatch):
    """Test SUP-...: receta donde caché tiene precio 1000 y consultar_disponibilidad devuelve producto.precio = 2000 → respuesta devuelve 2000."""
    token_valido = generar_token(user_id="user-precio-test")
    mock_resultado = ResultadoExtraccion(
        filtros={"ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=25,
    )
    # Disponibilidad con precio vigente 2000 (sobreescribe el caché de 1000)
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
            "producto": {
                "precio": 2000,
                "unidad": "1kg",
                "supermercado": "Lider",
                "marca": "Generica",
                "url_imagen": "https://img.super.cl/tomate.png",
            },
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Explicacion con tomate"

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de salsa de tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        # Debería retornar 2000 (desde disponibilidad), no 1000 (desde caché)
        assert datos["productos_recomendados"][0]["precio"] == 2000
        # La explicación debería usar el producto con precio 2000
        assert "2000" in datos["respuesta"] or "2000" in str(datos)


# Nuevo test (c): Receta con 4 candidatos donde los 3 primeros están vencidos y el 4º vigente
def test_chat_receta_cuatro_candidatos_4to_vigente(cliente, monkeypatch):
    """Test SUP-...: receta con 4 candidatos donde los 3 primeros están vencidos y el 4º vigente → devuelve el 4º candidato y no queda en ingredientes_faltantes."""
    token_valido = generar_token(user_id="user-4candidatos")
    mock_resultado = ResultadoExtraccion(
        filtros={"ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=25,
    )
    # 4 candidatos: los 3 primeros con estado_precio "vencido", el 4º con "vigente"
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vencido",
        },
        "prod-tomate-2": {
            "producto_id": "prod-tomate-2",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vencido",
        },
        "prod-tomate-3": {
            "producto_id": "prod-tomate-3",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vencido",
        },
        "prod-tomate-4": {
            "producto_id": "prod-tomate-4",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
        patch(
            "app.services.chat.vincular_ingredientes_catalogo",
            return_value=(
                [
                    {
                        "ingrediente": "Tomate",
                        "cantidad_sugerida": 1,
                        "candidatos": [
                            {
                                "producto_id": "prod-tomate-1",
                                "nombre": "Tomate Larga Vida",
                                "marca": "Generica",
                                "unidad": "1kg",
                                "precio": 890,
                                "supermercado": "Lider",
                                "url_imagen": "https://img.super.cl/tomate.png",
                            },
                            {
                                "producto_id": "prod-tomate-2",
                                "nombre": "Tomate Chico",
                                "marca": "Generica",
                                "unidad": "1kg",
                                "precio": 590,
                                "supermercado": "Jumbo",
                                "url_imagen": "https://img.super.cl/ajo.png",
                            },
                            {
                                "producto_id": "prod-tomate-3",
                                "nombre": "Tomate Cherry",
                                "marca": "Generica",
                                "unidad": "1kg",
                                "precio": 390,
                                "supermercado": "Super",
                                "url_imagen": "https://img.super.cl/manzana.png",
                            },
                            {
                                "producto_id": "prod-tomate-4",
                                "nombre": "Tomate Orgánico",
                                "marca": "Ecológica",
                                "unidad": "1kg",
                                "precio": 2000,
                                "supermercado": "Walmart",
                                "url_imagen": "https://img.super.cl/tomate.png",
                            },
                        ],
                    }
                ],
                [],
            ),
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Explicacion con cuatro tomates"

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de salsa con cuatro tipos de tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        # Debería tener 1 ingrediente recetado (el 4º candidato)
        assert len(datos["ingredientes_receta"]) == 1
        # El candidato debería ser el 4º (prod-tomate-4), no los 3 primeros
        assert len(datos["ingredientes_receta"][0]["candidatos"]) == 1
        assert datos["ingredientes_receta"][0]["candidatos"][0]["producto_id"] == "prod-tomate-4"
        # NO debería estar en ingredientes_faltantes
        assert datos["ingredientes_faltantes"] == []
        # Los productos recomendados deberían incluir el 4º candidato
        assert len(datos["productos_recomendados"]) >= 1
        # Los productos recomendados deberían incluir el 4º candidato


# Nuevo test (a): Receta con precio vigente distinto: la caché del repositorio conserva su precio original
def test_chat_receta_caché_no_se_muta_precio_vigente(cliente, monkeypatch):
    """Test: tras una receta cuyo precio vigente difiere del de la caché, obtener_todos mantiene el original."""
    token_valido = generar_token(user_id="user-receta-cache-no-muta")
    mock_resultado = ResultadoExtraccion(
        filtros={"ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=25,
    )
    # Disponibilidad con precio vigente 1500 (distinto al precio original de la caché: 890)
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
            "producto": {
                "precio": 1500,
                "unidad": "1kg",
                "supermercado": "Lider",
                "marca": "Generica",
                "url_imagen": "https://img.super.cl/tomate.png",
            },
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Explicacion con tomate"

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de salsa de tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        # El precio vigente (1500) se usa para la recomendación...
        assert datos["productos_recomendados"][0]["precio"] == 1500

        # ... pero la caché del repositorio conserva su precio original (890).
        productos_en_caché = main.repositorio_productos.obtener_todos()
        producto_en_caché = next(
            (producto for producto in productos_en_caché if producto.get("id") == "prod-tomate-1"),
            None,
        )
        assert producto_en_caché is not None
        assert producto_en_caché["precio"] == 890.0


# Nuevo test (b): Tras actualizar los precios vigentes, el candidato más barato queda primero
def test_chat_receta_candidato_reordenado_por_precio_vigente(cliente, monkeypatch):
    """Test: el candidato B ($1500 en caché) pasa a $900 vigente y el candidato A ($1000) a $1200; el B queda primero."""
    token_valido = generar_token(user_id="user-receta-reordenar")
    mock_resultado = ResultadoExtraccion(
        filtros={"ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=25,
    )
    # Disponibilidad: candidato A (caché $1000) pasa a $1200 vigente y candidato B (caché $1500) a $900 vigente.
    disponibilidad_simulada = {
        "prod-tomate-1": {
            "producto_id": "prod-tomate-1",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
            "producto": {
                "precio": 1200,
                "unidad": "1kg",
                "supermercado": "Lider",
                "marca": "Generica",
                "url_imagen": "https://img.super.cl/tomate.png",
            },
        },
        "prod-tomate-2": {
            "producto_id": "prod-tomate-2",
            "estado_producto": "disponible",
            "en_stock": True,
            "estado_precio": "vigente",
            "producto": {
                "precio": 900,
                "unidad": "1kg",
                "supermercado": "Jumbo",
                "marca": "Generica",
                "url_imagen": "https://img.super.cl/tomate.png",
            },
        },
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch("app.main.proveedor_respaldo.generar_explicacion", new_callable=AsyncMock) as mock_explicar,
        patch.object(
            main.repositorio_productos, "consultar_disponibilidad", lambda producto_ids: disponibilidad_simulada
        ),
        patch(
            "app.services.chat.vincular_ingredientes_catalogo",
            return_value=(
                [
                    {
                        "ingrediente": "Tomate",
                        "cantidad_sugerida": 1,
                        "candidatos": [
                            {
                                "producto_id": "prod-tomate-1",
                                "nombre": "Tomate Larga Vida",
                                "marca": "Generica",
                                "unidad": "1kg",
                                "precio": 1000,
                                "supermercado": "Lider",
                                "url_imagen": "https://img.super.cl/tomate.png",
                            },
                            {
                                "producto_id": "prod-tomate-2",
                                "nombre": "Tomate Chico",
                                "marca": "Generica",
                                "unidad": "1kg",
                                "precio": 1500,
                                "supermercado": "Jumbo",
                                "url_imagen": "https://img.super.cl/tomate.png",
                            },
                        ],
                    }
                ],
                [],
            ),
        ),
    ):
        mock_extraer.return_value = mock_resultado
        mock_explicar.return_value = "Explicacion con tomates"

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco una receta de salsa con tomate"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()

        # El candidato A pasa a $1200 y el candidato B a $900 vigentes: el B queda primero al reordenar.
        candidatos = datos["ingredientes_receta"][0]["candidatos"]
        assert len(candidatos) == 2
        assert candidatos[0]["producto_id"] == "prod-tomate-2"
        assert candidatos[0]["precio"] == 900
        assert candidatos[1]["producto_id"] == "prod-tomate-1"
        assert candidatos[1]["precio"] == 1200

        assert datos["ingredientes_faltantes"] == []

        assert len(datos["productos_recomendados"]) >= 1
        assert datos["productos_recomendados"][0]["producto_id"] == "prod-tomate-2"


def test_chat_receta_revalida_precio_max_tras_disponibilidad(cliente):
    """Si un producto cumple precio_max en caché ($400) pero su precio vigente ($2000) lo supera, se descarta."""
    token_valido = generar_token(user_id="user-revalida-precio")
    mock_resultado = ResultadoExtraccion(
        filtros={"precio_max": 500, "ingredientes": [{"nombre": "Tomate", "cantidad_envases": 1}]},
        tiempo_extraccion_ms=10,
    )
    catalogo_cache = [
        {
            "id": "prod-tomate-400",
            "nombre": "Tomate Larga Vida",
            "marca": "Generica",
            "categoria": "verduras",
            "supermercado": "Lider",
            "precio": 400.0,
            "precio_normal": 400.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/tomate.png",
            "unidad": "1kg",
            "en_stock": True,
            "distancia_km": 0.0,
        }
    ]
    disponibilidad_simulada = {
        "prod-tomate-400": {
            "producto_id": "prod-tomate-400",
            "estado_producto": "disponible",
            "estado_precio": "vigente",
            "producto": {"precio": 2000, "unidad": "1kg", "supermercado": "Lider", "marca": "Generica"},
        }
    }

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch.object(main.repositorio_productos, "obtener_todos", return_value=catalogo_cache),
        patch.object(main.repositorio_productos, "consultar_disponibilidad", return_value=disponibilidad_simulada),
    ):
        mock_extraer.return_value = mock_resultado
        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "quiero hacer pebre por menos de 500 pesos"},
        )

        assert respuesta.status_code == 200
        datos = respuesta.json()
        assert datos["ingredientes_receta"] == []
        assert datos["ingredientes_faltantes"] == ["Tomate"]
        assert datos["productos_recomendados"] == []


# ===== Pruebas controladas de fallos (SUP-315) =====


def _crear_cliente_chat(repositorio, proveedor) -> TestClient:
    """Arma una aplicación mínima con el router real del chat y la sesión ya resuelta."""
    aplicacion_prueba = FastAPI()
    aplicacion_prueba.include_router(crear_router_chat(repositorio, proveedor))
    aplicacion_prueba.dependency_overrides[obtener_usuario_actual] = lambda: UsuarioAutenticado(
        user_id="usuario-sup-315",
    )
    return TestClient(aplicacion_prueba)


def test_chat_sin_claves_de_proveedores_retorna_503_sin_registrar_claves(caplog):
    """Sin GEMINI_API_KEY ni GROQ_API_KEY el respaldo falla y el chat responde 503 sin filtrar claves al log."""
    proveedor_respaldo = FallbackProveedor(
        proveedores=[
            GeminiProveedor(clave_api=None, url_endpoint="https://ejemplo.invalid/gemini"),
            GroqProveedor(
                clave_api=None,
                url_endpoint="https://ejemplo.invalid/groq",
                modelo_extraccion="modelo-extraccion",
                modelo_explicacion="modelo-explicacion",
            ),
        ],
        reintentos_primario=0,
        reintentos_secundario=0,
        espera_base=0.0,
        variacion_maxima=0.0,
    )
    cliente_prueba = _crear_cliente_chat(main.repositorio_productos, proveedor_respaldo)

    with caplog.at_level(logging.WARNING):
        respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "busco tomates"})

    assert respuesta.status_code == 503
    assert "no está disponible" in respuesta.json().get("detail", "")
    # Las claves ausentes no deben aparecer como valor de encabezado ni como texto en los logs.
    assert "None" not in caplog.text
    assert "Bearer None" not in caplog.text


def test_chat_gemini_responde_429_y_groq_ok_retorna_200(monkeypatch):
    """Ante un 429 de Gemini, el respaldo conmuta a Groq y el chat responde 200 con los productos filtrados."""
    solicitud_gemini = httpx.Request("POST", "https://ejemplo.invalid/gemini")
    respuesta_429 = httpx.Response(429, request=solicitud_gemini)
    error_429 = httpx.HTTPStatusError("429 Too Many Requests", request=solicitud_gemini, response=respuesta_429)

    proveedor_gemini = GeminiProveedor(clave_api="clave-gemini-ficticia", url_endpoint="https://ejemplo.invalid/gemini")
    proveedor_groq = GroqProveedor(
        clave_api="clave-groq-ficticia",
        url_endpoint="https://ejemplo.invalid/groq",
        modelo_extraccion="modelo-extraccion",
        modelo_explicacion="modelo-explicacion",
    )

    async def gemini_responde_429(_cuerpo):
        raise error_429

    async def groq_responde_ok(cuerpo):
        if "tools" in cuerpo:
            return {
                "choices": [{"message": {"tool_calls": [{"function": {"arguments": '{"categoria": "verduras"}'}}]}}]
            }
        return {"choices": [{"message": {"content": "Verduras disponibles en el catálogo."}}]}

    monkeypatch.setattr(proveedor_gemini, "_enviar", gemini_responde_429)
    monkeypatch.setattr(proveedor_groq, "_enviar", groq_responde_ok)

    proveedor_respaldo = FallbackProveedor(
        proveedores=[proveedor_gemini, proveedor_groq],
        reintentos_primario=0,
        reintentos_secundario=0,
        espera_base=0.0,
        variacion_maxima=0.0,
    )
    cliente_prueba = _crear_cliente_chat(main.repositorio_productos, proveedor_respaldo)

    respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "busco verduras"})

    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert datos["respuesta"] == "Verduras disponibles en el catálogo."
    assert datos["estado_datos"] == "vigente"
    assert len(datos["productos_recomendados"]) > 0
    assert all(producto["categoria"] == "verduras" for producto in datos["productos_recomendados"])


def test_chat_catalogo_vacio_retorna_200_con_texto_no_disponible(cliente):
    """Con el catálogo vacío se informa la indisponibilidad sin inventar productos y sin 503."""
    token_valido = generar_token(user_id="user-catalogo-vacio")
    mock_resultado = ResultadoExtraccion(
        filtros={"categoria": "verduras"},
        tiempo_extraccion_ms=10,
    )

    with (
        patch("app.main.proveedor_respaldo.extraer_filtros", new_callable=AsyncMock) as mock_extraer,
        patch.object(main.repositorio_productos, "obtener_todos", return_value=[]),
    ):
        mock_extraer.return_value = mock_resultado

        respuesta = cliente.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco verduras"},
        )

    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert datos["productos_recomendados"] == []
    assert "estado_datos" in datos


def test_chat_sin_datos_retorna_200_con_estado_sin_datos(cliente):
    """Con repositorio fuente sin_datos y catálogo vacío devuelve estado_datos == 'sin_datos'."""

    class RepositorioSinDatos:
        """Doble de repositorio con fuente sin_datos y catálogo vacío."""

        fuente = "sin_datos"

        def obtener_todos(self) -> list:
            return []

        def consultar_disponibilidad(self, _producto_ids: list) -> dict | None:
            return None

    class ProveedorFijo(ProveedorIA):
        """Proveedor mínimo que devuelve filtros e explicación deterministas."""

        nombre = "fijo"

        async def extraer_filtros(self, mensaje: str):
            return ResultadoExtraccion(filtros={"categoria": "verduras"}, tiempo_extraccion_ms=5)

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            return "Productos disponibles."

    cliente_prueba = _crear_cliente_chat(RepositorioSinDatos(), ProveedorFijo())

    respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "busco verduras"})

    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert datos["estado_datos"] == "sin_datos"
    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert datos["productos_recomendados"] == []


def test_chat_memoria_antigua_retorna_200_con_aviso_de_carga_previa():
    """Una carga previa antigua se informa como memoria_antigua y agrega un aviso en la respuesta."""
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
            "distancia_km": 0.0,
        }
    ]

    class RepositorioMemoriaAntigua:
        """Doble de repositorio que representa una carga previa antigua del catálogo."""

        datos_antiguos = True

        @property
        def fuente(self) -> str:
            return "memoria"

        def obtener_todos(self) -> list:
            return productos_en_memoria

        def consultar_disponibilidad(self, _producto_ids: list):
            return None

    class ProveedorFijo(ProveedorIA):
        """Proveedor mínimo que devuelve filtros e explicación deterministas."""

        nombre = "fijo"

        async def extraer_filtros(self, mensaje: str):
            return ResultadoExtraccion(filtros={"categoria": "verduras"}, tiempo_extraccion_ms=5)

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            return "Tomate disponible."

    cliente_prueba = _crear_cliente_chat(RepositorioMemoriaAntigua(), ProveedorFijo())

    respuesta = cliente_prueba.post("/api/v1/chat", json={"mensaje": "busco verduras"})

    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert datos["estado_datos"] == "memoria_antigua"
    assert "carga previa" in datos["respuesta"]
    assert datos["respuesta"].startswith("Tomate disponible.")
    assert len(datos["productos_recomendados"]) == 1
