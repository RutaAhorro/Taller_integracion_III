"""Pruebas del limitador de solicitudes del chat."""

from unittest.mock import AsyncMock

import pytest
from fastapi import FastAPI, HTTPException
from fastapi.testclient import TestClient

from app.api.chat import crear_router_chat
from app.api.sesion import LimitadorSolicitudes, UsuarioAutenticado, obtener_usuario_actual
from app.domain.proveedores import ResultadoExtraccion
from app.repositories.catalogo import ProductoRepositorio


class RelojManual:
    """Reloj controlable para probar ventanas sin esperar tiempo real."""

    def __init__(self) -> None:
        """Inicializa el reloj en el segundo cero."""
        self.ahora = 0.0

    def __call__(self) -> float:
        """Retorna la marca de tiempo actual del reloj simulado."""
        return self.ahora

    def avanzar(self, segundos: float) -> None:
        """Avanza el reloj simulado la cantidad especificada de segundos."""
        self.ahora += segundos


def test_permite_solicitudes_hasta_el_limite() -> None:
    """Verifica que se permitan múltiples solicitudes consecutivas mientras no excedan la cuota."""
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=2, ventana_segundos=60, reloj=reloj)

    limitador.verificar("usuario-a")
    limitador.verificar("usuario-a")


def test_rechaza_exceso_con_retry_after() -> None:
    """Verifica que se lance HTTP 429 con cabecera Retry-After precisa al agotar la cuota."""
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    limitador.verificar("usuario-a")
    reloj.avanzar(10)

    with pytest.raises(HTTPException) as excepcion:
        limitador.verificar("usuario-a")

    assert excepcion.value.status_code == 429
    assert excepcion.value.headers["Retry-After"] == "50"


def test_aislamiento_estricto_entre_usuarios() -> None:
    """Verifica que el consumo de cuota de un usuario no afecte el límite de otro usuario."""
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=lambda: 0)
    limitador.verificar("usuario-a")
    limitador.verificar("usuario-b")

    with pytest.raises(HTTPException):
        limitador.verificar("usuario-a")


def test_se_recupera_al_superar_la_ventana() -> None:
    """Verifica que un usuario bloqueado recupere su cuota una vez transcurrida la ventana."""
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    limitador.verificar("usuario-a")
    reloj.avanzar(60)

    limitador.verificar("usuario-a")


def test_purga_oportunista_en_verificar() -> None:
    """Verifica que verificar ejecute automáticamente la purga tras alcanzar el intervalo configurado."""
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(
        maximo_solicitudes=10,
        ventana_segundos=60,
        reloj=reloj,
        intervalo_purga=4,
    )
    # Llamada 1 (contador = 1)
    limitador.verificar("usuario-antiguo")
    reloj.avanzar(70)

    # Llamada 2 (contador = 2 < 4)
    limitador.verificar("usuario-nuevo")
    assert "usuario-antiguo" in limitador._solicitudes

    # Llamada 3 (contador = 3 < 4)
    limitador.verificar("usuario-nuevo")
    assert "usuario-antiguo" in limitador._solicitudes

    # Llamada 4 (contador = 4 >= intervalo_purga -> purga ejecutada)
    limitador.verificar("usuario-nuevo")
    assert "usuario-antiguo" not in limitador._solicitudes
    assert "usuario-nuevo" in limitador._solicitudes


def test_validacion_parametros_invalidos() -> None:
    """Verifica que el constructor rechace valores negativos o nulos en sus argumentos."""
    with pytest.raises(ValueError, match="maximo_solicitudes"):
        LimitadorSolicitudes(maximo_solicitudes=0, ventana_segundos=60)

    with pytest.raises(ValueError, match="ventana_segundos"):
        LimitadorSolicitudes(maximo_solicitudes=5, ventana_segundos=-1)

    with pytest.raises(ValueError, match="intervalo_purga"):
        LimitadorSolicitudes(maximo_solicitudes=5, ventana_segundos=60, intervalo_purga=0)


def test_endpoint_retorna_429_sin_llamar_al_proveedor() -> None:
    """Verifica que el endpoint de FastAPI responda 429 sin invocar al proveedor LLM si excede la cuota."""
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    proveedor = type("ProveedorPrueba", (), {})()
    proveedor.extraer_filtros = AsyncMock(return_value=ResultadoExtraccion(filtros=None, tiempo_extraccion_ms=1))
    proveedor.generar_explicacion = AsyncMock()

    aplicacion = FastAPI()
    aplicacion.include_router(crear_router_chat(ProductoRepositorio("http://catalogo:8080"), proveedor, limitador))
    aplicacion.dependency_overrides[obtener_usuario_actual] = lambda: UsuarioAutenticado(user_id="usuario-a")
    cliente = TestClient(aplicacion)

    assert cliente.post("/api/v1/chat", json={"mensaje": "hola"}).status_code == 200
    respuesta = cliente.post("/api/v1/chat", json={"mensaje": "hola"})

    assert respuesta.status_code == 429
    assert respuesta.headers["Retry-After"] == "60"
    proveedor.extraer_filtros.assert_awaited_once()
