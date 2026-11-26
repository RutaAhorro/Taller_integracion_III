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
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/idtoken"
	"gorm.io/gorm"
)

// ===== Router =====

// NuevoEnrutador monta sondas, middlewares globales y las rutas del módulo. Cada
// petición protegida valida JWT localmente; no acepta identidad de headers.
func NuevoEnrutador(configuracion config.Configuracion, modulo *Modulo, redisCliente *redis.Client, sondaBase func(context.Context) error, registrador *slog.Logger) *gin.Engine {
	ajustes := configuracion.Ajustes
	enrutador := gin.New()
	_ = enrutador.SetTrustedProxies(configuracion.ProxiesConfiables)
	enrutador.RemoteIPHeaders = []string{"X-Forwarded-For"}
	enrutador.Use(observar(registrador), ManejarErrores(), cors(configuracion.OrigenesPermitidos), acotarPeticion(ajustes))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(RutaNoEncontrada())
	enrutador.NoMethod(MetodoNoPermitido())
	enrutador.GET("/_identidad/live", func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{"status": "ok", "service": "identidad"})
	})
	enrutador.GET("/_identidad/ready", func(contexto *gin.Context) {
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.PlazoReadiness)
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
	modulo.RegistrarRutas(enrutador.Group("/api/v1"))
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
				ResponderError(contexto, http.StatusForbidden, "Origen no permitido.", nil)
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origen)
			contexto.Header("Access-Control-Allow-Credentials", "true")
			contexto.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-CSRF-Token")
			contexto.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if contexto.Request.Method == http.MethodOptions {
			contexto.AbortWithStatus(http.StatusNoContent)
			return
		}
		contexto.Next()
	}
}

// patronRequestID acepta un X-Request-ID recibido sólo si es corto y seguro
// para escribirlo en logs; si no, se genera uno nuevo.
var patronRequestID = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

// observar propaga o genera X-Request-ID y registra método, ruta de plantilla,
// estado y duración, sin cuerpos ni cookies.
func observar(registrador *slog.Logger) gin.HandlerFunc {
	if registrador == nil {
		registrador = slog.Default()
	}
	return func(contexto *gin.Context) {
		id := contexto.GetHeader("X-Request-ID")
		if !patronRequestID.MatchString(id) {
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

// acotarPeticion acota la duración y el tamaño del cuerpo de cada petición.
func acotarPeticion(ajustes config.Ajustes) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contextoOperacion, cancelar := context.WithTimeout(contexto.Request.Context(), ajustes.PlazoPeticion)
		defer cancelar()
		contexto.Request = contexto.Request.WithContext(contextoOperacion)
		contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(ajustes.MaximoBytesCuerpo))
		contexto.Next()
	}
}

// ===== Módulo =====

// Modulo arma Identidad completa (repositorios, servicios y handlers) y es lo
// único que main ve del servicio.
type Modulo struct {
	handler       *ManejadorAuth
	restablecer   *ManejadorRestablecer
	adminHandler  *ManejadorAdmin
	direcciones   *ManejadorDirecciones
	repo          domain.RepositorioUsuarios
	redis         *redis.Client
	configuracion config.Configuracion
}

// NuevoModulo arma el módulo con una instantánea validada del arranque.
func NuevoModulo(base *gorm.DB, redisCliente *redis.Client, configuracion config.Configuracion) *Modulo {
	ajustes := configuracion.Ajustes
	repositorio := repositories.NuevoRepositorioUsuarios(base)
	repositorioAdmin := repositories.NuevoRepositorioAdmin(base)
	repositorioRecuperacion := repositories.NuevoRepositorioRecuperacion(base)
	return &Modulo{
		handler:       NuevoManejadorAuth(services.NuevoServicioAuth(repositorio, idtoken.Validate, configuracion.ClienteGoogle, ajustes.PlazoGoogle, configuracion.CorreosAdmin), configuracion.CookieSegura, configuracion.ClaveJWT),
		restablecer:   NuevoManejadorRestablecer(services.NuevoServicioRecuperacion(repositorioRecuperacion)),
		adminHandler:  NuevoManejadorAdmin(services.NuevoServicioAdmin(repositorioAdmin)),
		direcciones:   NuevoManejadorDirecciones(services.NuevoServicioDirecciones(repositories.NuevoRepositorioDirecciones(base))),
		repo:          repositorio,
		redis:         redisCliente,
		configuracion: configuracion,
	}
}

// RegistrarRutas monta /auth/*, /admin/usuarios/* y /usuarios/me/direcciones/* bajo group.
func (modulo *Modulo) RegistrarRutas(grupo *gin.RouterGroup) {
	RegistrarRutasAuth(grupo, modulo.handler, modulo.restablecer, modulo.redis, modulo.repo, modulo.configuracion)
	RegistrarRutasAdminUsuarios(grupo, modulo.adminHandler, modulo.repo, modulo.configuracion.ClaveJWT)
	RegistrarRutasDirecciones(grupo, modulo.direcciones, modulo.repo, modulo.configuracion.ClaveJWT)
}

// ===== Autenticación y perfil =====

// RegistrarRutasAuth monta /auth/*. Registro, login, Google y restablecimiento
// llevan contador por IP; las rutas de perfil revalidan la cuenta en api.usuarios
// además del JWT. El restablecimiento es público: lo autoriza el token de un solo uso.
func RegistrarRutasAuth(grupo *gin.RouterGroup, manejador *ManejadorAuth, manejadorRestablecer *ManejadorRestablecer, redisCliente *redis.Client, repositorio domain.RepositorioUsuarios, configuracion config.Configuracion) {
	ajustes := configuracion.Ajustes
	sesion := ExigirSesion(repositorio, configuracion.ClaveJWT)
	auth := grupo.Group("/auth")
	auth.POST("/register", LimitarRegistro(redisCliente, ajustes), manejador.RegistrarUsuario)
	auth.POST("/login", LimitarLogin(redisCliente, ajustes), manejador.LoginUsuario)
	auth.POST("/google", LimitarGoogle(redisCliente, ajustes), manejador.LoginGoogleUsuario)
	auth.POST("/restablecer", LimitarRestablecer(redisCliente, ajustes), manejadorRestablecer.Restablecer)
	auth.POST("/logout", manejador.CerrarSesion)
	auth.GET("/me", sesion, manejador.PerfilUsuario)
	auth.PUT("/me", sesion, manejador.ActualizarPerfil)
}

// ===== Administración de cuentas =====

// RegistrarRutasAdminUsuarios monta /admin/usuarios. Toda petición exige
// sesión JWT vigente y rol admin.
func RegistrarRutasAdminUsuarios(grupo *gin.RouterGroup, manejador *ManejadorAdmin, repositorio domain.RepositorioUsuarios, claveJWT string) {
	admin := grupo.Group("/admin/usuarios", ExigirSesion(repositorio, claveJWT), ExigirRol(domain.RolAdmin))
	admin.GET("", manejador.ListarUsuarios)
	admin.GET("/metricas", manejador.Metricas)
	admin.PUT("/:id", manejador.ActualizarUsuario)
}

// ===== Direcciones del usuario =====

// RegistrarRutasDirecciones monta /usuarios/me/direcciones. Toda petición exige
// sesión JWT vigente; sólo opera sobre las direcciones del usuario de la sesión.
// Se actualiza con PUT (reemplazo de los campos editables), igual que PUT /auth/me.
func RegistrarRutasDirecciones(grupo *gin.RouterGroup, manejador *ManejadorDirecciones, repositorio domain.RepositorioUsuarios, claveJWT string) {
	direcciones := grupo.Group("/usuarios/me/direcciones", ExigirSesion(repositorio, claveJWT))
	direcciones.GET("", manejador.ListarDirecciones)
	direcciones.POST("", manejador.CrearDireccion)
	direcciones.PUT("/:id", manejador.ActualizarDireccion)
	direcciones.DELETE("/:id", manejador.EliminarDireccion)
}
