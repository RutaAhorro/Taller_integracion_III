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

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// W3 (#99): los logins exitosos no consumen intentos; los fallidos sí.
// Requiere Redis real: IDENTIDAD_TEST_REDIS_URL (p. ej. redis://localhost:6379/15).
func TestLimitarLoginSoloCuentaFallidos(prueba *testing.T) {
	direccion := os.Getenv("IDENTIDAD_TEST_REDIS_URL")
	if direccion == "" {
		prueba.Skip("IDENTIDAD_TEST_REDIS_URL no definido")
	}
	opciones, errorObtenido := redis.ParseURL(direccion)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	redisCliente := redis.NewClient(opciones)
	defer redisCliente.Close()
	contextoOperacion, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	clave := "rate_limit:login:192.0.2.10"
	if errorOperacion := redisCliente.Del(contextoOperacion, clave).Err(); errorOperacion != nil {
		prueba.Skip("Redis no disponible:", errorOperacion)
	}
	defer redisCliente.Del(context.Background(), clave)

	gin.SetMode(gin.TestMode)
	estado := http.StatusOK
	enrutador := gin.New()
	enrutador.POST("/login", api.LimitarLogin(redisCliente, config.AjustesPredeterminados()), func(contexto *gin.Context) { contexto.Status(estado) })
	login := func() int {
		peticionHTTP := httptest.NewRequest("POST", "/login", strings.NewReader("{}"))
		peticionHTTP.RemoteAddr = "192.0.2.10:1234"
		escritor := httptest.NewRecorder()
		enrutador.ServeHTTP(escritor, peticionHTTP)
		return escritor.Code
	}
	for indice := 0; indice < 20; indice++ {
		if codigo := login(); codigo != http.StatusOK {
			prueba.Fatalf("login exitoso %d bloqueado con %d", indice+1, codigo)
		}
	}
	estado = http.StatusUnauthorized
	for indice := 0; indice < 5; indice++ {
		if codigo := login(); codigo != http.StatusUnauthorized {
			prueba.Fatalf("fallo %d: %d", indice+1, codigo)
		}
	}
	if codigo := login(); codigo != http.StatusTooManyRequests {
		prueba.Fatalf("el sexto fallo debe bloquear: %d", codigo)
	}
	estado = http.StatusOK
	if codigo := login(); codigo != http.StatusTooManyRequests {
		prueba.Fatalf("con el límite alcanzado tampoco entra un login correcto: %d", codigo)
	}
	if ttl := redisCliente.PTTL(contextoOperacion, clave).Val(); ttl <= 0 {
		prueba.Fatal("la clave quedó sin TTL")
	}
}
