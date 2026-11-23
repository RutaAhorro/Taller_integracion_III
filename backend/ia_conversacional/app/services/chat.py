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

from app import config
from app.domain.filtros import filtrar_productos, vincular_ingredientes_catalogo
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
    ingredientes_receta: list | None = None,
    ingredientes_faltantes: list | None = None,
) -> dict:
    """Arma la respuesta con la misma forma en todos los casos, para que el cliente no deba adivinar claves."""
    return {
        "respuesta": texto,
        "filtros_aplicados": filtros,
        "productos_recomendados": productos,
        "tiempo_extraccion_ms": tiempo_extraccion_ms,
        "tiempo_explicacion_ms": tiempo_explicacion_ms,
        "ingredientes_receta": ingredientes_receta or [],
        "ingredientes_faltantes": ingredientes_faltantes or [],
    }


# ===== Etapa 3 con respaldo =====


async def generar_explicacion_con_tiempo(
    generador: GeneradorExplicacion, mensaje_usuario: str, productos_filtrados: list, tiempo_limite: float | None = None
) -> tuple[str, int]:
    """
    Ejecuta la redacción con un plazo opcional, mide su duración en milisegundos
    y nunca propaga errores.

    Los productos filtrados ya son válidos: la explicación es un complemento,
    así que un fallo de red o de cuota (o plazo agotado) se reemplaza por un
    texto de respaldo. Si se provee tiempo_limite, cualquier vencimiento
    devuelve TEXTO_EXPLICACION_RESPALDO en lugar de propagar error/503.
    """
    if not productos_filtrados:
        return TEXTO_SIN_PRODUCTOS, 0

    inicio = time.perf_counter()
    explicacion = None
    try:
        if tiempo_limite is not None:
            with anyio.move_on_after(tiempo_limite):
                explicacion = await generador(mensaje_usuario, productos_filtrados)
        else:
            explicacion = await generador(mensaje_usuario, productos_filtrados)
    except Exception as error_explicacion:
        logger.warning("No se pudo generar la explicacion: %s", error_explicacion)
        explicacion = None
    tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)

    return explicacion or TEXTO_EXPLICACION_RESPALDO, tiempo_ms


# ===== Pipeline =====


# Señal personalizada para indicar que la Etapa 3 debe ejecutarse fuera de fail_after
class _ProceedToStage3(Exception):
    """Excepción usada para salir del bloque fail_after y continuar con la Etapa 3 fuera de él."""

    def __init__(
        self, es_receta, productos_etapa_3, filtros_etapa_3, ingredientes_receta=None, ingredientes_faltantes=None
    ):
        self.es_receta = es_receta
        self.productos_etapa_3 = productos_etapa_3
        self.filtros_etapa_3 = filtros_etapa_3
        self.ingredientes_receta = ingredientes_receta or []
        self.ingredientes_faltantes = ingredientes_faltantes or []
        super().__init__("proceed_to_stage_3")


async def procesar_mensaje(proveedor: ProveedorIA, repositorio: ProductoRepositorio, mensaje: str) -> dict:
    """Ejecuta las tres etapas con plazo total acotado.

    - fail_after cubre solo la Etapa 1 + Etapa 2 (extracción, catálogo, vinculación, disponibilidad).
    - La Etapa 3 (explicación) se ejecuta FUERA del fail_after con un plazo acotado,
      para que una explicación lenta nunca devuelva 503.
    """
    inicio = time.monotonic()

    try:
        with anyio.fail_after(config.CHAT_TIMEOUT_TOTAL):
            # Etapa 1: extracción de filtros por function calling.
            resultado = await proveedor.extraer_filtros(mensaje)

            if resultado.error:
                logger.error("Error del proveedor IA en extracción: %s", resultado.error)
                raise AsistenteNoDisponible(resultado.error)

            if not resultado.filtros:
                logger.debug(
                    "Sin function calling (texto libre omitido por anti-alucinación): %s", resultado.texto_libre
                )
                return _respuesta_chat(TEXTO_SIN_FILTROS, None, [], resultado.tiempo_extraccion_ms, 0)

            # Etapa 2: filtrado determinista. La carga de Catálogo es bloqueante y va en un hilo.
            productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
            if not productos:
                return _respuesta_chat(
                    TEXTO_CATALOGO_NO_DISPONIBLE, resultado.filtros, [], resultado.tiempo_extraccion_ms, 0
                )

            if "ingredientes" in resultado.filtros:
                # Receta: aplicar filtros base de precio/categoria/distancia antes de vincular ingredientes.
                filtros_base = {
                    clave: valor
                    for clave, valor in resultado.filtros.items()
                    if clave in ("precio_max", "categoria", "distancia_max")
                }
                if filtros_base:
                    productos = filtrar_productos(productos, filtros_base)

                # NOTE: limite_candidatos=None para no descartar un 4º candidato vigente antes de validar
                ingredientes_receta, ingredientes_faltantes = vincular_ingredientes_catalogo(
                    productos, resultado.filtros["ingredientes"], limite_candidatos=None
                )

                ids_candidatos = sorted(
                    {
                        candidato["producto_id"]
                        for ingrediente in ingredientes_receta
                        for candidato in ingrediente["candidatos"]
                    }
                )
                disponibilidad = await anyio.to_thread.run_sync(repositorio.consultar_disponibilidad, ids_candidatos)

                # Si disponibilidad es None, loguear warning y continuar con caché en memoria
                if disponibilidad is None:
                    logger.warning("Disponibilidad no validada para receta: usando caché en memoria.")

                # Validar disponibilidad y actualizar campos de los candidatos desde disponibilidad
                ingredientes_receta_validados = []
                productos_receta = []
                for ingrediente in ingredientes_receta:
                    candidatos = ingrediente["candidatos"]
                    if disponibilidad is not None:
                        candidatos = [
                            candidato
                            for candidato in candidatos
                            if candidato["producto_id"] in disponibilidad
                            and disponibilidad[candidato["producto_id"]].get("estado_producto") == "disponible"
                            and disponibilidad[candidato["producto_id"]].get("estado_precio") == "vigente"
                        ]
                        # Actualizar campos vigentes desde disponibilidad[pid].get("producto", {})
                        for candidato in candidatos:
                            pid = candidato["producto_id"]
                            if pid in disponibilidad:
                                prod_data = disponibilidad[pid].get("producto", {})
                                if "precio" in prod_data:
                                    candidato["precio"] = prod_data["precio"]
                                if "unidad" in prod_data:
                                    candidato["unidad"] = prod_data["unidad"]
                                if "supermercado" in prod_data:
                                    candidato["supermercado"] = prod_data["supermercado"]
                                if "marca" in prod_data:
                                    candidato["marca"] = prod_data["marca"]
                                if "url_imagen" in prod_data:
                                    candidato["url_imagen"] = prod_data["url_imagen"]

                    if not candidatos:
                        ingredientes_faltantes.append(ingrediente["ingrediente"])
                        continue

                    # Recortar candidatos a máximo 3 DESPUÉS de validar disponibilidad
                    candidatos = candidatos[:3]

                    ingredientes_receta_validados.append(
                        {
                            "ingrediente": ingrediente["ingrediente"],
                            "cantidad_sugerida": ingrediente["cantidad_sugerida"],
                            "candidatos": candidatos,
                        }
                    )
                    producto_encontrado = next(
                        (producto for producto in productos if str(producto.get("id")) == candidatos[0]["producto_id"]),
                        candidatos[0],
                    )
                    # Actualizar precio del producto desde disponibilidad si está disponible
                    if disponibilidad is not None and producto_encontrado is not None:
                        pid = str(producto_encontrado.get("id"))
                        if pid in disponibilidad:
                            prod_data = disponibilidad[pid].get("producto", {})
                            if "precio" in prod_data:
                                producto_encontrado["precio"] = prod_data["precio"]
                            if "unidad" in prod_data:
                                producto_encontrado["unidad"] = prod_data["unidad"]
                            if "supermercado" in prod_data:
                                producto_encontrado["supermercado"] = prod_data["supermercado"]
                            if "marca" in prod_data:
                                producto_encontrado["marca"] = prod_data["marca"]
                            if "url_imagen" in prod_data:
                                producto_encontrado["url_imagen"] = prod_data["url_imagen"]
                    productos_receta.append(producto_encontrado)

                # Señalizar que la Etapa 3 debe continuar fuera de fail_after
                raise _ProceedToStage3(
                    es_receta=True,
                    productos_etapa_3=productos_receta,
                    filtros_etapa_3=resultado.filtros,
                    ingredientes_receta=ingredientes_receta_validados,
                    ingredientes_faltantes=ingredientes_faltantes,
                )

            # Camino no-receta: filtrar productos con los filtros aplicados
            productos_filtrados = filtrar_productos(productos, resultado.filtros)
            raise _ProceedToStage3(
                es_receta=False,
                productos_etapa_3=productos_filtrados,
                filtros_etapa_3=resultado.filtros,
            )

    except _ProceedToStage3 as signal:
        # Fuera del bloque fail_after: calcular tiempo restante y ejecutar Etapa 3
        tiempo_restante = max(0.5, config.CHAT_TIMEOUT_TOTAL - (time.monotonic() - inicio))

        explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
            proveedor.generar_explicacion,
            mensaje,
            signal.productos_etapa_3,
            tiempo_limite=tiempo_restante,
        )

        return _respuesta_chat(
            explicacion,
            signal.filtros_etapa_3,
            signal.productos_etapa_3,
            resultado.tiempo_extraccion_ms,
            tiempo_explicacion_ms,
            ingredientes_receta=signal.ingredientes_receta,
            ingredientes_faltantes=signal.ingredientes_faltantes,
        )

    except TimeoutError as error_tiempo:
        raise AsistenteNoDisponible("Tiempo total agotado en el asistente") from error_tiempo
