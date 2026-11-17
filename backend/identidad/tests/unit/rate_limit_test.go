// Contadores de intentos por IP sobre un Redis aislado (IDENTIDAD_TEST_REDIS_URL).
package unit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// W3 (#99): los logins exitosos no consumen intentos; los fallidos sí.
// Requiere Redis real: IDENTIDAD_TEST_REDIS_URL (p. ej. redis://localhost:6379/15).
func TestRateLimitLoginSoloCuentaFallidos(prueba *testing.T) {
	url := os.Getenv("IDENTIDAD_TEST_REDIS_URL")
	if url == "" {
		prueba.Skip("IDENTIDAD_TEST_REDIS_URL no definido")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		prueba.Fatal(err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := "rate_limit:login:192.0.2.10"
	if err := rdb.Del(ctx, key).Err(); err != nil {
		prueba.Skip("Redis no disponible:", err)
	}
	defer rdb.Del(context.Background(), key)

	gin.SetMode(gin.TestMode)
	status := http.StatusOK
	peticion := gin.New()
	peticion.POST("/login", middleware.RateLimitLogin(rdb, config.AjustesPredeterminados()), func(contexto *gin.Context) { contexto.Status(status) })
	login := func() int {
		req := httptest.NewRequest("POST", "/login", strings.NewReader("{}"))
		req.RemoteAddr = "192.0.2.10:1234"
		escritor := httptest.NewRecorder()
		peticion.ServeHTTP(escritor, req)
		return escritor.Code
	}
	for indice := 0; indice < 20; indice++ {
		if code := login(); code != http.StatusOK {
			prueba.Fatalf("login exitoso %d bloqueado con %d", indice+1, code)
		}
	}
	status = http.StatusUnauthorized
	for indice := 0; indice < 5; indice++ {
		if code := login(); code != http.StatusUnauthorized {
			prueba.Fatalf("fallo %d: %d", indice+1, code)
		}
	}
	if code := login(); code != http.StatusTooManyRequests {
		prueba.Fatalf("el sexto fallo debe bloquear: %d", code)
	}
	status = http.StatusOK
	if code := login(); code != http.StatusTooManyRequests {
		prueba.Fatalf("con el límite alcanzado tampoco entra un login correcto: %d", code)
	}
	if ttl := rdb.PTTL(ctx, key).Val(); ttl <= 0 {
		prueba.Fatal("la clave quedó sin TTL")
	}
}
