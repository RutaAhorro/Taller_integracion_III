"""
Servicio de chat: orquesta el pipeline de tres etapas.

Extracción por LLM → búsqueda directa en Catálogo (`?q=`, primera página) y
filtrado determinista → explicación por LLM solo con los productos validados.
No depende de una copia parcial del catálogo en memoria ni conoce HTTP: el
router traduce `AsistenteNoDisponible` a 503.
"""

import logging
import math
import re
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass

import anyio

from app import config
from app.domain.filtros import (
    dividir_frases_compuestas,
    extraer_terminos_respaldo,
    filtrar_productos,
    formatear_precio_clp,
    terminos_busqueda_catalogo,
    vincular_ingredientes_catalogo,
)
from app.domain.proveedores import MILISEGUNDOS_POR_SEGUNDO, ProveedorIA
from app.repositories.catalogo import FUENTE_SIN_DATOS, ProductoRepositorio
from app.services.fallback import formatear_error_seguro

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

# Fracción máxima del plazo total que se reserva para la explicación cuando el plazo total es acotado.
FRACCION_RESERVA_MAXIMA_EXPLICACION = 0.5
PLAZO_MINIMO_ETAPAS_1_Y_2 = 0.001
MAXIMO_TERMINOS_EN_RESPALDO = 3

# Patrones para limpiar Markdown crudo, normalizar precios CLP y detectar oraciones completas.
PATRON_ENCABEZADO_MARKDOWN = re.compile(r"(?m)^\s*#{1,6}\s+")
PATRON_VINETA_ASTERISCO = re.compile(r"(?m)^(\s*)\*\s+")
PATRON_MARCAS_MARKDOWN = re.compile(r"\*\*|__|~~|`")
PATRON_CURSIVA_ASTERISCO = re.compile(r"(?<!\d)\*(?!\d)")
PATRON_ESPACIOS_HORIZONTALES = re.compile(r"[ \t]+")
PATRON_PRECIO_CON_ESPACIO = re.compile(r"\$\s*(\d{1,3}(?:\s\d{3})+)\b")
PATRON_PRECIO_SIN_FORMATO = re.compile(r"\$\s*(\d{4,})(?:\.0+)?\b")
# Solo 1 o 2 ceros decimales (p. ej. '$890.0'); nunca 3 ceros porque '.000' es separador de miles en CLP.
PATRON_PRECIO_DECIMAL_CERO = re.compile(r"\$\s*(\d{1,3}(?:\.\d{3})*)\.0{1,2}\b")
PATRON_FIN_ORACION = re.compile(r"[.!?][\"'”»)\]]*(?=\s|$)")
PATRON_MARCADOR_LISTA_NUMERADA = re.compile(r"(?:^|\n)\s*\d+$")
PATRON_PALABRA_FINAL = re.compile(r"([A-Za-zÁÉÍÓÚáéíóúÑñ]+)$")
ABREVIATURAS_NO_CORTE = frozenset(
    {"av", "sr", "sra", "srta", "dr", "dra", "prof", "nro", "num", "ej", "etc", "aprox", "min", "max"}
)

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
    termino_buscado: str | None = None


@dataclass(frozen=True)
class PresupuestoPlazos:
    """Reparto explícito del plazo total entre extracción+búsqueda, Catálogo y explicación."""

    plazo_etapas_1_y_2: float
    plazo_catalogo_max: float
    reserva_explicacion: float


def calcular_presupuesto_plazos() -> PresupuestoPlazos:
    """
    Reparte `CHAT_TIMEOUT_TOTAL` para que Catálogo no agote el tiempo de la explicación.

    - La explicación conserva un mínimo garantizado (`CHAT_EXPLICACION_PLAZO_MINIMO`, acotado
      a la mitad del plazo total si este se reduce en pruebas o configuración).
    - Catálogo no puede superar `CATALOGO_TIMEOUT` ni `CHAT_CATALOGO_FRACCION_PLAZO` del total.
    """
    plazo_total = config.CHAT_TIMEOUT_TOTAL
    reserva_explicacion = min(
        config.CHAT_EXPLICACION_PLAZO_MINIMO,
        plazo_total * FRACCION_RESERVA_MAXIMA_EXPLICACION,
    )
    plazo_etapas_1_y_2 = max(PLAZO_MINIMO_ETAPAS_1_Y_2, plazo_total - reserva_explicacion)
    plazo_catalogo_max = min(config.CATALOGO_TIMEOUT, plazo_total * config.CHAT_CATALOGO_FRACCION_PLAZO)
    return PresupuestoPlazos(
        plazo_etapas_1_y_2=plazo_etapas_1_y_2,
        plazo_catalogo_max=plazo_catalogo_max,
        reserva_explicacion=reserva_explicacion,
    )


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


def _tiene_evidencia_truncado(texto: str) -> bool:
    """Indica si el texto conserva delimitadores sin cerrar (`**`, backticks o paréntesis)."""
    if texto.count("**") % 2 != 0 or texto.count("`") % 2 != 0:
        return True
    profundidad_parentesis = 0
    for caracter in texto:
        if caracter == "(":
            profundidad_parentesis += 1
        elif caracter == ")" and profundidad_parentesis > 0:
            profundidad_parentesis -= 1
    return profundidad_parentesis > 0


def _buscar_ultimo_fin_oracion(texto: str) -> int | None:
    """Devuelve el índice de cierre de la última oración válida, ignorando listas numeradas y abreviaturas."""
    posicion_valida = None
    for coincidencia in PATRON_FIN_ORACION.finditer(texto):
        signo = texto[coincidencia.start()]
        if signo == ".":
            prefijo = texto[: coincidencia.start()]
            if PATRON_MARCADOR_LISTA_NUMERADA.search(prefijo):
                continue
            palabra_previa = PATRON_PALABRA_FINAL.search(prefijo)
            if palabra_previa and palabra_previa.group(1).lower() in ABREVIATURAS_NO_CORTE:
                continue
        posicion_valida = coincidencia.end()
    return posicion_valida


def _normalizar_precios_en_texto(texto: str) -> str:
    """Corrige cifras mal formateadas por el modelo ('$2 000' -> '$2.000', '$10600.0' -> '$10.600')."""
    texto_corregido = PATRON_PRECIO_CON_ESPACIO.sub(
        lambda coincidencia: "$" + coincidencia.group(1).replace(" ", "."),
        texto,
    )
    texto_corregido = PATRON_PRECIO_SIN_FORMATO.sub(
        lambda coincidencia: formatear_precio_clp(int(coincidencia.group(1))),
        texto_corregido,
    )
    return PATRON_PRECIO_DECIMAL_CERO.sub(r"$\1", texto_corregido)


def limpiar_explicacion(texto: str) -> str:
    """
    Quita formato Markdown crudo, normaliza precios CLP y recorta solo respuestas con evidencia de truncado.

    Solo recorta cuando hay `**`, backticks o paréntesis sin cerrar (ej. `**Yogh`),
    respetando explicaciones completas sin punto final (como listas de opciones) y
    evitando cortar en marcadores de lista numerada (`1.`, `2.`) o abreviaturas (`Av.`, `Sr.`).
    """
    texto_original = texto.strip()
    esta_truncado = _tiene_evidencia_truncado(texto_original)

    texto_limpio = PATRON_ENCABEZADO_MARKDOWN.sub("", texto_original)
    texto_limpio = PATRON_VINETA_ASTERISCO.sub(r"\1- ", texto_limpio)
    texto_limpio = PATRON_MARCAS_MARKDOWN.sub("", texto_limpio)
    texto_limpio = PATRON_CURSIVA_ASTERISCO.sub("", texto_limpio)
    texto_limpio = _normalizar_precios_en_texto(texto_limpio)
    texto_limpio = PATRON_ESPACIOS_HORIZONTALES.sub(" ", texto_limpio).strip()
    if not texto_limpio:
        return ""

    if not esta_truncado:
        return texto_limpio

    posicion_corte = _buscar_ultimo_fin_oracion(texto_limpio)
    if posicion_corte is not None:
        return texto_limpio[:posicion_corte].strip()

    return texto_limpio


def construir_explicacion_respaldo(productos_filtrados: list, termino_buscado: str | None = None) -> str:
    """
    Construye una frase fija corta con el término buscado y el precio mínimo en pesos chilenos.

    Se usa cuando el modelo falla o agota su plazo en etapa 3, para responder siempre con los
    productos reales ya encontrados en vez de devolver un error genérico.
    """
    if not termino_buscado or not termino_buscado.strip():
        return TEXTO_EXPLICACION_RESPALDO
    etiqueta = termino_buscado.strip()
    precios_validos = [
        float(producto["precio"])
        for producto in productos_filtrados
        if isinstance(producto, dict)
        and isinstance(producto.get("precio"), (int, float))
        and not isinstance(producto.get("precio"), bool)
        and math.isfinite(float(producto["precio"]))
        and float(producto["precio"]) > 0
    ]
    if precios_validos:
        precio_desde = formatear_precio_clp(min(precios_validos))
        return f"Estas son las opciones que encontré para «{etiqueta}», desde {precio_desde}."
    return f"Estas son las opciones que encontré para «{etiqueta}»."


async def generar_explicacion_con_tiempo(
    generador: GeneradorExplicacion,
    mensaje_usuario: str,
    productos_filtrados: list,
    tiempo_limite: float | None = None,
    termino_buscado: str | None = None,
) -> tuple[str, int]:
    """
    Ejecuta la redacción con un plazo opcional, mide su duración en milisegundos
    y nunca propaga errores.

    Los productos filtrados ya son válidos: la explicación es un complemento,
    así que un fallo de red o de cuota (o plazo agotado) se reemplaza por una
    frase fija corta basada en el término buscado (o `TEXTO_EXPLICACION_RESPALDO`).
    """
    if not productos_filtrados:
        return TEXTO_SIN_PRODUCTOS, 0

    inicio = time.perf_counter()
    explicacion = None
    try:
        if tiempo_limite is not None:
            with anyio.move_on_after(tiempo_limite) as plazo_explicacion:
                explicacion = await generador(mensaje_usuario, productos_filtrados)
            if plazo_explicacion.cancelled_caught:
                logger.warning("Falló la explicación: plazo agotado tras %.2fs.", tiempo_limite)
        else:
            explicacion = await generador(mensaje_usuario, productos_filtrados)
    except Exception as error_explicacion:
        logger.warning("Falló la explicación: %s", formatear_error_seguro(error_explicacion))
        explicacion = None
    tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)

    explicacion_limpia = limpiar_explicacion(explicacion) if explicacion else ""
    respaldo = construir_explicacion_respaldo(productos_filtrados, termino_buscado)
    return explicacion_limpia or respaldo, tiempo_ms


# ===== Candidatos de la etapa 2 =====


def _frases_a_buscar(filtros: dict) -> list[str]:
    """
    Devuelve los términos independientes a buscar en Catálogo.

    Prioriza ingredientes de receta, luego palabras clave y finalmente categoría;
    separa frases compuestas por 'o', 'y' o comas para buscar cada término aparte.
    """
    if "ingredientes" in filtros:
        return [ingrediente["nombre"] for ingrediente in filtros["ingredientes"]]
    if filtros.get("palabras_clave"):
        frases = []
        for palabra in filtros["palabras_clave"]:
            frases.extend(dividir_frases_compuestas(palabra))
        return list(dict.fromkeys(frases))
    categoria = filtros.get("categoria")
    if isinstance(categoria, str) and categoria.strip():
        return dividir_frases_compuestas(categoria)
    return []


def _etiqueta_termino_buscado(filtros: dict) -> str | None:
    """Resume los términos buscados para armar la frase fija de respaldo si falla la explicación."""
    frases = _frases_a_buscar(filtros)
    if not frases:
        return None
    return ", ".join(frases[:MAXIMO_TERMINOS_EN_RESPALDO])


def _precio_maximo(filtros: dict) -> float | None:
    """Devuelve el precio máximo extraído si es un número positivo; si no, None."""
    precio_max = filtros.get("precio_max")
    if isinstance(precio_max, (int, float)) and not isinstance(precio_max, bool) and precio_max > 0:
        return float(precio_max)
    return None


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
    repositorio: ProductoRepositorio,
    frases: list[str],
    precio_max: float | None,
    plazo_maximo: float,
) -> tuple[list, bool]:
    """
    Busca cada frase por separado en Catálogo (`?q=`, primera página) y combina resultados sin duplicados.

    Para cada frase se prueba primero el término tal cual y, solo si Catálogo devuelve 0
    resultados, se prueban la forma singular y variantes. Todas las frases corren en paralelo
    respetando `CATALOGO_BUSQUEDA_CONCURRENCIA` y comparten `plazo_maximo`.
    Devuelve `(productos, hubo_error_red)`: `hubo_error_red` es verdadero si alguna consulta
    falló por red/HTTP o se agotó el plazo.
    """
    maximo = config.CATALOGO_BUSQUEDA_MAX_CONSULTAS
    if len(frases) > maximo:
        logger.warning("Búsqueda en Catálogo recortada a %d de %d términos.", maximo, len(frases))
        frases = frases[:maximo]

    resultados: dict[str, list | None] = {}
    limitador = anyio.CapacityLimiter(config.CATALOGO_BUSQUEDA_CONCURRENCIA)

    async def _buscar_frase(frase: str) -> None:
        """Prueba la frase tal cual y, si no hay resultados, su forma singular y variantes."""
        variantes = terminos_busqueda_catalogo(frase)
        if not variantes:
            resultados[frase] = []
            return
        for termino in variantes:
            encontrados = await anyio.to_thread.run_sync(
                repositorio.buscar_productos, termino, precio_max, limiter=limitador, abandon_on_cancel=True
            )
            if encontrados is None:
                resultados[frase] = None
                return
            if encontrados:
                resultados[frase] = encontrados
                return
        resultados[frase] = []

    with anyio.move_on_after(plazo_maximo) as plazo_busquedas:
        async with anyio.create_task_group() as grupo_busquedas:
            for frase in frases:
                grupo_busquedas.start_soon(_buscar_frase, frase)
    if plazo_busquedas.cancelled_caught:
        logger.warning(
            "Plazo de búsqueda en Catálogo agotado (%.2fs): %d de %d términos respondieron.",
            plazo_maximo,
            len(resultados),
            len(frases),
        )

    exitosos = [resultados[frase] for frase in frases if resultados.get(frase) is not None]
    hubo_error_red = plazo_busquedas.cancelled_caught or len(exitosos) < len(frases)
    return _unir_sin_duplicados(*exitosos), hubo_error_red


async def _obtener_candidatos(
    repositorio: ProductoRepositorio, filtros: dict, plazo_catalogo: float
) -> tuple[list, str, bool]:
    """
    Obtiene los candidatos directamente desde Catálogo por `?q=`, sin copia parcial en memoria.

    Devuelve `(productos, estado_datos, catalogo_disponible)`:
    - Si Catálogo responde (incluso con 0 resultados), `estado_datos` es `vigente` y `catalogo_disponible` es True.
    - Si Catálogo falla por red o agota su plazo y no hubo resultados de ningún término,
      informa `sin_datos` y `catalogo_disponible = False` para responder que Catálogo no respondió.
    """
    frases = _frases_a_buscar(filtros)
    if not frases:
        return [], ESTADO_VIGENTE, True

    encontrados, hubo_error_red = await _buscar_en_catalogo(
        repositorio, frases, _precio_maximo(filtros), plazo_maximo=plazo_catalogo
    )
    if encontrados:
        return encontrados, ESTADO_VIGENTE, True
    if hubo_error_red:
        return [], ESTADO_SIN_DATOS, False
    return [], ESTADO_VIGENTE, True


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
    proveedor: ProveedorIA,
    repositorio: ProductoRepositorio,
    mensaje: str,
    presupuesto: PresupuestoPlazos,
) -> tuple[dict | None, DatosEtapa3 | None]:
    """
    Ejecuta las etapas 1 y 2 dentro de su parte reservada del plazo total.

    Devuelve un par `(respuesta, datos_etapa_3)`: con la respuesta ya armada cuando el
    proveedor no encuentra filtros o Catálogo no responde; o con los datos de la etapa 3
    cuando ya hay candidatos validados para explicar.
    """
    limite_etapas_1_y_2 = time.monotonic() + presupuesto.plazo_etapas_1_y_2

    with anyio.fail_after(presupuesto.plazo_etapas_1_y_2):
        # Etapa 1: extracción de filtros por function calling.
        resultado = await proveedor.extraer_filtros(mensaje)

        if resultado.error:
            logger.error("Falló la extracción: %s", resultado.error)
            raise AsistenteNoDisponible(resultado.error)

        filtros_base_extraccion = resultado.filtros if isinstance(resultado.filtros, dict) else {}
        filtros_efectivos = _completar_filtros_con_respaldo(filtros_base_extraccion, mensaje)
        if not filtros_efectivos:
            logger.debug(
                "Sin filtros ni términos útiles en el mensaje (texto libre omitido): %s", resultado.texto_libre
            )
            return _respuesta_chat(TEXTO_SIN_FILTROS, None, [], resultado.tiempo_extraccion_ms, 0), None

        # Etapa 2: búsqueda directa en Catálogo acotada a su fracción del plazo y al tiempo restante.
        plazo_catalogo = max(0.0, min(presupuesto.plazo_catalogo_max, limite_etapas_1_y_2 - time.monotonic()))
        productos, estado_datos, catalogo_disponible = await _obtener_candidatos(
            repositorio, filtros_efectivos, plazo_catalogo=plazo_catalogo
        )
        if not productos and not catalogo_disponible:
            return _respuesta_chat(
                TEXTO_CATALOGO_NO_DISPONIBLE,
                filtros_efectivos,
                [],
                resultado.tiempo_extraccion_ms,
                0,
                estado_datos=estado_datos,
            ), None

        termino_buscado = _etiqueta_termino_buscado(filtros_efectivos)

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

            # Si disponibilidad es None, loguear warning y continuar con los datos obtenidos de Catálogo
            if disponibilidad is None:
                logger.warning("Disponibilidad no validada para receta: continuando sin validación en lote.")
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
                    productos=productos_receta[: config.EXPLICACION_MAX_PRODUCTOS],
                    filtros=filtros_efectivos,
                    tiempo_extraccion_ms=resultado.tiempo_extraccion_ms,
                    ingredientes_receta=ingredientes_receta_validados,
                    ingredientes_faltantes=ingredientes_faltantes,
                    estado_datos=estado_datos,
                    termino_buscado=termino_buscado,
                ),
            )

        # Camino no-receta: filtrar deterministamente y respetar el límite de tarjetas.
        productos_filtrados = filtrar_productos(productos, filtros_efectivos)[: config.EXPLICACION_MAX_PRODUCTOS]
        return (
            None,
            DatosEtapa3(
                productos=productos_filtrados,
                filtros=filtros_efectivos,
                tiempo_extraccion_ms=resultado.tiempo_extraccion_ms,
                estado_datos=estado_datos,
                termino_buscado=termino_buscado,
            ),
        )


async def procesar_mensaje(proveedor: ProveedorIA, repositorio: ProductoRepositorio, mensaje: str) -> dict:
    """
    Ejecuta las etapas 1 y 2 con plazo acotado y la etapa 3 con mínimo garantizado y respaldo.

    - `_ejecutar_etapas_1_y_2` corre dentro de `presupuesto.plazo_etapas_1_y_2`.
    - La búsqueda en Catálogo no puede gastar más de `presupuesto.plazo_catalogo_max`.
    - Si devuelve datos de etapa 3, la explicación corre FUERA del `fail_after` con al menos
      `presupuesto.reserva_explicacion` segundos; si falla o vence, responde igualmente con
      los productos reales y una frase fija corta basada en el término buscado.
    """
    inicio = time.monotonic()
    presupuesto = calcular_presupuesto_plazos()

    try:
        respuesta, datos_etapa_3 = await _ejecutar_etapas_1_y_2(proveedor, repositorio, mensaje, presupuesto)
    except TimeoutError as error_tiempo:
        logger.error("Falló la extracción: plazo total agotado.")
        raise AsistenteNoDisponible("Tiempo total agotado en el asistente") from error_tiempo

    if respuesta is not None:
        return respuesta

    # Fuera del bloque fail_after: garantizar el mínimo de plazo reservado para la explicación.
    tiempo_restante = config.CHAT_TIMEOUT_TOTAL - (time.monotonic() - inicio)
    plazo_explicacion = max(presupuesto.reserva_explicacion, tiempo_restante)

    explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
        proveedor.generar_explicacion,
        mensaje,
        datos_etapa_3.productos,
        tiempo_limite=plazo_explicacion,
        termino_buscado=datos_etapa_3.termino_buscado,
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
