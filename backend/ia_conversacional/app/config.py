"""
Configuracion central: carga el .env, define rutas y agrupa todas las
constantes de los proveedores de IA (endpoints, modelos) 
"""

import os
from pathlib import Path

from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent.parent


PROJECT_ROOT = BASE_DIR.parent.parent
load_dotenv(PROJECT_ROOT / ".env")

PRODUCTOS_PATH = BASE_DIR / "productos.json"

# --------------------------------------------------------------------------
# Catálogo interno (SUP-246 / RNF-05: HTTP interno, sin credenciales de BD)
# --------------------------------------------------------------------------

CATALOGO_INTERNAL_URL = os.getenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
CATALOGO_TIMEOUT = float(os.getenv("CATALOGO_TIMEOUT", "5.0"))


GROQ_API_KEY = os.getenv("GROQ_API_KEY")
GROQ_ENDPOINT = "https://api.groq.com/openai/v1/chat/completions"


GROQ_MODEL_EXTRACCION = "llama-3.1-8b-instant"
GROQ_MODEL_EXPLICACION = "llama-3.3-70b-versatile"

# --------------------------------------------------------------------------
# Gemini
# --------------------------------------------------------------------------

GEMINI_API_KEY = os.getenv("GEMINI_API_KEY")


#flash lite soporta 500 por dia 
GEMINI_MODEL = "gemini-3.1-flash-lite"
GEMINI_ENDPOINT = (
    f"https://generativelanguage.googleapis.com/v1beta/models/{GEMINI_MODEL}:generateContent"
)