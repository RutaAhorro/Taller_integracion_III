"""
Proveedor Gemini (Google AI Studio) por la API REST `generateContent`.

Primario del respaldo. Las claves se envían en un encabezado y nunca se
registran en los logs.
"""

import logging
import time

import httpx

from app import config
from app.domain.filtros import sanitizar_filtros_crudos
from app.domain.prompts import DECLARACION_FUNCION_FILTROS, construir_prompt_explicacion
from app.domain.proveedores import MILISEGUNDOS_POR_SEGUNDO, ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)

# ===== Constantes =====

ENCABEZADO_CLAVE_API = "x-goog-api-key"
# ANY obliga a llamar la función. Con AUTO, pedidos como "quiero hacer pebre"
# se respondían en texto libre con conocimiento propio, saltándose el filtrado.
MODO_LLAMADA_OBLIGATORIA = "ANY"


# ===== Proveedor =====


class GeminiProveedor(ProveedorIA):
    """Implementa las etapas 1 y 3 con Gemini."""

    nombre = "gemini"

    def __init__(
        self,
        clave_api: str | None,
        url_endpoint: str,
        tiempo_espera: float = config.GEMINI_TIMEOUT,
        nivel_razonamiento: str = config.GEMINI_THINKING_LEVEL,
        max_tokens_extraccion: int = config.GEMINI_MAX_TOKENS_EXTRACCION,
        max_tokens_explicacion: int = config.GEMINI_MAX_TOKENS_EXPLICACION,
    ):
        """Guarda la clave, el endpoint del modelo y los límites de cada etapa."""
        self.clave_api = clave_api
        self.url_endpoint = url_endpoint
        self.tiempo_espera = tiempo_espera
        self.nivel_razonamiento = nivel_razonamiento
        self.max_tokens_extraccion = max_tokens_extraccion
        self.max_tokens_explicacion = max_tokens_explicacion

    def _encabezados(self) -> dict:
        """Encabezados HTTP con la clave de API de Google."""
        return {ENCABEZADO_CLAVE_API: self.clave_api, "Content-Type": "application/json"}

    async def _enviar(self, cuerpo: dict) -> dict:
        """Envía la petición a Gemini y devuelve el JSON; lanza httpx.HTTPStatusError ante un estado de error."""
        async with httpx.AsyncClient(timeout=self.tiempo_espera) as cliente_http:
            respuesta = await cliente_http.post(self.url_endpoint, headers=self._encabezados(), json=cuerpo)
            respuesta.raise_for_status()
            return respuesta.json()

    @staticmethod
    def _partes_visibles(partes: list) -> list:
        """Quita las partes de razonamiento (`thought: true`) para no mezclarlas con la respuesta real."""
        return [parte for parte in partes if not parte.get("thought")]

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        """Etapa 1 con function calling obligatorio; devuelve un resultado con `error` si la API falla."""
        if not self.clave_api:
            raise ValueError("GEMINI_API_KEY no configurada")
        inicio = time.perf_counter()
        cuerpo = {
            "contents": [{"role": "user", "parts": [{"text": mensaje}]}],
            "tools": [{"function_declarations": [DECLARACION_FUNCION_FILTROS]}],
            "toolConfig": {"functionCallingConfig": {"mode": MODO_LLAMADA_OBLIGATORIA}},
            # La extracción es simple: sin limitar el razonamiento tardaba ~3 s de más. El
            # presupuesto de tokens es compartido con el razonamiento, por eso también se acota.
            "generationConfig": {
                "thinkingConfig": {"thinkingLevel": self.nivel_razonamiento},
                "maxOutputTokens": self.max_tokens_extraccion,
            },
        }

        datos = await self._enviar(cuerpo)

        tiempo_ms = round((time.perf_counter() - inicio) * MILISEGUNDOS_POR_SEGUNDO)

        try:
            partes = self._partes_visibles(datos["candidates"][0]["content"]["parts"])
        except (KeyError, IndexError):
            raise ValueError("Respuesta inesperada de Gemini") from None

        for parte in partes:
            if "functionCall" in parte:
                filtros = sanitizar_filtros_crudos(parte["functionCall"].get("args", {}))
                logger.debug("Filtros extraidos (Gemini): %s", filtros)
                return ResultadoExtraccion(filtros=filtros, tiempo_extraccion_ms=tiempo_ms)

        # El modelo no llamó la función: el chat descarta este texto para no alucinar.
        texto_libre = "".join(parte.get("text", "") for parte in partes)
        return ResultadoExtraccion(texto_libre=texto_libre, tiempo_extraccion_ms=tiempo_ms)

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        """Etapa 3; deja propagar los errores HTTP para que el respaldo los reintente."""
        cuerpo = {
            "contents": [
                {
                    "role": "user",
                    "parts": [{"text": construir_prompt_explicacion(mensaje_usuario, productos_filtrados)}],
                }
            ],
            # Sin acotar el razonamiento, Gemini 3.1 consumía el presupuesto de tokens y dejaba la
            # respuesta vacía o cortada. "low" es el mínimo que admite esta serie.
            "generationConfig": {
                "maxOutputTokens": self.max_tokens_explicacion,
                "thinkingConfig": {"thinkingLevel": self.nivel_razonamiento},
            },
        }
        datos = await self._enviar(cuerpo)
        partes = self._partes_visibles(datos["candidates"][0]["content"]["parts"])
        return "".join(parte.get("text", "") for parte in partes)
