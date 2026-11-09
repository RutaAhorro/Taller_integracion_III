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
import os
import re
import subprocess
import sys

EXTENSIONES_VALIDAS = {".go", ".py", ".ts", ".tsx", ".js", ".mjs"}

PATRON_RECEPTOR_GO = re.compile(r"func\s*\(\s*([a-zA-Z])\s+\*?[a-zA-Z0-9_]+")
PATRON_VAR_CORTO_GO = re.compile(r"(?:^|[\s,;])([a-zA-Z])\s*:=")
PATRON_VAR_GO = re.compile(r"\bvar\s+([a-zA-Z])\b")
PATRON_FOR_GO = re.compile(r"\bfor\s+([a-zA-Z])\s*[,:]=")
PATRON_FOR_GO_VALOR = re.compile(r"\bfor\s+[a-zA-Z0-9_]+\s*,\s*([a-zA-Z])\s*:=")

PATRON_ASIGNACION_PY = re.compile(r"^\s*([a-zA-Z])\s*=(?!=)")
PATRON_FOR_PY = re.compile(r"\bfor\s+([a-zA-Z])\s+in\b")

PATRON_DECL_JS = re.compile(r"\b(?:const|let|var)\s+([a-zA-Z])\s*[:=]")
PATRON_ARROW_JS = re.compile(r"(?:^|[\s(,])([a-zA-Z])\s*=>")

PATRON_FIRMA_FUNC = re.compile(r"(?:func|def|function)\s*[a-zA-Z0-9_]*\s*\(([^)]*)\)")


def limpiar_comentarios_y_cadenas(linea: str, extension: str) -> str:
    """Elimina comentarios de una línea y cadenas literales simples."""
    if extension in {".go", ".ts", ".tsx", ".js", ".mjs"}:
        linea = re.sub(r"//.*$", "", linea)
    elif extension == ".py":
        linea = re.sub(r"#.*$", "", linea)
    linea = re.sub(r'"(?:\\.|[^"\\])*"', '""', linea)
    linea = re.sub(r"'(?:\\.|[^'\\])*'", "''", linea)
    linea = re.sub(r"`(?:\\.|[^`\\])*`", "``", linea)
    return linea.strip()


def extraer_parametros_invalidos(linea: str) -> list[str]:
    """Detecta parámetros de una sola letra en firmas de funciones."""
    coincidencia = PATRON_FIRMA_FUNC.search(linea)
    if not coincidencia:
        return []
    parametros_raw = coincidencia.group(1).split(",")
    invalidos = []
    for raw in parametros_raw:
        param = raw.strip()
        if not param:
            continue
        nombre = param.split(":")[0].split("=")[0].strip().split()[0] if param else ""
        if len(nombre) == 1 and nombre.isalpha() and nombre != "_":
            invalidos.append(nombre)
    return invalidos


def analizar_linea(linea_original: str, extension: str) -> list[tuple[str, str]]:
    """
    Retorna lista de tuplas (identificador, motivo) si se detecta
    una declaración con nombre de una sola letra.
    """
    limpia = limpiar_comentarios_y_cadenas(linea_original, extension)
    if not limpia:
        return []

    hallazgos: list[tuple[str, str]] = []

    # Parámetros de funciones
    for param in extraer_parametros_invalidos(limpia):
        hallazgos.append((param, "parámetro de función"))

    if extension == ".go":
        match_receptor = PATRON_RECEPTOR_GO.search(limpia)
        if match_receptor:
            hallazgos.append((match_receptor.group(1), "receptor de método"))
        match_var_corto = PATRON_VAR_CORTO_GO.search(limpia)
        if match_var_corto and match_var_corto.group(1) != "_":
            hallazgos.append((match_var_corto.group(1), "declaración corta ':='"))
        match_var = PATRON_VAR_GO.search(limpia)
        if match_var and match_var.group(1) != "_":
            hallazgos.append((match_var.group(1), "declaración 'var'"))
        match_for = PATRON_FOR_GO.search(limpia)
        if match_for and match_for.group(1) != "_":
            hallazgos.append((match_for.group(1), "índice de bucle 'for'"))
        match_for_v = PATRON_FOR_GO_VALOR.search(limpia)
        if match_for_v and match_for_v.group(1) != "_":
            hallazgos.append((match_for_v.group(1), "valor de bucle 'for range'"))

    elif extension == ".py":
        match_asig = PATRON_ASIGNACION_PY.search(limpia)
        if match_asig and match_asig.group(1) != "_":
            hallazgos.append((match_asig.group(1), "asignación de variable"))
        match_for_py = PATRON_FOR_PY.search(limpia)
        if match_for_py and match_for_py.group(1) != "_":
            hallazgos.append((match_for_py.group(1), "variable de bucle 'for'"))

    elif extension in {".ts", ".tsx", ".js", ".mjs"}:
        match_decl = PATRON_DECL_JS.search(limpia)
        if match_decl and match_decl.group(1) != "_":
            hallazgos.append((match_decl.group(1), "declaración 'const/let/var'"))
        match_arrow = PATRON_ARROW_JS.search(limpia)
        if match_arrow and match_arrow.group(1) != "_":
            hallazgos.append((match_arrow.group(1), "parámetro de función flecha"))

    return hallazgos


def procesar_diff(diff_texto: str) -> list[str]:
    """Analiza únicamente las líneas añadidas (+) del diff."""
    hunk_regex = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")
    archivo_actual = None
    linea_actual = 0
    extension_actual = ""
    errores: list[str] = []

    for linea in diff_texto.splitlines():
        if linea.startswith("+++ b/"):
            archivo_actual = linea[6:]
            _, extension_actual = os.path.splitext(archivo_actual)
            continue
        if linea.startswith("@@ "):
            match_hunk = hunk_regex.match(linea)
            if match_hunk:
                linea_actual = int(match_hunk.group(1))
            continue
        if not archivo_actual or extension_actual not in EXTENSIONES_VALIDAS:
            continue

        if linea.startswith("+") and not linea.startswith("+++"):
            contenido = linea[1:]
            for ident, motivo in analizar_linea(contenido, extension_actual):
                errores.append(
                    f"{archivo_actual}:{linea_actual}: identificador '{ident}' de una sola letra detectado en {motivo}."
                )
            linea_actual += 1
        elif not linea.startswith("-"):
            linea_actual += 1

    return errores


def main() -> int:
    parser = argparse.ArgumentParser(description="Verificador de nombres descriptivos sin una sola letra.")
    parser.add_argument("--base", default="origin/develop", help="Rama base para comparar diff (por defecto: origin/develop)")
    parser.add_argument("--staged", action="store_true", help="Analizar solo cambios en stage (para hook pre-commit)")
    parser.add_argument("--file", help="Analizar un archivo completo")
    args = parser.parse_args()

    errores: list[str] = []

    if args.file:
        if not os.path.exists(args.file):
            print(f"Error: archivo no encontrado: {args.file}", file=sys.stderr)
            return 2
        _, ext = os.path.splitext(args.file)
        if ext in EXTENSIONES_VALIDAS:
            with open(args.file, "r", encoding="utf-8", errors="replace") as fh:
                for num, linea in enumerate(fh, 1):
                    for ident, motivo in analizar_linea(linea, ext):
                        errores.append(f"{args.file}:{num}: identificador '{ident}' de una sola letra en {motivo}.")
    elif args.staged:
        cmd = ["git", "diff", "--cached", "-U0"]
        resultado = subprocess.run(cmd, capture_output=True, text=True)
        errores = procesar_diff(resultado.stdout)
    else:
        # Intentar con la base provista o caer a develop / HEAD~1
        base = args.base
        check_base = subprocess.run(["git", "rev-parse", "--verify", base], capture_output=True)
        if check_base.returncode != 0:
            base = "develop"
            check_base2 = subprocess.run(["git", "rev-parse", "--verify", base], capture_output=True)
            if check_base2.returncode != 0:
                base = "HEAD~1"

        cmd = ["git", "diff", "-U0", f"{base}...HEAD"]
        resultado = subprocess.run(cmd, capture_output=True, text=True)
        errores = procesar_diff(resultado.stdout)

    if errores:
        print("❌ Se encontraron declaraciones con identificadores de una sola letra:")
        for err in errores:
            print(f"   {err}")
        print("\n💡 Regla de estilo (CLAUDE.md / GEMINI.md): usa nombres descriptivos en español")
        print("   (ej. 'indice', 'respuesta', 'peticion', 'configuracion', 'errorObtenido') en lugar de variables de 1 letra.")
        return 1

    print("✅ Verificación de nomenclatura: ningún identificador de una letra en cambios nuevos o modificados.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
