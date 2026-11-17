package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

// NewRouter monta sondas, middlewares globales y las rutas del módulo. Cada
// petición protegida valida JWT localmente; no acepta identidad de headers.
// Recibe el módulo ya armado para que main pueda cerrarlo al apagar
// (despacho pendiente de correos de recuperación, SUP-184).
func NewRouter(cfg config.Config, modulo *Module, redisClient *redis.Client, ping func(context.Context) error, log *slog.Logger) *gin.Engine {
	ajustes := cfg.Ajustes
	enrutador := gin.New()
	_ = enrutador.SetTrustedProxies(cfg.TrustedProxies)
	enrutador.RemoteIPHeaders = []string{"X-Forwarded-For"}
	enrutador.Use(observe(log), httperrors.ErrorHandler(), cors(cfg.AllowedOrigins), boundedRequest(ajustes))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(httperrors.NotFoundHandler())
	enrutador.NoMethod(httperrors.MethodNotAllowedHandler())
	enrutador.GET("/_identidad/live", func(contexto *gin.Context) { contexto.JSON(200, gin.H{"status": "ok", "service": "identidad"}) })
	enrutador.GET("/_identidad/ready", func(contexto *gin.Context) {
		ctx, cancel := context.WithTimeout(contexto.Request.Context(), ajustes.ReadinessTimeout)
		defer cancel()
		checks := gin.H{"database": "error", "redis": "error"}
		// Login falla cerrado sin Redis: readiness debe reflejar esa dependencia.
		if ping != nil && ping(ctx) == nil {
			checks["database"] = "ok"
		}
		if redisClient != nil && redisClient.Ping(ctx).Err() == nil {
			checks["redis"] = "ok"
		}
		code, state := http.StatusOK, "ok"
		if checks["database"] != "ok" || checks["redis"] != "ok" {
			code, state = http.StatusServiceUnavailable, "error"
		}
		contexto.JSON(code, gin.H{"status": state, "service": "identidad", "checks": checks})
	})
	modulo.RegisterRoutes(enrutador.Group("/api/v1"))
	return enrutador
}

func cors(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		if origin != "" && origin != "*" && origin != "null" {
			allowed[origin] = true
		}
	}
	return func(contexto *gin.Context) {
		contexto.Writer.Header().Add("Vary", "Origin")
		origin := contexto.GetHeader("Origin")
		if origin != "" {
			if !allowed[origin] {
				httperrors.ResponderError(contexto, 403, "Origen no permitido.", nil)
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origin)
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

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func observe(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
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
		start := time.Now()
		contexto.Next()
		path := contexto.FullPath()
		if path == "" {
			path = "unmatched"
		}
		log.Info("http_request", "request_id", id, "method", contexto.Request.Method, "path", path, "status", contexto.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}

func boundedRequest(ajustes config.Ajustes) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		ctx, cancel := context.WithTimeout(contexto.Request.Context(), ajustes.RequestTimeout)
		defer cancel()
		contexto.Request = contexto.Request.WithContext(ctx)
		contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(ajustes.MaxBodyBytes))
		contexto.Next()
	}
}
