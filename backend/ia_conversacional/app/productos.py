"""
Acceso al catálogo de productos.

SUP-246 / RNF-05:
La IA nunca debe tener credenciales directas a PostgreSQL (DATABASE_URL,
drivers psycopg2/asyncpg, etc.) ni roles de base de datos.
La obtención de productos vigentes con stock se realiza mediante HTTP
interno hacia el microservicio de Catálogo (CATALOGO_INTERNAL_URL).

En caso de indisponibilidad de red, timeout o fallo en Catálogo, se aplica
degradación elegante (fallback a memoria previa o retorno de lista vacía
'sin_datos') para garantizar alta disponibilidad sin crashear FastAPI y
sin alucinar datos.
"""

from __future__ import annotations

import logging
import threading
import time
from typing import Any, List, Optional

import httpx

from app import config

logger = logging.getLogger(__name__)


class ProductoRepositorio:
    """Repositorio de productos con cliente HTTP interno hacia Catálogo y fallback resiliente."""

    def __init__(
        self,
        base_url: Optional[str] = None,
        timeout: Optional[float] = None,
        client: Optional[httpx.Client] = None,
        cache_ttl: Optional[float] = None,
        page_limit: Optional[int] = None,
        max_paginas: Optional[int] = None,
        auto_cargar: bool = True,
    ) -> None:
        url_config = base_url if base_url is not None else config.CATALOGO_INTERNAL_URL
        self._base_url = str(url_config).rstrip("/")
        self._timeout = timeout if timeout is not None else config.CATALOGO_TIMEOUT
        self._cache_ttl = cache_ttl if cache_ttl is not None else config.CATALOGO_CACHE_TTL
        self._page_limit = page_limit if page_limit is not None else config.CATALOGO_PAGE_LIMIT
        self._max_paginas = max_paginas if max_paginas is not None else config.CATALOGO_MAX_PAGINAS
        self._client = client
        self._lock = threading.Lock()
        self._productos: List[dict[str, Any]] = []
        self._ultima_actualizacion: float = 0.0
        self._fuente: str = "sin_datos"

        if auto_cargar:
            self.cargar_productos()

    @property
    def fuente(self) -> str:
        """Indica el origen de los productos: 'catalogo', 'memoria' o 'sin_datos'."""
        return self._fuente

    def _normalizar_producto(self, item: dict[str, Any]) -> dict[str, Any]:
        """Asegura consistencia de claves para filtros.py, explicacion.py y API."""
        normalizado = dict(item)

        precio_crudo = item.get("precio", 0.0)
        try:
            precio = float(precio_crudo)
        except (ValueError, TypeError):
            precio = 0.0

        precio_normal_crudo = item.get("precio_normal", precio)
        try:
            precio_normal = float(precio_normal_crudo)
        except (ValueError, TypeError):
            precio_normal = precio

        distancia_cruda = item.get("distancia_km", 0.0)
        try:
            distancia_km = float(distancia_cruda)
        except (ValueError, TypeError):
            distancia_km = 0.0

        normalizado["id"] = item.get("id")
        normalizado["nombre"] = str(item.get("nombre", ""))
        normalizado["marca"] = str(item.get("marca", ""))
        normalizado["categoria"] = str(item.get("categoria", ""))
        normalizado["supermercado"] = str(item.get("supermercado", ""))
        normalizado["precio"] = precio
        normalizado["precio_normal"] = precio_normal
        normalizado["precio_oferta"] = item.get("precio_oferta")
        normalizado["en_oferta"] = bool(item.get("en_oferta", False))
        normalizado["url_imagen"] = str(item.get("url_imagen", ""))
        normalizado["unidad"] = str(item.get("unidad", ""))
        normalizado["en_stock"] = bool(item.get("en_stock", True))
        normalizado["distancia_km"] = distancia_km

        return normalizado

    def _realizar_peticion_http(
        self,
        cliente_http: httpx.Client,
        pagina: int = 1,
        limite: Optional[int] = None,
    ) -> dict[str, Any] | list[Any]:
        """Ejecuta la llamada HTTP al endpoint de Catálogo."""
        url = f"{self._base_url}/api/v1/productos"
        parametros = {
            "en_stock": "true",
            "page": pagina,
            "limit": limite if limite is not None else self._page_limit,
        }
        respuesta = cliente_http.get(url, params=parametros)
        respuesta.raise_for_status()
        return respuesta.json()

    def _expirado(self) -> bool:
        """Indica si la caché local expiró según el TTL configurado."""
        if self._cache_ttl <= 0:
            return True
        return (time.time() - self._ultima_actualizacion) > self._cache_ttl

    def _cargar_sin_lock(self) -> List[dict[str, Any]]:
        """
        Lógica interna de carga y refresco desde Catálogo.
        Debe invocarse habiendo adquirido self._lock previamente.
        """
        productos_obtenidos: List[dict[str, Any]] = []

        if self._client is not None:
            cliente_a_usar = self._client
            es_cliente_propio = False
        else:
            cliente_a_usar = httpx.Client(timeout=self._timeout)
            es_cliente_propio = True

        try:
            try:
                primera_respuesta = self._realizar_peticion_http(
                    cliente_a_usar,
                    pagina=1,
                    limite=self._page_limit,
                )

                total_paginas = 1
                items_pagina: list[Any] = []

                if isinstance(primera_respuesta, dict):
                    items_pagina = primera_respuesta.get("data", [])
                    paginacion = primera_respuesta.get("paginacion")
                    if isinstance(paginacion, dict):
                        total_paginas = int(paginacion.get("total_paginas", 1))
                elif isinstance(primera_respuesta, list):
                    items_pagina = primera_respuesta

                for item in items_pagina:
                    if isinstance(item, dict) and item.get("en_stock", True):
                        productos_obtenidos.append(self._normalizar_producto(item))

                if total_paginas > self._max_paginas:
                    logger.warning(
                        "Catálogo truncado a %d páginas (%d totales disponibles)",
                        self._max_paginas,
                        total_paginas,
                    )

                # Obtener páginas subsecuentes si hay paginación activa
                max_paginas_a_cargar = min(total_paginas, self._max_paginas)
                for numero_pagina in range(2, max_paginas_a_cargar + 1):
                    resp_siguiente = self._realizar_peticion_http(
                        cliente_a_usar,
                        pagina=numero_pagina,
                        limite=self._page_limit,
                    )
                    if isinstance(resp_siguiente, dict):
                        items_siguiente = resp_siguiente.get("data", [])
                    elif isinstance(resp_siguiente, list):
                        items_siguiente = resp_siguiente
                    else:
                        items_siguiente = []

                    for item_siguiente in items_siguiente:
                        if isinstance(item_siguiente, dict) and item_siguiente.get("en_stock", True):
                            productos_obtenidos.append(self._normalizar_producto(item_siguiente))

            finally:
                if es_cliente_propio:
                    cliente_a_usar.close()

            if productos_obtenidos:
                self._productos = productos_obtenidos
                self._fuente = "catalogo"
                self._ultima_actualizacion = time.time()
                logger.info(
                    "Catálogo cargado exitosamente desde %s (%d productos vigentes con stock)",
                    self._base_url,
                    len(self._productos),
                )
                return self._productos

            logger.warning("Catálogo respondió sin productos disponibles.")

        except (httpx.TimeoutException, httpx.ConnectError, httpx.HTTPStatusError, httpx.RequestError) as http_error:
            logger.warning(
                "Degradación activada: no fue posible conectar con Catálogo en %s (%s).",
                self._base_url,
                http_error,
            )
        except Exception as error_inesperado:
            logger.warning(
                "Degradación activada: error inesperado al procesar productos de Catálogo (%s).",
                error_inesperado,
            )

        # Fallback resiliente: si ya teníamos productos válidos de Catálogo en memoria, conservarlos
        if self._productos and self._fuente in ("catalogo", "memoria"):
            self._fuente = "memoria"
            self._ultima_actualizacion = time.time()
            logger.info("Conservando %d productos en memoria tras fallo de refresco.", len(self._productos))
            return self._productos

        # En runtime sin catálogo real previo: devolver lista vacía sin inventar datos
        self._productos = []
        self._fuente = "sin_datos"
        self._ultima_actualizacion = time.time()
        logger.warning("Sin datos de catálogo disponibles. Modo degradado: 0 productos cargados.")
        return self._productos

    def cargar_productos(self) -> List[dict[str, Any]]:
        """
        Obtiene los productos vigentes con stock llamando al servicio interno de Catálogo.
        Maneja paginación y degradación ante fallo de red o timeout sin lanzar excepciones ni fugar sockets.
        """
        with self._lock:
            return self._cargar_sin_lock()

    def obtener_todos(self, forzar: bool = False) -> List[dict[str, Any]]:
        """
        Devuelve todos los productos vigentes con stock.
        Refresca el catálogo si el TTL expiró o si se solicita forzar recarga.
        Implementa double-checked locking para evitar llamadas concurrentes redundantes.
        """
        if not forzar and not self._expirado():
            return self._productos

        with self._lock:
            if not forzar and not self._expirado():
                return self._productos  # Otro hilo ya completó la recarga mientras este esperaba el lock
            return self._cargar_sin_lock()

    def __len__(self) -> int:
        """Consulta O(1) en memoria para evitar llamadas de red en healthchecks."""
        return len(self._productos)
