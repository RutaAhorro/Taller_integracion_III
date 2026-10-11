from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


class ScrapyCommandExecutor:
    """Ejecuta el spider real sin persistir el resultado en un archivo."""

    def __init__(self, command_runner=subprocess.run, spider_name="jumbo_rsc"):
        self.command_runner = command_runner
        self.spider_name = spider_name

    def __call__(self, job, cancelled=None):
        job = job or {}
        if job.get("product_id") and not job.get("product_url"):
            raise ValueError("Un scraping puntual requiere product_url")

        jobdir = Path(tempfile.mkdtemp(prefix="scrapy-jobdir-"))
        output_path = jobdir / "items.jsonl"
        command = [
            "scrapy",
            "crawl",
            "-s",
            f"JOBDIR={jobdir}",
            "-O",
            str(output_path),
        ]

        if job.get("product_url"):
            command.extend(["-a", f"product_url={job['product_url']}"])
        if job.get("product_id"):
            command.extend(["-a", f"product_id={job['product_id']}"])
        command.append(self.spider_name)

        try:
            if cancelled is None or self.command_runner is not subprocess.run:
                completed = self.command_runner(command, capture_output=True, text=True, check=False)
            else:
                process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                try:
                    while True:
                        if cancelled.is_set():
                            raise RuntimeError("Se perdió la lease del trabajo")
                        try:
                            stdout, stderr = process.communicate(timeout=1)
                            completed = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
                            break
                        except subprocess.TimeoutExpired:
                            continue
                finally:
                    if process.poll() is None:
                        process.terminate()
                        try:
                            process.communicate(timeout=10)
                        except subprocess.TimeoutExpired:
                            process.kill()
                            process.communicate()
            if completed.returncode != 0:
                error = completed.stderr.strip() or "Scrapy terminó con error"
                raise RuntimeError(error)

            items = []
            if output_path.exists():
                with output_path.open(encoding="utf-8") as output_file:
                    items = [json.loads(line) for line in output_file if line.strip()]
        finally:
            shutil.rmtree(jobdir, ignore_errors=True)

        return {
            "status": "completed",
            "stdout": completed.stdout,
            "stderr": completed.stderr,
            "items": items,
        }


def main():
    parser = argparse.ArgumentParser(description="Ejecuta spiders de supermercados")
    parser.add_argument("--product-url")
    parser.add_argument("--product-id")
    args = parser.parse_args()
    if args.product_id and not args.product_url:
        parser.error("--product-id requiere --product-url")

    job = {
        key: value
        for key, value in vars(args).items()
        if value is not None
    }
    result = ScrapyCommandExecutor()(job)
    for output in (result.get("stderr"), result.get("stdout")):
        if output:
            print(output, end="", file=sys.stderr)
    for item in result["items"]:
        print(json.dumps(item, ensure_ascii=False))


if __name__ == "__main__":
    main()
