// salud_test.go comprueba las sondas del Gateway: live/ready locales y el
// diagnóstico de dependencias, que informa sin retirar al Gateway.
package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ===== Diagnóstico de Control =====

// TestDiagnosticoExigeDependenciasSanas recorre respuestas de Control que no
// deben contar como sanas y verifica que sólo hay claves scraping*.
func TestDiagnosticoExigeDependenciasSanas(prueba *testing.T) {
	for _, caso := range []struct {
		nombre, cuerpo                 string
		estadoUpstream, estadoEsperado int
	}{
		{"sano", saludSana, 200, 200},
		{"base_caida", `{"status":"ok","checks":{"database":"error","redis":"ok"}}`, 200, 503},
		{"redis_caido", `{"status":"ok","checks":{"database":"ok","redis":"error"}}`, 200, 503},
		{"sin_checks", `{"status":"ok"}`, 200, 503},
		{"upstream_no_sano", `{"status":"error","checks":{"database":"ok","redis":"ok"}}`, 200, 503},
		{"respuesta_invalida", `<html>Error</html>`, 200, 503},
		{"error_upstream", saludSana, 500, 503},
		{"redireccion", ``, 302, 503},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			control := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				if peticion.Method != "GET" || peticion.URL.Path != "/_scraping/ready" || peticion.Header.Get("X-Request-ID") != "prueba-ready" {
					subprueba.Errorf("sonda de salud incorrecta: %s %s", peticion.Method, peticion.URL.Path)
				}
				respuesta.Header().Set("Location", "/redireccion-inesperada")
				respuesta.WriteHeader(caso.estadoUpstream)
				_, _ = io.WriteString(respuesta, caso.cuerpo)
			}))
			defer control.Close()
			pasarela := pasarelaPara(subprueba, control.URL, nil, nil)
			peticion, _ := http.NewRequest("GET", pasarela.URL+"/_gateway/dependencies", nil)
			peticion.Header.Set("X-Request-ID", "prueba-ready")
			respuesta, errorPeticion := http.DefaultClient.Do(peticion)
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			cuerpo := cuerpoDe(subprueba, respuesta)
			if respuesta.StatusCode != caso.estadoEsperado || respuesta.Header.Get("X-Request-ID") != "prueba-ready" {
				subprueba.Fatalf("respuesta de diagnóstico inesperada: %d %s", respuesta.StatusCode, cuerpo)
			}
			var estado struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if errorJSON := json.Unmarshal(cuerpo, &estado); errorJSON != nil {
				subprueba.Fatal(errorJSON)
			}
			_, tieneScraping := estado.Checks["scraping"]
			if len(estado.Checks) != 3 || !tieneScraping || (estado.Status == "ok") != (caso.estadoEsperado == 200) {
				subprueba.Fatalf("estado de dependencias incorrecto: %s", cuerpo)
			}
		})
	}
}

// TestLimiteDiagnosticoYVidaIndependiente verifica que el diagnóstico respeta
// su límite y que live/ready ni los métodos rechazados consultan upstreams.
func TestLimiteDiagnosticoYVidaIndependiente(prueba *testing.T) {
	var llamadas atomic.Int32
	control := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		llamadas.Add(1)
		<-peticion.Context().Done()
	}))
	defer control.Close()
	pasarela := pasarelaPara(prueba, control.URL, map[string]string{"GATEWAY_READINESS_TIMEOUT": "30ms"}, nil)
	cliente := &http.Client{Timeout: time.Second}
	for _, caso := range []struct {
		metodo, ruta   string
		estadoEsperado int
	}{
		{"GET", "/_gateway/dependencies", 503}, {"HEAD", "/_gateway/dependencies", 503},
		{"POST", "/_gateway/dependencies", 405}, {"GET", "/_gateway/live", 200},
		{"GET", "/_gateway/ready", 200}, {"HEAD", "/_gateway/ready", 200}, {"POST", "/_gateway/ready", 405},
	} {
		peticion, _ := http.NewRequest(caso.metodo, pasarela.URL+caso.ruta, nil)
		respuesta, errorPeticion := cliente.Do(peticion)
		if errorPeticion != nil {
			prueba.Fatal(errorPeticion)
		}
		cuerpo := cuerpoDe(prueba, respuesta)
		if respuesta.StatusCode != caso.estadoEsperado || (caso.metodo == "HEAD" && len(cuerpo) != 0) {
			prueba.Fatalf("%s %s: %d %s", caso.metodo, caso.ruta, respuesta.StatusCode, cuerpo)
		}
		if caso.estadoEsperado == 405 && respuesta.Header.Get("Allow") != "GET, HEAD" {
			prueba.Fatal("falta la cabecera Allow")
		}
	}
	if llamadas.Load() != 2 {
		prueba.Fatalf("live o un método rechazado consultó al upstream: %d llamadas", llamadas.Load())
	}
}

// ===== Diagnóstico por dominio =====

// TestDiagnosticoPorDominioNoRetiraGateway verifica, para cada dominio
// opcional, su clave en el diagnóstico y que su caída no afecta a ready.
func TestDiagnosticoPorDominioNoRetiraGateway(prueba *testing.T) {
	for _, caso := range []struct {
		dominio, variable, rutaSonda, cuerpoSano, subcheck string
	}{
		{"catalogo", "CATALOGO_URL", "/_catalogo/ready", saludSana, "catalogo_database"},
		{"identidad", "IDENTIDAD_URL", "/_identidad/ready", saludSana, "identidad_redis"},
		{"rutas", "RUTAS_URL", "/_rutas/ready", saludSana, "rutas_database"},
		{"ia", "IA_URL", "/health", `{"status":"ok","service":"ia_conversacional"}`, ""},
	} {
		prueba.Run(caso.dominio, func(subprueba *testing.T) {
			control := servidorSalud(subprueba, "/_scraping/ready", saludSana)
			dominio := servidorSalud(subprueba, caso.rutaSonda, caso.cuerpoSano)
			pasarela := pasarelaPara(subprueba, control.URL, map[string]string{caso.variable: dominio.URL}, nil)

			estado, checks := diagnostico(subprueba, pasarela)
			if estado != http.StatusOK || checks[caso.dominio] != "ok" || (caso.subcheck != "" && checks[caso.subcheck] != "ok") {
				subprueba.Fatalf("se esperaba %s sano: %d %v", caso.dominio, estado, checks)
			}

			dominio.Close()
			estado, checks = diagnostico(subprueba, pasarela)
			if estado != http.StatusServiceUnavailable || checks[caso.dominio] != "error" || checks["scraping"] != "ok" {
				subprueba.Fatalf("se esperaba %s caído y Control sano: %d %v", caso.dominio, estado, checks)
			}
			comprobarEstados(subprueba, pasarela, []casoEstado{{ruta: "/_gateway/ready", estado: http.StatusOK}})
		})
	}
}

// TestDiagnosticoSinAliasHeredados verifica que ya no se publican las claves
// backend/database/redis del monolito retirado.
func TestDiagnosticoSinAliasHeredados(prueba *testing.T) {
	control := servidorSalud(prueba, "/_scraping/ready", saludSana)
	pasarela := pasarelaPara(prueba, control.URL, nil, nil)
	_, checks := diagnostico(prueba, pasarela)
	for _, alias := range []string{"backend", "database", "redis"} {
		if _, existe := checks[alias]; existe {
			prueba.Fatalf("el diagnóstico aún publica el alias %q: %v", alias, checks)
		}
	}
}
