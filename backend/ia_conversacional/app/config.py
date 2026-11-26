"""
Configuración central del servicio.

Lee el entorno una sola vez al importar el módulo (con `.env` local y de la
raíz como respaldo) y valida todo al arrancar en `validar_configuracion`, para
que un valor inválido detenga el servicio con un mensaje claro en vez de
fallar a mitad de una petición.
"""

import logging
import math
import os
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import load_dotenv

logger = logging.getLogger(__name__)

# ===== Archivos .env =====

DIRECTORIO_SERVICIO = Path(__file__).resolve().parent.parent
DIRECTORIO_RAIZ_PROYECTO = DIRECTORIO_SERVICIO.parent.parent

# El .env del servicio tiene precedencia sobre el de la raíz; ninguno pisa el entorno real.
load_dotenv(DIRECTORIO_SERVICIO / ".env", override=False)
load_dotenv(DIRECTORIO_RAIZ_PROYECTO / ".env", override=False)

# ===== Constantes de validación =====

# HS256 exige una clave de al menos 256 bits para no ser débil.
LONGITUD_MINIMA_JWT_SECRET = 32
ESQUEMAS_HTTP_PERMITIDOS = ("http", "https")
PUERTO_MINIMO = 1
PUERTO_MAXIMO = 65535
RUTAS_DE_ORIGEN_PERMITIDAS = ("", "/")


# ===== Lectores de variables de entorno =====


def _leer_env_str(nombre_variable: str, valor_defecto: str) -> str:
    """Lee una variable de texto; devuelve el valor por defecto si falta o viene vacía."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    return valor_crudo.strip()


def _leer_env_float(nombre_variable: str, valor_defecto: float) -> float:
    """Lee una variable decimal; falla claro ante texto inválido o valores no finitos."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    try:
        valor_decimal = float(valor_crudo)
        if not math.isfinite(valor_decimal):
            raise ValueError("valor no finito")
        return valor_decimal
    except ValueError as error_parseo:
        raise RuntimeError(
            f"Configuracion invalida: {nombre_variable} debe ser un decimal valido y finito, recibido: '{valor_crudo}'."
        ) from error_parseo


def _leer_env_int(nombre_variable: str, valor_defecto: int) -> int:
    """Lee una variable entera; falla claro ante texto inválido."""
    valor_crudo = os.getenv(nombre_variable)
    if valor_crudo is None or not valor_crudo.strip():
        return valor_defecto
    try:
        return int(valor_crudo)
    except ValueError as error_parseo:
        raise RuntimeError(
            f"Configuracion invalida: {nombre_variable} debe ser un entero valido, recibido: '{valor_crudo}'."
        ) from error_parseo


# ===== Seguridad y CORS (variables compartidas, SUP-345) =====

JWT_SECRET = os.getenv("JWT_SECRET")

# Convención <SERVICIO>_CORS_ALLOWED_ORIGINS con respaldo en CORS_ALLOWED_ORIGINS (GUIA_CONFIGURACION).
CORS_ALLOWED_ORIGINS = (
    os.getenv("IA_CORS_ALLOWED_ORIGINS", "").strip()
    or os.getenv("CORS_ALLOWED_ORIGINS", "").strip()
    or "http://localhost:3000,http://localhost:5173"
)

# ===== Catálogo interno (HTTP, nunca PostgreSQL) =====

CATALOGO_INTERNAL_URL = _leer_env_str("CATALOGO_INTERNAL_URL", "http://catalogo:8080").rstrip("/")
CATALOGO_TIMEOUT = _leer_env_float("CATALOGO_TIMEOUT", 5.0)
CATALOGO_CACHE_TTL = _leer_env_float("IA_CATALOGO_CACHE_TTL", 60.0)
CATALOGO_PAGE_LIMIT = _leer_env_int("IA_CATALOGO_PAGE_LIMIT", 100)
CATALOGO_MAX_PAGINAS = _leer_env_int("IA_CATALOGO_MAX_PAGINAS", 10)
# Búsqueda dirigida por término (`?q=`): la caché precargada solo cubre las primeras
# páginas (las más baratas), así que las palabras clave e ingredientes se buscan en
# Catálogo completo. Catálogo recorta `limit` a su CATALOGO_MAX_PAGE_SIZE (100).
CATALOGO_BUSQUEDA_LIMITE = _leer_env_int("IA_CATALOGO_BUSQUEDA_LIMITE", 100)
# Tope de consultas por mensaje, para que una receta larga no agote el plazo del chat.
CATALOGO_BUSQUEDA_MAX_CONSULTAS = _leer_env_int("IA_CATALOGO_BUSQUEDA_MAX_CONSULTAS", 12)
# Búsquedas simultáneas contra Catálogo: acota hilos, conexiones y carga SQL por mensaje.
CATALOGO_BUSQUEDA_CONCURRENCIA = _leer_env_int("IA_CATALOGO_BUSQUEDA_CONCURRENCIA", 4)
CATALOGO_CONSULTA_TOKEN = os.getenv("CATALOGO_CONSULTA_TOKEN", "").strip() or None

# ===== Proveedor Groq (secundario) =====

GROQ_API_KEY = os.getenv("GROQ_API_KEY")
GROQ_ENDPOINT = _leer_env_str("GROQ_ENDPOINT", "https://api.groq.com/openai/v1/chat/completions")
# Groq retiró llama-3.1-8b-instant y llama-3.3-70b-versatile el 16-ago-2026.
GROQ_MODEL_EXTRACCION = _leer_env_str("GROQ_MODEL_EXTRACCION", "openai/gpt-oss-20b")
GROQ_MODEL_EXPLICACION = _leer_env_str("GROQ_MODEL_EXPLICACION", "openai/gpt-oss-120b")
GROQ_TIMEOUT = _leer_env_float("IA_GROQ_TIMEOUT", 6.0)
GROQ_MAX_TOKENS_EXTRACCION = _leer_env_int("IA_GROQ_MAX_TOKENS_EXTRACCION", 512)
GROQ_MAX_TOKENS_EXPLICACION = _leer_env_int("IA_GROQ_MAX_TOKENS_EXPLICACION", 200)
GROQ_FREQUENCY_PENALTY = _leer_env_float("IA_GROQ_FREQUENCY_PENALTY", 0.4)

# ===== Proveedor Gemini (primario) =====

GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")
GEMINI_MODEL = _leer_env_str("GEMINI_MODEL", "gemini-3.1-flash-lite")
GEMINI_API_BASE_URL = _leer_env_str("GEMINI_API_BASE_URL", "https://generativelanguage.googleapis.com/v1beta").rstrip(
    "/"
)
GEMINI_ENDPOINT = _leer_env_str("GEMINI_ENDPOINT", f"{GEMINI_API_BASE_URL}/models/{GEMINI_MODEL}:generateContent")
GEMINI_TIMEOUT = _leer_env_float("IA_GEMINI_TIMEOUT", 6.0)
GEMINI_THINKING_LEVEL = _leer_env_str("IA_GEMINI_THINKING_LEVEL", "low")
GEMINI_MAX_TOKENS_EXTRACCION = _leer_env_int("IA_GEMINI_MAX_TOKENS_EXTRACCION", 1200)
GEMINI_MAX_TOKENS_EXPLICACION = _leer_env_int("IA_GEMINI_MAX_TOKENS_EXPLICACION", 2048)

# ===== Reintentos y respaldo entre proveedores =====

FALLBACK_RETRIES_PRIMARIO = _leer_env_int("IA_FALLBACK_RETRIES_PRIMARIO", 0)
FALLBACK_RETRIES_SECUNDARIO = _leer_env_int("IA_FALLBACK_RETRIES_SECUNDARIO", 0)
FALLBACK_BACKOFF_BASE = _leer_env_float("IA_FALLBACK_BACKOFF_BASE", 0.5)
FALLBACK_JITTER_MAX = _leer_env_float("IA_FALLBACK_JITTER_MAX", 0.5)

# ===== Límites del chat =====

CHAT_MAX_MENSAJE_LEN = _leer_env_int("IA_CHAT_MAX_MENSAJE_LEN", 1000)
EXPLICACION_MAX_PRODUCTOS = _leer_env_int("IA_EXPLICACION_MAX_PRODUCTOS", 8)
CHAT_RATE_LIMIT_SOLICITUDES = _leer_env_int("IA_CHAT_RATE_LIMIT_SOLICITUDES", 10)
CHAT_RATE_LIMIT_VENTANA_SEGUNDOS = _leer_env_float("IA_CHAT_RATE_LIMIT_VENTANA_SEGUNDOS", 60.0)

CHAT_TIMEOUT_TOTAL = _leer_env_float("IA_CHAT_TIMEOUT_TOTAL", 18.0)


# ===== Antigüedad de la caché de Catálogo (SUP-315) =====

CATALOGO_ANTIGUEDAD_MAXIMA = _leer_env_float("IA_CATALOGO_ANTIGUEDAD_MAXIMA", 3600.0)


# ===== Validación al arrancar =====


def obtener_origenes_cors() -> list[str]:
    """Devuelve la lista limpia de orígenes CORS permitidos."""
    return [origen.strip() for origen in CORS_ALLOWED_ORIGINS.split(",") if origen.strip()]


def _puerto_valido(partes_url) -> bool:
    """Indica si el puerto de una URL está ausente o dentro del rango TCP; `urlsplit` lanza si no es numérico."""
    try:
        puerto = partes_url.port
    except ValueError:
        return False
    return puerto is None or PUERTO_MINIMO <= puerto <= PUERTO_MAXIMO


def _validar_catalogo_url() -> None:
    """Exige que CATALOGO_INTERNAL_URL sea un origen HTTP limpio: sin credenciales, path, query ni fragmento."""
    partes_url = urlsplit(CATALOGO_INTERNAL_URL)
    if partes_url.scheme not in ESQUEMAS_HTTP_PERMITIDOS:
        raise RuntimeError(f"CATALOGO_INTERNAL_URL esquema no soportado: '{partes_url.scheme}'. Debe ser http o https.")
    if not partes_url.hostname:
        raise RuntimeError("CATALOGO_INTERNAL_URL debe incluir un host o nombre de dominio valido.")
    if not _puerto_valido(partes_url):
        raise RuntimeError(
            f"CATALOGO_INTERNAL_URL puerto invalido en '{CATALOGO_INTERNAL_URL}'. "
            f"Debe ser un entero entre {PUERTO_MINIMO} y {PUERTO_MAXIMO}."
        )
    if partes_url.username or partes_url.password:
        raise RuntimeError("CATALOGO_INTERNAL_URL no debe incluir credenciales de usuario o contrasena.")
    if partes_url.path not in RUTAS_DE_ORIGEN_PERMITIDAS:
        raise RuntimeError(f"CATALOGO_INTERNAL_URL no debe incluir path: '{partes_url.path}'.")
    if partes_url.query or partes_url.fragment:
        raise RuntimeError("CATALOGO_INTERNAL_URL no debe incluir query parameters ni fragmentos.")


def _validar_origenes_cors() -> None:
    """Exige al menos un origen CORS y que cada uno sea un origen HTTP(S) exacto, sin comodines."""
    origenes = obtener_origenes_cors()
    if not origenes:
        raise RuntimeError("Debe configurarse al menos un origen valido en IA_CORS_ALLOWED_ORIGINS.")
    for origen in origenes:
        if "*" in origen:
            raise RuntimeError(f"Origen CORS no puede contener comodines '*': '{origen}'.")
        if origen.endswith("/"):
            raise RuntimeError(f"Origen CORS no puede terminar en barra '/': '{origen}'.")
        partes_origen = urlsplit(origen)
        if partes_origen.scheme not in ESQUEMAS_HTTP_PERMITIDOS or not partes_origen.hostname:
            raise RuntimeError(
                f"Origen CORS invalido: '{origen}'. Debe ser un origen HTTP(S) con host valido (ej. http://localhost:3000)."
            )
        if not _puerto_valido(partes_origen):
            raise RuntimeError(
                f"Origen CORS puerto invalido en '{origen}'. Debe ser un entero entre {PUERTO_MINIMO} y {PUERTO_MAXIMO}."
            )
        if partes_origen.username or partes_origen.password:
            raise RuntimeError(f"Origen CORS no puede contener credenciales: '{origen}'.")
        if partes_origen.path not in RUTAS_DE_ORIGEN_PERMITIDAS:
            raise RuntimeError(f"Origen CORS no puede contener path o ruta: '{origen}'.")
        if partes_origen.query or partes_origen.fragment:
            raise RuntimeError(f"Origen CORS no puede contener query parameters ni fragmentos: '{origen}'.")


def _validar_limites_numericos() -> None:
    """Exige que timeouts y límites sean positivos, y que caché, reintentos y jitter no sean negativos."""
    # Pares (nombre, valor) con referencias directas: un renombre lo detecta el linter. Se arman en
    # cada llamada para que las pruebas puedan sustituir los valores con monkeypatch.
    estrictamente_positivos = (
        ("CATALOGO_TIMEOUT", CATALOGO_TIMEOUT),
        ("CATALOGO_PAGE_LIMIT", CATALOGO_PAGE_LIMIT),
        ("CATALOGO_MAX_PAGINAS", CATALOGO_MAX_PAGINAS),
        ("CATALOGO_BUSQUEDA_LIMITE", CATALOGO_BUSQUEDA_LIMITE),
        ("CATALOGO_BUSQUEDA_MAX_CONSULTAS", CATALOGO_BUSQUEDA_MAX_CONSULTAS),
        ("CATALOGO_BUSQUEDA_CONCURRENCIA", CATALOGO_BUSQUEDA_CONCURRENCIA),
        ("GROQ_TIMEOUT", GROQ_TIMEOUT),
        ("GROQ_MAX_TOKENS_EXTRACCION", GROQ_MAX_TOKENS_EXTRACCION),
        ("GROQ_MAX_TOKENS_EXPLICACION", GROQ_MAX_TOKENS_EXPLICACION),
        ("GEMINI_TIMEOUT", GEMINI_TIMEOUT),
        ("GEMINI_MAX_TOKENS_EXTRACCION", GEMINI_MAX_TOKENS_EXTRACCION),
        ("GEMINI_MAX_TOKENS_EXPLICACION", GEMINI_MAX_TOKENS_EXPLICACION),
        ("FALLBACK_BACKOFF_BASE", FALLBACK_BACKOFF_BASE),
        ("CHAT_MAX_MENSAJE_LEN", CHAT_MAX_MENSAJE_LEN),
        ("EXPLICACION_MAX_PRODUCTOS", EXPLICACION_MAX_PRODUCTOS),
        ("IA_CHAT_TIMEOUT_TOTAL", CHAT_TIMEOUT_TOTAL),
        # Estos dos se nombran por su variable de entorno, como en sus mensajes originales.
        ("IA_CHAT_RATE_LIMIT_SOLICITUDES", CHAT_RATE_LIMIT_SOLICITUDES),
        ("IA_CHAT_RATE_LIMIT_VENTANA_SEGUNDOS", CHAT_RATE_LIMIT_VENTANA_SEGUNDOS),
        ("IA_CATALOGO_ANTIGUEDAD_MAXIMA", CATALOGO_ANTIGUEDAD_MAXIMA),
    )
    for nombre, valor in estrictamente_positivos:
        if valor <= 0:
            raise RuntimeError(f"{nombre} debe ser mayor a 0.")

    no_negativos = (
        ("CATALOGO_CACHE_TTL", CATALOGO_CACHE_TTL),
        ("FALLBACK_RETRIES_PRIMARIO", FALLBACK_RETRIES_PRIMARIO),
        ("FALLBACK_RETRIES_SECUNDARIO", FALLBACK_RETRIES_SECUNDARIO),
        ("FALLBACK_JITTER_MAX", FALLBACK_JITTER_MAX),
    )
    for nombre, valor in no_negativos:
        if valor < 0:
            raise RuntimeError(f"{nombre} debe ser mayor o igual a 0.")


def _advertir_configuracion_proveedores() -> None:
    """Avisa (sin detener el arranque) si no hay claves de IA o si GEMINI_ENDPOINT anula a GEMINI_MODEL."""
    if not GEMINI_API_KEY and not GROQ_API_KEY:
        logger.warning("Ni GEMINI_API_KEY ni GROQ_API_KEY estan configuradas. Los endpoints con IA fallaran.")

    if not CATALOGO_CONSULTA_TOKEN:
        logger.warning(
            "CATALOGO_CONSULTA_TOKEN no configurado: los candidatos de receta no validaran disponibilidad en lote."
        )

    endpoint_explicito = os.getenv("GEMINI_ENDPOINT")
    if endpoint_explicito:
        modelo_actual = os.getenv("GEMINI_MODEL") or GEMINI_MODEL
        if f"/models/{modelo_actual}:generateContent" not in endpoint_explicito:
            logger.warning(
                "GEMINI_ENDPOINT ('%s') esta configurado explicitamente y anula a GEMINI_MODEL ('%s').",
                endpoint_explicito,
                modelo_actual,
            )


def validar_configuracion() -> None:
    """Valida la configuración al arrancar y lanza RuntimeError ante el primer valor inválido."""
    if not JWT_SECRET or len(JWT_SECRET) < LONGITUD_MINIMA_JWT_SECRET:
        raise RuntimeError(
            f"JWT_SECRET debe estar configurado y tener al menos {LONGITUD_MINIMA_JWT_SECRET} caracteres."
        )
    _validar_catalogo_url()
    _validar_origenes_cors()
    _validar_limites_numericos()
    _advertir_configuracion_proveedores()
