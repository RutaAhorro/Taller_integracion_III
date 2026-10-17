"""
Tests unitarios para ProductoRepositorio (SUP-246).

Verifica:
1. Conexión HTTP interna exitosa hacia el microservicio de Catálogo.
2. Filtrado de productos vigentes con stock (en_stock=true).
3. Paginación automática en Catálogo.
4. Resiliencia y degradación elegante ante timeouts, caídas de red o errores 5xx (fallback a productos.json).
5. Preservación del estado en memoria si ocurre una falla transitoria posterior.
6. Compatibilidad determinista con filtros.py.
"""

import json
from pathlib import Path
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


@pytest.fixture
def temp_fallback_file(tmp_path: Path) -> Path:
    """Crea un archivo mock temporal para simular productos.json."""
    fallback_data = [
        {"id": 101, "nombre": "Fallback Arroz 1kg", "precio": 1290, "categoria": "abarrotes", "distancia_km": 1.0},
        {"id": 102, "nombre": "Fallback Fideos 400g", "precio": 890, "categoria": "abarrotes", "distancia_km": 1.5},
    ]
    file_path = tmp_path / "productos_fallback.json"
    file_path.write_text(json.dumps(fallback_data), encoding="utf-8")
    return file_path


def test_carga_exitosa_desde_catalogo_http(temp_fallback_file: Path):
    """Verifica la carga exitosa de productos desde el endpoint interno de Catálogo."""
    mock_client = MagicMock(spec=httpx.Client)
    mock_resp = MagicMock(spec=httpx.Response)
    mock_resp.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_resp.raise_for_status.return_value = None
    mock_client.get.return_value = mock_resp

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()

    # Verifica llamada HTTP interna
    mock_client.get.assert_called_with(
        "http://catalogo:8080/api/v1/productos",
        params={"en_stock": "true", "page": 1, "limit": 100},
    )

    # Solo deben estar los 2 productos con stock vigente (el 3ero tenía en_stock=False)
    assert len(productos) == 2
    assert productos[0]["id"] == "prod-uuid-1"
    assert productos[0]["nombre"] == "Leche Entera 1L"
    assert productos[0]["precio"] == 1190.0
    assert productos[0]["en_stock"] is True
    # Clave distancia_km normalizada por compatibilidad
    assert "distancia_km" in productos[0]
    assert productos[1]["id"] == "prod-uuid-2"


def test_paginacion_multiples_paginas(temp_fallback_file: Path):
    """Verifica que si Catálogo tiene múltiples páginas, se consultan y agregan todas."""
    mock_client = MagicMock(spec=httpx.Client)

    def side_effect(url, params):
        mock_r = MagicMock(spec=httpx.Response)
        mock_r.raise_for_status.return_value = None
        if params.get("page") == 1:
            mock_r.json.return_value = MOCK_CATALOGO_MULTI_PAG_1
        else:
            mock_r.json.return_value = MOCK_CATALOGO_MULTI_PAG_2
        return mock_r

    mock_client.get.side_effect = side_effect

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()
    assert len(productos) == 2
    nombres = [p["nombre"] for p in productos]
    assert "Manzana Royal Gala" in nombres
    assert "Plátano Granel" in nombres


def test_degradacion_timeout_aplica_fallback(temp_fallback_file: Path):
    """Verifica que ante un TimeoutException en Catálogo, se activa el fallback sin lanzar error."""
    mock_client = MagicMock(spec=httpx.Client)
    mock_client.get.side_effect = httpx.TimeoutException("Petición agotó el tiempo de espera")

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()
    assert len(productos) == 2
    assert productos[0]["nombre"] == "Fallback Arroz 1kg"
    assert productos[1]["nombre"] == "Fallback Fideos 400g"


def test_degradacion_error_conexion_aplica_fallback(temp_fallback_file: Path):
    """Verifica que si el contenedor de Catálogo no está disponible (ConnectError), se activa fallback."""
    mock_client = MagicMock(spec=httpx.Client)
    mock_client.get.side_effect = httpx.ConnectError("Connection refused")

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()
    assert len(productos) == 2
    assert productos[0]["id"] == 101


def test_degradacion_error_http_500_aplica_fallback(temp_fallback_file: Path):
    """Verifica que ante error HTTP 500 de Catálogo, se maneja limpiamente y recurre a fallback."""
    mock_client = MagicMock(spec=httpx.Client)
    mock_resp = MagicMock(spec=httpx.Response)
    mock_resp.status_code = 500
    mock_resp.raise_for_status.side_effect = httpx.HTTPStatusError(
        "500 Internal Server Error",
        request=MagicMock(),
        response=mock_resp,
    )
    mock_client.get.return_value = mock_resp

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()
    assert len(productos) == 2
    assert productos[0]["nombre"] == "Fallback Arroz 1kg"


def test_preserva_cache_en_memoria_tras_falla_posterior(temp_fallback_file: Path):
    """
    Si el repositorio ya cargó productos de Catálogo exitosamente y luego una
    recarga posterior falla, debe mantener los productos válidos en memoria.
    """
    mock_client = MagicMock(spec=httpx.Client)
    mock_resp = MagicMock(spec=httpx.Response)
    mock_resp.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_resp.raise_for_status.return_value = None
    mock_client.get.return_value = mock_resp

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    # Primera carga exitosa
    assert len(repo.obtener_todos()) == 2

    # Segunda carga falla (ej: Catálogo temporalmente no disponible)
    mock_client.get.side_effect = httpx.TimeoutException("Catálogo caído temporalmente")
    productos_despues_de_fallo = repo.obtener_todos(forzar=True)

    # Debe conservar los 2 productos previos de Catálogo, no sobrescribirlos con fallback
    assert len(productos_despues_de_fallo) == 2
    assert productos_despues_de_fallo[0]["id"] == "prod-uuid-1"


def test_compatibilidad_con_filtros_deterministas(temp_fallback_file: Path):
    """
    Verifica que la salida de ProductoRepositorio es procesada correctamente
    por la lógica determinista de filtros.py sin errores ni excepciones.
    """
    mock_client = MagicMock(spec=httpx.Client)
    mock_resp = MagicMock(spec=httpx.Response)
    mock_resp.json.return_value = MOCK_CATALOGO_PAGINA_1
    mock_resp.raise_for_status.return_value = None
    mock_client.get.return_value = mock_resp

    repo = ProductoRepositorio(
        ruta_fallback=temp_fallback_file,
        base_url="http://catalogo:8080",
        client=mock_client,
    )

    productos = repo.obtener_todos()

    # Filtro por categoría lácteos y precio máximo 1000
    filtros = {"categoria": "lacteos", "precio_max": 1000}
    filtrados = filtrar_productos(productos, filtros)

    # Solo Yogurt ($450) debe calzar, Leche ($1190) supera el precio_max
    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Yogurt Batido Vainilla"
    assert filtrados[0]["precio"] == 450.0


def test_fastapi_health_check_sin_crashear_modo_degradado():
    """Verifica que el microservicio FastAPI responde 200 en /health incluso con Catálogo caído."""
    from fastapi.testclient import TestClient
    from app.main import app

    client = TestClient(app)
    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "ok"
    assert data["service"] == "ia_conversacional"
    assert data["productos_cargados"] > 0


def test_fastapi_endpoint_productos_devuelve_catalogo():
    """Verifica que el endpoint /productos expone los productos disponibles."""
    from fastapi.testclient import TestClient
    from app.main import app

    client = TestClient(app)
    resp = client.get("/productos")
    assert resp.status_code == 200
    productos = resp.json()
    assert isinstance(productos, list)
    assert len(productos) > 0

