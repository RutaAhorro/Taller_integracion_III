// Package api es la capa HTTP de Catálogo: router, sondas, CORS, correlación,
// sesión JWT, rutas internas con token de servicio, handlers y formato de
// errores. Traduce HTTP a services y domain; no contiene reglas de negocio.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== Dependencias =====

// Dependencias reúne lo que el router necesita, ya construido y validado en
// main. Un token vacío deja sin registrar sus rutas internas.
type Dependencias struct {
	Productos          *services.ServicioProductos
	Fuentes            *services.ServicioFuentes
	Disponibilidad     *services.ServicioDisponibilidad
	Ingesta            domain.RepositorioIngesta
	ClaveJWT           string
	TokenIngesta       string
	TokenConsulta      string
	OrigenesPermitidos []string
	SondaBase          func(context.Context) error
	Registrador        *slog.Logger

	// Límites configurables de las rutas (internal/config).
	PlazoReadiness         time.Duration
	MaximoProductosPorLote int
	MaximoBytesCuerpo      int
}

// ===== Router =====

// NuevoEnrutador compone las rutas de Catálogo. JWT y rol se verifican en este
// servicio aunque se acceda sin Gateway; /internal/v1 nunca se publica en él.
func NuevoEnrutador(dependencias Dependencias) *gin.Engine {
	enrutador := gin.New()
	_ = enrutador.SetTrustedProxies(nil)
	enrutador.Use(observar(dependencias.Registrador), manejarErrores(dependencias.Registrador), cors(dependencias.OrigenesPermitidos))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(rutaNoEncontrada)
	enrutador.NoMethod(metodoNoPermitido)

	enrutador.GET("/_catalogo/live", func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo"})
	})
	listo := sondaReadiness(dependencias.SondaBase, dependencias.PlazoReadiness)
	enrutador.GET("/_catalogo/ready", listo)
	enrutador.GET("/health", listo)

	productos := manejadorProductos{servicio: dependencias.Productos}
	fuentes := manejadorFuentes{servicio: dependencias.Fuentes}
	publica := enrutador.Group("/api/v1")
	publica.GET("/productos", productos.listar)
	publica.GET("/productos/buscar", productos.buscar)
	publica.GET("/productos/categorias", productos.categorias)
	publica.GET("/productos/:id", productos.detalle)
	administracion := publica.Group("/admin/productos", exigirAdministrador(dependencias.ClaveJWT))
	administracion.GET("", productos.listarAdmin)
	administracion.GET("/", productos.listarAdmin)
	administracion.GET("/fuentes", fuentes.listar)
	administracion.PUT("/fuentes/:id", fuentes.actualizar)

	if dependencias.TokenIngesta != "" {
		registrarIngesta(enrutador.Group("/internal/v1", exigirTokenServicio(dependencias.TokenIngesta)), dependencias)
	}
	if dependencias.TokenConsulta != "" {
		registrarDisponibilidad(enrutador.Group("/internal/v1", exigirTokenServicio(dependencias.TokenConsulta)), dependencias)
	}
	return enrutador
}

// sondaReadiness responde 503 si PostgreSQL no contesta dentro del plazo.
func sondaReadiness(sondaBase func(context.Context) error, plazo time.Duration) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contextoSonda, cancelar := context.WithTimeout(contexto.Request.Context(), plazo)
		defer cancelar()
		if sondaBase(contextoSonda) != nil {
			contexto.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "service": "catalogo", "checks": gin.H{"database": "error"}})
			return
		}
		contexto.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo", "checks": gin.H{"database": "ok"}})
	}
}

// ===== CORS =====

// cors sólo refleja orígenes exactos de la lista y rechaza con 403 cualquier
// otro Origin, para que un sitio ajeno no use la cookie de sesión.
func cors(origenesPermitidos []string) gin.HandlerFunc {
	permitidos := make(map[string]bool, len(origenesPermitidos))
	for _, origen := range origenesPermitidos {
		permitidos[origen] = true
	}
	return func(contexto *gin.Context) {
		contexto.Writer.Header().Add("Vary", "Origin")
		origen := contexto.Request.Header.Get("Origin")
		if origen != "" {
			if !permitidos[origen] {
				responderError(contexto, http.StatusForbidden, "Origen no permitido.", "")
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origen)
			contexto.Header("Access-Control-Allow-Credentials", "true")
			contexto.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			contexto.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		}
		if contexto.Request.Method == http.MethodOptions {
			contexto.AbortWithStatus(http.StatusNoContent)
			return
		}
		contexto.Next()
	}
}

// ===== Correlación y registro =====

// bytesIdentificadorPeticion genera 32 caracteres hexadecimales.
const bytesIdentificadorPeticion = 16

// identificadorPeticionValido acepta el X-Request-ID de Gateway o Control sólo
// si no puede inyectar contenido en los logs.
var identificadorPeticionValido = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

// observar propaga o genera X-Request-ID y registra una línea por petición con
// la ruta registrada, nunca la URL con parámetros.
func observar(registrador *slog.Logger) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		identificador := contexto.GetHeader("X-Request-ID")
		if !identificadorPeticionValido.MatchString(identificador) {
			var aleatorio [bytesIdentificadorPeticion]byte
			_, _ = rand.Read(aleatorio[:])
			identificador = hex.EncodeToString(aleatorio[:])
		}
		contexto.Header("X-Request-ID", identificador)
		inicio := time.Now()
		contexto.Next()
		registrador.Info("http_request", "request_id", identificador, "method", contexto.Request.Method,
			"path", rutaRegistrada(contexto), "status", contexto.Writer.Status(), "duration_ms", time.Since(inicio).Milliseconds())
	}
}

// rutaRegistrada devuelve el patrón de la ruta o "unmatched" para 404.
func rutaRegistrada(contexto *gin.Context) string {
	if ruta := contexto.FullPath(); ruta != "" {
		return ruta
	}
	return "unmatched"
}
