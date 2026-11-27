"""
Servicio de chat: orquesta el pipeline de tres etapas.

Extracción por LLM → filtrado determinista sobre el catálogo real →
explicación por LLM solo con los productos validados. No conoce HTTP: el
router traduce `AsistenteNoDisponible` a 503.

Los candidatos de la etapa 2 salen de una búsqueda dirigida en Catálogo cuando
hay palabras clave o ingredientes: la caché en memoria solo cubre las primeras
páginas del catálogo (las más baratas) y no encontraba, por ejemplo, harina de
almendras. Si la búsqueda falla, se usa la caché como antes.
"""

import logging
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from itertools import zip_longest

import anyio

from app import config
from app.domain.filtros import (
    extraer_terminos_respaldo,
    filtrar_productos,
    terminos_busqueda_catalogo,
    vincular_ingredientes_catalogo,
)
from app.domain.proveedores import MILISEGUNDOS_POR_SEGUNDO, ProveedorIA
from app.repositories.catalogo import FUENTE_MEMORIA, FUENTE_SIN_DATOS, ProductoRepositorio

logger = logging.getLogger(__name__)

# ===== Constantes =====

TEXTO_SIN_FILTROS = 'No logré identificar qué productos buscas. ¿Puedes reformularlo? Ej: "verduras bajo $2.000"'
TEXTO_CATALOGO_NO_DISPONIBLE = (
    "El catálogo de productos no se encuentra disponible en este momento. Por favor, intenta más tarde."
)
TEXTO_SIN_PRODUCTOS = "No encontramos productos que coincidan con tu busqueda."
TEXTO_EXPLICACION_RESPALDO = "Encontramos estos productos para tu búsqueda."

# Estados de los datos usados en la respuesta (SUP-315).
ESTADO_VIGENTE = "vigente"
ESTADO_MEMORIA = "memoria"
ESTADO_MEMORIA_ANTIGUA = "memoria_antigua"
ESTADO_SIN_DATOS = FUENTE_SIN_DATOS
ESTADO_SIN_VALIDAR_DISPONIBILIDAD = "sin_validar_disponibilidad"
AVISO_MEMORIA_ANTIGUA = (
    "Nota: los datos provienen de una carga previa antigua del catálogo y pueden estar desactualizados."
)

AVISO_CARGA_PREVIA = "Nota: los datos provienen de una carga previa del catálogo y pueden haber cambiado."
AVISO_SIN_VALIDAR_DISPONIBILIDAD = "Nota: no fue posible validar el stock y el precio en tiempo real."

GeneradorExplicacion = Callable[[str, list], Awaitable[str]]


class AsistenteNoDisponible(Exception):
    """Ningún proveedor pudo extraer filtros: el chat no puede responder con datos validados."""


# ===== Datos de la etapa 3 =====


@dataclass(frozen=True)
class DatosEtapa3:
    """Datos validados de la etapa 2 que la etapa 3 necesita explicar (SUP-315)."""

    productos: list
    filtros: dict | None
    tiempo_extraccion_ms: int
    ingredientes_receta: list | None = None
    ingredientes_faltantes: list | None = None
    estado_datos: str = ESTADO_VIGENTE


# ===== Respuesta =====


def _respuesta_chat(
    texto: str,
    filtros: dict | None,
    productos: list,
    tiempo_extraccion_ms: int,
    tiempo_explicacion_ms: int,
    ingredientes_receta: list | None = None,
    ingredientes_faltantes: list | None = None,
    estado_datos: str = ESTADO_VIGENTE,
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
        "estado_datos": estado_datos,
    }


def _determinar_estado_datos(repositorio: ProductoRepositorio) -> str:
    """Clasifica la carga del catálogo: antigua, en memoria, sin datos o vigente (SUP-315)."""
    if repositorio.fuente == FUENTE_SIN_DATOS:
        return ESTADO_SIN_DATOS
    if repositorio.datos_antiguos:
        return ESTADO_MEMORIA_ANTIGUA
    if repositorio.fuente == FUENTE_MEMORIA:
        return ESTADO_MEMORIA
    return ESTADO_VIGENTE


def _anexar_aviso_estado_datos(explicacion: str, estado_datos: str) -> str:
    """Añade al texto una advertencia breve cuando la respuesta se apoya en datos degradados (SUP-315)."""
    if estado_datos == ESTADO_MEMORIA_ANTIGUA:
        return f"{explicacion} {AVISO_MEMORIA_ANTIGUA}"
    if estado_datos == ESTADO_MEMORIA:
        return f"{explicacion} {AVISO_CARGA_PREVIA}"
    if estado_datos == ESTADO_SIN_VALIDAR_DISPONIBILIDAD:
        return f"{explicacion} {AVISO_SIN_VALIDAR_DISPONIBILIDAD}"
    return explicacion


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


# ===== Candidatos de la etapa 2 =====


def _frases_a_buscar(filtros: dict) -> list[str]:
    """Devuelve los ingredientes de la receta o, si no hay receta, las palabras clave extraídas."""
    if "ingredientes" in filtros:
        return [ingrediente["nombre"] for ingrediente in filtros["ingredientes"]]
    return list(filtros.get("palabras_clave", []))


def _precio_maximo(filtros: dict) -> float | None:
    """Devuelve el precio máximo extraído si es un número positivo; si no, None."""
    precio_max = filtros.get("precio_max")
    if isinstance(precio_max, (int, float)) and not isinstance(precio_max, bool) and precio_max > 0:
        return float(precio_max)
    return None


def _planificar_consultas(frases: list[str]) -> tuple[list[str], bool]:
    """
    Ordena las consultas a Catálogo y las recorta al máximo configurado.

    Se intercalan por prioridad (primero la frase completa de cada palabra clave
    o ingrediente, luego sus variantes) para que el recorte afecte a las variantes
    y no deje un ingrediente entero sin buscar. Devuelve también si la búsqueda
    queda incompleta: por recorte o porque una frase no tiene ningún término que
    Catálogo acepte (p. ej. "te"), caso en que solo la caché puede resolverla.
    """
    terminos_por_frase = [terminos_busqueda_catalogo(frase) for frase in frases]
    frase_sin_terminos = any(not terminos for terminos in terminos_por_frase)
    consultas = list(
        dict.fromkeys(
            termino
            for terminos_de_un_nivel in zip_longest(*terminos_por_frase)
            for termino in terminos_de_un_nivel
            if termino
        )
    )
    maximo = config.CATALOGO_BUSQUEDA_MAX_CONSULTAS
    if len(consultas) > maximo:
        logger.warning("Búsqueda en Catálogo recortada a %d de %d consultas.", maximo, len(consultas))
        return consultas[:maximo], True
    return consultas, frase_sin_terminos


def _unir_sin_duplicados(*listas_productos: list) -> list:
    """Une listas de productos conservando la primera aparición de cada id."""
    productos_por_id: dict = {}
    for productos in listas_productos:
        for producto in productos:
            identificador = producto.get("id")
            if identificador is not None:
                productos_por_id.setdefault(str(identificador), producto)
    return list(productos_por_id.values())


async def _buscar_en_catalogo(
    repositorio: ProductoRepositorio, frases: list[str], precio_max: float | None
) -> tuple[list, bool]:
    """
    Busca en paralelo cada término en Catálogo y une los resultados sin duplicados.

    A lo más `CATALOGO_BUSQUEDA_CONCURRENCIA` consultas van en vuelo a la vez, y
    todas comparten un plazo de `CATALOGO_TIMEOUT`: una consulta lenta no alarga la
    etapa 2, solo queda como fallida. Devuelve `(productos, completa)`; `completa`
    es falso si alguna consulta falló, venció o se recortó, para que el llamador
    complete con la caché en memoria.
    """
    consultas, incompleta = _planificar_consultas(frases)
    resultados: dict[str, list | None] = {}
    limitador = anyio.CapacityLimiter(config.CATALOGO_BUSQUEDA_CONCURRENCIA)

    async def _buscar(termino: str) -> None:
        """Ejecuta una búsqueda bloqueante en un hilo para no detener el bucle de eventos."""
        resultados[termino] = await anyio.to_thread.run_sync(
            repositorio.buscar_productos, termino, precio_max, limiter=limitador, abandon_on_cancel=True
        )

    with anyio.move_on_after(config.CATALOGO_TIMEOUT) as plazo_busquedas:
        async with anyio.create_task_group() as grupo_busquedas:
            for termino in consultas:
                grupo_busquedas.start_soon(_buscar, termino)
    if plazo_busquedas.cancelled_caught:
        logger.warning(
            "Plazo de búsqueda en Catálogo agotado: %d de %d consultas respondieron.", len(resultados), len(consultas)
        )

    exitosos = [resultados[termino] for termino in consultas if resultados.get(termino) is not None]
    completa = not incompleta and len(exitosos) == len(consultas)
    return _unir_sin_duplicados(*exitosos), completa


async def _obtener_candidatos(repositorio: ProductoRepositorio, filtros: dict) -> tuple[list, str, bool]:
    """
    Obtiene los productos sobre los que se aplica el filtrado determinista.

    Devuelve `(productos, estado_datos, busqueda_completa)`. Con palabras clave o
    ingredientes se busca en Catálogo completo; si esa búsqueda queda incompleta
    se completan los resultados parciales con la caché en memoria sin lanzar otra
    descarga. Los datos reutilizados de la caché se clasifican siempre como
    `memoria` o `memoria_antigua` según su edad real, y si tampoco hay resultados
    parciales ni caché se informa `sin_datos`.
    Una búsqueda completa sin resultados significa que no hay coincidencias, no
    que el catálogo esté caído.
    """
    frases = _frases_a_buscar(filtros)
    if frases:
        encontrados, busqueda_completa = await _buscar_en_catalogo(repositorio, frases, _precio_maximo(filtros))
        if busqueda_completa:
            return encontrados, ESTADO_VIGENTE, True
        logger.warning("Búsqueda dirigida en Catálogo incompleta: se completa con la caché en memoria.")
        en_cache, cache_antigua = repositorio.obtener_cache_degradada()
        combinados = _unir_sin_duplicados(encontrados, en_cache)
        if not combinados:
            return [], ESTADO_SIN_DATOS, False
        if en_cache:
            estado_datos = ESTADO_MEMORIA_ANTIGUA if cache_antigua else ESTADO_MEMORIA
        else:
            estado_datos = ESTADO_VIGENTE
        return combinados, estado_datos, False

    # Sin términos (solo categoría o precio) se mantiene la caché precargada.
    productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
    return productos, _determinar_estado_datos(repositorio), False


# ===== Respaldo de criterios de búsqueda =====


def _tiene_criterio_busqueda(filtros: dict) -> bool:
    """Indica si los filtros incluyen al menos palabras clave, ingredientes o categoría."""
    return bool(filtros.get("palabras_clave") or filtros.get("ingredientes") or filtros.get("categoria"))


def _completar_filtros_con_respaldo(filtros: dict, mensaje: str) -> dict | None:
    """
    Garantiza un criterio de producto antes de consultar Catálogo.

    Si la extracción solo trajo precio o distancia, usa como respaldo los términos útiles
    del mensaje; si no queda ninguno, devuelve None para no mostrar lo más barato de la caché.
    """
    if _tiene_criterio_busqueda(filtros):
        return filtros

    terminos_respaldo = extraer_terminos_respaldo(mensaje)
    if not terminos_respaldo:
        return None

    logger.info("Extracción sin criterio de producto; se usan términos de respaldo del mensaje: %s", terminos_respaldo)
    return {**filtros, "palabras_clave": terminos_respaldo}


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
) -> tuple[dict | None, DatosEtapa3 | None]:
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

        filtros_efectivos = _completar_filtros_con_respaldo(resultado.filtros, mensaje)
        if not filtros_efectivos:
            logger.debug("Filtros sin criterio de producto ni términos útiles en el mensaje: %s", resultado.filtros)
            return _respuesta_chat(TEXTO_SIN_FILTROS, None, [], resultado.tiempo_extraccion_ms, 0), None

        # Etapa 2: filtrado determinista sobre los candidatos de Catálogo.
        productos, estado_datos, busqueda_completa = await _obtener_candidatos(repositorio, filtros_efectivos)
        if not productos and not busqueda_completa:
            return _respuesta_chat(
                TEXTO_CATALOGO_NO_DISPONIBLE,
                filtros_efectivos,
                [],
                resultado.tiempo_extraccion_ms,
                0,
                estado_datos=estado_datos,
            ), None

        if "ingredientes" in filtros_efectivos:
            # Receta: aplicar filtros base de precio/categoria/distancia antes de vincular ingredientes.
            filtros_base = {
                clave: valor
                for clave, valor in filtros_efectivos.items()
                if clave in ("precio_max", "categoria", "distancia_max")
            }
            if filtros_base:
                productos = filtrar_productos(productos, filtros_base)

            # NOTE: limite_candidatos=None para no descartar un 4º candidato vigente antes de validar
            ingredientes_receta, ingredientes_faltantes = vincular_ingredientes_catalogo(
                productos, filtros_efectivos["ingredientes"], limite_candidatos=None
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
                if estado_datos == ESTADO_VIGENTE:
                    estado_datos = ESTADO_SIN_VALIDAR_DISPONIBILIDAD

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
                precio_max = filtros_efectivos.get("precio_max")
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
                DatosEtapa3(
                    productos=productos_receta,
                    filtros=filtros_efectivos,
                    tiempo_extraccion_ms=resultado.tiempo_extraccion_ms,
                    ingredientes_receta=ingredientes_receta_validados,
                    ingredientes_faltantes=ingredientes_faltantes,
                    estado_datos=estado_datos,
                ),
            )

        # Camino no-receta: filtrar productos con los filtros aplicados
        productos_filtrados = filtrar_productos(productos, filtros_efectivos)
        return (
            None,
            DatosEtapa3(
                productos=productos_filtrados,
                filtros=filtros_efectivos,
                tiempo_extraccion_ms=resultado.tiempo_extraccion_ms,
                estado_datos=estado_datos,
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

    # Fuera del bloque fail_after: calcular tiempo restante y ejecutar Etapa 3
    tiempo_restante = max(0.5, config.CHAT_TIMEOUT_TOTAL - (time.monotonic() - inicio))

    explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
        proveedor.generar_explicacion,
        mensaje,
        datos_etapa_3.productos,
        tiempo_limite=tiempo_restante,
    )

    return _respuesta_chat(
        _anexar_aviso_estado_datos(explicacion, datos_etapa_3.estado_datos),
        datos_etapa_3.filtros,
        datos_etapa_3.productos,
        datos_etapa_3.tiempo_extraccion_ms,
        tiempo_explicacion_ms,
        ingredientes_receta=datos_etapa_3.ingredientes_receta,
        ingredientes_faltantes=datos_etapa_3.ingredientes_faltantes,
        estado_datos=datos_etapa_3.estado_datos,
    )
