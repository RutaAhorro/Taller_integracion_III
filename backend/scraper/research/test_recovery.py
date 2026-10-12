import io
import json
import os
import sqlite3
import tempfile
import threading
import unittest
from contextlib import nullcontext
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from urllib.error import HTTPError, URLError
from uuid import uuid4

from scraper_core.api_client import ScraperAPIClient, ScraperAPIError
from scraper_core.outbox import Outbox
from scraper_core.pipelines import ScraperCorePipeline
from scraper_core.worker import CrawlExhausted, process_job, handle_message
from scraper_core.storage import SpoolCollector, StoragePressure
from scraper_core.migrate_queue import migrate_one


class Response:
    def __init__(self, data): self.data = data
    def __enter__(self): return self
    def __exit__(self, *args): return False
    def read(self): return json.dumps(self.data).encode()


class RecoveryTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.id = str(uuid4())
        self.env = patch.dict(os.environ, {"SCRAPER_STATE_DIR": self.temp.name, "SCRAPER_TRABAJO_ID": self.id})
        self.env.start()
        self.addCleanup(self.env.stop)

    def spool(self):
        spool = Outbox(self.id, self.temp.name)
        self.addCleanup(spool.close)
        return spool

    def test_item_survives_restart_before_batch_is_sealed(self):
        first = Outbox(self.id, self.temp.name)
        first.add({"sku": "SKU", "precio_normal": 1000}, "Jumbo")
        first.close()
        spool = self.spool()
        self.assertEqual(spool.unbatched(), 1)
        spool.seal()
        self.assertEqual(spool.db.execute("SELECT count(*) FROM batches").fetchone()[0], 1)

    def test_lost_response_replays_exact_key_and_payload(self):
        spool = self.spool()
        spool.add({"sku": "SKU"}, "Jumbo")
        spool.seal()
        calls = []
        class Client:
            def ingest_batch(self, **payload):
                calls.append(payload)
                if len(calls) == 1:
                    raise ScraperAPIError("respuesta perdida")
        with self.assertRaises(ScraperAPIError): spool.replay(Client())
        spool.replay(Client())
        spool.replay(Client())
        self.assertEqual(len(calls), 2)
        self.assertEqual(calls[0], calls[1])

    def test_repeated_crawl_items_are_not_batched_again(self):
        spool = self.spool()
        spool.add({"sku": "SKU", "precio_normal": 1000}, "Jumbo")
        spool.seal()
        spool.add({"precio_normal": 1000, "sku": "SKU"}, "Jumbo")
        self.assertEqual(spool.unbatched(), 0)
        spool.add({"sku": "SKU", "precio_normal": 900}, "Jumbo")
        self.assertEqual(spool.unbatched(), 1)

    def test_pipeline_keeps_batches_when_api_is_down(self):
        class Client:
            def ingest_batch(self, **kwargs): raise ScraperAPIError("offline")
        pipeline = ScraperCorePipeline(Client(), 1)
        pipeline.process_item({"sku": "SKU", "supermercado": "Jumbo"}, None)
        pipeline.close_spider(None)
        spool = self.spool()
        self.assertEqual(spool.db.execute("SELECT count(*) FROM batches WHERE delivered=0").fetchone()[0], 1)

    def test_cancelled_lease_does_not_send_pending_batch(self):
        spool = self.spool()
        spool.add({"sku": "SKU"}, "Jumbo")
        spool.seal()
        event = threading.Event(); event.set()
        with self.assertRaises(RuntimeError): spool.replay(object(), event)

    def test_client_preserves_key_and_marks_counter_pending_retryable(self):
        requests = []
        def opener(request, timeout):
            requests.append(request)
            raise HTTPError(request.full_url, 409, "pending", {}, io.BytesIO(b'{"codigo":"contador_pendiente"}'))
        client = ScraperAPIClient("http://api/api/v1/scraper/productos", opener=opener)
        with self.assertRaises(ScraperAPIError) as failure:
            client.ingest_batch([{"sku": "SKU"}], batch_id=self.id)
        self.assertFalse(failure.exception.permanent)
        self.assertEqual(requests[0].get_header("Idempotency-key"), self.id)

    def test_accepted_pending_is_not_considered_delivered(self):
        client = ScraperAPIClient("http://api/api/v1/scraper/productos", opener=lambda *args, **kwargs: Response({"codigo":"entrega_pendiente"}))
        with self.assertRaises(ScraperAPIError): client.ingest_batch([{"sku":"SKU"}], batch_id=self.id)

    def test_finished_job_does_not_rerun_crawl(self):
        class Client:
            def get_work(self, work_id): return {"estado":"completado"}
        with patch("scraper_core.worker.ScrapyCommandExecutor") as executor:
            process_job({"trabajo_id":self.id}, Client())
            executor.assert_not_called()

    def test_finished_crawl_retries_finalization_without_scraping(self):
        self.spool().set_progress("crawl_done", 1)
        calls = []
        class Client:
            def get_work(self, work_id): return {"estado":"en_progreso"}
            def finalize_work(self, work_id, status):
                calls.append((work_id,status))
                return {"estado": status}
        with patch("scraper_core.worker.ScrapyCommandExecutor") as executor:
            process_job({"trabajo_id":self.id}, Client())
            executor.assert_not_called()
        self.assertEqual(calls, [(self.id,"completado")])

    def test_three_crawl_attempts_require_review(self):
        self.spool().set_progress("crawl_attempts", 3)
        class Client:
            def get_work(self, work_id): return {"estado":"en_progreso"}
        with patch("scraper_core.worker.ScrapyCommandExecutor") as executor:
            with self.assertRaises(CrawlExhausted): process_job({"trabajo_id":self.id},Client())
            executor.assert_not_called()

    def test_legacy_queue_not_removed_before_sql_acceptance(self):
        raw=json.dumps({'trabajo_id':self.id,'spider':'jumbo_rsc'})
        class Redis:
            removed=False
            def lindex(self,*args): return raw
            def lrem(self,*args): self.removed=True
        def fail(request,timeout): raise URLError('offline')
        redis=Redis()
        api=ScraperAPIClient('http://api/api/v1/scraper/productos',opener=fail)
        self.assertEqual(migrate_one(redis,api),self.id)
        with self.assertRaises(URLError): migrate_one(redis,api,apply=True)
        self.assertFalse(redis.removed)

    def test_legacy_queue_removed_after_sql_acceptance(self):
        raw=json.dumps({'trabajo_id':self.id,'spider':'jumbo_rsc'})
        class Redis:
            removed=False
            def lindex(self,*args): return raw
            def lrem(self,*args): self.removed=True
        response=Response({'trabajo_id':self.id}); response.status=202; response.headers={'X-Scraper-Queue':'durable-stream'}
        api=ScraperAPIClient('http://api/api/v1/scraper/productos',opener=lambda *args,**kwargs:response)
        redis=Redis()
        migrate_one(redis,api,apply=True)
        self.assertTrue(redis.removed)

    def test_legacy_queue_migration_sends_service_token(self):
        raw=json.dumps({'trabajo_id':self.id,'spider':'jumbo_rsc'})
        class Redis:
            def lindex(self,*args): return raw
            def lrem(self,*args): pass
        sent=[]
        response=Response({'trabajo_id':self.id}); response.status=202; response.headers={'X-Scraper-Queue':'durable-stream'}
        def opener(request,timeout):
            sent.append(request)
            return response
        api=ScraperAPIClient('http://api/api/v1/scraper/productos',opener=opener,token='s'*32)
        migrate_one(Redis(),api,apply=True)
        self.assertEqual(sent[0].get_header('Authorization'),'Bearer '+'s'*32)

    def test_legacy_api_acceptance_does_not_prove_durable_queue(self):
        raw=json.dumps({'trabajo_id':self.id,'spider':'jumbo_rsc'})
        class Redis:
            removed=False
            def lindex(self,*args): return raw
            def lrem(self,*args): self.removed=True
        response=Response({'trabajo_id':self.id}); response.status=202; response.headers={}
        api=ScraperAPIClient('http://api/api/v1/scraper/productos',opener=lambda *args,**kwargs:response)
        redis=Redis()
        with self.assertRaises(RuntimeError): migrate_one(redis,api,apply=True)
        self.assertFalse(redis.removed)

    def test_finalize_failed_deliveries_are_permanent_not_pending(self):
        def opener(request, timeout):
            raise HTTPError(request.full_url, 409, "failed", {}, io.BytesIO(b'{"codigo":"entregas_fallidas"}'))
        client = ScraperAPIClient("http://api/api/v1/scraper/productos", opener=opener)
        with self.assertRaises(ScraperAPIError) as failure:
            client.finalize_work(self.id, "completado")
        self.assertTrue(failure.exception.permanent)

    def test_terminal_job_can_be_acknowledged_without_opening_full_sqlite(self):
        api = SimpleNamespace(get_work=lambda _: {"estado": "completado"})
        with patch("scraper_core.outbox.sqlite3.connect") as connect:
            process_job({"trabajo_id": self.id}, api)
        connect.assert_not_called()

    def test_completed_spool_deleted_only_after_successful_ack(self):
        first = Outbox(self.id, self.temp.name); first.add({"sku": "SKU"}, "Jumbo"); first.close()
        directory = Path(self.temp.name) / self.id
        queue = SimpleNamespace(heartbeat=lambda _: nullcontext(threading.Event()), ack=lambda *args: False)
        message = ("1-0", {"payload": json.dumps({"trabajo_id": self.id})})
        with patch("scraper_core.worker.process_job", return_value={"estado": "completado"}):
            handle_message(queue, message, object())
            self.assertTrue(directory.exists())
            queue.ack = lambda *args: True
            handle_message(queue, message, object())
            self.assertFalse(directory.exists())

    def test_gc_recovers_crash_after_ack_but_preserves_pending_job(self):
        completed = Outbox(self.id, self.temp.name); completed.close()
        active_id = str(uuid4()); active = Outbox(active_id, self.temp.name); active.close()
        api = SimpleNamespace(get_work=lambda work: {"estado": "completado" if work == self.id else "en_progreso"})
        queue = SimpleNamespace(has_message=lambda _: False)
        self.assertEqual(SpoolCollector(self.temp.name).collect(api, queue), 1)
        self.assertFalse((Path(self.temp.name) / self.id).exists())
        self.assertTrue((Path(self.temp.name) / active_id).exists())

    def test_gc_never_deletes_unacked_or_locked_spool(self):
        import fcntl
        first = Outbox(self.id, self.temp.name); first.close()
        directory = Path(self.temp.name) / self.id
        api = SimpleNamespace(get_work=lambda _: {"estado": "completado"})
        collector = SpoolCollector(self.temp.name)
        self.assertEqual(collector.collect(api, SimpleNamespace(has_message=lambda _: True)), 0)
        with (directory / "execution.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            self.assertEqual(SpoolCollector(self.temp.name).collect(api, SimpleNamespace(has_message=lambda _: False)), 0)
        self.assertTrue(directory.exists())

    def test_failed_spool_retention_and_pressure_cleanup(self):
        first = Outbox(self.id, self.temp.name); first.close()
        api = SimpleNamespace(get_work=lambda _: {"estado": "fallido", "finalizado_el": "2099-01-01T00:00:00Z"})
        queue = SimpleNamespace(has_message=lambda _: False)
        self.assertEqual(SpoolCollector(self.temp.name).collect(api, queue), 0)
        with patch("scraper_core.storage.shutil.disk_usage", return_value=SimpleNamespace(free=0)):
            self.assertEqual(SpoolCollector(self.temp.name).collect(api, queue), 1)
        self.assertFalse((Path(self.temp.name) / self.id).exists())

    def test_full_disk_does_not_delete_active_items_or_accept_new_ones(self):
        spool = self.spool(); spool.add({"sku": "before"}, "Jumbo")
        with patch("scraper_core.storage.shutil.disk_usage", return_value=SimpleNamespace(free=0)):
            with self.assertRaises(StoragePressure): spool.add({"sku": "after"}, "Jumbo")
            api = SimpleNamespace(get_work=lambda _: {"estado": "en_progreso"})
            self.assertEqual(SpoolCollector(self.temp.name).collect(api, SimpleNamespace(has_message=lambda _: False)), 0)
        self.assertEqual(spool.unbatched(), 1)

    def test_gc_dependency_failure_preserves_spool(self):
        first = Outbox(self.id, self.temp.name); first.close()
        def unavailable(_): raise ScraperAPIError("offline")
        with self.assertRaises(ScraperAPIError):
            SpoolCollector(self.temp.name).collect(SimpleNamespace(get_work=unavailable), object())
        self.assertTrue((Path(self.temp.name) / self.id).exists())

    def test_sqlite_full_rolls_back_without_ack_or_losing_saved_items(self):
        spool = self.spool(); spool.add({"sku": "saved"}, "Jumbo")
        pages = spool.db.execute("PRAGMA page_count").fetchone()[0]
        spool.db.execute(f"PRAGMA max_page_count={pages + 1}")
        with self.assertRaises(sqlite3.OperationalError) as failure:
            spool.add({"sku": "full", "producto": "x" * 100000}, "Jumbo")
        self.assertEqual(failure.exception.sqlite_errorcode, sqlite3.SQLITE_FULL)
        self.assertEqual(spool.unbatched(), 1)
        acks = []
        queue = SimpleNamespace(heartbeat=lambda _: nullcontext(threading.Event()), ack=lambda *a: acks.append(a))
        message = ("1-0", {"payload": json.dumps({"trabajo_id": self.id})})
        with patch("scraper_core.worker.process_job", side_effect=failure.exception), self.assertLogs("scraper_core.worker", "ERROR"):
            handle_message(queue, message, object())
        self.assertEqual(acks, [])
        spool.db.execute("PRAGMA max_page_count=100000")
        spool.add({"sku": "recovered"}, "Jumbo")
        self.assertEqual(spool.unbatched(), 2)

    def test_legacy_duplicates_remove_tail_and_keep_fifo_order(self):
        raw = json.dumps({"trabajo_id": self.id, "spider": "jumbo_rsc"})
        other = json.dumps({"trabajo_id": str(uuid4()), "spider": "lider_rsc"})
        class Redis:
            values = [raw, other, raw]
            def lindex(self, key, index): return self.values[index]
            def lrem(self, key, count, value):
                indexes = range(len(self.values)) if count > 0 else range(len(self.values)-1, -1, -1)
                for i in indexes:
                    if self.values[i] == value:
                        self.values.pop(i)
                        return 1
        response = Response({"trabajo_id": self.id}); response.status = 202
        response.headers = {"X-Scraper-Queue": "durable-stream"}
        api = ScraperAPIClient("http://api/api/v1/scraper/productos", opener=lambda *a, **kw: response)
        redis = Redis(); migrate_one(redis, api, apply=True)
        self.assertEqual(redis.values, [raw, other])


if __name__ == "__main__":
    unittest.main()
