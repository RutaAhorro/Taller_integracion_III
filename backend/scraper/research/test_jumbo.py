"""Punto de entrada histórico para probar el spider de Jumbo.

La implementación vive en ``scraper_core/spiders/jumbo`` para que
``scrapy crawl jumbo_rsc`` y esta prueba no puedan divergir.
"""

import json
import sys
import unittest
from contextlib import redirect_stderr, redirect_stdout
from datetime import datetime, timedelta, timezone
from io import StringIO
from pathlib import Path
from unittest.mock import patch

import scrapy
from scrapy.http import Request, TextResponse

from scraper_core.freshness import (
    build_refresh_decision,
    RefreshQueue,
    RefreshScheduler,
    RefreshWorker,
    should_refresh_catalog,
    should_refresh_product,
    should_skip_refresh,
)
from scraper_core.output import ScraperResultPublisher
from scraper_core.runtime import ScrapyCommandExecutor, main as scraper_runtime_main
import scraper_core.settings as settings
from scraper_core.spiders.jumbo import JumboRscSpider


class JumboExtractionTest(unittest.TestCase):
    def test_extract_names_from_json_ld(self):
        response = TextResponse(
            url="https://www.jumbo.cl/frutas-y-verduras/verduras",
            body=(
                b'<script type="application/ld+json">'
                b'{"@graph":[{"item":{"@type":"Product",'
                b'"name":"Tomate Larga Vida","image":["https://img.test/tomate.jpg"],'
                    b'"offers":{"price":"$1.290"}}}]}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        self.assertEqual(
            JumboRscSpider._extract_products(response),
            [
                {
                    "producto": "Tomate Larga Vida",
                    "precio": 1290.0,
                    "precio_normal": 1290.0,
                    "precio_oferta": 1290.0,
                    "ean_gtin": None,
                    "sku": None,
                    "marca": None,
                    "formato_crudo": None,
                    "mecanica_promocion": None,
                    "en_stock": None,
                    "url_producto": None,
                    "imagen": "https://img.test/tomate.jpg",
                }
            ],
        )

    def test_category_is_taken_from_url(self):
        response = TextResponse(
            url="https://www.jumbo.cl/frutas-y-verduras/verduras",
            body=b"",
            encoding="utf-8",
        )

        self.assertEqual(response.url.rstrip("/").split("/")[-1], "verduras")

    def test_extracts_comparison_fields(self):
        response = TextResponse(
            url="https://www.jumbo.cl/frutas-y-verduras/verduras",
            body=(
                b'<script type="application/ld+json">'
                b'{"@type":"Product","name":"Lechuga", "gtin13":"7800000000001",'
                b'"sku":"SKU-1","brand":{"name":"Marca Test"},'
                b'"description":"1 unidad", "url":"https://jumbo.cl/p/lechuga",'
                b'"offers":{"price":"990","availability":"https://schema.org/InStock",'
                b'"offerDescription":"2x1"}}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        product = JumboRscSpider._extract_products(response)[0]

        self.assertEqual(product["ean_gtin"], "7800000000001")
        self.assertEqual(product["sku"], "SKU-1")
        self.assertEqual(product["marca"], "Marca Test")
        self.assertEqual(product["formato_crudo"], "1 unidad")
        self.assertEqual(product["mecanica_promocion"], "2x1")
        self.assertTrue(product["en_stock"])
        self.assertEqual(product["url_producto"], "https://jumbo.cl/p/lechuga")

    def test_category_requests_detail_for_missing_ean_when_enabled(self):
        spider = JumboRscSpider(enrich_ean=True)
        category_url = "https://www.jumbo.cl/frutas-y-verduras/verduras"
        response = TextResponse(
            url=category_url,
            request=Request(
                category_url,
                meta={"category_url": category_url, "page": 1},
            ),
            body=(
                b'<script type="application/ld+json">'
                b'{"@type":"Product","name":"Cafe",'
                b'"url":"https://www.jumbo.cl/cafe/p",'
                b'"offers":{"price":"990"}}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        results = list(spider.parse(response))

        detail_request = next(
            result
            for result in results
            if isinstance(result, Request)
            and result.callback == spider.parse_ean_detail
        )
        self.assertEqual(detail_request.url, "https://www.jumbo.cl/cafe/p")
        self.assertEqual(detail_request.cb_kwargs["category"], "verduras")

    def test_detail_parse_fills_ean_and_preserves_catalog_price(self):
        spider = JumboRscSpider(enrich_ean=True)
        product_url = "https://www.jumbo.cl/cafe/p"
        request = Request(
            product_url,
            cb_kwargs={
                "catalog_product": {
                    "producto": "Cafe",
                    "precio": 990.0,
                    "precio_normal": 990.0,
                    "precio_oferta": 990.0,
                    "ean_gtin": None,
                    "sku": None,
                    "marca": None,
                    "formato_crudo": None,
                    "mecanica_promocion": None,
                    "en_stock": None,
                    "url_producto": product_url,
                    "imagen": None,
                },
                "category": "despensa",
                "category_page_url": "https://www.jumbo.cl/despensa",
            },
        )
        response = TextResponse(
            url=product_url,
            request=request,
            body=(
                b'<script type="application/ld+json">'
                b'{"@type":"Product","name":"Cafe",'
                b'"url":"https://www.jumbo.cl/cafe/p",'
                b'"gtin13":"7800000000001","sku":"sku-detail",'
                b'"offers":{"price":"1200"}}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        item = next(spider.parse_ean_detail(response, **request.cb_kwargs))

        self.assertEqual(item["ean_gtin"], "7800000000001")
        self.assertEqual(item["sku"], "sku-detail")
        self.assertEqual(item["precio"], 990.0)
        self.assertEqual(item["categoria"], "despensa")
        self.assertEqual(item["supermercado"], "Jumbo")

    def test_page_url_preserves_category(self):
        url = JumboRscSpider._page_url(
            "https://www.jumbo.cl/frutas-y-verduras/verduras", 2
        )

        self.assertEqual(
            url, "https://www.jumbo.cl/frutas-y-verduras/verduras?page=2"
        )

    def test_catalog_pagination_stops_at_page_100(self):
        spider = JumboRscSpider()
        spider.max_pages = 100
        response = TextResponse(
            url="https://www.jumbo.cl/frutas-y-verduras?page=99",
            request=Request(
                "https://www.jumbo.cl/frutas-y-verduras?page=99",
                meta={"category_url": "https://www.jumbo.cl/frutas-y-verduras", "page": 99},
            ),
            body=(
                b'<script type="application/ld+json">'
                b'{"@type":"Product","name":"Producto prueba",'
                b'"offers":{"price":"100"}}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        results = list(spider.parse(response))

        self.assertTrue(any(isinstance(result, Request) and "page=100" in result.url for result in results))

        response.request.meta["page"] = 100
        response = response.replace(url="https://www.jumbo.cl/frutas-y-verduras?page=100")
        results = list(spider.parse(response))
        self.assertFalse(any(isinstance(result, Request) for result in results))

    def test_product_url_starts_one_request_without_category_pagination(self):
        product_url = "https://www.jumbo.cl/p/lechuga"
        spider = JumboRscSpider(product_url=product_url)
        spider.start_urls = ["https://www.jumbo.cl/frutas-y-verduras/verduras"]

        requests = list(spider.start_requests())

        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0].url, product_url)
        self.assertEqual(requests[0].callback, spider.parse_product)

    def test_product_url_rejects_untrusted_domains(self):
        with self.assertRaises(ValueError):
            JumboRscSpider._validate_product_url("https://example.com/product/1")

    def test_parse_product_returns_one_item_without_pagination(self):
        product_url = "https://www.jumbo.cl/p/lechuga"
        response = TextResponse(
            url=product_url,
            request=Request(product_url, meta={"product_url": product_url}),
            body=(
                b'<script type="application/ld+json">'
                b'{"@type":"Product","name":"Lechuga","url":"https://www.jumbo.cl/p/lechuga",'
                b'"offers":{"price":"990"}}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        items = list(JumboRscSpider().parse_product(response))

        self.assertEqual(len(items), 1)
        self.assertEqual(items[0]["producto"], "Lechuga")
        self.assertEqual(items[0]["precio"], 990.0)
        self.assertEqual(items[0]["url_producto"], product_url)
        self.assertEqual(items[0]["categoria"], "producto")

    def test_parse_product_selects_requested_product_from_json_ld_graph(self):
        product_url = "https://www.jumbo.cl/p/lechuga"
        response = TextResponse(
            url=product_url,
            request=Request(product_url, meta={"product_url": product_url}),
            body=(
                b'<script type="application/ld+json">'
                b'{"@graph":['
                b'{"@type":"Product","name":"Tomate","url":"/p/tomate",'
                b'"offers":{"price":"1200"}},'
                b'{"@type":"Product","name":"Lechuga","url":"/p/lechuga",'
                b'"offers":{"price":"990"}}]}'
                b'</script>'
            ),
            encoding="utf-8",
        )

        items = list(JumboRscSpider().parse_product(response))

        self.assertEqual(len(items), 1)
        self.assertEqual(items[0]["producto"], "Lechuga")
        self.assertEqual(items[0]["precio"], 990.0)

    def test_catalog_refresh_is_needed_when_never_updated(self):
        self.assertTrue(should_refresh_catalog(None))

    def test_catalog_refresh_handles_naive_datetime_without_type_error(self):
        naive = datetime.now() - timedelta(hours=2)
        self.assertTrue(should_refresh_catalog(naive, interval_seconds=3600))

    def test_catalog_refresh_is_not_needed_while_fresh(self):
        recent = datetime.now(timezone.utc) - timedelta(minutes=30)
        self.assertFalse(should_refresh_catalog(recent, interval_seconds=3600))

    def test_product_refresh_is_needed_after_ttl(self):
        stale = datetime.now(timezone.utc) - timedelta(minutes=45)
        self.assertTrue(should_refresh_product(stale, ttl_seconds=1800))

    def test_product_refresh_is_not_needed_while_fresh(self):
        recent = datetime.now(timezone.utc) - timedelta(minutes=10)
        self.assertFalse(should_refresh_product(recent, ttl_seconds=1800))

    def test_refresh_decision_marks_stale_product(self):
        stale = datetime.now(timezone.utc) - timedelta(minutes=45)
        decision = build_refresh_decision("sku-123", stale, ttl_seconds=1800)

        self.assertEqual(decision["product_id"], "sku-123")
        self.assertTrue(decision["needs_refresh"])
        self.assertEqual(decision["reason"], "ttl_exceeded")

    def test_duplicate_refresh_request_is_skipped(self):
        in_flight = {"sku-123"}
        self.assertTrue(should_skip_refresh("sku-123", in_flight))
        self.assertFalse(should_skip_refresh("sku-456", in_flight))

    def test_scheduler_marks_catalog_as_stale_after_interval(self):
        scheduler = RefreshScheduler(catalog_interval_seconds=3600, product_ttl_seconds=1800)
        stale = datetime.now(timezone.utc) - timedelta(hours=2)

        self.assertTrue(scheduler.catalog_needs_refresh(stale))
        self.assertFalse(
            scheduler.product_needs_refresh(
                "sku-1",
                datetime.now(timezone.utc) - timedelta(minutes=10),
            )
        )

    def test_queue_skips_duplicate_in_flight_refresh(self):
        queue = RefreshQueue(product_ttl_seconds=1800)

        queue.mark_started("sku-123")
        decision = queue.enqueue("sku-123")

        self.assertFalse(decision["needs_refresh"])
        self.assertEqual(decision["reason"], "in_flight")

    def test_queue_schedules_stale_product_refresh(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        stale = datetime.now(timezone.utc) - timedelta(minutes=45)

        decision = queue.enqueue("sku-777", last_updated_at=stale)

        self.assertTrue(decision["needs_refresh"])
        self.assertEqual(decision["reason"], "ttl_exceeded")

    def test_queue_exposes_enqueued_product_for_processing(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        stale = datetime.now(timezone.utc) - timedelta(minutes=45)

        queue.enqueue(
            "sku-777",
            last_updated_at=stale,
            product_url="https://www.jumbo.cl/p/sku-777",
        )
        job = queue.pop_next()

        self.assertEqual(
            job,
            {
                "product_id": "sku-777",
                "product_url": "https://www.jumbo.cl/p/sku-777",
            },
        )
        queue.mark_finished(job["product_id"])
        self.assertIsNone(queue.pop_next())

    def test_worker_processes_next_product_and_releases_in_flight_lock(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        queue.enqueue("sku-777", product_url="https://www.jumbo.cl/p/sku-777")
        processed = []
        worker = RefreshWorker(queue, lambda job: processed.append(job))

        result = worker.process_next()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(processed[0]["product_id"], "sku-777")
        self.assertNotIn("sku-777", queue.in_flight_refreshes)

    def test_worker_reports_failure_and_releases_in_flight_lock(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        queue.enqueue("sku-888")

        def fail(_job):
            raise RuntimeError("fallo de prueba")

        worker = RefreshWorker(queue, fail)
        result = worker.process_next()

        self.assertEqual(result["status"], "failed")
        self.assertIn("fallo de prueba", result["error"])
        self.assertNotIn("sku-888", queue.in_flight_refreshes)

    def test_worker_propagates_structured_failure_from_executor(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        queue.enqueue("sku-999", product_url="https://www.jumbo.cl/p/sku-999")

        def fail(job):
            return {
                "status": "failed",
                "product_id": job["product_id"],
                "error": "API no disponible",
                "result": None,
            }

        worker = RefreshWorker(queue, fail)
        result = worker.process_next()

        self.assertEqual(result["status"], "failed")
        self.assertEqual(result["product_id"], "sku-999")
        self.assertIn("API no disponible", result["error"])
        self.assertNotIn("sku-999", queue.in_flight_refreshes)

    def test_worker_processes_all_pending_products(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        queue.enqueue("sku-1")
        queue.enqueue("sku-2")
        processed = []
        worker = RefreshWorker(queue, lambda job: processed.append(job["product_id"]))

        results = worker.process_all()

        self.assertEqual(processed, ["sku-1", "sku-2"])
        self.assertEqual([result["status"] for result in results], ["completed", "completed"])
        self.assertEqual(queue.in_flight_refreshes, set())

    def test_result_publisher_sends_item_to_injected_sender(self):
        sent_items = []
        publisher = ScraperResultPublisher(lambda item: sent_items.append(item))

        result = publisher.publish({"sku": "sku-1", "precio": 1290.0})

        self.assertEqual(result["status"], "sent")
        self.assertEqual(sent_items, [{"sku": "sku-1", "precio": 1290.0}])

    def test_result_publisher_reports_api_failure(self):
        def fail(_item):
            raise RuntimeError("API no disponible")

        publisher = ScraperResultPublisher(fail)
        result = publisher.publish({"sku": "sku-1"})

        self.assertEqual(result["status"], "failed")
        self.assertIn("API no disponible", result["error"])

    def test_refresh_flow_consumes_queue_and_publishes_result(self):
        queue = RefreshQueue(product_ttl_seconds=1800)
        queue.enqueue("sku-3", product_url="https://www.jumbo.cl/p/sku-3")
        published = []
        publisher = ScraperResultPublisher(lambda item: published.append(item))
        worker = RefreshWorker(
            queue,
            lambda job: publisher.publish(
                {"sku": job["product_id"], "url": job["product_url"]}
            ),
        )

        result = worker.process_next()

        self.assertEqual(result["status"], "completed")
        self.assertEqual(published, [{"sku": "sku-3", "url": "https://www.jumbo.cl/p/sku-3"}])

    def test_scrapy_executor_returns_structured_catalog_items(self):
        commands = []

        def run_command(command, **kwargs):
            commands.append((command, kwargs))
            output_path = Path(command[command.index("-O") + 1])
            output_path.write_text(
                json.dumps({"sku": "sku-1", "precio": 1290.0}) + "\n",
                encoding="utf-8",
            )
            return type("Completed", (), {"returncode": 0, "stdout": "items", "stderr": ""})()

        executor = ScrapyCommandExecutor(command_runner=run_command, collect_items=True)
        result = executor({})

        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["items"], [{"sku": "sku-1", "precio": 1290.0}])
        self.assertIn("scrapy", commands[0][0])
        self.assertIn("crawl", commands[0][0])
        self.assertIn("-O", commands[0][0])
        self.assertTrue(commands[0][0][commands[0][0].index("-O") + 1].endswith(".jsonl"))

    def test_scrapy_executor_worker_mode_does_not_load_catalog_in_memory(self):
        commands = []

        def run_command(command, **kwargs):
            commands.append(command)
            return type("Completed", (), {"returncode": 0, "stdout": "", "stderr": ""})()

        result = ScrapyCommandExecutor(command_runner=run_command)({})

        self.assertNotIn("-O", commands[0])
        self.assertNotIn("items", result)

    def test_runtime_cli_emits_extracted_items_as_jsonlines(self):
        item = {"sku": "sku-42", "precio": 990.0}
        stdout = StringIO()
        stderr = StringIO()

        with patch("scraper_core.runtime.ScrapyCommandExecutor") as executor_class:
            executor_class.return_value.return_value = {
                "status": "completed",
                "stdout": "scrapy stdout\n",
                "stderr": "scrapy stderr\n",
                "items": [item],
            }
            with patch.object(
                sys,
                "argv",
                ["scraper-runtime", "--product-url", "https://www.jumbo.cl/p/42"],
            ):
                with redirect_stdout(stdout), redirect_stderr(stderr):
                    scraper_runtime_main()

        executor_class.return_value.assert_called_once_with(
            {"product_url": "https://www.jumbo.cl/p/42"}
        )
        executor_class.assert_called_once_with(collect_items=True)
        self.assertEqual(stdout.getvalue(), json.dumps(item, ensure_ascii=False) + "\n")
        self.assertEqual(stderr.getvalue(), "scrapy stderr\nscrapy stdout\n")

    def test_scrapy_executor_targets_product_url_for_queued_job(self):
        commands = []

        def run_command(command, **kwargs):
            commands.append((command, kwargs))
            return type("Completed", (), {"returncode": 0, "stdout": "item", "stderr": ""})()

        executor = ScrapyCommandExecutor(command_runner=run_command)
        result = executor(
            {
                "product_id": "sku-42",
                "product_url": "https://www.jumbo.cl/frutas-y-verduras/verduras?product=sku-42",
            }
        )

        self.assertEqual(result["status"], "completed")
        self.assertIn("-a", commands[0][0])
        self.assertIn("product_url=https://www.jumbo.cl/frutas-y-verduras/verduras?product=sku-42", commands[0][0])
        self.assertIn("JOBDIR=", " ".join(commands[0][0]))

    def test_scrapy_executor_rejects_product_id_without_url(self):
        executor = ScrapyCommandExecutor(command_runner=lambda *_args, **_kwargs: None)

        with self.assertRaisesRegex(ValueError, "requiere product_url"):
            executor({"product_id": "sku-42"})

    def test_scheduler_decides_catalog_and_product_update_flow(self):
        scheduler = RefreshScheduler(catalog_interval_seconds=3600, product_ttl_seconds=1800)
        last_catalog = datetime.now(timezone.utc) - timedelta(hours=3)
        last_product = datetime.now(timezone.utc) - timedelta(minutes=45)

        self.assertTrue(scheduler.catalog_needs_refresh(last_catalog))
        self.assertTrue(scheduler.product_needs_refresh("sku-99", last_product))
        self.assertEqual(
            scheduler.build_catalog_and_product_plan(last_catalog, "sku-99", last_product),
            {
                "catalog_refresh": True,
                "product_refresh": True,
                "product_reason": "ttl_exceeded",
                "manual_trigger": False,
            },
        )

    def test_scheduler_accepts_future_api_configuration(self):
        scheduler = RefreshScheduler.from_config(
            {
                "enabled": True,
                "catalog_interval_seconds": 7200,
                "product_ttl_seconds": 900,
            }
        )

        self.assertTrue(scheduler.enabled)
        self.assertEqual(scheduler.catalog_interval_seconds, 7200)
        self.assertEqual(scheduler.product_ttl_seconds, 900)

    def test_scheduler_can_force_catalog_refresh_from_manual_trigger(self):
        scheduler = RefreshScheduler.from_config({"enabled": True})
        recent = datetime.now(timezone.utc)

        plan = scheduler.build_catalog_and_product_plan(
            recent,
            "sku-1",
            recent,
            force_catalog_refresh=True,
        )

        self.assertTrue(plan["catalog_refresh"])
        self.assertTrue(plan["manual_trigger"])

    def test_scrapy_settings_use_ethic_rate_limit(self):
        self.assertTrue(settings.AUTOTHROTTLE_ENABLED)
        self.assertGreaterEqual(settings.DOWNLOAD_DELAY, 2)
        self.assertEqual(settings.CONCURRENT_REQUESTS, 1)
        self.assertEqual(settings.CONCURRENT_REQUESTS_PER_DOMAIN, 1)
        self.assertIn(429, settings.RETRY_HTTP_CODES)

    def test_scrapy_retry_policy_handles_transient_errors(self):
        self.assertTrue(settings.RETRY_ENABLED)
        self.assertGreaterEqual(settings.RETRY_TIMES, 3)
        self.assertGreaterEqual(settings.RETRY_BACKOFF_FACTOR, 2)
        self.assertIn(429, settings.RETRY_HTTP_CODES)
        self.assertIn(503, settings.RETRY_HTTP_CODES)

    def test_404_page_stops_pagination_cleanly(self):
        spider = JumboRscSpider()
        request = Request(
            "https://www.jumbo.cl/frutas-y-verduras/verduras?page=2",
            meta={"category_url": "https://www.jumbo.cl/frutas-y-verduras/verduras", "page": 2},
        )
        response = TextResponse(
            url=request.url,
            status=404,
            body=b"",
            encoding="utf-8",
            request=request,
        )
        self.assertEqual(list(spider.parse(response)), [])
        self.assertEqual(list(spider.parse(response)), [])


if __name__ == "__main__":
    unittest.main()