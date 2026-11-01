<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Scraper Worker (Go)

Worker de scraping en Go con Redis Streams y spool SQLite durable.

**38 archivos .go** detectados


### `cmd/extract/main.go` (package main)

- `(stringList).String() string`
- `(stringList).Set(value string) error`
- `main()`
- `run() error`
- `collectURLs(values []string, file string) ([]string, error)`
- `directoryExists(path string) bool`

### `cmd/extract/main_test.go` (package main)

- `TestCollectURLs(t *testing.T)`

### `cmd/worker/main.go` (package main)

- `main()`
- `run() error`

### `internal/config/config.go` (package config)

- Structs: Config
- `FromEnv() (Config, error)`
- `env(key, fallback string) string`
- `envInt(key string, fallback int) int`
- `parseLevel(raw string) slog.Level`

### `internal/config/config_test.go` (package config)

- `TestFromEnvDefaults(t *testing.T)`
- `TestFromEnvRejectsInvalidLimits(t *testing.T)`

### `internal/controlscraping/client.go` (package controlscraping)

- Structs: Error, Client, ingestPayload
- `(Error).Error() string`
- `(Error).Unwrap() error`
- `(Error).Permanent() bool`
- `New(endpoint, token string, timeout time.Duration) *Client`
- `(Client).IngestBatch(ctx context.Context,
	products []domain.Product,
	supermarket *string,
	workID string,
	batchID string,) (map[string]any, error)`
- `(Client).FinalizeWork(ctx context.Context,
	workID string,
	status string,
	extractedItems *int,
	blockedResponses *int,
	errorMessage string,) (map[string]any, error)`
- `(Client).GetWork(ctx context.Context, workID string) (map[string]any, error)`
- `(Client).do(request *http.Request) ([]byte, error)`
- `(Client).base() string`
- `(Client).ingestionEndpoint(workID string) string`
- `limitProducts(products []domain.Product) []domain.Product`
- `asInt(value any) (int, bool)`

### `internal/controlscraping/client_test.go` (package controlscraping)

- `floatPtr(value float64) *float64`
- `TestIngestBatchAndFinalizeWork(t *testing.T)`
- `TestIngestBatchLimitsDatabaseColumns(t *testing.T)`
- `value(pointer *string) string`
- `TestIngestBatchRejectsInconsistentReceipt(t *testing.T)`
- `TestPermanentAPIError(t *testing.T)`

### `internal/crawler/client.go` (package crawler)

- Structs: Response, BlockedError, HTTPStatusError, Options, Client, throttle, robotsEntry, robotsCache
- `(BlockedError).Error() string`
- `(HTTPStatusError).Error() string`
- `New(opts Options) *Client`
- `newHTTPClient(timeout time.Duration) *http.Client`
- `(Client).Fetch(ctx context.Context, rawURL string) (*Response, error)`
- `(Client).doRequest(ctx context.Context, rawURL string) (*Response, error)`
- `(Client).backoff(ctx context.Context, attempt int, retryAfter time.Duration) error`
- `retryAfter(header http.Header) time.Duration`
- `detectBlock(status int, body []byte) string`
- `newThrottle() *throttle`
- `(throttle).Wait(ctx context.Context, host string, delay time.Duration) error`
- `newRobotsCache(client *http.Client, userAgent string, logger *slog.Logger) *robotsCache`
- `(robotsCache).Allowed(ctx context.Context, target *url.URL) (bool, time.Duration, error)`
- `(robotsCache).fetch(ctx context.Context, base string) (*robotsEntry, error)`
- `parseCrawlDelay(body []byte, userAgent string) time.Duration`
- `matchesAgent(agents []string, userAgent string) bool`

### `internal/crawler/client_test.go` (package crawler)

- `testLogger() *slog.Logger`
- `testClient() *Client`
- `(roundTripFunc).RoundTrip(request *http.Request) (*http.Response, error)`
- `TestRobotsCacheCachesFailure(t *testing.T)`
- `TestFetchReturns404ForSpiderPagination(t *testing.T)`
- `TestFetchAcceptsPageWithCDNScripts(t *testing.T)`
- `TestFetchRetriesTransientStatus(t *testing.T)`
- `TestFetchReportsPersistentBlock(t *testing.T)`
- `TestFetchClassifiesBlockStatuses(t *testing.T)`
- `TestDetectBlockClassifiesStatusesAndBodies(t *testing.T)`
- `TestRetryAfter(t *testing.T)`
- `TestBackoffHonorsRetryAfter(t *testing.T)`
- `TestRetryAfterHTTPDate(t *testing.T)`
- `TestFetchRetriesTimeout(t *testing.T)`
- `TestDetectBlockIgnoresRecaptchaOnNormalPage(t *testing.T)`
- `TestDetectBlockRecognizesPerimeterX(t *testing.T)`
- `TestFetchHonorsRobots(t *testing.T)`

### `internal/domain/product.go` (package domain)

- Structs: Product, Item, Job
- `StringPtr(value string) *string`
- `BoolPtr(value bool) *bool`

### `internal/normalize/normalize.go` (package normalize)

- `PrecioCLP(value any) *float64`
- `priceText(value any) (string, bool)`
- `parseAmount(text string) (*big.Rat, bool)`
- `usesThousandsGroups(parts []string) bool`
- `EAN(value any) *string`
- `Truncar(value string, max int) string`
- `TruncarPtr(value *string, max int) *string`
- `LimitarProducto(product domain.Product) domain.Product`
- `SKUFromURL(rawURL string) *string`
- `isDigits(value string) bool`
- `Texto(value any) *string`
- `formatString(value any) string`

### `internal/normalize/normalize_test.go` (package normalize)

- `TestPrecioCLP(t *testing.T)`
- `TestEAN(t *testing.T)`
- `TestTexto(t *testing.T)`
- `TestTruncar(t *testing.T)`
- `TestSKUFromURL(t *testing.T)`

### `internal/queue/queue.go` (package queue)

- Structs: Options, Queue
- `New(client *redis.Client, opts Options) (*Queue, error)`
- `(Queue).Consumer() string`
- `(Queue).Stream() string`
- `(Queue).Group() string`
- `(Queue).EnsureGroup(ctx context.Context) error`
- `(Queue).Receive(ctx context.Context) (*redis.XMessage, error)`
- `(Queue).Renew(ctx context.Context, messageID string) (bool, error)`
- `(Queue).Ack(ctx context.Context, messageID, rawJob, workID, reason string) (bool, error)`
- `(Queue).HasMessage(ctx context.Context, workID string) (bool, error)`
- `(Queue).Heartbeat(parent context.Context, messageID string) (
	context.Context,
	context.CancelFunc,
	<-chan struct`
- `IsLost(lost <-chan struct{}) bool`

### `internal/queue/queue_test.go` (package queue)

- Structs: queueFixture
- `newQueueFixture(t *testing.T, consumer string) *queueFixture`
- `(queueFixture).addJob(t *testing.T, payload string) string`
- `TestQueueAckOwnershipAndDeadLetter(t *testing.T)`
- `TestQueueAckSuccessDoesNotCreateDeadLetter(t *testing.T)`
- `TestQueueRejectsShortLease(t *testing.T)`

### `internal/spiders/acuenta.go` (package spiders)

- Structs: Acuenta
- `NewAcuenta(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Acuenta`
- `acuentaCategoryFromURL(rawURL string) string`
- `acuentaPageNumber(rawURL string) int`
- `acuentaPageURL(rawURL string, page int) string`
- `extractAcuentaProducts(body []byte, responseURL *url.URL) []domain.Product`
- `extractAcuentaCards(body []byte, responseURL *url.URL) []domain.Product`
- `mergeAcuentaVisiblePrices(body []byte, responseURL *url.URL, products []domain.Product) []domain.Product`
- `extractAcuentaRSCProducts(body []byte, responseURL *url.URL) []domain.Product`
- `acuentaPromotionPrices(text string) map[string]*float64`
- `acuentaImageFromRSC(text, sku string) string`
- `acuentaResolveImageReference(text, window string) string`
- `acuentaImageFromSKU(sku string) string`
- `acuentaCanonicalImageURL(value string) *string`
- `acuentaNameMatchesSlug(name, slug string) bool`
- `acuentaTokens(value string) map[string]struct`
- `acuentaNameFromSlug(slug string) *string`
- `acuentaPricesFromText(text string) (*float64, *float64, *string)`
- `acuentaBrandFromText(text string) *string`
- `acuentaSKUFromURL(productURL string) *string`

### `internal/spiders/acuenta_test.go` (package spiders)

- `TestExtractAcuentaProductCard(t *testing.T)`
- `TestAcuentaMergesVisibleOfferIntoRSCProduct(t *testing.T)`
- `TestAcuentaNameFromSlugUsesTitleCase(t *testing.T)`
- `TestAcuentaPageURLRemovesInternalToken(t *testing.T)`
- `TestAcuentaMultiunitPromotion(t *testing.T)`
- `TestAcuentaDoesNotCrossRSCScriptBoundaries(t *testing.T)`
- `TestAcuentaUsesFieldsAfterSKU(t *testing.T)`
- `TestAcuentaResolvesEANReference(t *testing.T)`
- `TestAcuentaResolvesSpecialPriceChain(t *testing.T)`
- `TestAcuentaDoesNotUseMultiunitRSCPriceAsOffer(t *testing.T)`
- `TestAcuentaResolvesImageReference(t *testing.T)`
- `TestAcuentaUsesExactImageVariantForSKU(t *testing.T)`
- `TestAcuentaEncodesImageProxyBrackets(t *testing.T)`
- `TestValidateAcuentaCategoryURL(t *testing.T)`
- `TestAcuentaRunPagination(t *testing.T)`
- `TestAcuentaProductMode(t *testing.T)`
- `TestAcuentaRejectsNonProductURL(t *testing.T)`

### `internal/spiders/base.go` (package spiders)

- Structs: spiderConfig, baseSpider
- `newBaseSpider(fetcher Fetcher,
	categoryDir string,
	maxPages int,
	logger *slog.Logger,
	cfg spiderConfig,) baseSpider`
- `(baseSpider).Name() string`
- `(baseSpider).Run(ctx context.Context, job domain.Job, emit Emitter) error`
- `(baseSpider).runProduct(ctx context.Context, job domain.Job, emit Emitter) error`
- `(baseSpider).runCategory(ctx context.Context, categoryURL string, enrichEAN bool, emit Emitter) error`
- `(baseSpider).item(ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,) (domain.Item, error)`
- `(baseSpider).startURLs(overrides []string) ([]string, error)`
- `(baseSpider).appendCategoryURL(rawURL string) error`
- `(baseSpider).toItem(product domain.Product, category, responseURL string) domain.Item`
- `pageURLOrSelf(rawURL string, page int) string`
- `urlString(value *url.URL) string`

### `internal/spiders/common.go` (package spiders)

- `formatFromText(value string, pattern *regexp.Regexp) *string`
- `titleCase(value string) string`
- `readCategoryURLs(file string) []string`
- `appendCategoryURL(file, rawURL string, validate func(string) error) error`
- `envCategoryURLs(name string) []string`
- `dedupeStrings(values []string) []string`
- `categoryFromURL(rawURL string) string`
- `withPage(rawURL string, page int) string`
- `resolveURL(base *url.URL, reference string) string`
- `validateHTTPSHost(rawURL string, hosts map[string]struct{}, message string) error`
- `validateSiteURL(hosts map[string]struct{},
	prefix string,
	suffix string,
	message string,
	minSegments int,) func(string) error`
- `firstText(selection *goquery.Selection, selector string) string`
- `allText(selection *goquery.Selection) string`
- `collectText(node *html.Node, parts *[]string)`
- `firstAttr(selection *goquery.Selection, selector, attribute string) string`
- `noscriptImage(selection *goquery.Selection) string`
- `firstNonNilString(values ...*string) *string`
- `nonEmptyStringPtr(value string) *string`
- `lastPathSegment(rawURL string) string`
- `isDigits(value string) bool`
- `nonEmptySegments(path string) []string`
- `mustParsePath(rawURL string) string`
- `hasClass(classes, wanted string) bool`
- `appendQueryParam(rawURL, key, value string) string`
- `removeQueryParam(rawURL, key string) string`
- `queryPairs(rawQuery string) []string`
- `selectProductByURL(products []domain.Product, requested string, responseURL *url.URL) *domain.Product`
- `extractJSONLDProduct(body []byte, responseURL *url.URL) *domain.Product`
- `extractJSONLDProducts(body []byte, responseURL *url.URL) []domain.Product`
- `canonicalProductURL(rawURL string) string`
- `schemaOffer(entry map[string]any) map[string]any`
- `extractNormalPrice(entry map[string]any) *float64`
- `firstValue(data any, keys ...string) any`
- `firstPrice(data any, keys ...string) *float64`
- `valueString(value any) string`
- `stockValue(availability any) *bool`
- `walkJSON(value any) []any`
- `cugatImageURL(value any) *string`
- `isProductType(value any) bool`
- `firstMapValue(value any) any`

### `internal/spiders/complete.go` (package spiders)

- `CompleteItem(item domain.Item) domain.Item`

### `internal/spiders/complete_test.go` (package spiders)

- `TestCompleteItemDerivesSKUFromProductURL(t *testing.T)`
- `TestCompleteItemKeepsExistingSKU(t *testing.T)`
- `TestCompleteItemTruncatesSKU(t *testing.T)`
- `TestCompleteItemTruncatesAllCatalogFields(t *testing.T)`
- `TestCompleteItemDerivesSKUBeforeTruncating(t *testing.T)`

### `internal/spiders/cugat.go` (package spiders)

- Structs: Cugat
- `NewCugat(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Cugat`
- `cugatPageURL(rawURL string, page int) string`
- `cugatCategoryFromURL(rawURL string) string`
- `extractCugatProducts(body []byte, responseURL *url.URL) []domain.Product`
- `cugatPriceFrom(card *goquery.Selection, selector string) *float64`
- `extractCugatDetail(body []byte, responseURL *url.URL) *domain.Product`

### `internal/spiders/cugat_test.go` (package spiders)

- `TestRegistryContainsAllKnownSpiders(t *testing.T)`
- `TestExtractCugatProducts(t *testing.T)`
- `TestCugatImageFallsBackToNoscript(t *testing.T)`
- `TestCugatPageURLAndCategory(t *testing.T)`
- `TestExtractCugatDetail(t *testing.T)`
- `TestExtractCugatDetailDerivesSKUFromURL(t *testing.T)`
- `TestValidateCugatURLs(t *testing.T)`
- `TestCugatProductMode(t *testing.T)`
- `TestCugatRunPagination(t *testing.T)`

### `internal/spiders/empty_category_test.go` (package spiders)

- `TestEmptyCategoryDoesNotAbortChain(t *testing.T)`

### `internal/spiders/jumbo.go` (package spiders)

- Structs: Jumbo
- `NewJumbo(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Jumbo`
- `(Jumbo).enrich(ctx context.Context,
	product domain.Product,
	category string,
	responseURL *url.URL,
	enrichEAN bool,) (domain.Item, error)`
- `selectJumboProduct(products []domain.Product, requested string, responseURL *url.URL) *domain.Product`
- `extractJumboProducts(body []byte) []domain.Product`
- `skuFromURL(rawURL string) *string`

### `internal/spiders/jumbo_test.go` (package spiders)

- Structs: fakeFetcher
- `(fakeFetcher).Fetch(_ context.Context, rawURL string) (*crawler.Response, error)`
- `response(rawURL, body string) *crawler.Response`
- `mustURL(rawURL string) *url.URL`
- `quietLogger() *slog.Logger`
- `TestJumboEmptyCategoryDoesNotAbortRemainingCategories(t *testing.T)`
- `TestExtractJumboProducts(t *testing.T)`
- `TestExtractJumboComparisonFields(t *testing.T)`
- `TestExtractJumboNumericIdentifiers(t *testing.T)`
- `TestJumboProductMode(t *testing.T)`
- `TestJumboProductModeSelectsRequestedProduct(t *testing.T)`
- `TestJumboRejectsUntrustedProductURL(t *testing.T)`
- `TestJumboCategoryPagination(t *testing.T)`
- `TestJumboEnrichesEANFromDetail(t *testing.T)`
- `TestJumboRobotsDisallowedSkipsCategory(t *testing.T)`
- `TestWithPage(t *testing.T)`
- `TestCategoryFromURL(t *testing.T)`
- `TestValidateProductURL(t *testing.T)`
- `TestSkuFromURL(t *testing.T)`
- `TestJumboCategoryFileReadsComments(t *testing.T)`

### `internal/spiders/lider.go` (package spiders)

- Structs: Lider
- `NewLider(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Lider`
- `liderPageNumber(rawURL string) int`
- `liderPageURL(rawURL string, page int) string`
- `liderCategoryFromURL(rawURL string) string`
- `extractLiderProducts(body []byte, responseURL *url.URL) []domain.Product`
- `extractLiderNextData(document *goquery.Document, responseURL *url.URL) ([]domain.Product, bool)`
- `liderNextDataPayload(document *goquery.Document) (map[string]any, bool)`
- `extractLiderProduct(body []byte, responseURL *url.URL, requested string) *domain.Product`
- `liderProductFromItem(item map[string]any, responseURL *url.URL) (domain.Product, bool)`
- `extractLiderCards(document *goquery.Document, responseURL *url.URL) []domain.Product`

### `internal/spiders/lider_test.go` (package spiders)

- `TestExtractLiderProductsFromCards(t *testing.T)`
- `TestExtractLiderPrefersHydratedItems(t *testing.T)`
- `TestLiderFormatFromText(t *testing.T)`
- `TestValidateLiderURLs(t *testing.T)`
- `TestLiderPageURLAndCategory(t *testing.T)`
- `TestLiderRunPagination(t *testing.T)`
- `TestLiderProductMode(t *testing.T)`
- `TestLiderProductModeReadsNextDataProduct(t *testing.T)`
- `TestLiderRejectsNonProductURL(t *testing.T)`

### `internal/spiders/santa_isabel.go` (package spiders)

- Structs: SantaIsabel
- `NewSantaIsabel(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *SantaIsabel`
- `extractSantaIsabelProducts(body []byte, baseURL *url.URL) []domain.Product`
- `extractSantaIsabelRenderData(body []byte, baseURL *url.URL) []domain.Product`
- `extractSantaIsabelJSONLD(body []byte, baseURL *url.URL) []domain.Product`
- `santaIsabelSeller(value any) map[string]any`
- `firstMap(value any) map[string]any`
- `firstNonZeroPrice(data map[string]any, keys ...string) *float64`
- `firstTruthyString(values ...any) *string`
- `isTruthy(value any) bool`
- `numberValue(value any) *float64`
- `joinValueStrings(values ...any) *string`

### `internal/spiders/santa_isabel_test.go` (package spiders)

- `santaIsabelBody(t *testing.T, payload any) []byte`
- `santaIsabelPayload() map[string]any`
- `TestExtractSantaIsabelProducts(t *testing.T)`
- `TestExtractSantaIsabelJSONLD(t *testing.T)`
- `TestSantaIsabelOnlyUsesOwnSeller(t *testing.T)`
- `TestExtractSantaIsabelWithoutRenderData(t *testing.T)`
- `TestSantaIsabelCategoryPagination(t *testing.T)`
- `TestSantaIsabelAddURLCreatesCategoryFile(t *testing.T)`
- `TestSantaIsabelRejectsUntrustedCategory(t *testing.T)`
- `TestSantaIsabelRobotsDisallowed(t *testing.T)`
- `TestSantaIsabelProductMode(t *testing.T)`
- `TestSantaIsabelProductModeSelectsRequestedProduct(t *testing.T)`
- `TestSantaIsabelRejectsNonProductURL(t *testing.T)`

### `internal/spiders/spider.go` (package spiders)

- Structs: Registry
- `IsKnown(name string) bool`
- `Names() []string`
- `NewRegistry(fetcher Fetcher, categoryDir string, maxPages int, logger *slog.Logger) *Registry`
- `(Registry).Register(spider Spider)`
- `(Registry).Get(name string) (Spider, bool)`

### `internal/spool/spool.go` (package spool)

- Structs: Spool, itemRow, batchPayload
- `Open(workID, stateDir string, minFree int64) (*Spool, error)`
- `(Spool).initialize() error`
- `(Spool).WorkID() string`
- `(Spool).Directory() string`
- `(Spool).Close() error`
- `(Spool).Add(product domain.Product, market *string) error`
- `productFingerprint(market *string, data []byte) string`
- `(Spool).Count() (int, error)`
- `(Spool).Seal(batchSize int) error`
- `(Spool).firstUnbatchedMarket() (sql.NullString, bool, error)`
- `(Spool).unbatchedRows(market sql.NullString, batchSize int) ([]itemRow, error)`
- `(Spool).persistBatch(market sql.NullString, rows []itemRow) error`
- `(Spool).Replay(ctx context.Context, client *controlscraping.Client) error`
- `(Spool).Progress(key string) (int, error)`
- `(Spool).SetProgress(key string, value int) error`

### `internal/spool/spool_test.go` (package spool)

- `floatPtr(value float64) *float64`
- `TestSealAndReplay(t *testing.T)`
- `TestReplayRetriesSamePayloadAndIdempotencyKey(t *testing.T)`
- `TestReplayDoesNotSendWithCancelledContext(t *testing.T)`
- `TestOpenRejectsInvalidWorkID(t *testing.T)`

### `internal/storage/storage.go` (package storage)

- Structs: Collector
- `EnsureCapacity(directory string, minFree, extra int64) error`
- `FreeBytes(directory string) (int64, error)`
- `CleanupSpool(stateDir, workID string) (bool, error)`
- `NewCollector(stateDir string, minFree int64, retention time.Duration, logger *slog.Logger) *Collector`
- `(Collector).Collect(ctx context.Context, api *controlscraping.Client, queue *queue.Queue, limit int) (int, error)`
- `(Collector).isPressure() (bool, error)`
- `parseTimestamp(value string) (time.Time, error)`

### `internal/storage/storage_test.go` (package storage)

- `TestEnsureCapacityRejectsHugeExtra(t *testing.T)`
- `TestCleanupSpoolRespectsExecutionLock(t *testing.T)`

### `internal/worker/memory_test.go` (package worker)

- Structs: memoryGateResult, anonSampler, memorySpider
- `TestMemoryGate(t *testing.T)`
- `writeMemoryGateResult(t *testing.T, result memoryGateResult)`
- `startAnonSampler(interval time.Duration) *anonSampler`
- `(anonSampler).Stop() float64`
- `statusFieldKB(name string) (float64, bool)`
- `cgroupPeakMB() float64`
- `(memorySpider).Name() string`
- `(memorySpider).Run(_ context.Context, _ domain.Job, emit spiders.Emitter) error`
- `peakRSSMB(t *testing.T) float64`
- `envIntTest(name string, fallback int) int`
- `envFloatTest(name string, fallback float64) float64`
- `writeJSON(w http.ResponseWriter, payload any)`

### `internal/worker/recovery_test.go` (package worker)

- Structs: recoveryProcessAPI
- `newRecoveryProcessAPI(t *testing.T, workID string) (*httptest.Server, *recoveryProcessAPI)`
- `recoveryProcessWorker(t *testing.T, serverURL string, spider spiders.Spider) (*Worker, config.Config)`
- `TestProcessJobDoesNotCrawlTerminalJob(t *testing.T)`
- `TestProcessJobWithCrawlDoneFinalizesWithoutCrawling(t *testing.T)`
- `TestProcessJobStopsAfterThreeCrawlAttempts(t *testing.T)`

### `internal/worker/worker.go` (package worker)

- Structs: Worker, jobPayload, executionLock
- `New(cfg config.Config,
	api *controlscraping.Client,
	queueClient *queue.Queue,
	registry *spiders.Registry,
	logger *slog.Logger,) *Worker`
- `(Worker).Run(ctx context.Context) error`
- `(Worker).HandleMessage(ctx context.Context, message *redis.XMessage)`
- `(Worker).processJob(ctx context.Context, lost <-chan struct{}, job domain.Job) (string, error)`
- `(Worker).parseJob(raw string) (domain.Job, error)`
- `parseEnrich(value any) bool`
- `isTerminal(status map[string]any) bool`
- `stateOf(status map[string]any) string`
- `lockExecution(directory string) (*executionLock, error)`
- `(executionLock).Close() error`
- `sleep(ctx context.Context, duration time.Duration) bool`

### `internal/worker/worker_test.go` (package worker)

- Structs: fakeSpider, apiState, workerRunState
- `floatPtr(value float64) *float64`
- `testWorker(t *testing.T) *Worker`
- `TestParseJob(t *testing.T)`
- `TestParseJobAcceptsKnownSpider(t *testing.T)`
- `TestParseJobRejectsInvalidPayloads(t *testing.T)`
- `TestParseEnrich(t *testing.T)`
- `TestIsTerminal(t *testing.T)`
- `(fakeSpider).Name() string`
- `(fakeSpider).Run(_ context.Context, _ domain.Job, emit spiders.Emitter) error`
- `TestWorkerProcessesJobEndToEnd(t *testing.T)`
- `runWorkerJob(t *testing.T, workID string, spider spiders.Spider) *workerRunState`
- `TestWorkerFinalizesBlockedJob(t *testing.T)`
- `TestWorkerRejectsEmptyCrawl(t *testing.T)`
- `TestLockExecution(t *testing.T)`
