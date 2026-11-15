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
		contexto, cancelar := context.WithTimeout(contextoHTTP.Request.Context(), 65*time.Second)
		defer cancelar()
		contextoHTTP.Request = contextoHTTP.Request.WithContext(contexto)
		contextoHTTP.Request.Body = http.MaxBytesReader(contextoHTTP.Writer, contextoHTTP.Request.Body, 8<<20)
		contextoHTTP.Next()
	}
}

func CORS(origenes []string) gin.HandlerFunc {
	permitido := map[string]bool{}
	for _, origen := range origenes {
		if origen != "" && origen != "*" && origen != "null" {
			permitido[origen] = true
		}
	}
	return func(contextoHTTP *gin.Context) {
		contextoHTTP.Writer.Header().Add("Vary", "Origin")
		origen := contextoHTTP.GetHeader("Origin")
		if origen != "" {
			if !permitido[origen] {
				ResponderError(contextoHTTP, 403, "Origen no permitido.", nil)
				return
			}
			contextoHTTP.Header("Access-Control-Allow-Origin", origen)
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

var patronIDPeticion = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

func Observe(registrador *slog.Logger) gin.HandlerFunc {
	if registrador == nil {
		registrador = slog.Default()
	}
	return func(contextoHTTP *gin.Context) {
		identificador := contextoHTTP.GetHeader("X-Request-ID")
		if !patronIDPeticion.MatchString(identificador) {
			var token [16]byte
			_, _ = rand.Read(token[:])
			identificador = hex.EncodeToString(token[:])
		}
		contextoHTTP.Header("X-Request-ID", identificador)
		contextoHTTP.Set("request_id", identificador)
		inicio := time.Now()
		contextoHTTP.Next()
		ruta := contextoHTTP.FullPath()
		if ruta == "" {
			ruta = "unmatched"
		}
		registrador.Info("http_request", "request_id", identificador, "method", contextoHTTP.Request.Method, "path", ruta, "status", contextoHTTP.Writer.Status(), "duration_ms", time.Since(inicio).Milliseconds())
	}
}
