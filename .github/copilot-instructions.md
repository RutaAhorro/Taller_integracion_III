# Proyecto: Plataforma de Comparación de Precios y Optimización de Rutas

Stack: React 19 + Go 1.26 (Gin/GORM) + Python 3.11 (FastAPI) + PostgreSQL 15/PostGIS + Docker
Equipo: 6 integrantes, GitFlow estricto, Scrum semestral (UCT)

## Arquitectura Híbrida (Clúster + Local)

```text
=== Servidor Local (Pentium) ===
frontend/web/       → React 19 + Vite + TypeScript (Accesible vía Zapto)
backend/bot_discord/→ Go Discord Bot (Se conecta a la API K8s)

=== Clúster Kubernetes UCT ===
backend/api/        → Go 1.26 + Gin + GORM (API Gateway, expuesto vía Ingress/Port-Forward)
backend/scraper/    → Python + Scrapy (Standby worker)
PostgreSQL / Redis  → Pods dedicados en el clúster
```

Servicios de infraestructura: Kubernetes (UCT), PostgreSQL 15+PostGIS, Redis (broker)

## Base de Datos (3 esquemas PostgreSQL)

- `api.*`: usuarios, categorías, marcas, productos_normalizados, recetas, misiones_validacion, mapeos_productos_ia
- `scraper.*`: cadenas_supermercado, sucursales (geolocalización), trabajos_scraper, productos_crudos, capturas_precios
- `rutas.*`: listas_compras, articulos_lista_compras, ejecuciones_optimizacion, paradas_optimizacion, detalle_articulos_parada
- Modelos GORM: `backend/api/infrastructure/models.go` (23 modelos)
- DDL SQL: `infrastructure/db/` (init.sql, 01_schema_api.sql, 02_schema_scraper.sql, 03_schema_rutas.sql)

## Convenciones de Git (OBLIGATORIO)

- Ramas: `[área]/[tipo]/[tarea]-[nombre_dev]`
  - Áreas: frontend, backend, scraper, ia, docker, docs
  - Tipos: feat, fix, docs, refactor
  - Ejemplo: `frontend/feat/login-vicente`, `ia/feat/investigacion-alucinaciones`
- Commits: formato convencional (`feat: ...`, `fix: ...`, `docs: ...`, `refactor: ...`)
- Merge: siempre Squash and Merge hacia develop
- NUNCA hacer push directo a main o develop
- Eliminar rama en GitHub y local tras el merge

## Frontend: Reglas

- Componentes activos usan **PascalCase**: ProductCard.tsx, Sidebar.tsx, TopNav.tsx
- **IGNORAR archivos kebab-case** (product-card.tsx, side-bar.tsx): son versiones v0 obsoletas, no se usan
- UI primitives: src/components/ui/ (button, input, field, label, separator, tabs)
- State management: useState + useOutletContext (sin Redux/Zustand)
- Datos mock actuales: src/data/mock.ts (sin conexión a API real aún)
- Path alias: `@` → `src/`
- Linter: oxlint (no eslint)

## Backend Go: Reglas

- Framework: Gin con middleware CORS global
- ORM: GORM con driver pgx/v5
- Sanitización: `utils.SanitizarInputBusqueda()` para toda entrada de usuario
- Swagger: anotaciones swag en handlers, auto-generado en docs/
- Variable global `DB *gorm.DB` en main.go

## Python: Patrón Anti-Alucinación (3 Etapas)

1. LLM extrae filtros estructurados via function calling (NO texto libre)
2. Python filtra catálogo deterministamente en filtros.py (NUNCA el LLM)
3. LLM genera explicación SOLO con datos validados de Etapa 2
- Proveedores intercambiables: GroqProveedor, GeminiProveedor, FallbackProveedor
- Interfaz base: ProveedorIA(ABC) en app/proveedores/base.py

## Hardware y Docker

- Servidor: Intel Pentium (recursos limitados)
- Imágenes Docker: SOLO -alpine y -slim
- Respetar límites cgroups en docker-compose.yml
- No usar librerías pesadas sin justificación (OSMnx fue eliminado por OOM)
- Python: PYTHONDONTWRITEBYTECODE=1, pip --no-cache-dir

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Resumen de arquitectura (auto-generado)


## Mapa auto-generado: Catálogo y precios (Go + Gin)

Estado vigente SUP-271: Catálogo usa su credencial app_catalogo (DB_URL) y sólo cadenas, sucursales, productos, capturas y recibos propios. Valida permisos una vez al arrancar (timeout 5s); readiness sólo hace Ping y liveness no consulta SQL. Ingesta privada con CATALOGO_INTERNAL_TOKEN e idempotencia SUP-270. No accede a usuarios, trabajos ni outboxes de coordinación. Ver docs/Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md.

**27 archivos .go** detectados


### `cmd/catalogo/main.go` (package main)

- `main()`
- `run(log *slog.Logger) error`

### `cmd/seed_productos/main.go` (package main)

- Structs: productoSemilla
- `main()`

### `cmd/seed_productos/models.go` (package main)

- Structs: productoCrudoSemilla, capturaPrecioSemilla

### `internal/auth/jwt.go` (package auth)

- Structs: JWTClaims
- `ValidarToken(tokenString, secret string) (*jwt.Token, error)`

### `internal/config/config.go` (package config)

- Structs: Config
- `Load() (Config, error)`

### `internal/config/config_test.go` (package config)

- `TestLoad(t *testing.T)`
- `TestCORSOriginsConfiguration(t *testing.T)`
- `TestInternalTokenConfiguration(t *testing.T)`

### `internal/domain/producto.go` (package domain)

- Structs: FiltroProductosDTO, ProductoDTO, ProductoAdminDTO, MetadatosPaginacionDTO, PaginaProductosDTO, PaginaProductosAdminDTO, HistorialPrecioDTO, ProductoDetalleDTO

### `internal/domain/repository.go` (package domain)


### `internal/handlers/producto_handler.go` (package handlers)

- Structs: ProductoHandler
- `NewProductoHandler(service services.ProductoService) *ProductoHandler`
- `(ProductoHandler).ObtenerProductos(c *gin.Context)`
- `(ProductoHandler).ObtenerProductosAdmin(c *gin.Context)`
- `(ProductoHandler).BuscarProductos(c *gin.Context)`
- `(ProductoHandler).ObtenerDetalleProducto(c *gin.Context)`

### `internal/ingesta/http.go` (package ingesta)

- Structs: Config
- `Register(r *gin.Engine, cfg Config, logger *slog.Logger)`
- `decode(c *gin.Context, input any) bool`

### `internal/ingesta/http_test.go` (package ingesta)

- Structs: testStore
- `TestInternalSQLDataErrorsArePermanentAndPrivate(t *testing.T)`
- `(testStore).ObtenerCadena(context.Context, int, string) (*CadenaSupermercado, error)`
- `(testStore).ObtenerSucursal(context.Context, int, *int, *string) (*SucursalSupermercado, error)`
- `(testStore).IngestarLote(_ context.Context, input Request) (*IngestaResultadoDTO, error)`
- `TestInternalIngestSecurityAndValidation(t *testing.T)`
- `TestInternalBatchConflictIsControlled(t *testing.T)`
- `TestInternalIngestClosedAndGenericError(t *testing.T)`

### `internal/ingesta/lote.go` (package ingesta)

- `(Repository).IngestarLote(ctx context.Context, input Request) (*IngestaResultadoDTO, error)`

### `internal/ingesta/repository.go` (package ingesta)

- Structs: Repository
- `NewRepository(db *gorm.DB) *Repository`
- `(Repository).obtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*CadenaSupermercado, error)`
- `(Repository).obtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*SucursalSupermercado, error)`
- `(Repository).ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*CadenaSupermercado, error)`
- `(Repository).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigo *string) (*SucursalSupermercado, error)`

### `internal/ingesta/storage.go` (package ingesta)

- `VerifyStorage(ctx context.Context, db *gorm.DB, writer bool) error`

### `internal/ingesta/types.go` (package ingesta)

- Structs: CadenaSupermercado, SucursalSupermercado, ProductoScrapeadoDTO, IngestaResultadoDTO, Request

### `internal/middleware/error_handler.go` (package middleware)

- Structs: RespuestaError, ErrorValidacion
- `(ErrorValidacion).Error() string`
- `ErrorHandler(logger *slog.Logger) gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`
- `LogInternalError(c *gin.Context, err error)`

### `internal/middleware/jwt_auth.go` (package middleware)

- `RequireAuth(secret string) gin.HandlerFunc`

### `internal/middleware/rbac.go` (package middleware)

- `RequireRole(rolesPermitidos ...string) gin.HandlerFunc`

### `internal/repositories/producto_repository.go` (package repositories)

- Structs: gormProductoRepository
- `NewProductoRepository(db *gorm.DB) domain.ProductoRepository`
- `(gormProductoRepository).Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error)`
- `(gormProductoRepository).ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error)`
- `(gormProductoRepository).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `internal/server/server.go` (package server)

- `NewRouter(service services.ProductoService, secret string, ping func(context.Context) error, logger *slog.Logger, allowedOrigins []string, internal ...ingesta.Config) *gin.Engine`
- `cors(allowedOrigins []string) gin.HandlerFunc`
- `observe(logger *slog.Logger) gin.HandlerFunc`

### `internal/services/producto_service.go` (package services)

- Structs: productoService
- `NewProductoService(repo domain.ProductoRepository) ProductoService`
- `(productoService).ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error)`
- `(productoService).ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error)`
- `(productoService).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `internal/tests/architecture_test.go` (package tests)

- `TestCatalogDependencies(t *testing.T)`

### `internal/tests/producto_handler_test.go` (package tests)

- `TestProductoHandler_ObtenerProductos(t *testing.T)`
- `TestProductoHandler_BuscarProductos(t *testing.T)`
- `TestProductoHandler_ObtenerDetalleProducto(t *testing.T)`
- `TestProductoHandler_ObtenerProductosAdmin(t *testing.T)`

### `internal/tests/producto_service_test.go` (package tests)

- Structs: mockProductoRepository
- `(mockProductoRepository).Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error)`
- `(mockProductoRepository).ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error)`
- `(mockProductoRepository).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`
- `TestProductoService_ValoresPorDefecto(t *testing.T)`
- `TestProductoService_SanitizacionYLimites(t *testing.T)`
- `TestProductoService_Busqueda_Minimo3Caracteres(t *testing.T)`
- `TestProductoService_ObtenerPorID(t *testing.T)`
- `TestProductoService_ObtenerCatalogoAdmin(t *testing.T)`

### `internal/tests/security_test.go` (package tests)

- `TestCatalogCORS(t *testing.T)`
- `TestCatalogInternalErrorsArePrivate(t *testing.T)`
- `TestOnlyControlledValidationDetailsArePublic(t *testing.T)`

### `internal/tests/server_test.go` (package tests)

- `catalogRouter(repo *mockProductoRepository, ping func(context.Context) error) *gin.Engine`
- `signedToken(t *testing.T, role string, method jwt.SigningMethod, key string, expires *time.Time) string`
- `TestCatalogAdministrationAuthorization(t *testing.T)`
- `TestCatalogHealthAndDomainIsolation(t *testing.T)`
- `TestCatalogRequestCorrelation(t *testing.T)`

### `internal/utils/security.go` (package utils)

- `SanitizarInputBusqueda(input string) string`


## Mapa auto-generado: Control de scraping independiente (Go + Gin)

SUP-273: dueño exclusivo de trabajos y outboxes; SCRAPING_DB_URL/app_scraping, pool 5. /_scraping/live independiente y /_scraping/ready SQL con Redis diagnóstico. Conserva recuperación SUP-270/271, ingesta HTTP idempotente a Catálogo y contrato worker. Sin JWT_SECRET, modelos de usuarios ni persistencia de catálogo. Gateway usa SCRAPING_URL; no existe servicio backend/api. Ver docs/Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md.

**36 archivos .go** detectados


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

### `cmd/control_scraping/service_auth_test.go` (package main)

- `TestScraperRequiresServiceToken(t *testing.T)`
- `TestServiceTokenConfigurationFailsClosed(t *testing.T)`

### `cmd/control_scraping/swagger_test.go` (package main)

- `TestSwaggerOnlyDocumentsRegisteredRoutes(t *testing.T)`
- `TestSwaggerDisabledByDefault(t *testing.T)`

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

### `internal/middleware/service_auth.go` (package middleware)

- `ValidarTokenServicio(token, catalogoToken string) error`
- `RequireServiceToken(token string) gin.HandlerFunc`

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

- `RegistrarRutasScraper(rg *gin.RouterGroup, scraperHandler *handlers.ScraperHandler, auth ...gin.HandlerFunc)`

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


## Mapa auto-generado: Identidad (Go + Gin)

SUP-272: proceso independiente dueño de api.usuarios, sólo IDENTITY_DB_URL/app_identity (pool 5), Redis para límite login y JWT_SECRET compartido por contrato con Catálogo. Cinco rutas /api/v1/auth/* conservadas. CORS allowlist, cookies Secure configurables, HS256 estricto y sin fallback de clave. Proxy Gateway explícito obligatorio para IP real. Storage una vez al arrancar; ready SQL/Redis y live independiente. No importa API/Catálogo/scraping. Ver docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md.

**33 archivos .go** detectados


### `cmd/identidad/main.go` (package main)

- `main()`
- `run(log *slog.Logger) error`

### `internal/config/config.go` (package config)

- Structs: Config
- `Load() (Config, error)`

### `internal/config/config_test.go` (package config)

- `valid(t *testing.T)`
- `TestValidDefaults(t *testing.T)`
- `TestInvalidConfigFailsWithoutSecrets(t *testing.T)`
- `TestExplicitLocalAndProxyConfig(t *testing.T)`

### `internal/database/database.go` (package database)

- `Open(ctx context.Context, dsn string) (*gorm.DB, error)`

### `internal/domain/models.go` (package domain)

- Structs: Usuario

### `internal/domain/repository.go` (package domain)


### `internal/domain/usuario.go` (package domain)

- Structs: ActualizarPerfilDTO, PerfilUsuarioDTO

### `internal/handlers/auth_handler.go` (package handlers)

- Structs: RegistroRequest, RegistroResponse, LoginRequest, LoginResponse, AuthHandler, GoogleLoginRequest
- `NewAuthHandler(authService services.AuthService, secureCookie ...bool) *AuthHandler`
- `(AuthHandler).RegistrarUsuario(c *gin.Context)`
- `(AuthHandler).LoginUsuario(c *gin.Context)`
- `(AuthHandler).PerfilUsuario(c *gin.Context)`
- `(AuthHandler).ActualizarPerfil(c *gin.Context)`
- `(AuthHandler).GoogleLoginUsuario(c *gin.Context)`

### `internal/handlers/google_errors_test.go` (package handlers)

- Structs: googleErrorService
- `(googleErrorService).GoogleLogin(context.Context, string) (*domain.Usuario, error)`
- `TestGoogleErrorClassificationDoesNotExposeStorage(t *testing.T)`

### `internal/httperrors/error_handler.go` (package httperrors)

- Structs: RespuestaError
- `ErrorHandler() gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`

### `internal/identity/module.go` (package identity)

- Structs: Module
- `New(db *gorm.DB, rdb *redis.Client, secureCookie ...bool) *Module`
- `(Module).RegisterRoutes(group *gin.RouterGroup)`
- `(Module).RequireAuth() gin.HandlerFunc`
- `(Module).RequireRole(roles ...string) gin.HandlerFunc`
- `VerifyStorage(ctx context.Context, db *gorm.DB) error`

### `internal/middleware/jwt_auth.go` (package middleware)

- `RequireAuth() gin.HandlerFunc`

### `internal/middleware/rate_limit.go` (package middleware)

- `RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc`
- `rateLimiter(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration, soloFallidos bool) gin.HandlerFunc`
- `RateLimitLogin(rdb *redis.Client) gin.HandlerFunc`
- `RateLimitRegistro(rdb *redis.Client) gin.HandlerFunc`
- `RateLimitGoogle(rdb *redis.Client) gin.HandlerFunc`

### `internal/middleware/rbac.go` (package middleware)

- `RequireRole(rolesPermitidos ...string) gin.HandlerFunc`

### `internal/repositories/usuario_repository.go` (package repositories)

- Structs: gormUsuarioRepository
- `NewUsuarioRepository(db *gorm.DB) domain.UsuarioRepository`
- `(gormUsuarioRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).FindByID(ctx context.Context, id string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).Create(ctx context.Context, usuario *domain.Usuario) error`
- `(gormUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`
- `classifyWriteError(err error) error`

### `internal/routes/auth_routes.go` (package routes)

- `RegistrarRutasAuth(rg *gin.RouterGroup, authHandler *handlers.AuthHandler, rdb *redis.Client)`

### `internal/security/jwt.go` (package security)

- Structs: JWTClaims
- `getJWTSecret() []byte`
- `GenerarToken(usuarioID, rol, provider string) (string, error)`
- `ValidarToken(tokenString string) (*jwt.Token, error)`

### `internal/security/password.go` (package security)

- `HashPassword(password string) (string, error)`
- `CheckPasswordHash(password, hash string) bool`

### `internal/server/server.go` (package server)

- `NewRouter(cfg config.Config, db *gorm.DB, redisClient *redis.Client, ping func(context.Context) error, log *slog.Logger) *gin.Engine`
- `cors(origins []string) gin.HandlerFunc`
- `observe(log *slog.Logger) gin.HandlerFunc`
- `boundedRequest() gin.HandlerFunc`

### `internal/server/server_test.go` (package server)

- `TestHTTPBoundariesCorsAndJWT(t *testing.T)`
- `TestRecoveryDoesNotExposePanic(t *testing.T)`
- `TestReadinessDeadlineAndLiveness(t *testing.T)`
- `TestUnclassifiedContextErrorDoesNotExposeSQL(t *testing.T)`

### `internal/services/auth_service.go` (package services)

- Structs: RegistroDTO, UsuarioCreadoDTO, authService
- `NewAuthService(usuarioRepo domain.UsuarioRepository) AuthService`
- `validarComplejidadPassword(password string) error`
- `validarEmail(email string) bool`
- `(authService).Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error)`
- `(authService).Login(correo, password string) (*domain.Usuario, error)`
- `(authService).LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error)`
- `(authService).GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error)`
- `(authService).ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)`
- `NewAuthServiceWithGoogleValidator(repo domain.UsuarioRepository, validate func(context.Context, string, string) (*idtoken.Payload, error)) AuthService`
- `(authService).validateGoogleAccount(ctx context.Context, user *domain.Usuario, subject string) (*domain.Usuario, error)`

### `internal/tests/architecture_test.go` (package tests)

- `TestIdentityDoesNotDependOnOtherServices(t *testing.T)`

### `internal/tests/auth_handler_test.go` (package tests)

- `TestAuthHandler_RegistrarUsuario_AutoLogin(t *testing.T)`
- `TestAuthHandler_ActualizarPerfil(t *testing.T)`

### `internal/tests/auth_service_test.go` (package tests)

- Structs: mockUsuarioRepository
- `newMockUsuarioRepository() *mockUsuarioRepository`
- `(mockUsuarioRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(mockUsuarioRepository).FindByID(ctx context.Context, id string) (*domain.Usuario, error)`
- `(mockUsuarioRepository).Create(ctx context.Context, usuario *domain.Usuario) error`
- `(mockUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`
- `TestAuthService_PasswordComplexity(t *testing.T)`
- `TestAuthService_RegistroExitoso(t *testing.T)`
- `TestAuthService_Login(t *testing.T)`
- `TestAuthService_ActualizarPerfil(t *testing.T)`

### `internal/tests/google_crypto_test.go` (package tests)

- Structs: jwksFixture
- `(jwksFixture).RoundTrip(r *http.Request) (*http.Response, error)`
- `TestGoogleCryptographicValidation(t *testing.T)`

### `internal/tests/hardening_test.go` (package tests)

- Structs: googleRaceRepository
- `TestJWTRejectsUnsafeTokens(t *testing.T)`
- `TestGoogleClaimsAndInactiveAccounts(t *testing.T)`
- `TestPasswordBeyondBcryptLimitIsValidationError(t *testing.T)`
- `(googleRaceRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(googleRaceRepository).Create(ctx context.Context, user *domain.Usuario) error`
- `TestGoogleConcurrentCreationRechecksAccount(t *testing.T)`

### `internal/tests/jwt_fuzz_test.go` (package tests)

- `FuzzJWTParser(f *testing.F)`

### `internal/tests/jwt_test.go` (package tests)

- `TestJWT_GenerarYValidarToken(t *testing.T)`
- `TestJWT_TokenInvalido(t *testing.T)`
- `TestMiddleware_RequireAuth(t *testing.T)`
- `init()`

### `internal/tests/main_test.go` (package tests)

- `TestMain(m *testing.M)`

### `internal/tests/rbac_test.go` (package tests)

- `TestRequireRole(t *testing.T)`

### `internal/tests/review_followups_test.go` (package tests)

- `TestActualizarPerfilExigeContrasenaActual(t *testing.T)`
- `googlePayload(subject, email string) *idtoken.Payload`
- `TestGoogleVinculaCuentaLocalEInvalidaContrasena(t *testing.T)`
- `TestGoogleRechazaOtraIdentidadVinculada(t *testing.T)`
- `TestRateLimitLoginSoloCuentaFallidos(t *testing.T)`

### `internal/tests/review_regressions_test.go` (package tests)

- `TestProfileNameValidationPreservesStoredName(t *testing.T)`
- `TestNameSanitizationPreservesUnicodeBoundaries(t *testing.T)`
- `TestGoogleAvatarFitsPostgresWithoutBreakingLogin(t *testing.T)`

### `internal/utils/security.go` (package utils)

- `SanitizarInputBusqueda(input string) string`


## Mapa auto-generado: IA Conversacional (FastAPI)

**20 archivos .py** detectados


### `app/auth.py`

- Imports internos: app
- **class UsuarioAutenticado(BaseModel)**
- `async obtener_usuario_actual(authorization: ?) → UsuarioAutenticado`

### `app/config.py`


### `app/explicacion.py`

- `construir_prompt_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`
- `async generar_explicacion_con_tiempo(generador, mensaje_usuario: str, productos_filtrados: list)`

### `app/filtros.py`

- `sanitizar_filtros_crudos(crudo: dict) → dict`
- `rescatar_filtros_de_error_groq(response: httpx.Response) → ?`
- `_extraer_json_de_texto(texto: str) → ?`
- `_tokenizar_categoria(texto: str) → list`
- `_texto_contiene(a: str, b: str) → bool`
- `_coincide_por_texto(frase: str, producto: dict) → bool`
- `filtrar_productos(catalogo: list, filtros: dict) → list`

### `app/ia_definiciones.py`


### `app/main.py`

- Imports internos: app, app.ia_definiciones, app.productos, app.proveedores.fallback_proveedor, app.proveedores.gemini_proveedor, app.proveedores.groq_proveedor, app.routers.chat, app.routers.productos, app.utils
- `health_check()`

### `app/productos.py`

- **class ProductoRepositorio**
  - `__init__(ruta_json: Path)`
  - `_cargar() → list`
  - `obtener_todos() → list`
  - `__len__() → int`

### `app/proveedores/base.py`

- **class ResultadoExtraccion**
- **class ProveedorIA(ABC)**
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/fallback_proveedor.py`

- Imports internos: app.proveedores.base
- **class FallbackProveedor(ProveedorIA)**
  - `__init__(proveedores: ?, max_retries_primario: int, max_retries_secundario: int, backoff_base: float, timeout: float)`
  - `_es_error_recuperable(error: Exception) → bool`
  - `async _ejecutar_con_reintentos(proveedor: ProveedorIA, metodo: str) → any`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/gemini_proveedor.py`

- Imports internos: app.explicacion, app.filtros, app.proveedores.base
- **class GeminiProveedor(ProveedorIA)**
  - `__init__(api_key: ?, endpoint: str, modelo: str, definicion_funcion: dict)`
  - `_headers() → dict`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/groq_proveedor.py`

- Imports internos: app.explicacion, app.filtros, app.proveedores.base
- **class GroqProveedor(ProveedorIA)**
  - `__init__(api_key: ?, endpoint: str, modelo_extraccion: str, modelo_explicacion: str, definicion_funcion: dict)`
  - `_headers() → dict`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/routers/chat.py`

- Imports internos: app.auth, app.explicacion, app.filtros, app.productos, app.proveedores.base, app.schemas
- `crear_router_chat(repositorio: ProductoRepositorio, proveedor_fallback: ProveedorIA, proveedor_groq: ?, proveedor_gemini: ?) → APIRouter`

### `app/routers/productos.py`

- Imports internos: app.productos
- `crear_router_productos(repositorio: ProductoRepositorio) → APIRouter`

### `app/schemas.py`

- **class ChatRequest(BaseModel)**

### `app/utils.py`

- **class RespuestaUTF8(JSONResponse)**
- `fix_encoding_consola() → ?`

### `tests/conftest.py`


### `tests/test_chat_auth.py`

- Imports internos: app, app.main, app.proveedores.base
- `configurar_jwt_secret(monkeypatch)`
- `client()`
- `generar_token(user_id: str, rol: str, exp_delta: timedelta, secret: str, algorithm: str, extra_claims: dict) → str`
- `test_chat_sin_token_retorna_401(client)`
- `test_chat_header_malformado_retorna_401(client)`
- `test_chat_token_invalido_retorna_401(client)`
- `test_chat_token_firma_invalida_retorna_401(client)`
- `test_chat_token_expirado_retorna_401(client)`
- `test_chat_token_sin_user_id_retorna_401(client)`
- `test_chat_token_valido_retorna_200_con_pipeline_tres_etapas(client)`
- `test_chat_secret_no_configurado_retorna_401(client, monkeypatch)`
- `test_chat_texto_libre_sin_filtros(client)`
- `test_chat_error_en_proveedor(client)`
- `test_cors_configuracion_origenes(client)`


## Mapa auto-generado: Motor de Rutas (FastAPI + OR-Tools)

**5 archivos .py** detectados


### `app/algorithms/otp_ortools_strategy.py`

- Imports internos: app.core.strategy
- **class OTPOrToolsStrategy(RoutingStrategy)**
  - `__init__(otp_base_url: str)`
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/algorithms/prototipo_ruta_ficticia.py`

- `haversine(lat1: float, lon1: float, lat2: float, lon2: float) → float`
- `crear_coordenadas() → ?`
- `calcular_matriz_distancias(coordenadas: ?) → ?`
- `resolver_tsp(matriz_distancias: np.ndarray) → ?`
- `mostrar_resultados(orden: ?, nombres: ?, matriz_distancias: np.ndarray) → ?`
- `main() → ?`

### `app/core/strategy.py`

- **class RoutingStrategy(ABC)**
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/main.py`

- `root()`
- `health_check()`

### `tests/test_prototipo_ruta_ficticia.py`

- `ejecutar_demo()`


## Mapa auto-generado: Scraper (Scrapy)

Estado vigente SUP-270: worker con Redis Streams, lease/heartbeat/ACK y dead-letter; spool SQLite durable en SCRAPER_STATE_DIR. No accede a PostgreSQL. Un worker con volumen persistente RWO; conservar las claves de lote de trabajos activos. Limpia spools completados tras ACK; retiene fallidos 24h, con retirada terminal bajo presión y reserva de 64MiB. Ver docs/Guias/GUIA_SCRAPING_RECUPERACION.md.

**31 archivos .py** detectados


### `research/recovery_fixture.py`

- **class RecoveryFixture(Spider)**
  - `parse(response)`

### `research/smoke_recovery.py`

- `compose()`
- `sql(query)`
- `scraper_auth(path)`
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
  - `test_client_sends_service_token_on_every_call()`
  - `test_client_reads_token_from_environment()`
  - `test_client_without_token_sends_no_authorization()`
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
  - `test_legacy_queue_migration_sends_service_token()`
  - `test_rejected_credential_keeps_job_pending_and_logs_error()`
  - `test_worker_refuses_to_start_without_service_token()`
  - `test_legacy_api_acceptance_does_not_prove_durable_queue()`
  - `test_finalize_failed_deliveries_are_permanent_not_pending()`
  - `test_terminal_job_can_be_acknowledged_without_opening_full_sqlite()`
  - `test_completed_spool_deleted_only_after_successful_ack()`
  - `test_gc_recovers_crash_after_ack_but_preserves_pending_job()`
  - `test_gc_never_deletes_unacked_or_locked_spool()`
  - `test_failed_spool_retention_and_pressure_cleanup()`
  - `test_full_disk_does_not_delete_active_items_or_accept_new_ones()`
  - `test_gc_dependency_failure_preserves_spool()`
  - `test_sqlite_full_rolls_back_without_ack_or_losing_saved_items()`
  - `test_legacy_duplicates_remove_tail_and_keep_fifo_order()`
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
- **class Redis**
  - `lindex()`
  - `lrem()`
- **class Redis**
  - `lindex(key, index)`
  - `lrem(key, count, value)`

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
  - `__init__(endpoint: str, timeout: float, opener, token: ?)`
  - `_headers(content_type: bool) → dict`
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
  - `_initialize()`
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

### `scraper_core/storage.py`

- **class StoragePressure(OSError)**
- **class SpoolCollector**
  - `__init__(state_dir)`
  - `collect(api_client, queue, limit)`
- `reserve_bytes()`
- `ensure_capacity(directory, extra_bytes)`
- `cleanup_spool(state_dir, work_id)`

### `scraper_core/worker.py`

- **class CrawlExhausted(RuntimeError)**
- **class ReliableQueue**
  - `__init__(client, consumer, lease_ms)`
  - `ensure_group()`
  - `receive()`
  - `renew(message_id)`
  - `ack(message_id, raw_job, work_id, reason)`
  - `has_message(work_id)`
  - `heartbeat(message_id)`
- `job_environment(work_id)`
- `process_job(job: dict, api_client: ScraperAPIClient, cancelled) → dict`
- `handle_message(queue, message, api_client)`
- `run_worker()`


## Mapa auto-generado: Frontend (React + TypeScript)

**82 archivos activos** (excluidos kebab-case obsoletos)


### `core/routes.ts`

- Exports: MOCK_ROUTES, ROUTES

### `data/mock.ts`

- Exports: HOME, discountPct, formatPrice, mockUser, products, supermarketById, supermarkets
- Imports locales: ../types

### `features/admin/admin-api.ts`

- Exports: ADMIN_ENDPOINTS, ADMIN_HAS_BACKEND, adminPlanFromBackend, adminPlanToBackend, adminRoleFromBackend, adminRoleToBackend, fetchAdminSources, fetchAdminUsers, fetchCatalogoMetrics, fetchControlMetrics, fetchIdentidadMetrics, isAdminAuthError, updateAdminSource, updateAdminUser
- Tipos: CatalogoMetrics, ControlMetrics, IdentidadMetrics
- Imports locales: ./admin-mock-data

### `features/admin/admin-mock-data.ts`

- Exports: ADMIN_PLANS, ADMIN_ROLES, adminMetricsMock, adminSources, adminUsers
- Tipos: AdminPlan, AdminRole, AdminSection, AdminSource, AdminUser

### `features/admin/useAdminData.ts`

- Exports: useAdminMetrics, useAdminSources, useAdminUsers
- Tipos: AdminMetricsState, AdminPlan, AdminRole, AdminSource, AdminUser, CatalogoMetrics, ControlMetrics, IdentidadMetrics, ServiceState
- Imports locales: ./admin-api, ./admin-mock-data

### `hooks/useAuth.ts`

- Exports: useAuth
- Imports locales: ../contexts/AuthContext

### `hooks/useCatalog.ts`

- Exports: useCatalog
- Imports locales: @/layouts/MainLayout, @/lib/formatters, @/lib/products-api, @/types

### `hooks/useChatbot.ts`

- Exports: useChatbot
- Tipos: Message, MessageType
- Imports locales: @/hooks/useAuth

### `hooks/useGoogleAuth.ts`

- Exports: useGoogleAuth
- Imports locales: ./useAuth

### `hooks/useRouteOptimization.ts`

- Exports: useRouteOptimization
- Tipos: OptimizedStore
- Imports locales: @/data/mock

### `layouts/useActiveTab.ts`

- Exports: useActiveTab
- Tipos: ActiveTab

### `lib/constants.ts`

- Exports: HOME, supermarketById, supermarkets
- Imports locales: ../types

### `lib/data.ts`

- Exports: HOME, discountPct, formatPrice, products, supermarketById, supermarkets
- Tipos: Product, Supermarket

### `lib/formatters.ts`

- Exports: calculateDiscountPct, calculateSavings, formatPrice

### `lib/logger.ts`

- Exports: logger

### `lib/products-api.ts`

- Exports: FALLBACK_MARKET_NAME, marketMeta, slugifyMarket, toUiProduct
- Tipos: ApiPriceHistory, ApiProduct, ApiProductDetail, ProductPage
- Imports locales: ./constants, @/types

### `lib/utils.ts`

- Exports: cn
- Tipos: ClassValue

### `types/index.ts`

- Tipos: CadenaSupermercado, CapturaPrecio, Categoria, LoginPayload, Marca, PreferenciasDieteticas, ProductoCrudo, ProductoNormalizado, RegisterPayload, SucursalSupermercado, UiProduct, UiSupermarket, Usuario

### `app/router.tsx`

- Exports: AppRouter
- Imports locales: @/components/ErrorBoundary, @/components/auth/ProtectedRoute, @/contexts/AuthContext, @/contexts/CartContext, @/contexts/ToastContext, @/core/routes, @/features/admin/AdminDashboard, @/layouts/MainLayout, @/pages/Chatbot, @/pages/Crowdsourcing, @/pages/Dashboard, @/pages/History, @/pages/Home, @/pages/Login, @/pages/NotFound, @/pages/Onboarding, @/pages/Planes, @/pages/ProductDetail, @/pages/Profile, @/pages/RouteViewer, @/pages/Terms, @/pages/mocks/MockShell

### `components/BottomNav.tsx`

- Exports: BottomNav
- Tipos: BottomNavProps
- Imports locales: @/lib/utils

### `components/CartSidebar.tsx`

- Exports: CartSidebar
- Imports locales: @/components/ui/button, @/contexts/CartContext, @/lib/formatters

### `components/ErrorBoundary.tsx`

- Exports: ErrorBoundary
- Imports locales: @/lib/logger

### `components/Footer.tsx`

- Exports: Footer

### `components/ProductCard.tsx`

- Exports: ProductCard
- Tipos: ProductCardProps
- Imports locales: @/components/ui/button, @/core/routes, @/lib/formatters, @/lib/utils, @/types

### `components/ProductCarousel.tsx`

- Exports: ProductCarousel
- Imports locales: @/components/ProductCard, @/types

### `components/RouteMap.tsx`

- Exports: RouteMap
- Tipos: MapStop, RouteMapProps

### `components/Sidebar.tsx`

- Exports: SideBar
- Tipos: SideBarProps
- Imports locales: @/components/ui/button, @/types

### `components/TopNav.tsx`

- Exports: TopNav
- Tipos: TopNavProps
- Imports locales: @/components/ui/button, @/core/routes, @/hooks/useAuth

### `components/auth/AuthPanel.tsx`

- Exports: AuthPanel
- Imports locales: @/components/auth/PasswordStrength, @/components/auth/SocialButtons, @/components/ui/button, @/components/ui/field, @/components/ui/input, @/components/ui/tabs, @/contexts/ToastContext, @/hooks/useAuth

### `components/auth/AuthVisual.tsx`

- Exports: AuthVisual

### `components/auth/PasswordStrength.tsx`

- Exports: PasswordStrength
- Imports locales: @/lib/utils

### `components/auth/ProtectedRoute.tsx`

- Exports: ProtectedRoute
- Imports locales: @/hooks/useAuth

### `components/auth/SocialButtons.tsx`

- Exports: SocialButtons
- Imports locales: @/hooks/useGoogleAuth

### `components/chatbot/OutOfStockAlert.tsx`

- Exports: OutOfStockAlert
- Tipos: OutOfStockAlertProps
- Imports locales: @/lib/utils

### `components/chatbot/RichRecipeCard.tsx`

- Exports: RichRecipeCard
- Tipos: RecipeItem, RichRecipeCardProps
- Imports locales: @/lib/formatters, @/lib/utils

### `components/chatbot/TypingIndicator.tsx`

- Exports: TypingIndicator
- Imports locales: @/lib/utils

### `components/landing/LandingCTA.tsx`

- Exports: LandingCTA
- Imports locales: @/components/ui/button

### `components/landing/LandingFeatures.tsx`

- Exports: LandingFeatures
- Imports locales: @/data/landing.json

### `components/landing/LandingFooter.tsx`

- Exports: LandingFooter

### `components/landing/LandingHero.tsx`

- Exports: LandingHero
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingNavbar.tsx`

- Exports: LandingNavbar
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingSavings.tsx`

- Exports: LandingSavings
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingStores.tsx`

- Exports: LandingStores
- Imports locales: @/data/landing.json

### `components/landing/LandingTrustStrip.tsx`

- Exports: LandingTrustStrip
- Imports locales: @/data/landing.json

### `components/landing/LandingWorks.tsx`

- Exports: LandingHowItWorks
- Imports locales: @/data/landing.json

### `components/onboarding/OnboardingWizard.tsx`

- Exports: OnboardingWizard
- Imports locales: ./ProgressSteps, ./StepDiet, ./StepLocation, ./StepProfile, ./StepSupermarkets, @/lib/utils

### `components/onboarding/ProgressSteps.tsx`

- Exports: ProgressSteps
- Tipos: ProgressStepsProps
- Imports locales: @/lib/utils

### `components/onboarding/StepDiet.tsx`

- Exports: StepDiet
- Tipos: StepDietProps, Tag
- Imports locales: @/lib/utils

### `components/onboarding/StepLocation.tsx`

- Exports: StepLocation
- Tipos: StepLocationProps

### `components/onboarding/StepProfile.tsx`

- Exports: StepProfile
- Tipos: StepProfileProps
- Imports locales: @/lib/utils

### `components/onboarding/StepSupermarkets.tsx`

- Exports: StepSupermarkets
- Tipos: Chain, StepSupermarketsProps
- Imports locales: @/lib/utils

### `components/ui/ProductSkeleton.tsx`

- Exports: ProductSkeleton
- Imports locales: @/components/ui/skeleton

### `components/ui/SuccessCard.tsx`

- Exports: SuccessCard
- Tipos: SuccessCardProps
- Imports locales: @/lib/utils

### `components/ui/button.tsx`

- Tipos: VariantProps
- Imports locales: @/lib/utils

### `components/ui/field.tsx`

- Tipos: VariantProps
- Imports locales: @/components/ui/label, @/components/ui/separator, @/lib/utils

### `components/ui/tabs.tsx`

- Tipos: VariantProps
- Imports locales: @/lib/utils

### `contexts/AuthContext.tsx`

- Exports: AuthContext, AuthProvider
- Tipos: AuthContextType, ReactNode
- Imports locales: @/types

### `contexts/CartContext.tsx`

- Exports: CartProvider, useCart
- Tipos: CartContextType, CartItem
- Imports locales: @/contexts/ToastContext, @/types

### `contexts/ToastContext.tsx`

- Exports: ToastProvider, useToast
- Tipos: ReactNode, Toast, ToastContextType, ToastType
- Imports locales: @/lib/utils

### `features/admin/AdminDashboard.tsx`

- Exports: AdminDashboard
- Tipos: AdminMetricsState, AdminSection
- Imports locales: ./AdminLayout, ./admin-api, ./admin-mock-data, ./useAdminData, @/components/ui/input, @/core/routes, @/hooks/useAuth

### `features/admin/AdminLayout.tsx`

- Exports: AdminLayout
- Tipos: AdminLayoutProps, ReactNode
- Imports locales: ./admin-mock-data, @/core/routes, @/hooks/useAuth

### `layouts/AppShell.tsx`

- Exports: AppShell
- Tipos: ActiveTab
- Imports locales: @/components/BottomNav

### `layouts/MainLayout.tsx`

- Exports: MainLayout, useLayoutContext
- Tipos: LayoutContextType
- Imports locales: @/components/BottomNav, @/components/CartSidebar, @/components/Sidebar, @/components/TopNav, @/contexts/CartContext, @/lib/products-api, @/types

### `pages/Chatbot.tsx`

- Exports: Chatbot
- Imports locales: @/components/chatbot/OutOfStockAlert, @/components/chatbot/RichRecipeCard, @/components/chatbot/TypingIndicator, @/contexts/ToastContext, @/hooks/useChatbot, @/lib/utils

### `pages/Crowdsourcing.tsx`

- Exports: Crowdsourcing
- Imports locales: @/components/Footer, @/components/ui/SuccessCard, @/contexts/ToastContext, @/hooks/useAuth, @/lib/formatters, @/lib/utils

### `pages/Dashboard.tsx`

- Exports: Page
- Imports locales: @/components/Footer, @/components/ProductCard, @/components/ui/ProductSkeleton, @/contexts/CartContext, @/hooks/useCatalog, @/layouts/MainLayout, @/types

### `pages/History.tsx`

- Exports: History
- Imports locales: @/components/Footer, @/lib/utils

### `pages/Home.tsx`

- Exports: Home
- Imports locales: @/components/landing/LandingCTA, @/components/landing/LandingFeatures, @/components/landing/LandingFooter, @/components/landing/LandingHero, @/components/landing/LandingNavbar, @/components/landing/LandingSavings, @/components/landing/LandingStores, @/components/landing/LandingTrustStrip, @/components/landing/LandingWorks

### `pages/Login.tsx`

- Exports: Page
- Imports locales: @/components/auth/AuthPanel, @/components/auth/AuthVisual

### `pages/NotFound.tsx`

- Exports: NotFound
- Imports locales: @/components/ui/button, @/core/routes

### `pages/Onboarding.tsx`

- Exports: Page
- Imports locales: @/components/onboarding/OnboardingWizard

### `pages/Planes.tsx`

- Exports: Planes
- Imports locales: @/hooks/useAuth, @/pages/mocks/MockShell

### `pages/ProductDetail.tsx`

- Exports: ProductDetail
- Imports locales: @/components/Footer, @/components/ProductCarousel, @/components/ui/button, @/contexts/CartContext, @/lib/formatters, @/lib/products-api, @/types

### `pages/Profile.tsx`

- Exports: Profile
- Imports locales: @/hooks/useAuth, @/lib/utils, @/pages/mocks/MockShell

### `pages/RouteViewer.tsx`

- Exports: RouteViewer
- Tipos: as
- Imports locales: @/components/ui/skeleton, @/hooks/useRouteOptimization, @/lib/utils

### `pages/Terms.tsx`

- Exports: Terms
- Imports locales: @/components/ui/button

### `pages/mocks/MockShell.tsx`

- Exports: MockShell
- Tipos: MockShellProps

