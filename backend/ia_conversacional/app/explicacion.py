import json
import logging
import time

import httpx

logger = logging.getLogger(__name__)


def construir_prompt_explicacion(mensaje_usuario: str, productos_filtrados: list) -> str:
    """Prompt compartido entre Groq y Gemini para la etapa de explicacion."""
    resumen = [
        {
            "nombre": p["nombre"],
            "precio": p["precio"],
            "categoria": p["categoria"],
            "distancia_km": p["distancia_km"],
        }
        # los primeros 8 alcanzan de sobra (ya vienen ordenados por precio
        # ascendente) y mantiene el prompt corto -> respuesta mas rapida.
        for p in productos_filtrados[:8]
    ]

    return (
        f'El usuario pidio: "{mensaje_usuario}"\n\n'
        "Estos son los UNICOS productos reales disponibles que calzan con "
        "su busqueda (ya fueron filtrados por precio/categoria/distancia "
        "por el sistema, ordenados del mas barato al mas caro):\n"
        f"{json.dumps(resumen, ensure_ascii=False)}\n\n"
        "Redacta una respuesta breve en espanol (sin maximo de oraciones) "
        "explicando por que estos productos calzan con lo que pidio. "
        "Menciona nombres y precios reales de la lista. No inventes "
        "productos, precios, tiendas ni caracteristicas que no esten en "
        "la lista. No hagas preguntas de seguimiento, es una respuesta "
        "unica."
    )


async def generar_explicacion_con_tiempo(generador, mensaje_usuario: str, productos_filtrados: list):
    """
    Envoltura comun para ambos proveedores: mide cuanto demora la etapa
    de explicacion (para poder comparar velocidad real en el paso 8), y
    si la llamada falla (red, rate limit, etc.) no tumba el endpoint --
    los productos filtrados ya son validos igual, la explicacion es un
    plus, no algo de lo que dependa el resultado.

    `generador` es el metodo generar_explicacion(mensaje, productos) de
    un proveedor (GroqProveedor o GeminiProveedor) -- ver proveedores/.
    """
    if not productos_filtrados:
        return "No encontramos productos que coincidan con tu busqueda.", 0

    inicio = time.perf_counter()
    try:
        explicacion = await generador(mensaje_usuario, productos_filtrados)
    except Exception as e:
        logger.warning("No se pudo generar la explicacion: %s", e)
        explicacion = None
    tiempo_ms = round((time.perf_counter() - inicio) * 1000)

    return explicacion or "Encontramos estos productos para tu búsqueda.", tiempo_ms
