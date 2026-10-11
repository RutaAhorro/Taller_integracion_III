"""Retención del spool: nunca borrar trabajos activos ni mensajes sin ACK."""
from __future__ import annotations

import fcntl
import logging
import os
import shutil
import time
from datetime import datetime, timezone
from pathlib import Path
from uuid import UUID

logger = logging.getLogger(__name__)


class StoragePressure(OSError):
    pass


def reserve_bytes():
    value = int(os.getenv("SCRAPER_STATE_MIN_FREE_BYTES", str(64 * 1024 * 1024)))
    if value < 1024 * 1024:
        raise ValueError("SCRAPER_STATE_MIN_FREE_BYTES debe ser al menos 1 MiB")
    return value


def ensure_capacity(directory, extra_bytes=0):
    free = shutil.disk_usage(directory).free
    if free < reserve_bytes() + extra_bytes:
        raise StoragePressure("Spool sin espacio de reserva; se conserva para recuperación")


def cleanup_spool(state_dir, work_id):
    directory = Path(state_dir) / str(UUID(work_id))
    if directory.is_symlink() or not directory.is_dir():
        return False
    try:
        with (directory / "execution.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            shutil.rmtree(directory)
    except (BlockingIOError, FileNotFoundError):
        return False
    logger.info("worker_spool_removed trabajo_id=%s", work_id)
    return True


class SpoolCollector:
    """Recorre por turnos; repara también un crash ocurrido entre ACK y borrado."""

    def __init__(self, state_dir):
        self.directory = Path(state_dir)
        self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.entries = iter(sorted(self.directory.iterdir()))

    def collect(self, api_client, queue, limit=20):
        removed = 0
        deadline = time.monotonic() + 5
        retention = int(os.getenv("SCRAPER_FAILED_RETENTION_SECONDS", "86400"))
        if retention < 0:
            raise ValueError("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
        pressure = shutil.disk_usage(self.directory).free < reserve_bytes()
        for _ in range(limit):
            if time.monotonic() >= deadline:
                break
            try:
                directory = next(self.entries)
            except StopIteration:
                self.entries = iter(sorted(self.directory.iterdir()))
                break
            if directory.is_symlink() or not directory.is_dir():
                continue
            try:
                work_id = str(UUID(directory.name))
            except ValueError:
                continue
            # La API debe confirmar estado terminal y Redis ausencia de mensaje.
            # Ante una dependencia caída, el caller conserva todos los archivos.
            status = api_client.get_work(work_id)
            if status["estado"] not in ("completado", "fallido") or queue.has_message(work_id):
                continue
            if status["estado"] == "fallido" and not pressure:
                finished = status.get("finalizado_el")
                if not finished:
                    continue
                age = (datetime.now(timezone.utc) - datetime.fromisoformat(finished.replace("Z", "+00:00"))).total_seconds()
                if age < retention:
                    continue
            if cleanup_spool(self.directory, work_id):
                removed += 1
                if status["estado"] == "fallido":
                    logger.warning("worker_failed_spool_removed trabajo_id=%s storage_pressure=%s", work_id, pressure)
                pressure = shutil.disk_usage(self.directory).free < reserve_bytes()
        return removed
