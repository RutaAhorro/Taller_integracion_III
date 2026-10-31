"""Motor de rutas: dueño de rutas.* (listas de compra y, más adelante, optimizaciones)."""

from __future__ import annotations

import asyncio
import json
import logging
import re
import secrets
import sys
import time
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, Response

from app import db
from app.api.router import api_router
from app.config import Configuracion, cargar
from app.errores import registrar_manejadores, respuesta_error
from app.listas.repositorio import RepositorioListas, RepositorioListasPostgres

log = logging.getLogger("motor_rutas")
_REQUEST_ID = re.compile(r"^[a-zA-Z0-9._-]{1,128}$")


@asynccontextmanager
async def _almacenamiento(app: FastAPI, config: Configuracion) -> AsyncIterator[None]:
    pool = await db.abrir_pool(config.db_url, config.db_pool_max)
    try:
        await db.verificar_permisos(pool)
        app.state.repositorio = RepositorioListasPostgres(pool)
        app.state.ping = lambda: db.ping(pool)
        yield
    finally:
        await pool.close()


def create_app(config: Configuracion | None = None, repositorio: RepositorioListas | None = None) -> FastAPI:
    """Las pruebas inyectan configuración y repositorio; producción los lee del entorno."""

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        app.state.config = config or cargar()
        if repositorio is not None:
            app.state.repositorio = repositorio
            yield
            return
        async with _almacenamiento(app, app.state.config):
            yield

    app = FastAPI(title="Motor de rutas", version="1.0.0", lifespan=lifespan)
    app.state.ping = None
    registrar_manejadores(app)

    @app.middleware("http")
    async def cors(request: Request, call_next):
        origen = request.headers.get("origin")
        if origen is not None:
            if origen not in request.app.state.config.origenes_cors:
                respuesta = respuesta_error(403, "Origen no permitido.")
                respuesta.headers["Vary"] = "Origin"
                return respuesta
            if request.method == "OPTIONS":
                respuesta = Response(status_code=204)
            else:
                respuesta = await call_next(request)
            respuesta.headers.update({
                "Access-Control-Allow-Origin": origen,
                "Access-Control-Allow-Credentials": "true",
                "Access-Control-Allow-Headers": "Content-Type, X-Request-ID",
                "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
            })
        elif request.method == "OPTIONS":
            respuesta = Response(status_code=204)
        else:
            respuesta = await call_next(request)
        respuesta.headers.append("Vary", "Origin")
        return respuesta

    # Registrada al final para envolver también CORS: toda respuesta lleva X-Request-ID.
    @app.middleware("http")
    async def observar(request: Request, call_next):
        request_id = request.headers.get("x-request-id", "")
        if not _REQUEST_ID.match(request_id):
            request_id = secrets.token_hex(16)
        inicio = time.monotonic()
        respuesta = await call_next(request)
        respuesta.headers["X-Request-ID"] = request_id
        ruta = getattr(request.scope.get("route"), "path", "unmatched")
        log.info(json.dumps({"msg": "http_request", "request_id": request_id, "method": request.method, "path": ruta,
                             "status": respuesta.status_code, "duration_ms": int((time.monotonic() - inicio) * 1000)}))
        return respuesta

    @app.get("/_rutas/live", include_in_schema=False)
    async def live() -> dict:
        return {"status": "ok", "service": "motor_rutas"}

    @app.get("/_rutas/ready", include_in_schema=False)
    @app.get("/health", include_in_schema=False)
    async def ready(request: Request) -> JSONResponse:
        ping = request.app.state.ping
        sano = ping is None or await ping()
        cuerpo = {"status": "ok" if sano else "error", "service": "motor_rutas", "checks": {"database": "ok" if sano else "error"}}
        return JSONResponse(cuerpo, status_code=200 if sano else 503)

    app.include_router(api_router)
    return app


def _configurar_logs() -> None:
    logging.basicConfig(stream=sys.stdout, level=logging.INFO, format="%(message)s")
    # Clientes HTTP/SQL sólo registran advertencias: cada petición ya queda en http_request.
    for ruidoso in ("httpx", "psycopg.pool"):
        logging.getLogger(ruidoso).setLevel(logging.WARNING)


if sys.platform == "win32":
    # psycopg async no funciona con el ProactorEventLoop por defecto de Windows (sólo afecta desarrollo local).
    asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
_configurar_logs()
app = create_app()
