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


def dividir_frases_compuestas(texto: str) -> list[str]:
    """
    Separa una frase compuesta por comas o conjunciones ('o', 'y', 'e', 'u') en términos independientes.

    Permite buscar cada producto por separado en Catálogo (p. ej. "zanahorias o verduras" ->
    ["zanahorias", "verduras"]) sin partir cifras con coma decimal o de miles.
    """
    if not isinstance(texto, str):
        return []
    partes = PATRON_SEPARADOR_FRASES_MENSAJE.split(texto)
    return list(dict.fromkeys(parte.strip() for parte in partes if parte and parte.strip()))


def _a_lista_de_terminos(valor) -> list[str]:
    """Normaliza palabras clave: lista de textos o texto suelto, separando frases unidas por comas o conjunciones."""
    if isinstance(valor, list):
        terminos_crudos = valor
    elif isinstance(valor, str):
        # Visto en pruebas: a veces el modelo manda "tomate, ajo" o "zanahorias o verduras" en un solo texto.
        terminos_crudos = [valor]
    else:
        return []
    resultado = []
    for item in terminos_crudos:
        if isinstance(item, str) and item.strip():
            resultado.extend(dividir_frases_compuestas(item))
    return list(dict.fromkeys(resultado))


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

# Palabras vacías que no aportan al filtrar por texto ni en categorías ("harina de almendras", "frutas y verduras").
# Nota: "sin" NO es palabra vacía porque distingue productos clave ("leche sin lactosa", "sin gluten").
PALABRAS_VACIAS_CATEGORIA = {"y", "o", "u", "de", "del", "la", "el", "los", "las", "en", "con", "para", "por"}

# Palabras vacías adicionales que se descartan al elegir la palabra única para consultar `?q=` en Catálogo.
PALABRAS_VACIAS_BUSQUEDA = frozenset(
    {
        *PALABRAS_VACIAS_CATEGORIA,
        "a",
        "al",
        "e",
        "sin",
        "un",
        "una",
        "unos",
        "unas",
    }
)

# Bug real con "bajo consumo": "bajo" contiene "ajo" y calzaba con el producto Ajo.
# Bajo este largo se exige igualdad exacta en vez de coincidencia por prefijo.
LONGITUD_MINIMA_PREFIJO = 4

# Incluye dígitos para distinguir variantes como "arroz grado 2" frente a "arroz grado 1".
PATRON_SEPARADOR_TERMINOS = re.compile(r"[^a-zA-Z0-9ñáéíóúü]+")
# El respaldo desde el mensaje libre descarta números para no rescatar montos como "$2.000" ("000").
PATRON_SEPARADOR_PALABRAS_RESPALDO = re.compile(r"[^a-zA-Zñáéíóúü]+")

# ===== Términos de respaldo desde el mensaje =====

LONGITUD_MINIMA_TERMINO_RESPALDO = 2

# Separa enumeraciones de productos distintos (",", "y", "e", "o", "u") sin partir montos como "$2,000".
PATRON_SEPARADOR_FRASES_MENSAJE = re.compile(r"(?<!\d),(?!\d)|\b(?:y|e|o|u)\b", re.IGNORECASE)

# Reglas simples de singularización en español (sin librerías externas).
LONGITUD_MINIMA_PLURAL = 4
LONGITUD_MINIMA_PLURAL_ES = 5
LONGITUD_MINIMA_PLURAL_ONES = 6
VOCALES_ESPANOL = frozenset("aeiouáéíóúü")
CONSONANTES_PLURAL_ES = frozenset("lnrdjy")

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
        "tal",
        "es",
        "son",
        "ser",
        "estan",
        "estoy",
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
        "litro",
        "litros",
    }
)


def _separar_terminos(texto: str) -> list[str]:
    """Divide un texto en términos alfanuméricos y quita las palabras vacías."""
    terminos = PATRON_SEPARADOR_TERMINOS.split(texto)
    return [termino for termino in terminos if termino and termino not in PALABRAS_VACIAS_CATEGORIA]


def singularizar_palabra(palabra: str) -> str:
    """
    Convierte una palabra española en plural a su forma singular probable con reglas simples.

    Cubre los casos frecuentes de supermercado sin librerías externas:
    - `-ces` -> `-z` ("nueces" -> "nuez", "arroces" -> "arroz")
    - `-ones` -> `-ón` ("limones" -> "limón", "jamones" -> "jamón")
    - `-íes` / `-ies` -> `-í` / `-i` ("ajíes" -> "ají")
    - `<vocal><l|n|r|d|j|y>es` -> quita `es` ("yogures" -> "yogur", "panes" -> "pan", "cereales" -> "cereal")
    - `<vocal>s` -> quita `s` ("zanahorias" -> "zanahoria", "verduras" -> "verdura", "leches" -> "leche")
    """
    palabra_limpia = palabra.strip()
    if len(palabra_limpia) < LONGITUD_MINIMA_PLURAL or not palabra_limpia.endswith("s"):
        return palabra_limpia
    if len(palabra_limpia) >= LONGITUD_MINIMA_PLURAL_ES and palabra_limpia.endswith("ces"):
        return f"{palabra_limpia[:-3]}z"
    if len(palabra_limpia) >= LONGITUD_MINIMA_PLURAL_ONES and palabra_limpia.endswith("ones"):
        return f"{palabra_limpia[:-4]}ón"
    if len(palabra_limpia) >= LONGITUD_MINIMA_PLURAL_ES and palabra_limpia.endswith(("íes", "ies")):
        return palabra_limpia[:-2]
    if (
        len(palabra_limpia) >= LONGITUD_MINIMA_PLURAL_ES
        and palabra_limpia.endswith("es")
        and palabra_limpia[-3] in CONSONANTES_PLURAL_ES
        and palabra_limpia[-4] in VOCALES_ESPANOL
    ):
        return palabra_limpia[:-2]
    if palabra_limpia[-2] in VOCALES_ESPANOL:
        return palabra_limpia[:-1]
    return palabra_limpia


def singularizar_frase(frase: str) -> str:
    """Singulariza cada palabra de una frase manteniendo el resto intacto."""
    return " ".join(singularizar_palabra(palabra) for palabra in frase.split())


def formatear_precio_clp(precio: float | int) -> str:
    """Formatea un precio en pesos chilenos con signo peso y punto de miles (ej. 10600.0 -> '$10.600')."""
    return f"${round(precio):,}".replace(",", ".")


def _texto_contiene(texto_primero: str, texto_segundo: str) -> bool:
    """
    Compara dos palabras por igualdad, singular equivalente o prefijo de palabra, nunca por subcadena interna.

    Evita falsos positivos como "leche" en "Loncoleche" y, bajo `LONGITUD_MINIMA_PREFIJO`,
    exige igualdad exacta (o singular exacto como "panes" == "pan") para evitar choques
    en términos cortos ("bajo" vs "ajo", "sal" vs "salsa").
    """
    if texto_primero == texto_segundo:
        return True
    singular_primero = _normalizar_texto(singularizar_palabra(texto_primero))
    singular_segundo = _normalizar_texto(singularizar_palabra(texto_segundo))
    if singular_primero == singular_segundo:
        return True
    if len(texto_primero) < LONGITUD_MINIMA_PREFIJO or len(texto_segundo) < LONGITUD_MINIMA_PREFIJO:
        return False
    return texto_segundo.startswith(texto_primero) or texto_primero.startswith(texto_segundo)


def _normalizar_texto(texto: str) -> str:
    """Quita tildes (NFD) y pasa a minúsculas para comparar sin importar la forma del texto."""
    texto_nfd = unicodedata.normalize("NFD", texto)
    letras_sin_marcas = [letra for letra in texto_nfd if unicodedata.category(letra) != "Mn"]
    return "".join(letras_sin_marcas).strip().lower()


def categoria_mencionada_en_mensaje(categoria: str, mensaje: str) -> bool:
    """
    Indica si el usuario nombró explícitamente la categoría en su mensaje.

    Evalúa categorías simples ("verduras") y compuestas ("frutas y verduras")
    comparando tokens normalizados sin tildes y admitiendo singular/plural,
    para distinguir una categoría pedida por el usuario de una inferida por el LLM.
    """
    if not isinstance(categoria, str) or not isinstance(mensaje, str):
        return False
    categoria_normalizada = _normalizar_texto(categoria)
    mensaje_normalizado = _normalizar_texto(mensaje)
    if not categoria_normalizada or not mensaje_normalizado:
        return False

    tokens_mensaje = _separar_terminos(mensaje_normalizado)
    if not tokens_mensaje:
        return False

    grupos = re.split(r"\b(?:y|o|u)\b", categoria_normalizada)
    return any(
        all(any(_texto_contiene(termino, token) for token in tokens_mensaje) for termino in _separar_terminos(grupo))
        for grupo in grupos
        if _separar_terminos(grupo)
    )


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
        tokens = PATRON_SEPARADOR_PALABRAS_RESPALDO.split(segmento)
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


# Tope de términos sueltos que se prueban cuando la frase completa no devuelve resultados.
MAXIMO_TERMINOS_FALLBACK_CATALOGO = 3


def _terminos_una_palabra(frase_minuscula: str) -> list[str]:
    """Conserva el comportamiento original de una sola palabra: tal cual, singular y sin tildes."""
    candidatos = [frase_minuscula]
    palabra_singular = singularizar_palabra(frase_minuscula)
    if palabra_singular != frase_minuscula:
        candidatos.append(palabra_singular)
    candidatos.extend([_normalizar_texto(candidato) for candidato in candidatos])
    return list(dict.fromkeys(candidato for candidato in candidatos if candidato and admitido_por_catalogo(candidato)))


def terminos_busqueda_catalogo(frase: str) -> list[str]:
    """
    Devuelve los términos iniciales `?q=` para buscar una frase en Catálogo.

    - Una sola palabra mantiene el comportamiento original: la palabra tal cual, su
      singular y las variantes sin tildes. El llamador las prueba en orden.
    - Varias palabras envían la frase completa: Catálogo la trata como frase y ordena
      por relevancia, así que los productos buenos llegan al filtro de la IA. Si no
      hay coincidencias, el llamador usa `terminos_fallback_busqueda_catalogo`.
    """
    frase_minuscula = " ".join(frase.lower().split())
    if not frase_minuscula:
        return []
    if " " not in frase_minuscula:
        return _terminos_una_palabra(frase_minuscula)
    return [frase_minuscula] if admitido_por_catalogo(frase_minuscula) else []


def terminos_fallback_busqueda_catalogo(frase: str, maximo: int = MAXIMO_TERMINOS_FALLBACK_CATALOGO) -> list[str]:
    """
    Devuelve hasta `maximo` términos sueltos para cuando la frase completa dio 0 resultados.

    Descarta conectores y "sin" (`PALABRAS_VACIAS_BUSQUEDA`), agrega el singular de cada
    palabra para no perder variantes como "harina de almendra" y omite lo que Catálogo
    rechazaría con 400 (menos de 3 bytes tras sanear). "arroz" en "arroz grado 2" gana
    antes que el número suelto por el propio filtro de admisión.
    """
    frase_minuscula = " ".join(frase.lower().split())
    candidatos: list[str] = []
    for token in PATRON_SEPARADOR_TERMINOS.split(frase_minuscula):
        if not token or _normalizar_texto(token) in PALABRAS_VACIAS_BUSQUEDA or not admitido_por_catalogo(token):
            continue
        candidatos.append(token)
        palabra_singular = singularizar_palabra(token)
        if palabra_singular != token and admitido_por_catalogo(palabra_singular):
            candidatos.append(palabra_singular)
    return list(dict.fromkeys(candidatos))[:maximo]


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


def _filtrar_por_palabras_clave(productos: list, palabras_clave: list[str]) -> list:
    """
    Conserva los productos que coinciden con TODAS las palabras de al menos una frase buscada.

    Reutiliza `_coincide_por_texto(..., exigir_todos=True)` (coincidencia por palabra o
    prefijo y plural/singular). Como Catálogo se consulta con una sola palabra (p. ej.
    `q=almendras`), este filtro descarta falsos positivos ("Almendras 200 g") y conserva
    solo los productos que cumplen el término completo ("Harina de Almendras").
    """
    return [
        producto
        for producto in productos
        if any(_coincide_por_texto(frase, producto, exigir_todos=True) for frase in palabras_clave)
    ]


def filtrar_productos(catalogo: list, filtros: dict, preservar_orden: bool = False) -> list:
    """
    Aplica precio máximo, categoría, palabras clave y distancia máxima.

    Cuando `preservar_orden` es True (resultados recibidos directamente de Catálogo),
    conserva intacto el orden de relevancia que devolvió Catálogo. En caso contrario,
    ordena por relevancia local y precio ascendente (o solo precio ascendente sin palabras clave).
    """
    resultado = catalogo

    precio_max = filtros.get("precio_max")
    if isinstance(precio_max, (int, float)) and precio_max > 0:
        resultado = [producto for producto in resultado if producto["precio"] <= precio_max]

    palabras_clave = _a_lista_de_terminos(filtros.get("palabras_clave"))
    categoria = filtros.get("categoria")
    tiene_categoria = isinstance(categoria, str) and bool(categoria.strip())

    if palabras_clave:
        resultado = _filtrar_por_palabras_clave(resultado, palabras_clave)
        if tiene_categoria:
            # Con palabras clave presentes, la categoría solo acota si conserva al menos un
            # producto: los slugs de Catálogo (p. ej. "despensa") no siempre coinciden con el
            # nombre que usa el usuario o el modelo (p. ej. "abarrotes").
            con_categoria = [
                producto for producto in resultado if _coincide_por_texto(categoria, producto, exigir_todos=False)
            ]
            if con_categoria:
                resultado = con_categoria
    elif tiene_categoria:
        # Sin palabras clave: la categoría es el criterio principal y las compuestas ("frutas y verduras") son alternativas.
        resultado = [producto for producto in resultado if _coincide_por_texto(categoria, producto, exigir_todos=False)]

    distancia_max = filtros.get("distancia_max")
    if isinstance(distancia_max, (int, float)) and distancia_max > 0:
        resultado = [producto for producto in resultado if producto["distancia_km"] <= distancia_max]

    if preservar_orden:
        return list(resultado)

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
