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

# Bug real con "bajo consumo": "bajo" contiene "ajo" y calzaba con el producto Ajo.
# Bajo este largo se exige igualdad exacta en vez de coincidencia por prefijo.
LONGITUD_MINIMA_PREFIJO = 4

PATRON_SEPARADOR_TERMINOS = re.compile(r"[^a-zA-Zñáéíóúü]+")

# ===== Términos de respaldo desde el mensaje =====

LONGITUD_MINIMA_TERMINO_RESPALDO = 2

# Separa enumeraciones de productos distintos (",", "y", "e", "o", "u") sin partir montos como "$2,000".
PATRON_SEPARADOR_FRASES_MENSAJE = re.compile(r"(?<!\d),(?!\d)|\b(?:y|e|o|u)\b")

# Palabras frecuentes en mensajes de búsqueda que no nombran un producto.
PALABRAS_VACIAS_MENSAJE = frozenset(
    {
        *PALABRAS_VACIAS_CATEGORIA,
        "a",
        "al",
        "e",
        "por",
        "para",
        "sin",
        "un",
        "una",
        "unos",
        "unas",
        "que",
        "me",
        "mi",
        "mis",
        "se",
        "lo",
        "le",
        "les",
        "nos",
        "su",
        "sus",
        "este",
        "esta",
        "estos",
        "estas",
        "eso",
        "esa",
        "algo",
        "algun",
        "alguna",
        "algunos",
        "algunas",
        "todo",
        "todos",
        "cualquier",
        "cualquiera",
        "busco",
        "busca",
        "buscar",
        "quiero",
        "quisiera",
        "querer",
        "necesito",
        "necesita",
        "necesitar",
        "dame",
        "dime",
        "recomienda",
        "recomiendas",
        "recomiendo",
        "recomiendame",
        "recomendacion",
        "muestra",
        "muestrame",
        "ver",
        "comprar",
        "tengo",
        "tiene",
        "tienes",
        "tienen",
        "vende",
        "venden",
        "hay",
        "encontrar",
        "encuentro",
        "conseguir",
        "consigo",
        "disponible",
        "disponibles",
        "disponibilidad",
        "stock",
        "existencia",
        "existencias",
        "vigente",
        "vigentes",
        "actual",
        "actuales",
        "actualmente",
        "catalogo",
        "venta",
        "ahora",
        "hoy",
        "hola",
        "buenas",
        "buenos",
        "dias",
        "tardes",
        "noches",
        "favor",
        "gracias",
        "porfa",
        "puedes",
        "puede",
        "podrias",
        "productos",
        "producto",
        "cosas",
        "cosa",
        "opciones",
        "opcion",
        "supermercado",
        "supermercados",
        "tienda",
        "tiendas",
        "cerca",
        "cercano",
        "cercanos",
        "precio",
        "precios",
        "menos",
        "mas",
        "bajo",
        "abajo",
        "debajo",
        "maximo",
        "max",
        "hasta",
        "entre",
        "desde",
        "pesos",
        "peso",
        "clp",
        "lucas",
        "luca",
        "mil",
        "barato",
        "baratos",
        "barata",
        "baratas",
        "economico",
        "economicos",
        "economica",
        "economicas",
        "oferta",
        "ofertas",
        "descuento",
        "descuentos",
        "km",
        "kilometros",
        "metros",
    }
)


def _separar_terminos(texto: str) -> list[str]:
    """Divide un texto en términos alfabéticos y quita las palabras vacías."""
    terminos = PATRON_SEPARADOR_TERMINOS.split(texto)
    return [termino for termino in terminos if termino and termino not in PALABRAS_VACIAS_CATEGORIA]


def _texto_contiene(texto_primero: str, texto_segundo: str) -> bool:
    """
    Compara dos palabras por igualdad o prefijo de palabra, nunca por subcadena interna.

    Evita falsos positivos como "leche" en "Loncoleche" y, bajo `LONGITUD_MINIMA_PREFIJO`,
    exige igualdad exacta para evitar choques en términos cortos ("bajo" vs "ajo", "sal" vs "salsa").
    """
    if len(texto_primero) < LONGITUD_MINIMA_PREFIJO or len(texto_segundo) < LONGITUD_MINIMA_PREFIJO:
        return texto_primero == texto_segundo
    return texto_segundo.startswith(texto_primero) or texto_primero.startswith(texto_segundo)


def _normalizar_texto(texto: str) -> str:
    """Quita tildes (NFD) y pasa a minúsculas para comparar sin importar la forma del texto."""
    texto_nfd = unicodedata.normalize("NFD", texto)
    letras_sin_marcas = [letra for letra in texto_nfd if unicodedata.category(letra) != "Mn"]
    return "".join(letras_sin_marcas).strip().lower()


def _coincide_por_texto(frase: str, producto: dict, exigir_todos: bool = True) -> bool:
    """Indica si los términos de la frase calzan por palabra o prefijo con la categoría o el nombre del producto.

    Con `exigir_todos` se exige que todos calcen; sin el flag, las frases compuestas
    separadas por conjunción ("frutas y verduras") se evalúan como alternativas: basta con
    que un grupo calce.
    """
    frase_normalizada = _normalizar_texto(frase)
    if not frase_normalizada:
        return False

    categoria_tokens = _separar_terminos(_normalizar_texto(producto.get("categoria") or ""))
    nombre_tokens = _separar_terminos(_normalizar_texto(producto.get("nombre") or ""))
    objetivos = [*categoria_tokens, *nombre_tokens]
    if not objetivos:
        return False

    terminos = _separar_terminos(frase_normalizada) or [frase_normalizada]
    if exigir_todos:
        # Todos los términos de la frase deben calzar: "aji verde" exige "aji" y "verde".
        return all(any(_texto_contiene(termino, objetivo) for objetivo in objetivos) for termino in terminos)
    # Sin exigir todos, la frase puede ser una categoría compuesta ("frutas y verduras"): al
    # separar por conjunción queda una alternancia de subcategorías, así que basta un grupo.
    grupos = re.split(r"\b(?:y|o|u)\b", frase_normalizada)
    return any(
        all(any(_texto_contiene(termino, objetivo) for objetivo in objetivos) for termino in _separar_terminos(grupo))
        for grupo in grupos
        if _separar_terminos(grupo)
    )


def extraer_terminos_respaldo(mensaje: str) -> list[str]:
    """
    Extrae frases útiles de producto del mensaje cuando la extracción no trajo palabras clave, ingredientes ni categoría.

    Conserva cada producto multipalabra como una sola palabra clave (p. ej. "harina de almendras" -> ["harina almendras"])
    para que `filtrar_productos` exija todos sus términos en vez de aceptar cualquiera por separado.
    Descarta números, símbolos de precio, verbos de búsqueda y palabras vacías.
    """
    mensaje_normalizado = _normalizar_texto(mensaje)
    if not mensaje_normalizado:
        return []

    frases_utiles = []
    for segmento in PATRON_SEPARADOR_FRASES_MENSAJE.split(mensaje_normalizado):
        tokens = PATRON_SEPARADOR_TERMINOS.split(segmento)
        terminos_utiles = list(
            dict.fromkeys(
                token
                for token in tokens
                if len(token) >= LONGITUD_MINIMA_TERMINO_RESPALDO and token not in PALABRAS_VACIAS_MENSAJE
            )
        )
        if terminos_utiles:
            frases_utiles.append(" ".join(terminos_utiles))
    return list(dict.fromkeys(frases_utiles))


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
    terminos = [termino for termino in _separar_terminos(frase_minuscula) if len(termino) >= LONGITUD_MINIMA_PREFIJO]
    if len(terminos) > 1 or (terminos and terminos[0] != frase_minuscula):
        candidatos.append(max(terminos, key=len))
    candidatos.extend([_normalizar_texto(candidato) for candidato in candidatos])

    # Conserva el orden (la frase completa primero), quita repetidos y lo que Catálogo rechazaría.
    return list(dict.fromkeys(candidato for candidato in candidatos if candidato and admitido_por_catalogo(candidato)))


# ===== Filtrado del catálogo =====

RELEVANCIA_ALTA = 0
RELEVANCIA_NORMAL = 1


def _nivel_relevancia_producto(producto: dict, palabras_clave: list[str]) -> int:
    """
    Calcula la prioridad de un producto respecto a las palabras clave (menor es más relevante).

    Prioriza coincidencia en la primera palabra del nombre o en la categoría del producto
    (`RELEVANCIA_ALTA`) por sobre menciones secundarias en el nombre (`RELEVANCIA_NORMAL`).
    """
    tokens_nombre = _separar_terminos(_normalizar_texto(producto.get("nombre") or ""))
    primera_palabra = tokens_nombre[0] if tokens_nombre else ""
    tokens_categoria = _separar_terminos(_normalizar_texto(producto.get("categoria") or ""))

    for frase in palabras_clave:
        frase_normalizada = _normalizar_texto(frase)
        if not frase_normalizada:
            continue
        terminos = _separar_terminos(frase_normalizada) or [frase_normalizada]
        coincide_primera_palabra = bool(primera_palabra) and any(
            _texto_contiene(termino, primera_palabra) for termino in terminos
        )
        coincide_categoria = any(
            _texto_contiene(termino, token_categoria) for termino in terminos for token_categoria in tokens_categoria
        )
        if coincide_primera_palabra or coincide_categoria:
            return RELEVANCIA_ALTA

    return RELEVANCIA_NORMAL


def _cantidad_terminos_coincidentes(frase: str, producto: dict) -> int:
    """Cuenta cuántos términos de la frase calzan por palabra o prefijo con la categoría o el nombre del producto."""
    frase_normalizada = _normalizar_texto(frase)
    if not frase_normalizada:
        return 0

    categoria_tokens = _separar_terminos(_normalizar_texto(producto.get("categoria") or ""))
    nombre_tokens = _separar_terminos(_normalizar_texto(producto.get("nombre") or ""))
    objetivos = [*categoria_tokens, *nombre_tokens]
    if not objetivos:
        return 0

    terminos = _separar_terminos(frase_normalizada) or [frase_normalizada]
    return sum(1 for termino in terminos if any(_texto_contiene(termino, objetivo) for objetivo in objetivos))


def _filtrar_por_palabras_clave(productos: list, palabras_clave: list[str]) -> list:
    """
    Conserva para cada frase los productos que calcen el mayor número de sus términos (> 0).

    Si existen productos que coinciden con todos los términos (p. ej. "harina almendras" ->
    "Harina de Almendras"), solo se conservan esos; si ninguno calza todos (p. ej.
    "leche litro" frente a "Leche Entera 1 L"), se conservan los que calcen más términos.
    """
    indices_seleccionados: set[int] = set()
    for frase in palabras_clave:
        puntajes = [
            (indice, _cantidad_terminos_coincidentes(frase, producto)) for indice, producto in enumerate(productos)
        ]
        maximo_coincidencias = max((puntaje for _, puntaje in puntajes), default=0)
        if maximo_coincidencias == 0:
            continue
        for indice, puntaje in puntajes:
            if puntaje == maximo_coincidencias:
                indices_seleccionados.add(indice)
    return [producto for indice, producto in enumerate(productos) if indice in indices_seleccionados]


def filtrar_productos(catalogo: list, filtros: dict) -> list:
    """
    Aplica precio máximo, categoría, palabras clave y distancia máxima.

    Con palabras clave conserva los productos con mayor número de términos coincidentes
    por frase, ordena primero por relevancia (primera palabra del nombre o categoría) y
    luego por precio ascendente; sin palabras clave ordena solo por precio ascendente.
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
        resultado = _filtrar_por_palabras_clave(resultado, palabras_clave)

    distancia_max = filtros.get("distancia_max")
    if isinstance(distancia_max, (int, float)) and distancia_max > 0:
        resultado = [producto for producto in resultado if producto["distancia_km"] <= distancia_max]

    if palabras_clave:
        return sorted(
            resultado,
            key=lambda producto: (_nivel_relevancia_producto(producto, palabras_clave), producto["precio"]),
        )

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
