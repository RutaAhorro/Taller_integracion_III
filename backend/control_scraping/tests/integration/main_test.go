package integration_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

// Comprueba la composición real y que la API ya no aloje Identidad ni Catálogo.
func TestModuleCompositionHTTP(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	enrutador, _, errorObtenido := app.NewApplication(nil, nil)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	token := "cookie-que-control-no-interpreta"

	casos := []struct {
		nombre, metodo, ruta, cuerpo, cookieSesion string
		codigoEstado                               int
	}{
		{"liveness sin dependencias", "GET", "/_api/live", "", "", 200},
		{"readiness sin bases", "GET", "/health", "", "", 503},
		{"readiness v1 sin bases", "GET", "/api/v1/health", "", "", 503},
		{"perfil fuera de API", "GET", "/api/v1/auth/me", "", "", 404},
		{"actualización fuera de API", "PUT", "/api/v1/auth/me", "{}", "", 404},
		{"admin fuera de API", "GET", "/api/v1/admin/productos", "", "", 404},
		{"admin fuera de API con cookie", "GET", "/api/v1/admin/productos", "", token, 404},
		{"admin barra fuera de API", "GET", "/api/v1/admin/productos/", "", token, 404},
		{"perfil con cookie fuera de API", "GET", "/api/v1/auth/me", "", token, 404},
		{"login fuera de API", "POST", "/api/v1/auth/login", "{}", "", 404},
		{"registro fuera de API", "POST", "/api/v1/auth/register", "{}", "", 404},
		{"google fuera de API", "POST", "/api/v1/auth/google", "{}", "", 404},
		{"búsqueda fuera de API", "GET", "/api/v1/productos/buscar?q=le", "", "", 404},
		{"catálogo fuera de API", "GET", "/api/v1/productos", "", "", 404},
		{"detalle fuera de API", "GET", "/api/v1/productos/product-id", "", "", 404},
		{"trabajo inválido", "GET", "/api/v1/scraper/trabajos/invalid", "", "", 400},
		{"creación inválida", "POST", "/api/v1/scraper/trabajos", "{}", "", 400},
		{"finalización inválida", "PUT", "/api/v1/scraper/trabajos/invalid/finalizar", "{}", "", 400},
		{"ingesta con trabajo inválida", "POST", "/api/v1/scraper/trabajos/invalid/productos", "{}", "", 400},
		{"ingesta directa inválida", "POST", "/api/v1/scraper/productos", "{}", "", 400},
		{"ejecución con ID inválido sin Redis", "POST", "/api/v1/scraper/trabajos/invalid/ejecutar", "{}", "", 400},
	}
	for _, casoPrueba := range casos {
		prueba.Run(casoPrueba.nombre, func(prueba *testing.T) {
			peticion := httptest.NewRequest(casoPrueba.metodo, casoPrueba.ruta, strings.NewReader(casoPrueba.cuerpo))
			peticion.Header.Set("Content-Type", "application/json")
			peticion.Header.Set("Authorization", "Bearer "+testServiceToken)
			if casoPrueba.cookieSesion != "" {
				peticion.AddCookie(&http.Cookie{Name: "jwt", Value: casoPrueba.cookieSesion})
			}
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != casoPrueba.codigoEstado {
				prueba.Fatalf("HTTP %d; esperado %d: %s", respuesta.Code, casoPrueba.codigoEstado, respuesta.Body)
			}

		})
	}
}
