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


def _aplicar_datos_vigentes(destino: dict, datos_producto: dict) -> None:
    """Actualiza en destino los campos que el catálogo reporta como vigentes, si existen."""
    if "precio" in datos_producto:
        destino["precio"] = datos_producto["precio"]
    if "unidad" in datos_producto:
        destino["unidad"] = datos_producto["unidad"]
    if "supermercado" in datos_producto:
        destino["supermercado"] = datos_producto["supermercado"]
    if "marca" in datos_producto:
        destino["marca"] = datos_producto["marca"]
    if "url_imagen" in datos_producto:
        destino["url_imagen"] = datos_producto["url_imagen"]


async def _ejecutar_etapas_1_y_2(
    proveedor: ProveedorIA, repositorio: ProductoRepositorio, mensaje: str
) -> tuple[dict | None, tuple | None]:
    """
    Ejecuta las etapas 1 y 2 dentro del plazo total acotado.

    Devuelve un par `(respuesta, datos_etapa_3)`: con la respuesta ya armada cuando el
    proveedor no encuentra filtros o el catálogo no está disponible; o con los datos de
    la etapa 3 cuando ya hay candidatos validados para explicar.
    """
    with anyio.fail_after(config.CHAT_TIMEOUT_TOTAL):
        # Etapa 1: extracción de filtros por function calling.
        resultado = await proveedor.extraer_filtros(mensaje)

        if resultado.error:
            logger.error("Error del proveedor IA en extracción: %s", resultado.error)
            raise AsistenteNoDisponible(resultado.error)

        if not resultado.filtros:
            logger.debug("Sin function calling (texto libre omitido por anti-alucinación): %s", resultado.texto_libre)
            return _respuesta_chat(TEXTO_SIN_FILTROS, None, [], resultado.tiempo_extraccion_ms, 0), None

        # Etapa 2: filtrado determinista. La carga de Catálogo es bloqueante y va en un hilo.
        productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
        if not productos:
            return _respuesta_chat(
                TEXTO_CATALOGO_NO_DISPONIBLE, resultado.filtros, [], resultado.tiempo_extraccion_ms, 0
            ), None

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
                    candidatos_filtrados = [
                        candidato
                        for candidato in candidatos
                        if candidato["producto_id"] in disponibilidad
                        and disponibilidad[candidato["producto_id"]].get("estado_producto") == "disponible"
                        and disponibilidad[candidato["producto_id"]].get("estado_precio") == "vigente"
                    ]
                    candidatos_actualizados = []
                    for candidato in candidatos_filtrados:
                        # Copiar el candidato antes de aplicar datos vigentes: nunca mutar la lista original.
                        candidato_actualizado = dict(candidato)
                        _aplicar_datos_vigentes(
                            candidato_actualizado,
                            disponibilidad[candidato_actualizado["producto_id"]].get("producto", {}),
                        )
                        candidatos_actualizados.append(candidato_actualizado)
                    candidatos = candidatos_actualizados

                # Revalidar precio_max contra los precios vigentes tras consultar_disponibilidad
                precio_max = resultado.filtros.get("precio_max")
                if isinstance(precio_max, (int, float)) and not isinstance(precio_max, bool) and precio_max > 0:
                    candidatos = [candidato for candidato in candidatos if candidato["precio"] <= precio_max]

                if not candidatos:
                    ingredientes_faltantes.append(ingrediente["ingrediente"])
                    continue

                # Ordenar por precio vigente ascendente y recortar a maximo 3 DESPUÉS de actualizar precios.
                candidatos.sort(key=lambda candidato: candidato["precio"])
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
                # Copiar el producto antes de aplicar datos vigentes si está disponible.
                if disponibilidad is not None and producto_encontrado is not None:
                    producto_encontrado = dict(producto_encontrado)
                    identificador_producto = str(producto_encontrado.get("id"))
                    if identificador_producto in disponibilidad:
                        _aplicar_datos_vigentes(
                            producto_encontrado, disponibilidad[identificador_producto].get("producto", {})
                        )
                productos_receta.append(producto_encontrado)

            return (
                None,
                (
                    True,
                    productos_receta,
                    resultado.filtros,
                    resultado.tiempo_extraccion_ms,
                    ingredientes_receta_validados,
                    ingredientes_faltantes,
                ),
            )

        # Camino no-receta: filtrar productos con los filtros aplicados
        productos_filtrados = filtrar_productos(productos, resultado.filtros)
        return (
            None,
            (
                False,
                productos_filtrados,
                resultado.filtros,
                resultado.tiempo_extraccion_ms,
                None,
                None,
            ),
        )


async def procesar_mensaje(proveedor: ProveedorIA, repositorio: ProductoRepositorio, mensaje: str) -> dict:
    """
    Ejecuta las etapas 1 y 2 con plazo total acotado y la etapa 3 con respaldo.

    - `_ejecutar_etapas_1_y_2` corre dentro del fail_after (etapas 1 + 2).
    - Si devuelve respuesta temprana, se devuelve directa.
    - Si devuelve los datos de la etapa 3, esta se ejecuta FUERA del fail_after con un
      plazo acotado por el tiempo restante, para que una explicación lenta nunca devuelva 503.
    """
    inicio = time.monotonic()

    try:
        respuesta, datos_etapa_3 = await _ejecutar_etapas_1_y_2(proveedor, repositorio, mensaje)
    except TimeoutError as error_tiempo:
        raise AsistenteNoDisponible("Tiempo total agotado en el asistente") from error_tiempo

    if respuesta is not None:
        return respuesta

    (
        _es_receta,
        productos_etapa_3,
        filtros_etapa_3,
        tiempo_extraccion_ms,
        ingredientes_receta,
        ingredientes_faltantes,
    ) = datos_etapa_3

    # Fuera del bloque fail_after: calcular tiempo restante y ejecutar Etapa 3
    tiempo_restante = max(0.5, config.CHAT_TIMEOUT_TOTAL - (time.monotonic() - inicio))

    explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
        proveedor.generar_explicacion,
        mensaje,
        productos_etapa_3,
        tiempo_limite=tiempo_restante,
    )

    return _respuesta_chat(
        explicacion,
        filtros_etapa_3,
        productos_etapa_3,
        tiempo_extraccion_ms,
        tiempo_explicacion_ms,
        ingredientes_receta=ingredientes_receta,
        ingredientes_faltantes=ingredientes_faltantes,
    )
