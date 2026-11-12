package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/ingesta"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// NewRouter compone sólo las rutas de Catálogo. La verificación de JWT y rol
// sucede en este servicio, incluso cuando se accede directamente sin Gateway.
func NewRouter(service services.ProductoService, fuentes services.FuenteService, secret string, ping func(context.Context) error, logger *slog.Logger, allowedOrigins []string, internal ...ingesta.Config) *gin.Engine {
	enrutador := gin.New()
	_ = enrutador.SetTrustedProxies(nil)
	enrutador.Use(observe(logger), middleware.ErrorHandler(logger), cors(allowedOrigins))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(middleware.NotFoundHandler())
	enrutador.NoMethod(middleware.MethodNotAllowedHandler())

	enrutador.GET("/_catalogo/live", func(contexto *gin.Context) {
		contexto.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo"})
	})
	ready := func(contexto *gin.Context) {
		ctx, cancel := context.WithTimeout(contexto.Request.Context(), 2*time.Second)
		defer cancel()
		if ping == nil || ping(ctx) != nil {
			contexto.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "service": "catalogo", "checks": gin.H{"database": "error"}})
			return
		}
		contexto.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo", "checks": gin.H{"database": "ok"}})
	}
	enrutador.GET("/_catalogo/ready", ready)
	enrutador.GET("/health", ready)

	handler := handlers.NewProductoHandler(service)
	fuenteHandler := handlers.NewFuenteHandler(fuentes)
	v1 := enrutador.Group("/api/v1")
	v1.GET("/productos", handler.ObtenerProductos)
	v1.GET("/productos/buscar", handler.BuscarProductos)
	v1.GET("/productos/:id", handler.ObtenerDetalleProducto)
	admin := v1.Group("/admin/productos", middleware.RequireAuth(secret), middleware.RequireRole("admin"))
	admin.GET("", handler.ObtenerProductosAdmin)
	admin.GET("/", handler.ObtenerProductosAdmin)
	admin.GET("/fuentes", fuenteHandler.ObtenerFuentes)
	admin.PUT("/fuentes/:id", fuenteHandler.ActualizarFuente)
	if len(internal) > 0 {
		ingesta.Register(enrutador, internal[0], logger)
	}
	return enrutador
}

func cors(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" && origin != "*" && origin != "null" {
			allowed[origin] = true
		}
	}
	return func(contexto *gin.Context) {
		contexto.Writer.Header().Add("Vary", "Origin")
		origin := contexto.Request.Header.Get("Origin")
		if origin != "" {
			if !allowed[origin] {
				middleware.ResponderError(contexto, http.StatusForbidden, "Origen no permitido.", nil)
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origin)
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

var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func observe(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(contexto *gin.Context) {
		requestID := contexto.GetHeader("X-Request-ID")
		if !validRequestID.MatchString(requestID) {
			var value [16]byte
			_, _ = rand.Read(value[:])
			requestID = hex.EncodeToString(value[:])
		}
		contexto.Header("X-Request-ID", requestID)
		start := time.Now()
		contexto.Next()
		path := contexto.FullPath()
		if path == "" {
			path = "unmatched"
		}
		logger.Info("http_request", "request_id", requestID, "method", contexto.Request.Method,
			"path", path, "status", contexto.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}
