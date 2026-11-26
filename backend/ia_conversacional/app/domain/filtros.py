"""
Reglas deterministas de los filtros: saneamiento de lo que extrae el LLM
(etapa 1) y filtrado del catálogo real (etapa 2).

Aquí el modelo nunca elige productos: el código descarta los campos mal
tipados, aplica los filtros sobre los productos de Catálogo y los ordena por
precio ascendente. No depende de HTTP ni de proveedores.
"""

import re
import unicodedata

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


def _a_lista_de_ingredientes(valor) -> list[dict]:
    """
    Normaliza la lista de ingredientes de una receta.

    Cada ingrediente exige un nombre no vacio (se strip) y una cantidad de
    envases entera >= 1 (con default 1 si falta o es invalida, excluyendo bools).
    """
    if not isinstance(valor, list):
        return []
    ingredientes = []
    for item in valor:
        if not isinstance(item, dict):
            continue
        nombre = item.get("nombre")
        if not isinstance(nombre, str):
            continue
        nombre_limpio = nombre.strip()
        if not nombre_limpio:
            continue
        cantidad_envases = item.get("cantidad_envases")
        if (
            isinstance(cantidad_envases, bool)
            or not isinstance(cantidad_envases, (int, float))
            or (isinstance(cantidad_envases, float) and not cantidad_envases.is_integer())
            or cantidad_envases < 1
        ):
            cantidad_envases = 1
        else:
            cantidad_envases = int(cantidad_envases)
        ingredientes.append({"nombre": nombre_limpio, "cantidad_envases": cantidad_envases})
    return ingredientes


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

    ingredientes = _a_lista_de_ingredientes(crudo.get("ingredientes"))
    if ingredientes:
        limpio["ingredientes"] = ingredientes

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


def _normalizar_texto(texto: str) -> str:
    """Quita tildes (NFD) y pasa a minúsculas para comparar sin importar la forma del texto."""
    texto_nfd = unicodedata.normalize("NFD", texto)
    letras_sin_marcas = [letra for letra in texto_nfd if unicodedata.category(letra) != "Mn"]
    return "".join(letras_sin_marcas).strip().lower()


def _coincide_por_texto(frase: str, producto: dict, exigir_todos: bool = True) -> bool:
    """Indica si la frase completa o todos sus términos calzan con la categoría o el nombre del producto.

    Con `exigir_todos` se exige que todos calzen; sin el flag, las frases compuestas
    separadas por conjuncion ("frutas y verduras") se evaluan como alternativas: basta con
    que un grupo calze.
    """
    frase_normalizada = _normalizar_texto(frase)
    if not frase_normalizada:
        return False

    categoria_producto = _normalizar_texto(producto["categoria"])
    nombre_producto = _normalizar_texto(producto["nombre"])
    nombres_tokens = _separar_terminos(nombre_producto)
    terminos = _separar_terminos(frase_normalizada) or [frase_normalizada]
    objetivos = [categoria_producto, nombre_producto, *nombres_tokens]
    if exigir_todos:
        # Todos los términos de la frase deben calzar: "aji verde" exige "aji" y "verde".
        return all(any(_texto_contiene(termino, objetivo) for objetivo in objetivos) for termino in terminos)
    # Sin exigir todos, la frase puede ser una categoria compuesta ("frutas y verduras"): al
    # separar por conjuncion queda una alternancia de subcategorias, asi que basta un grupo.
    grupos = re.split(r"\b(?:y|o|u)\b", frase_normalizada)
    return any(
        all(any(_texto_contiene(termino, objetivo) for objetivo in objetivos) for termino in _separar_terminos(grupo))
        for grupo in grupos
        if _separar_terminos(grupo)
    )


# ===== Términos para la búsqueda en Catálogo =====

# Contrato de Catálogo (services/productos.go): sanea `q` con esta lista de caracteres
# y responde 400 si quedan menos de 3 bytes. Se replica para no enviar consultas
# que fallarían y degradarían todo el mensaje a la caché.
LARGO_MINIMO_BUSQUEDA_CATALOGO = 3
PATRON_CARACTERES_NO_ADMITIDOS_CATALOGO = re.compile(r"[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]")


def admitido_por_catalogo(termino: str) -> bool:
    """Indica si Catálogo acepta el término como `q` (al menos 3 bytes tras sanearlo como él)."""
    saneado = PATRON_CARACTERES_NO_ADMITIDOS_CATALOGO.sub("", termino.strip())
    return len(saneado.encode("utf-8")) >= LARGO_MINIMO_BUSQUEDA_CATALOGO


def terminos_busqueda_catalogo(frase: str) -> list[str]:
    """
    Devuelve los textos que se envían como `?q=` a Catálogo para una palabra clave o ingrediente.

    Catálogo busca la subcadena literal (ILIKE, sensible a tildes): "harina almendras"
    no encontraría "Harina de Almendras". Por eso se envía la frase completa y, si
    tiene varias palabras, también su término más largo; el filtrado determinista
    exige después que calcen todos los términos. Se agregan las variantes sin
    tildes porque los títulos de las cadenas no siempre las llevan.
    """
    frase_minuscula = " ".join(frase.lower().split())
    if not frase_minuscula:
        return []

    candidatos = [frase_minuscula]
    terminos = [termino for termino in _separar_terminos(frase_minuscula) if len(termino) >= LONGITUD_MINIMA_SUBCADENA]
    if len(terminos) > 1 or (terminos and terminos[0] != frase_minuscula):
        candidatos.append(max(terminos, key=len))
    candidatos.extend([_normalizar_texto(candidato) for candidato in candidatos])

    # Conserva el orden (la frase completa primero), quita repetidos y lo que Catálogo rechazaría.
    return list(dict.fromkeys(candidato for candidato in candidatos if candidato and admitido_por_catalogo(candidato)))


# ===== Filtrado del catálogo =====


def filtrar_productos(catalogo: list, filtros: dict) -> list:
    """
    Aplica precio máximo, categoría, palabras clave y distancia máxima, y ordena por precio.
    """
    resultado = catalogo

    precio_max = filtros.get("precio_max")
    if isinstance(precio_max, (int, float)) and precio_max > 0:
        resultado = [producto for producto in resultado if producto["precio"] <= precio_max]

    categoria = filtros.get("categoria")
    if isinstance(categoria, str) and categoria.strip():
        # Sin exigir_todos: las categorías compuestas ("frutas y verduras") matchean productos de cualquiera de sus partes.
        resultado = [producto for producto in resultado if _coincide_por_texto(categoria, producto, exigir_todos=False)]

    palabras_clave = _a_lista_de_terminos(filtros.get("palabras_clave"))
    if palabras_clave:
        resultado = [
            producto
            for producto in resultado
            if any(_coincide_por_texto(termino, producto) for termino in palabras_clave)
        ]

    distancia_max = filtros.get("distancia_max")
    if isinstance(distancia_max, (int, float)) and distancia_max > 0:
        resultado = [producto for producto in resultado if producto["distancia_km"] <= distancia_max]

    return sorted(resultado, key=lambda producto: producto["precio"])


# ===== Vinculación de ingredientes con el catálogo =====


def vincular_ingredientes_catalogo(
    catalogo: list, ingredientes: list[dict], limite_candidatos: int | None = 3
) -> tuple[list[dict], list[str]]:
    """
    Vincula los ingredientes de una receta con los productos reales del catálogo.

    Devuelve los ingredientes resueltos con sus candidatos sugeridos (ordenados
    por precio ascendente, hasta el limite; todos los candidatos si el limite es None) y
    los nombres sin candidato.
    """
    ingredientes_receta = []
    ingredientes_faltantes = []

    for item in ingredientes:
        candidatos = [
            producto
            for producto in catalogo
            if producto.get("id") is not None and _coincide_por_texto(item["nombre"], producto)
        ]
        candidatos = sorted(candidatos, key=lambda producto: producto["precio"])
        if limite_candidatos is not None:
            candidatos = candidatos[:limite_candidatos]

        if not candidatos:
            ingredientes_faltantes.append(item["nombre"])
            continue

        ingredientes_receta.append(
            {
                "ingrediente": item["nombre"],
                "cantidad_sugerida": item["cantidad_envases"],
                "candidatos": [
                    {
                        "producto_id": str(producto["id"]),
                        "nombre": producto["nombre"],
                        "marca": producto.get("marca", ""),
                        "unidad": producto.get("unidad", ""),
                        "precio": producto["precio"],
                        "supermercado": producto.get("supermercado", ""),
                        "url_imagen": producto.get("url_imagen", ""),
                    }
                    for producto in candidatos
                ],
            }
        )

    return ingredientes_receta, ingredientes_faltantes
