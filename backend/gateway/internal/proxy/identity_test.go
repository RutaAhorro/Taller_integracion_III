package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdentityRoutingAndFailureIsolation(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-User-ID") != "" || r.Header.Get("X-Role") != "" || r.Header.Get("X-User-Role") != "" {
				t.Error("identidad de header falsificado llegó al servicio")
			}
			w.Header().Set("X-Upstream", name)
			w.Header().Add("Set-Cookie", "jwt=issued-token; Path=/; HttpOnly; Secure; SameSite=Lax")
			w.Header().Add("Set-Cookie", "other=preserved; Path=/")
			json.NewEncoder(w).Encode(map[string]string{"path": r.URL.EscapedPath(), "query": r.URL.RawQuery, "cookie": r.Header.Get("Cookie"), "origin": r.Header.Get("Origin")})
		}))
	}
	backend, catalog, identity := upstream("backend"), upstream("catalogo"), upstream("identidad")
	defer backend.Close()
	defer catalog.Close()
	defer identity.Close()
	g := gatewayFor(t, backend.URL, map[string]string{"CATALOGO_URL": catalog.URL, "IDENTIDAD_URL": identity.URL}, nil)
	for _, tc := range []struct{ path, target string }{{"/api/v1/auth", "identidad"}, {"/api/v1/auth/login", "identidad"}, {"/api/v1/auth/me", "identidad"}, {"/api/v1/auth/unknown", "identidad"}, {"/api/v1/auth/a%2Fb", "identidad"}, {"/api/v1/authentication", "backend"}, {"/api/v1/auth-falso", "backend"}, {"/_identidad/ready", "backend"}, {"/api/v1/productos", "catalogo"}, {"/api/v1/scraper/trabajos", "backend"}, {"/health", "backend"}} {
		for _, method := range []string{"GET", "POST", "PUT", "OPTIONS"} {
			t.Run(method+tc.path, func(t *testing.T) {
				req, _ := http.NewRequest(method, g.URL+tc.path+"?value=a%2Fb&value=c+d", strings.NewReader(`{"correo":"fixture@example.test"}`))
				req.Header.Set("Cookie", "jwt=existing-cookie")
				req.Header.Set("Origin", "https://frontend.example")
				req.Header.Set("X-User-ID", "forged")
				req.Header.Set("X-Role", "admin")
				req.Header.Set("X-User-Role", "admin")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]string
				if err = json.Unmarshal(bodyOf(t, res), &body); err != nil {
					t.Fatal(err)
				}
				if res.Header.Get("X-Upstream") != tc.target || body["path"] != tc.path || body["query"] != req.URL.RawQuery || body["cookie"] != "jwt=existing-cookie" || body["origin"] != "https://frontend.example" || len(res.Header.Values("Set-Cookie")) != 2 {
					t.Fatalf("contrato alterado: %v %v", body, res.Header)
				}
			})
		}
	}
	identity.Close()
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/auth/login", 502}, {"/api/v1/productos", 200}, {"/api/v1/scraper/trabajos", 200}, {"/_gateway/ready", 200}} {
		res, err := http.Get(g.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body := bodyOf(t, res)
		if res.StatusCode != tc.status {
			t.Fatalf("%s %d", tc.path, res.StatusCode)
		}
		if tc.status == 502 && !strings.Contains(string(body), "identidad_unavailable") {
			t.Fatalf("error no identifica upstream: %s", body)
		}
	}
}
func TestIdentityDiagnosticAndLocalReadiness(t *testing.T) {
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","checks":{"database":"ok","redis":"ok"}}`))
	}))
	defer b.Close()
	i := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_identidad/ready" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(`{"status":"ok","checks":{"database":"ok","redis":"ok"}}`))
	}))
	defer i.Close()
	g := gatewayFor(t, b.URL, map[string]string{"IDENTIDAD_URL": i.URL}, nil)
	res, _ := http.Get(g.URL + "/_gateway/dependencies")
	body := bodyOf(t, res)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"identidad_database":"ok"`) {
		t.Fatal(string(body))
	}
	i.Close()
	res, _ = http.Get(g.URL + "/_gateway/dependencies")
	body = bodyOf(t, res)
	if res.StatusCode != 503 || !strings.Contains(string(body), `"backend":"ok"`) || !strings.Contains(string(body), `"identidad":"error"`) {
		t.Fatal(string(body))
	}
	res, _ = http.Get(g.URL + "/_gateway/ready")
	bodyOf(t, res)
	if res.StatusCode != 200 {
		t.Fatal("Identidad retiró Gateway")
	}
}
