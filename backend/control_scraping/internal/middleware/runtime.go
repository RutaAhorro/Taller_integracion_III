package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

// Mismo máximo que Catálogo; no corta lotes válidos ni permite requests infinitos.
func BoundedRequest() gin.HandlerFunc {
	return func(contextoHTTP *gin.Context) {
		ctx, cancel := context.WithTimeout(contextoHTTP.Request.Context(), 65*time.Second)
		defer cancel()
		contextoHTTP.Request = contextoHTTP.Request.WithContext(ctx)
		contextoHTTP.Request.Body = http.MaxBytesReader(contextoHTTP.Writer, contextoHTTP.Request.Body, 8<<20)
		contextoHTTP.Next()
	}
}

func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		if origin != "" && origin != "*" && origin != "null" {
			allowed[origin] = true
		}
	}
	return func(contextoHTTP *gin.Context) {
		contextoHTTP.Writer.Header().Add("Vary", "Origin")
		origin := contextoHTTP.GetHeader("Origin")
		if origin != "" {
			if !allowed[origin] {
				ResponderError(contextoHTTP, 403, "Origen no permitido.", nil)
				return
			}
			contextoHTTP.Header("Access-Control-Allow-Origin", origin)
			contextoHTTP.Header("Access-Control-Allow-Credentials", "true")
			contextoHTTP.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-CSRF-Token, Idempotency-Key")
			contextoHTTP.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		}
		if contextoHTTP.Request.Method == http.MethodOptions {
			contextoHTTP.AbortWithStatus(204)
			return
		}
		contextoHTTP.Next()
	}
}

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func Observe(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(contextoHTTP *gin.Context) {
		id := contextoHTTP.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			var token [16]byte
			_, _ = rand.Read(token[:])
			id = hex.EncodeToString(token[:])
		}
		contextoHTTP.Header("X-Request-ID", id)
		contextoHTTP.Set("request_id", id)
		start := time.Now()
		contextoHTTP.Next()
		path := contextoHTTP.FullPath()
		if path == "" {
			path = "unmatched"
		}
		log.Info("http_request", "request_id", id, "method", contextoHTTP.Request.Method, "path", path, "status", contextoHTTP.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}
