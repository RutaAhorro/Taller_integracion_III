package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

func gatewayFor(t *testing.T, backend string, env map[string]string, logger *slog.Logger) *httptest.Server {
	t.Helper()
	c, err := config.Load(func(key string) string {
		if key == "BACKEND_URL" {
			return backend
		}
		return env[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway := New(c, logger)
	t.Cleanup(gateway.CloseIdleConnections)
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	return server
}

func bodyOf(t *testing.T, res *http.Response) []byte {
	t.Helper()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDiagnosticsRequireHealthyDependencies(t *testing.T) {
	for _, tc := range []struct {
		name, body                     string
		upstreamStatus, expectedStatus int
	}{
		{"healthy", `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`, 200, 200},
		{"database_down", `{"status":"ok","checks":{"database":"error","redis":"ok"}}`, 200, 503},
		{"redis_down", `{"status":"ok","checks":{"database":"ok","redis":"error"}}`, 200, 503},
		{"missing_checks", `{"status":"ok"}`, 200, 503},
		{"backend_unhealthy", `{"status":"error","checks":{"database":"ok","redis":"ok"}}`, 200, 503},
		{"invalid_response", `<html>Error</html>`, 200, 503},
		{"backend_error", `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`, 500, 503},
		{"redirect", ``, 302, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/health" || r.Header.Get("X-Request-ID") != "ready-test" {
					t.Errorf("incorrect health probe: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Location", "/unexpected-redirect")
				w.WriteHeader(tc.upstreamStatus)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer backend.Close()
			gateway := gatewayFor(t, backend.URL, nil, nil)
			req, _ := http.NewRequest("GET", gateway.URL+"/_gateway/dependencies", nil)
			req.Header.Set("X-Request-ID", "ready-test")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body := bodyOf(t, res)
			if res.StatusCode != tc.expectedStatus || res.Header.Get("X-Request-ID") != "ready-test" {
				t.Fatalf("unexpected readiness response: %d %s", res.StatusCode, body)
			}
			var state struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(body, &state); err != nil {
				t.Fatal(err)
			}
			if len(state.Checks) != 3 || (state.Status == "ok") != (tc.expectedStatus == 200) {
				t.Fatalf("incorrect dependency state: %s", body)
			}
		})
	}
}

func TestReadinessTimeoutAndLivenessIndependence(t *testing.T) {
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"GATEWAY_READINESS_TIMEOUT": "30ms"}, nil)
	client := &http.Client{Timeout: time.Second}
	for _, tc := range []struct {
		method, path   string
		expectedStatus int
	}{
		{"GET", "/_gateway/dependencies", 503}, {"HEAD", "/_gateway/dependencies", 503},
		{"POST", "/_gateway/dependencies", 405}, {"GET", "/_gateway/live", 200},
		{"GET", "/_gateway/ready", 200}, {"HEAD", "/_gateway/ready", 200}, {"POST", "/_gateway/ready", 405},
	} {
		req, _ := http.NewRequest(tc.method, gateway.URL+tc.path, nil)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body := bodyOf(t, res)
		if res.StatusCode != tc.expectedStatus || (tc.method == "HEAD" && len(body) != 0) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, res.StatusCode, body)
		}
		if tc.expectedStatus == 405 && res.Header.Get("Allow") != "GET, HEAD" {
			t.Fatal("missing Allow header")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("liveness or rejected method queried the backend: %d calls", calls.Load())
	}
}

func TestTransparentRequestAndResponse(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			payload := "{\"productos\":[{\"producto\":\"Leche\"}]}"
			observed := make(chan *http.Request, 1)
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(body))
				observed <- r
				w.Header().Set("Content-Type", "application/json")
				w.Header().Add("Set-Cookie", "jwt=test-session; Path=/; HttpOnly; SameSite=Lax")
				w.Header().Add("Set-Cookie", "preference=compact; Path=/")
				w.Header().Set("Access-Control-Allow-Origin", "https://frontend.example")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("{\"mensaje\":\"Respuesta del backend\"}"))
			}))
			defer backend.Close()
			gateway := gatewayFor(t, backend.URL, nil, nil)
			req, err := http.NewRequest(method, gateway.URL+"/api/v1/productos/a%2Fb?q=leche+entera&q=oferta&empty=&encoded=%2F", strings.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Host = "api.example"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Cookie", "jwt=test-session")
			req.Header.Set("Origin", "https://frontend.example")
			req.Header.Set("X-Request-ID", "sup-264.request_1")
			req.Header.Set("X-Forwarded-For", "203.0.113.200")
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Role", "admin")
			req.Header.Set("X-User-ID", "forged-user")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body := bodyOf(t, res)
			if res.StatusCode != http.StatusCreated || len(res.Header.Values("Set-Cookie")) != 2 ||
				res.Header.Get("X-Request-ID") != "sup-264.request_1" ||
				res.Header.Get("Access-Control-Allow-Origin") != "https://frontend.example" ||
				res.Header.Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatalf("response contract changed: status=%d headers=%v", res.StatusCode, res.Header)
			}
			if method != "HEAD" && string(body) != "{\"mensaje\":\"Respuesta del backend\"}" {
				t.Fatalf("response body changed: %s", body)
			}
			got := <-observed
			gotBody, _ := io.ReadAll(got.Body)
			if got.Method != method || got.Host != "api.example" || got.URL.EscapedPath() != "/api/v1/productos/a%2Fb" ||
				got.URL.RawQuery != req.URL.RawQuery || string(gotBody) != payload ||
				got.Header.Get("Authorization") != "Bearer test-token" || got.Header.Get("Cookie") != "jwt=test-session" ||
				got.Header.Get("Origin") != "https://frontend.example" || got.Header.Get("Content-Type") != "application/json" ||
				got.Header.Get("X-Request-ID") != "sup-264.request_1" {
				t.Fatalf("request contract changed: method=%s host=%s uri=%s headers=%v body=%s", got.Method, got.Host, got.RequestURI, got.Header, gotBody)
			}
			if got.Header.Get("X-Forwarded-For") != "127.0.0.1" || got.Header.Get("X-Forwarded-Proto") != "http" ||
				got.Header.Get("X-Role") != "" || got.Header.Get("X-User-ID") != "" {
				t.Fatalf("forged identity forwarded: %v", got.Header)
			}
		})
	}
}

func TestBackendStatusesAndHealthAreNotReplaced(t *testing.T) {
	for _, status := range []int{200, 400, 401, 403, 404, 405, 409, 429, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "15")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("backend-response"))
			}))
			defer backend.Close()
			gateway := gatewayFor(t, backend.URL, nil, nil)
			res, err := http.Get(gateway.URL + "/api/v1/health")
			if err != nil {
				t.Fatal(err)
			}
			if got := string(bodyOf(t, res)); res.StatusCode != status || got != "backend-response" || res.Header.Get("Retry-After") != "15" {
				t.Fatalf("backend response changed: status=%d body=%q", res.StatusCode, got)
			}
		})
	}
}

func TestRedirectAndGzipArePreserved(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte("backend-content"))
	_ = gz.Close()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/api/v1/auth/me?from=login")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, nil, nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(gateway.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	_ = bodyOf(t, res)
	if res.StatusCode != http.StatusTemporaryRedirect || res.Header.Get("Location") != "/api/v1/auth/me?from=login" {
		t.Fatal("redirect changed")
	}
	res, err = client.Get(gateway.URL + "/compressed")
	if err != nil {
		t.Fatal(err)
	}
	if got := bodyOf(t, res); res.Header.Get("Content-Encoding") != "gzip" || !bytes.Equal(got, compressed.Bytes()) {
		t.Fatal("upstream compression changed")
	}
}

func TestUpstreamFailureAndLocalLiveness(t *testing.T) {
	backend := httptest.NewServer(http.NotFoundHandler())
	backend.Close()
	gateway := gatewayFor(t, backend.URL, nil, nil)
	res, err := http.Get(gateway.URL + "/api/v1/productos")
	if err != nil {
		t.Fatal(err)
	}
	body := bodyOf(t, res)
	if res.StatusCode != http.StatusBadGateway || !bytes.Contains(body, []byte("backend_unavailable")) || res.Header.Get("X-Request-ID") == "" {
		t.Fatalf("incorrect upstream failure: %d %s", res.StatusCode, body)
	}
	res, err = http.Get(gateway.URL + "/_gateway/live")
	if err != nil {
		t.Fatal(err)
	}
	if body := bodyOf(t, res); res.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("gateway")) {
		t.Fatalf("local liveness failed: %d %s", res.StatusCode, body)
	}
}

func TestUpstreamTimeoutCancelsBackend(t *testing.T) {
	for _, env := range []map[string]string{
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT": "50ms"},
		{"GATEWAY_REQUEST_TIMEOUT": "50ms"},
	} {
		t.Run("timeout", func(t *testing.T) {
			canceled := make(chan struct{})
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
				close(canceled)
			}))
			defer backend.Close()
			gateway := gatewayFor(t, backend.URL, env, nil)
			res, err := http.Get(gateway.URL + "/slow")
			if err != nil {
				t.Fatal(err)
			}
			if body := bodyOf(t, res); res.StatusCode != http.StatusGatewayTimeout || !bytes.Contains(body, []byte("backend_timeout")) {
				t.Fatalf("incorrect timeout response: %d %s", res.StatusCode, body)
			}
			select {
			case <-canceled:
			case <-time.After(2 * time.Second):
				t.Fatal("timeout was not propagated to backend")
			}
		})
	}
}

func TestClientCancellationReachesBackend(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/waiting", nil)
	done := make(chan error, 1)
	go func() {
		res, err := http.DefaultClient.Do(req)
		if res != nil {
			_ = res.Body.Close()
		}
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not receive request")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("client did not observe cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not stop")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("backend request did not stop")
	}
}

func TestDeadlineInterruptsResponseAlreadyStarted(t *testing.T) {
	canceled := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial-response"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"GATEWAY_REQUEST_TIMEOUT": "100ms"}, nil)
	res, err := http.Get(gateway.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "partial-response" || err == nil {
		t.Fatalf("stream was not interrupted after deadline: status=%d body=%q err=%v", res.StatusCode, body, err)
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("stream deadline did not cancel backend")
	}
}

func TestWritesAreNotReplayedAfterConnectionFailure(t *testing.T) {
	var writes atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			return
		}
		writes.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close() // Simulate a write committed before its response is lost.
		}
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, nil, nil)
	res, err := http.Get(gateway.URL + "/warm-connection")
	if err != nil {
		t.Fatal(err)
	}
	_ = bodyOf(t, res)
	req, _ := http.NewRequest("POST", gateway.URL+"/api/v1/scraper/trabajos", nil)
	req.Header.Set("Idempotency-Key", "test-observation")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = bodyOf(t, res)
	if res.StatusCode != http.StatusBadGateway || writes.Load() != 1 {
		t.Fatalf("write replayed or failure hidden: status=%d writes=%d", res.StatusCode, writes.Load())
	}
}

func TestTrustedProxyCannotSupplyForgedClientChain(t *testing.T) {
	observed := make(chan http.Header, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- r.Header.Clone()
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, map[string]string{"GATEWAY_TRUSTED_PROXIES": "127.0.0.1"}, nil)
	req, _ := http.NewRequest("GET", gateway.URL+"/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.200, 198.51.100.10")
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = bodyOf(t, res)
	got := <-observed
	if got.Get("X-Forwarded-For") != "198.51.100.10" || got.Get("X-Forwarded-Proto") != "https" {
		t.Fatalf("incorrect trusted proxy handling: %v", got)
	}
}

func TestTrustedProxyHandlesMalformedClientChain(t *testing.T) {
	for _, tc := range []struct{ name, chain, expectedIP string }{
		{"malformed_prefix", "invalid_string, 203.0.113.5", "203.0.113.5"},
		{"empty_prefix", ", 203.0.113.5", "203.0.113.5"},
		{"multiple_trusted_hops", "invalid_string, 203.0.113.5, 10.0.0.2", "203.0.113.5"},
		{"mapped_addresses", "invalid_string, ::ffff:203.0.113.5, ::ffff:10.0.0.2", "203.0.113.5"},
		{"malformed_boundary", "203.0.113.200, invalid_string, 10.0.0.2", "10.0.0.2"},
		{"malformed_nearest_hop", "203.0.113.200, invalid_string", "127.0.0.1"},
		{"missing_chain", "", "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := make(chan http.Header, 1)
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				observed <- r.Header.Clone()
			}))
			defer backend.Close()
			gateway := gatewayFor(t, backend.URL, map[string]string{"GATEWAY_TRUSTED_PROXIES": "127.0.0.1,10.0.0.0/8"}, nil)
			req, _ := http.NewRequest("GET", gateway.URL+"/", nil)
			req.Header.Set("X-Forwarded-For", tc.chain)
			req.Header.Set("X-Forwarded-Proto", "https")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = bodyOf(t, res)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("request failed: %d", res.StatusCode)
			}
			got := <-observed
			if got.Get("X-Forwarded-For") != tc.expectedIP || got.Get("X-Forwarded-Proto") != "https" {
				t.Fatalf("chain %q: expected client IP %s and HTTPS, received %v", tc.chain, tc.expectedIP, got)
			}
		})
	}
}

func TestRequestIDAndLogsDoNotExposeCredentials(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	observed := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- r.Header.Get("X-Request-ID")
	}))
	defer backend.Close()
	gateway := gatewayFor(t, backend.URL, nil, logger)
	req, _ := http.NewRequest("GET", gateway.URL+"/api/v1/auth/me?token=query-secret", nil)
	req.Header.Set("X-Request-ID", "invalid id with spaces")
	req.Header.Set("Authorization", "Bearer authorization-secret")
	req.Header.Set("Cookie", "jwt=cookie-secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = bodyOf(t, res)
	id := <-observed
	if len(id) != 32 || id != res.Header.Get("X-Request-ID") {
		t.Fatalf("request ID not generated/propagated: %q", id)
	}
	// Closing the server waits for handler completion before examining logs.
	gateway.Close()
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["request_id"] != id || entry["path"] != "/api/v1/auth/me" {
		t.Fatalf("correlation missing from logs: %v", entry)
	}
	for _, secret := range []string{"query-secret", "authorization-secret", "cookie-secret"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("credentials logged")
		}
	}
}
