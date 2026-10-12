package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

// Cada petición protegida valida JWT localmente. No acepta identidad de headers.
func NewRouter(cfg config.Config, db *gorm.DB, redisClient *redis.Client, ping func(context.Context) error, log *slog.Logger) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.RemoteIPHeaders = []string{"X-Forwarded-For"}
	r.Use(observe(log), httperrors.ErrorHandler(), cors(cfg.AllowedOrigins), boundedRequest())
	r.HandleMethodNotAllowed = true
	r.NoRoute(httperrors.NotFoundHandler())
	r.NoMethod(httperrors.MethodNotAllowedHandler())
	r.GET("/_identidad/live", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "identidad"}) })
	r.GET("/_identidad/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
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
		c.JSON(code, gin.H{"status": state, "service": "identidad", "checks": checks})
	})
	identity.New(db, redisClient, cfg.CookieSecure).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func cors(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		if origin != "" && origin != "*" && origin != "null" {
			allowed[origin] = true
		}
	}
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin != "" {
			if !allowed[origin] {
				httperrors.ResponderError(c, 403, "Origen no permitido.", nil)
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-CSRF-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func observe(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			var token [16]byte
			_, _ = rand.Read(token[:])
			id = hex.EncodeToString(token[:])
		}
		c.Header("X-Request-ID", id)
		c.Set("request_id", id)
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		log.Info("http_request", "request_id", id, "method", c.Request.Method, "path", path, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}

func boundedRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
		c.Next()
	}
}
