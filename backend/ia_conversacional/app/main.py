"""
Punto de entrada del backend.

Este archivo solo arma las piezas: crea la app FastAPI, instancia el
repositorio de productos y los proveedores de IA con su configuracion, y
los conecta a los routers. Toda la logica real (filtrado, llamadas a las
APIs, manejo de errores, prompts) vive en app/ -- ver el README para el
detalle de que hace cada modulo.
"""

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app import config
from app.ia_definiciones import definicion_funcion_filtros, definicion_funcion_filtros_gemini
from app.productos import ProductoRepositorio
from app.proveedores.gemini_proveedor import GeminiProveedor
from app.proveedores.groq_proveedor import GroqProveedor
from app.proveedores.fallback_proveedor import FallbackProveedor
from app.routers.chat import crear_router_chat
from app.utils import RespuestaUTF8, fix_encoding_consola

fix_encoding_consola()


@asynccontextmanager
async def lifespan(app_instancia: FastAPI):
    config.validar_configuracion()
    yield


app = FastAPI(
    title="IA Conversacional",
    default_response_class=RespuestaUTF8,
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=config.obtener_origenes_cors(),
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

repositorio_productos = ProductoRepositorio(
    base_url=config.CATALOGO_INTERNAL_URL,
    timeout=config.CATALOGO_TIMEOUT,
    cache_ttl=config.CATALOGO_CACHE_TTL,
    page_limit=config.CATALOGO_PAGE_LIMIT,
    max_paginas=config.CATALOGO_MAX_PAGINAS,
)

proveedor_groq = GroqProveedor(
    api_key=config.GROQ_API_KEY,
    endpoint=config.GROQ_ENDPOINT,
    modelo_extraccion=config.GROQ_MODEL_EXTRACCION,
    modelo_explicacion=config.GROQ_MODEL_EXPLICACION,
    definicion_funcion=definicion_funcion_filtros,
)

proveedor_gemini = GeminiProveedor(
    api_key=config.GEMINI_API_KEY,
    endpoint=config.GEMINI_ENDPOINT,
    modelo=config.GEMINI_MODEL,
    definicion_funcion=definicion_funcion_filtros_gemini,
)

proveedor_fallback = FallbackProveedor(
    proveedores=[
        proveedor_gemini,
        proveedor_groq,
    ],
    max_retries_primario=config.FALLBACK_RETRIES_PRIMARIO,
    max_retries_secundario=config.FALLBACK_RETRIES_SECUNDARIO,
    backoff_base=config.FALLBACK_BACKOFF_BASE,
)

# --------------------------------------------------------------------------
# Healthcheck & Root
# --------------------------------------------------------------------------

@app.get("/health", tags=["health"])
def health_check():
    return {
        "status": "ok",
        "service": "ia_conversacional",
        "productos_cargados": len(repositorio_productos),
        "fuente": repositorio_productos.fuente,
    }

# --------------------------------------------------------------------------
# Routers
# --------------------------------------------------------------------------

app.include_router(
    crear_router_chat(
        repositorio=repositorio_productos,
        proveedor_fallback=proveedor_fallback,
    )
)
