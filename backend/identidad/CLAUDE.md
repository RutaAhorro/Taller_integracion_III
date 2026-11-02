# Servicio Identidad

Entry `cmd/identidad/main.go`, puerto 8080. Módulo y proceso propios; no importar
API/Catálogo ni agregar bibliotecas compartidas con sus entidades. JWT/cookies
y rutas públicas conservados. Ver `docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

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
