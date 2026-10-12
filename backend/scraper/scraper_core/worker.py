from __future__ import annotations

import fcntl
import json
import logging
import os
import socket
import sqlite3
import threading
import time
from contextlib import contextmanager
from uuid import UUID, uuid4

from redis import Redis
from redis.exceptions import RedisError, ResponseError

from scraper_core.api_client import ScraperAPIClient, ScraperAPIError
from scraper_core.outbox import Outbox
from scraper_core.runtime import ScrapyCommandExecutor
from scraper_core.storage import SpoolCollector, StoragePressure, cleanup_spool, ensure_capacity

QUEUE_NAME = "scraper:jobs:stream"
GROUP = "scraper-workers"
JOB_INDEX = "scraper:jobs:index"
JOB_CLOSED_PREFIX = "scraper:jobs:closed:"
DEAD_LETTER = "scraper:jobs:dead"
ALLOWED_SPIDERS = {"jumbo_rsc", "santa_isabel_rsc", "cugat_rsc", "acuenta_rsc", "lider_rsc"}
logger = logging.getLogger(__name__)


class CrawlExhausted(RuntimeError):
    pass


@contextmanager
def job_environment(work_id):
    previous = os.environ.get("SCRAPER_TRABAJO_ID")
    os.environ["SCRAPER_TRABAJO_ID"] = work_id
    try:
        yield
    finally:
        if previous is None:
            os.environ.pop("SCRAPER_TRABAJO_ID", None)
        else:
            os.environ["SCRAPER_TRABAJO_ID"] = previous


def process_job(job: dict, api_client: ScraperAPIClient, cancelled=None) -> dict:
    work_id = str(UUID(job["trabajo_id"]))
    spider = job.get("spider", "jumbo_rsc")
    if spider not in ALLOWED_SPIDERS or (job.get("product_id") and not job.get("product_url")):
        raise ValueError("Parámetros de spider inválidos")
    state_dir = os.environ.setdefault("SCRAPER_STATE_DIR", "/var/lib/scraper")
    # Consultar antes de abrir SQLite permite retirar spools terminales aun
    # cuando el volumen ya esté lleno.
    status = api_client.get_work(work_id)
    if status["estado"] in ("completado", "fallido"):
        return status
    spool = Outbox(work_id, state_dir)
    try:
        # El volumen soporta un worker; flock impide un segundo proceso
        # sobre el mismo spool durante una recuperación de lease.
        with (spool.directory / "execution.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            if cancelled and cancelled.is_set():
                raise RuntimeError("Se perdió la lease del trabajo")
            status = api_client.get_work(work_id)
            if status["estado"] in ("completado", "fallido"):
                return status
            spool.seal()
            spool.replay(api_client, cancelled)
            result = {}
            if not spool.progress("crawl_done"):
                attempts = spool.progress("crawl_attempts")
                if attempts >= 3:
                    raise CrawlExhausted("El crawl agotó 3 intentos; el spool se conserva")
                spool.set_progress("crawl_attempts", attempts + 1)
                with job_environment(work_id):
                    result = ScrapyCommandExecutor(spider_name=spider)(job, cancelled=cancelled)
                if not spool.progress("crawl_done"):
                    raise RuntimeError("Scrapy no confirmó un cierre exitoso")
            spool.seal()
            spool.replay(api_client, cancelled)
            if cancelled and cancelled.is_set():
                raise RuntimeError("Se perdió la lease del trabajo")
            # La API usa su ledger; no sobrescribirlo con len(items).
            result["estado"] = api_client.finalize_work(work_id, "completado")["estado"]
            return result
    finally:
        spool.close()


# Comprobar propietario y renovar/confirmar en el mismo comando evita que
# un consumidor antiguo haga ACK después de que otro reclame el mensaje.
RENEW = """
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[2], ARGV[2], 1)
if #p == 0 or p[1][2] ~= ARGV[3] then return 0 end
redis.call('XCLAIM', KEYS[1], ARGV[1], ARGV[3], 0, ARGV[2], 'JUSTID')
return 1
"""
ACK = """
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[2], ARGV[2], 1)
if #p == 0 or p[1][2] ~= ARGV[3] then return 0 end
if ARGV[5] ~= '' then
  redis.call('XADD', KEYS[3], '*', 'payload', ARGV[4], 'reason', ARGV[5], 'source_id', ARGV[2])
end
redis.call('XACK', KEYS[1], ARGV[1], ARGV[2])
redis.call('XDEL', KEYS[1], ARGV[2])
if redis.call('HGET', KEYS[2], ARGV[6]) == ARGV[2] then redis.call('HDEL', KEYS[2], ARGV[6]) end
if ARGV[6] ~= '' then redis.call('SET', KEYS[4], '1', 'EX', 86400) end
return 1
"""


class ReliableQueue:
    def __init__(self, client, consumer=None, lease_ms=60000):
        if lease_ms < 10000:
            raise ValueError("SCRAPER_LEASE_MS debe ser al menos 10000")
        self.client = client
        self.consumer = consumer or f"{socket.gethostname()}-{uuid4()}"
        self.lease_ms = lease_ms
        self.cursor = "0-0"
        self.renew_script = client.register_script(RENEW)
        self.ack_script = client.register_script(ACK)

    def ensure_group(self):
        try:
            self.client.xgroup_create(QUEUE_NAME, GROUP, id="0", mkstream=True)
        except ResponseError as error:
            if "BUSYGROUP" not in str(error):
                raise

    def receive(self):
        claimed = self.client.xautoclaim(QUEUE_NAME, GROUP, self.consumer, self.lease_ms,
                                        start_id=self.cursor, count=1)
        self.cursor = claimed[0]
        if claimed[1]:
            return claimed[1][0]
        # Un único worker con volumen RWO: esperar una lease reciente antes
        # de tomar trabajo nuevo tras un reinicio. No se pierde su prioridad.
        if self.client.xpending(QUEUE_NAME, GROUP)["pending"]:
            return None
        messages = self.client.xreadgroup(GROUP, self.consumer, {QUEUE_NAME: ">"}, count=1, block=1000)
        return messages[0][1][0] if messages else None

    def renew(self, message_id):
        return bool(self.renew_script(keys=[QUEUE_NAME], args=[GROUP, message_id, self.consumer]))

    def ack(self, message_id, raw_job, work_id="", reason=""):
        return bool(self.ack_script(keys=[QUEUE_NAME, JOB_INDEX, DEAD_LETTER, JOB_CLOSED_PREFIX + work_id],
                                    args=[GROUP, message_id, self.consumer, raw_job, reason, work_id]))

    def has_message(self, work_id):
        message_id = self.client.hget(JOB_INDEX, work_id)
        return bool(message_id and self.client.xrange(QUEUE_NAME, message_id, message_id, count=1))

    @contextmanager
    def heartbeat(self, message_id):
        stopped, lost = threading.Event(), threading.Event()

        def run():
            while not stopped.wait(self.lease_ms / 3000):
                try:
                    if not self.renew(message_id):
                        lost.set()
                        return
                except RedisError:
                    lost.set()
                    return
        thread = threading.Thread(target=run, daemon=True)
        if not self.renew(message_id):
            lost.set()
        thread.start()
        try:
            yield lost
        finally:
            stopped.set()
            thread.join(timeout=5)


def handle_message(queue, message, api_client):
    message_id, fields = message
    raw = fields.get("payload", "")
    try:
        job = json.loads(raw)
        work_id = str(UUID(job["trabajo_id"]))
        if job.get("spider", "jumbo_rsc") not in ALLOWED_SPIDERS:
            raise ValueError("Spider inválido")
        if job.get("product_id") and not job.get("product_url"):
            raise ValueError("product_id requiere product_url")
    except (ValueError, KeyError, TypeError):
        queue.ack(message_id, raw, reason="invalid_payload")
        return

    with queue.heartbeat(message_id) as cancelled:
        try:
            result = process_job(job, api_client, cancelled)
            if not cancelled.is_set():
                if queue.ack(message_id, raw, work_id) and result.get("estado") == "completado":
                    cleanup_spool(os.getenv("SCRAPER_STATE_DIR", "/var/lib/scraper"), work_id)
        except (CrawlExhausted, ScraperAPIError) as error:
            permanent = isinstance(error, CrawlExhausted) or error.permanent
            if permanent and not cancelled.is_set():
                # Sin confirmación de la API se mantiene el mensaje pendiente.
                if not (isinstance(error, ScraperAPIError) and error.status == 404):
                    api_client.finalize_work(work_id, "fallido", error_message="Trabajo enviado a revisión; consultar spool y entregas")
                queue.ack(message_id, raw, work_id, reason="attempts_exhausted" if isinstance(error, CrawlExhausted) else "delivery_rejected")
            else:
                logger.warning("worker_delivery_pending trabajo_id=%s", work_id)
        except (RuntimeError, OSError, sqlite3.Error):
            logger.exception("worker_execution_pending trabajo_id=%s", work_id)


def run_worker():
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
    client = Redis.from_url(os.getenv("REDIS_URL", "redis://localhost:6379/0"), decode_responses=True,
                            socket_timeout=5, socket_connect_timeout=5, socket_keepalive=True)
    api_client = ScraperAPIClient(os.getenv("SCRAPER_API_URL", "http://localhost:8082/api/v1/scraper/productos"),
                                  timeout=float(os.getenv("SCRAPER_API_TIMEOUT", "30")))
    queue = ReliableQueue(client, lease_ms=int(os.getenv("SCRAPER_LEASE_MS", "60000")))
    state_dir = os.environ.setdefault("SCRAPER_STATE_DIR", "/var/lib/scraper")
    collector = SpoolCollector(state_dir)
    gc_api = ScraperAPIClient(api_client.endpoint, timeout=2)
    next_collection = 0
    logger.info("worker_started stream=%s consumer=%s", QUEUE_NAME, queue.consumer)
    while True:
        try:
            queue.ensure_group()
            if time.monotonic() >= next_collection:
                collector.collect(gc_api, queue)
                next_collection = time.monotonic() + 10
            # La limpieza puede liberar un PVC lleno sin recrear el pod.
            # Si sólo quedan trabajos activos, pausar sin borrar datos.
            ensure_capacity(state_dir)
            message = queue.receive()
            if message:
                handle_message(queue, message, api_client)
            else:
                time.sleep(.2)
        except StoragePressure:
            logger.error("worker_storage_pressure; sin espacio de reserva, se conservan trabajos activos")
            time.sleep(5)
        except (RedisError, ScraperAPIError, OSError, sqlite3.Error):
            logger.warning("worker_dependency_unavailable; se conserva el mensaje pendiente")
            time.sleep(5)


if __name__ == "__main__":
    run_worker()
