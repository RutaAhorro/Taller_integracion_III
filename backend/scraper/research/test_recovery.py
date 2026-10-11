import io
import json
import os
import tempfile
import threading
import unittest
from pathlib import Path
from unittest.mock import patch
from urllib.error import HTTPError, URLError
from uuid import uuid4

from scraper_core.api_client import ScraperAPIClient, ScraperAPIError
from scraper_core.outbox import Outbox
from scraper_core.pipelines import ScraperCorePipeline
from scraper_core.worker import CrawlExhausted, process_job
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
            def finalize_work(self, work_id, status): calls.append((work_id,status))
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


if __name__ == "__main__":
    unittest.main()
