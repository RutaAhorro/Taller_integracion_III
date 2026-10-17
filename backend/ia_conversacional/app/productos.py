"""
Acceso al catálogo de productos.

SUP-246 / RNF-05:
La IA nunca debe tener credenciales directas a PostgreSQL (DATABASE_URL,
drivers psycopg2/asyncpg, etc.) ni roles de base de datos.
La obtención de productos vigentes con stock se realiza mediante HTTP
interno hacia el microservicio de Catálogo (CATALOGO_INTERNAL_URL).

En caso de indisponibilidad de red, timeout o fallo en Catálogo, se aplica
degradación elegante (fallback a memoria previa o a productos.json) para
garantizar alta disponibilidad sin crashear FastAPI.
"""

from __future__ import annotations

import json
import logging
from pathlib import Path
import time
from typing import Any, List, Optional

import httpx

from app import config

logger = logging.getLogger(__name__)


class ProductoRepositorio:
    """Repositorio de productos con cliente HTTP interno hacia Catálogo y fallback resiliente."""

    def __init__(
        self,
        ruta_fallback: Optional[Path | str] = None,
        base_url: Optional[str] = None,
        timeout: float = 5.0,
        client: Optional[httpx.Client] = None,
        cache_ttl: float = 60.0,
    ) -> None:
        self._ruta_fallback = Path(ruta_fallback) if ruta_fallback else config.PRODUCTOS_PATH
        raw_url = base_url if base_url is not None else getattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080")
        self._base_url = str(raw_url).rstrip("/")
        self._timeout = timeout
        self._client = client
        self._cache_ttl = cache_ttl
        self._productos: List[dict[str, Any]] = []
        self._ultima_actualizacion: float = 0.0

        # Carga inicial al inicializar el repositorio
        self.cargar_productos()

    def _normalizar_producto(self, item: dict[str, Any]) -> dict[str, Any]:
        """Asegura consistencia de claves para filtros.py, explicacion.py y API."""
        normalizado = dict(item)

        precio_raw = item.get("precio", 0.0)
        try:
            precio = float(precio_raw)
        except (ValueError, TypeError):
            precio = 0.0

        precio_normal_raw = item.get("precio_normal", precio)
        try:
            precio_normal = float(precio_normal_raw)
        except (ValueError, TypeError):
            precio_normal = precio

        distancia_raw = item.get("distancia_km", 0.0)
        try:
            distancia_km = float(distancia_raw)
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

    def _cargar_fallback(self) -> List[dict[str, Any]]:
        """Carga productos estáticos desde el archivo JSON de fallback."""
        if not self._ruta_fallback.exists():
            logger.warning("Archivo de fallback no encontrado en %s", self._ruta_fallback)
            return []

        try:
            with open(self._ruta_fallback, "r", encoding="utf-8") as f:
                datos = json.load(f)
                if isinstance(datos, list):
                    return [self._normalizar_producto(p) for p in datos]
                logger.warning("Formato inesperado en archivo de fallback: no es lista.")
                return []
        except Exception as exc:
            logger.error("Error al leer archivo de fallback %s: %s", self._ruta_fallback, exc)
            return []

    def _realizar_peticion_http(self, client: httpx.Client, page: int = 1, limit: int = 100) -> dict[str, Any] | list[Any]:
        """Ejecuta la llamada HTTP al endpoint de Catálogo."""
        url = f"{self._base_url}/api/v1/productos"
        params = {
            "en_stock": "true",
            "page": page,
            "limit": limit,
        }
        response = client.get(url, params=params)
        response.raise_for_status()
        return response.json()

    def cargar_productos(self) -> List[dict[str, Any]]:
        """
        Obtiene los productos vigentes con stock llamando al servicio interno de Catálogo.
        Maneja paginación y degradación ante fallo de red o timeout sin lanzar excepciones ni fugar sockets.
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
                primera_respuesta = self._realizar_peticion_http(cliente_a_usar, page=1)

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

                if total_paginas > 10:
                    logger.warning("Catálogo truncado a 10 páginas (%d totales disponibles)", total_paginas)

                # Obtener páginas subsecuentes si hay paginación activa (límite de seguridad: 10 páginas)
                max_paginas = min(total_paginas, 10)
                for pag in range(2, max_paginas + 1):
                    resp_siguiente = self._realizar_peticion_http(cliente_a_usar, page=pag)
                    if isinstance(resp_siguiente, dict):
                        items_sig = resp_siguiente.get("data", [])
                    elif isinstance(resp_siguiente, list):
                        items_sig = resp_siguiente
                    else:
                        items_sig = []

                    for item in items_sig:
                        if isinstance(item, dict) and item.get("en_stock", True):
                            productos_obtenidos.append(self._normalizar_producto(item))

            finally:
                if es_cliente_propio:
                    cliente_a_usar.close()

            if productos_obtenidos:
                self._productos = productos_obtenidos
                self._ultima_actualizacion = time.time()
                logger.info(
                    "Catálogo cargado exitosamente desde %s (%d productos vigentes con stock)",
                    self._base_url,
                    len(self._productos),
                )
                return self._productos

            logger.warning("Catálogo respondió sin productos disponibles. Manteniendo estado previo o fallback.")

        except (httpx.TimeoutException, httpx.ConnectError, httpx.HTTPStatusError, httpx.RequestError) as http_err:
            logger.warning(
                "Degradación activada: no fue posible conectar con Catálogo en %s (%s).",
                self._base_url,
                http_err,
            )
        except Exception as exc:
            logger.warning(
                "Degradación activada: error inesperado al procesar productos de Catálogo (%s).",
                exc,
            )

        # Fallback resiliente: si ya teníamos productos válidos, conservarlos y resetear TTL
        if self._productos:
            self._ultima_actualizacion = time.time()
            logger.info("Conservando %d productos en memoria tras fallo de refresco.", len(self._productos))
            return self._productos

        # Si no había productos en memoria, recurrir a productos.json
        logger.info("Cargando productos de fallback desde %s.", self._ruta_fallback)
        self._productos = self._cargar_fallback()
        self._ultima_actualizacion = time.time()
        return self._productos

    def obtener_todos(self, forzar: bool = False) -> List[dict[str, Any]]:
        """
        Devuelve todos los productos vigentes con stock.
        Refresca el catálogo si el TTL expiró o si se solicita forzar recarga.
        """
        ahora = time.time()
        expirado = self._cache_ttl > 0 and (ahora - self._ultima_actualizacion) > self._cache_ttl

        if forzar or expirado or not self._productos:
            return self.cargar_productos()

        return self._productos

    def __len__(self) -> int:
        """Consulta O(1) en memoria para evitar llamadas de red en healthchecks."""
        return len(self._productos)
