"""
Reglas deterministas de los filtros: saneamiento de lo que extrae el LLM
(etapa 1) y filtrado del catálogo real (etapa 2).

Aquí el modelo nunca elige productos: el código descarta los campos mal
tipados, aplica los filtros sobre los productos de Catálogo y los ordena por
precio ascendente. No depende de HTTP ni de proveedores.
"""

import re

# ===== Saneamiento de filtros =====


def _a_numero(valor) -> float | None:
    """Acepta números o textos numéricos; descarta lo demás (p. ej. 'omision') devolviendo None."""
    if isinstance(valor, (int, float)):
        return valor
    if isinstance(valor, str):
        try:
            return float(valor)
        except ValueError:
            return None
    return None


def _a_lista_de_terminos(valor) -> list[str]:
    """Normaliza palabras clave: lista de textos o un texto suelto separado por comas."""
    if isinstance(valor, list):
        terminos = valor
    elif isinstance(valor, str):
        # Visto en pruebas: a veces el modelo manda "tomate, ajo" en vez de una lista.
        terminos = valor.split(",")
    else:
        return []
    return [termino.strip() for termino in terminos if isinstance(termino, str) and termino.strip()]


def sanitizar_filtros_crudos(crudo: dict) -> dict:
    """
    Conserva solo los campos con el tipo esperado.

    Descartar un campo mal tipado sale mejor que perder la extracción completa.
    """
    limpio = {}

    precio_max = _a_numero(crudo.get("precio_max"))
    if precio_max is not None:
        limpio["precio_max"] = precio_max

    categoria = crudo.get("categoria")
    if isinstance(categoria, str) and categoria.strip():
        limpio["categoria"] = categoria

    distancia_max = _a_numero(crudo.get("distancia_max"))
    if distancia_max is not None:
        limpio["distancia_max"] = distancia_max

    palabras_clave = _a_lista_de_terminos(crudo.get("palabras_clave"))
    if palabras_clave:
        limpio["palabras_clave"] = palabras_clave

    return limpio


# ===== Coincidencia por texto =====

# Palabras vacías que no aportan al comparar categorías ("frutas y verduras").
PALABRAS_VACIAS_CATEGORIA = {"y", "o", "u", "de", "del", "la", "el", "los", "las", "en", "con"}

# Bug real con "bajo consumo": "bajo" contiene "ajo" y calzaba con el producto
# Ajo. Bajo este largo se exige igualdad exacta en vez de contención.
LONGITUD_MINIMA_SUBCADENA = 4

PATRON_SEPARADOR_TERMINOS = re.compile(r"[^a-zA-Zñáéíóúü]+")


def _separar_terminos(texto: str) -> list[str]:
    """Divide un texto en términos alfabéticos y quita las palabras vacías."""
    terminos = PATRON_SEPARADOR_TERMINOS.split(texto)
    return [termino for termino in terminos if termino and termino not in PALABRAS_VACIAS_CATEGORIA]


def _texto_contiene(texto_primero: str, texto_segundo: str) -> bool:
    """Compara por contención en ambos sentidos, o por igualdad si alguno es más corto que el mínimo."""
    if len(texto_primero) < LONGITUD_MINIMA_SUBCADENA or len(texto_segundo) < LONGITUD_MINIMA_SUBCADENA:
        return texto_primero == texto_segundo
    return texto_primero in texto_segundo or texto_segundo in texto_primero


def _coincide_por_texto(frase: str, producto: dict) -> bool:
    """Indica si la frase, completa o alguno de sus términos, calza con la categoría o el nombre del producto."""
    frase = frase.strip().lower()
    if not frase:
        return False

    categoria_producto = producto["categoria"].lower()
    nombre_producto = producto["nombre"].lower()

    candidatos = [frase, *_separar_terminos(frase)]
    return any(
        _texto_contiene(candidato, categoria_producto) or _texto_contiene(candidato, nombre_producto)
        for candidato in candidatos
    )


# ===== Filtrado del catálogo =====


def filtrar_productos(catalogo: list, filtros: dict) -> list:
    """
    Aplica precio máximo, categoría y distancia máxima, y ordena por precio.

    `palabras_clave` no filtra a propósito: el modelo lo restringía tanto que
    casi cualquier pregunta sin una palabra concreta terminaba sin resultados.
    """
    resultado = catalogo

    precio_max = filtros.get("precio_max")
    if isinstance(precio_max, (int, float)) and precio_max > 0:
        resultado = [producto for producto in resultado if producto["precio"] <= precio_max]

    categoria = filtros.get("categoria")
    if isinstance(categoria, str) and categoria.strip():
        resultado = [producto for producto in resultado if _coincide_por_texto(categoria, producto)]

    distancia_max = filtros.get("distancia_max")
    if isinstance(distancia_max, (int, float)) and distancia_max > 0:
        resultado = [producto for producto in resultado if producto["distancia_km"] <= distancia_max]

    return sorted(resultado, key=lambda producto: producto["precio"])
