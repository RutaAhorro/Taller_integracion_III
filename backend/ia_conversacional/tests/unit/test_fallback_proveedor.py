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

from app.proveedores.base import ProveedorIA, ResultadoExtraccion
from app.proveedores.fallback_proveedor import FallbackProveedor


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
    monkeypatch.setattr("app.proveedores.fallback_proveedor.asyncio.sleep", mock_dormir)

    proveedor_primario = ProveedorFalso("primario")
    proveedor_secundario = ProveedorFalso("secundario")

    # Primario falla con timeout siempre
    async def primario_falla(*args, **kwargs):
        proveedor_primario.llamadas_explicacion += 1
        raise httpx.TimeoutException("Timeout simulado en primario")

    proveedor_primario.generar_explicacion = primario_falla

    orquestador = FallbackProveedor(
        proveedores=[proveedor_primario, proveedor_secundario],
        max_retries_primario=2,
        max_retries_secundario=1,
        backoff_base=0.01,
        jitter_max=0.0,
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
    monkeypatch.setattr("app.proveedores.fallback_proveedor.asyncio.sleep", mock_dormir)

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
        max_retries_primario=3,
        max_retries_secundario=1,
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
