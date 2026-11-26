"""
Tests unitarios para el proveedor de fallback y resiliencia (FallbackProveedor).

Verifica:
1. Clasificación estricta de _es_error_recuperable (transitorios 429, 500, 502, 503, 504,
   TimeoutException y RequestError retornan True; errores cliente 400 y 401 retornan False).
2. FallbackProveedor.generar_explicacion con proveedores simulados en memoria:
   - Reintenta proveedor primario ante error recuperable con backoff y conmuta a secundario.
   - Conmuta directamente sin reintentar ante error 400 no recuperable.
"""

from unittest.mock import AsyncMock, MagicMock

import httpx
import pytest

from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.repositories.gemini import GeminiProveedor
from app.services.fallback import FallbackProveedor


class ProveedorFalso(ProveedorIA):
    """Proveedor auxiliar en memoria para pruebas deterministas."""

    def __init__(self, nombre_proveedor: str):
        self.nombre = nombre_proveedor
        self.llamadas_explicacion = 0
        self.llamadas_extraccion = 0

    async def extraer_filtros(self, mensaje_usuario: str):
        self.llamadas_extraccion += 1
        return ResultadoExtraccion(filtros={}, tiempo_extraccion_ms=1)

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        self.llamadas_explicacion += 1
        return f"Explicacion generada por {self.nombre}"


def test_es_error_recuperable_clasificacion():
    """Verifica que solo errores de red y estados transitorios sean marcados como recuperables."""
    proveedor_fallback = FallbackProveedor(proveedores=[])

    # Errores de red y timeouts
    assert proveedor_fallback._es_error_recuperable(httpx.TimeoutException("Tiempo agotado")) is True
    assert proveedor_fallback._es_error_recuperable(httpx.RequestError("Fallo de socket")) is True

    # Códigos HTTP recuperables (rate limit y errores transitorios de servidor)
    for codigo_estado in [429, 500, 502, 503, 504]:
        mock_respuesta = MagicMock(spec=httpx.Response)
        mock_respuesta.status_code = codigo_estado
        error_http = httpx.HTTPStatusError("Error transitorio", request=MagicMock(), response=mock_respuesta)
        assert proveedor_fallback._es_error_recuperable(error_http) is True

    # Códigos HTTP no recuperables (errores de cliente definitivos)
    for codigo_estado in [400, 401, 403, 404, 422]:
        mock_respuesta = MagicMock(spec=httpx.Response)
        mock_respuesta.status_code = codigo_estado
        error_http = httpx.HTTPStatusError("Error cliente", request=MagicMock(), response=mock_respuesta)
        assert proveedor_fallback._es_error_recuperable(error_http) is False

    # Errores genéricos
    assert proveedor_fallback._es_error_recuperable(ValueError("Parametro invalido")) is False
    assert proveedor_fallback._es_error_recuperable(RuntimeError("Fallo interno")) is False


@pytest.mark.anyio
async def test_generar_explicacion_reintenta_primario_recuperable_y_conmuta_a_secundario(monkeypatch):
    """Verifica que un error recuperable reintente según max_retries y luego conmute al secundario."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_primario = ProveedorFalso("primario")
    proveedor_secundario = ProveedorFalso("secundario")

    # Primario falla con timeout siempre
    async def primario_falla(*args, **kwargs):
        proveedor_primario.llamadas_explicacion += 1
        raise httpx.TimeoutException("Timeout simulado en primario")

    proveedor_primario.generar_explicacion = primario_falla

    orquestador = FallbackProveedor(
        proveedores=[proveedor_primario, proveedor_secundario],
        reintentos_primario=2,
        reintentos_secundario=1,
        espera_base=0.01,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.generar_explicacion(
        mensaje_usuario="quiero verduras",
        productos_filtrados=[],
    )

    assert resultado == "Explicacion generada por secundario"
    # 1 intento inicial + 2 reintentos = 3 llamadas al primario
    assert proveedor_primario.llamadas_explicacion == 3
    # 1 llamada exitosa al secundario
    assert proveedor_secundario.llamadas_explicacion == 1
    # Se invocó asyncio.sleep 2 veces para los reintentos del primario
    assert mock_dormir.await_count == 2


@pytest.mark.anyio
async def test_generar_explicacion_no_reintenta_error_400_no_recuperable(monkeypatch):
    """Verifica que un error HTTP 400 en primario no realice reintentos y conmute inmediatamente al secundario."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_primario = ProveedorFalso("primario")
    proveedor_secundario = ProveedorFalso("secundario")

    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.status_code = 400
    error_400 = httpx.HTTPStatusError("Bad Request", request=MagicMock(), response=mock_respuesta)

    async def primario_error_400(*args, **kwargs):
        proveedor_primario.llamadas_explicacion += 1
        raise error_400

    proveedor_primario.generar_explicacion = primario_error_400

    orquestador = FallbackProveedor(
        proveedores=[proveedor_primario, proveedor_secundario],
        reintentos_primario=3,
        reintentos_secundario=1,
    )

    resultado = await orquestador.generar_explicacion(
        mensaje_usuario="quiero pan",
        productos_filtrados=[],
    )

    assert resultado == "Explicacion generada por secundario"
    # Exactamente 1 llamada en primario sin reintentos
    assert proveedor_primario.llamadas_explicacion == 1
    assert proveedor_secundario.llamadas_explicacion == 1
    # Cero llamadas a sleep porque no hubo reintentos
    assert mock_dormir.await_count == 0


@pytest.mark.anyio
async def test_fallback_extraer_filtros_conmuta_gemini_429_a_groq(monkeypatch):
    """Verifica que FallbackProveedor extrae filtros: ante un 429 recuperable en Gemini reintenta y conmuta a Groq."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_gemini = ProveedorFalso("gemini")

    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.status_code = 429
    error_429 = httpx.HTTPStatusError("Rate limit", request=MagicMock(), response=mock_respuesta)

    async def gemini_falla(*args, **kwargs):
        proveedor_gemini.llamadas_extraccion += 1
        raise error_429

    proveedor_gemini.extraer_filtros = gemini_falla

    proveedor_groq = ProveedorFalso("groq")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_gemini, proveedor_groq],
        reintentos_primario=1,
        reintentos_secundario=0,
        espera_base=0.01,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.extraer_filtros(mensaje="quiero verduras")

    assert resultado.error is None
    assert resultado.filtros == {}
    # El 429 fue recuperable: un reintento en Gemini y luego la conmutacion a Groq.
    assert proveedor_gemini.llamadas_extraccion == 2
    assert proveedor_groq.llamadas_extraccion == 1
    assert mock_dormir.await_count == 1


@pytest.mark.anyio
async def test_fallback_extraer_filtros_sin_clave_api_conmuta_sin_reintentos(monkeypatch):
    """Verifica que un proveedor sin clave API conmute al siguiente sin reintentos ni sleep."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_gemini_sin_clave = GeminiProveedor(clave_api=None, url_endpoint="https://ejemplo.com/gemini")
    proveedor_groq = ProveedorFalso("groq")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_gemini_sin_clave, proveedor_groq],
        reintentos_primario=3,
        reintentos_secundario=3,
        espera_base=5.0,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.extraer_filtros(mensaje="quiero verduras")

    assert resultado.error is None
    assert resultado.filtros == {}
    # Error no recuperable (clave ausente): conmuta sin reintentos ni espera.
    assert mock_dormir.await_count == 0
    assert proveedor_groq.llamadas_extraccion == 1


@pytest.mark.anyio
async def test_fallback_log_incluye_tipo_excepcion_y_oculta_key_en_url(caplog):
    """Verifica que el log de fallo incluya el tipo de excepción aunque el mensaje esté vacío y oculte key=."""
    proveedor_timeout = ProveedorFalso("gemini")

    async def falla_timeout_vacio(*args, **kwargs):
        raise httpx.ReadTimeout("")

    proveedor_timeout.generar_explicacion = falla_timeout_vacio

    proveedor_url_secreta = ProveedorFalso("groq")

    async def falla_con_key(*args, **kwargs):
        raise httpx.RequestError(
            "Fallo llamando a https://generativelanguage.googleapis.com/v1beta?key=secreto123&alt=json"
        )

    proveedor_url_secreta.generar_explicacion = falla_con_key

    orquestador = FallbackProveedor(
        proveedores=[proveedor_timeout, proveedor_url_secreta],
        reintentos_primario=0,
        reintentos_secundario=0,
    )

    with caplog.at_level("WARNING"):
        with pytest.raises(RuntimeError, match="key=\\*\\*\\*"):
            await orquestador.generar_explicacion(mensaje_usuario="hola", productos_filtrados=[])

    assert "gemini falló tras 1 intentos: ReadTimeout" in caplog.text
    assert "groq falló tras 1 intentos: RequestError:" in caplog.text
    assert "secreto123" not in caplog.text
    assert "key=***" in caplog.text
