package crawler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClient(maxRetries int) *crawler.Client {
	return crawler.New(crawler.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    2 * time.Second,
		MaxSize:    1 << 20,
		MaxRetries: maxRetries,
		BackoffMin: time.Millisecond,
		BackoffMax: 50 * time.Millisecond,
		Logger:     testLogger(),
	})
}

func TestFetchDevuelve404ParaLaPaginacion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	response, err := testClient(0).Fetch(context.Background(), server.URL+"/missing")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusNotFound {
		t.Errorf("status = %d", response.Status)
	}
}

// Un 200 real con scripts públicos de Cloudflare/CDN no es un challenge: el
// cuerpo debe entregarse al spider en vez de abortar el raspado.
func TestFetchAceptaPaginasConScriptsDeCDN(t *testing.T) {
	body := `<html><head><script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>` +
		`<script src="https://static.cloudflareinsights.com/beacon.min.js"></script></head>` +
		`<body><h1>Leche Entera</h1>` +
		`<div class="wpforms-recaptcha-container"></div>` +
		`<script>var config={"readOnlyDisallowedFields":["captcha"]};</script></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	response, err := testClient(0).Fetch(context.Background(), server.URL+"/producto")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if string(response.Body) != body {
		t.Errorf("body = %q", response.Body)
	}
}

func TestFetchReintentaEstadosTransitorios(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	response, err := testClient(1).Fetch(context.Background(), server.URL+"/retry")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusOK || string(response.Body) != "ok" {
		t.Errorf("response = %d %q", response.Status, response.Body)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestFetchReportaBloqueoPersistente(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Just a moment"))
	}))
	defer server.Close()

	_, err := testClient(1).Fetch(context.Background(), server.URL+"/blocked")
	var blocked *crawler.BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("error = %T %v, want *BlockedError", err, err)
	}
	if blocked.BlockedResponses != 2 {
		t.Errorf("BlockedResponses = %d, want 2 (intento inicial + reintento)", blocked.BlockedResponses)
	}
}

func TestFetchClasificaEstadosDeBloqueo(t *testing.T) {
	for _, status := range []int{
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(status)
			}))
			defer server.Close()

			_, err := testClient(0).Fetch(context.Background(), server.URL+"/blocked")
			var blocked *crawler.BlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("error = %T %v, want *BlockedError", err, err)
			}
			if blocked.Status != status {
				t.Errorf("status = %d, want %d", blocked.Status, status)
			}
			if blocked.BlockedResponses != 1 {
				t.Errorf("BlockedResponses = %d, want 1", blocked.BlockedResponses)
			}
		})
	}
}

func TestFetchDetectaMarcadoresDeChallenge(t *testing.T) {
	cases := map[string]string{
		"cloudflare":      "cf-chl",
		"just a moment":   "Just a moment...",
		"access denied":   "<h1>Access Denied</h1>",
		"unusual traffic": "Our systems have detected unusual traffic",
		"captcha":         `<div id="captcha"></div>`,
		"captcha solve":   "please solve the captcha.",
		"perimeter x":     `<title>Robot or human?</title><div id="px-captcha"></div>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			_, err := testClient(0).Fetch(context.Background(), server.URL+"/challenge")
			var blocked *crawler.BlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("error = %T %v, want *BlockedError", err, err)
			}
			if blocked.Reason == "" {
				t.Error("BlockedError.Reason vacío")
			}
		})
	}
}

func TestFetchReintentaTimeout(t *testing.T) {
	var pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		pageCalls.Add(1)
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer server.Close()

	client := crawler.New(crawler.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    20 * time.Millisecond,
		MaxSize:    1 << 20,
		MaxRetries: 1,
		BackoffMin: time.Millisecond,
		BackoffMax: 50 * time.Millisecond,
		Logger:     testLogger(),
	})
	if _, err := client.Fetch(context.Background(), server.URL+"/slow"); err == nil {
		t.Fatal("Fetch() debió fallar por timeout")
	}
	if pageCalls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (timeout + reintento)", pageCalls.Load())
	}
}

func TestFetchRespetaRobots(t *testing.T) {
	var pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		pageCalls.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	_, err := testClient(0).Fetch(context.Background(), server.URL+"/private")
	if !errors.Is(err, crawler.ErrRobotsDisallowed) {
		t.Fatalf("error = %v, want ErrRobotsDisallowed", err)
	}
	if pageCalls.Load() != 0 {
		t.Errorf("se solicitó una URL prohibida %d veces", pageCalls.Load())
	}
}

func TestFetchRespetaRetryAfter(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	start := time.Now()
	response, err := testClient(1).Fetch(context.Background(), server.URL+"/retry")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusOK {
		t.Errorf("status = %d", response.Status)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("Fetch() esperó %v; debía respetar Retry-After de 1 s", elapsed)
	}
}

func TestFetchRechazaRespuestasDemasiadoGrandes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()

	client := crawler.New(crawler.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    time.Second,
		MaxSize:    4,
		MaxRetries: 0,
		Logger:     testLogger(),
	})
	if _, err := client.Fetch(context.Background(), server.URL+"/grande"); !errors.Is(err, crawler.ErrResponseTooLarge) {
		t.Fatalf("error = %v, want ErrResponseTooLarge", err)
	}
}
