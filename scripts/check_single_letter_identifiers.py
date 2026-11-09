#!/usr/bin/env python3
"""
Validador de nomenclatura para CI y pre-commit.
Rechaza identificadores de una sola letra (como 'c', 'r', 'p', 'w') en declaraciones
de código nuevas o modificadas, según la regla de estilo de CLAUDE.md / GEMINI.md:
"Nombres descriptivos en español para variables, parámetros y receptores (configuracion,
listaID, respuesta), nunca c, p, u o l...".

Excepción permitida: '_' (identificador mudo o descarte).

Uso:
  python3 scripts/check_single_letter_identifiers.py [--base origin/develop]
  python3 scripts/check_single_letter_identifiers.py --staged
  python3 scripts/check_single_letter_identifiers.py --file ruta/al/archivo.go
"""

from __future__ import annotations

import argparse
import ast
import os
import re
import subprocess
import sys

EXTENSIONES_VALIDAS = {".go", ".py", ".ts", ".tsx", ".js", ".mjs"}


def enmascarar_comentarios_y_cadenas(codigo_fuente: str, extension: str) -> str:
    """
    Reemplaza comentarios y cadenas literales por espacios, conservando los
    saltos de linea ('\\n') para preservar exactamente la numeracion de lineas.
    """
    resultado_caracteres: list[str] = []
    indice = 0
    total_caracteres = len(codigo_fuente)
    es_lenguaje_go = (extension == ".go")
    es_lenguaje_javascript = (extension in {".js", ".mjs", ".ts", ".tsx"})

    while indice < total_caracteres:
        # Comentario de una linea //
        if codigo_fuente[indice:indice + 2] == "//":
            resultado_caracteres.append("  ")
            indice += 2
            while indice < total_caracteres and codigo_fuente[indice] != "\n":
                resultado_caracteres.append(" ")
                indice += 1
            continue

        # Comentario de bloque /* ... */
        if codigo_fuente[indice:indice + 2] == "/*":
            resultado_caracteres.append("  ")
            indice += 2
            while indice < total_caracteres and codigo_fuente[indice:indice + 2] != "*/":
                if codigo_fuente[indice] == "\n":
                    resultado_caracteres.append("\n")
                else:
                    resultado_caracteres.append(" ")
                indice += 1
            if indice < total_caracteres:
                resultado_caracteres.append("  ")
                indice += 2
            continue

        # Cadenas raw en Go o plantillas multilinea en JS/TS
        if codigo_fuente[indice] == "`" and (es_lenguaje_go or es_lenguaje_javascript):
            resultado_caracteres.append(" ")
            indice += 1
            while indice < total_caracteres and codigo_fuente[indice] != "`":
                if codigo_fuente[indice] == "\n":
                    resultado_caracteres.append("\n")
                else:
                    resultado_caracteres.append(" ")
                indice += 1
            if indice < total_caracteres:
                resultado_caracteres.append(" ")
                indice += 1
            continue

        # Cadena con comillas dobles "..."
        if codigo_fuente[indice] == '"':
            resultado_caracteres.append(" ")
            indice += 1
            while indice < total_caracteres and codigo_fuente[indice] != '"':
                if codigo_fuente[indice] == "\\":
                    resultado_caracteres.append("  ")
                    indice += 2
                    continue
                if codigo_fuente[indice] == "\n":
                    resultado_caracteres.append("\n")
                else:
                    resultado_caracteres.append(" ")
                indice += 1
            if indice < total_caracteres:
                resultado_caracteres.append(" ")
                indice += 1
            continue

        # Caracter o cadena con comillas simples '...'
        if codigo_fuente[indice] == "'":
            resultado_caracteres.append(" ")
            indice += 1
            while indice < total_caracteres and codigo_fuente[indice] != "'":
                if codigo_fuente[indice] == "\\":
                    resultado_caracteres.append("  ")
                    indice += 2
                    continue
                if codigo_fuente[indice] == "\n":
                    resultado_caracteres.append("\n")
                else:
                    resultado_caracteres.append(" ")
                indice += 1
            if indice < total_caracteres:
                resultado_caracteres.append(" ")
                indice += 1
            continue

        resultado_caracteres.append(codigo_fuente[indice])
        indice += 1

    return "".join(resultado_caracteres)


class DetectorPython(ast.NodeVisitor):
    """Analizador sintactico para archivos Python basado en el AST oficial."""

    def __init__(self) -> None:
        self.violaciones: list[tuple[int, str, str]] = []

    def _verificar_nombre(self, nombre_identificador: str, numero_linea: int, tipo_declaracion: str) -> None:
        if len(nombre_identificador) == 1 and nombre_identificador.isalpha() and nombre_identificador != "_":
            self.violaciones.append((numero_linea, nombre_identificador, tipo_declaracion))

    def _extraer_identificadores_asignacion(self, nodo_objetivo: ast.AST, tipo_declaracion: str) -> None:
        if isinstance(nodo_objetivo, ast.Name):
            self._verificar_nombre(nodo_objetivo.id, nodo_objetivo.lineno, tipo_declaracion)
        elif isinstance(nodo_objetivo, (ast.Tuple, ast.List)):
            for elemento in nodo_objetivo.elts:
                self._extraer_identificadores_asignacion(elemento, tipo_declaracion)
        elif isinstance(nodo_objetivo, ast.Starred):
            self._extraer_identificadores_asignacion(nodo_objetivo.value, tipo_declaracion)

    def _analizar_argumentos_funcion(self, nodo_argumentos: ast.arguments) -> None:
        todos_los_argumentos = (
            getattr(nodo_argumentos, "posonlyargs", []) +
            nodo_argumentos.args +
            nodo_argumentos.kwonlyargs
        )
        for argumento in todos_los_argumentos:
            self._verificar_nombre(argumento.arg, argumento.lineno, "parámetro de función")
        if nodo_argumentos.vararg:
            self._verificar_nombre(nodo_argumentos.vararg.arg, nodo_argumentos.vararg.lineno, "parámetro *args")
        if nodo_argumentos.kwarg:
            self._verificar_nombre(nodo_argumentos.kwarg.arg, nodo_argumentos.kwarg.lineno, "parámetro **kwargs")

    def visit_FunctionDef(self, nodo: ast.FunctionDef) -> None:
        self._analizar_argumentos_funcion(nodo.args)
        self.generic_visit(nodo)

    def visit_AsyncFunctionDef(self, nodo: ast.AsyncFunctionDef) -> None:
        self._analizar_argumentos_funcion(nodo.args)
        self.generic_visit(nodo)

    def visit_Lambda(self, nodo: ast.Lambda) -> None:
        self._analizar_argumentos_funcion(nodo.args)
        self.generic_visit(nodo)

    def visit_Assign(self, nodo: ast.Assign) -> None:
        for objetivo in nodo.targets:
            self._extraer_identificadores_asignacion(objetivo, "asignación de variable")
        self.generic_visit(nodo)

    def visit_AnnAssign(self, nodo: ast.AnnAssign) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "asignación tipada")
        self.generic_visit(nodo)

    def visit_AugAssign(self, nodo: ast.AugAssign) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "asignación compuesta")
        self.generic_visit(nodo)

    def visit_For(self, nodo: ast.For) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "variable de bucle 'for'")
        self.generic_visit(nodo)

    def visit_AsyncFor(self, nodo: ast.AsyncFor) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "variable de bucle 'for'")
        self.generic_visit(nodo)

    def visit_NamedExpr(self, nodo: ast.NamedExpr) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "operador walrus ':='")
        self.generic_visit(nodo)

    def visit_comprehension(self, nodo: ast.comprehension) -> None:
        self._extraer_identificadores_asignacion(nodo.target, "variable de comprensión")
        self.generic_visit(nodo)

    def visit_With(self, nodo: ast.With) -> None:
        for elemento in nodo.items:
            if elemento.optional_vars:
                self._extraer_identificadores_asignacion(elemento.optional_vars, "cláusula 'with ... as'")
        self.generic_visit(nodo)

    def visit_AsyncWith(self, nodo: ast.AsyncWith) -> None:
        for elemento in nodo.items:
            if elemento.optional_vars:
                self._extraer_identificadores_asignacion(elemento.optional_vars, "cláusula 'with ... as'")
        self.generic_visit(nodo)

    def visit_ExceptHandler(self, nodo: ast.ExceptHandler) -> None:
        if nodo.name:
            linea_excepcion = getattr(nodo, "lineno", 1)
            self._verificar_nombre(nodo.name, linea_excepcion, "cláusula 'except ... as'")
        self.generic_visit(nodo)


def analizar_python(codigo_fuente: str) -> list[tuple[int, str, str]]:
    """Parsea el archivo Python con ast y reporta identificadores de 1 letra."""
    try:
        arbol_sintactico = ast.parse(codigo_fuente)
    except SyntaxError:
        return []
    detector = DetectorPython()
    detector.visit(arbol_sintactico)
    return detector.violaciones


def analizar_go(codigo_enmascarado: str) -> list[tuple[int, str, str]]:
    """Detecta declaraciones en Go sobre el codigo con comentarios/cadenas enmascarados."""
    violaciones: list[tuple[int, str, str]] = []

    def registrar_hallazgo(posicion_caracter: int, nombre_token: str, descripcion: str) -> None:
        if len(nombre_token) == 1 and nombre_token.isalpha() and nombre_token != "_":
            numero_linea = codigo_enmascarado[:posicion_caracter].count("\n") + 1
            violaciones.append((numero_linea, nombre_token, descripcion))

    # 1. Firmas de funciones y metodos (receptor y parametros)
    patron_funciones_go = re.compile(
        r"\bfunc\s*(?:\((?P<receptor>[^)]*)\)\s*)?(?:[a-zA-Z0-9_]+\s*)?\((?P<parametros>[^)]*)\)"
    )
    for coincidencia in patron_funciones_go.finditer(codigo_enmascarado):
        # Analisis de receptor: func (servidor *Servidor) o func (s *Servidor)
        if coincidencia.group("receptor"):
            texto_receptor = coincidencia.group("receptor")
            desplazamiento_receptor = coincidencia.start("receptor")
            tokens_receptor = texto_receptor.strip().split()
            if tokens_receptor:
                identificador_receptor = tokens_receptor[0].lstrip("*")
                posicion_relativa = texto_receptor.find(tokens_receptor[0])
                registrar_hallazgo(desplazamiento_receptor + posicion_relativa, identificador_receptor, "receptor de método")

        # Analisis de parametros: func Atender(w http.ResponseWriter, r *http.Request)
        if coincidencia.group("parametros"):
            texto_parametros = coincidencia.group("parametros")
            desplazamiento_parametros = coincidencia.start("parametros")
            for coincidencia_parametro in re.finditer(r"(?:^|[\n,;])\s*([a-zA-Z0-9_*]+)", texto_parametros):
                identificador_parametro = coincidencia_parametro.group(1).lstrip("*")
                posicion_absoluta = desplazamiento_parametros + coincidencia_parametro.start(1)
                registrar_hallazgo(posicion_absoluta, identificador_parametro, "parámetro de función")

    # 2. Declaracion corta := (ej: c, err := fn() o for i, valor := range)
    patron_asignacion_corta = re.compile(r"(?:^|[\n;{(])([^\n;{(]+?):=")
    for coincidencia in patron_asignacion_corta.finditer(codigo_enmascarado):
        texto_lado_izquierdo = coincidencia.group(1)
        desplazamiento_base = coincidencia.start(1)
        coincidencia_palabra_clave = re.match(r"^\s*(?:for|if|switch)\s+", texto_lado_izquierdo)
        if coincidencia_palabra_clave:
            texto_lado_izquierdo = texto_lado_izquierdo[coincidencia_palabra_clave.end():]
            desplazamiento_base += coincidencia_palabra_clave.end()
        for coincidencia_token in re.finditer(r"\b([a-zA-Z0-9_]+)\b", texto_lado_izquierdo):
            identificador_variable = coincidencia_token.group(1)
            posicion_variable = desplazamiento_base + coincidencia_token.start(1)
            registrar_hallazgo(posicion_variable, identificador_variable, "declaración corta ':='")

    # 3. Declaraciones con palabra clave var
    patron_declaracion_var = re.compile(r"\bvar\s+(?:(?:\((?P<bloque>[^)]*)\))|(?P<simple>[^\n;=]+))")
    for coincidencia in patron_declaracion_var.finditer(codigo_enmascarado):
        if coincidencia.group("bloque"):
            texto_bloque = coincidencia.group("bloque")
            desplazamiento_bloque = coincidencia.start("bloque")
            for linea_var in re.finditer(r"^\s*([a-zA-Z0-9_]+)", texto_bloque, re.MULTILINE):
                identificador_var = linea_var.group(1)
                posicion_var = desplazamiento_bloque + linea_var.start(1)
                registrar_hallazgo(posicion_var, identificador_var, "declaración 'var'")
        elif coincidencia.group("simple"):
            texto_simple = coincidencia.group("simple")
            desplazamiento_simple = coincidencia.start("simple")
            for token_var in re.finditer(r"^\s*([a-zA-Z0-9_]+)", texto_simple):
                identificador_var = token_var.group(1)
                posicion_var = desplazamiento_simple + token_var.start(1)
                registrar_hallazgo(posicion_var, identificador_var, "declaración 'var'")

    return violaciones


def analizar_js_ts(codigo_enmascarado: str) -> list[tuple[int, str, str]]:
    """Detecta declaraciones en JavaScript/TypeScript sobre el codigo enmascarado."""
    violaciones: list[tuple[int, str, str]] = []

    def registrar_hallazgo(posicion_caracter: int, nombre_token: str, descripcion: str) -> None:
        nombre_limpio = nombre_token.lstrip(".")
        if len(nombre_limpio) == 1 and nombre_limpio.isalpha() and nombre_limpio != "_":
            numero_linea = codigo_enmascarado[:posicion_caracter].count("\n") + 1
            violaciones.append((numero_linea, nombre_limpio, descripcion))

    # 1. Declaraciones const / let / var: simples, arrays y objetos
    patron_declaracion_variables = re.compile(
        r"\b(?:const|let|var)\s+(?:(?P<arreglo>\[[^\]]+\])|(?P<objeto>\{[^}]+\})|(?P<identificador>[a-zA-Z0-9_]+))"
    )
    for coincidencia in patron_declaracion_variables.finditer(codigo_enmascarado):
        if coincidencia.group("arreglo"):
            texto_arreglo = coincidencia.group("arreglo")
            base_posicion = coincidencia.start("arreglo")
            contenido_interior = texto_arreglo[1:-1] if texto_arreglo.startswith("[") and texto_arreglo.endswith("]") else texto_arreglo
            for fragmento in contenido_interior.split(","):
                variable_cruda = fragmento.split("=")[0].strip()
                coincidencia_nombre = re.search(r"\b([a-zA-Z0-9_]+)\b", variable_cruda)
                if coincidencia_nombre:
                    nombre_var = coincidencia_nombre.group(1)
                    posicion_en_arreglo = texto_arreglo.find(nombre_var)
                    registrar_hallazgo(base_posicion + (posicion_en_arreglo if posicion_en_arreglo != -1 else 0), nombre_var, "destructuración array")
        elif coincidencia.group("objeto"):
            texto_objeto = coincidencia.group("objeto")
            base_posicion = coincidencia.start("objeto")
            contenido_interior = texto_objeto[1:-1] if texto_objeto.startswith("{") and texto_objeto.endswith("}") else texto_objeto
            for fragmento in contenido_interior.split(","):
                sin_valor_defecto = fragmento.split("=")[0].strip()
                partes_alias = sin_valor_defecto.split(":")
                variable_objetivo = partes_alias[-1].strip()
                coincidencia_nombre = re.search(r"\b([a-zA-Z0-9_]+)\b", variable_objetivo)
                if coincidencia_nombre:
                    nombre_var = coincidencia_nombre.group(1)
                    posicion_en_objeto = texto_objeto.find(nombre_var)
                    registrar_hallazgo(base_posicion + (posicion_en_objeto if posicion_en_objeto != -1 else 0), nombre_var, "destructuración objeto")
        elif coincidencia.group("identificador"):
            nombre_identificador = coincidencia.group("identificador")
            registrar_hallazgo(coincidencia.start("identificador"), nombre_identificador, "declaración de variable")

    # 2. Parametros arrow sin parentesis: r => ...
    patron_arrow_directo = re.compile(r"(?:^|[\s(,;])([a-zA-Z])\s*=>")
    for coincidencia in patron_arrow_directo.finditer(codigo_enmascarado):
        registrar_hallazgo(coincidencia.start(1), coincidencia.group(1), "parámetro de función flecha")

    # 3. Parametros en funciones y arrows con parentesis: (r) => o function(r: Respuesta)
    patron_parametros_funciones = re.compile(
        r"\((?P<parametros_arrow>[^)]*)\)\s*(?::\s*[^=]+)?\s*=>|\bfunction\s*[a-zA-Z0-9_]*\s*\((?P<parametros_normales>[^)]*)\)"
    )
    for coincidencia in patron_parametros_funciones.finditer(codigo_enmascarado):
        texto_parametros = coincidencia.group("parametros_arrow") if coincidencia.group("parametros_arrow") is not None else coincidencia.group("parametros_normales")
        desplazamiento_parametros = coincidencia.start("parametros_arrow") if coincidencia.group("parametros_arrow") is not None else coincidencia.start("parametros_normales")
        if not texto_parametros:
            continue
        for fragmento_parametro in texto_parametros.split(","):
            parametro_crudo = fragmento_parametro.strip()
            if not parametro_crudo:
                continue
            nombre_parametro = parametro_crudo.split(":")[0].split("=")[0].strip()
            coincidencia_nombre = re.search(r"\b([a-zA-Z0-9_]+)\b", nombre_parametro)
            if coincidencia_nombre:
                nombre_var = coincidencia_nombre.group(1)
                posicion_parametro = texto_parametros.find(parametro_crudo)
                registrar_hallazgo(desplazamiento_parametros + (posicion_parametro if posicion_parametro != -1 else 0), nombre_var, "parámetro de función")

    # 4. Bucles for (const r of ...) o for (let i = 0; ...)
    patron_bucles_for = re.compile(r"\bfor\s*\(\s*(?:const|let|var)\s+([a-zA-Z])\s+(?:of|in)\b")
    for coincidencia in patron_bucles_for.finditer(codigo_enmascarado):
        registrar_hallazgo(coincidencia.start(1), coincidencia.group(1), "variable de bucle 'for'")

    return violaciones


def analizar_archivo(ruta_archivo: str, lineas_filtradas: set[int] | None = None) -> list[str]:
    """Lee y analiza un archivo completo, filtrando las violaciones por las lineas indicadas."""
    _, extension = os.path.splitext(ruta_archivo)
    if extension not in EXTENSIONES_VALIDAS:
        return []

    try:
        with open(ruta_archivo, "r", encoding="utf-8", errors="replace") as descriptor_archivo:
            contenido_archivo = descriptor_archivo.read()
    except OSError as error_lectura:
        print(f"⚠️ No se pudo leer {ruta_archivo}: {error_lectura}", file=sys.stderr)
        return []

    violaciones_detectadas: list[tuple[int, str, str]] = []

    if extension == ".py":
        violaciones_detectadas = analizar_python(contenido_archivo)
    elif extension == ".go":
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(contenido_archivo, extension)
        violaciones_detectadas = analizar_go(codigo_enmascarado)
    elif extension in {".ts", ".tsx", ".js", ".mjs"}:
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(contenido_archivo, extension)
        violaciones_detectadas = analizar_js_ts(codigo_enmascarado)

    # Deduplicar preservando orden
    conjunto_vistos: set[tuple[int, str]] = set()
    violaciones_unicas: list[tuple[int, str, str]] = []
    for numero_linea, identificador, tipo_declaracion in violaciones_detectadas:
        clave_unica = (numero_linea, identificador)
        if clave_unica not in conjunto_vistos:
            conjunto_vistos.add(clave_unica)
            violaciones_unicas.append((numero_linea, identificador, tipo_declaracion))

    mensajes_error: list[str] = []
    for numero_linea, identificador, tipo_declaracion in violaciones_unicas:
        if lineas_filtradas is None or numero_linea in lineas_filtradas:
            mensajes_error.append(
                f"{ruta_archivo}:{numero_linea}: identificador '{identificador}' de una sola letra detectado en {tipo_declaracion}."
            )

    return mensajes_error


def extraer_lineas_modificadas_por_archivo(diff_texto: str) -> dict[str, set[int]]:
    """Extrae las lineas anadidas/modificadas del diff agrupadas por archivo."""
    lineas_por_archivo: dict[str, set[int]] = {}
    archivo_actual: str | None = None
    expresion_hunk = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")

    for linea in diff_texto.splitlines():
        if linea.startswith("+++ b/"):
            archivo_actual = linea[6:].strip()
            if archivo_actual not in lineas_por_archivo:
                lineas_por_archivo[archivo_actual] = set()
            continue
        if linea.startswith("+++ /dev/null"):
            archivo_actual = None
            continue
        if archivo_actual and linea.startswith("@@ "):
            coincidencia_hunk = expresion_hunk.match(linea)
            if coincidencia_hunk:
                linea_inicio = int(coincidencia_hunk.group(1))
                cantidad_lineas = int(coincidencia_hunk.group(2)) if coincidencia_hunk.group(2) is not None else 1
                if cantidad_lineas > 0:
                    lineas_por_archivo[archivo_actual].update(range(linea_inicio, linea_inicio + cantidad_lineas))

    return lineas_por_archivo


def obtener_diff(argumentos: argparse.Namespace) -> str:
    """Ejecuta git diff y valida codigos de salida, fallando si la base no existe."""
    if argumentos.staged:
        comando_diff = ["git", "diff", "--cached", "-U0"]
        resultado_diff = subprocess.run(comando_diff, capture_output=True, text=True)
        if resultado_diff.returncode != 0:
            print(f"❌ Error al ejecutar 'git diff --cached' (código {resultado_diff.returncode}):", file=sys.stderr)
            if resultado_diff.stderr:
                print(f"   {resultado_diff.stderr.strip()}", file=sys.stderr)
            sys.exit(2)
        return resultado_diff.stdout

    referencia_base = argumentos.base
    resultado_verificacion_base = subprocess.run(
        ["git", "rev-parse", "--verify", referencia_base],
        capture_output=True,
        text=True
    )
    if resultado_verificacion_base.returncode != 0:
        print(f"❌ Error: la referencia base '{referencia_base}' no existe o no es válida.", file=sys.stderr)
        if resultado_verificacion_base.stderr:
            print(f"   {resultado_verificacion_base.stderr.strip()}", file=sys.stderr)
        sys.exit(2)

    comando_diff = ["git", "diff", "-U0", f"{referencia_base}...HEAD"]
    resultado_diff = subprocess.run(comando_diff, capture_output=True, text=True)
    if resultado_diff.returncode != 0:
        print(f"❌ Error al ejecutar 'git diff' (código {resultado_diff.returncode}):", file=sys.stderr)
        if resultado_diff.stderr:
            print(f"   {resultado_diff.stderr.strip()}", file=sys.stderr)
        sys.exit(2)

    return resultado_diff.stdout


def main() -> int:
    analizador_argumentos = argparse.ArgumentParser(description="Verificador de nombres descriptivos sin una sola letra.")
    analizador_argumentos.add_argument("--base", default="origin/develop", help="Rama base para comparar diff (por defecto: origin/develop)")
    analizador_argumentos.add_argument("--staged", action="store_true", help="Analizar solo cambios en stage (para hook pre-commit)")
    analizador_argumentos.add_argument("--file", help="Analizar un archivo completo")
    argumentos = analizador_argumentos.parse_args()

    errores_encontrados: list[str] = []

    if argumentos.file:
        if not os.path.exists(argumentos.file):
            print(f"❌ Error: archivo no encontrado: {argumentos.file}", file=sys.stderr)
            return 2
        errores_encontrados.extend(analizar_archivo(argumentos.file, lineas_filtradas=None))
    else:
        diff_texto = obtener_diff(argumentos)
        archivos_modificados = extraer_lineas_modificadas_por_archivo(diff_texto)
        for ruta_relativa, lineas_anadidas in archivos_modificados.items():
            _, extension = os.path.splitext(ruta_relativa)
            if extension not in EXTENSIONES_VALIDAS:
                continue
            if not os.path.isfile(ruta_relativa):
                continue
            if not lineas_anadidas:
                continue
            errores_encontrados.extend(analizar_archivo(ruta_relativa, lineas_filtradas=lineas_anadidas))

    if errores_encontrados:
        print("❌ Se encontraron declaraciones con identificadores de una sola letra:")
        for error_descripcion in errores_encontrados:
            print(f"   {error_descripcion}")
        print("\n💡 Regla de estilo (CLAUDE.md / GEMINI.md): usa nombres descriptivos en español")
        print("   (ej. 'indice', 'respuesta', 'peticion', 'configuracion', 'errorObtenido') en lugar de variables de 1 letra.")
        return 1

    print("✅ Verificación de nomenclatura: ningún identificador de una letra en cambios nuevos o modificados.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
