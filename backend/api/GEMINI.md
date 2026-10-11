# Microservicio: API Gateway (Go 1.26 + Gin + GORM)

Puerto: 8080 | Dockerfile: Dockerfile | Entry: main.go

## Estructura de paquetes

```
main.go (package main) ──→ docs (blank import para Swagger init)
infrastructure/models.go   (23 modelos GORM, 3 esquemas PostgreSQL)
utils/security.go          (sanitización de inputs)
docs/docs.go               (auto-generado por swaggo/swag)
```

## Dependencias externas (go.mod)

- gin-gonic/gin v1.12.0 — Framework HTTP
- gorm.io/gorm v1.31.2 + gorm.io/driver/postgres v1.6.2 — ORM + driver pgx/v5
- google/uuid v1.6.0 — UUIDs
- swaggo/swag + swaggo/gin-swagger + swaggo/files — Swagger UI

## Funciones en main.go

- `initDB()`: conecta a PostgreSQL via DB_URL (env) o DSN por defecto
- `setupRouter() *gin.Engine`: configura Gin, CORS (orígenes *), monta rutas
- `RootHandler(c *gin.Context)`: GET / → JSON bienvenida + link a Swagger
- `main()`: inicializa DB, router, arranca en PORT (default 8080)

## Rutas registradas

- `GET /` → RootHandler
- `GET /swagger/*any` → Swagger UI

## Modelos GORM (infrastructure/models.go)

23 structs con TableName() explícito en 3 esquemas:

### Esquema api.*
Usuario, PreferencialDieteticaUsuario, DireccionUsuario, PerfilTransporteUsuario,
TarjetaFidelidadUsuario, MisionValidacion, Categoria, Marca, ProductoNormalizado,
EquivalenciaProducto, Receta, IngredienteReceta, MapeoProductoIA

### Esquema scraper.*
CadenaSupermercado, SucursalSupermercado, TrabajoScraper, ProductoCrudo, CapturaPrecio

### Esquema rutas.*
ListaCompra, ArticuloListaCompra, EjecucionOptimizacion, ParadaOptimizacion, DetalleArticuloParada

## Utilidades (utils/security.go)

- `SanitizarInputBusqueda(input string) string`: trim, trunca 100 chars, regex anti-inyección

## Convenciones

- Swagger: anotaciones @Summary, @Tags, @Router en cada handler
- Variable global DB *gorm.DB (inicializada en initDB)
- CORS permite todos los orígenes (desarrollo)
- tests/ existe pero vacío (solo .gitkeep)

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: API de Identidad y Scraping (Go + Gin)

Estado vigente SUP-270 (prevalece sobre el contexto histórico anterior): main compone Identidad y Scraping y arranca recuperación con cierre controlado. Scraping persiste trabajos y sus outboxes scraper.entregas_ingesta/entregas_trabajos; entrega lotes a Catálogo con clave estable y confirma contador+entrega juntos. Publica ejecuciones desde SQL a Redis Streams. No consulta/escribe productos ni recibos de Catálogo. Requiere migración 05_recuperacion_scraping.sql. Ver docs/Guias/GUIA_SCRAPING_RECUPERACION.md.

**44 archivos .go** detectados


### `cmd/seed_productos/main.go` (package main)

- Structs: productoSemilla
- `main()`

### `infrastructure/models.go` (package infrastructure)

- Structs: Usuario, PreferencialDieteticaUsuario, DireccionUsuario, PerfilTransporteUsuario, TarjetaFidelidadUsuario, MisionValidacion, Categoria, Marca, ProductoNormalizado, EquivalenciaProducto, Receta, IngredienteReceta, MapeoProductoIA, CadenaSupermercado, SucursalSupermercado, TrabajoScraper, ProductoCrudo, CapturaPrecio, ListaCompra, ArticuloListaCompra, EjecucionOptimizacion, ParadaOptimizacion, DetalleArticuloParada

### `internal/identity/internal/domain/models.go` (package domain)

- Structs: Usuario

### `internal/identity/internal/domain/repository.go` (package domain)


### `internal/identity/internal/domain/usuario.go` (package domain)

- Structs: ActualizarPerfilDTO, PerfilUsuarioDTO

### `internal/identity/internal/handlers/auth_handler.go` (package handlers)

- Structs: RegistroRequest, RegistroResponse, LoginRequest, LoginResponse, AuthHandler, GoogleLoginRequest
- `NewAuthHandler(authService services.AuthService) *AuthHandler`
- `(AuthHandler).RegistrarUsuario(c *gin.Context)`
- `(AuthHandler).LoginUsuario(c *gin.Context)`
- `(AuthHandler).PerfilUsuario(c *gin.Context)`
- `(AuthHandler).ActualizarPerfil(c *gin.Context)`
- `(AuthHandler).GoogleLoginUsuario(c *gin.Context)`

### `internal/identity/internal/middleware/jwt_auth.go` (package middleware)

- `RequireAuth() gin.HandlerFunc`

### `internal/identity/internal/middleware/rate_limit.go` (package middleware)

- `RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc`
- `RateLimitLogin(rdb *redis.Client) gin.HandlerFunc`

### `internal/identity/internal/middleware/rbac.go` (package middleware)

- `RequireRole(rolesPermitidos ...string) gin.HandlerFunc`

### `internal/identity/internal/repositories/usuario_repository.go` (package repositories)

- Structs: gormUsuarioRepository
- `NewUsuarioRepository(db *gorm.DB) domain.UsuarioRepository`
- `(gormUsuarioRepository).FindByEmail(ctx context.Context, email string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).FindByID(ctx context.Context, id string) (*domain.Usuario, error)`
- `(gormUsuarioRepository).Create(ctx context.Context, usuario *domain.Usuario) error`
- `(gormUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`

### `internal/identity/internal/routes/auth_routes.go` (package routes)

- `RegistrarRutasAuth(rg *gin.RouterGroup, authHandler *handlers.AuthHandler, rdb *redis.Client)`

### `internal/identity/internal/security/jwt.go` (package security)

- Structs: JWTClaims
- `getJWTSecret() []byte`
- `GenerarToken(usuarioID, rol, provider string) (string, error)`
- `ValidarToken(tokenString string) (*jwt.Token, error)`

### `internal/identity/internal/security/password.go` (package security)

- `HashPassword(password string) (string, error)`
- `CheckPasswordHash(password, hash string) bool`

### `internal/identity/internal/services/auth_service.go` (package services)

- Structs: RegistroDTO, UsuarioCreadoDTO, authService
- `NewAuthService(usuarioRepo domain.UsuarioRepository) AuthService`
- `validarComplejidadPassword(password string) error`
- `validarEmail(email string) bool`
- `(authService).Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error)`
- `(authService).Login(correo, password string) (*domain.Usuario, error)`
- `(authService).LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error)`
- `(authService).GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error)`
- `(authService).ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)`

### `internal/identity/module.go` (package identity)

- Structs: Module
- `New(db *gorm.DB, rdb *redis.Client) *Module`
- `(Module).RegisterRoutes(group *gin.RouterGroup)`
- `(Module).RequireAuth() gin.HandlerFunc`
- `(Module).RequireRole(roles ...string) gin.HandlerFunc`

### `internal/identity/tests/auth_handler_test.go` (package tests)

- `TestAuthHandler_RegistrarUsuario_AutoLogin(t *testing.T)`
- `TestAuthHandler_ActualizarPerfil(t *testing.T)`

### `internal/identity/tests/auth_service_test.go` (package tests)

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

### `internal/identity/tests/jwt_test.go` (package tests)

- `TestJWT_GenerarYValidarToken(t *testing.T)`
- `TestJWT_TokenInvalido(t *testing.T)`
- `TestMiddleware_RequireAuth(t *testing.T)`
- `init()`

### `internal/identity/tests/rbac_test.go` (package tests)

- `TestRequireRole(t *testing.T)`

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

### `internal/scraping/internal/repositories/job_publisher.go` (package repositories)

- Structs: jobPublisher
- `NewJobPublisher(rdb *redis.Client, db *gorm.DB) *jobPublisher`
- `(jobPublisher).Close() error`
- `(jobPublisher).Publicar(ctx context.Context, e domain.Ejecucion) error`

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

- `initDB()`
- `initRedis()`
- `setupRouter() (*gin.Engine, error)`
- `setupApplication() (*gin.Engine, *scraping.Module, error)`
- `RootHandler(c *gin.Context)`
- `HealthHandler(c *gin.Context)`
- `main()`

### `main_test.go` (package main)

- `TestModuleCompositionHTTP(t *testing.T)`

### `middleware/error_handler.go` (package middleware)

- Structs: RespuestaError
- `ErrorHandler() gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`

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
