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

import logging
import threading
import time
from unittest.mock import MagicMock

import anyio
import httpx
import pytest

from app import config
from app.domain.filtros import terminos_busqueda_catalogo
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.repositories.catalogo import FUENTE_CATALOGO, FUENTE_MEMORIA, ProductoRepositorio
from app.services.chat import (
    AVISO_CARGA_PREVIA,
    AVISO_MEMORIA_ANTIGUA,
    ESTADO_MEMORIA,
    ESTADO_MEMORIA_ANTIGUA,
    ESTADO_SIN_DATOS,
    ESTADO_VIGENTE,
    TEXTO_CATALOGO_NO_DISPONIBLE,
    TEXTO_SIN_FILTROS,
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
    respuesta.status_code = 503
    respuesta.raise_for_status.side_effect = httpx.HTTPStatusError(
        "503 Service Unavailable", request=MagicMock(), response=respuesta
    )
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = respuesta
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("leche") is None


def test_buscar_productos_termino_corto_no_llama_a_catalogo():
    """Catálogo responde 400 con menos de 3 bytes: no se consulta y se trata como sin coincidencias."""
    cliente_http = MagicMock(spec=httpx.Client)
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    assert repositorio.buscar_productos("te") == []
    assert repositorio.buscar_productos("a%") == []
    cliente_http.get.assert_not_called()


def test_buscar_productos_error_no_registra_la_url_interna(caplog: pytest.LogCaptureFixture):
    """El log de un fallo informa tipo y código HTTP, sin la URL interna de Catálogo."""
    peticion = httpx.Request("GET", f"{URL_CATALOGO}/api/v1/productos?q=leche")
    respuesta = httpx.Response(503, request=peticion)
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = respuesta
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    with caplog.at_level(logging.WARNING):
        assert repositorio.buscar_productos("leche") is None

    assert "HTTPStatusError 503" in caplog.text
    assert "catalogo:8080" not in caplog.text


def test_buscar_productos_reutiliza_un_cliente_compartido():
    """Sin cliente inyectado se crea un único cliente para todas las búsquedas y `cerrar` lo libera."""
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO)

    primero = repositorio._obtener_cliente_busqueda()
    assert repositorio._obtener_cliente_busqueda() is primero

    repositorio.cerrar()
    assert primero.is_closed
    assert repositorio._obtener_cliente_busqueda() is not primero
    repositorio.cerrar()


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


def test_terminos_busqueda_descarta_lo_que_catalogo_rechaza():
    """Se omiten los términos con menos de 3 bytes tras sanear ("te"); "té" sí tiene 3 bytes."""
    assert terminos_busqueda_catalogo("te") == []
    assert terminos_busqueda_catalogo("té") == ["té"]


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
        self.recargas = 0

    def productos_en_memoria(self) -> list:
        """Devuelve la caché fija sin red."""
        return self._cache

    def obtener_cache_degradada(self) -> tuple[list, bool]:
        """Devuelve la caché fija y su indicador de antigüedad sin ir a la red."""
        return self._cache, self.datos_antiguos

    def obtener_todos(self) -> list:
        """Devuelve la caché fija y cuenta las recargas pedidas."""
        self.recargas += 1
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


def test_chat_busqueda_caida_usa_la_cache_sin_recargarla():
    """Con caché disponible no se pide otra recarga completa: Catálogo ya no respondió."""
    repositorio = RepositorioDoble(cache=[_normalizar(LECHE_ENTERA)], busquedas=None, fuente=FUENTE_MEMORIA)

    _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert repositorio.recargas == 0


def test_chat_frase_sin_terminos_admitidos_usa_la_cache():
    """Una palabra clave que Catálogo rechazaría ("te") se resuelve con la caché, sin consultar."""
    te_negro = _normalizar(_producto("prod-te", "Te Negro 100 bolsitas", 1990.0))
    repositorio = RepositorioDoble(cache=[te_negro], busquedas={})

    datos = _procesar(repositorio, {"palabras_clave": ["te"]})

    assert repositorio.terminos_buscados == []
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-te"]


def test_chat_limita_las_busquedas_simultaneas(monkeypatch):
    """Nunca hay más consultas en vuelo que `CATALOGO_BUSQUEDA_CONCURRENCIA`."""
    monkeypatch.setattr(config, "CATALOGO_BUSQUEDA_CONCURRENCIA", 2)

    class RepositorioConcurrencia(RepositorioDoble):
        """Mide el máximo de búsquedas simultáneas."""

        def __init__(self) -> None:
            super().__init__(cache=[], busquedas={})
            self._candado_conteo = threading.Lock()
            self.en_vuelo = 0
            self.maximo_en_vuelo = 0

        def buscar_productos(self, termino: str, precio_max: float | None = None) -> list:
            with self._candado_conteo:
                self.en_vuelo += 1
                self.maximo_en_vuelo = max(self.maximo_en_vuelo, self.en_vuelo)
            time.sleep(0.05)
            with self._candado_conteo:
                self.en_vuelo -= 1
            return []

    repositorio = RepositorioConcurrencia()

    _procesar(repositorio, {"palabras_clave": ["leche", "harina", "arroz", "aceite", "azucar", "fideos"]})

    assert repositorio.maximo_en_vuelo == 2


def test_chat_busqueda_lenta_no_alarga_la_etapa_2(monkeypatch):
    """Una consulta que excede el plazo común se abandona y se completa con la caché."""
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 0.2)

    class RepositorioLento(RepositorioDoble):
        """La búsqueda de "harina" tarda más que el plazo."""

        def buscar_productos(self, termino: str, precio_max: float | None = None) -> list | None:
            if termino == "harina":
                time.sleep(1.0)
            return super().buscar_productos(termino, precio_max)

    repositorio = RepositorioLento(
        cache=[_normalizar(HARINA_TRIGO)],
        busquedas={"leche": [_normalizar(LECHE_ENTERA)]},
    )

    inicio = time.monotonic()
    datos = _procesar(repositorio, {"palabras_clave": ["leche", "harina"]})

    assert time.monotonic() - inicio < 0.9
    assert sorted(producto["id"] for producto in datos["productos_recomendados"]) == ["prod-harina-trigo", "prod-leche"]


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


def test_chat_degradado_con_cache_real_antigua_informa_memoria_antigua():
    """Tras una carga real exitosa envejecida, una búsqueda fallida no puede informarse como vigente."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": [LECHE_ENTERA], "paginacion": {"total_paginas": 1}})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http, antiguedad_maxima=1.0)
    repositorio.cargar_productos()
    assert repositorio.fuente == FUENTE_CATALOGO

    # Envejecemos la caché sin dormir y hacemos fallar la búsqueda posterior.
    repositorio._ultima_carga_exitosa -= 3600.0
    cliente_http.get.side_effect = httpx.ReadTimeout("tiempo agotado")

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["estado_datos"] == ESTADO_MEMORIA_ANTIGUA
    assert datos["estado_datos"] != ESTADO_VIGENTE
    assert AVISO_MEMORIA_ANTIGUA in datos["respuesta"]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_degradado_con_cache_real_reciente_informa_memoria_sin_aviso_antiguedad():
    """Tras una carga real exitosa reciente, una búsqueda fallida se clasifica como memoria sin aviso de antigüedad."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": [LECHE_ENTERA], "paginacion": {"total_paginas": 1}})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http, antiguedad_maxima=60.0)
    repositorio.cargar_productos()
    assert repositorio.fuente == FUENTE_CATALOGO

    cliente_http.get.side_effect = httpx.ReadTimeout("tiempo agotado")

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["estado_datos"] == ESTADO_MEMORIA
    assert AVISO_CARGA_PREVIA in datos["respuesta"]
    assert AVISO_MEMORIA_ANTIGUA not in datos["respuesta"]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_busqueda_vencida_con_cache_vacia_no_encadena_recarga(monkeypatch):
    """Con caché vacía y búsqueda vencida, no se invoca obtener_todos ni se excede el plazo común."""
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 0.05)

    class RepositorioLentoSinCache(RepositorioDoble):
        """Simula búsqueda lenta y una recarga completa también lenta."""

        def buscar_productos(self, termino: str, precio_max: float | None = None) -> list | None:
            time.sleep(0.25)
            return super().buscar_productos(termino, precio_max)

        def obtener_todos(self) -> list:
            time.sleep(0.20)
            return super().obtener_todos()

    repositorio = RepositorioLentoSinCache(cache=[], busquedas={})

    inicio = time.monotonic()
    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})
    duracion = time.monotonic() - inicio

    assert duracion < 0.20
    assert repositorio.recargas == 0
    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert datos["estado_datos"] == ESTADO_SIN_DATOS
    assert datos["productos_recomendados"] == []


def test_chat_busqueda_vencida_conserva_parciales_sin_recargar_cache(monkeypatch):
    """Si un término vence pero otro respondió, conserva los parciales y no llama a obtener_todos."""
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 0.05)

    class RepositorioParcialLento(RepositorioDoble):
        """'leche' responde al instante; 'harina' y la recarga completa son lentas."""

        def buscar_productos(self, termino: str, precio_max: float | None = None) -> list | None:
            if termino == "harina":
                time.sleep(0.25)
            return super().buscar_productos(termino, precio_max)

        def obtener_todos(self) -> list:
            time.sleep(0.20)
            return super().obtener_todos()

    repositorio = RepositorioParcialLento(
        cache=[],
        busquedas={"leche": [_normalizar(LECHE_ENTERA)]},
    )

    inicio = time.monotonic()
    datos = _procesar(repositorio, {"palabras_clave": ["leche", "harina"]})
    duracion = time.monotonic() - inicio

    assert duracion < 0.20
    assert repositorio.recargas == 0
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_solo_precio_usa_terminos_utiles_del_mensaje_como_respaldo():
    """Si el LLM solo devuelve precio_max, rescata 'leche' del mensaje en vez de devolver lo más barato de la caché."""
    bicarbonato = _normalizar(_producto("prod-bicarbonato", "Bicarbonato de Sodio 100 g", 170.0, "despensa"))
    repositorio = RepositorioDoble(
        cache=[bicarbonato],
        busquedas={"leche": [_normalizar(LECHE_ENTERA)]},
    )

    datos = _procesar(repositorio, {"precio_max": 2000}, mensaje="Busco leche por menos de $2.000")

    assert repositorio.terminos_buscados == [("leche", 2000.0)]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]
    assert datos["filtros_aplicados"] == {"precio_max": 2000, "palabras_clave": ["leche"]}


def test_chat_solo_precio_sin_terminos_utiles_no_devuelve_cache_barata():
    """Si el LLM solo devuelve precio_max y el mensaje no tiene términos de producto, responde que no entendió."""
    bicarbonato = _normalizar(_producto("prod-bicarbonato", "Bicarbonato de Sodio 100 g", 170.0, "despensa"))
    repositorio = RepositorioDoble(cache=[bicarbonato], busquedas={})

    datos = _procesar(repositorio, {"precio_max": 2000}, mensaje="Algo barato por menos de $2.000")

    assert datos["respuesta"] == TEXTO_SIN_FILTROS
    assert datos["productos_recomendados"] == []
    assert datos["filtros_aplicados"] is None
    assert repositorio.recargas == 0


def test_chat_respaldo_busco_harina_de_almendras_no_devuelve_harina_tostada():
    """El respaldo conserva 'harina almendras' como una sola palabra clave y descarta 'Harina Tostada Granel'."""
    harina_tostada = _normalizar(_producto("prod-tostada", "Harina Tostada Granel", 1200.0, "despensa"))
    harina_almendras = _normalizar(_producto("prod-almendras", "Harina de Almendras 500 g", 4990.0, "despensa"))
    repositorio = RepositorioDoble(
        cache=[harina_tostada],
        busquedas={"almendras": [harina_tostada, harina_almendras]},
    )

    datos = _procesar(repositorio, {"precio_max": 6000}, mensaje="Busco harina de almendras disponible")

    assert datos["filtros_aplicados"] == {"precio_max": 6000, "palabras_clave": ["harina almendras"]}
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-almendras"]


# ===== Utilidades =====


def _normalizar(producto: dict) -> dict:
    """Normaliza como lo hace el repositorio real, para que el filtrado encuentre todas las claves."""
    return ProductoRepositorio(url_base=URL_CATALOGO, cliente=MagicMock(spec=httpx.Client))._normalizar_producto(
        producto
    )
