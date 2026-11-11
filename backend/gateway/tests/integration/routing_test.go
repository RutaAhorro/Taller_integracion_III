package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCatalogRoutingPreservesRequestsAndBoundaries(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-User-Role") != "" {
				t.Error("forged role reached upstream")
			}
			w.Header().Set("X-Upstream", name)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"path": r.URL.EscapedPath(), "query": r.URL.RawQuery, "method": r.Method, "cookie": r.Header.Get("Cookie"), "origin": r.Header.Get("Origin")})
		}))
	}
	backend, catalogo := upstream("backend"), upstream("catalogo")
	defer backend.Close()
	defer catalogo.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalogo.URL}, nil)
	for _, tc := range []struct{ path, destination string }{
		{"/api/v1/productos", "catalogo"}, {"/api/v1/productos/", "catalogo"},
		{"/api/v1/productos/buscar", "catalogo"}, {"/api/v1/productos/a%2Fb", "catalogo"},
		{"/api/v1/admin/productos", "catalogo"}, {"/api/v1/admin/productos/", "catalogo"},
		{"/api/v1/productos-falso", "backend"}, {"/api/v1/admin/productos-falso", "backend"},
		{"/api/v1/auth/login", "backend"}, {"/api/v1/scraper/productos", "backend"},
		{"/api/v1/scraper/trabajos/123/productos", "backend"}, {"/api/v1/health", "backend"},
		{"/swagger/doc.json", "backend"}, {"/_catalogo/ready", "backend"},
	} {
		for _, method := range []string{"GET", "POST", "PUT", "OPTIONS"} {
			t.Run(method+tc.path, func(t *testing.T) {
				req, _ := http.NewRequest(method, gateway.URL+tc.path+"?q=leche+entera&q=oferta&encoded=%2F", strings.NewReader("payload"))
				req.Header.Set("Cookie", "jwt=issued-by-identity")
				req.Header.Set("Origin", "https://frontend.example")
				req.Header.Set("X-User-Role", "admin")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]string
				if err := json.Unmarshal(bodyOf(t, res), &body); err != nil {
					t.Fatal(err)
				}
				if res.Header.Get("X-Upstream") != tc.destination || body["path"] != tc.path || body["query"] != req.URL.RawQuery || body["method"] != method || body["cookie"] != req.Header.Get("Cookie") || body["origin"] != req.Header.Get("Origin") {
					t.Fatalf("routing changed request: %v", body)
				}
			})
		}
	}
}

func TestIndependentUpstreamFailures(t *testing.T) {
	for _, failed := range []string{"catalogo", "backend"} {
		t.Run(failed, func(t *testing.T) {
			var backendCalls, catalogCalls atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { backendCalls.Add(1); _, _ = io.WriteString(w, "backend") }))
			catalogo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				catalogCalls.Add(1)
				_, _ = io.WriteString(w, "catalogo")
			}))
			defer backend.Close()
			defer catalogo.Close()
			gateway := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalogo.URL}, nil)
			if failed == "catalogo" {
				catalogo.Close()
			} else {
				backend.Close()
			}
			for _, path := range []string{"/api/v1/productos", "/api/v1/auth/me", "/_gateway/ready"} {
				res, err := http.Get(gateway.URL + path)
				if err != nil {
					t.Fatal(err)
				}
				body := bodyOf(t, res)
				failure := (path == "/api/v1/productos" && failed == "catalogo") || (path == "/api/v1/auth/me" && failed == "backend")
				if failure {
					if res.StatusCode != 502 || !strings.Contains(string(body), failed+"_unavailable") {
						t.Fatalf("incorrect error: %d %s", res.StatusCode, body)
					}
					var payload map[string]string
					if err := json.Unmarshal(body, &payload); err != nil {
						t.Fatal(err)
					}
					message := map[string]string{"backend": "El backend no está disponible.", "catalogo": "Catálogo no está disponible."}[failed]
					if payload["mensaje"] != message {
						t.Fatalf("error contract changed: %s", body)
					}
				} else if res.StatusCode != 200 {
					t.Fatalf("another domain lost service: %d %s", res.StatusCode, body)
				}
			}
			if failed == "catalogo" && backendCalls.Load() != 1 {
				t.Fatal("catalog request fell back to backend")
			}
			if failed == "backend" && catalogCalls.Load() != 1 {
				t.Fatal("backend request fell back to catalog")
			}
		})
	}
}

func TestCatalogTimeoutHasNoBackendFallback(t *testing.T) {
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	catalogo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer backend.Close()
	defer catalogo.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalogo.URL, "GATEWAY_REQUEST_TIMEOUT": "30ms"}, nil)
	res, err := http.Get(gateway.URL + "/api/v1/productos")
	if err != nil {
		t.Fatal(err)
	}
	if body := bodyOf(t, res); res.StatusCode != 504 || !strings.Contains(string(body), "catalogo_timeout") || calls.Load() != 0 {
		t.Fatalf("wrong timeout/fallback: %d %s", res.StatusCode, body)
	}
}

func TestDiagnosticsIncludeCatalogWithoutBlockingReadiness(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`)
	}))
	catalogo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_catalogo/ready" {
			t.Error("wrong catalog probe")
		}
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"status":"error"}`)
	}))
	defer backend.Close()
	defer catalogo.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalogo.URL}, nil)
	res, err := http.Get(gateway.URL + "/_gateway/dependencies")
	if err != nil {
		t.Fatal(err)
	}
	var health struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(bodyOf(t, res), &health); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 503 || health.Checks["backend"] != "ok" || health.Checks["catalogo"] != "error" || health.Checks["catalogo_database"] != "error" {
		t.Fatalf("wrong diagnostics: %v", health)
	}
	res, err = http.Get(gateway.URL + "/_gateway/ready")
	if err != nil {
		t.Fatal(err)
	}
	bodyOf(t, res)
	if res.StatusCode != 200 {
		t.Fatal("catalog failure withdrew gateway")
	}
}

// TestCustomRoutePrefixesRouteChildPaths reproduce el caso de la revisión de
// SUP-350: con GATEWAY_CATALOGO_PREFIXES=/api/v1/productos/ las rutas hijas
// deben seguir llegando a Catálogo porque el "/" final se recorta al cargar.
// También verifica que un prefijo propio enrute sin capturar rutas vecinas.
func TestCustomRoutePrefixesRouteChildPaths(prueba *testing.T) {
	servidorDestino := func(nombre string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			respuesta.Header().Set("X-Upstream", nombre)
			_, _ = io.WriteString(respuesta, peticion.URL.EscapedPath())
		}))
	}
	backend, catalogo := servidorDestino("backend"), servidorDestino("catalogo")
	defer backend.Close()
	defer catalogo.Close()

	for _, escenario := range []struct {
		nombre, prefijos string
		rutas            map[string]string
	}{
		{
			nombre:   "barra_final",
			prefijos: "/api/v1/productos/",
			rutas: map[string]string{
				"/api/v1/productos":        "catalogo",
				"/api/v1/productos/":       "catalogo",
				"/api/v1/productos/123":    "catalogo",
				"/api/v1/productos/a%2Fb":  "catalogo",
				"/api/v1/productos-falso":  "backend",
				"/api/v1/admin/productos":  "backend",
				"/api/v1/scraper/trabajos": "backend",
			},
		},
		{
			nombre:   "prefijos_propios",
			prefijos: "/api/v2/catalogo, /api/v1/ofertas/",
			rutas: map[string]string{
				"/api/v2/catalogo":         "catalogo",
				"/api/v2/catalogo/item/9":  "catalogo",
				"/api/v1/ofertas/semana":   "catalogo",
				"/api/v2/catalogo-viejo":   "backend",
				"/api/v1/productos":        "backend",
				"/api/v1/productos/123":    "backend",
				"/api/v1/scraper/trabajos": "backend",
			},
		},
	} {
		prueba.Run(escenario.nombre, func(subprueba *testing.T) {
			gateway := gatewayFor(subprueba, backend.URL, map[string]string{
				"CATALOGO_URL":              catalogo.URL,
				"GATEWAY_CATALOGO_PREFIXES": escenario.prefijos,
			}, nil)
			for ruta, destino := range escenario.rutas {
				respuesta, errorPeticion := http.Get(gateway.URL + ruta)
				if errorPeticion != nil {
					subprueba.Fatal(errorPeticion)
				}
				cuerpo := bodyOf(subprueba, respuesta)
				if respuesta.StatusCode != http.StatusOK || respuesta.Header.Get("X-Upstream") != destino || string(cuerpo) != ruta {
					subprueba.Fatalf("%s: estado=%d destino=%q ruta=%q, se esperaba %q", ruta, respuesta.StatusCode, respuesta.Header.Get("X-Upstream"), cuerpo, destino)
				}
			}
		})
	}
}
