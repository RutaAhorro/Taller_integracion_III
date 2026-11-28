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
import re
from collections.abc import Awaitable, Callable
from contextvars import ContextVar
from typing import TypeVar

import httpx

from app import config
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)

# Almacena por tarea asíncrona el proveedor que generó la última explicación sin estado compartido.
_ultimo_proveedor_explicacion: ContextVar[str | None] = ContextVar("ultimo_proveedor_explicacion", default=None)


def reiniciar_ultimo_proveedor_explicacion() -> None:
    """Limpia el registro del proveedor de explicación al iniciar una consulta."""
    _ultimo_proveedor_explicacion.set(None)


def obtener_ultimo_proveedor_explicacion() -> str | None:
    """Devuelve el nombre del proveedor que generó la explicación en la tarea actual, o None."""
    return _ultimo_proveedor_explicacion.get()


# ===== Constantes =====

# 429 = cuota, 5xx = servidor caído o sobrecargado (a veces transitorio).
CODIGOS_HTTP_RECUPERABLES = {429, 500, 502, 503, 504}
FACTOR_BACKOFF = 2
ETAPA_EXTRACCION = "extracción"
ETAPA_EXPLICACION = "explicación"
PATRON_PARAMETRO_CLAVE = re.compile(r"([?&]key=)[^&\s'\"]+")
PATRON_VALOR_CABECERA_ILEGAL = re.compile(r"(?is)(illegal header value\s+b?)(['\"])(?:\\.|(?!\2).)*\2")
PATRON_CABECERA_DICCIONARIO = re.compile(
    r"""(?i)(['"](?:authorization|x-goog-api-key|x-api-key)['"]\s*:\s*b?)(['"])(?:\\.|(?!\2).)*\2"""
)
PATRON_CABECERA_SECRETA = re.compile(
    r"(?i)(authorization\s*[:=]\s*(?:bearer|basic)\s+|(?:x-goog-api-key|x-api-key)\s*[:=]\s*|bearer\s+)[^\s,'\"}\]]+"
)

Resultado = TypeVar("Resultado")


# ===== Utilidades de registro =====


def formatear_error_seguro(error: Exception | None) -> str:
    """Devuelve tipo y detalle del error ocultando claves en URLs, diccionarios y cabeceras HTTP."""
    if error is None:
        return "ErrorDesconocido"
    tipo_error = type(error).__name__
    detalle_sanitizado = PATRON_PARAMETRO_CLAVE.sub(r"\1***", str(error))
    detalle_sanitizado = PATRON_VALOR_CABECERA_ILEGAL.sub(r"\1\2***\2", detalle_sanitizado)
    detalle_sanitizado = PATRON_CABECERA_DICCIONARIO.sub(r"\1\2***\2", detalle_sanitizado)
    detalle_sanitizado = PATRON_CABECERA_SECRETA.sub(r"\1***", detalle_sanitizado).strip()
    if not detalle_sanitizado:
        return tipo_error
    return f"{tipo_error}: {detalle_sanitizado}"


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
        etapa: str,
    ) -> Resultado:
        """Ejecuta la operación, reintentando solo los errores recuperables con backoff y variación aleatoria."""
        for intento in range(reintentos + 1):
            try:
                return await operacion()
            except Exception as error:
                detalle_error = formatear_error_seguro(error)
                if not self._es_error_recuperable(error):
                    logger.warning(
                        "Falló la %s (error no recuperable) en %s: %s", etapa, proveedor.nombre, detalle_error
                    )
                    raise
                if intento == reintentos:
                    logger.error(
                        "Falló la %s en %s tras %d intentos: %s",
                        etapa,
                        proveedor.nombre,
                        reintentos + 1,
                        detalle_error,
                    )
                    raise
                espera = self.espera_base * (FACTOR_BACKOFF**intento) + random.uniform(0, self.variacion_maxima)
                logger.info(
                    "Falló la %s (error recuperable) en %s (intento %d/%d): %s. Reintentando en %.2fs...",
                    etapa,
                    proveedor.nombre,
                    intento + 1,
                    reintentos + 1,
                    detalle_error,
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
                    ETAPA_EXTRACCION,
                )
                if not resultado.proveedor:
                    resultado.proveedor = proveedor.nombre
                logger.info("Extracción resuelta por %s", resultado.proveedor)
                return resultado
            except Exception as error:
                ultimo_error = error
                logger.warning(
                    "Falló la %s en %s definitivamente, pasando al siguiente...", ETAPA_EXTRACCION, proveedor.nombre
                )

        return ResultadoExtraccion(
            error=f"Todos los proveedores fallaron en {ETAPA_EXTRACCION}. Último error: {formatear_error_seguro(ultimo_error)}"
        )

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        """Devuelve la primera explicación no vacía; lanza RuntimeError si ningún proveedor la logra."""
        ultimo_error = None
        for indice_proveedor, proveedor in enumerate(self.proveedores):
            try:
                explicacion = await self._ejecutar_con_reintentos(
                    proveedor,
                    lambda proveedor=proveedor: proveedor.generar_explicacion(mensaje_usuario, productos_filtrados),
                    self._reintentos_para(indice_proveedor),
                    ETAPA_EXPLICACION,
                )
                if not explicacion or not explicacion.strip():
                    raise ValueError("Explicación vacía o None")
                _ultimo_proveedor_explicacion.set(proveedor.nombre)
                logger.info("Explicación generada por %s", proveedor.nombre)
                return explicacion
            except Exception as error:
                ultimo_error = error
                logger.warning(
                    "Falló la %s en %s definitivamente, pasando al siguiente...", ETAPA_EXPLICACION, proveedor.nombre
                )

        raise RuntimeError(
            f"Todos los proveedores fallaron en {ETAPA_EXPLICACION}. Último error: {formatear_error_seguro(ultimo_error)}"
        )
