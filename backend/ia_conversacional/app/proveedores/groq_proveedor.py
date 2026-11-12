"""Implementacion de ProveedorIA para GroqCloud."""

import json
import logging
import time

import httpx

from app import config
from app.explicacion import construir_prompt_explicacion
from app.filtros import rescatar_filtros_de_error_groq, sanitizar_filtros_crudos
from app.proveedores.base import ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)


class GroqProveedor(ProveedorIA):
    nombre = "groq"

    def __init__(
        self,
        api_key: str | None,
        endpoint: str,
        modelo_extraccion: str,
        modelo_explicacion: str,
        definicion_funcion: dict,
        timeout: float | None = None,
        max_tokens_extraccion: int | None = None,
        max_tokens_explicacion: int | None = None,
        frequency_penalty: float | None = None,
    ):
        self.api_key = api_key
        self.endpoint = endpoint
        self.modelo_extraccion = modelo_extraccion
        self.modelo_explicacion = modelo_explicacion
        self.definicion_funcion = definicion_funcion
        self.timeout = timeout if timeout is not None else config.GROQ_TIMEOUT
        self.max_tokens_extraccion = (
            max_tokens_extraccion
            if max_tokens_extraccion is not None
            else config.GROQ_MAX_TOKENS_EXTRACCION
        )
        self.max_tokens_explicacion = (
            max_tokens_explicacion
            if max_tokens_explicacion is not None
            else config.GROQ_MAX_TOKENS_EXPLICACION
        )
        self.frequency_penalty = (
            frequency_penalty
            if frequency_penalty is not None
            else config.GROQ_FREQUENCY_PENALTY
        )

    def _headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        inicio = time.perf_counter()
        payload = {
            "model": self.modelo_extraccion,
            "messages": [{"role": "user", "content": mensaje}],
            "tools": [self.definicion_funcion],
            # Forzado (en vez de "auto"): con "auto" el modelo a veces
            # decide responder en texto libre usando su propio
            # conocimiento de cocina en vez de extraer filtros 
            "tool_choice": {
                "type": "function",
                "function": {"name": self.definicion_funcion["function"]["name"]},
            },
            "max_tokens": self.max_tokens_extraccion,
            "frequency_penalty": self.frequency_penalty,
        }

        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                respuesta = await client.post(self.endpoint, headers=self._headers(), json=payload)
                respuesta.raise_for_status()
                data = respuesta.json()
        except httpx.HTTPStatusError as error_http:
            tiempo_ms = round((time.perf_counter() - inicio) * 1000)
            rescatados = rescatar_filtros_de_error_groq(error_http.response)
            if rescatados is not None:
                logger.info("Groq rechazo el schema; filtros rescatados del failed_generation: %s", rescatados)
                return ResultadoExtraccion(
                    filtros=rescatados,
                    tiempo_extraccion_ms=tiempo_ms,
                    nota=(
                        "Groq rechazo la tool call por validacion de schema "
                        "(un campo vino con el tipo incorrecto, ej. texto en "
                        "vez de numero). Se descarto ese campo puntual y se "
                        "siguio con los filtros validos en vez de fallar."
                    ),
                )
            logger.warning("Groq respondio %s: %s", error_http.response.status_code, error_http.response.text)
            return ResultadoExtraccion(error=f"Groq respondio {error_http.response.status_code}: {error_http.response.text}")
        except httpx.RequestError as error_red:
            logger.warning("Error de red conectando a Groq: %s", error_red)
            return ResultadoExtraccion(error=f"No se pudo conectar a Groq: {error_red}")

        tiempo_ms = round((time.perf_counter() - inicio) * 1000)
        mensaje_modelo = data["choices"][0]["message"]
        tool_calls = mensaje_modelo.get("tool_calls")

        if not tool_calls:
            # El modelo respondio con texto libre en vez de llamar la funcion
            return ResultadoExtraccion(
                texto_libre=mensaje_modelo.get("content"),
                nota="El modelo no uso function calling esta vez",
                tiempo_extraccion_ms=tiempo_ms,
            )

        filtros = json.loads(tool_calls[0]["function"]["arguments"])
        filtros = sanitizar_filtros_crudos(filtros)
        logger.debug("Filtros extraidos (Groq): %s", filtros)
        return ResultadoExtraccion(filtros=filtros, tiempo_extraccion_ms=tiempo_ms)

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        prompt = construir_prompt_explicacion(mensaje_usuario, productos_filtrados)
        payload = {
            "model": self.modelo_explicacion,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": self.max_tokens_explicacion,
        }

        async with httpx.AsyncClient(timeout=self.timeout) as client:
            respuesta = await client.post(self.endpoint, headers=self._headers(), json=payload)
            respuesta.raise_for_status()
            data = respuesta.json()

        return data["choices"][0]["message"]["content"]
