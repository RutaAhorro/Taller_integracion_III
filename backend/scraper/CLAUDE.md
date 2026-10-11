# Microservicio: Scraper de Supermercados (Python + Scrapy)

Modo: Standby worker (tail -f /dev/null) | Dockerfile: Dockerfile | Ejecución manual via docker exec

## Estructura de módulos

```
scraper_core/
├── __init__.py
├── items.py          → ScraperCoreItem(@dataclass) - modelo base (vacío)
├── middlewares.py     → ScraperCoreSpiderMiddleware, ScraperCoreDownloaderMiddleware
├── pipelines.py       → ScraperCorePipeline.process_item()
├── settings.py        → Config Scrapy (throttle, concurrency, delays)
└── spiders/
    ├── __init__.py
    └── jumbo.py       → JumboRscSpider(scrapy.Spider)
research/
├── jumbo_categories.txt  → URLs de categorías de Jumbo (14 URLs)
└── test_jumbo.py         → Tests unitarios del spider
```

## Spider principal: JumboRscSpider [scraper_core/spiders/jumbo.py]

- `name = "jumbo_rsc"`, `allowed_domains = ["jumbo.cl"]`
- `max_pages = 100` por categoría
- Extrae productos desde bloques `<script type="application/ld+json">`
- Campos extraídos: nombre, precio, imagen
- URLs de categorías desde: archivo txt + env JUMBO_CATEGORY_URLS + arg -a add_url
- Paginación automática con parámetro ?page=X
- `_walk_json(value)`: generador recursivo para JSON anidados

## Configuración clave [scraper_core/settings.py]

- CONCURRENT_REQUESTS = 1, CONCURRENT_REQUESTS_PER_DOMAIN = 1
- DOWNLOAD_DELAY = 1, AUTOTHROTTLE_ENABLED = True (2-60s)
- DOWNLOAD_MAXSIZE = 5MB (control de memoria)
- ROBOTSTXT_OBEY = True
- JOBDIR desde env SCRAPY_JOBDIR (persistencia de estado)

## Ejecución

```bash
docker compose exec web_scraper_alimentos scrapy crawl jumbo_rsc -O /tmp/jumbo.json
```

## Dependencias (requirements.txt)

- Scrapy >= 2.11.0
- psycopg2-binary >= 2.9.0 (para futura persistencia en PostgreSQL)
- redis >= 5.0.0 (para colas y deduplicación)

## Notas

- Pipeline y middlewares están en estado boilerplate (no conectan a DB/Redis aún)
- Contenedor limitado a 3GB RAM via cgroups en docker-compose.yml

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Scraper (Scrapy)

Estado vigente SUP-270: worker con Redis Streams, lease/heartbeat/ACK y dead-letter; spool SQLite durable en SCRAPER_STATE_DIR. No accede a PostgreSQL. Un worker con volumen persistente RWO; conservar las claves de lote. Ver docs/Guias/GUIA_SCRAPING_RECUPERACION.md.

**30 archivos .py** detectados


### `research/recovery_fixture.py`

- **class RecoveryFixture(Spider)**
  - `parse(response)`

### `research/smoke_recovery.py`

- `compose()`
- `sql(query)`
- `request(path, data, status, method, key)`
- `wait_for(predicate, timeout)`
- `job()`
- `batch(sku)`
- `path(work_id)`
- `captures(sku)`
- `recovered_crawl(job, cancelled)`
- `fixture_crawl(job, cancelled)`

### `research/test_acuenta.py`

- **class AcuentaExtractionTest(unittest.TestCase)**
  - `_response(body, url)`
  - `test_extracts_product_card()`
  - `test_merges_visible_offer_into_rsc_product()`
  - `test_page_url_removes_internal_rsc_token()`
  - `test_max_pages_argument_sets_per_category_limit()`
  - `test_extracts_multiunit_promotion()`
  - `test_does_not_cross_rsc_script_boundaries()`
  - `test_uses_product_fields_after_sku_not_previous_category_fields()`
  - `test_resolves_rsc_ean_reference()`
  - `test_resolves_rsc_special_price_reference_chain()`
  - `test_does_not_use_multiunit_rsc_price_as_unit_offer()`
  - `test_resolves_rsc_image_reference()`
  - `test_uses_exact_image_variant_published_for_sku()`
  - `test_encodes_image_proxy_brackets()`
  - `test_rejects_untrusted_urls()`

### `research/test_api_pipeline.py`

- **class FakeResponse**
  - `__init__(payload)`
  - `__enter__()`
  - `__exit__(exc_type, exc_value, traceback)`
  - `read()`
- **class APIPipelineTest(unittest.TestCase)**
  - `test_client_posts_ingestion_contract()`
  - `test_pipeline_flushes_batches()`
- **class FakeClient**
  - `ingest_batch(products, supermarket, work_id)`

### `research/test_cugat.py`

- **class CugatExtractionTest(unittest.TestCase)**
  - `_response(body, url)`
  - `test_extracts_woocommerce_card_fields()`
  - `test_page_url_and_category_preserve_cugat_path()`
  - `test_extracts_product_json_ld()`
  - `test_rejects_untrusted_urls()`

### `research/test_jumbo.py`

- **class JumboExtractionTest(unittest.TestCase)**
  - `test_extract_names_from_json_ld()`
  - `test_category_is_taken_from_url()`
  - `test_extracts_comparison_fields()`
  - `test_category_requests_detail_for_missing_ean_when_enabled()`
  - `test_detail_parse_fills_ean_and_preserves_catalog_price()`
  - `test_page_url_preserves_category()`
  - `test_catalog_pagination_stops_at_page_100()`
  - `test_product_url_starts_one_request_without_category_pagination()`
  - `test_product_url_rejects_untrusted_domains()`
  - `test_parse_product_returns_one_item_without_pagination()`
  - `test_parse_product_selects_requested_product_from_json_ld_graph()`
  - `test_catalog_refresh_is_needed_when_never_updated()`
  - `test_catalog_refresh_handles_naive_datetime_without_type_error()`
  - `test_catalog_refresh_is_not_needed_while_fresh()`
  - `test_product_refresh_is_needed_after_ttl()`
  - `test_product_refresh_is_not_needed_while_fresh()`
  - `test_refresh_decision_marks_stale_product()`
  - `test_duplicate_refresh_request_is_skipped()`
  - `test_scheduler_marks_catalog_as_stale_after_interval()`
  - `test_queue_skips_duplicate_in_flight_refresh()`
  - `test_queue_schedules_stale_product_refresh()`
  - `test_queue_exposes_enqueued_product_for_processing()`
  - `test_worker_processes_next_product_and_releases_in_flight_lock()`
  - `test_worker_reports_failure_and_releases_in_flight_lock()`
  - `test_worker_propagates_structured_failure_from_executor()`
  - `test_worker_processes_all_pending_products()`
  - `test_result_publisher_sends_item_to_injected_sender()`
  - `test_result_publisher_reports_api_failure()`
  - `test_refresh_flow_consumes_queue_and_publishes_result()`
  - `test_scrapy_executor_returns_structured_catalog_items()`
  - `test_runtime_cli_emits_extracted_items_as_jsonlines()`
  - `test_scrapy_executor_targets_product_url_for_queued_job()`
  - `test_scrapy_executor_rejects_product_id_without_url()`
  - `test_scheduler_decides_catalog_and_product_update_flow()`
  - `test_scheduler_accepts_future_api_configuration()`
  - `test_scheduler_can_force_catalog_refresh_from_manual_trigger()`
  - `test_scrapy_settings_use_ethic_rate_limit()`
  - `test_scrapy_retry_policy_handles_transient_errors()`
  - `test_404_page_stops_pagination_cleanly()`

### `research/test_lider.py`

- **class LiderExtractionTest(unittest.TestCase)**
  - `_response(body, url)`
  - `test_extracts_regular_and_offer_prices_from_product_cards()`
  - `test_prefers_all_hydrated_items_over_partial_product_cards()`
  - `test_accepts_super_lider_category_and_product_routes()`
  - `test_page_url_preserves_category_and_other_query_parameters()`
  - `test_page_limit_argument_is_configurable()`
  - `test_only_accepts_allowed_catalog_and_product_routes()`

### `research/test_normalization.py`

- **class ScraperNormalizationTest(unittest.TestCase)**
  - `test_normalizes_clp_price_formats()`
  - `test_rejects_invalid_prices()`
  - `test_normalizes_ean_without_losing_leading_zeroes()`
  - `test_normalizes_whitespace_in_text()`

### `research/test_recovery.py`

- **class Response**
  - `__init__(data)`
  - `__enter__()`
  - `__exit__()`
  - `read()`
- **class RecoveryTest(unittest.TestCase)**
  - `setUp()`
  - `spool()`
  - `test_item_survives_restart_before_batch_is_sealed()`
  - `test_lost_response_replays_exact_key_and_payload()`
  - `test_repeated_crawl_items_are_not_batched_again()`
  - `test_pipeline_keeps_batches_when_api_is_down()`
  - `test_cancelled_lease_does_not_send_pending_batch()`
  - `test_client_preserves_key_and_marks_counter_pending_retryable()`
  - `test_accepted_pending_is_not_considered_delivered()`
  - `test_finished_job_does_not_rerun_crawl()`
  - `test_finished_crawl_retries_finalization_without_scraping()`
  - `test_three_crawl_attempts_require_review()`
  - `test_legacy_queue_not_removed_before_sql_acceptance()`
  - `test_legacy_queue_removed_after_sql_acceptance()`
  - `test_legacy_api_acceptance_does_not_prove_durable_queue()`
- **class Client**
  - `ingest_batch()`
- **class Client**
  - `ingest_batch()`
- **class Client**
  - `get_work(work_id)`
- **class Client**
  - `get_work(work_id)`
  - `finalize_work(work_id, status)`
- **class Client**
  - `get_work(work_id)`
- **class Redis**
  - `lindex()`
  - `lrem()`
- **class Redis**
  - `lindex()`
  - `lrem()`
- **class Redis**
  - `lindex()`
  - `lrem()`

### `research/test_santa_isabel.py`

- **class SantaIsabelExtractionTest(unittest.TestCase)**
  - `_response(payload, page)`
  - `test_extracts_santa_isabel_render_data()`
  - `test_category_pagination_preserves_category()`
  - `test_spider_obeys_robots_txt()`
  - `test_only_uses_santa_isabel_seller()`

### `scraper_core/api_client.py`

- **class NoRedirect(HTTPRedirectHandler)**
  - `redirect_request(req, fp, code, msg, headers, newurl)`
- **class ScraperAPIError(RuntimeError)**
  - `__init__(message)`
  - `permanent()`
- **class ScraperAPIClient**
  - `__init__(endpoint: str, timeout: float, opener)`
  - `ingest_batch(products: ?, supermarket: ?, branch_id: ?, branch_code: ?, work_id: ?, batch_id: ?) → dict`
  - `finalize_work(work_id: str, status: str, extracted_items: ?, error_message: ?) → dict`
  - `_ingestion_endpoint(work_id: ?) → str`
  - `get_work(work_id: str) → dict`

### `scraper_core/extensions.py`

- **class DurableCrawlExtension**
  - `__init__(crawler)`
  - `from_crawler(cls, crawler)`
  - `on_error()`
  - `closed(spider, reason)`

### `scraper_core/freshness.py`

- **class RefreshQueue**
  - `__init__(product_ttl_seconds: int)`
  - `in_flight_refreshes()`
  - `mark_started(product_id)`
  - `mark_finished(product_id)`
  - `pop_next()`
  - `enqueue(product_id, last_updated_at, product_url)`
- **class RefreshWorker**
  - `__init__(queue, executor)`
  - `process_next()`
  - `process_all()`
- **class RefreshScheduler**
  - `__init__(catalog_interval_seconds: int, product_ttl_seconds: int, enabled: bool)`
  - `from_config(cls, config)`
  - `catalog_needs_refresh(last_updated_at)`
  - `product_needs_refresh(product_id, last_updated_at)`
  - `build_catalog_and_product_plan(last_catalog_updated_at, product_id, last_product_updated_at, force_catalog_refresh)`
- `_normalize_datetime(value)`
- `should_refresh_catalog(last_updated_at, interval_seconds: int) → bool`
- `should_refresh_product(last_updated_at, ttl_seconds: int) → bool`
- `build_refresh_decision(product_id, last_updated_at, ttl_seconds: int)`
- `should_skip_refresh(product_id, in_flight_refreshes)`

### `scraper_core/items.py`

- **class ScraperCoreItem**

### `scraper_core/middlewares.py`

- **class ScraperCoreSpiderMiddleware**
  - `from_crawler(cls, crawler)`
  - `process_spider_input(response, spider)`
  - `process_spider_output(response, result, spider)`
  - `process_spider_exception(response, exception, spider)`
  - `async process_start(start)`
  - `spider_opened(spider)`
- **class ScraperCoreDownloaderMiddleware**
  - `from_crawler(cls, crawler)`
  - `process_request(request, spider)`
  - `process_response(request, response, spider)`
  - `process_exception(request, exception, spider)`
  - `spider_opened(spider)`

### `scraper_core/migrate_queue.py`

- `migrate_one(redis, api)`
- `main()`

### `scraper_core/normalization.py`

- `normalizar_precio_clp(value)`
- `_uses_thousands_groups(parts)`
- `normalizar_ean_gtin(value)`
- `normalizar_texto(value)`

### `scraper_core/outbox.py`

- **class Outbox**
  - `__init__(work_id: str, state_dir: ?)`
  - `close()`
  - `add(product: dict, market: ?)`
  - `unbatched()`
  - `seal(batch_size)`
  - `replay(client, cancelled)`
  - `progress(key)`
  - `set_progress(key, value)`

### `scraper_core/output.py`

- **class ScraperResultPublisher**
  - `__init__(sender)`
  - `publish(item)`

### `scraper_core/pipelines.py`

- **class ScraperCorePipeline**
  - `__init__(api_client, batch_size)`
  - `from_crawler(cls, crawler)`
  - `process_item(item, spider)`
  - `close_spider(spider)`
  - `_flush()`

### `scraper_core/runtime.py`

- **class ScrapyCommandExecutor**
  - `__init__(command_runner, spider_name)`
  - `__call__(job, cancelled)`
- `main()`

### `scraper_core/settings.py`


### `scraper_core/spiders/acuenta.py`

- **class AcuentaRscSpider(scrapy.Spider)**
  - `__init__()`
  - `from_crawler(cls, crawler)`
  - `start_requests()`
  - `parse(response)`
  - `handle_error(failure)`
  - `_extract_products(response)`
  - `_merge_visible_prices(response, products)`
  - `_extract_rsc_promotion_prices(text)`
  - `_extract_rsc_products(response)`
  - `_resolve_image_reference(text, window)`
  - `_image_from_rsc(text, sku)`
  - `_image_from_sku(sku)`
  - `_canonical_image_url(image_url)`
  - `_name_matches_slug(name, slug)`
  - `_name_from_slug(slug)`
  - `_prices_from_text(text)`
  - `_brand_from_text(text)`
  - `_format_from_text(text)`
  - `_sku_from_url(product_url)`
  - `_to_scraped_item(product, category, response_url)`
  - `_category_from_url(category_url)`
  - `_page_number(url)`
  - `_page_url(category_url, page)`
  - `_read_category_urls(cls)`
  - `_append_category_url(url)`
  - `_validate_category_url(url)`

### `scraper_core/spiders/cugat.py`

- **class CugatRscSpider(scrapy.Spider)**
  - `__init__()`
  - `from_crawler(cls, crawler)`
  - `start_requests()`
  - `parse(response)`
  - `parse_product(response)`
  - `parse_product_detail(response, catalog_product, category, category_page_url)`
  - `handle_detail_error(failure)`
  - `handle_error(failure)`
  - `_extract_products(response)`
  - `_price_from(card, selector)`
  - `_extract_detail_product(response)`
  - `_walk_json(value)`
  - `_stock_value(availability)`
  - `_to_scraped_item(product, category, response_url)`
  - `_category_from_url(category_url)`
  - `_page_url(category_url, page)`
  - `_read_category_urls(cls)`
  - `_append_category_url(url)`
  - `_validate_category_url(url)`
  - `_validate_product_url(url)`

### `scraper_core/spiders/jumbo.py`

- **class JumboRscSpider(scrapy.Spider)**
  - `parse(response)`
  - `parse_ean_detail(response, catalog_product, category, category_page_url)`
  - `handle_ean_detail_error(failure)`
  - `start_requests()`
  - `parse_product(response)`
  - `_to_scraped_item(product, category, response_url)`
  - `_validate_product_url(url)`
  - `_canonical_product_url(url)`
  - `_page_url(category_url, page)`
  - `handle_error(failure)`
  - `__init__()`
  - `from_crawler(cls, crawler)`
  - `_read_category_urls(cls)`
  - `_append_category_url(url)`
  - `_extract_products(response)`
  - `_first_value(data)`
  - `_first_price(data)`
  - `_sku_from_url(product_url)`
  - `_normal_price(entry)`
  - `_stock_value(availability)`
  - `_walk_json(value)`

### `scraper_core/spiders/lider.py`

- **class LiderRscSpider(scrapy.Spider)**
  - `__init__()`
  - `from_crawler(cls, crawler)`
  - `start_requests()`
  - `parse(response)`
  - `handle_error(failure)`
  - `_extract_products(response)`
  - `_extract_next_data_products(response)`
  - `_extract_card_products(response)`
  - `_price_from(selector, price_selector)`
  - `_format_from_text(name)`
  - `_to_scraped_item(product, category, response_url)`
  - `_category_from_url(category_url)`
  - `_page_number(url)`
  - `_page_url(category_url, page)`
  - `_read_category_urls(cls)`
  - `_append_category_url(url)`
  - `_validate_category_url(url)`
  - `_validate_product_url(url)`

### `scraper_core/spiders/santa_isabel.py`

- **class SantaIsabelRscSpider(scrapy.Spider)**
  - `__init__()`
  - `from_crawler(cls, crawler)`
  - `start_requests()`
  - `parse(response)`
  - `_extract_products(response)`
  - `_number(value)`
  - `_to_scraped_item(product, category, response_url)`
  - `_page_url(category_url, page)`
  - `_read_category_urls(cls)`
  - `_append_category_url(url)`
  - `handle_error(failure)`
  - `_validate_category_url(url)`

### `scraper_core/worker.py`

- **class CrawlExhausted(RuntimeError)**
- **class ReliableQueue**
  - `__init__(client, consumer, lease_ms)`
  - `ensure_group()`
  - `receive()`
  - `renew(message_id)`
  - `ack(message_id, raw_job, work_id, reason)`
  - `heartbeat(message_id)`
- `job_environment(work_id)`
- `process_job(job: dict, api_client: ScraperAPIClient, cancelled) → dict`
- `handle_message(queue, message, api_client)`
- `run_worker()`
