"""
Cliente del catálogo de productos con caché en memoria.

IA nunca tiene credenciales ni drivers de PostgreSQL (RNF-05, SUP-246): los
productos vigentes con stock se piden a Catálogo por HTTP interno
(`CATALOGO_INTERNAL_URL`). Si Catálogo falla, se conservan los productos
previos (`memoria`) o se informa que no hay datos (`sin_datos`), sin
inventar productos ni tumbar el servicio.
"""

from __future__ import annotations

import logging
import math
import threading
import time
from typing import Any

import httpx

from app import config
from app.domain.filtros import admitido_por_catalogo

logger = logging.getLogger(__name__)

# ===== Constantes =====

# Contrato de Catálogo: GET /api/v1/productos?en_stock=true&page=&limit=[&q=&precio_max=]
RUTA_PRODUCTOS_CATALOGO = "/api/v1/productos"
# Contrato de Catálogo: POST /internal/v1/productos/disponibilidad (SUP-312).
RUTA_DISPONIBILIDAD_CATALOGO = "/internal/v1/productos/disponibilidad"
PRIMERA_PAGINA = 1

FUENTE_CATALOGO = "catalogo"
FUENTE_MEMORIA = "memoria"
FUENTE_SIN_DATOS = "sin_datos"


# ===== Repositorio =====


class ProductoRepositorio:
    """Carga los productos de Catálogo, los normaliza y los guarda en caché con TTL."""

    def __init__(
        self,
        url_base: str = config.CATALOGO_INTERNAL_URL,
        tiempo_espera: float = config.CATALOGO_TIMEOUT,
        cliente: httpx.Client | None = None,
        token_consulta: str | None = config.CATALOGO_CONSULTA_TOKEN,
        ttl_cache: float = config.CATALOGO_CACHE_TTL,
        limite_pagina: int = config.CATALOGO_PAGE_LIMIT,
        max_paginas: int = config.CATALOGO_MAX_PAGINAS,
        antiguedad_maxima: float = config.CATALOGO_ANTIGUEDAD_MAXIMA,
    ) -> None:
        """
        Prepara el repositorio sin llamar a la red.

        La primera carga ocurre al arrancar la aplicación o en la primera
        consulta; `cliente` permite inyectar un doble HTTP en las pruebas.
        """
        self._url_base = str(url_base).rstrip("/")
        self._tiempo_espera = tiempo_espera
        self._ttl_cache = ttl_cache
        self._limite_pagina = limite_pagina
        self._max_paginas = max_paginas
        self._cliente = cliente
        # Cliente compartido por las búsquedas dirigidas: reutiliza conexiones en vez de
        # abrir uno por consulta. Se crea al primer uso y se cierra con `cerrar()`.
        self._cliente_busqueda: httpx.Client | None = None
        self._token_consulta = token_consulta
        self._candado = threading.Lock()
        self._candado_cliente_busqueda = threading.Lock()
        self._productos: list[dict[str, Any]] = []
        self._antiguedad_maxima: float = antiguedad_maxima
        self._ultima_actualizacion: float = 0.0
        self._ultima_carga_exitosa: float = 0.0
        self._fuente: str = FUENTE_SIN_DATOS

    @property
    def fuente(self) -> str:
        """Origen de los productos en memoria: 'catalogo', 'memoria' o 'sin_datos'."""
        return self._fuente

    @property
    def datos_antiguos(self) -> bool:
        """Indica si los datos en caché superaron el límite de antigüedad máxima.

        Retorna True solo cuando la fuente es 'memoria' (catálogo inalcanzable)
        y la última carga exitosa desde Catálogo superó antigüedad_maxima.
        """
        return (
            self._fuente == FUENTE_MEMORIA
            and self._ultima_carga_exitosa > 0
            and (time.time() - self._ultima_carga_exitosa) > self._antiguedad_maxima
        )

    def __len__(self) -> int:
        """Cantidad en memoria, sin red, para que /health no dependa de Catálogo."""
        return len(self._productos)

    # ===== Normalización =====

    def _normalizar_producto(self, item: dict[str, Any]) -> dict[str, Any]:
        """Garantiza las claves y tipos que usan el filtrado, la explicación y la respuesta."""
        normalizado = dict(item)
        precio = _a_decimal(item.get("precio", 0.0), 0.0)
        normalizado.update(
            {
                "id": item.get("id"),
                "nombre": str(item.get("nombre", "")),
                "marca": str(item.get("marca", "")),
                "categoria": str(item.get("categoria", "")),
                "supermercado": str(item.get("supermercado", "")),
                "precio": precio,
                "precio_normal": _a_decimal(item.get("precio_normal", precio), precio),
                "precio_oferta": item.get("precio_oferta"),
                "en_oferta": bool(item.get("en_oferta", False)),
                "url_imagen": str(item.get("url_imagen", "")),
                "unidad": str(item.get("unidad", "")),
                "en_stock": bool(item.get("en_stock", True)),
                # Catálogo no entrega distancia; queda en 0.0 (pendiente de SUP-315).
                "distancia_km": _a_decimal(item.get("distancia_km", 0.0), 0.0),
            }
        )
        return normalizado

    # ===== Acceso HTTP a Catálogo =====

    def _pedir_pagina(self, cliente_http: httpx.Client, pagina: int) -> tuple[list[Any], int]:
        """
        Pide una página de productos en stock y devuelve sus ítems y el total de páginas.

        Acepta la respuesta paginada (`data` + `paginacion`) o una lista simple.
        """
        respuesta = cliente_http.get(
            f"{self._url_base}{RUTA_PRODUCTOS_CATALOGO}",
            params={"en_stock": "true", "page": pagina, "limit": self._limite_pagina},
        )
        respuesta.raise_for_status()
        cuerpo = respuesta.json()

        if isinstance(cuerpo, list):
            return cuerpo, PRIMERA_PAGINA
        if not isinstance(cuerpo, dict):
            return [], PRIMERA_PAGINA
        paginacion = cuerpo.get("paginacion")
        total_paginas = int(paginacion.get("total_paginas", 1)) if isinstance(paginacion, dict) else PRIMERA_PAGINA
        return cuerpo.get("data", []), total_paginas

    def _productos_en_stock(self, items: list[Any]) -> list[dict[str, Any]]:
        """Normaliza los ítems válidos que siguen en stock, descartando precios inválidos o <= 0."""
        resultado = []
        for item in items:
            if not isinstance(item, dict) or not item.get("en_stock", True):
                continue
            # Descartar productos cuyo precio sea no numérico o <= 0
            precio_crudo = item.get("precio", 0.0)
            try:
                precio = float(precio_crudo)
            except (ValueError, TypeError):
                logger.debug("Producto %s descartado: precio no numérico (%s)", item.get("id"), precio_crudo)
                continue
            if not math.isfinite(precio) or precio <= 0:
                logger.debug("Producto %s descartado: precio inválido o <= 0 (%s)", item.get("id"), precio)
                continue
            resultado.append(self._normalizar_producto(item))
        return resultado

    def _descargar_catalogo(self) -> list[dict[str, Any]]:
        """Recorre las páginas hasta el máximo configurado o el agotamiento del tiempo; cierra el cliente propio aunque falle."""
        cliente_http = self._cliente or httpx.Client(timeout=self._tiempo_espera)
        inicio = time.monotonic()
        try:
            items, total_paginas = self._pedir_pagina(cliente_http, PRIMERA_PAGINA)
            productos = self._productos_en_stock(items)

            if total_paginas > self._max_paginas:
                logger.warning(
                    "Catálogo truncado a %d páginas (%d totales disponibles)",
                    self._max_paginas,
                    total_paginas,
                )
            for pagina in range(PRIMERA_PAGINA + 1, min(total_paginas, self._max_paginas) + 1):
                tiempo_transcurrido = time.monotonic() - inicio
                if tiempo_transcurrido >= self._tiempo_espera:
                    logger.warning(
                        "Tiempo de espera de Catálogo agotado (%.1fs): se corta la paginacion en la pagina %d",
                        tiempo_transcurrido,
                        pagina,
                    )
                    break
                items, _ = self._pedir_pagina(cliente_http, pagina)
                productos.extend(self._productos_en_stock(items))
            return productos
        finally:
            if self._cliente is None:
                cliente_http.close()

    def buscar_productos(
        self,
        termino: str,
        precio_max: float | None = None,
        limite: int = config.CATALOGO_BUSQUEDA_LIMITE,
    ) -> list[dict[str, Any]] | None:
        """
        Busca en todo Catálogo los productos en stock cuyo nombre, marca o categoría contienen `termino`.

        La caché solo guarda las primeras páginas (las más baratas), así que una
        palabra clave como "harina de almendras" puede no estar en ella aunque exista.
        Esta búsqueda usa `?q=` sobre el catálogo completo; el filtrado determinista
        de la etapa 2 sigue aplicándose después. Devuelve `None` si Catálogo falla,
        para que el chat use la caché en lugar de responder que no hay productos.
        """
        termino_limpio = termino.strip()
        if not admitido_por_catalogo(termino_limpio):
            # Catálogo respondería 400 (`q` corto); no es una caída, así que no hay coincidencias.
            return []
        parametros: dict[str, Any] = {"en_stock": "true", "q": termino_limpio, "page": PRIMERA_PAGINA, "limit": limite}
        if precio_max is not None and math.isfinite(precio_max) and precio_max > 0:
            parametros["precio_max"] = precio_max

        cliente_http = self._obtener_cliente_busqueda()
        try:
            respuesta = cliente_http.get(f"{self._url_base}{RUTA_PRODUCTOS_CATALOGO}", params=parametros)
            respuesta.raise_for_status()
            cuerpo = respuesta.json()
            # Misma forma que el listado paginado (`data` + `paginacion`) o una lista simple.
            items = cuerpo.get("data", []) if isinstance(cuerpo, dict) else cuerpo
            if not isinstance(items, list):
                items = []
            return self._productos_en_stock(items)
        except (httpx.HTTPError, ValueError) as error_busqueda:
            logger.warning(
                "No fue posible buscar '%s' en Catálogo (%s).", termino_limpio, _describir_error(error_busqueda)
            )
            return None

    def _obtener_cliente_busqueda(self) -> httpx.Client:
        """Devuelve el cliente inyectado o el compartido de búsquedas, creándolo una sola vez entre hilos."""
        if self._cliente is not None:
            return self._cliente
        with self._candado_cliente_busqueda:
            if self._cliente_busqueda is None:
                self._cliente_busqueda = httpx.Client(
                    timeout=self._tiempo_espera,
                    limits=httpx.Limits(max_connections=config.CATALOGO_BUSQUEDA_CONCURRENCIA),
                )
            return self._cliente_busqueda

    def cerrar(self) -> None:
        """Cierra el cliente compartido de búsquedas al apagar la aplicación."""
        with self._candado_cliente_busqueda:
            if self._cliente_busqueda is not None:
                self._cliente_busqueda.close()
                self._cliente_busqueda = None

    def consultar_disponibilidad(self, producto_ids: list[str]) -> dict[str, dict[str, Any]] | None:
        """Consulta en lote la disponibilidad de candidatos ante Catálogo (SUP-312)."""
        if not self._token_consulta or not producto_ids:
            return None
        cliente_http = self._cliente or httpx.Client(timeout=self._tiempo_espera)
        try:
            respuesta = cliente_http.post(
                f"{self._url_base}{RUTA_DISPONIBILIDAD_CATALOGO}",
                json={"producto_ids": producto_ids},
                headers={"Authorization": f"Bearer {self._token_consulta}"},
            )
            respuesta.raise_for_status()
            return {
                str(item["producto_id"]): item
                for item in respuesta.json().get("productos", [])
                if isinstance(item, dict) and item.get("producto_id")
            }
        except (httpx.HTTPError, ValueError) as error_consulta:
            logger.warning("No fue posible consultar disponibilidad en Catálogo (%s).", error_consulta)
            return None
        finally:
            if self._cliente is None:
                cliente_http.close()

    # ===== Carga con degradación =====

    def _cargar_sin_candado(self) -> list[dict[str, Any]]:
        """Refresca desde Catálogo y aplica la degradación si falla. Requiere sostener el candado."""
        try:
            productos = self._descargar_catalogo()
            if productos:
                self._productos = productos
                self._fuente = FUENTE_CATALOGO
                self._ultima_actualizacion = time.time()
                self._ultima_carga_exitosa = self._ultima_actualizacion
                logger.info(
                    "Catálogo cargado exitosamente desde %s (%d productos vigentes con stock)",
                    self._url_base,
                    len(productos),
                )
                return self._productos
            logger.warning("Catálogo respondió sin productos disponibles.")
        except httpx.HTTPError as error_http:
            logger.warning(
                "Degradación activada: no fue posible conectar con Catálogo en %s (%s).",
                self._url_base,
                error_http,
            )
        except Exception as error_inesperado:
            logger.warning(
                "Degradación activada: error inesperado al procesar productos de Catálogo (%s).",
                error_inesperado,
            )

        # Se renueva la marca de tiempo también al degradar, para no reintentar en ráfaga.
        # NOTA: _ultima_carga_exitosa NO se sobrescribe al degradar a FUENTE_MEMORIA,
        #       para que datos_antiguos pueda detectar cuándo la última carga válida
        #       superó el límite de antigüedad.
        self._ultima_actualizacion = time.time()
        if self._productos:
            self._fuente = FUENTE_MEMORIA
            logger.info("Conservando %d productos en memoria tras fallo de refresco.", len(self._productos))
            return self._productos

        self._fuente = FUENTE_SIN_DATOS
        logger.warning("Sin datos de catálogo disponibles. Modo degradado: 0 productos cargados.")
        return self._productos

    def _expirado(self) -> bool:
        """Indica si la caché venció; con TTL 0 siempre se consulta Catálogo."""
        if self._ttl_cache <= 0:
            return True
        return (time.time() - self._ultima_actualizacion) > self._ttl_cache

    def cargar_productos(self) -> list[dict[str, Any]]:
        """Fuerza una recarga desde Catálogo; nunca lanza excepciones."""
        with self._candado:
            return self._cargar_sin_candado()

    def productos_en_memoria(self) -> list[dict[str, Any]]:
        """Devuelve la caché actual sin ir a la red, aunque el TTL haya vencido."""
        return self._productos

    def obtener_todos(self) -> list[dict[str, Any]]:
        """
        Devuelve los productos en caché y los refresca si el TTL venció.

        Revisa el vencimiento antes y después de tomar el candado, para que
        peticiones concurrentes no repitan la misma descarga.
        """
        if not self._expirado():
            return self._productos
        with self._candado:
            if not self._expirado():
                return self._productos
            return self._cargar_sin_candado()


# ===== Utilidades =====


def _describir_error(error: Exception) -> str:
    """Resume un error HTTP con su tipo y código, sin la URL interna que incluye su texto."""
    if isinstance(error, httpx.HTTPStatusError):
        # getattr: describir el error nunca debe lanzar otra excepción dentro del manejo del fallo.
        return f"{type(error).__name__} {getattr(error.response, 'status_code', 'desconocido')}"
    return type(error).__name__


def _a_decimal(valor: Any, valor_defecto: float) -> float:
    """Convierte a float o devuelve el valor por defecto si Catálogo envía algo no numérico."""
    try:
        return float(valor)
    except (ValueError, TypeError):
        return valor_defecto
