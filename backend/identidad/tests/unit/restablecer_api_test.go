// Pruebas del endpoint público de restablecimiento sin Redis ni PostgreSQL:
// validan el DTO y la traducción de errores de dominio a respuestas HTTP.

package unit_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// enrutadorRestablecer monta sólo el handler, sin el limitador Redis, para
// comprobar el contrato HTTP del caso de uso.
func enrutadorRestablecer(tokens *tokensFalsos) *gin.Engine {
	gin.SetMode(gin.TestMode)
	enrutador := gin.New()
	manejador := api.NuevoManejadorRestablecer(services.NuevoServicioRecuperacion(tokens))
	enrutador.POST("/api/v1/auth/restablecer", manejador.Restablecer)
	return enrutador
}

// pedirRestablecer envía el cuerpo JSON y devuelve el estado y el mensaje.
func pedirRestablecer(prueba *testing.T, enrutador *gin.Engine, cuerpo string) (int, string) {
	prueba.Helper()
	peticion := httptest.NewRequest(http.MethodPost, "/api/v1/auth/restablecer", bytes.NewBufferString(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)

	var contenido struct {
		Mensaje string `json:"mensaje"`
	}
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &contenido); errorObtenido != nil {
		prueba.Fatalf("respuesta no JSON: %s", respuesta.Body.String())
	}
	return respuesta.Code, contenido.Mensaje
}

// TestRestablecerEndpointExitoso comprueba la respuesta pública sin datos sensibles.
func TestRestablecerEndpointExitoso(prueba *testing.T) {
	tokens := &tokensFalsos{tokenValido: "token-valido"}
	respuesta := httptest.NewRecorder()
	peticion := httptest.NewRequest(http.MethodPost, "/api/v1/auth/restablecer",
		bytes.NewBufferString(`{"token":"token-valido","password":"ClaveFuerte1!"}`))
	peticion.Header.Set("Content-Type", "application/json")
	enrutadorRestablecer(tokens).ServeHTTP(respuesta, peticion)

	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
	if strings.Contains(respuesta.Body.String(), "password_hash") || strings.Contains(respuesta.Body.String(), "token") {
		prueba.Fatalf("la respuesta expone credenciales: %s", respuesta.Body.String())
	}
}

// TestRestablecerEndpointTokenNoValido cubre token inexistente, expirado y usado.
func TestRestablecerEndpointTokenNoValido(prueba *testing.T) {
	casos := []struct {
		nombre        string
		errorServicio error
	}{
		{"inexistente", domain.ErrTokenRecuperacionInvalido},
		{"expirado", domain.ErrTokenRecuperacionInvalido},
		{"usado", domain.ErrTokenRecuperacionInvalido},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			tokens := &tokensFalsos{errorValidar: caso.errorServicio}
			estado, mensaje := pedirRestablecer(subprueba, enrutadorRestablecer(tokens),
				`{"token":"token-vencido","password":"ClaveFuerte1!"}`)
			if estado != http.StatusBadRequest {
				subprueba.Fatalf("estado=%d mensaje=%s", estado, mensaje)
			}
			if !strings.Contains(mensaje, "expiró") {
				subprueba.Fatalf("mensaje=%s", mensaje)
			}
		})
	}
}

// TestRestablecerEndpointClaveDebil comprueba que se devuelve la política de
// contraseñas del cambio voluntario, sin duplicarla en el handler.
func TestRestablecerEndpointClaveDebil(prueba *testing.T) {
	tokens := &tokensFalsos{tokenValido: "token-valido"}
	estado, mensaje := pedirRestablecer(prueba, enrutadorRestablecer(tokens),
		`{"token":"token-valido","password":"corta"}`)

	if estado != http.StatusBadRequest || !strings.Contains(mensaje, "mayúscula") {
		prueba.Fatalf("estado=%d mensaje=%s", estado, mensaje)
	}
	if tokens.vecesConsumir != 0 {
		prueba.Fatalf("se consumió el token con una clave inválida: %d", tokens.vecesConsumir)
	}
}

// TestRestablecerEndpointErrorInterno distingue fallos de base de errores de entrada.
func TestRestablecerEndpointErrorInterno(prueba *testing.T) {
	tokens := &tokensFalsos{
		tokenValido:   "token-valido",
		errorConsumir: errors.New("base caída"),
	}
	estado, mensaje := pedirRestablecer(prueba, enrutadorRestablecer(tokens),
		`{"token":"token-valido","password":"ClaveFuerte1!"}`)

	if estado != http.StatusInternalServerError {
		prueba.Fatalf("estado=%d mensaje=%s", estado, mensaje)
	}
	if !strings.Contains(mensaje, "Error interno") {
		prueba.Fatalf("mensaje=%s", mensaje)
	}
}

// TestRestablecerEndpointCuerpoInvalido comprueba el binding de campos obligatorios.
func TestRestablecerEndpointCuerpoInvalido(prueba *testing.T) {
	tokens := &tokensFalsos{tokenValido: "token-valido"}
	for _, cuerpo := range []string{`{}`, `{"token":"token-valido"}`, `{"password":"ClaveFuerte1!"}`, `no-json`} {
		estado, _ := pedirRestablecer(prueba, enrutadorRestablecer(tokens), cuerpo)
		if estado != http.StatusBadRequest {
			prueba.Fatalf("cuerpo=%s estado=%d", cuerpo, estado)
		}
	}
}
