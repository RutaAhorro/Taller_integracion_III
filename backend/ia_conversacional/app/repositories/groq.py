"""
Proveedor Groq (GroqCloud, API compatible con OpenAI).

Secundario del respaldo. Usa un modelo liviano para extraer filtros y uno
más capaz para redactar la explicación.
"""

import json
import logging
import time

import httpx

from app import config
from app.domain.filtros import sanitizar_filtros_crudos
from app.domain.prompts import DECLARACION_FUNCION_FILTROS, NOMBRE_FUNCION_FILTROS, construir_prompt_explicacion
from app.domain.proveedores import MILISEGUNDOS_POR_SEGUNDO, ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)

# ===== Constantes =====

# Groq (API compatible con OpenAI) envuelve la declaración común de la función.
HERRAMIENTA_FILTROS_OPENAI = {"type": "function", "function": DECLARACION_FUNCION_FILTROS}

# Forzar la función en vez de "auto": con "auto" el modelo a veces respondía
# texto libre con su propio conocimiento de cocina en vez de extraer filtros.
ELECCION_HERRAMIENTA_OBLIGATORIA = {"type": "function", "function": {"name": NOMBRE_FUNCION_FILTROS}}


# ===== Rescate de errores de esquema de Groq =====


def _extraer_json_de_texto(texto: str) -> dict | None:
    """Toma el objeto JSON delimitado por la primera y la última llave, o None si no es válido."""
    inicio = texto.find("{")
    fin = texto.rfind("}")
    if inicio == -1 or fin < inicio:
        return None
    try:
        return json.loads(texto[inicio : fin + 1])
    except json.JSONDecodeError:
        return None


def rescatar_filtros_de_error_groq(respuesta: httpx.Response) -> dict | None:
    """
    Recupera filtros del `failed_generation` que Groq adjunta al rechazar una tool call.

    Groq responde 400 si un campo no calza con el esquema; los demás campos
    suelen ser válidos y se aprovechan tras sanearlos.
    """
    try:
        cuerpo = respuesta.json()
    except ValueError:
        return None

    generacion_fallida = cuerpo.get("error", {}).get("failed_generation")
    if isinstance(generacion_fallida, str):
        crudo = _extraer_json_de_texto(generacion_fallida)
    elif isinstance(generacion_fallida, dict):
        crudo = generacion_fallida
    else:
        crudo = None

    if crudo is None:
        return None
    return sanitizar_filtros_crudos(crudo)


# ===== Proveedor =====


class GroqProveedor(ProveedorIA):
    """Implementa las etapas 1 y 3 con Groq."""

    nombre = "groq"

    def __init__(
        self,
        clave_api: str | None,
        url_endpoint: str,
        modelo_extraccion: str,
        modelo_explicacion: str,
        tiempo_espera: float = config.GROQ_TIMEOUT,
        max_tokens_extraccion: int = config.GROQ_MAX_TOKENS_EXTRACCION,
        max_tokens_explicacion: int = config.GROQ_MAX_TOKENS_EXPLICACION,
        penalizacion_frecuencia: float = config.GROQ_FREQUENCY_PENALTY,
    ):
        """Guarda la clave, el endpoint, los modelos de cada etapa y sus límites."""
        self.clave_api = clave_api
        self.url_endpoint = url_endpoint
        self.modelo_extraccion = modelo_extraccion
        self.modelo_explicacion = modelo_explicacion
        self.tiempo_espera = tiempo_espera
        self.max_tokens_extraccion = max_tokens_extraccion
        self.max_tokens_explicacion = max_tokens_explicacion
        self.penalizacion_frecuencia = penalizacion_frecuencia

    def _encabezados(self) -> dict:
        """Encabezados HTTP con la clave como Bearer."""
        return {"Authorization": f"Bearer {self.clave_api}", "Content-Type": "application/json"}

    async def _enviar(self, cuerpo: dict) -> dict:
        """Envía la petición a Groq y devuelve el JSON; lanza httpx.HTTPStatusError ante un estado de error."""
        async with httpx.AsyncClient(timeout=self.tiempo_espera) as cliente_http:
            respuesta = await cliente_http.post(self.url_endpoint, headers=self._encabezados(), json=cuerpo)
            respuesta.raise_for_status()
            return respuesta.json()

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        """Etapa 1 con tool call obligatoria; rescata filtros si Groq rechaza el esquema."""
        if not self.clave_api:
            raise ValueError("GROQ_API_KEY no configurada")
        inicio = time.perf_counter()
        cuerpo = {
            "model": self.modelo_extraccion,
            "messages": [{"role": "user", "content": mensaje}],
            "tools": [HERRAMIENTA_FILTROS_OPENAI],
            "tool_choice": ELECCION_HERRAMIENTA_OBLIGATORIA,
            "max_tokens": self.max_tokens_extraccion,
            "frequency_penalty": self.penalizacion_frecuencia,
        }

        try:
            datos = await self._enviar(cuerpo)
        except httpx.HTTPStatusError as error_http:
            tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)
            rescatados = rescatar_filtros_de_error_groq(error_http.response)
            if rescatados is not None:
                logger.info("Groq rechazo el schema; filtros rescatados del failed_generation: %s", rescatados)
                return ResultadoExtraccion(filtros=rescatados, tiempo_extraccion_ms=tiempo_ms)
            estado_http = error_http.response.status_code
            logger.warning("Groq respondió HTTP %s sin filtros rescatables.", estado_http)
            # Sin filtros rescatables el error debe relanzarse: el respaldo ya decide reintentar/conmutar.
            raise

        tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)
        mensaje_modelo = datos["choices"][0]["message"]
        llamadas_herramienta = mensaje_modelo.get("tool_calls")

        if not llamadas_herramienta:
            # El modelo no llamó la función: el chat descarta este texto para no alucinar.
            return ResultadoExtraccion(texto_libre=mensaje_modelo.get("content"), tiempo_extraccion_ms=tiempo_ms)

        filtros = sanitizar_filtros_crudos(json.loads(llamadas_herramienta[0]["function"]["arguments"]))
        logger.debug("Filtros extraidos (Groq): %s", filtros)
        return ResultadoExtraccion(filtros=filtros, tiempo_extraccion_ms=tiempo_ms)

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        """Etapa 3; deja propagar los errores HTTP para que el respaldo los reintente."""
        cuerpo = {
            "model": self.modelo_explicacion,
            "messages": [
                {"role": "user", "content": construir_prompt_explicacion(mensaje_usuario, productos_filtrados)}
            ],
            "max_tokens": self.max_tokens_explicacion,
        }
        datos = await self._enviar(cuerpo)
        return datos["choices"][0]["message"]["content"]
