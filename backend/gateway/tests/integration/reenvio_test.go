// reenvio_test.go comprueba X-Forwarded-* con proxies fiables y que
// X-Request-ID y los logs no exponen credenciales.
package integration_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===== Proxies fiables =====

// servidorCabeceras publica las cabeceras de cada petición recibida.
func servidorCabeceras(prueba *testing.T) (*httptest.Server, chan http.Header) {
	observadas := make(chan http.Header, 1)
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		observadas <- peticion.Header.Clone()
	}))
	prueba.Cleanup(servidor.Close)
	return servidor, observadas
}

// TestProxyFiableNoPuedeAportarCadenaFalsificada verifica que sólo se toma el
// último salto que agregó el proxy fiable.
func TestProxyFiableNoPuedeAportarCadenaFalsificada(prueba *testing.T) {
	upstream, observadas := servidorCabeceras(prueba)
	pasarela := pasarelaPara(prueba, upstream.URL, map[string]string{"GATEWAY_TRUSTED_PROXIES": "127.0.0.1"}, nil)
	peticion, _ := http.NewRequest("GET", pasarela.URL+"/", nil)
	peticion.Header.Set("X-Forwarded-For", "203.0.113.200, 198.51.100.10")
	peticion.Header.Set("X-Forwarded-Proto", "https")
	respuesta, errorPeticion := http.DefaultClient.Do(peticion)
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	_ = cuerpoDe(prueba, respuesta)
	recibidas := <-observadas
	if recibidas.Get("X-Forwarded-For") != "198.51.100.10" || recibidas.Get("X-Forwarded-Proto") != "https" {
		prueba.Fatalf("manejo incorrecto del proxy fiable: %v", recibidas)
	}
}

// TestProxyFiableManejaCadenaMalFormada recorre cadenas con saltos inválidos.
func TestProxyFiableManejaCadenaMalFormada(prueba *testing.T) {
	for _, caso := range []struct{ nombre, cadena, ipEsperada string }{
		{"prefijo_mal_formado", "texto_invalido, 203.0.113.5", "203.0.113.5"},
		{"prefijo_vacio", ", 203.0.113.5", "203.0.113.5"},
		{"varios_saltos_fiables", "texto_invalido, 203.0.113.5, 10.0.0.2", "203.0.113.5"},
		{"direcciones_mapeadas", "texto_invalido, ::ffff:203.0.113.5, ::ffff:10.0.0.2", "203.0.113.5"},
		{"limite_mal_formado", "203.0.113.200, texto_invalido, 10.0.0.2", "10.0.0.2"},
		{"salto_cercano_mal_formado", "203.0.113.200, texto_invalido", "127.0.0.1"},
		{"sin_cadena", "", "127.0.0.1"},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			upstream, observadas := servidorCabeceras(subprueba)
			pasarela := pasarelaPara(subprueba, upstream.URL, map[string]string{"GATEWAY_TRUSTED_PROXIES": "127.0.0.1,10.0.0.0/8"}, nil)
			peticion, _ := http.NewRequest("GET", pasarela.URL+"/", nil)
			peticion.Header.Set("X-Forwarded-For", caso.cadena)
			peticion.Header.Set("X-Forwarded-Proto", "https")
			respuesta, errorPeticion := http.DefaultClient.Do(peticion)
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			_ = cuerpoDe(subprueba, respuesta)
			if respuesta.StatusCode != http.StatusOK {
				subprueba.Fatalf("falló la petición: %d", respuesta.StatusCode)
			}
			recibidas := <-observadas
			if recibidas.Get("X-Forwarded-For") != caso.ipEsperada || recibidas.Get("X-Forwarded-Proto") != "https" {
				subprueba.Fatalf("cadena %q: se esperaba IP %s y HTTPS, se recibió %v", caso.cadena, caso.ipEsperada, recibidas)
			}
		})
	}
}

// ===== Identificador y logs =====

// TestIDPeticionYLogsNoExponenCredenciales verifica que un X-Request-ID
// inválido se reemplaza y que query, Authorization y cookies no van al log.
func TestIDPeticionYLogsNoExponenCredenciales(prueba *testing.T) {
	var salidaLog bytes.Buffer
	registro := slog.New(slog.NewJSONHandler(&salidaLog, nil))
	observado := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		observado <- peticion.Header.Get("X-Request-ID")
	}))
	defer upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, nil, registro)
	peticion, _ := http.NewRequest("GET", pasarela.URL+"/api/v1/scraper/trabajos?token=secreto-query", nil)
	peticion.Header.Set("X-Request-ID", "id invalido con espacios")
	peticion.Header.Set("Authorization", "Bearer secreto-autorizacion")
	peticion.Header.Set("Cookie", "jwt=secreto-cookie")
	respuesta, errorPeticion := http.DefaultClient.Do(peticion)
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	_ = cuerpoDe(prueba, respuesta)
	idPeticion := <-observado
	if len(idPeticion) != 32 || idPeticion != respuesta.Header.Get("X-Request-ID") {
		prueba.Fatalf("no se generó o propagó el ID de petición: %q", idPeticion)
	}
	// Cerrar el servidor espera a que termine el manejador antes de leer el log.
	pasarela.Close()
	var entrada map[string]any
	if errorJSON := json.Unmarshal(bytes.TrimSpace(salidaLog.Bytes()), &entrada); errorJSON != nil {
		prueba.Fatal(errorJSON)
	}
	if entrada["request_id"] != idPeticion || entrada["path"] != "/api/v1/scraper/trabajos" {
		prueba.Fatalf("falta la correlación en el log: %v", entrada)
	}
	for _, secreto := range []string{"secreto-query", "secreto-autorizacion", "secreto-cookie"} {
		if strings.Contains(salidaLog.String(), secreto) {
			prueba.Fatal("se registraron credenciales")
		}
	}
}
