"""
Pruebas de la búsqueda dirigida en Catálogo (`?q=`) para el chat.

La caché en memoria solo cubre las primeras páginas del catálogo (las más
baratas), así que palabras clave como "harina de almendras" no aparecían.
Verifica:
1. `ProductoRepositorio.buscar_productos` envía `q`, `en_stock`, `limit` y `precio_max`,
   normaliza la respuesta y devuelve None ante fallos HTTP (sin inventar datos).
2. `terminos_busqueda_catalogo` arma las consultas literales que entiende Catálogo.
3. El pipeline usa la búsqueda para palabras clave e ingredientes, mantiene el
   filtrado determinista y degrada a la caché si la búsqueda falla.
"""

from unittest.mock import MagicMock

import anyio
import httpx

from app import config
from app.domain.filtros import terminos_busqueda_catalogo
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.repositories.catalogo import FUENTE_CATALOGO, FUENTE_MEMORIA, ProductoRepositorio
from app.services.chat import (
    ESTADO_MEMORIA,
    ESTADO_VIGENTE,
    TEXTO_CATALOGO_NO_DISPONIBLE,
    TEXTO_SIN_PRODUCTOS,
    procesar_mensaje,
)

URL_CATALOGO = "http://catalogo:8080"

# ===== Datos de prueba =====


def _producto(identificador: str, nombre: str, precio: float, categoria: str = "despensa") -> dict:
    """Arma un producto con la forma que entrega Catálogo."""
    return {
        "id": identificador,
        "nombre": nombre,
        "marca": "Generica",
        "categoria": categoria,
        "supermercado": "Jumbo",
        "precio": precio,
        "precio_normal": precio,
        "precio_oferta": None,
        "en_oferta": False,
        "url_imagen": "",
        "unidad": "1 un",
        "en_stock": True,
    }


HARINA_ALMENDRAS = _producto("prod-harina-almendras", "Harina de Almendras 500 g", 5990.0)
HARINA_TRIGO = _producto("prod-harina-trigo", "Harina de Trigo 1 kg", 1290.0)
LECHE_ENTERA = _producto("prod-leche", "Leche Entera 1 L", 1190.0, "lacteos")
YOGUR_BARATO = _producto("prod-yogur", "Yoghurt Vainilla 120 g", 250.0, "lacteos")


def _respuesta_http(cuerpo: object) -> MagicMock:
    """Doble de `httpx.Response` con el cuerpo JSON indicado."""
    respuesta = MagicMock(spec=httpx.Response)
    respuesta.json.return_value = cuerpo
    respuesta.raise_for_status.return_value = None
    return respuesta


# ===== Repositorio: buscar_productos =====


def test_buscar_productos_envia_q_en_stock_limite_y_precio_max():
    """La búsqueda pide a Catálogo el término exacto, solo en stock y con el precio máximo."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": [HARINA_ALMENDRAS], "paginacion": {"total_paginas": 1}})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    productos = repositorio.buscar_productos("  harina de almendras ", precio_max=8000, limite=50)

    cliente_http.get.assert_called_once_with(
        f"{URL_CATALOGO}/api/v1/productos",
        params={"en_stock": "true", "q": "harina de almendras", "page": 1, "limit": 50, "precio_max": 8000},
    )
    assert [producto["id"] for producto in productos] == ["prod-harina-almendras"]
    assert productos[0]["precio"] == 5990.0
    assert "distancia_km" in productos[0]


def test_buscar_productos_sin_precio_max_no_envia_el_parametro():
    """Sin precio máximo (o con uno inválido) no se envía `precio_max` a Catálogo."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": []})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("leche", precio_max=0) == []

    parametros = cliente_http.get.call_args.kwargs["params"]
    assert "precio_max" not in parametros
    assert parametros["limit"] == config.CATALOGO_BUSQUEDA_LIMITE


def test_buscar_productos_descarta_precios_invalidos_y_sin_stock():
    """Se aplican las mismas reglas que la carga completa: precio > 0 y en stock."""
    sin_stock = dict(LECHE_ENTERA, id="prod-sin-stock", en_stock=False)
    precio_cero = dict(LECHE_ENTERA, id="prod-precio-cero", precio=0)
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": [LECHE_ENTERA, sin_stock, precio_cero]})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    productos = repositorio.buscar_productos("leche")

    assert [producto["id"] for producto in productos] == ["prod-leche"]


def test_buscar_productos_termino_vacio_no_llama_a_catalogo():
    """Un término vacío no genera tráfico y devuelve una lista vacía."""
    cliente_http = MagicMock(spec=httpx.Client)
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("   ") == []
    cliente_http.get.assert_not_called()


def test_buscar_productos_error_http_devuelve_none():
    """Ante un 5xx la búsqueda devuelve None (degradación), nunca productos inventados."""
    respuesta = MagicMock(spec=httpx.Response)
    respuesta.raise_for_status.side_effect = httpx.HTTPStatusError(
        "503 Service Unavailable", request=MagicMock(), response=respuesta
    )
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = respuesta
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("leche") is None


def test_buscar_productos_error_de_red_devuelve_none():
    """Un timeout de red también degrada a None."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.side_effect = httpx.ReadTimeout("tiempo agotado")
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("leche") is None


# ===== Dominio: términos de búsqueda =====


def test_terminos_busqueda_frase_simple():
    """Una palabra se envía tal cual, en minúsculas."""
    assert terminos_busqueda_catalogo("Leche") == ["leche"]


def test_terminos_busqueda_frase_compuesta_agrega_termino_mas_largo():
    """Una frase de varias palabras agrega su término más largo, porque Catálogo busca la subcadena literal."""
    assert terminos_busqueda_catalogo("harina almendras") == ["harina almendras", "almendras"]


def test_terminos_busqueda_agrega_variante_sin_tildes():
    """Con tildes se agrega la variante sin ellas, porque Catálogo distingue tildes."""
    assert terminos_busqueda_catalogo("Azúcar flor") == ["azúcar flor", "azúcar", "azucar flor", "azucar"]


def test_terminos_busqueda_vacia():
    """Un texto vacío no genera consultas."""
    assert terminos_busqueda_catalogo("   ") == []


# ===== Pipeline del chat =====


class ProveedorFijo(ProveedorIA):
    """Proveedor determinista: devuelve los filtros indicados y una explicación fija."""

    nombre = "fijo"

    def __init__(self, filtros: dict) -> None:
        """Guarda los filtros que devolverá la etapa 1."""
        self._filtros = filtros

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        """Devuelve los filtros fijos sin llamar a ningún modelo."""
        return ResultadoExtraccion(filtros=self._filtros, tiempo_extraccion_ms=1)

    async def generar_explicacion(self, mensaje: str, productos: list) -> str:
        """Devuelve un texto fijo para no depender de un LLM."""
        return "Explicación de prueba."


class RepositorioDoble:
    """Doble del repositorio con caché fija y resultados de búsqueda por término."""

    def __init__(self, cache: list, busquedas: dict | None, fuente: str = FUENTE_CATALOGO) -> None:
        """`busquedas` mapea término → productos; None simula a Catálogo caído para toda búsqueda."""
        self._cache = cache
        self._busquedas = busquedas
        self.fuente = fuente
        self.datos_antiguos = False
        self.terminos_buscados: list[tuple[str, float | None]] = []

    def obtener_todos(self) -> list:
        """Devuelve la caché fija."""
        return self._cache

    def buscar_productos(self, termino: str, precio_max: float | None = None) -> list | None:
        """Registra la consulta y devuelve el resultado configurado para el término."""
        self.terminos_buscados.append((termino, precio_max))
        if self._busquedas is None:
            return None
        return self._busquedas.get(termino, [])

    def consultar_disponibilidad(self, producto_ids: list) -> dict | None:
        """Sin token de consulta: la disponibilidad no se valida."""
        return None


def _procesar(repositorio: RepositorioDoble, filtros: dict, mensaje: str = "mensaje de prueba") -> dict:
    """Ejecuta el pipeline completo de forma síncrona."""
    return anyio.run(procesar_mensaje, ProveedorFijo(filtros), repositorio, mensaje)


def test_chat_encuentra_producto_fuera_de_la_cache():
    """El caso del diagnóstico: la harina de almendras no está en la caché, pero sí en Catálogo."""
    repositorio = RepositorioDoble(
        cache=[_normalizar(YOGUR_BARATO)],
        busquedas={"harina de almendras": [_normalizar(HARINA_ALMENDRAS)]},
    )

    datos = _procesar(repositorio, {"palabras_clave": ["harina de almendras"]})

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-harina-almendras"]
    assert datos["estado_datos"] == ESTADO_VIGENTE
    assert ("harina de almendras", None) in repositorio.terminos_buscados


def test_chat_aplica_filtro_determinista_sobre_lo_que_devuelve_catalogo():
    """El término más largo trae productos de más ("almendras"); el filtro exige todos los términos."""
    repositorio = RepositorioDoble(
        cache=[],
        busquedas={
            "harina almendras": [],
            "almendras": [
                _normalizar(HARINA_ALMENDRAS),
                _normalizar(_producto("prod-almendras", "Almendras 200 g", 3990.0)),
            ],
        },
    )

    datos = _procesar(repositorio, {"palabras_clave": ["harina almendras"]})

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-harina-almendras"]


def test_chat_envia_precio_max_y_lo_revalida():
    """`precio_max` viaja a Catálogo y el filtro determinista lo vuelve a aplicar."""
    leche_cara = _normalizar(_producto("prod-leche-cara", "Leche Sin Lactosa 1 L", 2490.0, "lacteos"))
    repositorio = RepositorioDoble(cache=[], busquedas={"leche": [_normalizar(LECHE_ENTERA), leche_cara]})

    datos = _procesar(repositorio, {"palabras_clave": ["leche"], "precio_max": 2000})

    assert repositorio.terminos_buscados == [("leche", 2000.0)]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_busqueda_sin_resultados_no_dice_que_el_catalogo_esta_caido():
    """Una búsqueda exitosa y vacía responde 'sin coincidencias', aunque la caché esté vacía."""
    repositorio = RepositorioDoble(cache=[], busquedas={})

    datos = _procesar(repositorio, {"palabras_clave": ["caviar"]})

    assert datos["respuesta"] == TEXTO_SIN_PRODUCTOS
    assert datos["productos_recomendados"] == []
    assert datos["estado_datos"] == ESTADO_VIGENTE


def test_chat_busqueda_caida_degrada_a_la_cache():
    """Si Catálogo no responde a la búsqueda, se usa la caché y el estado lo informa el repositorio."""
    repositorio = RepositorioDoble(cache=[_normalizar(LECHE_ENTERA)], busquedas=None, fuente=FUENTE_MEMORIA)

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]
    assert datos["estado_datos"] == ESTADO_MEMORIA


def test_chat_busqueda_caida_y_cache_vacia_informa_catalogo_no_disponible():
    """Sin búsqueda ni caché no se inventan productos: se informa que el catálogo no está disponible."""
    repositorio = RepositorioDoble(cache=[], busquedas=None)

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert datos["productos_recomendados"] == []


def test_chat_solo_categoria_usa_la_cache_sin_buscar():
    """Sin palabras clave ni ingredientes se mantiene la caché precargada."""
    repositorio = RepositorioDoble(cache=[_normalizar(LECHE_ENTERA)], busquedas={})

    datos = _procesar(repositorio, {"categoria": "lacteos"})

    assert repositorio.terminos_buscados == []
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_receta_busca_cada_ingrediente():
    """Cada ingrediente se busca en Catálogo y se vincula con sus candidatos reales."""
    repositorio = RepositorioDoble(
        cache=[],
        busquedas={"harina": [_normalizar(HARINA_TRIGO)], "leche": [_normalizar(LECHE_ENTERA)]},
    )
    filtros = {
        "ingredientes": [
            {"nombre": "harina", "cantidad_envases": 1},
            {"nombre": "leche", "cantidad_envases": 2},
            {"nombre": "vainilla", "cantidad_envases": 1},
        ]
    }

    datos = _procesar(repositorio, filtros)

    terminos = [termino for termino, _precio in repositorio.terminos_buscados]
    assert sorted(terminos) == ["harina", "leche", "vainilla"]
    assert [ingrediente["ingrediente"] for ingrediente in datos["ingredientes_receta"]] == ["harina", "leche"]
    assert datos["ingredientes_faltantes"] == ["vainilla"]


def test_chat_recorta_consultas_y_completa_con_la_cache(monkeypatch):
    """Con más consultas que el máximo se recorta y se agrega la caché para no perder ingredientes."""
    monkeypatch.setattr(config, "CATALOGO_BUSQUEDA_MAX_CONSULTAS", 1)
    repositorio = RepositorioDoble(
        cache=[_normalizar(LECHE_ENTERA)],
        busquedas={"harina": [_normalizar(HARINA_TRIGO)]},
    )
    filtros = {
        "ingredientes": [
            {"nombre": "harina", "cantidad_envases": 1},
            {"nombre": "leche", "cantidad_envases": 1},
        ]
    }

    datos = _procesar(repositorio, filtros)

    assert len(repositorio.terminos_buscados) == 1
    assert [ingrediente["ingrediente"] for ingrediente in datos["ingredientes_receta"]] == ["harina", "leche"]


# ===== Utilidades =====


def _normalizar(producto: dict) -> dict:
    """Normaliza como lo hace el repositorio real, para que el filtrado encuentre todas las claves."""
    return ProductoRepositorio(url_base=URL_CATALOGO, cliente=MagicMock(spec=httpx.Client))._normalizar_producto(
        producto
    )
