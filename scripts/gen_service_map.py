#!/usr/bin/env python3
"""Genera índices breves para las IA, sin copiar firmas de todo el repositorio.

Uso: python3 scripts/gen_service_map.py [ruta-del-servicio]
CLAUDE.md contiene las instrucciones manuales canónicas; GEMINI.md es su copia.
La raíz enlaza los servicios en lugar de concatenar sus mapas.
Solo se escribe debajo de MARKER, salvo las copias GEMINI/Copilot.
"""

import argparse
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MARKER = "<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->"
EXCLUDED = {"node_modules", "vendor", ".venv", "venv", "__pycache__", "tmp", "dist", ".git"}
SERVICES = {
    "backend/gateway": ("Gateway", (".go",)),
    "backend/identidad": ("Identidad", (".go",)),
    "backend/catalogo": ("Catálogo", (".go",)),
    "backend/control_scraping": ("Control de scraping", (".go",)),
    "backend/scraper_go": ("Scraper Go", (".go",)),
    "backend/bot_discord": ("Bot Discord", (".go",)),
    "backend/ia_conversacional": ("IA conversacional", (".py",)),
    "backend/motor_rutas": ("Motor de rutas / Listas", (".py",)),
    "frontend/web/src": ("Frontend", (".ts", ".tsx")),
}


def context_dir(service: str) -> Path:
    path = ROOT / service
    return path.parent if service == "frontend/web/src" else path


def collect_files(path: Path, extensions: tuple[str, ...]) -> list[Path]:
    return sorted(
        file for file in path.rglob("*")
        if file.is_file() and file.suffix in extensions
        and not EXCLUDED.intersection(file.relative_to(path).parts)
    )


def generate_map(path: Path, extensions: tuple[str, ...]) -> str:
    files = collect_files(path, extensions)
    groups = Counter(
        file.relative_to(path).parts[0]
        if len(file.relative_to(path).parts) > 1 else "(raíz)"
        for file in files
    )
    lines = ["## Índice breve generado", "",
             f"{len(files)} archivos fuente, incluidos los de pruebas. Distribución:"]
    # Mantener un límite incluso si se incorporan muchos directorios nuevos.
    for name, count in sorted(groups.items())[:12]:
        lines.append(f"- `{name}`: {count} archivos.")
    if len(groups) > 12:
        lines.append("- Otros directorios: consultar el árbol del servicio.")
    lines.extend(["", "Buscar símbolos en el código; este índice no acredita funciones desplegadas."])
    return "\n".join(lines) + "\n"


def write_if_changed(path: Path, content: str) -> None:
    if not path.exists() or path.read_text(encoding="utf-8") != content:
        path.write_text(content, encoding="utf-8")


def update_context_pair(directory: Path, dynamic: str) -> None:
    canonical = directory / "CLAUDE.md"
    fallback = directory / "GEMINI.md"
    source = canonical if canonical.exists() else fallback
    current = source.read_text(encoding="utf-8") if source.exists() else ""
    manual = current.split(MARKER, 1)[0].rstrip()
    content = f"{manual}\n\n{MARKER}\n\n{dynamic}" if manual else f"{MARKER}\n\n{dynamic}"
    for filename in ("CLAUDE.md", "GEMINI.md"):
        write_if_changed(directory / filename, content)


def update_root_files() -> None:
    lines = ["## Contextos por servicio", ""]
    for service, (name, _) in SERVICES.items():
        if (ROOT / service).is_dir():
            relative = context_dir(service).relative_to(ROOT)
            lines.append(f"- {name}: `{relative}/CLAUDE.md`.")
    update_context_pair(ROOT, "\n".join(lines) + "\n")
    copilot = ROOT / ".github" / "copilot-instructions.md"
    copilot.parent.mkdir(parents=True, exist_ok=True)
    write_if_changed(copilot, (ROOT / "CLAUDE.md").read_text(encoding="utf-8"))


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("service", nargs="?", choices=SERVICES)
    args = parser.parse_args(argv)
    for service, (_, extensions) in SERVICES.items():
        if args.service and service != args.service:
            continue
        path = ROOT / service
        if path.is_dir():
            update_context_pair(context_dir(service), generate_map(path, extensions))
            print(f"Actualizado: {service}")
    if not args.service:
        update_root_files()
        print("Actualizados: contextos raíz y Copilot")


if __name__ == "__main__":
    main()
