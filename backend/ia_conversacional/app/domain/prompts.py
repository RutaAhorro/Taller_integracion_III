"""
Textos que el pipeline envía a los LLM: la función de extracción (etapa 1) y
el prompt de explicación (etapa 3).

La declaración de la función obliga al modelo a devolver filtros en vez de
texto libre o productos propios; Gemini la usa tal cual y Groq la envuelve en
el formato OpenAI. El prompt de explicación solo incluye productos ya
filtrados, para que el modelo no invente productos, precios ni tiendas.
"""

import json

from app import config

# ===== Función de extracción (etapa 1) =====

NOMBRE_FUNCION_FILTROS = "extraer_filtros_busqueda"

# Gemini espera name/description/parameters directamente.
DECLARACION_FUNCION_FILTROS = {
    "name": NOMBRE_FUNCION_FILTROS,
    "description": (
        "Extrae los parametros de busqueda de productos a partir del mensaje en lenguaje natural del usuario."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "precio_max": {
                "type": "number",
                "description": (
                    "Precio maximo en pesos chilenos que el usuario esta "
                    "dispuesto a pagar. Si el usuario no menciona un "
                    "precio, NO incluyas esta clave. Nunca escribas texto "
                    "como 'omision' o null aqui: o va un numero real, o "
                    "la clave no aparece."
                ),
            },
            "categoria": {
                "type": "string",
                "description": (
                    "Categoria del producto buscado, por ejemplo: verduras, "
                    "frutas, lacteos, panaderia, abarrotes, carnes. Si el "
                    "usuario no menciona una categoria, NO incluyas esta "
                    "clave."
                ),
            },
            "distancia_max": {
                "type": "number",
                "description": (
                    "Distancia maxima en km que el usuario esta dispuesto "
                    "a recorrer. Si el usuario no menciona una distancia "
                    "(aunque diga 'cerca' sin dar un numero), NO incluyas "
                    "esta clave. Nunca escribas texto como 'omision' o "
                    "null aqui: o va un numero real, o la clave no "
                    "aparece."
                ),
            },
            "palabras_clave": {
                "type": "array",
                "items": {"type": "string"},
                "description": (
                    "Lista de terminos de busqueda que ayudan a encontrar "
                    "productos reales relacionados con el pedido, MAS ALLA "
                    "de solo repetir palabras textuales del mensaje. Usa tu "
                    "propio conocimiento general para asociar el pedido con "
                    "productos o caracteristicas concretas, incluso si la "
                    "conexion no es obvia o el pedido no es sobre comida. "
                    "Ejemplos: 'algo con tomate' -> ['tomate']; 'receta con "
                    "tomate y ajo' -> ['tomate', 'ajo']; 'comida salada que "
                    "tenga platano' -> ['platano', 'tostones', 'chicharron "
                    "de platano', 'platano frito'] (asociaciones reales de "
                    "platano en preparaciones saladas, no solo la palabra "
                    "literal); 'lavadora barata con bajo consumo' -> "
                    "['lavadora', 'bajo consumo', 'eficiente']; 'quiero "
                    "hacer pebre' -> ['tomate', 'cebolla', 'cilantro', "
                    "'aji verde', 'limon'] (los ingredientes REALES de la "
                    "receta chilena de pebre, nunca la palabra 'pebre' en "
                    "si, porque esa palabra no es un producto de venta); "
                    "'quiero hacer cazuela' -> ['papa', 'zapallo', 'choclo', "
                    "'carne', 'zanahoria']. Si el usuario menciona el "
                    "nombre de una receta o plato que no reconoces con "
                    "certeza, igual responde con tu mejor estimacion de "
                    "ingredientes tipicos en vez de dejar el campo vacio -- "
                    "una asociacion aproximada sirve mas que ninguna. "
                    "Incluye tanto los terminos literales del usuario como "
                    "asociaciones razonables. Si el pedido ya es una "
                    "categoria amplia y generica (ej. 'verduras baratas'), "
                    "usa 'categoria' en vez de esto y deja este campo vacio."
                ),
            },
            "ingredientes": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "nombre": {
                            "type": "string",
                            "description": (
                                "Ingrediente real de la receta, no el plato ni la "
                                "categoria (ej. 'papa' para una cazuela). Si el "
                                "usuario no pide una receta, NO incluyas esta clave."
                            ),
                        },
                        "cantidad_envases": {
                            "type": "integer",
                            "minimum": 1,
                            "description": (
                                "Cantidad en envases del ingrediente que el usuario "
                                "podria necesitar para la receta. Si el usuario no "
                                "menciona una cantidad, usala como 1."
                            ),
                        },
                    },
                    "required": ["nombre"],
                },
                "description": (
                    "Lista de ingredientes reales para una preparacion o receta, "
                    "con su cantidad en envases, cuando el pedido sugiere hacer una "
                    "comida casera. Si el pedido no es una receta, NO incluyas esta "
                    "clave."
                ),
            },
        },
        "required": [],
    },
}


# ===== Prompt de explicación (etapa 3) =====


def construir_prompt_explicacion(mensaje_usuario: str, productos_filtrados: list) -> str:
    """Arma el prompt común a Gemini y Groq con un resumen de los productos validados."""
    # Ya vienen ordenados por precio: los primeros bastan y un prompt corto responde más rápido.
    resumen = [
        {
            "nombre": producto["nombre"],
            "precio": producto["precio"],
            "categoria": producto["categoria"],
            "distancia_km": producto["distancia_km"],
        }
        for producto in productos_filtrados[: config.EXPLICACION_MAX_PRODUCTOS]
    ]

    return (
        f'El usuario pidio: "{mensaje_usuario}"\n\n'
        "Estos son los UNICOS productos reales disponibles que calzan con "
        "su busqueda (ya fueron filtrados por precio/categoria/distancia "
        "por el sistema, ordenados del mas barato al mas caro):\n"
        f"{json.dumps(resumen, ensure_ascii=False)}\n\n"
        "Redacta una respuesta breve en espanol (sin maximo de oraciones) "
        "explicando por que estos productos calzan con lo que pidio. "
        "Responde en texto plano, sin formato Markdown. "
        "Menciona nombres y precios reales de la lista. No inventes "
        "productos, precios, tiendas ni caracteristicas que no esten en "
        "la lista. No hagas preguntas de seguimiento, es una respuesta "
        "unica."
    )
