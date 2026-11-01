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

**34 archivos .go** detectados


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

### `internal/domain/fuente.go` (package domain)

- Structs: FuenteDTO, ActualizarFuenteDTO

### `internal/domain/fuente_repository.go` (package domain)


### `internal/domain/producto.go` (package domain)

- Structs: FiltroProductosDTO, ProductoDTO, ProductoAdminDTO, MetadatosPaginacionDTO, PaginaProductosDTO, PaginaProductosAdminDTO, HistorialPrecioDTO, ProductoDetalleDTO

### `internal/domain/repository.go` (package domain)


### `internal/handlers/fuente_handler.go` (package handlers)

- Structs: FuenteHandler
- `NewFuenteHandler(service services.FuenteService) *FuenteHandler`
- `(FuenteHandler).ObtenerFuentes(c *gin.Context)`
- `(FuenteHandler).ActualizarFuente(c *gin.Context)`

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

### `internal/repositories/fuente_repository.go` (package repositories)

- Structs: gormFuenteRepository, fuenteRow
- `NewFuenteRepository(db *gorm.DB) domain.FuenteRepository`
- `(fuenteRow).aDTO() domain.FuenteDTO`
- `(gormFuenteRepository).ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error)`
- `(gormFuenteRepository).ObtenerFuente(ctx context.Context, id int) (*domain.FuenteDTO, error)`
- `(gormFuenteRepository).ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*domain.FuenteDTO, error)`

### `internal/repositories/producto_repository.go` (package repositories)

- Structs: gormProductoRepository
- `NewProductoRepository(db *gorm.DB) domain.ProductoRepository`
- `(gormProductoRepository).Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error)`
- `(gormProductoRepository).ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error)`
- `(gormProductoRepository).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `internal/server/server.go` (package server)

- `NewRouter(service services.ProductoService, fuentes services.FuenteService, secret string, ping func(context.Context) error, logger *slog.Logger, allowedOrigins []string, internal ...ingesta.Config) *gin.Engine`
- `cors(allowedOrigins []string) gin.HandlerFunc`
- `observe(logger *slog.Logger) gin.HandlerFunc`

### `internal/services/fuente_service.go` (package services)

- Structs: fuenteService
- `NewFuenteService(repo domain.FuenteRepository) FuenteService`
- `(fuenteService).ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error)`
- `(fuenteService).ActualizarFuente(ctx context.Context, id int, input domain.ActualizarFuenteDTO) (*domain.FuenteDTO, error)`

### `internal/services/producto_service.go` (package services)

- Structs: productoService
- `NewProductoService(repo domain.ProductoRepository) ProductoService`
- `(productoService).ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error)`
- `(productoService).ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error)`
- `(productoService).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `internal/tests/architecture_test.go` (package tests)

- `TestCatalogDependencies(t *testing.T)`

### `internal/tests/fuente_handler_test.go` (package tests)

- `fuenteHTTPRouter(repo *mockFuenteRepository) *gin.Engine`
- `solicitarFuente(t *testing.T, router http.Handler, method, path, cookie, body string) *httptest.ResponseRecorder`
- `TestFuenteHandler_HTTP(t *testing.T)`

### `internal/tests/fuente_service_test.go` (package tests)

- Structs: mockFuenteRepository
- `(mockFuenteRepository).ListarFuentes(ctx context.Context) ([]domain.FuenteDTO, error)`
- `(mockFuenteRepository).ObtenerFuente(ctx context.Context, id int) (*domain.FuenteDTO, error)`
- `(mockFuenteRepository).ActualizarEstadoFuente(ctx context.Context, id int, habilitada bool) (*domain.FuenteDTO, error)`
- `TestFuenteService_Listar(t *testing.T)`
- `TestFuenteService_ActualizarFuente(t *testing.T)`

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
- `TestCatalogoClientObtenerCadenaPropagaEstado(t *testing.T)`
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
- `(gormTrabajoRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error)`

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
- `(scraperService).ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error)`
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
- `TestExecutionForwardsAddURLAndEnrichEAN(t *testing.T)`
- `TestExecutionRejectsEmptyAddURL(t *testing.T)`

### `internal/tests/scraper_test.go` (package tests)

- Structs: mockScraperRepository
- `newMockScraperRepository() *mockScraperRepository`
- `(mockScraperRepository).CrearTrabajo(ctx context.Context, trabajo *domain.TrabajoScraper) error`
- `(mockScraperRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*domain.TrabajoScraper, error)`
- `(mockScraperRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error)`
- `(mockScraperRepository).ObtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*domain.CadenaSupermercado, error)`
- `(mockScraperRepository).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*domain.SucursalSupermercado, error)`
- `(mockScraperRepository).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`
- `(mockScraperRepository).IncrementarElementos(ctx context.Context, id uuid.UUID, cantidad int) error`
- `TestScraperService_IniciarTrabajo_Exitoso(t *testing.T)`
- `TestScraperService_FinalizarTrabajo_Validaciones(t *testing.T)`
- `TestScraperService_IngestarProductos_LimitesYValidaciones(t *testing.T)`
- `TestScraperService_IngestaDirectaPorSucursalFuenteDeshabilitada(t *testing.T)`
- `TestScraperService_IngestaDirectaFuenteDeshabilitada(t *testing.T)`
- `TestScraperHandler_EndpointsHTTP(t *testing.T)`
- `stringPtr(s string) *string`
- `TestScraperService_IniciarTrabajo_FuenteDeshabilitada(t *testing.T)`
- `TestScraperService_ObtenerCadena(t *testing.T)`
- `TestScraperService_ObtenerCadena_NoEncontrada(t *testing.T)`
- `TestScraperHandler_IngestaDirectaFuenteDeshabilitada(t *testing.T)`
- `TestScraperHandler_EjecutarTrabajo_FuenteDeshabilitada(t *testing.T)`
- `TestScraperHandler_EjecutarTrabajo_CadenaNoEncontrada(t *testing.T)`
- `TestScraperHandler_IniciarTrabajo_FuenteDeshabilitada(t *testing.T)`

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

**51 archivos .go** detectados


### `cmd/identidad/main.go` (package main)

- `main()`
- `correoPrueba(args []string, log *slog.Logger) error`
- `run(log *slog.Logger) error`
- `recuperacionDesde(ctx context.Context, cfg config.Config, db *gorm.DB, log *slog.Logger) (identity.Recuperacion, error)`

### `internal/config/config.go` (package config)

- Structs: Config
- `Load() (Config, error)`

### `internal/config/config_test.go` (package config)

- `valid(t *testing.T)`
- `TestURLRestablecer(t *testing.T)`
- `TestValidDefaults(t *testing.T)`
- `TestInvalidConfigFailsWithoutSecrets(t *testing.T)`
- `TestExplicitLocalAndProxyConfig(t *testing.T)`

### `internal/config/correo.go` (package config)

- Structs: Correo
- `LoadCorreo() (Correo, error)`

### `internal/config/correo_test.go` (package config)

- `sinCorreo(t *testing.T)`
- `conSMTP(t *testing.T)`
- `TestCorreoSinCredencialesQuedaDeshabilitado(t *testing.T)`
- `TestCorreoSMTPConDefaults(t *testing.T)`
- `TestCorreoLogNoExigeCredenciales(t *testing.T)`
- `TestCorreoInvalidoNoExponeClave(t *testing.T)`
- `TestLoadIncluyeCorreoYFallaSiEsInvalido(t *testing.T)`

### `internal/database/database.go` (package database)

- `Open(ctx context.Context, dsn string) (*gorm.DB, error)`

### `internal/domain/admin_usuario.go` (package domain)

- Structs: FiltroUsuariosAdminDTO, ActualizarUsuarioAdminDTO, UsuarioAdminDTO, MetadatosPaginacionDTO, PaginaUsuariosAdminDTO, MetricasUsuariosDTO
- `NormalizarRol(rol string) (string, bool)`
- `NormalizarPlan(plan string) (string, bool)`

### `internal/domain/auditoria.go` (package domain)

- Structs: AuditoriaAdminUsuario

### `internal/domain/models.go` (package domain)

- Structs: Usuario

### `internal/domain/recuperacion.go` (package domain)


### `internal/domain/repository.go` (package domain)


### `internal/domain/usuario.go` (package domain)

- Structs: ActualizarPerfilDTO, PerfilUsuarioDTO

### `internal/handlers/admin_usuario_handler.go` (package handlers)

- Structs: ActualizarUsuarioAdminRequest, AdminUsuarioHandler
- `NewAdminUsuarioHandler(adminService services.AdminUsuarioService) *AdminUsuarioHandler`
- `(AdminUsuarioHandler).ListarUsuarios(c *gin.Context)`
- `(AdminUsuarioHandler).Metricas(c *gin.Context)`
- `(AdminUsuarioHandler).ActualizarUsuario(c *gin.Context)`
- `bindJSONStrict(c *gin.Context, destino any) error`

### `internal/handlers/auth_handler.go` (package handlers)

- Structs: RegistroRequest, RegistroResponse, LoginRequest, LoginResponse, AuthHandler, GoogleLoginRequest
- `rolCanonico(rol string) string`
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

- Structs: Module, Recuperacion
- `New(db *gorm.DB, rdb *redis.Client, secureCookie ...bool) *Module`
- `NewConRecuperacion(db *gorm.DB, rdb *redis.Client, recuperacion Recuperacion, secureCookie ...bool) *Module`
- `(Module).Cerrar(ctx context.Context) error`
- `(Module).RegisterRoutes(group *gin.RouterGroup)`
- `(Module).RequireAuth() gin.HandlerFunc`
- `(Module).RequireRole(roles ...string) gin.HandlerFunc`
- `VerifyStorage(ctx context.Context, db *gorm.DB) error`
- `VerificarRecuperacion(ctx context.Context, db *gorm.DB) error`

### `internal/mailer/mailer.go` (package mailer)

- Structs: Mensaje, deshabilitado, registro
- `Nuevo(cfg config.Correo, log *slog.Logger) (Mailer, error)`
- `MensajePrueba(para, origen string, ahora time.Time) Mensaje`
- `(registro).Enviar(ctx context.Context, m Mensaje) error`
- `validar(m Mensaje) (*mail.Address, error)`
- `construir(remitente, para *mail.Address, m Mensaje, ahora time.Time) ([]byte, error)`
- `escribirQP(w io.Writer, texto string) error`
- `messageID(remitente string) string`

### `internal/mailer/mailer_test.go` (package mailer)

- Structs: servidorFalso
- `certificadoLocal(t *testing.T) (*tls.Config, *x509.CertPool)`
- `(servidorFalso).iniciar(t *testing.T) int`
- `(servidorFalso).atender(conn net.Conn)`
- `nuevoSMTPDePrueba(t *testing.T, puerto int, modoTLS string, clave string, pool *x509.CertPool) *SMTP`
- `etapa(err error) string`
- `TestSMTPEntregaPorSTARTTLSConTildes(t *testing.T)`
- `TestSMTPEntregaConTLSImplicito(t *testing.T)`
- `TestSMTPNoEnviaCredencialesSinSTARTTLS(t *testing.T)`
- `TestSMTPRechazaCertificadoNoConfiable(t *testing.T)`
- `TestSMTPCredencialesRechazadasNoExponenClave(t *testing.T)`
- `TestSMTPServidorColgadoRespetaDeadline(t *testing.T)`
- `TestSMTPConexionRechazadaIndicaEtapa(t *testing.T)`
- `TestMensajesInvalidosNoAbrenConexion(t *testing.T)`
- `TestConstruirAlternativaTextoYHTML(t *testing.T)`
- `TestNuevoSegunModo(t *testing.T)`

### `internal/mailer/smtp.go` (package mailer)

- Structs: SMTP, ErrorSMTP
- `(ErrorSMTP).Error() string`
- `(ErrorSMTP).Unwrap() error`
- `NuevoSMTP(cfg config.Correo) (*SMTP, error)`
- `(SMTP).Enviar(ctx context.Context, m Mensaje) error`
- `nombreLocal() string`

### `internal/middleware/jwt_auth.go` (package middleware)

- `RequireAuth(repo domain.UsuarioRepository) gin.HandlerFunc`

### `internal/middleware/rate_limit.go` (package middleware)

- `RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc`
- `rateLimiter(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration, soloFallidos bool) gin.HandlerFunc`
- `RateLimitLogin(rdb *redis.Client) gin.HandlerFunc`
- `RateLimitRegistro(rdb *redis.Client) gin.HandlerFunc`
- `RateLimitGoogle(rdb *redis.Client) gin.HandlerFunc`

### `internal/middleware/rbac.go` (package middleware)

- `RequireRole(rolesPermitidos ...string) gin.HandlerFunc`

### `internal/repositories/recuperacion_repository.go` (package repositories)

- Structs: gormRecuperacionRepository
- `NewRecuperacionRepository(db *gorm.DB) domain.RecuperacionRepository`
- `(gormRecuperacionRepository).SolicitudesRecientes(ctx context.Context, usuarioID uuid.UUID, ventana, espera time.Duration) (int, bool, error)`
- `(gormRecuperacionRepository).EmitirToken(ctx context.Context, usuarioID uuid.UUID, hash []byte, vigencia time.Duration) error`

### `internal/repositories/usuario_repository.go` (package repositories)

- Structs: gormUsuarioRepository, repositorioCerrado
- `NewUsuarioRepository(db *gorm.DB) domain.UsuarioRepository`
- `NewAdminUsuarioRepository(db *gorm.DB) domain.AdminUsuarioRepository`
- `(gormUsuarioRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).FindByID(ctx context.Context, id string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error)`
- `(gormUsuarioRepository).Contar(ctx context.Context) (int64, int64, error)`
- `(gormUsuarioRepository).Create(ctx context.Context, usuario *domain.Usuario) error`
- `(gormUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`
- `(gormUsuarioRepository).ActualizarConGuarda(ctx context.Context, id string, datos map[string]interface{}, actorID string) error`
- `escaparPatronLike(texto string) string`
- `classifyWriteError(err error) error`

### `internal/repositories/usuario_repository_test.go` (package repositories)

- `TestEscaparPatronLike(t *testing.T)`

### `internal/routes/admin_usuario_routes.go` (package routes)

- `RegistrarRutasAdminUsuarios(rg *gin.RouterGroup, handler *handlers.AdminUsuarioHandler, repo domain.UsuarioRepository)`

### `internal/routes/auth_routes.go` (package routes)

- `RegistrarRutasAuth(rg *gin.RouterGroup, authHandler *handlers.AuthHandler, rdb *redis.Client, repo domain.UsuarioRepository)`

### `internal/security/jwt.go` (package security)

- Structs: JWTClaims
- `getJWTSecret() []byte`
- `GenerarToken(usuarioID, rol, provider string) (string, error)`
- `ValidarToken(tokenString string) (*jwt.Token, error)`

### `internal/security/password.go` (package security)

- `HashPassword(password string) (string, error)`
- `CheckPasswordHash(password, hash string) bool`

### `internal/security/recuperacion.go` (package security)

- `GenerarTokenRecuperacion() (token string, hash []byte, err error)`
- `HashTokenRecuperacion(token string) []byte`

### `internal/server/server.go` (package server)

- `NewRouter(cfg config.Config, db *gorm.DB, redisClient *redis.Client, ping func(context.Context) error, log *slog.Logger) *gin.Engine`
- `NewRouterConModulo(cfg config.Config, modulo *identity.Module, redisClient *redis.Client, ping func(context.Context) error, log *slog.Logger) *gin.Engine`
- `cors(origins []string) gin.HandlerFunc`
- `observe(log *slog.Logger) gin.HandlerFunc`
- `boundedRequest() gin.HandlerFunc`

### `internal/server/server_test.go` (package server)

- `TestHTTPBoundariesCorsAndJWT(t *testing.T)`
- `TestRecoveryDoesNotExposePanic(t *testing.T)`
- `TestReadinessDeadlineAndLiveness(t *testing.T)`
- `TestUnclassifiedContextErrorDoesNotExposeSQL(t *testing.T)`

### `internal/services/admin_usuario_service.go` (package services)

- Structs: adminUsuarioService
- `NewAdminUsuarioService(usuarioRepo domain.AdminUsuarioRepository) AdminUsuarioService`
- `(adminUsuarioService).Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) (*domain.PaginaUsuariosAdminDTO, error)`
- `(adminUsuarioService).Metricas(ctx context.Context) (*domain.MetricasUsuariosDTO, error)`
- `(adminUsuarioService).Actualizar(ctx context.Context, id string, input domain.ActualizarUsuarioAdminDTO, actorID string) (*domain.UsuarioAdminDTO, error)`
- `normalizarRol(rol string) (string, error)`
- `normalizarPlan(plan string) (string, error)`
- `mapearUsuarioAdmin(usuario *domain.Usuario) domain.UsuarioAdminDTO`

### `internal/services/auth_service.go` (package services)

- Structs: RegistroDTO, UsuarioCreadoDTO, authService
- `NewAuthService(usuarioRepo domain.UsuarioRepository) AuthService`
- `validarComplejidadPassword(password string) error`
- `validarEmail(email string) bool`
- `(authService).registrarUltimoAcceso(ctx context.Context, usuario *domain.Usuario)`
- `(authService).Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error)`
- `(authService).Login(correo, password string) (*domain.Usuario, error)`
- `(authService).LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error)`
- `(authService).GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error)`
- `(authService).ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)`
- `NewAuthServiceWithGoogleValidator(repo domain.UsuarioRepository, validate func(context.Context, string, string) (*idtoken.Payload, error)) AuthService`
- `(authService).validateGoogleAccount(ctx context.Context, user *domain.Usuario, subject string) (*domain.Usuario, error)`

### `internal/services/recuperacion_service.go` (package services)

- Structs: recuperacionService, recuperacionApagada
- `NewRecuperacionService(usuarios domain.UsuarioRepository, tokens domain.RecuperacionRepository, correo mailer.Mailer, urlRestablecer string, log *slog.Logger) RecuperacionService`
- `normalizarCorreoRecuperacion(correo string) (string, error)`
- `(recuperacionService).Solicitar(correo string) error`
- `(recuperacionService).Cerrar(ctx context.Context) error`
- `(recuperacionService).trabajar()`
- `(recuperacionService).procesar(correo string)`
- `mensajeRecuperacion(usuario *domain.Usuario, urlRestablecer, token string) (mailer.Mensaje, error)`

### `internal/tests/admin_usuario_test.go` (package tests)

- `newAdminUsuariosTestRouter(t *testing.T) (*gin.Engine, *mockUsuarioRepository)`
- `asegurarActorAdmin(t *testing.T, repo *mockUsuarioRepository, rol string) string`
- `adminRequest(t *testing.T, router *gin.Engine, repo *mockUsuarioRepository, method, path, rol, body string) *httptest.ResponseRecorder`
- `adminRequestComo(t *testing.T, router *gin.Engine, actorID, rol, method, path, body string) *httptest.ResponseRecorder`
- `TestAdminUsuariosRechazaNoAdminCon403(t *testing.T)`
- `TestAdminUsuariosSinSesionRetorna401(t *testing.T)`
- `TestAdminUsuariosListaConRolAdmin(t *testing.T)`
- `TestAdminUsuariosActivaDesactivaYAsignaColaborador(t *testing.T)`
- `TestAdminUsuariosValidaRolYDestino(t *testing.T)`
- `TestAdminUsuariosRechazaFiltrosInvalidos(t *testing.T)`
- `TestAdminUsuariosPaginaPorDefecto(t *testing.T)`
- `TestAdminUsuariosRechazaLimiteExcesivo(t *testing.T)`
- `TestAdminUsuariosMetricas(t *testing.T)`
- `TestAdminUsuariosProtegeUltimoAdminActivo(t *testing.T)`
- `TestAdminUsuariosRechazaAutoDegradacionPeroPermitePlan(t *testing.T)`
- `TestAdminUsuariosNormalizaAliasYAuditaElCambio(t *testing.T)`
- `TestAdminUsuariosNoAuditaMutacionIdempotente(t *testing.T)`
- `TestAdminUsuariosFiltraPorTextoRolYEstado(t *testing.T)`
- `TestAdminUsuariosPaginaYFueraDeRango(t *testing.T)`
- `TestAdminUsuariosActualizaPlan(t *testing.T)`
- `TestAdminUsuariosRechazaCamposDesconocidos(t *testing.T)`

### `internal/tests/architecture_test.go` (package tests)

- `TestIdentityDoesNotDependOnOtherServices(t *testing.T)`

### `internal/tests/auth_handler_test.go` (package tests)

- `TestAuthHandler_RegistrarUsuario_AutoLogin(t *testing.T)`
- `TestAuthHandler_LoginNormalizaRolLegacy(t *testing.T)`
- `TestAuthHandler_ActualizarPerfil(t *testing.T)`

### `internal/tests/auth_service_test.go` (package tests)

- Structs: mockUsuarioRepository
- `newMockUsuarioRepository() *mockUsuarioRepository`
- `(mockUsuarioRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(mockUsuarioRepository).FindByID(ctx context.Context, id string) (*domain.Usuario, error)`
- `(mockUsuarioRepository).Create(ctx context.Context, usuario *domain.Usuario) error`
- `(mockUsuarioRepository).Contar(ctx context.Context) (int64, int64, error)`
- `(mockUsuarioRepository).buscarPorID(id string) *domain.Usuario`
- `(mockUsuarioRepository).aplicarDatos(target *domain.Usuario, datos map[string]interface{})`
- `(mockUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`
- `(mockUsuarioRepository).ActualizarConGuarda(ctx context.Context, id string, datos map[string]interface{}, actorID string) error`
- `(mockUsuarioRepository).Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error)`
- `(mockUsuarioRepository).ContarAdminsActivos(ctx context.Context, excluirID string) (int64, error)`
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
- `TestMiddleware_RequireAuthExigeRepositorio(t *testing.T)`
- `TestMiddleware_RequireAuthRevalidaEstadoYRolEnBaseDeDatos(t *testing.T)`
- `init()`

### `internal/tests/main_test.go` (package tests)

- `TestMain(m *testing.M)`

### `internal/tests/rbac_test.go` (package tests)

- `TestRequireRole(t *testing.T)`

### `internal/tests/recuperacion_repository_test.go` (package tests)

- `TestRecuperacionRepositoryPostgres(t *testing.T)`

### `internal/tests/recuperacion_service_test.go` (package tests)

- Structs: tokenEmitido, tokensFalsos, correoFalso, usuariosQueFallan
- `(tokensFalsos).SolicitudesRecientes(context.Context, uuid.UUID, time.Duration, time.Duration) (int, bool, error)`
- `(tokensFalsos).EmitirToken(_ context.Context, usuario uuid.UUID, hash []byte, vigencia time.Duration) error`
- `(correoFalso).Enviar(ctx context.Context, m mailer.Mensaje) error`
- `usuarioDePrueba(repo *mockUsuarioRepository, correo string, activo bool, conClave bool) *domain.Usuario`
- `procesarPendientes(t *testing.T, s services.RecuperacionService)`
- `TestRecuperacionEmiteTokenHasheadoYEnviaCorreo(t *testing.T)`
- `TestRecuperacionNoRevelaCuentasNiProveedor(t *testing.T)`
- `TestRecuperacionValidaFormatoSinEncolar(t *testing.T)`
- `TestRecuperacionLimitaSolicitudesPorCuenta(t *testing.T)`
- `TestRecuperacionFallasQuedanEnLogSinCorreoDelUsuario(t *testing.T)`
- `TestRecuperacionApagadaSinTransporteOURL(t *testing.T)`
- `TestRecuperacionColaLlenaNoBloqueaYCerrarRespetaPlazo(t *testing.T)`
- `TestTokenRecuperacionAleatorioYHashEstable(t *testing.T)`

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

**22 archivos .py** detectados


### `app/auth.py`

- Imports internos: app
- **class UsuarioAutenticado(BaseModel)**
- `_extraer_bearer(authorization: ?) → ?`
- `async obtener_usuario_actual(jwt_cookie: ?, authorization: ?) → UsuarioAutenticado`

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

- Imports internos: app
- **class ProductoRepositorio**
  - `__init__(ruta_fallback: ?, base_url: ?, timeout: float, client: ?, cache_ttl: float) → ?`
  - `fuente() → str`
  - `_normalizar_producto(item: ?) → ?`
  - `_cargar_fallback() → ?`
  - `_realizar_peticion_http(client: httpx.Client, page: int, limit: int) → ?`
  - `_expirado() → bool`
  - `_cargar_sin_lock() → ?`
  - `cargar_productos() → ?`
  - `obtener_todos(forzar: bool) → ?`
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
- `catalogo_fijo(monkeypatch)`
- `client()`
- `generar_token(user_id: str, rol: str, exp_delta: timedelta, secret: str, algorithm: str, extra_claims: dict) → str`
- `test_chat_sin_token_retorna_401(client)`
- `test_chat_header_malformado_retorna_401(client)`
- `test_chat_token_invalido_retorna_401(client)`
- `test_chat_token_firma_invalida_retorna_401(client)`
- `test_chat_token_expirado_retorna_401(client)`
- `test_chat_token_sin_user_id_retorna_401(client)`
- `test_chat_token_valido_retorna_200_con_pipeline_tres_etapas(client)`
- `test_chat_secret_no_configurado_retorna_500(client, monkeypatch)`
- `test_chat_texto_libre_sin_filtros(client)`
- `test_chat_etapa_3_falla_retorna_200_con_productos_y_fallback(client)`
- `test_chat_error_en_proveedor(client)`
- `test_chat_con_cookie_jwt_retorna_200(client)`
- `test_chat_token_sin_claim_exp_retorna_401(client)`
- `test_chat_mensaje_largo_excede_maximo_retorna_422(client)`
- `test_endpoints_antiguos_groq_gemini_eliminados(client)`
- `test_cors_configuracion_origenes(client)`

### `tests/test_productos.py`

- Imports internos: app.auth, app.filtros, app.main, app.productos, app.proveedores.base, app.routers.chat
- **class MockProveedor(ProveedorIA)**
  - `__init__()`
  - `async extraer_filtros(mensaje: str)`
  - `async generar_explicacion(mensaje: str, productos: list)`
- `temp_fallback_file(tmp_path: Path) → Path`
- `test_carga_exitosa_desde_catalogo_http(temp_fallback_file: Path)`
- `test_paginacion_multiples_paginas(temp_fallback_file: Path)`
- `test_degradacion_timeout_aplica_fallback(temp_fallback_file: Path)`
- `test_degradacion_error_conexion_aplica_fallback(temp_fallback_file: Path)`
- `test_degradacion_error_http_500_aplica_fallback(temp_fallback_file: Path)`
- `test_preserva_cache_en_memoria_tras_falla_posterior(temp_fallback_file: Path)`
- `test_compatibilidad_con_filtros_deterministas(temp_fallback_file: Path)`
- `test_fastapi_health_check_sin_crashear_modo_degradado()`
- `test_fastapi_endpoint_productos_devuelve_catalogo()`
- `test_len_es_o1_y_no_llama_a_red(temp_fallback_file: Path)`
- `test_modo_degradado_resetea_ttl_evitando_llamadas_en_rafaga(temp_fallback_file: Path)`
- `test_httpx_client_propio_cierra_socket_ante_error(temp_fallback_file: Path)`
- `test_telemetria_truncamiento_mas_de_10_paginas(temp_fallback_file: Path, caplog: pytest.LogCaptureFixture)`
- `test_chat_router_ejecuta_obtener_todos_sin_bloquear(temp_fallback_file: Path)`
- `test_concurrencia_double_checked_locking_evita_llamadas_duplicadas(temp_fallback_file: Path)`
- `test_catalogo_caido_modo_sin_datos_respeta_ttl(temp_fallback_file: Path)`


## Mapa auto-generado: Motor de Rutas (FastAPI + OR-Tools)

**20 archivos .py** detectados


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

### `app/api/router.py`

- Imports internos: app.listas.router

### `app/auth.py`

- Imports internos: app.errores
- **class Sesion**
- `validar_token(token: str, secreto: str) → Sesion`
- `sesion_actual(request: Request) → Sesion`

### `app/config.py`

- **class ErrorConfiguracion(ValueError)**
- **class Configuracion**
- `cargar(entorno: ?) → Configuracion`

### `app/core/strategy.py`

- **class RoutingStrategy(ABC)**
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/db.py`

- **class ErrorAlmacenamiento(RuntimeError)**
- `async abrir_pool(dsn: str, maximo: int) → AsyncConnectionPool`
- `async verificar_permisos(pool: AsyncConnectionPool) → ?`
- `async ping(pool: AsyncConnectionPool, plazo: float) → bool`

### `app/errores.py`

- **class ErrorAPI(Exception)**
  - `__init__(estado: int, mensaje: str, detalle: ?) → ?`
- `respuesta_error(estado: int, mensaje: str, detalle: ?, cabeceras: ?) → JSONResponse`
- `registrar_manejadores(app: FastAPI) → ?`

### `app/listas/modelos.py`

- **class ListaCrear(BaseModel)**
  - `_nombre(cls, valor: str) → str`
- **class ListaActualizar(BaseModel)**
  - `_nombre(cls, valor: ?) → ?`
  - `_algun_cambio() → ListaActualizar`
- **class Articulo(BaseModel)**
- **class Lista(BaseModel)**
- **class ListaDetalle(Lista)**
- **class RespuestaListas(BaseModel)**
- `normalizar_nombre(valor: str) → str`

### `app/listas/repositorio.py`

- Imports internos: app.listas.modelos
- **class RepositorioListas(Protocol)**
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- **class RepositorioListasPostgres**
  - `__init__(pool: AsyncConnectionPool) → ?`
  - `async _una(consulta: str, parametros: tuple) → ?`
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- `_utc(valor: datetime) → datetime`
- `_lista(fila: dict) → Lista`

### `app/listas/router.py`

- Imports internos: app.auth, app.errores, app.listas.modelos, app.listas.repositorio
- `repositorio(request: Request) → RepositorioListas`
- `_id_o_404(lista_id: str) → str`
- `async listar(sesion: Sesion, repo: RepositorioListas) → RespuestaListas`
- `async crear(datos: ListaCrear, sesion: Sesion, repo: RepositorioListas) → Lista`
- `async obtener(lista_id: str, sesion: Sesion, repo: RepositorioListas) → ListaDetalle`
- `async actualizar(lista_id: str, datos: ListaActualizar, sesion: Sesion, repo: RepositorioListas) → Lista`
- `async eliminar(lista_id: str, sesion: Sesion, repo: RepositorioListas) → Response`

### `app/main.py`

- Imports internos: app, app.api.router, app.config, app.errores, app.listas.repositorio
- `async _almacenamiento(app: FastAPI, config: Configuracion) → ?`
- `create_app(config: ?, repositorio: ?) → FastAPI`
- `_configurar_logs() → ?`

### `tests/apoyo.py`

- Imports internos: app.config, app.listas.modelos
- **class RepositorioMemoria**
  - `__init__() → ?`
  - `_lista(fila: dict) → Lista`
  - `_propia(usuario_id: str, lista_id: str) → ?`
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- `token(usuario_id: ?, secreto: str, algoritmo: str) → str`

### `tests/test_config.py`

- Imports internos: app.config
- **class ConfiguracionTest(unittest.TestCase)**
  - `test_valida()`
  - `test_rechaza_sin_exponer_secretos()`

### `tests/test_listas.py`

- Imports internos: app.main
- **class ListasTest(unittest.TestCase)**
  - `setUp() → ?`
  - `como(usuario_id: str) → ?`
  - `crear(nombre: str) → dict`
  - `test_crud_completo_del_dueno()`
  - `test_listas_ajenas_responden_404_sin_modificarse()`
  - `test_id_mal_formado_es_404()`
  - `test_validacion_con_envoltorio_y_sin_eco_del_valor()`
  - `test_json_mal_formado_usa_envoltorio()`
  - `test_nombre_sin_caracteres_de_control()`
  - `test_tope_de_listas_por_usuario()`
  - `test_rutas_y_metodos_desconocidos_usan_envoltorio()`
- **class AutenticacionTest(unittest.TestCase)**
  - `setUp() → ?`
  - `estado_con(valor: ?) → int`
  - `test_token_valido()`
  - `test_rechaza_tokens_inseguros()`
  - `test_ignora_identidad_en_headers()`
- **class InfraestructuraHTTPTest(unittest.TestCase)**
  - `setUp() → ?`
  - `test_cors_lista_blanca()`
  - `test_request_id_se_propaga_o_genera()`
  - `test_probes()`

### `tests/test_prototipo_ruta_ficticia.py`

- `ejecutar_demo()`

### `tests/test_repositorio_postgres.py`

- Imports internos: app, app.listas.repositorio, app.main
- **class RepositorioPostgresTest(unittest.IsolatedAsyncioTestCase)**
  - `async asyncSetUp() → ?`
  - `async test_permisos_y_crud_aislado_por_usuario()`
  - `async test_detalle_con_articulos_y_borrado_en_cascada()`
  - `async test_app_rutas_no_sale_de_su_matriz()`


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


## Mapa auto-generado: Frontend (React + TypeScript)

**86 archivos activos** (excluidos kebab-case obsoletos)


### `core/routes.ts`

- Exports: MOCK_ROUTES, ROUTES

### `data/mock.ts`

- Exports: HOME, discountPct, formatPrice, mockUser, products, supermarketById, supermarkets
- Imports locales: ../types

### `features/admin/admin-api.ts`

- Exports: ADMIN_ENDPOINTS, ADMIN_HAS_BACKEND, adminPlanFromBackend, adminPlanToBackend, adminRoleFromBackend, adminRoleToBackend, adminSourceFromBackend, adminUserFromBackend, fetchCatalogoMetrics, fetchControlMetrics, fetchIdentidadMetrics, isAdminApiError, isAdminAuthError, isPersistableId
- Tipos: AdminUsersPage, AdminUsersQuery, CatalogoMetrics, ControlMetrics, ErrorBackend, FuenteBackend, IdentidadMetrics, PaginaUsuariosBackend, PaginacionBackend, UsuarioBackend
- Imports locales: ./admin-mock-data

### `features/admin/admin-mock-data.ts`

- Exports: ADMIN_PLANS, ADMIN_ROLES, adminMetricsMock, adminSources, adminUsers
- Tipos: AdminPlan, AdminRole, AdminSection, AdminSource, AdminUser, AdminUserRole

### `features/admin/useAdminData.ts`

- Exports: useAdminMetrics, useAdminSources, useAdminUsers
- Tipos: AdminMetricsState, AdminPlan, AdminRole, AdminSource, AdminUser, AdminUsersPagination, AdminUsersParams, CatalogoMetrics, ControlMetrics, IdentidadMetrics, ServiceState
- Imports locales: ./admin-api, ./admin-mock-data, @/core/routes, @/hooks/useAuth

### `features/listas/listas-api.ts`

- Exports: listasApi
- Tipos: EstadoLista, Lista, ListaDetalle

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

### `hooks/useRole.ts`

- Exports: useRole
- Imports locales: ./useAuth, @/lib/roles

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

### `lib/roles.ts`

- Exports: ROLE_ADMIN, ROLE_COLABORADOR, ROLE_REGISTRADO, hasAllowedRole, normalizeRole

### `lib/utils.ts`

- Exports: cn
- Tipos: ClassValue

### `types/index.ts`

- Tipos: CadenaSupermercado, CapturaPrecio, Categoria, LoginPayload, Marca, PreferenciasDieteticas, ProductoCrudo, ProductoNormalizado, RegisterPayload, SucursalSupermercado, UiProduct, UiSupermarket, Usuario

### `app/router.tsx`

- Exports: AppRouter
- Imports locales: @/components/ErrorBoundary, @/components/auth/ProtectedRoute, @/contexts/AuthContext, @/contexts/CartContext, @/contexts/ToastContext, @/core/routes, @/features/admin/AdminDashboard, @/layouts/MainLayout, @/lib/roles, @/pages/Chatbot, @/pages/Crowdsourcing, @/pages/Dashboard, @/pages/History, @/pages/Home, @/pages/Login, @/pages/NotFound, @/pages/Onboarding, @/pages/Planes, @/pages/ProductDetail, @/pages/Profile, @/pages/RouteViewer, @/pages/ShoppingLists, @/pages/Terms, @/pages/mocks/MockShell

### `components/BottomNav.tsx`

- Exports: BottomNav
- Tipos: BottomNavProps
- Imports locales: @/hooks/useRole, @/lib/utils

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
- Imports locales: @/components/ui/button, @/hooks/useRole, @/types

### `components/TopNav.tsx`

- Exports: TopNav
- Tipos: FormEvent, TopNavProps
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
- Imports locales: @/contexts/ToastContext, @/core/routes, @/hooks/useAuth, @/lib/roles

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

### `pages/ShoppingLists.tsx`

- Exports: ShoppingLists
- Tipos: EstadoLista, FormEvent, Lista, ListaDetalle
- Imports locales: @/components/ui/button, @/components/ui/input, @/contexts/ToastContext, @/core/routes, @/features/listas/listas-api, @/hooks/useAuth

### `pages/Terms.tsx`

- Exports: Terms
- Imports locales: @/components/ui/button

### `pages/mocks/MockShell.tsx`

- Exports: MockShell
- Tipos: MockShellProps

