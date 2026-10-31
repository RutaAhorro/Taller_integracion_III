package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrapingRoutingWithoutMonolith(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-User-ID") != "" || r.Header.Get("X-Role") != "" {
				t.Error("header de identidad falsificado")
			}
			w.Header().Set("X-Upstream", name)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "checks": map[string]string{"database": "ok", "redis": "ok"}, "path": r.URL.EscapedPath(), "key": r.Header.Get("Idempotency-Key")})
		}))
	}
	scraping, identity, catalog := upstream("scraping"), upstream("identidad"), upstream("catalogo")
	defer scraping.Close()
	defer identity.Close()
	defer catalog.Close()
	g := gatewayFor(t, "", map[string]string{"SCRAPING_URL": scraping.URL, "IDENTIDAD_URL": identity.URL, "CATALOGO_URL": catalog.URL}, nil)
	for _, tc := range []struct {
		path, target string
		status       int
	}{
		{"/api/v1/scraper/trabajos", "scraping", 200}, {"/api/v1/scraper/a%2Fb", "scraping", 200},
		{"/health", "scraping", 200}, {"/api/v1/health", "scraping", 200}, {"/swagger/doc.json", "scraping", 200},
		{"/api/v1/auth/me", "identidad", 200}, {"/api/v1/productos", "catalogo", 200},
		{"/api/v1/scrapertools", "", 404}, {"/_scraping/ready", "", 404}, {"/_scraping/live", "", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req, _ := http.NewRequest("POST", g.URL+tc.path, strings.NewReader(`{"fixture":true}`))
			req.Header.Set("Idempotency-Key", "stable-batch-id")
			req.Header.Set("X-User-ID", "forged")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body := bodyOf(t, res)
			if res.StatusCode != tc.status || res.Header.Get("X-Upstream") != tc.target {
				t.Fatalf("%d %s", res.StatusCode, body)
			}
			if tc.status == 200 && !strings.Contains(string(body), "stable-batch-id") {
				t.Fatal("clave de idempotencia perdida")
			}
		})
	}
	scraping.Close()
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/scraper/trabajos", 502}, {"/api/v1/auth/me", 200}, {"/api/v1/productos", 200}, {"/_gateway/ready", 200}} {
		res, err := http.Get(g.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body := bodyOf(t, res)
		if res.StatusCode != tc.status {
			t.Fatal(tc.path, res.StatusCode)
		}
		if tc.status == 502 && !strings.Contains(string(body), "scraping_unavailable") {
			t.Fatal(string(body))
		}
	}
}

func TestScrapingTimeoutDoesNotFallbackToLegacy(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("hubo fallback al monolito") }))
	defer legacy.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(150 * time.Millisecond) }))
	defer slow.Close()
	g := gatewayFor(t, legacy.URL, map[string]string{"SCRAPING_URL": slow.URL, "GATEWAY_REQUEST_TIMEOUT": "50ms"}, nil)
	start := time.Now()
	res, err := http.Post(g.URL+"/api/v1/scraper/productos", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	body := bodyOf(t, res)
	if res.StatusCode != 504 || !strings.Contains(string(body), "scraping_timeout") || time.Since(start) > time.Second {
		t.Fatal(res.StatusCode, string(body))
	}
}
