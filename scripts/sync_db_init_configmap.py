#!/usr/bin/env python3
"""Sincroniza el ConfigMap db-init-scripts de Kubernetes con infrastructure/db/.

Uso:
    python3 scripts/sync_db_init_configmap.py           # Genera/actualiza infrastructure/k8s/db/configmap.yaml
    python3 scripts/sync_db_init_configmap.py --check   # Valida sin modificar; sale con código 1 si difiere
"""

# ===== Importaciones =====
import argparse
import difflib
import re
import sys
from pathlib import Path

# ===== Rutas y constantes =====
RUTA_RAIZ = Path(__file__).resolve().parent.parent
RUTA_DB_DIR = RUTA_RAIZ / "infrastructure" / "db"
RUTA_README = RUTA_DB_DIR / "README.md"
RUTA_CONFIGMAP_DEFAULT = RUTA_RAIZ / "infrastructure" / "k8s" / "db" / "configmap.yaml"
MAXIMO_LINEAS_DIFERENCIA = 50


# ===== Lectura del README =====
def extraer_archivos_desde_readme(ruta_documento: Path) -> list[str]:
    """Lee la tabla de orden de ejecución en el README y devuelve la lista de nombres de archivo."""
    contenido_texto = ruta_documento.read_text(encoding="utf-8")
    seccion_orden = False
    archivos_ordenados: list[str] = []

    patron_fila = re.compile(r"^\|\s*\d+\s*\|\s*`([^`]+)`\s*\|")

    for linea_actual in contenido_texto.splitlines():
        linea_limpia = linea_actual.strip()
        if "## Orden de ejecución en una base nueva" in linea_limpia:
            seccion_orden = True
            continue
        if seccion_orden and linea_limpia.startswith("## "):
            break
        if seccion_orden:
            coincidencia = patron_fila.match(linea_limpia)
            if coincidencia:
                nombre_archivo = coincidencia.group(1).strip()
                archivos_ordenados.append(nombre_archivo)

    if not archivos_ordenados:
        raise ValueError(
            f"No se encontraron archivos en la tabla de orden de {ruta_documento}"
        )

    return archivos_ordenados


# ===== Generación del YAML =====
def formatear_bloque_yaml(contenido_archivo: str, indentacion: str = "    ") -> str:
    """Formatea el contenido como un bloque literal de YAML (|)."""
    lineas_resultado: list[str] = []
    for linea_cruda in contenido_archivo.splitlines():
        if linea_cruda:
            lineas_resultado.append(indentacion + linea_cruda)
        else:
            lineas_resultado.append("")
    # Bloque literal YAML | debe terminar con un salto de línea
    return "\n".join(lineas_resultado) + "\n"


def generar_manifiesto_configmap(
    archivos_requeridos: list[str], ruta_directorio_db: Path
) -> str:
    """Genera el texto YAML completo para el ConfigMap db-init-scripts."""
    lineas_manifiesto: list[str] = [
        "# Código generado automáticamente por scripts/sync_db_init_configmap.py",
        "# NO EDITAR ESTE ARCHIVO MANUALMENTE.",
        "# Para actualizar, modifica los scripts en infrastructure/db/ y ejecuta:",
        "#   python3 scripts/sync_db_init_configmap.py",
        "apiVersion: v1",
        "kind: ConfigMap",
        "metadata:",
        "  name: db-init-scripts",
        "  labels:",
        "    app: db",
        "data:",
    ]

    for nombre_archivo in archivos_requeridos:
        ruta_archivo = ruta_directorio_db / nombre_archivo
        if not ruta_archivo.is_file():
            raise FileNotFoundError(
                f"El archivo {nombre_archivo} requerido en README no existe en {ruta_directorio_db}"
            )

        # Asegurar finales de línea LF estándar
        contenido_limpio = ruta_archivo.read_text(encoding="utf-8").replace(
            "\r\n", "\n"
        )
        lineas_manifiesto.append(f"  {nombre_archivo}: |")
        bloque_indentado = formatear_bloque_yaml(contenido_limpio, indentacion="    ")
        lineas_manifiesto.append(bloque_indentado.rstrip("\n"))

    return "\n".join(lineas_manifiesto) + "\n"


# ===== Punto de entrada y CLI =====
def main() -> int:
    """Punto de entrada principal para sincronizar o verificar el ConfigMap db-init-scripts."""
    analizador = argparse.ArgumentParser(description=__doc__)
    analizador.add_argument(
        "--check",
        action="store_true",
        help="Verifica si el archivo generado coincide con el existente sin modificarlo.",
    )
    analizador.add_argument(
        "--output",
        type=Path,
        default=RUTA_CONFIGMAP_DEFAULT,
        help="Ruta de destino del ConfigMap YAML (por defecto: infrastructure/k8s/db/configmap.yaml).",
    )
    analizador.add_argument(
        "--quiet",
        action="store_true",
        help="Modo silencioso, no imprime información de progreso.",
    )

    argumentos = analizador.parse_args()

    archivos_ordenados = extraer_archivos_desde_readme(RUTA_README)
    if not argumentos.quiet:
        print(f"Detectados {len(archivos_ordenados)} scripts en {RUTA_README.name}:")
        for indice, nombre_script in enumerate(archivos_ordenados, start=1):
            print(f"  {indice:2d}. {nombre_script}")

    manifiesto_generado = generar_manifiesto_configmap(archivos_ordenados, RUTA_DB_DIR)

    ruta_destino: Path = argumentos.output

    if argumentos.check:
        if not ruta_destino.exists():
            print(f"Error: {ruta_destino} no existe.", file=sys.stderr)
            return 1

        contenido_existente = ruta_destino.read_text(encoding="utf-8")
        if contenido_existente != manifiesto_generado:
            print(
                f"Error: {ruta_destino} está desactualizado respecto a {RUTA_DB_DIR}.",
                file=sys.stderr,
            )
            diferencias = list(
                difflib.unified_diff(
                    contenido_existente.splitlines(),
                    manifiesto_generado.splitlines(),
                    fromfile=str(ruta_destino),
                    tofile="generado",
                    lineterm="",
                )
            )
            for linea_diff in diferencias[:MAXIMO_LINEAS_DIFERENCIA]:
                print(linea_diff, file=sys.stderr)
            if len(diferencias) > MAXIMO_LINEAS_DIFERENCIA:
                lineas_restantes = len(diferencias) - MAXIMO_LINEAS_DIFERENCIA
                print(
                    f"... y {lineas_restantes} líneas de diferencia adicionales.",
                    file=sys.stderr,
                )
            return 1

        if not argumentos.quiet:
            print(f"OK: {ruta_destino} está 100% sincronizado con {RUTA_DB_DIR}.")
        return 0

    ruta_destino.parent.mkdir(parents=True, exist_ok=True)
    ruta_destino.write_text(manifiesto_generado, encoding="utf-8")
    if not argumentos.quiet:
        print(f"Manifiesto ConfigMap escrito exitosamente en {ruta_destino}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
