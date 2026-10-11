<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Catálogo y precios (Go + Gin)

Estado vigente SUP-270: Catálogo consulta y persiste productos/precios. Su ingesta privada requiere CATALOGO_INTERNAL_TOKEN y deduplica lote_id en scraper.lotes_catalogo, en la misma transacción que las capturas. No accede a usuarios, trabajos ni outboxes de coordinación. Ver docs/Guias/GUIA_SCRAPING_RECUPERACION.md.

**24 archivos .go** detectados


### `cmd/catalogo/main.go` (package main)

- `main()`
- `run(log *slog.Logger) error`

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
