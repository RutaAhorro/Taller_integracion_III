"""Errores HTTP con el mismo envoltorio `estado/mensaje/detalle` de los servicios Go."""

from __future__ import annotations

import logging

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

log = logging.getLogger("motor_rutas")


class ErrorAPI(Exception):
    def __init__(self, estado: int, mensaje: str, detalle: str | None = None) -> None:
        super().__init__(mensaje)
        self.estado, self.mensaje, self.detalle = estado, mensaje, detalle


def respuesta_error(estado: int, mensaje: str, detalle: str | None = None, cabeceras: dict[str, str] | None = None) -> JSONResponse:
    cuerpo: dict[str, object] = {"estado": estado, "mensaje": mensaje}
    if detalle:
        cuerpo["detalle"] = detalle
    return JSONResponse(cuerpo, status_code=estado, headers=cabeceras)


def registrar_manejadores(app: FastAPI) -> None:
    @app.exception_handler(ErrorAPI)
    async def _error_api(_: Request, error: ErrorAPI) -> JSONResponse:
        return respuesta_error(error.estado, error.mensaje, error.detalle)

    # Sólo publica la ruta del campo y el tipo de error, nunca el valor recibido.
    @app.exception_handler(RequestValidationError)
    async def _validacion(_: Request, error: RequestValidationError) -> JSONResponse:
        campos = []
        for problema in error.errors():
            ruta = ".".join(str(parte) for parte in problema.get("loc", ()) if parte != "body")
            campos.append(f"{ruta or 'cuerpo'}: {problema.get('msg', 'valor inválido')}")
        return respuesta_error(400, "Solicitud inválida.", "; ".join(campos)[:500])

    @app.exception_handler(StarletteHTTPException)
    async def _http(request: Request, error: StarletteHTTPException) -> JSONResponse:
        if error.status_code == 404:
            return respuesta_error(404, "La ruta o recurso solicitado no existe.")
        if error.status_code == 405:
            return respuesta_error(405, "El método HTTP utilizado no está permitido para este recurso.", cabeceras=dict(error.headers or {}))
        if error.status_code == 400:
            # FastAPI lo lanza cuando el cuerpo no es JSON válido (por ejemplo, UTF-8 corrupto).
            return respuesta_error(400, "Solicitud inválida.", "El cuerpo no es JSON válido.")
        return respuesta_error(error.status_code, "Solicitud no procesable.")

    @app.exception_handler(Exception)
    async def _interno(request: Request, error: Exception) -> JSONResponse:
        log.error("error_interno ruta=%s tipo=%s", request.url.path, type(error).__name__, exc_info=error)
        return respuesta_error(500, "Ocurrió un error interno en el servidor.")
