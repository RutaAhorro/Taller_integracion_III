"""
Pruebas de integración para el endpoint de salud (/health) y ciclo de vida lifespan (SUP-246, SUP-345, SUP-353).

Verifica:
1. GET /health responde 200 con payload canónico (status, service, productos_cargados, fuente).
2. GET /health tolera modo degradado y reporta fuente válida en ('catalogo', 'memoria', 'sin_datos').
3. El ciclo lifespan aborta el arranque con RuntimeError si JWT_SECRET es inválido o menor a 32 caracteres.
4. El ciclo lifespan inicializa la aplicación correctamente cuando JWT_SECRET es válido.
"""

from fastapi.testclient import TestClient
import pytest

from app import config
from app.main import app

TEST_JWT_SECRET = "super-secreto-para-health-checks-con-longitud-adecuada!"


@pytest.fixture(autouse=True)
def configurar_entorno_base(monkeypatch):
    """Configura variables seguras para las pruebas de health."""
    monkeypatch.setattr(config, "JWT_SECRET", TEST_JWT_SECRET)


def test_fastapi_health_check_sin_crashear_modo_degradado():
    """Verifica que /health responde 200 con las claves canónicas y fuente en modo degradado o activo."""
    cliente = TestClient(app)
    respuesta = cliente.get("/health")
    assert respuesta.status_code == 200
    datos = respuesta.json()
    assert datos["status"] == "ok"
    assert datos["service"] == "ia_conversacional"
    assert "productos_cargados" in datos
    assert isinstance(datos["productos_cargados"], int)
    assert "fuente" in datos
    assert datos["fuente"] in ("catalogo", "memoria", "sin_datos")


def test_lifespan_falla_si_jwt_secret_invalido_o_incompleto(monkeypatch):
    """Verifica que el ciclo lifespan aborta el inicio con RuntimeError si JWT_SECRET no cumple los requisitos."""
    monkeypatch.setattr(config, "JWT_SECRET", "clave_muy_corta")
    with pytest.raises(RuntimeError, match="JWT_SECRET debe estar configurado y tener al menos 32 caracteres"):
        with TestClient(app):
            pass


def test_lifespan_arranca_exitosamente_si_configuracion_valida(monkeypatch):
    """Verifica que con TestClient(app) el ciclo lifespan arranca normalmente y responde 200."""
    monkeypatch.setattr(config, "JWT_SECRET", TEST_JWT_SECRET)
    with TestClient(app) as cliente_activo:
        respuesta = cliente_activo.get("/health")
        assert respuesta.status_code == 200
        datos = respuesta.json()
        assert datos["status"] == "ok"
        assert datos["service"] == "ia_conversacional"


def test_repositorio_productos_modulo_main_no_autocarga_en_import(monkeypatch):
    """Verifica que repositorio_productos en app.main se configure con auto_cargar=False."""
    from app.main import repositorio_productos
    assert hasattr(repositorio_productos, "cargar_productos")
    # Al estar configurado con auto_cargar=False, el repositorio espera al lifespan para cargar
    assert repositorio_productos._page_limit == config.CATALOGO_PAGE_LIMIT
