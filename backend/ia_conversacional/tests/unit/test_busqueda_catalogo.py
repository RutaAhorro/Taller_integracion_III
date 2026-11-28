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


def test_terminos_busqueda_frase_compuesta_usa_una_sola_palabra_y_su_singular():
    """Una frase de varias palabras consulta solo la palabra más larga (y su singular si aplica)."""
    assert terminos_busqueda_catalogo("harina de almendras") == ["almendras", "almendra"]
    assert terminos_busqueda_catalogo("harina almendras") == ["almendras", "almendra"]
    assert terminos_busqueda_catalogo("aceite de oliva") == ["aceite"]
    assert terminos_busqueda_catalogo("leche sin lactosa") == ["lactosa"]
    assert terminos_busqueda_catalogo("arroz grado 2") == ["arroz"]


def test_terminos_busqueda_plurales_agrega_singular_despues_del_original():
    """Primero se prueba el término tal cual y después la forma singular (con y sin tilde si aplica)."""
    assert terminos_busqueda_catalogo("zanahorias") == ["zanahorias", "zanahoria"]
    assert terminos_busqueda_catalogo("verduras") == ["verduras", "verdura"]
    assert terminos_busqueda_catalogo("limones") == ["limones", "limón", "limon"]


def test_terminos_busqueda_agrega_variante_sin_tildes():
    """Con tildes elige la palabra más larga y agrega su variante sin tildes, porque Catálogo distingue tildes."""
    assert terminos_busqueda_catalogo("Azúcar flor") == ["azúcar", "azucar"]


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
    """El caso del diagnóstico: 'harina de almendras' consulta Catálogo solo con `q=almendras` y filtra en IA."""
    repositorio = RepositorioDoble(
        cache=[_normalizar(YOGUR_BARATO)],
        busquedas={"almendras": [_normalizar(HARINA_ALMENDRAS)]},
    )

    datos = _procesar(repositorio, {"palabras_clave": ["harina de almendras"]})

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-harina-almendras"]
    assert datos["estado_datos"] == ESTADO_VIGENTE
    assert repositorio.terminos_buscados == [("almendras", None)]


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


def test_chat_busqueda_caida_no_usa_la_cache_e_informa_catalogo_no_disponible():
    """Si Catálogo no responde por red, no se usa la copia parcial en memoria: se informa que no respondió."""
    repositorio = RepositorioDoble(cache=[_normalizar(LECHE_ENTERA)], busquedas=None, fuente=FUENTE_MEMORIA)

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["productos_recomendados"] == []
    assert datos["estado_datos"] == ESTADO_SIN_DATOS
    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert AVISO_CARGA_PREVIA not in datos["respuesta"]
    assert repositorio.recargas == 0


def test_chat_frase_sin_terminos_admitidos_no_usa_la_cache():
    """Un término que Catálogo rechazaría ('te') no cae a la copia parcial en memoria."""
    te_negro = _normalizar(_producto("prod-te", "Te Negro 100 bolsitas", 1990.0))
    repositorio = RepositorioDoble(cache=[te_negro], busquedas={})

    datos = _procesar(repositorio, {"palabras_clave": ["te"]})

    assert repositorio.terminos_buscados == []
    assert datos["productos_recomendados"] == []
    assert datos["respuesta"] == TEXTO_SIN_PRODUCTOS
    assert datos["estado_datos"] == ESTADO_VIGENTE


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
    """Una consulta que excede el plazo de Catálogo se abandona sin caer a la copia en memoria."""
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 0.2)

    class RepositorioLento(RepositorioDoble):
        """La búsqueda de 'harina' tarda más que el plazo."""

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
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]
    assert datos["estado_datos"] == ESTADO_VIGENTE


def test_chat_busqueda_caida_y_cache_vacia_informa_catalogo_no_disponible():
    """Sin respuesta de Catálogo no se inventan productos: se informa que el catálogo no está disponible."""
    repositorio = RepositorioDoble(cache=[], busquedas=None)

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert datos["productos_recomendados"] == []


def test_chat_solo_categoria_busca_en_catalogo_sin_usar_cache():
    """Con solo categoría se consulta Catálogo por `?q=` en vez de depender de la copia en memoria."""
    repositorio = RepositorioDoble(cache=[], busquedas={"lacteos": [_normalizar(LECHE_ENTERA)]})

    datos = _procesar(repositorio, {"categoria": "lacteos"})

    assert repositorio.terminos_buscados == [("lacteos", None)]
    assert repositorio.recargas == 0
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


def test_chat_recorta_consultas_sin_completar_con_cache(monkeypatch):
    """Con más términos que el máximo se recorta sin caer a la copia en memoria."""
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
    assert [ingrediente["ingrediente"] for ingrediente in datos["ingredientes_receta"]] == ["harina"]
    assert datos["ingredientes_faltantes"] == ["leche"]


def test_chat_error_red_catalogo_real_no_usa_cache_previa():
    """Incluso si existiera carga previa en memoria, un fallo de red responde sin_datos y sin aviso de carga previa."""
    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.return_value = _respuesta_http({"data": [LECHE_ENTERA], "paginacion": {"total_paginas": 1}})
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http, antiguedad_maxima=1.0)
    repositorio.cargar_productos()
    assert repositorio.fuente == FUENTE_CATALOGO

    cliente_http.get.side_effect = httpx.ReadTimeout("tiempo agotado")

    datos = _procesar(repositorio, {"palabras_clave": ["leche"]})

    assert datos["estado_datos"] == ESTADO_SIN_DATOS
    assert datos["respuesta"] == TEXTO_CATALOGO_NO_DISPONIBLE
    assert AVISO_CARGA_PREVIA not in datos["respuesta"]
    assert AVISO_MEMORIA_ANTIGUA not in datos["respuesta"]
    assert datos["productos_recomendados"] == []


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


# ===== Pruebas de plurales, términos compuestos, reparto de plazos y casos simulados =====


def test_chat_plural_prueba_original_y_cae_a_singular():
    """Prueba primero 'zanahorias' y, al recibir 0 resultados, busca 'zanahoria' y devuelve el producto real."""
    zanahoria = _normalizar(_producto("prod-zanahoria", "Zanahoria Granel 1 kg", 890.0, "verduras"))
    repositorio = RepositorioDoble(cache=[], busquedas={"zanahoria": [zanahoria]})

    datos = _procesar(repositorio, {"palabras_clave": ["zanahorias"]}, mensaje="Que zanahorias estan baratas?")

    assert repositorio.terminos_buscados == [("zanahorias", None), ("zanahoria", None)]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-zanahoria"]
    assert datos["estado_datos"] == ESTADO_VIGENTE


def test_chat_plural_no_consulta_singular_si_original_ya_trajo_resultados():
    """Si el término original ya devolvió resultados, no se hace una segunda petición por el singular."""
    fideos = _normalizar(_producto("prod-fideos", "Fideos Spaghetti 400 g", 990.0, "despensa"))
    repositorio = RepositorioDoble(cache=[], busquedas={"fideos": [fideos]})

    datos = _procesar(repositorio, {"palabras_clave": ["fideos"]}, mensaje="Busco fideos")

    assert repositorio.terminos_buscados == [("fideos", None)]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-fideos"]


def test_chat_terminos_unidos_por_o_combina_sin_duplicados_y_respeta_limite(monkeypatch):
    """'zanahorias o verduras' busca cada término aparte, une sin duplicados y respeta el límite de tarjetas."""
    monkeypatch.setattr(config, "EXPLICACION_MAX_PRODUCTOS", 3)
    zanahoria = _normalizar(_producto("prod-zanahoria", "Zanahoria Granel", 890.0, "verduras"))
    lechuga = _normalizar(_producto("prod-lechuga", "Lechuga Costina", 990.0, "verduras"))
    tomate = _normalizar(_producto("prod-tomate", "Tomate Larga Vida", 1190.0, "verduras"))
    zapallo = _normalizar(_producto("prod-zapallo", "Zapallo Camote", 1490.0, "verduras"))
    repositorio = RepositorioDoble(
        cache=[],
        busquedas={
            "zanahoria": [zanahoria],
            "verdura": [zanahoria, lechuga, tomate, zapallo],
        },
    )

    datos = _procesar(
        repositorio,
        {"palabras_clave": ["zanahorias o verduras"]},
        mensaje="qué zanahorias o verduras tienes baratas",
    )

    ids = [producto["id"] for producto in datos["productos_recomendados"]]
    assert len(ids) == 3
    assert len(ids) == len(set(ids))
    assert "prod-zanahoria" in ids
    assert ("zanahoria", None) in repositorio.terminos_buscados
    assert ("verdura", None) in repositorio.terminos_buscados


def test_chat_catalogo_lento_no_deja_sin_tiempo_a_la_explicacion(monkeypatch):
    """Catálogo no puede gastar más de su fracción del plazo y la explicación conserva su mínimo garantizado."""
    monkeypatch.setattr(config, "CHAT_TIMEOUT_TOTAL", 1.0)
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 5.0)
    monkeypatch.setattr(config, "CHAT_CATALOGO_FRACCION_PLAZO", 0.2)
    monkeypatch.setattr(config, "CHAT_EXPLICACION_PLAZO_MINIMO", 0.4)

    class RepositorioParcialLento(RepositorioDoble):
        """'leche' responde rápido; 'harina' tarda más que la fracción permitida para Catálogo."""

        def buscar_productos(self, termino: str, precio_max: float | None = None) -> list | None:
            if termino == "harina":
                time.sleep(0.8)
            return super().buscar_productos(termino, precio_max)

    class ProveedorExplicacionMedia(ProveedorFijo):
        """La explicación tarda 0.25s: cabe holgadamente en el mínimo garantizado (0.4s)."""

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            await anyio.sleep(0.25)
            return "Explicación completa dentro del plazo reservado."

    repositorio = RepositorioParcialLento(cache=[], busquedas={"leche": [_normalizar(LECHE_ENTERA)]})
    proveedor = ProveedorExplicacionMedia({"palabras_clave": ["leche", "harina"]})

    inicio = time.monotonic()
    datos = anyio.run(procesar_mensaje, proveedor, repositorio, "leche o harina")
    duracion = time.monotonic() - inicio

    assert duracion < 0.85
    assert datos["respuesta"] == "Explicación completa dentro del plazo reservado."
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]


def test_chat_explicacion_falla_responde_con_frase_fija_y_productos():
    """Si la explicación falla con excepción, devuelve los productos reales y una frase fija corta con el término y precio CLP."""
    zanahoria = _normalizar(_producto("prod-zanahoria", "Zanahoria Granel", 10600.0, "verduras"))
    repositorio = RepositorioDoble(cache=[], busquedas={"zanahoria": [zanahoria]})

    class ProveedorExplicacionFallida(ProveedorFijo):
        """Simula error de red/cuota en etapa 3."""

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            raise RuntimeError("Proveedor sin cuota")

    datos = anyio.run(
        procesar_mensaje,
        ProveedorExplicacionFallida({"palabras_clave": ["zanahoria"]}),
        repositorio,
        "Busco zanahoria",
    )

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-zanahoria"]
    assert datos["respuesta"] == "Estas son las opciones que encontré para «zanahoria», desde $10.600."


def test_chat_busqueda_sin_resultados_responde_mensaje_claro_y_no_copia_parcial():
    """Sin coincidencias en Catálogo responde mensaje claro, incluso si hubiera productos en la copia en memoria."""
    repositorio = RepositorioDoble(cache=[_normalizar(LECHE_ENTERA), _normalizar(YOGUR_BARATO)], busquedas={})

    datos = _procesar(repositorio, {"palabras_clave": ["trufa blanca"]})

    assert datos["respuesta"] == TEXTO_SIN_PRODUCTOS
    assert datos["productos_recomendados"] == []
    assert datos["estado_datos"] == ESTADO_VIGENTE
    assert AVISO_CARGA_PREVIA not in datos["respuesta"]
    assert repositorio.recargas == 0


def test_casos_chat_con_cliente_catalogo_simulado_pr208():
    """
    Simula el contrato real de búsqueda por prefijo/palabra de Catálogo (PR #208):
    'zanahorias' no coincide con 'Zanahoria Granel', solo 'zanahoria' coincide.
    Verifica que los 6 mensajes críticos devuelven productos reales y nunca caen en error genérico ni 0 resultados.
    """
    catalogo_simulado = [
        _producto("prod-leche-entera", "Leche Entera 1 L", 1190.0, "lacteos"),
        _producto("prod-leche-cara", "Leche Sin Lactosa 1 L", 2490.0, "lacteos"),
        _producto("prod-harina-almendras", "Harina de Almendras 500 g", 10600.0, "despensa"),
        _producto("prod-harina-tostada", "Harina Tostada Granel", 1200.0, "despensa"),
        _producto("prod-zanahoria", "Zanahoria Granel 1 kg", 890.0, "verduras"),
        _producto("prod-lechuga", "Lechuga Costina", 990.0, "verduras"),
        _producto("prod-tomate", "Tomate Larga Vida", 1290.0, "verduras"),
    ]

    def simular_get_catalogo(_url: str, params: dict) -> MagicMock:
        """Simula GET /api/v1/productos?q= con coincidencia por prefijo de palabra en nombre/categoría."""
        consulta = str(params.get("q", "")).strip().lower()
        precio_max = params.get("precio_max")
        coincidencias = []
        for item in catalogo_simulado:
            if precio_max is not None and item["precio"] > float(precio_max):
                continue
            texto_nombre = item["nombre"].lower()
            texto_categoria = item["categoria"].lower()
            palabras_objetivo = [*texto_nombre.split(), *texto_categoria.split()]
            if (
                texto_nombre.startswith(consulta)
                or any(palabra.startswith(consulta) for palabra in palabras_objetivo)
                or consulta in texto_nombre
            ):
                coincidencias.append(item)
        return _respuesta_http({"data": coincidencias, "paginacion": {"total_paginas": 1}})

    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.side_effect = simular_get_catalogo
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    casos = [
        ("Busco leche por menos de $2.000", {"palabras_clave": ["leche"], "precio_max": 2000}, "prod-leche-entera"),
        ("Busco harina de almendras", {"palabras_clave": ["harina de almendras"]}, "prod-harina-almendras"),
        # También cubre cuando el LLM o el respaldo omite la palabra vacía "de" ("harina almendras").
        ("Busco harina de almendras", {"palabras_clave": ["harina almendras"]}, "prod-harina-almendras"),
        ("¿Me recomiendas leche?", {"palabras_clave": ["leche"]}, "prod-leche-entera"),
        # Simula el caso en que el LLM responde texto libre sin llamar a la herramienta: rescate por respaldo.
        ("me recomiendas leche?", None, "prod-leche-entera"),
        ("qué zanahorias o verduras tienes baratas", {"palabras_clave": ["zanahorias o verduras"]}, "prod-zanahoria"),
        ("Que zanahorias estan baratas?", {"palabras_clave": ["zanahorias"]}, "prod-zanahoria"),
    ]

    for mensaje, filtros, id_esperado in casos:
        datos = anyio.run(procesar_mensaje, ProveedorFijo(filtros), repositorio, mensaje)
        ids = [producto["id"] for producto in datos["productos_recomendados"]]
        assert id_esperado in ids, f"Falló '{mensaje}': ids={ids}, respuesta={datos['respuesta']}"
        assert datos["respuesta"] != TEXTO_SIN_PRODUCTOS
        assert datos["respuesta"] != TEXTO_SIN_FILTROS
        assert datos["respuesta"] != TEXTO_CATALOGO_NO_DISPONIBLE
        assert "Lo siento, en este momento no puedo procesar tu solicitud" not in datos["respuesta"]
        assert datos["estado_datos"] == ESTADO_VIGENTE


# ===== Búsqueda por palabra única, filtrado multipalabra, orden, plazos (~14 s) y log diagnóstico =====


def test_chat_harina_de_almendras_solo_catalogo_con_q_almendras():
    """Con Catálogo que solo responde a `q=almendras`, encuentra la harina y descarta almendras sueltas."""
    harina_con_piel = _producto("prod-harina-piel", "Harina de Almendras con Piel Libre de Gluten 500 g", 6990.0)
    almendras_tostadas = _producto("prod-almendras-solas", "Almendras Tostadas 200 g", 3990.0)
    consultas_realizadas: list[str] = []

    def simular_get_solo_almendras(_url: str, params: dict) -> MagicMock:
        termino = str(params.get("q", "")).strip().lower()
        consultas_realizadas.append(termino)
        if termino == "almendras":
            return _respuesta_http({"data": [almendras_tostadas, harina_con_piel], "paginacion": {"total_paginas": 1}})
        return _respuesta_http({"data": [], "paginacion": {"total_paginas": 1}})

    cliente_http = MagicMock(spec=httpx.Client)
    cliente_http.get.side_effect = simular_get_solo_almendras
    repositorio = ProductoRepositorio(url_base=URL_CATALOGO, cliente=cliente_http)

    for palabras in (["harina de almendras"], ["harina almendras"]):
        consultas_realizadas.clear()
        datos = anyio.run(
            procesar_mensaje,
            ProveedorFijo({"palabras_clave": palabras}),
            repositorio,
            "Busco harina de almendras",
        )
        assert consultas_realizadas == ["almendras"]
        assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-harina-piel"]


def test_chat_multipalabra_aceite_oliva_leche_sin_lactosa_y_arroz_grado_2():
    """Consulta con una sola palabra y exige todas las palabras del término en el filtro de la IA."""
    aceite_maravilla = _normalizar(_producto("prod-aceite-maravilla", "Aceite de Maravilla 1 L", 1890.0))
    aceite_oliva = _normalizar(_producto("prod-aceite-oliva", "Aceite de Oliva Extra Virgen 500 ml", 5490.0))
    leche_sin_lactosa = _normalizar(_producto("prod-leche-sl", "Leche Descremada Sin Lactosa 1 L", 1390.0, "lacteos"))
    queso_sin_lactosa = _normalizar(_producto("prod-queso-sl", "Queso Gouda Sin Lactosa 200 g", 2490.0, "lacteos"))
    leche_entera = _normalizar(_producto("prod-leche-con", "Leche Entera con Lactosa 1 L", 1190.0, "lacteos"))
    arroz_g1 = _normalizar(_producto("prod-arroz-1", "Arroz Grado 1 1 kg", 1490.0))
    arroz_g2 = _normalizar(_producto("prod-arroz-2", "Arroz Grado 2 1 kg", 1190.0))
    arroz_basmati = _normalizar(_producto("prod-arroz-basmati", "Arroz Basmati 500 g", 2290.0))

    repositorio = RepositorioDoble(
        cache=[],
        busquedas={
            "aceite": [aceite_maravilla, aceite_oliva],
            "lactosa": [queso_sin_lactosa, leche_entera, leche_sin_lactosa],
            "arroz": [arroz_g1, arroz_g2, arroz_basmati],
        },
    )

    datos_aceite = _procesar(repositorio, {"palabras_clave": ["aceite de oliva"]}, mensaje="aceite de oliva")
    assert [producto["id"] for producto in datos_aceite["productos_recomendados"]] == ["prod-aceite-oliva"]
    assert repositorio.terminos_buscados == [("aceite", None)]

    repositorio.terminos_buscados.clear()
    datos_leche = _procesar(repositorio, {"palabras_clave": ["leche sin lactosa"]}, mensaje="leche sin lactosa")
    assert [producto["id"] for producto in datos_leche["productos_recomendados"]] == ["prod-leche-sl"]
    assert repositorio.terminos_buscados == [("lactosa", None)]

    repositorio.terminos_buscados.clear()
    datos_arroz = _procesar(repositorio, {"palabras_clave": ["arroz grado 2"]}, mensaje="arroz grado 2")
    assert [producto["id"] for producto in datos_arroz["productos_recomendados"]] == ["prod-arroz-2"]
    assert repositorio.terminos_buscados == [("arroz", None)]


def test_chat_multipalabra_cae_a_singular_si_plural_no_trae_resultados():
    """Si la palabra elegida en plural ('almendras') da 0 resultados, prueba su singular ('almendra')."""
    harina_singular = _normalizar(_producto("prod-harina-almendra", "Harina de Almendra 500 g", 5590.0))
    repositorio = RepositorioDoble(cache=[], busquedas={"almendra": [harina_singular]})

    datos = _procesar(repositorio, {"palabras_clave": ["harina de almendras"]}, mensaje="Busco harina de almendras")

    assert repositorio.terminos_buscados == [("almendras", None), ("almendra", None)]
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-harina-almendra"]


def test_chat_multipalabra_sin_resultados_descarta_coincidencia_parcial():
    """Si Catálogo solo devuelve 'Almendras Tostadas' para `q=almendras`, 'harina de almendras' responde sin resultados."""
    almendras_solas = _normalizar(_producto("prod-almendras", "Almendras Tostadas 200 g", 3990.0))
    repositorio = RepositorioDoble(cache=[], busquedas={"almendras": [almendras_solas]})

    datos = _procesar(repositorio, {"palabras_clave": ["harina de almendras"]}, mensaje="Busco harina de almendras")

    assert datos["productos_recomendados"] == []
    assert datos["respuesta"] == TEXTO_SIN_PRODUCTOS
    assert datos["estado_datos"] == ESTADO_VIGENTE


def test_chat_mantiene_orden_de_relevancia_que_devuelve_catalogo():
    """Conserva el orden de relevancia entregado por Catálogo sin reordenar por precio."""
    harina_mas_relevante = _normalizar(_producto("prod-rel-1", "Harina de Almendras con Piel 500 g", 6990.0))
    harina_menos_relevante = _normalizar(_producto("prod-rel-2", "Mix Harina de Almendras 500 g", 4490.0))
    almendras_descartadas = _normalizar(_producto("prod-descarte", "Almendras 200 g", 2990.0))
    repositorio = RepositorioDoble(
        cache=[],
        busquedas={"almendras": [almendras_descartadas, harina_mas_relevante, harina_menos_relevante]},
    )

    datos = _procesar(repositorio, {"palabras_clave": ["harina de almendras"]})

    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-rel-1", "prod-rel-2"]


def test_chat_omite_explicacion_si_etapas_1_y_2_superan_mitad_del_plazo(monkeypatch, caplog):
    """Con proveedor lento en extracción (>50% del plazo), omite la explicación y responde con frase fija antes del plazo."""
    monkeypatch.setattr(config, "CHAT_TIMEOUT_TOTAL", 0.6)
    monkeypatch.setattr(config, "CHAT_EXPLICACION_PLAZO_MINIMO", 0.2)
    monkeypatch.setattr(config, "CHAT_UMBRAL_OMITIR_EXPLICACION", 0.5)

    class ProveedorExtraccionLenta(ProveedorIA):
        nombre = "gemini"

        def __init__(self) -> None:
            self.llamadas_explicacion = 0

        async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
            await anyio.sleep(0.35)
            return ResultadoExtraccion(
                filtros={"palabras_clave": ["leche"]}, tiempo_extraccion_ms=350, proveedor="gemini"
            )

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            self.llamadas_explicacion += 1
            await anyio.sleep(2.0)
            return "No debería haberse llamado."

    proveedor = ProveedorExtraccionLenta()
    repositorio = RepositorioDoble(cache=[], busquedas={"leche": [_normalizar(LECHE_ENTERA)]})

    inicio = time.monotonic()
    with caplog.at_level(logging.INFO):
        datos = anyio.run(procesar_mensaje, proveedor, repositorio, "Busco leche secreta-no-loguear")
    duracion = time.monotonic() - inicio

    assert duracion < 0.6
    assert proveedor.llamadas_explicacion == 0
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]
    assert datos["respuesta"] == "Estas son las opciones que encontré para «leche», desde $1.190."
    assert datos["tiempo_explicacion_ms"] == 0
    assert "Consulta chat:" in caplog.text
    assert "proveedor=gemini" in caplog.text
    assert "productos=1" in caplog.text
    assert "frase_fija=si" in caplog.text
    assert "secreta-no-loguear" not in caplog.text


def test_chat_respeta_plazo_total_con_proveedor_lento_en_explicacion(monkeypatch, caplog):
    """Con proveedor lento en etapa 3, responde dentro del plazo total con tarjetas y frase fija."""
    monkeypatch.setattr(config, "CHAT_TIMEOUT_TOTAL", 0.5)
    monkeypatch.setattr(config, "CHAT_EXPLICACION_PLAZO_MINIMO", 0.2)
    monkeypatch.setattr(config, "CHAT_UMBRAL_OMITIR_EXPLICACION", 0.5)

    class ProveedorExplicacionMuyLenta(ProveedorIA):
        nombre = "groq"

        async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
            await anyio.sleep(0.05)
            return ResultadoExtraccion(filtros={"palabras_clave": ["leche"]}, tiempo_extraccion_ms=50, proveedor="groq")

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            await anyio.sleep(2.0)
            return "Respuesta tardía."

    repositorio = RepositorioDoble(cache=[], busquedas={"leche": [_normalizar(LECHE_ENTERA)]})

    inicio = time.monotonic()
    with caplog.at_level(logging.INFO):
        datos = anyio.run(procesar_mensaje, ProveedorExplicacionMuyLenta(), repositorio, "¿Me recomiendas leche?")
    duracion = time.monotonic() - inicio

    assert duracion < 0.5
    assert [producto["id"] for producto in datos["productos_recomendados"]] == ["prod-leche"]
    assert datos["respuesta"] == "Estas son las opciones que encontré para «leche», desde $1.190."
    assert "Consulta chat:" in caplog.text
    assert "proveedor=groq" in caplog.text
    assert "frase_fija=si" in caplog.text


def test_chat_registra_linea_info_con_tiempos_por_etapa_sin_datos_sensibles(caplog):
    """Por cada consulta al chat se emite una línea INFO con duraciones por etapa, proveedor, productos y frase_fija."""

    class ProveedorGeminiSimulado(ProveedorIA):
        nombre = "gemini"

        async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
            return ResultadoExtraccion(
                filtros={"palabras_clave": ["harina de almendras"]},
                tiempo_extraccion_ms=120,
                proveedor="gemini",
            )

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            return "Encontré Harina de Almendras a $5.990."

    repositorio = RepositorioDoble(cache=[], busquedas={"almendras": [_normalizar(HARINA_ALMENDRAS)]})

    with caplog.at_level(logging.INFO):
        datos = anyio.run(
            procesar_mensaje,
            ProveedorGeminiSimulado(),
            repositorio,
            "Busco harina de almendras token-privado-usuario",
        )

    assert len(datos["productos_recomendados"]) == 1
    lineas_diagnostico = [registro.message for registro in caplog.records if "Consulta chat:" in registro.message]
    assert len(lineas_diagnostico) == 1
    linea = lineas_diagnostico[0]
    assert "extraccion_ms=120" in linea
    assert "catalogo_ms=" in linea
    assert "explicacion_ms=" in linea
    assert "total_ms=" in linea
    assert "proveedor=gemini" in linea
    assert "productos=1" in linea
    assert "frase_fija=no" in linea
    assert "token-privado-usuario" not in caplog.text
    assert "harina de almendras" not in linea


# ===== Utilidades =====


def _normalizar(producto: dict) -> dict:
    """Normaliza como lo hace el repositorio real, para que el filtrado encuentre todas las claves."""
    return ProductoRepositorio(url_base=URL_CATALOGO, cliente=MagicMock(spec=httpx.Client))._normalizar_producto(
        producto
    )
