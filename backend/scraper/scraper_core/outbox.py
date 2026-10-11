"""Spool local durable: guardar cada item antes de enviarlo a la API.

SQLite confirma items, claves de lote y progreso en el volumen del worker.
Un reinicio vuelve a enviar exactamente el documento y la clave almacenados.
"""
from __future__ import annotations

import hashlib
import json
import os
import sqlite3
from pathlib import Path
from uuid import UUID, uuid4

from scraper_core.storage import ensure_capacity


class Outbox:
    def __init__(self, work_id: str, state_dir: str | Path):
        self.work_id = str(UUID(work_id))
        self.directory = Path(state_dir) / self.work_id
        self.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        ensure_capacity(self.directory)
        self.db = sqlite3.connect(self.directory / "outbox.sqlite", timeout=30)
        try:
            self._initialize()
        except BaseException:
            self.db.close()
            raise

    def _initialize(self):
        self.db.execute("PRAGMA journal_mode=WAL")
        self.db.execute("PRAGMA synchronous=FULL")
        self.db.executescript("""
            CREATE TABLE IF NOT EXISTS items (
                id INTEGER PRIMARY KEY, fingerprint TEXT UNIQUE NOT NULL,
                market TEXT, product TEXT NOT NULL, batch_id TEXT);
            CREATE TABLE IF NOT EXISTS batches (
                id TEXT PRIMARY KEY, payload TEXT NOT NULL, delivered INTEGER NOT NULL DEFAULT 0);
            CREATE TABLE IF NOT EXISTS progress (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
        """)
        os.chmod(self.directory / "outbox.sqlite", 0o600)

    def close(self):
        self.db.close()

    def add(self, product: dict, market: str | None):
        data = json.dumps(product, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        ensure_capacity(self.directory, len(data.encode("utf-8")) * 4)
        fingerprint = hashlib.sha256(json.dumps([market, data]).encode()).hexdigest()
        with self.db:
            self.db.execute("INSERT OR IGNORE INTO items(fingerprint,market,product) VALUES (?,?,?)",
                            (fingerprint, market, data))

    def unbatched(self):
        return self.db.execute("SELECT count(*) FROM items WHERE batch_id IS NULL").fetchone()[0]

    def seal(self, batch_size=100):
        if not 1 <= batch_size <= 1000:
            raise ValueError("El tamaño de lote debe estar entre 1 y 1000")
        while True:
            with self.db:
                first = self.db.execute("SELECT market FROM items WHERE batch_id IS NULL ORDER BY id LIMIT 1").fetchone()
                if first is None:
                    return
                market = first[0]
                rows = self.db.execute("SELECT id,product FROM items WHERE batch_id IS NULL AND market IS ? ORDER BY id LIMIT ?",
                                       (market, batch_size)).fetchall()
                ensure_capacity(self.directory, sum(len(row[1].encode("utf-8")) for row in rows) * 2)
                payload = {"products": [json.loads(row[1]) for row in rows], "supermarket": market}
                batch_id = str(uuid4())
                self.db.execute("INSERT INTO batches(id,payload) VALUES (?,?)", (batch_id, json.dumps(payload, ensure_ascii=False)))
                self.db.executemany("UPDATE items SET batch_id=? WHERE id=?", [(batch_id, row[0]) for row in rows])

    def replay(self, client, cancelled=None):
        for batch_id, data in self.db.execute("SELECT id,payload FROM batches WHERE delivered=0 ORDER BY rowid").fetchall():
            if cancelled and cancelled.is_set():
                raise RuntimeError("Se perdió la lease del trabajo")
            payload = json.loads(data)
            client.ingest_batch(**payload, work_id=self.work_id, batch_id=batch_id)
            # Una respuesta perdida conserva esta fila y su clave original.
            with self.db:
                self.db.execute("UPDATE batches SET delivered=1 WHERE id=?", (batch_id,))

    def progress(self, key):
        row = self.db.execute("SELECT value FROM progress WHERE key=?", (key,)).fetchone()
        return row[0] if row else 0

    def set_progress(self, key, value):
        with self.db:
            self.db.execute("INSERT INTO progress(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", (key, value))
