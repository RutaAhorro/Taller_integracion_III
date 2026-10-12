# Coordinación de scraping en API

Contexto vigente SUP-272. La API conserva sólo coordinación; Identidad reside
en `backend/identidad` y Catálogo en `backend/catalogo`. Entry `main.go`, puerto
8080. Ver `docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md`. No reintroducir dominios
extraídos ni credenciales SQL de otro propietario.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Coordinación de scraping en API (Go + Gin)

Estado vigente SUP-272: API ya no importa ni registra Identidad; usa sólo SCRAPING_DB_URL/app_scraping y un pool de 5 conexiones. Conserva los outboxes y todas las correcciones SUP-270/271. /health comprueba sólo SQL de coordinación y diagnostica Redis; /_api/live no consulta dependencias. Auth pertenece al servicio backend/identidad. Ver GUIA_IDENTIDAD_DESACOPLADA.md y GUIA_DATOS_Y_PERMISOS_BACKEND.md.

**33 archivos .go** detectados


### `cmd/seed_productos/main.go` (package main)

- Structs: productoSemilla
- `main()`

### `health_test.go` (package main)

- Structs: healthConnector, healthDriver, healthConn
- `TestHealthRedisTimeoutDoesNotRemoveReadiness(t *testing.T)`
- `(healthConnector).Connect(context.Context) (driver.Conn, error)`
- `(healthConnector).Driver() driver.Driver`
- `(healthConn).Ping(ctx context.Context) error`
- `probeDatabase(t *testing.T, connector *healthConnector) *gorm.DB`
- `TestHealthRequiresScrapingDatabase(t *testing.T)`

### `infrastructure/models.go` (package infrastructure)

- Structs: Usuario, PreferencialDieteticaUsuario, DireccionUsuario, PerfilTransporteUsuario, TarjetaFidelidadUsuario, MisionValidacion, Categoria, Marca, ProductoNormalizado, EquivalenciaProducto, Receta, IngredienteReceta, MapeoProductoIA, CadenaSupermercado, SucursalSupermercado, TrabajoScraper, ProductoCrudo, CapturaPrecio, ListaCompra, ArticuloListaCompra, EjecucionOptimizacion, ParadaOptimizacion, DetalleArticuloParada

### `internal/database/connections.go` (package database)

- Structs: Config
- `Load() (Config, error)`
- `Open(ctx context.Context, dsn string) (*gorm.DB, error)`
- `Close(db *gorm.DB)`

### `internal/database/connections_test.go` (package database)

- `TestRuntimeCredentials(t *testing.T)`

### `internal/scraping/internal/domain/entrega.go` (package domain)

- Structs: Entrega, loteIDKey, Ejecucion
- `ConLoteID(ctx context.Context, id string) context.Context`
- `LoteID(ctx context.Context) string`

### `internal/scraping/internal/domain/models.go` (package domain)

- Structs: CadenaSupermercado, SucursalSupermercado, TrabajoScraper

### `internal/scraping/internal/domain/repository.go` (package domain)


### `internal/scraping/internal/domain/scraper.go` (package domain)

- Structs: IniciarTrabajoDTO, TrabajoScraperDTO, FinalizarTrabajoDTO, ProductoScrapeadoDTO, IngestaLoteDTO, IngestaResultadoDTO

### `internal/scraping/internal/handlers/scraper_handler.go` (package handlers)

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

### `internal/scraping/internal/repositories/catalogo_client.go` (package repositories)

- Structs: catalogoClient, requestIDKey
- `NewCatalogoClient(origin, token string, timeout time.Duration) (domain.IngestaRepository, error)`
- `(catalogoClient).call(ctx context.Context, path string, payload, result any) (bool, error)`
- `WithRequestID(ctx context.Context, id string) context.Context`
- `(catalogoClient).ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error)`
- `(catalogoClient).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigo *string) (*domain.SucursalSupermercado, error)`
- `(catalogoClient).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`

### `internal/scraping/internal/repositories/catalogo_client_test.go` (package repositories)

- `TestCatalogoClientContract(t *testing.T)`
- `TestCatalogoClientFailuresAndNoRetry(t *testing.T)`
- `TestCatalogoClientTimeoutAndCancellation(t *testing.T)`
- `TestCatalogoClientInvalidConfiguration(t *testing.T)`

### `internal/scraping/internal/repositories/ejecucion_repository.go` (package repositories)

- Structs: ejecucionRepository
- `NewEjecucionRepository(db *gorm.DB) domain.EjecucionRepository`
- `(ejecucionRepository).RegistrarEjecucion(ctx context.Context, e domain.Ejecucion) error`
- `(ejecucionRepository).Pendientes(ctx context.Context) ([]domain.Ejecucion, error)`
- `(ejecucionRepository).PosponerPublicacion(ctx context.Context, id uuid.UUID) error`

### `internal/scraping/internal/repositories/entrega_repository.go` (package repositories)

- Structs: entregaRepository
- `NewEntregaRepository(db *gorm.DB) domain.EntregaRepository`
- `(entregaRepository).Obtener(ctx context.Context, id uuid.UUID) (*domain.Entrega, error)`
- `(entregaRepository).Registrar(ctx context.Context, e *domain.Entrega) (*domain.Entrega, error)`
- `(entregaRepository).Adquirir(ctx context.Context, id *uuid.UUID) (*domain.Entrega, error)`
- `(entregaRepository).FijarSucursal(ctx context.Context, id uuid.UUID, branch int) (int, error)`
- `(entregaRepository).Confirmar(ctx context.Context, e *domain.Entrega, result domain.IngestaResultadoDTO) error`
- `(entregaRepository).Reprogramar(ctx context.Context, e *domain.Entrega, code string, permanent bool) error`

### `internal/scraping/internal/repositories/idempotencia_test.go` (package repositories)

- `TestBatchKeyIsSentWithoutJobIdentityAndConflictIsNotRetried(t *testing.T)`
- `TestCatalogDataRejectionIsDistinctFromInfrastructureFailure(t *testing.T)`

### `internal/scraping/internal/repositories/job_publisher.go` (package repositories)

- Structs: jobPublisher
- `NewJobPublisher(rdb *redis.Client, db *gorm.DB) *jobPublisher`
- `(jobPublisher).Close() error`
- `(jobPublisher).Publicar(ctx context.Context, e domain.Ejecucion) error`

### `internal/scraping/internal/repositories/job_publisher_test.go` (package repositories)

- `TestPublisherStalledRedisAllowsJobFinalization(t *testing.T)`

### `internal/scraping/internal/repositories/trabajo_repository.go` (package repositories)

- Structs: gormTrabajoRepository
- `NewTrabajoRepository(db *gorm.DB) domain.TrabajoRepository`
- `(gormTrabajoRepository).CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error`
- `(gormTrabajoRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error)`
- `(gormTrabajoRepository).IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error`
- `(gormTrabajoRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*domain.TrabajoScraper, error)`

### `internal/scraping/internal/routes/scraper_routes.go` (package routes)

- `RegistrarRutasScraper(rg *gin.RouterGroup, scraperHandler *handlers.ScraperHandler)`

### `internal/scraping/internal/services/ejecuciones.go` (package services)

- Structs: QueueService
- `NewQueueService(jobs domain.EjecucionRepository, publisher domain.JobPublisher) *QueueService`
- `(QueueService).Encolar(ctx context.Context, id string, payload []byte) error`
- `(QueueService).Recuperar(ctx context.Context)`

### `internal/scraping/internal/services/entregas.go` (package services)

- `(scraperService).ingestarDurable(ctx context.Context, trabajoID *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).entregar(ctx context.Context, e *domain.Entrega) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).Recuperar(ctx context.Context)`

### `internal/scraping/internal/services/scraper_service.go` (package services)

- Structs: scraperService
- `NewScraperService(trabajos domain.TrabajoRepository, ingesta domain.IngestaRepository, entregas ...domain.EntregaRepository) ScraperService`
- `(scraperService).IniciarTrabajo(ctx context.Context, input domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).FinalizarTrabajo(ctx context.Context, id string, input domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).ObtenerTrabajo(ctx context.Context, id string) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).IngestarProductos(ctx context.Context, trabajoIDStr *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).mapearTrabajoDTO(t *domain.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO`
- `(scraperService).nombreTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) string`
- `(scraperService).nombreCadena(ctx context.Context, cadenaID int) string`

### `internal/scraping/module.go` (package scraping)

- Structs: Module
- `New(db *gorm.DB, rdb *redis.Client) (*Module, error)`
- `(Module).VerifyStorage(ctx context.Context) error`
- `(Module).Start(ctx context.Context)`
- `(Module).RegisterRoutes(group *gin.RouterGroup)`

### `internal/scraping/tests/ingesta_failures_test.go` (package tests)

- Structs: failingCounter
- `(failingCounter).IncrementarElementos(ctx context.Context, id uuid.UUID, n int) error`
- `TestCatalogFailureDoesNotIncrementJob(t *testing.T)`
- `TestCommittedCatalogBatchAndFailedCounterIsExplicit(t *testing.T)`

### `internal/scraping/tests/queue_test.go` (package tests)

- Structs: sqlQueue
- `(sqlQueue).RegistrarEjecucion(_ context.Context, e domain.Ejecucion) error`
- `(sqlQueue).Pendientes(context.Context) ([]domain.Ejecucion, error)`
- `(sqlQueue).PosponerPublicacion(context.Context, uuid.UUID) error`
- `TestExecutionWithoutRedisClientStillPersistsSQL(t *testing.T)`

### `internal/scraping/tests/scraper_test.go` (package tests)

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

### `main.go` (package main)

- `initDB(ctx context.Context) error`
- `initRedis()`
- `setupRouter() (*gin.Engine, error)`
- `setupApplication() (*gin.Engine, *scraping.Module, error)`
- `RootHandler(c *gin.Context)`
- `HealthHandler(c *gin.Context)`
- `LivenessHandler(c *gin.Context)`
- `main()`

### `main_test.go` (package main)

- `TestModuleCompositionHTTP(t *testing.T)`

### `middleware/error_handler.go` (package middleware)

- Structs: RespuestaError
- `ErrorHandler() gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`

### `swagger_test.go` (package main)

- `TestSwaggerOnlyDocumentsRegisteredRoutes(t *testing.T)`

### `tests/architecture_test.go` (package tests)

- `TestModuleDependencies(t *testing.T)`

### `tests/middleware_test.go` (package tests)

- `init()`
- `setupTestRouter() *gin.Engine`
- `TestNotFoundHandler(t *testing.T)`
- `TestMethodNotAllowedHandler(t *testing.T)`
- `TestPanicRecoveryHandler(t *testing.T)`
- `TestResponderError(t *testing.T)`

### `utils/security.go` (package utils)

- `SanitizarInputBusqueda(input string) string`
