#!/usr/bin/env python3
"""
Generador de mapas de arquitectura para archivos de contexto IA.

Parsea los archivos fuente del proyecto (Python, Go, TypeScript) y genera
la sección dinámica de los GEMINI.md / CLAUDE.md de cada microservicio.

Usa SOLO stdlib de Python — cero dependencias externas.

Uso:
    python3 scripts/gen_service_map.py              # Regenera todos los servicios
    python3 scripts/gen_service_map.py backend/api   # Regenera solo un servicio
"""

import ast
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

SERVICES = {
    "backend/catalogo": {
        "lang": "go",
        "name": "Catálogo y precios (Go + Gin)",
        "extensions": [".go"],
        "exclude": ["tmp"],
    },
    "backend/api": {
        "lang": "go",
        "name": "API de Identidad y Scraping (Go + Gin)",
        "extensions": [".go"],
        "exclude": ["tmp", "docs/docs.go"],
    },
    "backend/ia_conversacional": {
        "lang": "python",
        "name": "IA Conversacional (FastAPI)",
        "extensions": [".py"],
        "exclude": ["__pycache__"],
    },
    "backend/motor_rutas": {
        "lang": "python",
        "name": "Motor de Rutas (FastAPI + OR-Tools)",
        "extensions": [".py"],
        "exclude": ["__pycache__"],
    },
    "backend/scraper": {
        "lang": "python",
        "name": "Scraper (Scrapy)",
        "extensions": [".py"],
        "exclude": ["__pycache__"],
    },
    "frontend/web/src": {
        "lang": "typescript",
        "name": "Frontend (React + TypeScript)",
        "extensions": [".ts", ".tsx"],
        "exclude": ["node_modules"],
        "context_dir": "frontend/web",
    },
}

MARKER = "<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->"


# ─── Parsers ────────────────────────────────────────────────────────────────────


def parse_python_file(filepath: Path) -> dict:
    """Extrae clases, funciones e imports de un archivo Python usando ast."""
    try:
        source = filepath.read_text(encoding="utf-8", errors="replace")
        tree = ast.parse(source, filename=str(filepath))
    except (SyntaxError, ValueError):
        return {"classes": [], "functions": [], "imports": []}

    classes = []
    functions = []
    imports = []

    for node in ast.walk(tree):
        if isinstance(node, ast.ClassDef):
            bases = [_name(b) for b in node.bases]
            methods = []
            for item in node.body:
                if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
                    sig = _func_signature(item)
                    methods.append(sig)
            classes.append({
                "name": node.name,
                "bases": bases,
                "methods": methods,
            })
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            # Solo funciones de nivel módulo
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                parent = _get_parent(tree, node)
                if isinstance(parent, ast.Module):
                    functions.append(_func_signature(node))

        elif isinstance(node, ast.ImportFrom):
            if node.module and not node.module.startswith(("__",)):
                imports.append(node.module)
        elif isinstance(node, ast.Import):
            for alias in node.names:
                imports.append(alias.name)

    return {"classes": classes, "functions": functions, "imports": imports}


def parse_go_file(filepath: Path) -> dict:
    """Extrae structs, funciones e imports de un archivo Go usando regex."""
    try:
        source = filepath.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return {"structs": [], "functions": [], "imports": [], "package": ""}

    # Package
    pkg_match = re.search(r"^package\s+(\w+)", source, re.MULTILINE)
    package = pkg_match.group(1) if pkg_match else ""

    # Structs
    structs = re.findall(r"type\s+(\w+)\s+struct\s*\{", source)

    # Functions (incluyendo métodos con receiver)
    func_pattern = re.compile(
        r"func\s+(?:\((\w+)\s+\*?(\w+)\)\s+)?(\w+)\s*\(([^)]*)\)\s*([^{]*)\{",
        re.MULTILINE,
    )
    functions = []
    for m in func_pattern.finditer(source):
        receiver_var, receiver_type, name, params, returns = m.groups()
        returns = returns.strip().rstrip("{").strip()
        if receiver_type:
            sig = f"({receiver_type}).{name}({params.strip()})"
        else:
            sig = f"{name}({params.strip()})"
        if returns:
            sig += f" {returns}"
        functions.append(sig)

    # Imports
    import_block = re.search(r'import\s*\((.*?)\)', source, re.DOTALL)
    imports = []
    if import_block:
        for line in import_block.group(1).split("\n"):
            line = line.strip().strip('"').strip()
            if line and not line.startswith("//") and not line.startswith("_"):
                # Limpiar alias
                parts = line.split('"')
                if len(parts) >= 2:
                    imports.append(parts[1] if parts[1] else parts[0])
                elif line:
                    imports.append(line.strip('"'))

    return {
        "structs": structs,
        "functions": functions,
        "imports": imports,
        "package": package,
    }


def parse_typescript_file(filepath: Path) -> dict:
    """Extrae componentes, tipos e imports de un archivo TS/TSX usando regex."""
    try:
        source = filepath.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return {"components": [], "types": [], "imports": []}

    # Exports (funciones y componentes)
    export_patterns = [
        re.compile(r"export\s+(?:default\s+)?function\s+(\w+)"),
        re.compile(r"export\s+(?:const|let)\s+(\w+)"),
        re.compile(r"export\s+default\s+(\w+)\s*;"),
    ]
    components = set()
    for p in export_patterns:
        for m in p.finditer(source):
            name = m.group(1)
            if name not in ("default",):
                components.add(name)

    # Types e interfaces
    type_patterns = [
        re.compile(r"(?:export\s+)?(?:type|interface)\s+(\w+)"),
    ]
    types = set()
    for p in type_patterns:
        for m in p.finditer(source):
            types.add(m.group(1))

    # Imports (solo rutas relativas o alias @)
    import_pattern = re.compile(r"""from\s+['"]([^'"]+)['"]""")
    imports = []
    for m in import_pattern.finditer(source):
        path = m.group(1)
        if path.startswith(".") or path.startswith("@/"):
            imports.append(path)

    return {
        "components": sorted(components),
        "types": sorted(types),
        "imports": imports,
    }


# ─── Helpers AST ────────────────────────────────────────────────────────────────


def _name(node) -> str:
    if isinstance(node, ast.Name):
        return node.id
    elif isinstance(node, ast.Attribute):
        return f"{_name(node.value)}.{node.attr}"
    return "?"


def _func_signature(node) -> str:
    prefix = "async " if isinstance(node, ast.AsyncFunctionDef) else ""
    args = []
    for arg in node.args.args:
        name = arg.arg
        if name == "self":
            continue
        annotation = ""
        if arg.annotation:
            annotation = f": {_name(arg.annotation)}"
        args.append(f"{name}{annotation}")
    ret = ""
    if node.returns:
        ret = f" → {_name(node.returns)}"
    return f"{prefix}{node.name}({', '.join(args)}){ret}"


def _get_parent(tree, target_node):
    """Encuentra el padre de un nodo en el AST."""
    for node in ast.walk(tree):
        for child in ast.iter_child_nodes(node):
            if child is target_node:
                return node
    return tree


# ─── Generación de mapa ────────────────────────────────────────────────────────


def collect_files(base_path: Path, extensions: list, exclude: list) -> list:
    """Recolecta archivos fuente, excluyendo directorios/archivos específicos."""
    files = []
    for ext in extensions:
        for f in sorted(base_path.rglob(f"*{ext}")):
            rel = str(f.relative_to(base_path))
            skip = False
            for ex in exclude:
                if ex in rel:
                    skip = True
                    break
            if not skip and f.is_file():
                files.append(f)
    return files


def generate_python_map(service_path: Path, config: dict) -> str:
    """Genera mapa de arquitectura para un servicio Python."""
    files = collect_files(service_path, config["extensions"], config["exclude"])
    if not files:
        return "_Sin archivos Python detectados._\n"

    lines = [f"\n## Mapa auto-generado: {config['name']}\n"]
    lines.append(f"**{len(files)} archivos .py** detectados\n")

    for f in files:
        rel = f.relative_to(service_path)
        data = parse_python_file(f)

        # Saltar __init__.py vacíos
        if not data["classes"] and not data["functions"] and rel.name == "__init__.py":
            continue

        lines.append(f"\n### `{rel}`\n")

        if data["imports"]:
            local_imports = [i for i in data["imports"] if i.startswith("app")]
            if local_imports:
                lines.append(f"- Imports internos: {', '.join(sorted(set(local_imports)))}")

        for cls in data["classes"]:
            bases_str = f"({', '.join(cls['bases'])})" if cls["bases"] else ""
            lines.append(f"- **class {cls['name']}{bases_str}**")
            for method in cls["methods"]:
                lines.append(f"  - `{method}`")

        for func in data["functions"]:
            lines.append(f"- `{func}`")

    return "\n".join(lines) + "\n"


def generate_go_map(service_path: Path, config: dict) -> str:
    """Genera mapa de arquitectura para un servicio Go."""
    files = collect_files(service_path, config["extensions"], config["exclude"])
    if not files:
        return "_Sin archivos Go detectados._\n"

    lines = [f"\n## Mapa auto-generado: {config['name']}\n"]
    lines.append(f"**{len(files)} archivos .go** detectados\n")

    for f in files:
        rel = f.relative_to(service_path)
        data = parse_go_file(f)

        lines.append(f"\n### `{rel}` (package {data['package']})\n")

        if data["structs"]:
            lines.append(f"- Structs: {', '.join(data['structs'])}")

        for func in data["functions"]:
            lines.append(f"- `{func}`")

    return "\n".join(lines) + "\n"


def generate_typescript_map(service_path: Path, config: dict) -> str:
    """Genera mapa de arquitectura para un servicio TypeScript/React."""
    files = collect_files(service_path, config["extensions"], config["exclude"])
    if not files:
        return "_Sin archivos TypeScript detectados._\n"

    # Filtrar archivos kebab-case (son obsoletos v0)
    active_files = []
    for f in files:
        name = f.stem
        # Si existe la versión PascalCase, saltar la kebab-case
        if "-" in name:
            pascal_name = "".join(w.capitalize() for w in name.split("-"))
            pascal_file = f.parent / f"{pascal_name}{f.suffix}"
            if pascal_file.exists():
                continue
        active_files.append(f)

    lines = [f"\n## Mapa auto-generado: {config['name']}\n"]
    lines.append(f"**{len(active_files)} archivos activos** (excluidos kebab-case obsoletos)\n")

    for f in active_files:
        rel = f.relative_to(service_path)
        data = parse_typescript_file(f)

        if not data["components"] and not data["types"]:
            continue

        lines.append(f"\n### `{rel}`\n")

        if data["components"]:
            lines.append(f"- Exports: {', '.join(data['components'])}")

        if data["types"]:
            lines.append(f"- Tipos: {', '.join(data['types'])}")

        if data["imports"]:
            local = [i for i in data["imports"] if i.startswith("@/") or i.startswith(".")]
            if local:
                lines.append(f"- Imports locales: {', '.join(sorted(set(local)))}")

    return "\n".join(lines) + "\n"


# ─── Update de archivos ────────────────────────────────────────────────────────


def update_context_file(filepath: Path, dynamic_content: str):
    """Actualiza la sección dinámica de un archivo GEMINI.md/CLAUDE.md."""
    if not filepath.exists():
        filepath.write_text(MARKER + "\n" + dynamic_content, encoding="utf-8")
        return

    current = filepath.read_text(encoding="utf-8")
    if MARKER in current:
        static_part = current.split(MARKER)[0]
        new_content = static_part + MARKER + "\n" + dynamic_content
    else:
        new_content = current.rstrip() + "\n\n" + MARKER + "\n" + dynamic_content

    filepath.write_text(new_content, encoding="utf-8")


def update_root_files(all_maps: dict):
    """Actualiza la sección dinámica de los archivos raíz."""
    summary_lines = ["\n## Resumen de arquitectura (auto-generado)\n"]
    for svc_key, content in all_maps.items():
        summary_lines.append(content)

    dynamic = "\n".join(summary_lines) + "\n"

    for filename in ["GEMINI.md", "CLAUDE.md"]:
        update_context_file(ROOT / filename, dynamic)

    # Copilot: reescribir completo (no tiene sección estática separada, es copia del GEMINI.md)
    gemini_root = ROOT / "GEMINI.md"
    copilot_file = ROOT / ".github" / "copilot-instructions.md"
    copilot_file.parent.mkdir(parents=True, exist_ok=True)
    copilot_file.write_text(
        gemini_root.read_text(encoding="utf-8"), encoding="utf-8"
    )


# ─── Main ──────────────────────────────────────────────────────────────────────


def main():
    target = sys.argv[1] if len(sys.argv) > 1 else None
    all_maps = {}

    for svc_key, config in SERVICES.items():
        if target and not svc_key.startswith(target):
            continue

        service_path = ROOT / svc_key
        if not service_path.exists():
            continue

        # Generar mapa según lenguaje
        if config["lang"] == "python":
            content = generate_python_map(service_path, config)
        elif config["lang"] == "go":
            content = generate_go_map(service_path, config)
        elif config["lang"] == "typescript":
            content = generate_typescript_map(service_path, config)
        else:
            continue

        all_maps[svc_key] = content

        # Actualizar archivos del servicio
        context_dir = ROOT / config.get("context_dir", svc_key)
        for filename in ["GEMINI.md", "CLAUDE.md"]:
            update_context_file(context_dir / filename, content)

        print(f"  ✅ {svc_key}")

    # Actualizar archivos raíz
    if not target:
        update_root_files(all_maps)
        print(f"  ✅ Archivos raíz (GEMINI.md, CLAUDE.md, copilot-instructions.md)")

    print(f"\n🧠 Mapas de contexto IA actualizados.")


if __name__ == "__main__":
    main()
