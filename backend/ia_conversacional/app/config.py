"""
Configuracion central: carga el .env, define rutas y agrupa todas las
constantes de los proveedores de IA (endpoints, modelos, limites y timeouts).
"""

import logging
import os
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent.parent
PROJECT_ROOT = BASE_DIR.parent.parent

# Carga de variables de entorno (.env local con precedencia sobre la raiz del proyecto)
load_dotenv(BASE_DIR / ".env", override=False)
load_dotenv(PROJECT_ROOT / ".env", override=False)


def _leer_env_str(nombre_variable: str, valor_defecto: str) -> str:
    """Lee una variable de texto, retornando el default si no existe o viene vacia."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    return valor_crudo.strip()


def _leer_env_float(nombre_variable: str, valor_defecto: float) -> float:
    """Lee una variable decimal de entorno; tolera vacio con default y falla claro ante formato invalido."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    try:
        return float(valor_crudo)
    except ValueError as error_parseo:
        raise RuntimeError(
            f"Configuracion invalida: {nombre_variable} debe ser un decimal valido, recibido: '{valor_crudo}'."
        ) from error_parseo


def _leer_env_int(nombre_variable: str, valor_defecto: int) -> int:
    """Lee una variable entera de entorno; tolera vacio con default y falla claro ante formato invalido."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    try:
        return int(valor_crudo)
    except ValueError as error_parseo:
        raise RuntimeError(
            f"Configuracion invalida: {nombre_variable} debe ser un entero valido, recibido: '{valor_crudo}'."
        ) from error_parseo


# --------------------------------------------------------------------------
# Seguridad y Autenticacion (SUP-345)
# --------------------------------------------------------------------------
JWT_SECRET = os.getenv("JWT_SECRET")

# --------------------------------------------------------------------------
# CORS (SUP-345)
# --------------------------------------------------------------------------
CORS_ALLOWED_ORIGINS = (
    os.getenv("IA_CORS_ALLOWED_ORIGINS", "").strip()
    or os.getenv("CORS_ALLOWED_ORIGINS", "").strip()
    or "http://localhost:3000,http://localhost:5173"
)

# --------------------------------------------------------------------------
# Catalogo interno (SUP-246 / RNF-05 / SUP-345)
# --------------------------------------------------------------------------
CATALOGO_INTERNAL_URL = _leer_env_str("CATALOGO_INTERNAL_URL", "http://catalogo:8080").rstrip("/")
CATALOGO_TIMEOUT = _leer_env_float("CATALOGO_TIMEOUT", 5.0)
CATALOGO_CACHE_TTL = _leer_env_float("IA_CATALOGO_CACHE_TTL", 60.0)
CATALOGO_PAGE_LIMIT = _leer_env_int("IA_CATALOGO_PAGE_LIMIT", 100)
CATALOGO_MAX_PAGINAS = _leer_env_int("IA_CATALOGO_MAX_PAGINAS", 10)

# --------------------------------------------------------------------------
# Proveedor Groq (SUP-345)
# --------------------------------------------------------------------------
GROQ_API_KEY = os.getenv("GROQ_API_KEY")
GROQ_ENDPOINT = _leer_env_str("GROQ_ENDPOINT", "https://api.groq.com/openai/v1/chat/completions")
GROQ_MODEL_EXTRACCION = _leer_env_str("GROQ_MODEL_EXTRACCION", "llama-3.1-8b-instant")
GROQ_MODEL_EXPLICACION = _leer_env_str("GROQ_MODEL_EXPLICACION", "llama-3.3-70b-versatile")
GROQ_TIMEOUT = _leer_env_float("IA_GROQ_TIMEOUT", 15.0)
GROQ_MAX_TOKENS_EXTRACCION = _leer_env_int("IA_GROQ_MAX_TOKENS_EXTRACCION", 512)
GROQ_MAX_TOKENS_EXPLICACION = _leer_env_int("IA_GROQ_MAX_TOKENS_EXPLICACION", 200)
GROQ_FREQUENCY_PENALTY = _leer_env_float("IA_GROQ_FREQUENCY_PENALTY", 0.4)

# --------------------------------------------------------------------------
# Proveedor Gemini (SUP-345)
# --------------------------------------------------------------------------
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
GEMINI_MODEL = _leer_env_str("GEMINI_MODEL", "gemini-3.1-flash-lite")
GEMINI_API_BASE_URL = _leer_env_str(
    "GEMINI_API_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"
).rstrip("/")
GEMINI_ENDPOINT = _leer_env_str(
    "GEMINI_ENDPOINT",
    f"{GEMINI_API_BASE_URL}/models/{GEMINI_MODEL}:generateContent",
)
GEMINI_TIMEOUT = _leer_env_float("IA_GEMINI_TIMEOUT", 15.0)
GEMINI_THINKING_LEVEL = _leer_env_str("IA_GEMINI_THINKING_LEVEL", "low")
GEMINI_MAX_TOKENS_EXTRACCION = _leer_env_int("IA_GEMINI_MAX_TOKENS_EXTRACCION", 1200)
GEMINI_MAX_TOKENS_EXPLICACION = _leer_env_int("IA_GEMINI_MAX_TOKENS_EXPLICACION", 2048)

# --------------------------------------------------------------------------
# Fallback y Resiliencia (SUP-345)
# --------------------------------------------------------------------------
FALLBACK_RETRIES_PRIMARIO = _leer_env_int("IA_FALLBACK_RETRIES_PRIMARIO", 4)
FALLBACK_RETRIES_SECUNDARIO = _leer_env_int("IA_FALLBACK_RETRIES_SECUNDARIO", 1)
FALLBACK_BACKOFF_BASE = _leer_env_float("IA_FALLBACK_BACKOFF_BASE", 1.5)
FALLBACK_JITTER_MAX = _leer_env_float("IA_FALLBACK_JITTER_MAX", 0.5)

# --------------------------------------------------------------------------
# Limites de Chat y Explicacion
# --------------------------------------------------------------------------
CHAT_MAX_MENSAJE_LEN = _leer_env_int("IA_CHAT_MAX_MENSAJE_LEN", 1000)
EXPLICACION_MAX_PRODUCTOS = _leer_env_int("IA_EXPLICACION_MAX_PRODUCTOS", 8)


def obtener_origenes_cors() -> list[str]:
    """Retorna la lista limpia de origenes permitidos para CORS."""
    return [
        origen.strip()
        for origen in CORS_ALLOWED_ORIGINS.split(",")
        if origen.strip()
    ]


def validar_configuracion() -> None:
    """
    Valida la configuracion al arrancar la aplicacion (fail-fast).
    Lanza RuntimeError si faltan parametros criticos o si contienen valores invalidos.
    """
    if not JWT_SECRET or len(JWT_SECRET) < 32:
        raise RuntimeError("JWT_SECRET debe estar configurado y tener al menos 32 caracteres.")

    partes_url = urlsplit(CATALOGO_INTERNAL_URL)
    if partes_url.scheme not in ("http", "https"):
        raise RuntimeError(
            f"CATALOGO_INTERNAL_URL esquema no soportado: '{partes_url.scheme}'. Debe ser http o https."
        )
    if not partes_url.netloc:
        raise RuntimeError("CATALOGO_INTERNAL_URL debe incluir un host o netloc valido.")
    if partes_url.username or partes_url.password:
        raise RuntimeError("CATALOGO_INTERNAL_URL no debe incluir credenciales de usuario o contrasena.")
    if partes_url.path not in ("", "/"):
        raise RuntimeError(f"CATALOGO_INTERNAL_URL no debe incluir path: '{partes_url.path}'.")
    if partes_url.query != "" or partes_url.fragment != "":
        raise RuntimeError("CATALOGO_INTERNAL_URL no debe incluir query parameters ni fragmentos.")

    origenes = obtener_origenes_cors()
    if not origenes:
        raise RuntimeError("Debe configurarse al menos un origen valido en IA_CORS_ALLOWED_ORIGINS.")
    for origen in origenes:
        if "*" in origen:
            raise RuntimeError(f"Origen CORS no puede contener comodines '*': '{origen}'.")
        if origen.endswith("/"):
            raise RuntimeError(f"Origen CORS no puede terminar en barra '/': '{origen}'.")

    if CATALOGO_TIMEOUT <= 0:
        raise RuntimeError("CATALOGO_TIMEOUT debe ser mayor a 0.")
    if CATALOGO_PAGE_LIMIT <= 0:
        raise RuntimeError("CATALOGO_PAGE_LIMIT debe ser mayor a 0.")
    if CATALOGO_MAX_PAGINAS <= 0:
        raise RuntimeError("CATALOGO_MAX_PAGINAS debe ser mayor a 0.")
    if GROQ_TIMEOUT <= 0:
        raise RuntimeError("GROQ_TIMEOUT debe ser mayor a 0.")
    if GROQ_MAX_TOKENS_EXTRACCION <= 0:
        raise RuntimeError("GROQ_MAX_TOKENS_EXTRACCION debe ser mayor a 0.")
    if GROQ_MAX_TOKENS_EXPLICACION <= 0:
        raise RuntimeError("GROQ_MAX_TOKENS_EXPLICACION debe ser mayor a 0.")
    if GEMINI_TIMEOUT <= 0:
        raise RuntimeError("GEMINI_TIMEOUT debe ser mayor a 0.")
    if GEMINI_MAX_TOKENS_EXTRACCION <= 0:
        raise RuntimeError("GEMINI_MAX_TOKENS_EXTRACCION debe ser mayor a 0.")
    if GEMINI_MAX_TOKENS_EXPLICACION <= 0:
        raise RuntimeError("GEMINI_MAX_TOKENS_EXPLICACION debe ser mayor a 0.")
    if FALLBACK_BACKOFF_BASE <= 0:
        raise RuntimeError("FALLBACK_BACKOFF_BASE debe ser mayor a 0.")
    if CHAT_MAX_MENSAJE_LEN <= 0:
        raise RuntimeError("CHAT_MAX_MENSAJE_LEN debe ser mayor a 0.")
    if EXPLICACION_MAX_PRODUCTOS <= 0:
        raise RuntimeError("EXPLICACION_MAX_PRODUCTOS debe ser mayor a 0.")

    if CATALOGO_CACHE_TTL < 0:
        raise RuntimeError("CATALOGO_CACHE_TTL debe ser mayor o igual a 0.")
    if FALLBACK_RETRIES_PRIMARIO < 0:
        raise RuntimeError("FALLBACK_RETRIES_PRIMARIO debe ser mayor o igual a 0.")
    if FALLBACK_RETRIES_SECUNDARIO < 0:
        raise RuntimeError("FALLBACK_RETRIES_SECUNDARIO debe ser mayor o igual a 0.")
    if FALLBACK_JITTER_MAX < 0:
        raise RuntimeError("FALLBACK_JITTER_MAX debe ser mayor o igual a 0.")

    if not GEMINI_API_KEY and not GROQ_API_KEY:
        logging.getLogger(__name__).warning(
            "Ni GEMINI_API_KEY ni GROQ_API_KEY estan configuradas. Los endpoints con IA fallaran."
        )
