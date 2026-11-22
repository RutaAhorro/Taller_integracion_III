// Package integration_test prueba el Gateway completo contra upstreams
// httptest. comun_test.go reúne los ayudantes compartidos por los demás
// archivos de la suite.
package integration_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Constantes =====

const (
	// cabeceraUpstream la escribe cada servidor eco para saber quién respondió.
	cabeceraUpstream = "X-Upstream"
	// saludSana es la respuesta de un upstream sano con base de datos y Redis.
	saludSana = `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`
)

// ===== Ayudantes =====

// pasarelaPara arranca un Gateway con SCRAPING_URL y el entorno adicional.
func pasarelaPara(prueba *testing.T, urlScraping string, entorno map[string]string, registro *slog.Logger) *httptest.Server {
	prueba.Helper()
	configuracion, errorCarga := config.Cargar(func(clave string) string {
		if clave == "SCRAPING_URL" {
			return urlScraping
		}
		return entorno[clave]
	})
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	pasarela := api.Nueva(configuracion, registro)
	prueba.Cleanup(pasarela.CerrarConexionesInactivas)
	servidor := httptest.NewServer(pasarela)
	prueba.Cleanup(servidor.Close)
	return servidor
}

// cuerpoDe lee y cierra el cuerpo de la respuesta.
func cuerpoDe(prueba *testing.T, respuesta *http.Response) []byte {
	prueba.Helper()
	defer respuesta.Body.Close()
	cuerpo, errorLectura := io.ReadAll(respuesta.Body)
	if errorLectura != nil {
		prueba.Fatal(errorLectura)
	}
	return cuerpo
}

// servidorEco responde con lo que recibió y marca su nombre en X-Upstream.
// Falla la prueba si llega alguna cabecera de identidad falsificable.
func servidorEco(prueba *testing.T, nombre string) *httptest.Server {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Header.Get("X-User-ID") != "" || peticion.Header.Get("X-Role") != "" || peticion.Header.Get("X-User-Role") != "" {
			prueba.Error("una cabecera de identidad falsificada llegó al servicio")
		}
		respuesta.Header().Set(cabeceraUpstream, nombre)
		respuesta.Header().Add("Set-Cookie", "jwt=token-emitido; Path=/; HttpOnly; Secure; SameSite=Lax")
		respuesta.Header().Add("Set-Cookie", "otra=conservada; Path=/")
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(map[string]string{
			"path": peticion.URL.EscapedPath(), "query": peticion.URL.RawQuery, "method": peticion.Method,
			"cookie": peticion.Header.Get("Cookie"), "origin": peticion.Header.Get("Origin"),
			"auth": peticion.Header.Get("Authorization"), "key": peticion.Header.Get("Idempotency-Key"),
		})
	}))
	prueba.Cleanup(servidor.Close)
	return servidor
}

// casoEnrutamiento indica qué upstream debe atender una ruta; destino vacío
// significa que ningún dominio la reclama y el Gateway responde 404.
type casoEnrutamiento struct{ ruta, destino string }

// comprobarEnrutamiento envía cada ruta con cada método y verifica destino,
// ruta, query, cookies, Origin, Authorization y Set-Cookie sin cambios.
func comprobarEnrutamiento(prueba *testing.T, pasarela *httptest.Server, casos []casoEnrutamiento, metodos []string) {
	prueba.Helper()
	for _, caso := range casos {
		for _, metodo := range metodos {
			prueba.Run(metodo+caso.ruta, func(subprueba *testing.T) {
				peticion, _ := http.NewRequest(metodo, pasarela.URL+caso.ruta+"?valor=a%2Fb&valor=c+d", strings.NewReader(`{"dato":true}`))
				peticion.Header.Set("Cookie", "jwt=cookie-existente")
				peticion.Header.Set("Origin", "https://frontend.example")
				peticion.Header.Set("Authorization", "Bearer token-valido")
				peticion.Header.Set("X-User-ID", "falsificado")
				peticion.Header.Set("X-Role", "admin")
				peticion.Header.Set("X-User-Role", "admin")
				respuesta, errorPeticion := http.DefaultClient.Do(peticion)
				if errorPeticion != nil {
					subprueba.Fatal(errorPeticion)
				}
				cuerpo := cuerpoDe(subprueba, respuesta)
				if caso.destino == "" {
					if respuesta.StatusCode != http.StatusNotFound || respuesta.Header.Get(cabeceraUpstream) != "" {
						subprueba.Fatalf("se esperaba 404 del Gateway: %d %s", respuesta.StatusCode, cuerpo)
					}
					return
				}
				var eco map[string]string
				if errorJSON := json.Unmarshal(cuerpo, &eco); errorJSON != nil {
					subprueba.Fatal(errorJSON)
				}
				if respuesta.Header.Get(cabeceraUpstream) != caso.destino || eco["path"] != caso.ruta ||
					eco["query"] != peticion.URL.RawQuery || eco["method"] != metodo ||
					eco["cookie"] != "jwt=cookie-existente" || eco["origin"] != "https://frontend.example" ||
					eco["auth"] != "Bearer token-valido" || len(respuesta.Header.Values("Set-Cookie")) != 2 {
					subprueba.Fatalf("contrato alterado: destino=%q eco=%v cabeceras=%v", respuesta.Header.Get(cabeceraUpstream), eco, respuesta.Header)
				}
			})
		}
	}
}

// casoEstado es una ruta con el estado HTTP y el código de error esperados.
type casoEstado struct {
	ruta        string
	estado      int
	codigoError string
}

// comprobarEstados hace GET a cada ruta y verifica estado y código de error.
func comprobarEstados(prueba *testing.T, pasarela *httptest.Server, casos []casoEstado) {
	prueba.Helper()
	for _, caso := range casos {
		respuesta, errorPeticion := http.Get(pasarela.URL + caso.ruta)
		if errorPeticion != nil {
			prueba.Fatal(errorPeticion)
		}
		cuerpo := cuerpoDe(prueba, respuesta)
		if respuesta.StatusCode != caso.estado || (caso.codigoError != "" && !strings.Contains(string(cuerpo), caso.codigoError)) {
			prueba.Fatalf("%s: estado=%d cuerpo=%s; se esperaba %d %s", caso.ruta, respuesta.StatusCode, cuerpo, caso.estado, caso.codigoError)
		}
	}
}

// diagnostico consulta /_gateway/dependencies y devuelve estado y checks.
func diagnostico(prueba *testing.T, pasarela *httptest.Server) (int, map[string]string) {
	prueba.Helper()
	respuesta, errorPeticion := http.Get(pasarela.URL + "/_gateway/dependencies")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	var estado struct {
		Checks map[string]string `json:"checks"`
	}
	if errorJSON := json.Unmarshal(cuerpoDe(prueba, respuesta), &estado); errorJSON != nil {
		prueba.Fatal(errorJSON)
	}
	return respuesta.StatusCode, estado.Checks
}

// servidorSalud responde siempre el cuerpo dado y exige la ruta de sonda.
func servidorSalud(prueba *testing.T, rutaSonda, cuerpo string) *httptest.Server {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path != rutaSonda {
			prueba.Errorf("sonda inesperada %s; se esperaba %s", peticion.URL.Path, rutaSonda)
		}
		_, _ = io.WriteString(respuesta, cuerpo)
	}))
	prueba.Cleanup(servidor.Close)
	return servidor
}
