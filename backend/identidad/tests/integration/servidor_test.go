// Router completo sin infraestructura externa: CORS, sesión que falla cerrada
// sin base, sondas, límites de petición y errores que no exponen SQL.

package integration_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ===== Fronteras HTTP =====

func TestHTTPBoundariesCorsAndJWT(prueba *testing.T) {
	configuracion := config.Configuracion{ClaveJWT: strings.Repeat("j", 32), OrigenesPermitidos: []string{"https://frontend.example"}}
	enrutador := enrutadorDePrueba(prueba, configuracion, func(context.Context) error { return nil })
	// Con la base inalcanzable un token válido no se autoriza y la
	// petición termina en 500 fail-closed, nunca en un bypass de la revalidación.
	const usuarioID = "c14bd236-294a-41b8-8cbb-5473bb610415"
	token, errorObtenido := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": usuarioID, "rol": "registrado", "provider": "local", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(strings.Repeat("j", 32)))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	for _, caso := range []struct {
		name, method, path, origin, body, cookie string
		status                                   int
	}{
		{"live", "GET", "/_identidad/live", "", "", "", 200}, {"ready without Redis", "GET", "/_identidad/ready", "", "", "", 503},
		{"missing cookie", "GET", "/api/v1/auth/me", "", "", "", 401}, {"valid cookie fails closed without DB", "GET", "/api/v1/auth/me", "", "", token, 500},
		{"allowed CORS", "GET", "/api/v1/auth/me", "https://frontend.example", "", token, 500}, {"hostile CORS", "GET", "/api/v1/auth/me", "https://evil.example", "", token, 403},
		{"hostile mutation", "PUT", "/api/v1/auth/me", "https://evil.example", `{"nombre_completo":"forged"}`, token, 403},
		{"allowed preflight", "OPTIONS", "/api/v1/auth/me", "https://frontend.example", "", "", 204}, {"hostile preflight", "OPTIONS", "/api/v1/auth/me", "https://evil.example", "", "", 403},
		{"fail closed registration", "POST", "/api/v1/auth/register", "", `{}`, "", 500}, {"fail closed login", "POST", "/api/v1/auth/login", "", `{}`, "", 500}, {"fail closed google", "POST", "/api/v1/auth/google", "", `{}`, "", 500},
		{"catalog not owned", "GET", "/api/v1/productos", "", "", "", 404}, {"scraping not owned", "POST", "/api/v1/scraper/trabajos", "", `{}`, "", 404}, {"method not allowed", "DELETE", "/api/v1/auth/me", "", "", "", 405},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			peticionHTTP := httptest.NewRequest(caso.method, caso.path, strings.NewReader(caso.body))
			peticionHTTP.Header.Set("Content-Type", "application/json")
			peticionHTTP.Header.Set("Origin", caso.origin)
			peticionHTTP.Header.Set("X-User-ID", "forged")
			peticionHTTP.Header.Set("X-Role", "admin")
			if caso.cookie != "" {
				peticionHTTP.AddCookie(&http.Cookie{Name: "jwt", Value: caso.cookie})
			}
			escritor := httptest.NewRecorder()
			enrutador.ServeHTTP(escritor, peticionHTTP)
			if escritor.Code != caso.status {
				prueba.Fatalf("%d %s", escritor.Code, escritor.Body)
			}
			if caso.origin != "https://frontend.example" && escritor.Header().Get("Access-Control-Allow-Credentials") != "" {
				prueba.Fatal("credenciales CORS fuera de allowlist")
			}
			if escritor.Header().Get("X-Request-ID") == "" {
				prueba.Fatal("correlación ausente")
			}
			if escritor.Code == 500 && strings.Contains(escritor.Body.String(), "detalle") {
				prueba.Fatal("error interno expuesto")
			}
		})
	}
}
func TestRecoveryDoesNotExposePanic(prueba *testing.T) {
	enrutador := enrutadorDePrueba(prueba, config.Configuracion{}, nil)
	enrutador.GET("/panic", func(contexto *gin.Context) { panic("private SQL table and credentials") })
	escritor := httptest.NewRecorder()
	enrutador.ServeHTTP(escritor, httptest.NewRequest("GET", "/panic", nil))
	if escritor.Code != 500 || strings.Contains(escritor.Body.String(), "private") {
		prueba.Fatal(escritor.Code, escritor.Body)
	}
}
func TestReadinessDeadlineAndLiveness(prueba *testing.T) {
	llamadas := 0
	enrutador := enrutadorDePrueba(prueba, config.Configuracion{}, func(contextoOperacion context.Context) error {
		llamadas++
		<-contextoOperacion.Done()
		return errors.New("private diagnostic")
	})
	contexto, cancelar := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancelar()
	escritor := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(escritor, httptest.NewRequest("GET", "/_identidad/ready", nil).WithContext(contexto))
	if escritor.Code != 503 || time.Since(inicio) > time.Second {
		prueba.Fatal("deadline perdido")
	}
	escritor = httptest.NewRecorder()
	enrutador.ServeHTTP(escritor, httptest.NewRequest("GET", "/_identidad/live", nil))
	if llamadas != 1 || escritor.Code != 200 {
		prueba.Fatal("live consulta DB")
	}
}

func TestUnclassifiedContextErrorDoesNotExposeSQL(prueba *testing.T) {
	enrutador := enrutadorDePrueba(prueba, config.Configuracion{}, nil)
	enrutador.GET("/context-error", func(contexto *gin.Context) { _ = contexto.Error(errors.New("private SQL table and credentials")) })
	escritor := httptest.NewRecorder()
	enrutador.ServeHTTP(escritor, httptest.NewRequest("GET", "/context-error", nil))
	if escritor.Code != 500 || strings.Contains(escritor.Body.String(), "private") || strings.Contains(escritor.Body.String(), "detalle") {
		prueba.Fatal(escritor.Code, escritor.Body)
	}
}

// ===== Límites configurados =====

// TestRouterAplicaLimitesConfigurados comprueba tamaño de cuerpo y cancelación.
func TestRouterAplicaLimitesConfigurados(prueba *testing.T) {
	ajustes := config.AjustesPredeterminados()
	ajustes.MaximoBytesCuerpo = 8
	ajustes.PlazoPeticion = 25 * time.Millisecond
	enrutador := enrutadorDePrueba(prueba, config.Configuracion{Ajustes: ajustes}, nil)
	enrutador.POST("/limites", func(contexto *gin.Context) {
		cuerpo, errorLectura := io.ReadAll(contexto.Request.Body)
		var exceso *http.MaxBytesError
		if !errors.As(errorLectura, &exceso) || len(cuerpo) != ajustes.MaximoBytesCuerpo {
			prueba.Error("no se aplicó el límite de cuerpo configurado")
		}
		<-contexto.Request.Context().Done()
		if !errors.Is(contexto.Request.Context().Err(), context.DeadlineExceeded) {
			prueba.Error("la petición no terminó por el plazo configurado")
		}
		contexto.Status(http.StatusNoContent)
	})
	respuesta := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodPost, "/limites", strings.NewReader("123456789")))
	if respuesta.Code != http.StatusNoContent || time.Since(inicio) > time.Second {
		prueba.Fatal("el router no respetó el plazo de la petición")
	}
}

// TestReadinessUsaSuPlazo comprueba que la sonda tiene su presupuesto propio.
func TestReadinessUsaSuPlazo(prueba *testing.T) {
	ajustes := config.AjustesPredeterminados()
	ajustes.PlazoReadiness = 20 * time.Millisecond
	enrutador := enrutadorDePrueba(prueba, config.Configuracion{Ajustes: ajustes}, func(contexto context.Context) error {
		<-contexto.Done()
		return contexto.Err()
	})
	respuesta := httptest.NewRecorder()
	inicio := time.Now()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, "/_identidad/ready", nil))
	if respuesta.Code != http.StatusServiceUnavailable || time.Since(inicio) > time.Second {
		prueba.Fatal("la sonda no respetó su presupuesto configurado")
	}
}
