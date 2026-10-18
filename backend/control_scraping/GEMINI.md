# Control de scraping independiente

SUP-273: servicio dueño de trabajos/outboxes, no de usuarios o productos.
Entry `cmd/control_scraping/main.go`, puerto 8080. Sólo SCRAPING_DB_URL/app_scraping
(pool 5), Redis y token interno de Catálogo; sin JWT_SECRET ni modelos compartidos.
Conservar los protocolos/lotes/leases de SUP-270/271 y los contratos públicos.
Gateway usa SCRAPING_URL; probes privados por loopback. Ver
`docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Control de scraping independiente (Go + Gin)

SUP-273: dueño exclusivo de trabajos y outboxes; SCRAPING_DB_URL/app_scraping, pool 5. /_scraping/live independiente y /_scraping/ready SQL con Redis diagnóstico. Conserva recuperación SUP-270/271, ingesta HTTP idempotente a Catálogo y contrato worker. Sin JWT_SECRET, modelos de usuarios ni persistencia de catálogo. Gateway usa SCRAPING_URL; no existe servicio backend/api. Ver docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md.

**34 archivos .go** detectados


### `cmd/control_scraping/hardening_test.go` (package main)

- `TestScrapingCORSRejectsBeforeBusinessHandler(t *testing.T)`
- `TestRedisConfigurationNeverFallsBack(t *testing.T)`

### `cmd/control_scraping/health_test.go` (package main)

- Structs: healthConnector, healthDriver, healthConn
- `TestHealthRedisTimeoutDoesNotRemoveReadiness(t *testing.T)`
- `(healthConnector).Connect(context.Context) (driver.Conn, error)`
- `(healthConnector).Driver() driver.Driver`
- `(healthConn).Ping(ctx context.Context) error`
- `probeDatabase(t *testing.T, connector *healthConnector) *gorm.DB`
- `TestHealthRequiresScrapingDatabase(t *testing.T)`

### `cmd/control_scraping/main.go` (package main)

- `initDB(ctx context.Context) error`
- `initRedis() error`
- `setupRouter() (*gin.Engine, error)`
- `setupApplication() (*gin.Engine, *control.Module, error)`
- `RootHandler(c *gin.Context)`
- `HealthHandler(c *gin.Context)`
- `ReadinessHandler(c *gin.Context)`
- `writeHealth(c *gin.Context, service string)`
- `LivenessHandler(c *gin.Context)`
- `main()`
- `run() error`

### `cmd/control_scraping/main_test.go` (package main)

- `TestModuleCompositionHTTP(t *testing.T)`

### `cmd/control_scraping/swagger_test.go` (package main)

- `TestSwaggerOnlyDocumentsRegisteredRoutes(t *testing.T)`

### `internal/control/lifecycle_test.go` (package control)

- `TestCanceledRecoveryLoopsClosePublisherBeforeReturn(t *testing.T)`
- `TestRecoveryShutdownHasDeadline(t *testing.T)`

### `internal/control/module.go` (package control)

- Structs: Module
- `New(db *gorm.DB, rdb *redis.Client) (*Module, error)`
- `(Module).VerifyStorage(ctx context.Context) error`
- `(Module).Start(ctx context.Context)`
- `(Module).Wait(ctx context.Context) error`
- `(Module).RegisterRoutes(group *gin.RouterGroup)`

### `internal/database/connections.go` (package database)

- Structs: Config
- `Load() (Config, error)`
- `Open(ctx context.Context, dsn string) (*gorm.DB, error)`
- `Close(db *gorm.DB)`

### `internal/database/connections_test.go` (package database)

- `TestRuntimeCredentials(t *testing.T)`

### `internal/domain/entrega.go` (package domain)

- Structs: Entrega, loteIDKey, Ejecucion
- `ConLoteID(ctx context.Context, id string) context.Context`
- `LoteID(ctx context.Context) string`

### `internal/domain/models.go` (package domain)

- Structs: CadenaSupermercado, SucursalSupermercado, TrabajoScraper

### `internal/domain/repository.go` (package domain)


### `internal/domain/scraper.go` (package domain)

- Structs: IniciarTrabajoDTO, TrabajoScraperDTO, FinalizarTrabajoDTO, ProductoScrapeadoDTO, IngestaLoteDTO, IngestaResultadoDTO

### `internal/handlers/scraper_handler.go` (package handlers)

- Structs: ScraperHandler, ejecutarScraperRequest
- `(ScraperHandler).SetQueue(queue *services.QueueService)`
- `NewScraperHandler(service services.ScraperService, rdb ...*redis.Client) *ScraperHandler`
- `(ScraperHandler).EjecutarTrabajo(c *gin.Context)`
- `(ScraperHandler).IniciarTrabajo(c *gin.Context)`
- `(ScraperHandler).FinalizarTrabajo(c *gin.Context)`
- `(ScraperHandler).ObtenerTrabajo(c *gin.Context)`
- `(ScraperHandler).IngestarProductosConTrabajo(c *gin.Context)`
- `(ScraperHandler).IngestarProductosDirecto(c *gin.Context)`
- `(ScraperHandler).manejarErrorIngesta(c *gin.Context, err error)`
- `prepararLote(c *gin.Context, input *domain.IngestaLoteDTO) bool`
- `(ScraperHandler).responderErrorCatalogo(c *gin.Context, err error) bool`

### `internal/middleware/error_handler.go` (package middleware)

- Structs: RespuestaError
- `ErrorHandler() gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`

### `internal/middleware/error_handler_test.go` (package middleware)

- `TestInternalDiagnosticsAreNotReturned(t *testing.T)`

### `internal/middleware/runtime.go` (package middleware)

- `BoundedRequest() gin.HandlerFunc`
- `CORS(origins []string) gin.HandlerFunc`
- `Observe(log *slog.Logger) gin.HandlerFunc`

### `internal/repositories/catalogo_client.go` (package repositories)

- Structs: catalogoClient, requestIDKey
- `NewCatalogoClient(origin, token string, timeout time.Duration) (domain.IngestaRepository, error)`
- `(catalogoClient).call(ctx context.Context, path string, payload, result any) (bool, error)`
- `WithRequestID(ctx context.Context, id string) context.Context`
- `(catalogoClient).ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error)`
- `(catalogoClient).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigo *string) (*domain.SucursalSupermercado, error)`
- `(catalogoClient).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`

### `internal/repositories/catalogo_client_test.go` (package repositories)

- `TestCatalogoClientContract(t *testing.T)`
- `TestCatalogoClientFailuresAndNoRetry(t *testing.T)`
- `TestCatalogoClientTimeoutAndCancellation(t *testing.T)`
- `TestCatalogoClientInvalidConfiguration(t *testing.T)`

### `internal/repositories/ejecucion_repository.go` (package repositories)

- Structs: ejecucionRepository
- `NewEjecucionRepository(db *gorm.DB) domain.EjecucionRepository`
- `(ejecucionRepository).RegistrarEjecucion(ctx context.Context, e domain.Ejecucion) error`
- `(ejecucionRepository).Pendientes(ctx context.Context) ([]domain.Ejecucion, error)`
- `(ejecucionRepository).PosponerPublicacion(ctx context.Context, id uuid.UUID) error`

### `internal/repositories/entrega_repository.go` (package repositories)

- Structs: entregaRepository
- `NewEntregaRepository(db *gorm.DB) domain.EntregaRepository`
- `(entregaRepository).Obtener(ctx context.Context, id uuid.UUID) (*domain.Entrega, error)`
- `(entregaRepository).Registrar(ctx context.Context, e *domain.Entrega) (*domain.Entrega, error)`
- `(entregaRepository).Adquirir(ctx context.Context, id *uuid.UUID) (*domain.Entrega, error)`
- `(entregaRepository).FijarSucursal(ctx context.Context, id uuid.UUID, branch int) (int, error)`
- `(entregaRepository).Confirmar(ctx context.Context, e *domain.Entrega, result domain.IngestaResultadoDTO) error`
- `(entregaRepository).Reprogramar(ctx context.Context, e *domain.Entrega, code string, permanent bool) error`

### `internal/repositories/idempotencia_test.go` (package repositories)

- `TestBatchKeyIsSentWithoutJobIdentityAndConflictIsNotRetried(t *testing.T)`
- `TestCatalogDataRejectionIsDistinctFromInfrastructureFailure(t *testing.T)`

### `internal/repositories/job_publisher.go` (package repositories)

- Structs: jobPublisher
- `NewJobPublisher(rdb *redis.Client, db *gorm.DB) *jobPublisher`
- `(jobPublisher).Close() error`
- `(jobPublisher).Publicar(ctx context.Context, e domain.Ejecucion) error`

### `internal/repositories/job_publisher_test.go` (package repositories)

- `TestPublisherStalledRedisAllowsJobFinalization(t *testing.T)`

### `internal/repositories/trabajo_repository.go` (package repositories)

- Structs: gormTrabajoRepository
- `NewTrabajoRepository(db *gorm.DB) domain.TrabajoRepository`
- `(gormTrabajoRepository).CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error`
- `(gormTrabajoRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error)`
- `(gormTrabajoRepository).IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error`
- `(gormTrabajoRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*domain.TrabajoScraper, error)`

### `internal/routes/scraper_routes.go` (package routes)

- `RegistrarRutasScraper(rg *gin.RouterGroup, scraperHandler *handlers.ScraperHandler)`

### `internal/services/ejecuciones.go` (package services)

- Structs: QueueService
- `NewQueueService(jobs domain.EjecucionRepository, publisher domain.JobPublisher) *QueueService`
- `(QueueService).Encolar(ctx context.Context, id string, payload []byte) error`
- `(QueueService).Recuperar(ctx context.Context)`

### `internal/services/entregas.go` (package services)

- `(scraperService).ingestarDurable(ctx context.Context, trabajoID *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).entregar(ctx context.Context, e *domain.Entrega) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).Recuperar(ctx context.Context)`

### `internal/services/scraper_service.go` (package services)

- Structs: scraperService
- `NewScraperService(trabajos domain.TrabajoRepository, ingesta domain.IngestaRepository, entregas ...domain.EntregaRepository) ScraperService`
- `(scraperService).IniciarTrabajo(ctx context.Context, input domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).FinalizarTrabajo(ctx context.Context, id string, input domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).ObtenerTrabajo(ctx context.Context, id string) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).IngestarProductos(ctx context.Context, trabajoIDStr *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).mapearTrabajoDTO(t *domain.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO`
- `(scraperService).nombreTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) string`
- `(scraperService).nombreCadena(ctx context.Context, cadenaID int) string`

### `internal/tests/ingesta_failures_test.go` (package tests)

- Structs: failingCounter
- `(failingCounter).IncrementarElementos(ctx context.Context, id uuid.UUID, n int) error`
- `TestCatalogFailureDoesNotIncrementJob(t *testing.T)`
- `TestCommittedCatalogBatchAndFailedCounterIsExplicit(t *testing.T)`

### `internal/tests/queue_test.go` (package tests)

- Structs: sqlQueue
- `(sqlQueue).RegistrarEjecucion(_ context.Context, e domain.Ejecucion) error`
- `(sqlQueue).Pendientes(context.Context) ([]domain.Ejecucion, error)`
- `(sqlQueue).PosponerPublicacion(context.Context, uuid.UUID) error`
- `TestExecutionWithoutRedisClientStillPersistsSQL(t *testing.T)`

### `internal/tests/scraper_test.go` (package tests)

- Structs: mockScraperRepository
- `newMockScraperRepository() *mockScraperRepository`
- `(mockScraperRepository).CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error`
- `(mockScraperRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error)`
- `(mockScraperRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*domain.TrabajoScraper, error)`
- `(mockScraperRepository).ObtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*domain.CadenaSupermercado, error)`
- `(mockScraperRepository).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*domain.SucursalSupermercado, error)`
- `(mockScraperRepository).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`
- `(mockScraperRepository).IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error`
- `TestScraperService_IniciarTrabajo_Exitoso(t *testing.T)`
- `TestScraperService_FinalizarTrabajo_Validaciones(t *testing.T)`
- `TestScraperService_IngestarProductos_LimitesYValidaciones(t *testing.T)`
- `TestScraperHandler_EndpointsHTTP(t *testing.T)`
- `stringPtr(s string) *string`

### `tests/architecture_test.go` (package tests)

- `TestModuleDependencies(t *testing.T)`

### `tests/middleware_test.go` (package tests)

- `init()`
- `setupTestRouter() *gin.Engine`
- `TestNotFoundHandler(t *testing.T)`
- `TestMethodNotAllowedHandler(t *testing.T)`
- `TestPanicRecoveryHandler(t *testing.T)`
- `TestResponderError(t *testing.T)`
