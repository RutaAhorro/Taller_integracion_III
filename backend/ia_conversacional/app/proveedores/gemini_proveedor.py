"""Implementacion de ProveedorIA para Gemini (Google AI Studio)."""

import logging
import time

import httpx

from app import config
from app.explicacion import construir_prompt_explicacion
from app.filtros import sanitizar_filtros_crudos
from app.proveedores.base import ProveedorIA, ResultadoExtraccion

logger = logging.getLogger(__name__)


class GeminiProveedor(ProveedorIA):
    nombre = "gemini"

    def __init__(
        self,
        api_key: str | None,
        endpoint: str,
        modelo: str,
        definicion_funcion: dict,
        timeout: float | None = None,
        thinking_level: str | None = None,
        max_tokens_extraccion: int | None = None,
        max_tokens_explicacion: int | None = None,
    ):
        self.api_key = api_key
        self.endpoint = endpoint
        self.modelo = modelo
        self.definicion_funcion = definicion_funcion
        self.timeout = timeout if timeout is not None else config.GEMINI_TIMEOUT
        self.thinking_level = (
            thinking_level if thinking_level is not None else config.GEMINI_THINKING_LEVEL
        )
        self.max_tokens_extraccion = (
            max_tokens_extraccion
            if max_tokens_extraccion is not None
            else config.GEMINI_MAX_TOKENS_EXTRACCION
        )
        self.max_tokens_explicacion = (
            max_tokens_explicacion
            if max_tokens_explicacion is not None
            else config.GEMINI_MAX_TOKENS_EXPLICACION
        )

    def _headers(self) -> dict:
        return {
            "x-goog-api-key": self.api_key,
            "Content-Type": "application/json",
        }

    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        inicio = time.perf_counter()
        payload = {
            "contents": [{"role": "user", "parts": [{"text": mensaje}]}],
            "tools": [{"function_declarations": [self.definicion_funcion]}],
            # Forzado (mode ANY, en vez de dejar el default AUTO): con
            # AUTO, Gemini a veces decide que un pedido tipo "quiero hacer
            # pebre" es una pregunta de cocina normal y responde en texto
            # libre con su propio conocimiento -- ahi se salta el
            # filtrado real por completo y puede mencionar productos que
            # no existen en el catalogo ("tienda"). ANY obliga a que
            # siempre llame la funcion, para que la explicacion final
            # quede acotada a productos reales o almenos contenga productos
            # de la tienda
            "toolConfig": {
                "functionCallingConfig": {"mode": "ANY"},
            },
            # la extraccion es una tarea simple, no necesita razonamiento
            # extendido, y por defecto tardaba como 3s de mas por el
            # thinking. Se incluye maxOutputTokens aqui tambien por la
            # misma razon (budget compartido thinking+respuesta) aunque
            # la tool call en si es corta, para no arriesgarse a un
            # 400/vacio en un mensaje largo.
            "generationConfig": {
                "thinkingConfig": {"thinkingLevel": self.thinking_level},
                "maxOutputTokens": self.max_tokens_extraccion,
            },
        }

        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                respuesta = await client.post(self.endpoint, headers=self._headers(), json=payload)
                respuesta.raise_for_status()
                data = respuesta.json()
        except httpx.HTTPStatusError as error_http:
            estado = None
            mensaje_error = error_http.response.text
            try:
                cuerpo_error = error_http.response.json().get("error", {})
                estado = cuerpo_error.get("status")
                mensaje_error = cuerpo_error.get("message", mensaje_error)
            except ValueError:
                pass

            logger.warning(
                "Gemini respondio %s: %s",
                error_http.response.status_code,
                mensaje_error,
            )

            nota = None
            if error_http.response.status_code == 429:
                nota = (
                    "Si estado_error es RESOURCE_EXHAUSTED, revisa tu cuota real "
                    f"para {self.modelo} en https://aistudio.google.com/rate-limit "
                    "(los limites del free tier son por proyecto y varian por "
                    "cuenta, no son un numero fijo publicado). El mensaje de "
                    "arriba dice si es limite por minuto (espera unos segundos "
                    "y reintenta) o por dia (no se libera hasta medianoche "
                    "hora del Pacifico)."
                )

            return ResultadoExtraccion(
                error=f"Gemini respondio {error_http.response.status_code}: {mensaje_error}",
                error_estado=estado,
                nota=nota,
            )
        except httpx.RequestError as error_red:
            logger.warning("Error de red conectando a Gemini: %s", error_red)
            return ResultadoExtraccion(error=f"No se pudo conectar a Gemini: {error_red}")

        tiempo_ms = round((time.perf_counter() - inicio) * 1000)

        try:
            parts = data["candidates"][0]["content"]["parts"]
        except (KeyError, IndexError):
            return ResultadoExtraccion(error="Respuesta inesperada de Gemini", extra={"raw": data})

        for parte in parts:
            if parte.get("thought"):
                continue  # ver nota en generar_explicacion
            if "functionCall" in parte:
                filtros = parte["functionCall"].get("args", {})
                filtros = sanitizar_filtros_crudos(filtros)
                logger.debug("Filtros extraidos (Gemini): %s", filtros)
                return ResultadoExtraccion(filtros=filtros, tiempo_extraccion_ms=tiempo_ms)

        # El modelo no llamo la funcion, respondio con texto libre
        texto_libre = "".join(parte.get("text", "") for parte in parts if not parte.get("thought"))
        return ResultadoExtraccion(
            texto_libre=texto_libre,
            nota="El modelo no uso function calling esta vez",
            tiempo_extraccion_ms=tiempo_ms,
        )

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        prompt = construir_prompt_explicacion(mensaje_usuario, productos_filtrados)
        payload = {
            "contents": [{"role": "user", "parts": [{"text": prompt}]}],
            "generationConfig": {
                "maxOutputTokens": self.max_tokens_explicacion,
                # Bug real: sin esto, gemini 3.1 piensa por defecto
                # antes de responder (el thinking dinamico) y esa vez se comio
                # el budget de tokens dejando la respuesta vacia/cortada
                # lo que se vio como el texto suelto "Brief response in
                # Spanish." en vez de la explicacion real, el "low" es el
                # nivel minimo soportado para este modelo (flash de la
                # serie 3 no permite apagarlo del todo, solo bajarlo) y de
                # paso, responde mas rapido

                "thinkingConfig": {"thinkingLevel": self.thinking_level},
            },
        }

        async with httpx.AsyncClient(timeout=self.timeout) as client:
            respuesta = await client.post(self.endpoint, headers=self._headers(), json=payload)
            respuesta.raise_for_status()
            data = respuesta.json()

        partes = data["candidates"][0]["content"]["parts"]
        # es por si en algun momento se activa includeThoughts (nosotros
        # no lo pedimos pero por si acaso), las partes de razonamiento
        # vienen marcadas con "thought": true y no son la respuesta real
        # se filtran para no mezclar el resumen del razonamiento con la
        # explicacion
        return "".join(parte.get("text", "") for parte in partes if not parte.get("thought"))
