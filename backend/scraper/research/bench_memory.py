#!/usr/bin/env python3
"""Benchmark de RAM del runtime de scraping.

Compara el comportamiento anterior (cargar el catálogo completo en una lista)
contra el actual (el worker no retiene ítems). Cada modo corre en un proceso
aparte para que el RSS pico sea independiente y no se contamine entre pruebas.

Uso:
    .venv/bin/python research/bench_memory.py --items 150000
    .venv/bin/python research/bench_memory.py --items 150000 --max-worker-mb 120

El límite `--max-worker-mb` sirve para CI: falla si el modo worker vuelve a
retener el catálogo en memoria.
"""
from __future__ import annotations

import argparse
import json
import resource
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

# Forma realista de un producto de supermercado (~400 bytes en JSON).
PRODUCT = {
    "sku": "1234567890123",
    "nombre": "Producto de prueba con nombre largo " * 2,
    "precio": 1990.0,
    "precio_oferta": 1490.0,
    "imagen": "https://jumbo.cl/img/1234567890.jpg",
    "categoria": "despensa-y-bebidas",
    "url_producto": "https://www.jumbo.cl/p/producto-1234567890",
    "ean": "1234567890123",
    "supermercado": "jumbo",
    "fecha_extraccion": "2026-10-20T12:00:00Z",
}


def rss_mb() -> float:
    """RSS pico del proceso actual, en MiB.

    Se lee VmHWM de /proc porque ru_maxrss puede heredar el pico del proceso
    padre al hacer fork y dar cifras infladas según cómo se lance el script.
    """
    try:
        with open("/proc/self/status", encoding="ascii") as status:
            for line in status:
                if line.startswith("VmHWM:"):
                    return int(line.split()[1]) / 1024
    except OSError:
        pass
    return resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024


def write_fixture(path: Path, items: int) -> int:
    line = json.dumps(PRODUCT, ensure_ascii=False)
    with path.open("w", encoding="utf-8") as output:
        for _ in range(items):
            output.write(line + "\n")
    return len(line.encode("utf-8")) + 1


def run_child(mode: str, items: int) -> dict:
    from scraper_core.runtime import ScrapyCommandExecutor

    class Completed:
        returncode = 0
        stdout = ""
        stderr = ""

    with tempfile.TemporaryDirectory(prefix="ram-bench-") as tmp:
        fixture = Path(tmp) / "items.jsonl"
        baseline = rss_mb()
        bytes_per_item = write_fixture(fixture, items)
        after_write = rss_mb()

        if mode == "old":
            # Comportamiento previo de runtime.py: materializar todo el catálogo.
            with fixture.open(encoding="utf-8") as output:
                loaded = [json.loads(line) for line in output if line.strip()]
            assert len(loaded) == items

        elif mode == "new":
            # Comportamiento actual del worker: no pide ni carga items.
            def runner(command, **kwargs):
                assert "-O" not in command, "el worker no debe pedir items.jsonl"
                return Completed()

            result = ScrapyCommandExecutor(command_runner=runner)({})
            assert "items" not in result

        elif mode == "cli":
            # La CLI sí colecta, pero sólo para un producto puntual.
            def runner(command, **kwargs):
                target = Path(command[command.index("-O") + 1])
                target.write_text(fixture.read_text(encoding="utf-8"), encoding="utf-8")
                return Completed()

            result = ScrapyCommandExecutor(command_runner=runner, collect_items=True)({})
            assert len(result["items"]) == items

        else:
            raise SystemExit(f"modo inválido: {mode}")

    return {
        "mode": mode,
        "items": items,
        "fixture_mb": round(bytes_per_item * items / 1024 / 1024, 1),
        "baseline_mb": round(baseline, 1),
        "after_write_mb": round(after_write, 1),
        "peak_mb": round(rss_mb(), 1),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--items", type=int, default=150_000)
    parser.add_argument("--max-worker-mb", type=float, default=None)
    parser.add_argument("--child", choices=("old", "new", "cli"), help=argparse.SUPPRESS)
    args = parser.parse_args()

    if args.items < 1:
        parser.error("--items debe ser mayor que cero")

    if args.child:
        print(json.dumps(run_child(args.child, args.items)))
        return 0

    results = []
    for mode in ("old", "new", "cli"):
        completed = subprocess.run(
            [sys.executable, __file__, "--child", mode, "--items", str(args.items)],
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            print(completed.stderr, file=sys.stderr)
            return completed.returncode
        results.append(json.loads(completed.stdout.strip().splitlines()[-1]))

    labels = {
        "old": "antes (lista completa)",
        "new": "ahora (worker)",
        "cli": "CLI (collect_items=True)",
    }
    print(f"Ítems: {args.items:,}  (~{results[0]['fixture_mb']} MB de JSONL)\n")
    print(f"{'Modo':<28}{'RSS pico':>12}{'Δ vs base':>14}")
    for result in results:
        delta = result["peak_mb"] - result["baseline_mb"]
        print(f"{labels[result['mode']]:<28}{result['peak_mb']:>9.1f} MB{delta:>12.1f} MB")

    worker = next(result for result in results if result["mode"] == "new")
    print(f"\nWorker: {worker['peak_mb']:.1f} MB RSS pico")
    if args.max_worker_mb is not None and worker["peak_mb"] > args.max_worker_mb:
        print(
            f"FALLA: worker {worker['peak_mb']:.1f} MB > límite {args.max_worker_mb} MB",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
