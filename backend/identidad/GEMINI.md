# Servicio Identidad

Entry `cmd/identidad/main.go`, puerto 8080. Módulo y proceso propios; no importar
API/Catálogo ni agregar bibliotecas compartidas con sus entidades. JWT/cookies
y rutas públicas conservados. Ver `docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

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
