import json
import re

import httpx

# --------------------------------------------------------------------------
# Saneamiento y rescate de filtros con errores de schema 
# --------------------------------------------------------------------------


def sanitizar_filtros_crudos(crudo: dict) -> dict:
    """
    En vez de perder la llamada entera, se descarta solo el campo que no
    calza con el tipo esperado y se conservan los demas.
    """
    limpio = {}

    precio_max = crudo.get("precio_max")
    if isinstance(precio_max, (int, float)):
        limpio["precio_max"] = precio_max
    elif isinstance(precio_max, str):
        try:
            limpio["precio_max"] = float(precio_max)
        except ValueError:
            pass  # texto no numerico (ej. "omision") -> se descarta

    categoria = crudo.get("categoria")
    if isinstance(categoria, str) and categoria.strip():
        limpio["categoria"] = categoria

    distancia_max = crudo.get("distancia_max")
    if isinstance(distancia_max, (int, float)):
        limpio["distancia_max"] = distancia_max
    elif isinstance(distancia_max, str):
        try:
            limpio["distancia_max"] = float(distancia_max)
        except ValueError:
            pass

    palabras_clave = crudo.get("palabras_clave")
    if isinstance(palabras_clave, list):
        limpio_lista = [termino.strip() for termino in palabras_clave if isinstance(termino, str) and termino.strip()]
        if limpio_lista:
            limpio["palabras_clave"] = limpio_lista
    elif isinstance(palabras_clave, str) and palabras_clave.strip():
        # visto en pruebas: a veces el modelo manda un string suelto
        # ("tomate, ajo") en vez de una lista -- se separa por coma.
        limpio_lista = [termino.strip() for termino in palabras_clave.split(",") if termino.strip()]
        if limpio_lista:
            limpio["palabras_clave"] = limpio_lista

    return limpio


def rescatar_filtros_de_error_groq(response: httpx.Response) -> dict | None:
    try:
        body = response.json()
    except ValueError:
        return None

    failed_generation = body.get("error", {}).get("failed_generation")
    if not failed_generation:
        return None

    if isinstance(failed_generation, str):
        crudo = _extraer_json_de_texto(failed_generation)
        if crudo is None:
            return None
    elif isinstance(failed_generation, dict):
        crudo = failed_generation
    else:
        return None

    return sanitizar_filtros_crudos(crudo)


def _extraer_json_de_texto(texto: str) -> dict | None:

    inicio = texto.find("{")
    fin = texto.rfind("}")
    if inicio == -1 or fin == -1 or fin < inicio:
        return None
    try:
        return json.loads(texto[inicio : fin + 1])
    except json.JSONDecodeError:
        return None


# --------------------------------------------------------------------------
# Matching por texto (categoria/palabras_clave "ruidosas")
# --------------------------------------------------------------------------

_STOPWORDS_CATEGORIA = {"y", "o", "u", "de", "del", "la", "el", "los", "las", "en", "con"}

# Bug real encontrado probando "bajo consumo": el token "bajo" contenia
# literalmente "ajo" como substring y matcheaba el producto "Ajo" sin
# ninguna relacion real asi quepara strings cortos se exige igualdad exacta en
# vez de contencion, que es donde este tipo de falso positivo aparece.
_LONGITUD_MIN_SUBSTRING = 4


def _tokenizar_categoria(texto: str) -> list:

    tokens = re.split(r"[^a-zA-Zñáéíóúü]+", texto)
    return [token for token in tokens if token and token not in _STOPWORDS_CATEGORIA]


def _texto_contiene(texto_primero: str, texto_segundo: str) -> bool:
    if len(texto_primero) < _LONGITUD_MIN_SUBSTRING or len(texto_segundo) < _LONGITUD_MIN_SUBSTRING:
        return texto_primero == texto_segundo
    return texto_primero in texto_segundo or texto_segundo in texto_primero


def _coincide_por_texto(frase: str, producto: dict) -> bool:
    frase = frase.strip().lower()
    if not frase:
        return False

    cat_producto = producto["categoria"].lower()
    nombre_producto = producto["nombre"].lower()

    if _texto_contiene(frase, cat_producto) or _texto_contiene(frase, nombre_producto):
        return True

    tokens = _tokenizar_categoria(frase)
    return any(
        _texto_contiene(tok, cat_producto) or _texto_contiene(tok, nombre_producto)
        for tok in tokens
    )


# --------------------------------------------------------------------------
# Filtrado real del catalogo (aqui el modelo nunca elige productos)
# --------------------------------------------------------------------------


def filtrar_productos(catalogo: list, filtros: dict) -> list:
    """
    quité el filtrado por palabras_clave porque el modelo a veces lo reprime mucho
    finalmente cualquier pregunta sin alguna palabra clave concreta termina en que
    no puede hacer nada 
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
