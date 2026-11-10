"""
Proveedor de IA con reintentos inteligentes (backoff exponencial) y fallback.
Prioridad: Gemini (con reintentos) → Groq (con reintentos simples).
"""

import asyncio
import logging
import random
from typing import Any

import httpx

from app import config
from app.proveedores.base import ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)


class FallbackProveedor(ProveedorIA):
    """
    Intenta con el primer proveedor (ej. Gemini). Si falla con errores
    recuperables (429, 503, timeout), reintenta con backoff exponencial.
    Si tras varios reintentos sigue fallando, pasa al segundo proveedor (ej. Groq).
    """

    nombre = "fallback"

    def __init__(
        self,
        proveedores: list[ProveedorIA],
        max_retries_primario: int | None = None,
        max_retries_secundario: int | None = None,
        backoff_base: float | None = None,
        jitter_max: float | None = None,
    ):
        self.proveedores = proveedores
        self.max_retries_primario = (
            max_retries_primario
            if max_retries_primario is not None
            else config.FALLBACK_RETRIES_PRIMARIO
        )
        self.max_retries_secundario = (
            max_retries_secundario
            if max_retries_secundario is not None
            else config.FALLBACK_RETRIES_SECUNDARIO
        )
        self.backoff_base = (
            backoff_base
            if backoff_base is not None
            else config.FALLBACK_BACKOFF_BASE
        )
        self.jitter_max = (
            jitter_max
            if jitter_max is not None
            else config.FALLBACK_JITTER_MAX
        )

    def _es_error_recuperable(self, error: Exception) -> bool:
        """Determina si el error es transitorio (podemos reintentar)."""
        if isinstance(error, httpx.TimeoutException):
            return True
        if isinstance(error, httpx.RequestError):
            return True
        if isinstance(error, httpx.HTTPStatusError):
            # 429 = rate limit, 503 = servidor sobrecargado, 500 = error interno (a veces transitorio)
            return error.response.status_code in {429, 500, 502, 503, 504}
        return False

    async def _ejecutar_con_reintentos(
        self,
        proveedor: ProveedorIA,
        metodo: str,
        *args,
        max_retries: int = 3,
        **kwargs,
    ) -> Any:
        """
        Ejecuta un método del proveedor con reintentos y backoff.
        """
        ultimo_error = None
        for intento in range(max_retries + 1):
            try:
                if metodo == "extraer_filtros":
                    return await proveedor.extraer_filtros(*args, **kwargs)
                elif metodo == "generar_explicacion":
                    return await proveedor.generar_explicacion(*args, **kwargs)
                else:
                    raise ValueError(f"Método desconocido: {metodo}")
            except Exception as error:
                ultimo_error = error
                if not self._es_error_recuperable(error):
                    logger.warning("Error no recuperable en %s: %s", proveedor.nombre, error)
                    raise
                if intento < max_retries:
                    espera = (self.backoff_base * (2 ** intento)) + random.uniform(0, self.jitter_max)
                    logger.info(
                        "Error recuperable en %s (intento %d/%d): %s. Reintentando en %.2fs...",
                        proveedor.nombre,
                        intento + 1,
                        max_retries + 1,
                        error,
                        espera,
                    )
                    await asyncio.sleep(espera)
                else:
                    logger.error("%s falló tras %d intentos: %s", proveedor.nombre, max_retries + 1, error)
                    raise

        # Esto nunca debería ejecutarse, pero por si acaso
        raise ultimo_error or RuntimeError("Fallo desconocido")

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        ultimo_error = None

        for idx, proveedor in enumerate(self.proveedores):
            # El primario tiene más reintentos; los secundarios menos
            max_retries = self.max_retries_primario if idx == 0 else self.max_retries_secundario
            try:
                resultado = await self._ejecutar_con_reintentos(
                    proveedor,
                    "extraer_filtros",
                    mensaje,
                    max_retries=max_retries,
                )
                # Si llegamos aquí, el proveedor respondió exitosamente
                nota = f"Usado proveedor: {proveedor.nombre}"
                if resultado.nota:
                    nota += f" (nota: {resultado.nota})"
                resultado.nota = nota
                return resultado
            except Exception as error:
                ultimo_error = error
                logger.warning("%s falló definitivamente, pasando al siguiente...", proveedor.nombre)
                continue

        # Si todos fallaron
        return ResultadoExtraccion(
            error=f"Todos los proveedores fallaron. Último error: {ultimo_error}",
            nota="Se intentaron: " + ", ".join(prov.nombre for prov in self.proveedores),
        )

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        ultimo_error = None

        for idx, proveedor in enumerate(self.proveedores):
            max_retries = self.max_retries_primario if idx == 0 else self.max_retries_secundario
            try:
                explicacion = await self._ejecutar_con_reintentos(
                    proveedor,
                    "generar_explicacion",
                    mensaje_usuario,
                    productos_filtrados,
                    max_retries=max_retries,
                )
                if explicacion and explicacion.strip():
                    logger.info("Explicación generada por %s", proveedor.nombre)
                    return explicacion
                else:
                    raise ValueError("Explicación vacía o None")
            except Exception as error:
                ultimo_error = error
                logger.warning("%s falló generando explicación, pasando al siguiente...", proveedor.nombre)
                continue

        # Si todos fallaron
        raise RuntimeError(f"Todos los proveedores fallaron al generar explicación. Último error: {ultimo_error}")
