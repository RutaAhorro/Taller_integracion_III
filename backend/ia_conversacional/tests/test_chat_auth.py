"""
Pruebas de autenticación JWT y ejecución del endpoint POST /api/v1/chat (SUP-247).
Criterios de aceptación verificados:
- 401 si no hay token Authorization.
- 401 si el token tiene firma inválida o expiró.
- 200 con token válido firmado con HS256 y JWT_SECRET.
- Ejecución determinista del pipeline de 3 etapas.
"""

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, patch
import jwt
import pytest
from fastapi.testclient import TestClient

from app import config
from app.main import app
from app.proveedores.base import ResultadoExtraccion

TEST_JWT_SECRET = "super-secret-key-32-bytes-minimum-length-fixture!"


@pytest.fixture(autouse=True)
def configurar_jwt_secret(monkeypatch):
    """Asegura que JWT_SECRET esté configurado para todas las pruebas."""
    monkeypatch.setattr(config, "JWT_SECRET", TEST_JWT_SECRET)


@pytest.fixture
def client():
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


def test_chat_sin_token_retorna_401(client):
    """Test 401 si no hay header Authorization en POST /api/v1/chat."""
    response = client.post("/api/v1/chat", json={"mensaje": "Hola, busco tomates"})
    assert response.status_code == 401
    assert "Authorization" in response.json().get("detail", "")


def test_chat_header_malformado_retorna_401(client):
    """Test 401 si el header Authorization no usa el esquema Bearer."""
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": "Token token-sin-bearer"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert response.status_code == 401
    assert "Bearer" in response.json().get("detail", "")


def test_chat_token_invalido_retorna_401(client):
    """Test 401 si el token es inválido o no se puede decodificar."""
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": "Bearer esto_no_es_un_jwt_valido"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert response.status_code == 401


def test_chat_token_firma_invalida_retorna_401(client):
    """Test 401 si el token fue firmado con una clave distinta a JWT_SECRET."""
    token_firma_erronea = generar_token(secret="clave-falsa-no-coincidente-1234567890!")
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_firma_erronea}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert response.status_code == 401


def test_chat_token_expirado_retorna_401(client):
    """Test 401 si el token expiró."""
    token_expirado = generar_token(exp_delta=timedelta(hours=-2))
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_expirado}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert response.status_code == 401
    assert "expirado" in response.json().get("detail", "").lower()


def test_chat_token_sin_user_id_retorna_401(client):
    """Test 401 si el token carece del claim user_id."""
    ahora = datetime.now(timezone.utc)
    payload = {
        "rol": "registrado",
        "exp": ahora + timedelta(hours=1),
    }
    token_sin_user_id = jwt.encode(payload, TEST_JWT_SECRET, algorithm="HS256")
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token_sin_user_id}"},
        json={"mensaje": "Hola, busco tomates"},
    )
    assert response.status_code == 401


def test_chat_token_valido_retorna_200_con_pipeline_tres_etapas(client):
    """
    Test 200 con token válido firmado con JWT_SECRET y HS256.
    Verifica las 3 etapas deterministas:
    1. Extracción de filtros vía function calling.
    2. Filtrado real sobre el repositorio determinista (con productos reales de productos.json).
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
        mock_explicar.return_value = "Los productos más convenientes son Tomate a $990 y Cebolla a $690."

        response = client.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "busco verduras por menos de 1000 pesos"},
        )

        assert response.status_code == 200
        data = response.json()

        # Etapa 1 llamada
        mock_extraer.assert_awaited_once_with("busco verduras por menos de 1000 pesos")

        # Etapa 2 verificada: productos filtrados reales de productos.json
        # En productos.json:
        # Tomate (990, verduras) <= 1000
        # Cebolla (690, verduras) <= 1000
        # Ajo (590, verduras) <= 1000
        # Zapallo italiano (890, verduras) <= 1000
        assert len(data["productos_recomendados"]) > 0
        for p in data["productos_recomendados"]:
            assert p["categoria"] == "verduras"
            assert p["precio"] <= 1000
            assert "id" in p
            assert "precio" in p

        # Etapa 3 llamada con los productos filtrados de la etapa 2
        mock_explicar.assert_awaited_once()
        args_llamada = mock_explicar.call_args[0]
        # args_llamada[0] es el mensaje, args_llamada[1] son los productos filtrados
        assert args_llamada[0] == "busco verduras por menos de 1000 pesos"
        assert args_llamada[1] == data["productos_recomendados"]

        # Respuesta estructurada correcta
        assert data["respuesta"] == "Los productos más convenientes son Tomate a $990 y Cebolla a $690."
        assert data["filtros_aplicados"] == filtros_simulados
        assert data["user_id"] == "user-456"


def test_chat_secret_no_configurado_retorna_401(client, monkeypatch):
    """Test 401 si JWT_SECRET no está configurado en el servidor."""
    monkeypatch.setattr(config, "JWT_SECRET", None)
    token = generar_token()
    response = client.post(
        "/api/v1/chat",
        headers={"Authorization": f"Bearer {token}"},
        json={"mensaje": "Hola"},
    )
    assert response.status_code == 401


def test_chat_texto_libre_sin_filtros(client):
    """Test cuando el modelo responde en texto libre sin extraer filtros."""
    token_valido = generar_token(user_id="user-789")
    mock_resultado = ResultadoExtraccion(
        filtros=None,
        texto_libre="¡Hola! ¿En qué puedo ayudarte hoy?",
        tiempo_extraccion_ms=20,
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.return_value = mock_resultado

        response = client.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "Hola qué tal"},
        )

        assert response.status_code == 200
        data = response.json()
        assert data["filtros_aplicados"] is None
        assert data["productos_recomendados"] == []
        assert data["respuesta"] == "¡Hola! ¿En qué puedo ayudarte hoy?"
        assert data["user_id"] == "user-789"


def test_chat_error_en_proveedor(client):
    """Test cuando el proveedor retorna un error."""
    token_valido = generar_token(user_id="user-999")
    mock_resultado = ResultadoExtraccion(
        error="Límite de cuota excedido",
    )

    with patch("app.main.proveedor_fallback.extraer_filtros", new_callable=AsyncMock) as mock_extraer:
        mock_extraer.return_value = mock_resultado

        response = client.post(
            "/api/v1/chat",
            headers={"Authorization": f"Bearer {token_valido}"},
            json={"mensaje": "Hola"},
        )

        assert response.status_code == 200
        data = response.json()
        assert "error" in data
        assert data["user_id"] == "user-999"


def test_cors_configuracion_origenes(client):
    """Verifica que el middleware CORS esté aplicando orígenes configurados y no '*'."""
    from app.main import app

    # Obtener los orígenes configurados en el middleware CORS de la app
    cors_middleware = next(
        (m for m in app.user_middleware if m.cls.__name__ == "CORSMiddleware"),
        None
    )
    assert cors_middleware is not None
    allowed_origins = cors_middleware.kwargs.get("allow_origins", [])
    assert "*" not in allowed_origins
    assert "http://localhost:3000" in allowed_origins
    assert "http://localhost:5173" in allowed_origins

