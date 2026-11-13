"""
Tests unitarios para ProductoRepositorio (SUP-246, SUP-353).

Verifica:
1. Conexión HTTP interna exitosa hacia el microservicio de Catálogo.
2. Filtrado de productos vigentes con stock (en_stock=true).
3. Paginación automática en Catálogo y límite de seguridad de páginas.
4. Resiliencia y degradación elegante ante timeouts, caídas de red o errores 5xx (sin_datos).
5. Preservación del estado en memoria si ocurre una falla transitoria posterior.
6. Compatibilidad determinista con filtros.py.
7. Concurrencia segura y prevención de llamadas redundantes con double-checked locking y TTL.
"""

from unittest.mock import MagicMock, patch

import httpx
import pytest

from app.filtros import filtrar_productos
from app.productos import ProductoRepositorio


MOCK_CATALOGO_PAGINA_1 = {
    "data": [
        {
            "id": "prod-uuid-1",
            "nombre": "Leche Entera 1L",
            "marca": "Colun",
            "categoria": "lacteos",
            "supermercado": "Lider",
            "precio": 1190.0,
            "precio_normal": 1190.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/leche.png",
            "unidad": "1L",
            "en_stock": True,
        },
        {
            "id": "prod-uuid-2",
            "nombre": "Yogurt Batido Vainilla",
            "marca": "Soprole",
            "categoria": "lacteos",
            "supermercado": "Jumbo",
            "precio": 450.0,
            "precio_normal": 490.0,
            "precio_oferta": 450.0,
            "en_oferta": True,
            "url_imagen": "https://img.super.cl/yogurt.png",
            "unidad": "120g",
            "en_stock": True,
        },
        {
            "id": "prod-uuid-3",
            "nombre": "Mantequilla con sal",
            "marca": "Colun",
            "categoria": "lacteos",
            "supermercado": "Lider",
            "precio": 2200.0,
            "precio_normal": 2200.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/mantequilla.png",
            "unidad": "250g",
            "en_stock": False,  # Sin stock: debe ser omitido
        },
    ],
    "paginacion": {
        "total_registros": 3,
        "pagina_actual": 1,
        "total_paginas": 1,
        "limite": 100,
    },
}

MOCK_CATALOGO_MULTI_PAG_1 = {
    "data": [
        {
            "id": "prod-p1",
            "nombre": "Manzana Royal Gala",
            "marca": "Generica",
            "categoria": "frutas",
            "supermercado": "Lider",
            "precio": 1290.0,
            "precio_normal": 1290.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/manzana.png",
            "unidad": "1kg",
            "en_stock": True,
        }
    ],
    "paginacion": {
        "total_registros": 2,
        "pagina_actual": 1,
        "total_paginas": 2,
        "limite": 1,
    },
}

MOCK_CATALOGO_MULTI_PAG_2 = {
    "data": [
        {
            "id": "prod-p2",
            "nombre": "Plátano Granel",
            "marca": "Generica",
            "categoria": "frutas",
            "supermercado": "Lider",
            "precio": 990.0,
            "precio_normal": 990.0,
            "precio_oferta": None,
            "en_oferta": False,
            "url_imagen": "https://img.super.cl/platano.png",
            "unidad": "1kg",
            "en_stock": True,
        }
    ],
    "paginacion": {
        "total_registros": 2,
        "pagina_actual": 2,
        "total_paginas": 2,
        "limite": 1,
    },
}


def test_carga_exitosa_desde_catalogo_http():
    """Verifica la carga exitosa de productos desde el endpoint interno de Catálogo."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()

    mock_cliente.get.assert_called_with(
        "http://catalogo:8080/api/v1/productos",
        params={"en_stock": "true", "page": 1, "limit": 100},
    )

    assert len(productos) == 2
    assert productos[0]["id"] == "prod-uuid-1"
    assert productos[0]["nombre"] == "Leche Entera 1L"
    assert productos[0]["precio"] == 1190.0
    assert productos[0]["en_stock"] is True
    assert "distancia_km" in productos[0]
    assert productos[1]["id"] == "prod-uuid-2"
    assert repo.fuente == "catalogo"


def test_paginacion_multiples_paginas():
    """Verifica que si Catálogo tiene múltiples páginas, se consultan y agregan todas."""
    mock_cliente = MagicMock(spec=httpx.Client)

    def side_effect(url_peticion, params):
        mock_resp = MagicMock(spec=httpx.Response)
        mock_resp.raise_for_status.return_value = None
        if params.get("page") == 1:
            mock_resp.json.return_value = MOCK_CATALOGO_MULTI_PAG_1
        else:
            mock_resp.json.return_value = MOCK_CATALOGO_MULTI_PAG_2
        return mock_resp

    mock_cliente.get.side_effect = side_effect

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()
    assert len(productos) == 2
    nombres = [producto["nombre"] for producto in productos]
    assert "Manzana Royal Gala" in nombres
    assert "Plátano Granel" in nombres


def test_degradacion_timeout_aplica_fallback():
    """Verifica que ante un TimeoutException sin caché previo, retorna lista vacía y fuente sin_datos."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_cliente.get.side_effect = httpx.TimeoutException("Petición agotó el tiempo de espera")

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()
    assert productos == []
    assert repo.fuente == "sin_datos"


def test_degradacion_error_conexion_aplica_fallback():
    """Verifica que si Catálogo no está disponible (ConnectError), entra en modo sin_datos sin crashear."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_cliente.get.side_effect = httpx.ConnectError("Connection refused")

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()
    assert productos == []
    assert repo.fuente == "sin_datos"


def test_degradacion_error_http_500_aplica_fallback():
    """Verifica que ante error HTTP 500 de Catálogo sin caché previa, entra en modo sin_datos."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.status_code = 500
    mock_respuesta.raise_for_status.side_effect = httpx.HTTPStatusError(
        "500 Internal Server Error",
        request=MagicMock(),
        response=mock_respuesta,
    )
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()
    assert productos == []
    assert repo.fuente == "sin_datos"


def test_preserva_cache_en_memoria_tras_falla_posterior():
    """
    Si el repositorio ya cargó productos de Catálogo exitosamente y luego una
    recarga posterior falla, debe mantener los productos válidos en memoria y fuente 'memoria'.
    """
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    # Primera carga exitosa
    assert len(repo.obtener_todos()) == 2
    assert repo.fuente == "catalogo"

    # Segunda carga falla (ej: Catálogo temporalmente no disponible)
    mock_cliente.get.side_effect = httpx.TimeoutException("Catálogo caído temporalmente")
    productos_despues_de_fallo = repo.obtener_todos(forzar=True)

    # Debe conservar los 2 productos previos de Catálogo
    assert len(productos_despues_de_fallo) == 2
    assert productos_despues_de_fallo[0]["id"] == "prod-uuid-1"
    assert repo.fuente == "memoria"


def test_compatibilidad_con_filtros_deterministas():
    """
    Verifica que la salida de ProductoRepositorio es procesada correctamente
    por la lógica determinista de filtros.py sin errores ni excepciones.
    """
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    productos = repo.obtener_todos()

    filtros = {"categoria": "lacteos", "precio_max": 1000}
    filtrados = filtrar_productos(productos, filtros)

    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Yogurt Batido Vainilla"
    assert filtrados[0]["precio"] == 450.0


def test_len_es_o1_y_no_llama_a_red():
    """Verifica que __len__ es O(1) en memoria y jamás dispara peticiones HTTP."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
    )

    mock_cliente.get.reset_mock()
    total = len(repo)
    assert total == 2
    assert mock_cliente.get.call_count == 0


def test_modo_degradado_resetea_ttl_evitando_llamadas_en_rafaga():
    """
    En modo degradado con productos en memoria, el fallo debe resetear _ultima_actualizacion
    para que el TTL amortigüe llamadas posteriores y no sature la red.
    """
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
        cache_ttl=60.0,
    )

    # Forzar recarga que falla
    mock_cliente.get.side_effect = httpx.TimeoutException("Catálogo caído")
    productos = repo.obtener_todos(forzar=True)
    assert len(productos) == 2

    # Inmediatamente después, una llamada regular (forzar=False) no debe intentar HTTP
    mock_cliente.get.reset_mock()
    productos_en_cache = repo.obtener_todos(forzar=False)
    assert len(productos_en_cache) == 2
    assert mock_cliente.get.call_count == 0


def test_httpx_client_propio_cierra_socket_ante_error():
    """
    Verifica que si se usa un cliente propio de httpx y la primera llamada falla,
    el bloque finally garantiza que cliente.close() sea ejecutado para evitar fugas de sockets.
    """
    mock_cliente_instancia = MagicMock()
    mock_cliente_instancia.get.side_effect = httpx.TimeoutException("Fallo en primer request")

    with patch("app.productos.httpx.Client", return_value=mock_cliente_instancia):
        ProductoRepositorio(
            base_url="http://catalogo:8080",
            client=None,
        )

    mock_cliente_instancia.close.assert_called()


def test_telemetria_truncamiento_mas_de_10_paginas(caplog: pytest.LogCaptureFixture):
    """Verifica que si Catálogo reporta más de 10 páginas, se emite un warning de truncamiento."""
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = {
        "data": [
            {
                "id": "prod-1",
                "nombre": "Producto 1",
                "precio": 1000.0,
                "en_stock": True,
            }
        ],
        "paginacion": {
            "total_registros": 1500,
            "pagina_actual": 1,
            "total_paginas": 15,
            "limite": 100,
        },
    }
    mock_respuesta.raise_for_status.return_value = None
    mock_cliente.get.return_value = mock_respuesta

    with caplog.at_level("WARNING"):
        ProductoRepositorio(
            base_url="http://catalogo:8080",
            client=mock_cliente,
        )

    assert any("Catálogo truncado a 10 páginas (15 totales disponibles)" in registro.message for registro in caplog.records)


def test_concurrencia_double_checked_locking_evita_llamadas_duplicadas():
    """
    Verifica que ante peticiones simultáneas con TTL expirado, el patrón
    double-checked locking hace que solo el primer hilo consulte a Catálogo,
    y los demás hilos reutilicen la caché sin volver a realizar peticiones HTTP.
    """
    import concurrent.futures
    import time

    mock_cliente = MagicMock(spec=httpx.Client)

    def mock_get(*args, **kwargs):
        time.sleep(0.05)
        respuesta = MagicMock(spec=httpx.Response)
        respuesta.json.return_value = MOCK_CATALOGO_PAGINA_1
        respuesta.raise_for_status.return_value = None
        return respuesta

    mock_cliente.get.side_effect = mock_get

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
        cache_ttl=0.1,
    )

    time.sleep(0.12)
    mock_cliente.get.reset_mock()

    def tarea_hilo():
        return repo.obtener_todos(forzar=False)

    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        resultados = list(executor.map(lambda numero_hilo: tarea_hilo(), range(5)))

    assert len(resultados) == 5
    for resultado in resultados:
        assert len(resultado) == 2

    assert mock_cliente.get.call_count == 1


def test_catalogo_caido_modo_sin_datos_respeta_ttl():
    """
    Verifica que si Catálogo está caído y no hay datos previos (sin_datos),
    las llamadas subsiguientes dentro del TTL no vuelven a intentar conectar
    inmediatamente, evitando saturar o retrasar cada petición.
    """
    mock_cliente = MagicMock(spec=httpx.Client)
    mock_cliente.get.side_effect = httpx.ConnectError("Catálogo no disponible")

    repo = ProductoRepositorio(
        base_url="http://catalogo:8080",
        client=mock_cliente,
        cache_ttl=60.0,
    )

    assert repo.fuente == "sin_datos"
    assert len(repo) == 0
    assert mock_cliente.get.call_count == 1

    mock_cliente.get.reset_mock()

    productos = repo.obtener_todos(forzar=False)
    assert productos == []
    assert repo.fuente == "sin_datos"
    assert mock_cliente.get.call_count == 0
