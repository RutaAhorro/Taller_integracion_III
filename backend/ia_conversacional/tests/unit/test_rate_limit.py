"""Pruebas del limitador de solicitudes del chat."""

from unittest.mock import AsyncMock

import pytest
from fastapi import FastAPI, HTTPException
from fastapi.testclient import TestClient

from app.auth import UsuarioAutenticado, obtener_usuario_actual
from app.productos import ProductoRepositorio
from app.proveedores.base import ResultadoExtraccion
from app.rate_limit import LimitadorSolicitudes
from app.routers.chat import crear_router_chat


class RelojManual:
    """Reloj controlable para probar ventanas sin esperar tiempo real."""

    def __init__(self):
        self.ahora = 0.0

    def __call__(self) -> float:
        return self.ahora

    def avanzar(self, segundos: float) -> None:
        self.ahora += segundos


def test_permite_solicitudes_hasta_el_limite():
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=2, ventana_segundos=60, reloj=reloj)

    limitador.verificar("usuario-a")
    limitador.verificar("usuario-a")


def test_rechaza_exceso_con_retry_after():
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    limitador.verificar("usuario-a")
    reloj.avanzar(10)

    with pytest.raises(HTTPException) as excepcion:
        limitador.verificar("usuario-a")

    assert excepcion.value.status_code == 429
    assert excepcion.value.headers["Retry-After"] == "50"


def test_aislamiento_estricto_entre_usuarios():
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=lambda: 0)
    limitador.verificar("usuario-a")
    limitador.verificar("usuario-b")

    with pytest.raises(HTTPException):
        limitador.verificar("usuario-a")


def test_se_recupera_al_superar_la_ventana():
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    limitador.verificar("usuario-a")
    reloj.avanzar(60)

    limitador.verificar("usuario-a")


def test_purgar_inactivos_libera_memoria():
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=2, ventana_segundos=60, reloj=reloj)
    limitador.verificar("usuario-activo")
    limitador.verificar("usuario-inactivo")

    # Avanzamos tiempo para que usuario-inactivo expire, pero usuario-activo renueva
    reloj.avanzar(70)
    limitador.verificar("usuario-activo")

    assert "usuario-inactivo" in limitador._solicitudes
    limitador.purgar_inactivos()
    assert "usuario-inactivo" not in limitador._solicitudes
    assert "usuario-activo" in limitador._solicitudes


def test_validacion_parametros_invalidos():
    with pytest.raises(ValueError, match="maximo_solicitudes"):
        LimitadorSolicitudes(maximo_solicitudes=0)

    with pytest.raises(ValueError, match="ventana_segundos"):
        LimitadorSolicitudes(maximo_solicitudes=5, ventana_segundos=-1)


def test_endpoint_retorna_429_sin_llamar_al_proveedor():
    reloj = RelojManual()
    limitador = LimitadorSolicitudes(maximo_solicitudes=1, ventana_segundos=60, reloj=reloj)
    proveedor = type("ProveedorPrueba", (), {})()
    proveedor.extraer_filtros = AsyncMock(
        return_value=ResultadoExtraccion(filtros=None, tiempo_extraccion_ms=1)
    )
    proveedor.generar_explicacion = AsyncMock()

    aplicacion = FastAPI()
    aplicacion.include_router(
        crear_router_chat(ProductoRepositorio("http://catalogo:8080"), proveedor, limitador)
    )
    aplicacion.dependency_overrides[obtener_usuario_actual] = lambda: UsuarioAutenticado(
        user_id="usuario-a", rol="registrado"
    )
    cliente = TestClient(aplicacion)

    assert cliente.post("/api/v1/chat", json={"mensaje": "hola"}).status_code == 200
    respuesta = cliente.post("/api/v1/chat", json={"mensaje": "hola"})

    assert respuesta.status_code == 429
    assert respuesta.headers["Retry-After"] == "60"
    proveedor.extraer_filtros.assert_awaited_once()
