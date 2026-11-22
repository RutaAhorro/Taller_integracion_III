// salud.go responde las sondas propias del Gateway. live/ready son locales:
// un dominio caído no debe retirar del Service las rutas de los demás. El
// diagnóstico de dependencias es aparte y sólo informa.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Rutas y límites =====

const (
	rutaVivo         = "/_gateway/live"
	rutaListo        = "/_gateway/ready"
	rutaDependencias = "/_gateway/dependencies"

	// limiteCuerpoSalud acota la lectura de la respuesta de salud del upstream.
	limiteCuerpoSalud = 64 * 1024

	estadoOK    = "ok"
	estadoError = "error"

	cuerpoSondaLocal = "{\"status\":\"ok\",\"service\":\"gateway\"}\n"
)

// sondaDependencia describe cómo consultar la salud de un dominio y qué
// claves de su campo checks se copian al diagnóstico (entrada → salida).
type sondaDependencia struct {
	origen    *url.URL
	ruta      string
	dominio   string
	subchecks map[string]string
}

// ===== Respuesta de sondas =====

// responderSonda atiende las rutas /_gateway/*; devuelve false si la ruta no
// es una sonda y debe enrutarse.
func responderSonda(respuesta http.ResponseWriter, peticion *http.Request, configuracion config.Configuracion, cliente *http.Client, idPeticion string) bool {
	ruta := peticion.URL.Path
	if ruta != rutaVivo && ruta != rutaListo && ruta != rutaDependencias {
		return false
	}
	if peticion.Method != http.MethodGet && peticion.Method != http.MethodHead {
		respuesta.Header().Set("Allow", "GET, HEAD")
		respuesta.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	respuesta.Header().Set("Content-Type", tipoContenidoJSON)
	if ruta != rutaDependencias {
		respuesta.WriteHeader(http.StatusOK)
		if peticion.Method != http.MethodHead {
			_, _ = io.WriteString(respuesta, cuerpoSondaLocal)
		}
		return true
	}

	contexto, cancelar := context.WithTimeout(peticion.Context(), configuracion.LimiteDiagnostico)
	defer cancelar()
	checks := consultarDependencias(contexto, cliente, configuracion, idPeticion)
	estadoHTTP, estado := http.StatusOK, estadoOK
	for _, resultado := range checks {
		if resultado != estadoOK {
			estadoHTTP, estado = http.StatusServiceUnavailable, "not_ready"
		}
	}
	respuesta.WriteHeader(estadoHTTP)
	if peticion.Method != http.MethodHead {
		_ = json.NewEncoder(respuesta).Encode(map[string]any{"status": estado, "service": "gateway", "checks": checks})
	}
	return true
}

// ===== Diagnóstico de dependencias =====

// sondasConfiguradas lista las sondas de los dominios con origen definido.
func sondasConfiguradas(configuracion config.Configuracion) []sondaDependencia {
	sondas := []sondaDependencia{{configuracion.Scraping, "/_scraping/ready", dominioScraping,
		map[string]string{"database": "scraping_database", "redis": "scraping_redis"}}}
	if configuracion.Catalogo != nil {
		sondas = append(sondas, sondaDependencia{configuracion.Catalogo, "/_catalogo/ready", dominioCatalogo,
			map[string]string{"database": "catalogo_database"}})
	}
	if configuracion.Identidad != nil {
		sondas = append(sondas, sondaDependencia{configuracion.Identidad, "/_identidad/ready", dominioIdentidad,
			map[string]string{"database": "identidad_database", "redis": "identidad_redis"}})
	}
	if configuracion.Rutas != nil {
		sondas = append(sondas, sondaDependencia{configuracion.Rutas, "/_rutas/ready", dominioRutas,
			map[string]string{"database": "rutas_database"}})
	}
	if configuracion.IA != nil {
		sondas = append(sondas, sondaDependencia{configuracion.IA, "/health", dominioIA, nil})
	}
	return sondas
}

// consultarDependencias consulta todas las sondas en paralelo y une sus
// resultados en un mapa plano dominio/subcheck → "ok" | "error".
func consultarDependencias(contexto context.Context, cliente *http.Client, configuracion config.Configuracion, idPeticion string) map[string]string {
	sondas := sondasConfiguradas(configuracion)
	resultados := make(chan map[string]string, len(sondas))
	for _, sonda := range sondas {
		go func() { resultados <- consultarSonda(contexto, cliente, sonda, idPeticion) }()
	}
	checks := make(map[string]string)
	for range sondas {
		for clave, valor := range <-resultados {
			checks[clave] = valor
		}
	}
	return checks
}

// consultarSonda marca el dominio "ok" sólo con 200, JSON válido y
// status "ok"; cada subcheck exige además su propio "ok" en checks.
func consultarSonda(contexto context.Context, cliente *http.Client, sonda sondaDependencia, idPeticion string) map[string]string {
	checks := map[string]string{sonda.dominio: estadoError}
	for _, salida := range sonda.subchecks {
		checks[salida] = estadoError
	}
	destino := *sonda.origen
	destino.Path, destino.RawPath, destino.RawQuery, destino.ForceQuery = sonda.ruta, "", "", false
	peticion, _ := http.NewRequestWithContext(contexto, http.MethodGet, destino.String(), nil)
	peticion.Header.Set(cabeceraIDPeticion, idPeticion)
	respuesta, errorConsulta := cliente.Do(peticion)
	if errorConsulta != nil {
		return checks
	}
	defer respuesta.Body.Close()
	var salud struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if respuesta.StatusCode != http.StatusOK ||
		json.NewDecoder(io.LimitReader(respuesta.Body, limiteCuerpoSalud)).Decode(&salud) != nil ||
		salud.Status != estadoOK {
		return checks
	}
	checks[sonda.dominio] = estadoOK
	for entrada, salida := range sonda.subchecks {
		if salud.Checks[entrada] == estadoOK {
			checks[salida] = estadoOK
		}
	}
	return checks
}
