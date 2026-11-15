// Package integration_test comprueba que Swagger publique sólo el contrato activo.
package integration_test

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

// Cada operación publicada debe existir en el router del servicio, incluso al
// generar Swagger después de extraer otro dominio.
func TestSwaggerOnlyDocumentsRegisteredRoutes(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "true")
	enrutador, _, errorObtenido := app.NewApplication(nil, nil)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", "/swagger/doc.json", nil))
	if respuesta.Code != 200 {
		prueba.Fatal(respuesta.Code, respuesta.Body)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &doc); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(doc.Paths) == 0 || doc.Paths["/api/v1/scraper/trabajos"] == nil {
		prueba.Fatal("missing coordination contract")
	}
	rutas := make(map[string]bool)
	for _, ruta := range enrutador.Routes() {
		rutas[ruta.Method+" "+ruta.Path] = true
	}
	parametro := regexp.MustCompile(`\{([^}]+)\}`)
	for ruta, metodos := range doc.Paths {
		if strings.HasPrefix(ruta, "/_") {
			prueba.Errorf("Swagger público incluye probe privado: %s", ruta)
		}
		for metodo := range metodos {
			clave := strings.ToUpper(metodo) + " " + parametro.ReplaceAllString(ruta, ":$1")
			if !rutas[clave] {
				prueba.Errorf("Swagger advertises an unregistered operation: %s", clave)
			}
		}
	}
}

// SUP-274: sin habilitarlo explícitamente, el contrato no se publica.
func TestSwaggerDisabledByDefault(prueba *testing.T) {
	prepareApplicationEnvironment(prueba)
	for _, valor := range []string{"", "false"} {
		prueba.Run("valor "+valor, func(prueba *testing.T) {
			prueba.Setenv("SCRAPING_SWAGGER_ENABLED", valor)
			enrutador, _, errorObtenido := app.NewApplication(nil, nil)
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			for _, ruta := range []string{"/swagger/doc.json", "/swagger/index.html"} {
				respuesta := httptest.NewRecorder()
				enrutador.ServeHTTP(respuesta, httptest.NewRequest("GET", ruta, nil))
				if respuesta.Code != 404 {
					prueba.Fatalf("%s publicado sin habilitar: %d", ruta, respuesta.Code)
				}
			}
		})
	}
	prueba.Run("valor inválido", func(prueba *testing.T) {
		prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "yes")
		if _, _, errorObtenido := app.NewApplication(nil, nil); errorObtenido == nil {
			prueba.Fatal("un valor ambiguo debe impedir el arranque")
		}
	})
}
