package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SUP-240: /api/v1/listas pertenece al motor de rutas; el resto de prefijos no cambia de dueño.
func TestRutasRoutingAndFailureIsolation(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-User-ID") != "" || r.Header.Get("X-Role") != "" {
				t.Error("identidad de header falsificado llegó al servicio")
			}
			w.Header().Set("X-Upstream", name)
			json.NewEncoder(w).Encode(map[string]string{"path": r.URL.EscapedPath(), "cookie": r.Header.Get("Cookie")})
		}))
	}
	backend, catalog, rutas := upstream("backend"), upstream("catalogo"), upstream("rutas")
	defer backend.Close()
	defer catalog.Close()
	defer rutas.Close()
	g := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalog.URL, "RUTAS_URL": rutas.URL}, nil)
	for _, tc := range []struct{ path, target string }{
		{"/api/v1/listas", "rutas"}, {"/api/v1/listas/", "rutas"}, {"/api/v1/listas/0b9d0c1e-1111-4111-8111-111111111111", "rutas"},
		{"/api/v1/listas-falsas", "backend"}, {"/api/v1/listasx", "backend"}, {"/_rutas/ready", "backend"}, {"/api/v1/productos", "catalogo"},
	} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			req, _ := http.NewRequest(method, g.URL+tc.path, strings.NewReader(`{"nombre":"Semana"}`))
			req.Header.Set("Cookie", "jwt=existing-cookie")
			req.Header.Set("X-User-ID", "forged")
			req.Header.Set("X-Role", "admin")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]string
			if err = json.Unmarshal(bodyOf(t, res), &body); err != nil {
				t.Fatal(err)
			}
			if res.Header.Get("X-Upstream") != tc.target || body["path"] != tc.path || body["cookie"] != "jwt=existing-cookie" {
				t.Fatalf("%s %s: %v %v", method, tc.path, res.Header.Get("X-Upstream"), body)
			}
		}
	}
	rutas.Close()
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/listas", 502}, {"/api/v1/productos", 200}, {"/_gateway/ready", 200}} {
		res, err := http.Get(g.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body := bodyOf(t, res)
		if res.StatusCode != tc.status || (tc.status == 502 && !strings.Contains(string(body), "rutas_unavailable")) {
			t.Fatalf("%s %d %s", tc.path, res.StatusCode, body)
		}
	}
}

func TestListasSinRutasURLNoSeEnrutan(t *testing.T) {
	scraping := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("listas no debe caer en Control: %s", r.URL.Path)
	}))
	defer scraping.Close()
	g := gatewayFor(t, "", map[string]string{"SCRAPING_URL": scraping.URL}, nil)
	res, err := http.Get(g.URL + "/api/v1/listas")
	if err != nil {
		t.Fatal(err)
	}
	if bodyOf(t, res); res.StatusCode != http.StatusNotFound {
		t.Fatalf("esperaba 404 sin RUTAS_URL, obtuvo %d", res.StatusCode)
	}
}

func TestRutasDiagnosticDoesNotWithdrawGateway(t *testing.T) {
	healthy := func(path string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				t.Errorf("probe inesperado %s", r.URL.Path)
			}
			w.Write([]byte(`{"status":"ok","checks":{"database":"ok","redis":"ok"}}`))
		}))
	}
	b, rutas := healthy("/api/v1/health"), healthy("/_rutas/ready")
	defer b.Close()
	defer rutas.Close()
	g := gatewayFor(t, b.URL, map[string]string{"RUTAS_URL": rutas.URL}, nil)
	res, _ := http.Get(g.URL + "/_gateway/dependencies")
	if body := bodyOf(t, res); res.StatusCode != 200 || !strings.Contains(string(body), `"rutas_database":"ok"`) {
		t.Fatal(string(body))
	}
	rutas.Close()
	res, _ = http.Get(g.URL + "/_gateway/dependencies")
	if body := bodyOf(t, res); res.StatusCode != 503 || !strings.Contains(string(body), `"rutas":"error"`) {
		t.Fatal(string(body))
	}
	res, _ = http.Get(g.URL + "/_gateway/ready")
	if bodyOf(t, res); res.StatusCode != 200 {
		t.Fatal("el motor de rutas retiró Gateway")
	}
}
