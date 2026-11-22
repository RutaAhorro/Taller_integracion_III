"""
Punto de entrada del servicio (`uvicorn app.main:app`, puerto 8002).

Solo arma las piezas: crea la aplicación FastAPI, el repositorio de Catálogo
y los proveedores de IA con su configuración, y monta el chat y /health. La
lógica vive en `app/api`, `app/services`, `app/repositories` y `app/domain`.
"""

import sys
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app import config
from app.api.chat import crear_router_chat
from app.repositories.catalogo import ProductoRepositorio
from app.repositories.gemini import GeminiProveedor
from app.repositories.groq import GroqProveedor
from app.services.fallback import FallbackProveedor

# ===== Codificación UTF-8 =====


def configurar_consola_utf8() -> None:
    """Fuerza UTF-8 en stdout/stderr para que tildes y ñ no rompan los logs en consolas Windows."""
    for flujo in (sys.stdout, sys.stderr):
        # Algunos flujos sustituidos (p. ej. por pytest) no tienen reconfigure.
        if hasattr(flujo, "reconfigure"):
            flujo.reconfigure(encoding="utf-8", errors="replace")


class RespuestaUTF8(JSONResponse):
    """Respuesta JSON que declara el charset para que los clientes no adivinen la codificación."""

    media_type = "application/json; charset=utf-8"


configurar_consola_utf8()


# ===== Dependencias =====

repositorio_productos = ProductoRepositorio()

proveedor_respaldo = FallbackProveedor(
    proveedores=[
        GeminiProveedor(clave_api=config.GEMINI_API_KEY, url_endpoint=config.GEMINI_ENDPOINT),
        GroqProveedor(
            clave_api=config.GROQ_API_KEY,
            url_endpoint=config.GROQ_ENDPOINT,
            modelo_extraccion=config.GROQ_MODEL_EXTRACCION,
            modelo_explicacion=config.GROQ_MODEL_EXPLICACION,
        ),
    ],
)


# ===== Aplicación =====


@asynccontextmanager
async def ciclo_de_vida(aplicacion: FastAPI):
    """Valida la configuración y precarga el catálogo antes de aceptar peticiones."""
    config.validar_configuracion()
    repositorio_productos.cargar_productos()
    yield


app = FastAPI(title="IA Conversacional", default_response_class=RespuestaUTF8, lifespan=ciclo_de_vida)

app.add_middleware(
    CORSMiddleware,
    allow_origins=config.obtener_origenes_cors(),
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health", tags=["health"])
def consultar_salud():
    """Liveness sin red: informa cuántos productos hay en memoria y de qué fuente vienen."""
    return {
        "status": "ok",
        "service": "ia_conversacional",
        "productos_cargados": len(repositorio_productos),
        "fuente": repositorio_productos.fuente,
    }


app.include_router(crear_router_chat(repositorio=repositorio_productos, proveedor=proveedor_respaldo))
