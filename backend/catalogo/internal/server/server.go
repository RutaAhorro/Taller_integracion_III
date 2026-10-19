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
func NewRouter(service services.ProductoService, secret string, ping func(context.Context) error, logger *slog.Logger, allowedOrigins []string, internal ...ingesta.Config) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(observe(logger), middleware.ErrorHandler(logger), cors(allowedOrigins))
	r.HandleMethodNotAllowed = true
	r.NoRoute(middleware.NotFoundHandler())
	r.NoMethod(middleware.MethodNotAllowedHandler())

	r.GET("/_catalogo/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo"})
	})
	ready := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if ping == nil || ping(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "service": "catalogo", "checks": gin.H{"database": "error"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalogo", "checks": gin.H{"database": "ok"}})
	}
	r.GET("/_catalogo/ready", ready)
	r.GET("/health", ready)

	handler := handlers.NewProductoHandler(service)
	v1 := r.Group("/api/v1")
	v1.GET("/productos", handler.ObtenerProductos)
	v1.GET("/productos/buscar", handler.BuscarProductos)
	v1.GET("/productos/:id", handler.ObtenerDetalleProducto)
	admin := v1.Group("/admin/productos", middleware.RequireAuth(secret), middleware.RequireRole("admin"))
	admin.GET("", handler.ObtenerProductosAdmin)
	admin.GET("/", handler.ObtenerProductosAdmin)
	admin.GET("/fuentes", handler.ObtenerFuentes)
	admin.PUT("/fuentes/:id", handler.ActualizarFuente)
	if len(internal) > 0 {
		ingesta.Register(r, internal[0], logger)
	}
	return r
}

func cors(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" && origin != "*" && origin != "null" {
			allowed[origin] = true
		}
	}
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			if !allowed[origin] {
				middleware.ResponderError(c, http.StatusForbidden, "Origen no permitido.", nil)
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func observe(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !validRequestID.MatchString(requestID) {
			var value [16]byte
			_, _ = rand.Read(value[:])
			requestID = hex.EncodeToString(value[:])
		}
		c.Header("X-Request-ID", requestID)
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		logger.Info("http_request", "request_id", requestID, "method", c.Request.Method,
			"path", path, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}
