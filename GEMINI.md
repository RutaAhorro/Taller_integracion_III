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


## Mapa auto-generado: API Gateway (Go + Gin)

**33 archivos .go** detectados


### `cmd/seed_productos/main.go` (package main)

- Structs: productoSemilla
- `main()`

### `domain/producto.go` (package domain)

- Structs: FiltroProductosDTO, ProductoDTO, ProductoAdminDTO, MetadatosPaginacionDTO, PaginaProductosDTO, PaginaProductosAdminDTO, HistorialPrecioDTO, ProductoDetalleDTO

### `domain/scraper.go` (package domain)

- Structs: IniciarTrabajoDTO, TrabajoScraperDTO, FinalizarTrabajoDTO, ProductoScrapeadoDTO, IngestaLoteDTO, IngestaResultadoDTO

### `domain/usuario.go` (package domain)

- Structs: ActualizarPerfilDTO, PerfilUsuarioDTO

### `handlers/auth_handler.go` (package handlers)

- Structs: RegistroRequest, RegistroResponse, LoginRequest, LoginResponse, AuthHandler, GoogleLoginRequest
- `NewAuthHandler(authService services.AuthService) *AuthHandler`
- `(AuthHandler).RegistrarUsuario(c *gin.Context)`
- `(AuthHandler).LoginUsuario(c *gin.Context)`
- `(AuthHandler).PerfilUsuario(c *gin.Context)`
- `(AuthHandler).ActualizarPerfil(c *gin.Context)`
- `(AuthHandler).GoogleLoginUsuario(c *gin.Context)`

### `handlers/producto_handler.go` (package handlers)

- Structs: ProductoHandler
- `NewProductoHandler(service services.ProductoService) *ProductoHandler`
- `(ProductoHandler).ObtenerProductos(c *gin.Context)`
- `(ProductoHandler).ObtenerProductosAdmin(c *gin.Context)`
- `(ProductoHandler).BuscarProductos(c *gin.Context)`
- `(ProductoHandler).ObtenerDetalleProducto(c *gin.Context)`

### `handlers/scraper_handler.go` (package handlers)

- Structs: ScraperHandler, ejecutarScraperRequest
- `NewScraperHandler(service services.ScraperService, rdb ...*redis.Client) *ScraperHandler`
- `(ScraperHandler).EjecutarTrabajo(c *gin.Context)`
- `(ScraperHandler).IniciarTrabajo(c *gin.Context)`
- `(ScraperHandler).FinalizarTrabajo(c *gin.Context)`
- `(ScraperHandler).ObtenerTrabajo(c *gin.Context)`
- `(ScraperHandler).IngestarProductosConTrabajo(c *gin.Context)`
- `(ScraperHandler).IngestarProductosDirecto(c *gin.Context)`
- `(ScraperHandler).manejarErrorIngesta(c *gin.Context, err error)`

### `infrastructure/models.go` (package infrastructure)

- Structs: Usuario, PreferencialDieteticaUsuario, DireccionUsuario, PerfilTransporteUsuario, TarjetaFidelidadUsuario, MisionValidacion, Categoria, Marca, ProductoNormalizado, EquivalenciaProducto, Receta, IngredienteReceta, MapeoProductoIA, CadenaSupermercado, SucursalSupermercado, TrabajoScraper, ProductoCrudo, CapturaPrecio, ListaCompra, ArticuloListaCompra, EjecucionOptimizacion, ParadaOptimizacion, DetalleArticuloParada

### `main.go` (package main)

- `initDB()`
- `initRedis()`
- `setupRouter() *gin.Engine`
- `RootHandler(c *gin.Context)`
- `HealthHandler(c *gin.Context)`
- `main()`

### `middleware/error_handler.go` (package middleware)

- Structs: RespuestaError
- `ErrorHandler() gin.HandlerFunc`
- `NotFoundHandler() gin.HandlerFunc`
- `MethodNotAllowedHandler() gin.HandlerFunc`
- `ResponderError(c *gin.Context, estado int, mensaje string, err error)`

### `middleware/jwt_auth.go` (package middleware)

- `RequireAuth() gin.HandlerFunc`

### `middleware/rate_limit.go` (package middleware)

- `RateLimiterIP(rdb *redis.Client, prefijo string, maxIntentos int64, ventana time.Duration) gin.HandlerFunc`
- `RateLimitLogin(rdb *redis.Client) gin.HandlerFunc`

### `middleware/rbac.go` (package middleware)

- `RequireRole(rolesPermitidos ...string) gin.HandlerFunc`

### `repositories/producto_repository.go` (package repositories)

- Structs: gormProductoRepository
- `NewProductoRepository(db *gorm.DB) ProductoRepository`
- `(gormProductoRepository).Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error)`
- `(gormProductoRepository).ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error)`
- `(gormProductoRepository).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `repositories/scraper_repository.go` (package repositories)

- Structs: gormScraperRepository
- `NewScraperRepository(db *gorm.DB) ScraperRepository`
- `(gormScraperRepository).CrearTrabajo(ctx context.Context, trabajo *infrastructure.TrabajoScraper) error`
- `(gormScraperRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*infrastructure.TrabajoScraper, error)`
- `(gormScraperRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*infrastructure.TrabajoScraper, error)`
- `(gormScraperRepository).ObtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*infrastructure.CadenaSupermercado, error)`
- `(gormScraperRepository).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*infrastructure.SucursalSupermercado, error)`
- `(gormScraperRepository).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`

### `repositories/usuario_repository.go` (package repositories)

- Structs: gormUsuarioRepository
- `NewUsuarioRepository(db *gorm.DB) UsuarioRepository`
- `(gormUsuarioRepository).FindByEmail(ctx context.Context, email string) (*infrastructure.Usuario, error)`
- `(gormUsuarioRepository).FindByID(ctx context.Context, id string) (*infrastructure.Usuario, error)`
- `(gormUsuarioRepository).Create(ctx context.Context, usuario *infrastructure.Usuario) error`
- `(gormUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`

### `routes/admin_routes.go` (package routes)

- `RegistrarRutasAdmin(rg *gin.RouterGroup, db *gorm.DB)`

### `routes/auth_routes.go` (package routes)

- `RegistrarRutasAuth(rg *gin.RouterGroup, db *gorm.DB, rdb *redis.Client)`

### `routes/producto_routes.go` (package routes)

- `RegistrarRutasProductos(rg *gin.RouterGroup, db *gorm.DB)`

### `routes/scraper_routes.go` (package routes)

- `RegistrarRutasScraper(rg *gin.RouterGroup, db *gorm.DB, rdb *redis.Client)`

### `services/auth_service.go` (package services)

- Structs: RegistroDTO, UsuarioCreadoDTO, authService
- `NewAuthService(usuarioRepo repositories.UsuarioRepository) AuthService`
- `validarComplejidadPassword(password string) error`
- `validarEmail(email string) bool`
- `(authService).Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error)`
- `(authService).Login(correo, password string) (*infrastructure.Usuario, error)`
- `(authService).LoginWithContext(ctx context.Context, correo, password string) (*infrastructure.Usuario, error)`
- `(authService).GoogleLogin(ctx context.Context, tokenGoogle string) (*infrastructure.Usuario, error)`
- `(authService).ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)`

### `services/producto_service.go` (package services)

- Structs: productoService
- `NewProductoService(repo repositories.ProductoRepository) ProductoService`
- `(productoService).ObtenerCatalogo(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosDTO, error)`
- `(productoService).ObtenerCatalogoAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) (*domain.PaginaProductosAdminDTO, error)`
- `(productoService).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`

### `services/scraper_service.go` (package services)

- Structs: scraperService
- `NewScraperService(repo repositories.ScraperRepository) ScraperService`
- `(scraperService).IniciarTrabajo(ctx context.Context, input domain.IniciarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).FinalizarTrabajo(ctx context.Context, id string, input domain.FinalizarTrabajoDTO) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).ObtenerTrabajo(ctx context.Context, id string) (*domain.TrabajoScraperDTO, error)`
- `(scraperService).IngestarProductos(ctx context.Context, trabajoIDStr *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error)`
- `(scraperService).mapearTrabajoDTO(t *infrastructure.TrabajoScraper, cadenaNombre string) *domain.TrabajoScraperDTO`

### `tests/auth_handler_test.go` (package tests)

- `TestAuthHandler_RegistrarUsuario_AutoLogin(t *testing.T)`
- `TestAuthHandler_ActualizarPerfil(t *testing.T)`

### `tests/auth_service_test.go` (package tests)

- Structs: mockUsuarioRepository
- `newMockUsuarioRepository() *mockUsuarioRepository`
- `(mockUsuarioRepository).FindByEmail(ctx context.Context, email string) (*infrastructure.Usuario, error)`
- `(mockUsuarioRepository).FindByID(ctx context.Context, id string) (*infrastructure.Usuario, error)`
- `(mockUsuarioRepository).Create(ctx context.Context, usuario *infrastructure.Usuario) error`
- `(mockUsuarioRepository).Actualizar(ctx context.Context, id string, datos map[string]interface{}) error`
- `TestAuthService_PasswordComplexity(t *testing.T)`
- `TestAuthService_RegistroExitoso(t *testing.T)`
- `TestAuthService_Login(t *testing.T)`
- `TestAuthService_ActualizarPerfil(t *testing.T)`

### `tests/jwt_test.go` (package tests)

- `TestJWT_GenerarYValidarToken(t *testing.T)`
- `TestJWT_TokenInvalido(t *testing.T)`
- `TestMiddleware_RequireAuth(t *testing.T)`
- `init()`

### `tests/middleware_test.go` (package tests)

- `init()`
- `setupTestRouter() *gin.Engine`
- `TestNotFoundHandler(t *testing.T)`
- `TestMethodNotAllowedHandler(t *testing.T)`
- `TestPanicRecoveryHandler(t *testing.T)`
- `TestResponderError(t *testing.T)`

### `tests/producto_handler_test.go` (package tests)

- `TestProductoHandler_ObtenerProductos(t *testing.T)`
- `TestProductoHandler_BuscarProductos(t *testing.T)`
- `TestProductoHandler_ObtenerDetalleProducto(t *testing.T)`
- `TestProductoHandler_ObtenerProductosAdmin(t *testing.T)`

### `tests/producto_service_test.go` (package tests)

- Structs: mockProductoRepository
- `(mockProductoRepository).Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error)`
- `(mockProductoRepository).ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error)`
- `(mockProductoRepository).ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error)`
- `TestProductoService_ValoresPorDefecto(t *testing.T)`
- `TestProductoService_SanitizacionYLimites(t *testing.T)`
- `TestProductoService_Busqueda_Minimo3Caracteres(t *testing.T)`
- `TestProductoService_ObtenerPorID(t *testing.T)`
- `TestProductoService_ObtenerCatalogoAdmin(t *testing.T)`

### `tests/rbac_test.go` (package tests)

- `TestRequireRole(t *testing.T)`

### `tests/scraper_test.go` (package tests)

- Structs: mockScraperRepository
- `newMockScraperRepository() *mockScraperRepository`
- `(mockScraperRepository).CrearTrabajo(ctx context.Context, trabajo *infrastructure.TrabajoScraper) error`
- `(mockScraperRepository).ObtenerTrabajoPorID(ctx context.Context, id uuid.UUID) (*infrastructure.TrabajoScraper, error)`
- `(mockScraperRepository).FinalizarTrabajo(ctx context.Context, id uuid.UUID, estado string, elementosExtraidos *int, registroErrores *string) (*infrastructure.TrabajoScraper, error)`
- `(mockScraperRepository).ObtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*infrastructure.CadenaSupermercado, error)`
- `(mockScraperRepository).ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*infrastructure.SucursalSupermercado, error)`
- `(mockScraperRepository).IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error)`
- `TestScraperService_IniciarTrabajo_Exitoso(t *testing.T)`
- `TestScraperService_FinalizarTrabajo_Validaciones(t *testing.T)`
- `TestScraperService_IngestarProductos_LimitesYValidaciones(t *testing.T)`
- `TestScraperHandler_EndpointsHTTP(t *testing.T)`
- `stringPtr(s string) *string`

### `utils/jwt.go` (package utils)

- Structs: JWTClaims
- `getJWTSecret() []byte`
- `GenerarToken(usuarioID, rol, provider string) (string, error)`
- `ValidarToken(tokenString string) (*jwt.Token, error)`

### `utils/security.go` (package utils)

- `SanitizarInputBusqueda(input string) string`
- `HashPassword(password string) (string, error)`
- `CheckPasswordHash(password, hash string) bool`


## Mapa auto-generado: IA Conversacional (FastAPI)

**17 archivos .py** detectados


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

- Imports internos: app.explicacion, app.filtros, app.productos, app.proveedores.base, app.schemas
- `crear_router_chat(repositorio: ProductoRepositorio, proveedor_groq: ProveedorIA, proveedor_gemini: ProveedorIA) → APIRouter`

### `app/routers/productos.py`

- Imports internos: app.productos
- `crear_router_productos(repositorio: ProductoRepositorio) → APIRouter`

### `app/schemas.py`

- **class ChatRequest(BaseModel)**

### `app/utils.py`

- **class RespuestaUTF8(JSONResponse)**
- `fix_encoding_consola() → ?`


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

**24 archivos .py** detectados


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

### `research/test_santa_isabel.py`

- **class SantaIsabelExtractionTest(unittest.TestCase)**
  - `_response(payload, page)`
  - `test_extracts_santa_isabel_render_data()`
  - `test_category_pagination_preserves_category()`
  - `test_spider_obeys_robots_txt()`
  - `test_only_uses_santa_isabel_seller()`

### `scraper_core/api_client.py`

- **class ScraperAPIError(RuntimeError)**
- **class ScraperAPIClient**
  - `__init__(endpoint: str, timeout: float, opener)`
  - `ingest_batch(products: ?, supermarket: ?, branch_id: ?, branch_code: ?, work_id: ?) → dict`
  - `finalize_work(work_id: str, status: str, extracted_items: int, error_message: ?) → dict`
  - `_ingestion_endpoint(work_id: ?) → str`

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

### `scraper_core/normalization.py`

- `normalizar_precio_clp(value)`
- `_uses_thousands_groups(parts)`
- `normalizar_ean_gtin(value)`
- `normalizar_texto(value)`

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
  - `__call__(job)`
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

- `process_job(job: dict, api_client: ScraperAPIClient) → dict`
- `run_worker()`


## Mapa auto-generado: Frontend (React + TypeScript)

**77 archivos activos** (excluidos kebab-case obsoletos)


### `core/routes.ts`

- Exports: MOCK_ROUTES, ROUTES

### `data/mock.ts`

- Exports: HOME, discountPct, formatPrice, mockUser, products, supermarketById, supermarkets
- Imports locales: ../types

### `hooks/useAuth.ts`

- Exports: useAuth
- Imports locales: ../contexts/AuthContext

### `hooks/useCatalog.ts`

- Exports: useCatalog
- Imports locales: @/lib/constants, @/lib/products-api, @/types

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

- Exports: toUiProduct
- Tipos: ApiPriceHistory, ApiProduct, ApiProductDetail, ProductPage
- Imports locales: ./constants, @/types

### `lib/utils.ts`

- Exports: cn
- Tipos: ClassValue

### `types/index.ts`

- Tipos: CadenaSupermercado, CapturaPrecio, Categoria, LoginPayload, Marca, PreferenciasDieteticas, ProductoCrudo, ProductoNormalizado, RegisterPayload, SucursalSupermercado, UiProduct, UiSupermarket, Usuario

### `app/router.tsx`

- Exports: AppRouter
- Imports locales: @/components/ErrorBoundary, @/components/auth/ProtectedRoute, @/contexts/AuthContext, @/contexts/CartContext, @/contexts/ToastContext, @/core/routes, @/layouts/MainLayout, @/pages/Chatbot, @/pages/Crowdsourcing, @/pages/Dashboard, @/pages/History, @/pages/Home, @/pages/Login, @/pages/NotFound, @/pages/Onboarding, @/pages/Planes, @/pages/ProductDetail, @/pages/Profile, @/pages/RouteViewer, @/pages/Terms, @/pages/mocks/MockShell

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

