package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// SUP-278: /api/v1/chat pertenece a IA conversacional;
// el resto de prefijos y servicios conserva su enrutamiento y aislamiento.
func TestIARoutingAndFailureIsolation(prueba *testing.T) {
	upstream := func(nombre string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			if peticion.Header.Get("X-User-ID") != "" || peticion.Header.Get("X-Role") != "" {
				prueba.Error("identidad de header falsificado llego al servicio")
			}
			respuesta.Header().Set("X-Upstream", nombre)
			_ = json.NewEncoder(respuesta).Encode(map[string]string{
				"path":   peticion.URL.EscapedPath(),
				"cookie": peticion.Header.Get("Cookie"),
				"auth":   peticion.Header.Get("Authorization"),
			})
		}))
	}

	backendServidor := upstream("backend")
	catalogoServidor := upstream("catalogo")
	iaServidor := upstream("ia")
	defer backendServidor.Close()
	defer catalogoServidor.Close()
	defer iaServidor.Close()

	pasarela := gatewayFor(prueba, backendServidor.URL, map[string]string{
		"CATALOGO_URL": catalogoServidor.URL,
		"IA_URL":       iaServidor.URL,
	}, nil)

	casosRutas := []struct{ ruta, destinoEsperado string }{
		{"/api/v1/chat", "ia"},
		{"/api/v1/chat/", "ia"},
		{"/api/v1/chat-falso", "backend"},
		{"/api/v1/chatx", "backend"},
		{"/api/v1/productos", "catalogo"},
	}

	for _, caso := range casosRutas {
		for _, metodo := range []string{"GET", "POST"} {
			solicitud, _ := http.NewRequest(metodo, pasarela.URL+caso.ruta, strings.NewReader(`{"mensaje":"Hola"}`))
			solicitud.Header.Set("Cookie", "jwt=cookie-valida")
			solicitud.Header.Set("Authorization", "Bearer token-valido")
			solicitud.Header.Set("X-User-ID", "forged-id")
			solicitud.Header.Set("X-Role", "forged-role")

			respuesta, err := http.DefaultClient.Do(solicitud)
			if err != nil {
				prueba.Fatal(err)
			}
			var cuerpo map[string]string
			if err = json.Unmarshal(bodyOf(prueba, respuesta), &cuerpo); err != nil {
				prueba.Fatal(err)
			}
			if respuesta.Header.Get("X-Upstream") != caso.destinoEsperado || cuerpo["path"] != caso.ruta || cuerpo["cookie"] != "jwt=cookie-valida" || cuerpo["auth"] != "Bearer token-valido" {
				prueba.Fatalf("%s %s: upstream=%v cuerpo=%v", metodo, caso.ruta, respuesta.Header.Get("X-Upstream"), cuerpo)
			}
		}
	}

	// Simular caida de IA: debe aislar la falla con 502 sin afectar otros endpoints
	iaServidor.Close()
	casosFallo := []struct {
		ruta         string
		estadoCodigo int
	}{
		{"/api/v1/chat", 502},
		{"/api/v1/productos", 200},
		{"/_gateway/ready", 200},
	}
	for _, casoFallo := range casosFallo {
		respuesta, err := http.Get(pasarela.URL + casoFallo.ruta)
		if err != nil {
			prueba.Fatal(err)
		}
		cuerpo := bodyOf(prueba, respuesta)
		if respuesta.StatusCode != casoFallo.estadoCodigo || (casoFallo.estadoCodigo == 502 && !strings.Contains(string(cuerpo), "ia_unavailable")) {
			prueba.Fatalf("%s esperado=%d recibido=%d cuerpo=%s", casoFallo.ruta, casoFallo.estadoCodigo, respuesta.StatusCode, cuerpo)
		}
	}
}

func TestChatSinIAURLNoSeEnruta(prueba *testing.T) {
	scrapingServidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		prueba.Errorf("chat no debe caer en Control: %s", peticion.URL.Path)
	}))
	defer scrapingServidor.Close()

	pasarela := gatewayFor(prueba, "", map[string]string{"SCRAPING_URL": scrapingServidor.URL}, nil)
	respuesta, err := http.Get(pasarela.URL + "/api/v1/chat")
	if err != nil {
		prueba.Fatal(err)
	}
	_ = bodyOf(prueba, respuesta)
	if respuesta.StatusCode != http.StatusNotFound {
		prueba.Fatalf("esperaba 404 sin IA_URL, obtuvo %d", respuesta.StatusCode)
	}
}

func TestIADiagnosticDoesNotWithdrawGateway(prueba *testing.T) {
	servidorSano := func(rutaSonda string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			if peticion.URL.Path != rutaSonda {
				prueba.Errorf("probe inesperado %s", peticion.URL.Path)
			}
			_, _ = respuesta.Write([]byte(`{"status":"ok","service":"ia_conversacional"}`))
		}))
	}

	backendServidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		_, _ = respuesta.Write([]byte(`{"status":"ok","checks":{"database":"ok","redis":"ok"}}`))
	}))
	iaServidor := servidorSano("/health")
	defer backendServidor.Close()
	defer iaServidor.Close()

	pasarela := gatewayFor(prueba, backendServidor.URL, map[string]string{"IA_URL": iaServidor.URL}, nil)
	respuesta, _ := http.Get(pasarela.URL + "/_gateway/dependencies")
	cuerpo := bodyOf(prueba, respuesta)
	if respuesta.StatusCode != 200 || !strings.Contains(string(cuerpo), `"ia":"ok"`) {
		prueba.Fatalf("esperaba ia:ok en dependencias, obtuvo: %s", string(cuerpo))
	}

	iaServidor.Close()
	respuesta, _ = http.Get(pasarela.URL + "/_gateway/dependencies")
	cuerpo = bodyOf(prueba, respuesta)
	if respuesta.StatusCode != 503 || !strings.Contains(string(cuerpo), `"ia":"error"`) {
		prueba.Fatalf("esperaba ia:error en dependencias, obtuvo: %s", string(cuerpo))
	}
}

func TestIATimeoutResponds504(prueba *testing.T) {
	servidorLento := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		time.Sleep(120 * time.Millisecond)
	}))
	defer servidorLento.Close()

	servidorControl := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		prueba.Errorf("peticion no debe llegar a control: %s", peticion.URL.Path)
	}))
	defer servidorControl.Close()

	pasarela := gatewayFor(prueba, "", map[string]string{
		"SCRAPING_URL":                    servidorControl.URL,
		"IA_URL":                          servidorLento.URL,
		"GATEWAY_RESPONSE_HEADER_TIMEOUT": "40ms",
	}, nil)

	inicio := time.Now()
	respuesta, errorPeticion := http.Post(pasarela.URL+"/api/v1/chat", "application/json", strings.NewReader(`{"mensaje":"hola"}`))
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	cuerpo := bodyOf(prueba, respuesta)
	if respuesta.StatusCode != http.StatusGatewayTimeout || !strings.Contains(string(cuerpo), "ia_timeout") || time.Since(inicio) > time.Second {
		prueba.Fatalf("esperaba 504 ia_timeout, obtuvo %d: %s", respuesta.StatusCode, string(cuerpo))
	}
}
