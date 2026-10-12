import json
import unittest
import unittest.mock
from urllib.request import Request

from scraper_core.api_client import ScraperAPIClient
from scraper_core.pipelines import ScraperCorePipeline


class FakeResponse:
    def __init__(self, payload):
        self.payload = payload

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        return False

    def read(self):
        return json.dumps(self.payload).encode("utf-8")


class APIPipelineTest(unittest.TestCase):
    def test_client_posts_ingestion_contract(self):
        requests = []

        def opener(request, timeout):
            requests.append((request, timeout))
            return FakeResponse({"insertados": 1})

        client = ScraperAPIClient(
            "http://api.test/api/v1/scraper/productos",
            timeout=12,
            opener=opener,
        )
        result = client.ingest_batch(
            [{"sku": "sku-1", "producto": "Leche", "precio_normal": 1000}],
            supermarket="Jumbo",
        )

        request, timeout = requests[0]
        self.assertEqual(result, {"insertados": 1})
        self.assertEqual(request.full_url, "http://api.test/api/v1/scraper/productos")
        self.assertEqual(timeout, 12)
        self.assertEqual(
            json.loads(request.data),
            {
                "supermercado": "Jumbo",
                "productos": [
                    {"sku": "sku-1", "producto": "Leche", "precio_normal": 1000}
                ],
            },
        )

    def test_client_sends_service_token_on_every_call(self):
        requests = []

        def opener(request, timeout):
            requests.append(request)
            if request.get_method() == "PUT":
                return FakeResponse({"id": "w1", "estado": "completado"})
            if request.get_method() == "GET":
                return FakeResponse({"id": "w1", "estado": "en_progreso"})
            return FakeResponse({"insertados": 1})

        client = ScraperAPIClient("http://api.test/api/v1/scraper/productos", opener=opener, token="s" * 32)
        client.ingest_batch([{"sku": "sku-1"}], supermarket="Jumbo")
        client.finalize_work("w1", "completado")
        client.get_work("w1")

        self.assertEqual(len(requests), 3)
        for request in requests:
            self.assertEqual(request.get_header("Authorization"), "Bearer " + "s" * 32)

    def test_client_reads_token_from_environment(self):
        with unittest.mock.patch.dict("os.environ", {"SCRAPING_SERVICE_TOKEN": "e" * 32}):
            client = ScraperAPIClient("http://api.test/api/v1/scraper/productos")
        self.assertEqual(client._headers()["Authorization"], "Bearer " + "e" * 32)

    def test_client_without_token_sends_no_authorization(self):
        with unittest.mock.patch.dict("os.environ", {}, clear=True):
            client = ScraperAPIClient("http://api.test/api/v1/scraper/productos")
        self.assertNotIn("Authorization", client._headers())

    def test_pipeline_flushes_batches(self):
        sent = []

        class FakeClient:
            def ingest_batch(self, products, supermarket=None, work_id=None):
                sent.append((products, supermarket, work_id))

        pipeline = ScraperCorePipeline(FakeClient(), batch_size=2)
        pipeline.process_item(
            {
                "sku": "sku-1",
                "producto": "Leche",
                "precio_normal": 1000,
                "supermercado": "Jumbo",
            },
            spider=None,
        )
        self.assertEqual(sent, [])

        pipeline.process_item(
            {
                "sku": "sku-2",
                "producto": "Pan",
                "precio_normal": 800,
                "supermercado": "Jumbo",
            },
            spider=None,
        )
        self.assertEqual(
            sent,
            [
                (
                    [
                        {"sku": "sku-1", "producto": "Leche", "precio_normal": 1000},
                        {"sku": "sku-2", "producto": "Pan", "precio_normal": 800},
                    ],
                    "Jumbo",
                    None,
                )
            ],
        )
        self.assertEqual(pipeline.batch, [])


if __name__ == "__main__":
    unittest.main()
