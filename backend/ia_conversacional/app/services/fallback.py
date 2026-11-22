"""
Proveedor de respaldo: reintentos con backoff exponencial y cambio de proveedor.

Prueba los proveedores en orden (Gemini y luego Groq). Ante un error
transitorio reintenta con espera creciente; si se agotan los reintentos,
pasa al siguiente. Los errores no recuperables cambian de proveedor sin
reintentar.
"""

import asyncio
import logging
import random
from collections.abc import Awaitable, Callable
from typing import TypeVar

import httpx

from app import config
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)

# ===== Constantes =====

# 429 = cuota, 5xx = servidor caído o sobrecargado (a veces transitorio).
CODIGOS_HTTP_RECUPERABLES = {429, 500, 502, 503, 504}
FACTOR_BACKOFF = 2

Resultado = TypeVar("Resultado")


# ===== Proveedor de respaldo =====


class FallbackProveedor(ProveedorIA):
    """Encadena proveedores: el primero recibe más reintentos que los siguientes."""

    nombre = "fallback"

    def __init__(
        self,
        proveedores: list[ProveedorIA],
        reintentos_primario: int | None = None,
        reintentos_secundario: int | None = None,
        espera_base: float | None = None,
        variacion_maxima: float | None = None,
    ):
        """Guarda los proveedores; los parámetros omitidos se toman de la configuración."""
        self.proveedores = proveedores
        self.reintentos_primario = (
            reintentos_primario if reintentos_primario is not None else config.FALLBACK_RETRIES_PRIMARIO
        )
        self.reintentos_secundario = (
            reintentos_secundario if reintentos_secundario is not None else config.FALLBACK_RETRIES_SECUNDARIO
        )
        self.espera_base = espera_base if espera_base is not None else config.FALLBACK_BACKOFF_BASE
        self.variacion_maxima = variacion_maxima if variacion_maxima is not None else config.FALLBACK_JITTER_MAX

    def _es_error_recuperable(self, error: Exception) -> bool:
        """Indica si el error es transitorio: red, timeout o un estado HTTP recuperable."""
        if isinstance(error, httpx.HTTPStatusError):
            return error.response.status_code in CODIGOS_HTTP_RECUPERABLES
        # TimeoutException hereda de RequestError.
        return isinstance(error, httpx.RequestError)

    def _reintentos_para(self, indice_proveedor: int) -> int:
        """El primario tiene más reintentos; los secundarios, menos."""
        return self.reintentos_primario if indice_proveedor == 0 else self.reintentos_secundario

    async def _ejecutar_con_reintentos(
        self,
        proveedor: ProveedorIA,
        operacion: Callable[[], Awaitable[Resultado]],
        reintentos: int,
    ) -> Resultado:
        """Ejecuta la operación, reintentando solo los errores recuperables con backoff y variación aleatoria."""
        for intento in range(reintentos + 1):
            try:
                return await operacion()
            except Exception as error:
                if not self._es_error_recuperable(error):
                    logger.warning("Error no recuperable en %s: %s", proveedor.nombre, error)
                    raise
                if intento == reintentos:
                    logger.error("%s falló tras %d intentos: %s", proveedor.nombre, reintentos + 1, error)
                    raise
                espera = self.espera_base * (FACTOR_BACKOFF**intento) + random.uniform(0, self.variacion_maxima)
                logger.info(
                    "Error recuperable en %s (intento %d/%d): %s. Reintentando en %.2fs...",
                    proveedor.nombre,
                    intento + 1,
                    reintentos + 1,
                    error,
                    espera,
                )
                await asyncio.sleep(espera)
        raise AssertionError("inalcanzable: el último intento retorna o relanza")

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        """Devuelve la extracción del primer proveedor que responda, o un resultado con error si fallan todos."""
        ultimo_error = None
        for indice_proveedor, proveedor in enumerate(self.proveedores):
            try:
                resultado = await self._ejecutar_con_reintentos(
                    proveedor,
                    lambda proveedor=proveedor: proveedor.extraer_filtros(mensaje),
                    self._reintentos_para(indice_proveedor),
                )
                logger.info("Extracción resuelta por %s", proveedor.nombre)
                return resultado
            except Exception as error:
                ultimo_error = error
                logger.warning("%s falló definitivamente, pasando al siguiente...", proveedor.nombre)

        return ResultadoExtraccion(error=f"Todos los proveedores fallaron. Último error: {ultimo_error}")

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        """Devuelve la primera explicación no vacía; lanza RuntimeError si ningún proveedor la logra."""
        ultimo_error = None
        for indice_proveedor, proveedor in enumerate(self.proveedores):
            try:
                explicacion = await self._ejecutar_con_reintentos(
                    proveedor,
                    lambda proveedor=proveedor: proveedor.generar_explicacion(mensaje_usuario, productos_filtrados),
                    self._reintentos_para(indice_proveedor),
                )
                if not explicacion or not explicacion.strip():
                    raise ValueError("Explicación vacía o None")
                logger.info("Explicación generada por %s", proveedor.nombre)
                return explicacion
            except Exception as error:
                ultimo_error = error
                logger.warning("%s falló generando explicación, pasando al siguiente...", proveedor.nombre)

        raise RuntimeError(f"Todos los proveedores fallaron al generar explicación. Último error: {ultimo_error}")
