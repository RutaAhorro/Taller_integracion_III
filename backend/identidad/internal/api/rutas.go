// Package api es la capa HTTP de Identidad: router, middlewares, sesión JWT,
// handlers y formato de errores. Traduce HTTP a casos de uso de services y no
// contiene reglas de negocio.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/idtoken"
	"gorm.io/gorm"
)

// ===== Router =====

// NewRouter monta sondas, middlewares globales y las rutas del módulo. Cada
// petición protegida valida JWT localmente; no acepta identidad de headers.
// Recibe el módulo ya armado para que main pueda cerrarlo al apagar
// (despacho pendiente de correos de recuperación, SUP-184).
func NewRouter(configuracion config.Config, modulo *Module, redisCliente *redis.Client, sondaBase func(context.Context) error, registrador *slog.Logger) *gin.Engine {
	ajustes := configuracion.Ajustes
	enrutador := gin.New()
	_ = enrutador.SetTrustedProxies(configuracion.TrustedProxies)
	enrutador.RemoteIPHeaders = []string{"X-Forwarded-For"}
	enrutador.Use(observe(registrador), ErrorHandler(), cors(configuracion.AllowedOrigins), boundedRequest(ajustes))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(NotFoundHandler())
	enrutador.NoMethod(MethodNotAllowedHandler())
	enrutador.GET("/_identidad/live", func(contexto *gin.Context) { contexto.JSON(200, gin.H{"status": "ok", "service": "identidad"}) })
	enrutador.GET("/_identidad/ready", func(contexto *gin.Context) {
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.ReadinessTimeout)
		defer cancelar()
		verificaciones := gin.H{"database": "error", "redis": "error"}
		// Login falla cerrado sin Redis: readiness debe reflejar esa dependencia.
		if sondaBase != nil && sondaBase(contextoOperacion) == nil {
			verificaciones["database"] = "ok"
		}
		if redisCliente != nil && redisCliente.Ping(contextoOperacion).Err() == nil {
			verificaciones["redis"] = "ok"
		}
		codigo, estado := http.StatusOK, "ok"
		if verificaciones["database"] != "ok" || verificaciones["redis"] != "ok" {
			codigo, estado = http.StatusServiceUnavailable, "error"
		}
		contexto.JSON(codigo, gin.H{"status": estado, "service": "identidad", "checks": verificaciones})
	})
	modulo.RegisterRoutes(enrutador.Group("/api/v1"))
	return enrutador
}

// ===== Middlewares globales =====

// cors rechaza con 403 los orígenes fuera de la allowlist antes de ejecutar
// handlers y sólo concede credenciales a los orígenes exactos permitidos.
func cors(origenes []string) gin.HandlerFunc {
	permitidos := map[string]bool{}
	for _, origen := range origenes {
		if origen != "" && origen != "*" && origen != "null" {
			permitidos[origen] = true
		}
	}
	return func(contexto *gin.Context) {
		contexto.Writer.Header().Add("Vary", "Origin")
		origen := contexto.GetHeader("Origin")
		if origen != "" {
			if !permitidos[origen] {
				ResponderError(contexto, 403, "Origen no permitido.", nil)
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origen)
			contexto.Header("Access-Control-Allow-Credentials", "true")
			contexto.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-CSRF-Token")
			contexto.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		}
		if contexto.Request.Method == http.MethodOptions {
			contexto.AbortWithStatus(204)
			return
		}
		contexto.Next()
	}
}

// requestIDPattern acepta un X-Request-ID recibido sólo si es corto y seguro
// para escribirlo en logs; si no, se genera uno nuevo.
var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

// observe propaga o genera X-Request-ID y registra método, ruta de plantilla,
// estado y duración, sin cuerpos ni cookies.
func observe(registrador *slog.Logger) gin.HandlerFunc {
	if registrador == nil {
		registrador = slog.Default()
	}
	return func(contexto *gin.Context) {
		id := contexto.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			var token [16]byte
			_, _ = rand.Read(token[:])
			id = hex.EncodeToString(token[:])
		}
		contexto.Header("X-Request-ID", id)
		contexto.Set("request_id", id)
		inicio := time.Now()
		contexto.Next()
		ruta := contexto.FullPath()
		if ruta == "" {
			ruta = "unmatched"
		}
		registrador.Info("http_request", "request_id", id, "method", contexto.Request.Method, "path", ruta, "status", contexto.Writer.Status(), "duration_ms", time.Since(inicio).Milliseconds())
	}
}

// boundedRequest acota la duración y el tamaño del cuerpo de cada petición.
func boundedRequest(ajustes config.Ajustes) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.RequestTimeout)
		defer cancelar()
		contexto.Request = contexto.Request.WithContext(contextoOperacion)
		contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(ajustes.MaxBodyBytes))
		contexto.Next()
	}
}

// ===== Módulo =====

// Module arma Identidad completa (repositorios, servicios y handlers) y es lo
// único que main ve del servicio.
type Module struct {
	handler       *AuthHandler
	adminHandler  *AdminUsuarioHandler
	repo          domain.UsuarioRepository
	redis         *redis.Client
	configuracion config.Config
	// recuperacion emite tokens y despacha su correo (SUP-184). El endpoint que la
	// expone (POST /auth/recuperar) es SUP-233.
	recuperacion services.RecuperacionService
}

// Recuperacion configura SUP-184. Sin Correo o sin URLRestablecer queda apagada:
// Solicitar sólo valida el formato y no emite tokens ni correos.
type Recuperacion struct {
	Correo         mailer.Mailer
	URLRestablecer string
	Log            *slog.Logger
}

// NewModule arma el módulo con una instantánea validada del arranque.
func NewModule(base *gorm.DB, redisCliente *redis.Client, recuperacion Recuperacion, configuracion config.Config) *Module {
	ajustes := configuracion.Ajustes
	repositorio := repositories.NewUsuarioRepository(base)
	repositorioAdmin := repositories.NewAdminUsuarioRepository(base)
	return &Module{
		handler:       NewAuthHandler(services.NewAuthService(repositorio, idtoken.Validate, configuracion.GoogleClientID, ajustes.GoogleTimeout), configuracion.CookieSecure, configuracion.JWTSecret),
		adminHandler:  NewAdminUsuarioHandler(services.NewAdminUsuarioService(repositorioAdmin)),
		repo:          repositorio,
		redis:         redisCliente,
		configuracion: configuracion,
		recuperacion:  services.NewRecuperacionService(repositorio, repositories.NewRecuperacionRepository(base), recuperacion.Correo, recuperacion.URLRestablecer, recuperacion.Log, ajustes),
	}
}

// Cerrar despacha las solicitudes de recuperación pendientes antes de apagar el proceso.
func (modulo *Module) Cerrar(contexto context.Context) error {
	return modulo.recuperacion.Cerrar(contexto)
}

// RegisterRoutes monta /auth/* y /admin/usuarios/* bajo group.
func (modulo *Module) RegisterRoutes(grupo *gin.RouterGroup) {
	RegistrarRutasAuth(grupo, modulo.handler, modulo.redis, modulo.repo, modulo.configuracion)
	RegistrarRutasAdminUsuarios(grupo, modulo.adminHandler, modulo.repo, modulo.configuracion.JWTSecret)
}

// ===== Autenticación y perfil =====

// RegistrarRutasAuth monta /auth/*. Registro, login y Google llevan contador
// por IP; las rutas de perfil revalidan la cuenta en api.usuarios además del JWT.
func RegistrarRutasAuth(grupo *gin.RouterGroup, manejador *AuthHandler, redisCliente *redis.Client, repositorio domain.UsuarioRepository, configuracion config.Config) {
	ajustes := configuracion.Ajustes
	sesion := RequireAuth(repositorio, configuracion.JWTSecret)
	auth := grupo.Group("/auth")
	auth.POST("/register", RateLimitRegistro(redisCliente, ajustes), manejador.RegistrarUsuario)
	auth.POST("/login", RateLimitLogin(redisCliente, ajustes), manejador.LoginUsuario)
	auth.POST("/google", RateLimitGoogle(redisCliente, ajustes), manejador.GoogleLoginUsuario)
	auth.POST("/logout", manejador.CerrarSesion)
	auth.GET("/me", sesion, manejador.PerfilUsuario)
	auth.PUT("/me", sesion, manejador.ActualizarPerfil)
}

// ===== Administración de cuentas =====

// RegistrarRutasAdminUsuarios monta /admin/usuarios. Toda petición exige
// sesión JWT vigente y rol admin.
func RegistrarRutasAdminUsuarios(grupo *gin.RouterGroup, manejador *AdminUsuarioHandler, repositorio domain.UsuarioRepository, claveJWT string) {
	admin := grupo.Group("/admin/usuarios", RequireAuth(repositorio, claveJWT), RequireRole(domain.RolAdmin))
	admin.GET("", manejador.ListarUsuarios)
	admin.GET("/metricas", manejador.Metricas)
	admin.PUT("/:id", manejador.ActualizarUsuario)
}
