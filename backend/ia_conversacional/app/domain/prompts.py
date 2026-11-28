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
from app.domain.filtros import formatear_precio_clp

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
                    "Categoria amplia nombrada explicitamente por el usuario "
                    "cuando NO pide un producto especifico, por ejemplo: "
                    "'verduras', 'frutas', 'lacteos', 'panaderia'. Si el "
                    "usuario pide un producto concreto (ej. 'aceite de oliva', "
                    "'oliva', 'harina almendras', 'leche sin lactosa'), NO "
                    "incluyas ni infieras esta clave; usa solo 'palabras_clave'."
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
                    "Lista de terminos de busqueda de productos. Si el usuario "
                    "pide un producto especifico (incluso de varias palabras, "
                    "como 'aceite de oliva', 'harina almendras', 'leche sin "
                    "lactosa', 'azucar flor' o 'jamon iberico'), conserva la "
                    "frase completa en un solo elemento (ej. ['aceite de oliva'], "
                    "['harina de almendras']) sin dividirla en palabras sueltas "
                    "ni agregar genericos o sustitutos (no agregues 'aceite', "
                    "'aceite vegetal' ni 'harina' suelta). Solo expande a "
                    "productos relacionados cuando el pedido sea un plato, "
                    "receta o concepto amplio (ej. 'quiero hacer pebre' -> "
                    "['tomate', 'cebolla', 'cilantro', 'aji verde', 'limon']; "
                    "'quiero hacer cazuela' -> ['papa', 'zapallo', 'choclo', "
                    "'carne', 'zanahoria']; 'comida salada con platano' -> "
                    "['platano', 'tostones', 'platano frito']). Si el pedido es "
                    "solo una categoria amplia y generica (ej. 'verduras "
                    "baratas'), usa 'categoria' en vez de esto y deja este "
                    "campo vacio."
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


def _resumir_producto_para_explicacion(producto: dict) -> dict:
    """Extrae solo nombre, marca, precio formateado y tienda; nunca distancia ni slugs de categoría."""
    item = {
        "nombre": str(producto.get("nombre", "")),
        "precio": formatear_precio_clp(producto["precio"]),
    }
    marca = str(producto.get("marca") or "").strip()
    if marca:
        item["marca"] = marca
    tienda = str(producto.get("supermercado") or "").strip()
    if tienda:
        item["tienda"] = tienda
    return item


def construir_prompt_explicacion(mensaje_usuario: str, productos_filtrados: list) -> str:
    """Arma el prompt común a Groq y Gemini solo con datos verificables (nombre, marca, precio y tienda)."""
    # Se limita a los primeros resultados más relevantes para mantener el prompt corto y rápido.
    resumen = [
        _resumir_producto_para_explicacion(producto)
        for producto in productos_filtrados[: config.EXPLICACION_MAX_PRODUCTOS]
    ]

    return (
        f'El usuario pidió: "{mensaje_usuario}"\n\n'
        "Estos son los ÚNICOS productos reales devueltos por el catálogo "
        "(ordenados por relevancia):\n"
        f"{json.dumps(resumen, ensure_ascii=False)}\n\n"
        "Reglas estrictas para redactar tu respuesta en español:\n"
        "- Responde en texto plano, sin formato Markdown, en 2 o 3 oraciones breves y sin preguntas de seguimiento.\n"
        "- Solo puedes afirmar nombre, marca, precio y tienda presentes en la lista entregada.\n"
        "- Copia cada precio exactamente como viene formateado (por ejemplo, $10.600), sin agregar decimales ni espacios entre dígitos.\n"
        "- Prohibido inventar propiedades, ingredientes, atributos nutricionales o usos que no figuren literalmente en el nombre del producto (por ejemplo: sin gluten, vegano, orgánico o sustituto), y no menciones distancias ni categorías.\n"
        "- Si ningún producto de la lista contiene el término que buscó el usuario, dilo claramente (por ejemplo: 'No encontré coincidencias exactas para lo que pediste') en lugar de presentar otros productos como equivalentes o sustitutos.\n"
        "- No repitas la lista completa de tarjetas: el usuario ya ve todos los productos debajo; resume brevemente o destaca solo 1 o 2 opciones relevantes."
    )
