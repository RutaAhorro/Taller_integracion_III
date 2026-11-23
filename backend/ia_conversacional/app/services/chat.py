"""
Servicio de chat: orquesta el pipeline de tres etapas.

Extracción por LLM → filtrado determinista sobre el catálogo real →
explicación por LLM solo con los productos validados. No conoce HTTP: el
router traduce `AsistenteNoDisponible` a 503.
"""

import logging
import time
from collections.abc import Awaitable, Callable

import anyio

from app.domain.filtros import filtrar_productos
from app.domain.proveedores import MILISEGUNDOS_POR_SEGUNDO, ProveedorIA
from app.repositories.catalogo import ProductoRepositorio

logger = logging.getLogger(__name__)

# ===== Constantes =====

TEXTO_SIN_FILTROS = 'No logré identificar qué productos buscas. ¿Puedes reformularlo? Ej: "verduras bajo $2.000"'
TEXTO_CATALOGO_NO_DISPONIBLE = (
    "El catálogo de productos no se encuentra disponible en este momento. Por favor, intenta más tarde."
)
TEXTO_SIN_PRODUCTOS = "No encontramos productos que coincidan con tu busqueda."
TEXTO_EXPLICACION_RESPALDO = "Encontramos estos productos para tu búsqueda."

GeneradorExplicacion = Callable[[str, list], Awaitable[str]]


class AsistenteNoDisponible(Exception):
    """Ningún proveedor pudo extraer filtros: el chat no puede responder con datos validados."""


# ===== Respuesta =====


def _respuesta_chat(
    texto: str,
    filtros: dict | None,
    productos: list,
    tiempo_extraccion_ms: int,
    tiempo_explicacion_ms: int,
) -> dict:
    """Arma la respuesta con la misma forma en todos los casos, para que el cliente no deba adivinar claves."""
    return {
        "respuesta": texto,
        "filtros_aplicados": filtros,
        "productos_recomendados": productos,
        "tiempo_extraccion_ms": tiempo_extraccion_ms,
        "tiempo_explicacion_ms": tiempo_explicacion_ms,
    }


# ===== Etapa 3 con respaldo =====


async def generar_explicacion_con_tiempo(
    generador: GeneradorExplicacion, mensaje_usuario: str, productos_filtrados: list
) -> tuple[str, int]:
    """
    Ejecuta la redacción, mide su duración en milisegundos y nunca propaga errores.

    Los productos filtrados ya son válidos: la explicación es un complemento,
    así que un fallo de red o de cuota se reemplaza por un texto de respaldo.
    """
    if not productos_filtrados:
        return TEXTO_SIN_PRODUCTOS, 0

    inicio = time.perf_counter()
    try:
        explicacion = await generador(mensaje_usuario, productos_filtrados)
    except Exception as error_explicacion:
        logger.warning("No se pudo generar la explicacion: %s", error_explicacion)
        explicacion = None
    tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)

    return explicacion or TEXTO_EXPLICACION_RESPALDO, tiempo_ms


# ===== Pipeline =====


async def procesar_mensaje(proveedor: ProveedorIA, repositorio: ProductoRepositorio, mensaje: str) -> dict:
    """Ejecuta las tres etapas; lanza AsistenteNoDisponible solo si la extracción falla en todos los proveedores."""
    # Etapa 1: extracción de filtros por function calling.
    resultado = await proveedor.extraer_filtros(mensaje)
    if resultado.error:
        logger.error("Error del proveedor IA en extracción: %s", resultado.error)
        raise AsistenteNoDisponible(resultado.error)

    if not resultado.filtros:
        logger.debug("Sin function calling (texto libre omitido por anti-alucinación): %s", resultado.texto_libre)
        return _respuesta_chat(TEXTO_SIN_FILTROS, None, [], resultado.tiempo_extraccion_ms, 0)

    # Etapa 2: filtrado determinista. La carga de Catálogo es bloqueante y va en un hilo.
    productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
    if not productos:
        return _respuesta_chat(TEXTO_CATALOGO_NO_DISPONIBLE, resultado.filtros, [], resultado.tiempo_extraccion_ms, 0)

    productos_filtrados = filtrar_productos(productos, resultado.filtros)

    # Etapa 3: explicación solo con los productos validados en la etapa 2.
    explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
        proveedor.generar_explicacion, mensaje, productos_filtrados
    )
    return _respuesta_chat(
        explicacion,
        resultado.filtros,
        productos_filtrados,
        resultado.tiempo_extraccion_ms,
        tiempo_explicacion_ms,
    )
