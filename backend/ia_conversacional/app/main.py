"""
Punto de entrada del backend.

Este archivo solo arma las piezas: crea la app FastAPI, instancia el
repositorio de productos y los proveedores de IA con su configuracion, y
los conecta a los routers. Toda la logica real (filtrado, llamadas a las
APIs, manejo de errores, prompts) vive en app/ -- ver el README para el
detalle de que hace cada modulo.

no me funciona la z y derrepente se me olvida usar el teclado en pantalla perdon
"""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app import config
from app.ia_definiciones import definicion_funcion_filtros, definicion_funcion_filtros_gemini
from app.productos import ProductoRepositorio
from app.proveedores.gemini_proveedor import GeminiProveedor
from app.proveedores.groq_proveedor import GroqProveedor
from app.proveedores.fallback_proveedor import FallbackProveedor
from app.routers.chat import crear_router_chat
from app.routers.productos import crear_router_productos
from app.utils import RespuestaUTF8, fix_encoding_consola

fix_encoding_consola()

app = FastAPI(
    title="Prototipo IA - Comparacion Groq vs Gemini",
    default_response_class=RespuestaUTF8,
)


origenes_cors = [
    origin.strip()
    for origin in config.CORS_ALLOWED_ORIGINS.split(",")
    if origin.strip()
]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origenes_cors,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


repositorio_productos = ProductoRepositorio(
    base_url=config.CATALOGO_INTERNAL_URL,
    timeout=config.CATALOGO_TIMEOUT,
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
    max_retries_primario=4,   
    max_retries_secundario=1,   
    backoff_base=1.5,           
    timeout=30.0,               
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

app.include_router(crear_router_productos(repositorio_productos))
app.include_router(
    crear_router_chat(
        repositorio=repositorio_productos,
        proveedor_fallback=proveedor_fallback,
        proveedor_groq=proveedor_groq,
        proveedor_gemini=proveedor_gemini,
    )
)