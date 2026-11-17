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

func TestFetchDevuelve404ParaLaPaginacion(prueba *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		http.NotFound(respuesta, peticion)
	}))
	defer server.Close()

	response, err := testClient(0).Fetch(context.Background(), server.URL+"/missing")
	if err != nil {
		prueba.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusNotFound {
		prueba.Errorf("status = %d", response.Status)
	}
}

// Un 200 real con scripts públicos de Cloudflare/CDN no es un challenge: el
// cuerpo debe entregarse al spider en vez de abortar el raspado.
func TestFetchAceptaPaginasConScriptsDeCDN(prueba *testing.T) {
	body := `<html><head><script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>` +
		`<script src="https://static.cloudflareinsights.com/beacon.min.js"></script></head>` +
		`<body><h1>Leche Entera</h1>` +
		`<div class="wpforms-recaptcha-container"></div>` +
		`<script>var config={"readOnlyDisallowedFields":["captcha"]};</script></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		_, _ = respuesta.Write([]byte(body))
	}))
	defer server.Close()

	response, err := testClient(0).Fetch(context.Background(), server.URL+"/producto")
	if err != nil {
		prueba.Fatalf("Fetch() error = %v", err)
	}
	if string(response.Body) != body {
		prueba.Errorf("body = %q", response.Body)
	}
}

func TestFetchReintentaEstadosTransitorios(prueba *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		if calls.Add(1) == 1 {
			respuesta.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer server.Close()

	response, err := testClient(1).Fetch(context.Background(), server.URL+"/retry")
	if err != nil {
		prueba.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusOK || string(response.Body) != "ok" {
		prueba.Errorf("response = %d %q", response.Status, response.Body)
	}
	if calls.Load() != 2 {
		prueba.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestFetchReportaBloqueoPersistente(prueba *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		respuesta.WriteHeader(http.StatusForbidden)
		_, _ = respuesta.Write([]byte("Just a moment"))
	}))
	defer server.Close()

	_, err := testClient(1).Fetch(context.Background(), server.URL+"/blocked")
	var blocked *crawler.BlockedError
	if !errors.As(err, &blocked) {
		prueba.Fatalf("error = %T %v, want *BlockedError", err, err)
	}
	if blocked.BlockedResponses != 2 {
		prueba.Errorf("BlockedResponses = %d, want 2 (intento inicial + reintento)", blocked.BlockedResponses)
	}
}

func TestFetchClasificaEstadosDeBloqueo(prueba *testing.T) {
	for _, status := range []int{
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		prueba.Run(http.StatusText(status), func(prueba *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				if peticion.URL.Path == "/robots.txt" {
					http.NotFound(respuesta, peticion)
					return
				}
				respuesta.WriteHeader(status)
			}))
			defer server.Close()

			_, err := testClient(0).Fetch(context.Background(), server.URL+"/blocked")
			var blocked *crawler.BlockedError
			if !errors.As(err, &blocked) {
				prueba.Fatalf("error = %T %v, want *BlockedError", err, err)
			}
			if blocked.Status != status {
				prueba.Errorf("status = %d, want %d", blocked.Status, status)
			}
			if blocked.BlockedResponses != 1 {
				prueba.Errorf("BlockedResponses = %d, want 1", blocked.BlockedResponses)
			}
		})
	}
}

func TestFetchDetectaMarcadoresDeChallenge(prueba *testing.T) {
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
		prueba.Run(name, func(prueba *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				if peticion.URL.Path == "/robots.txt" {
					http.NotFound(respuesta, peticion)
					return
				}
				_, _ = respuesta.Write([]byte(body))
			}))
			defer server.Close()

			_, err := testClient(0).Fetch(context.Background(), server.URL+"/challenge")
			var blocked *crawler.BlockedError
			if !errors.As(err, &blocked) {
				prueba.Fatalf("error = %T %v, want *BlockedError", err, err)
			}
			if blocked.Reason == "" {
				prueba.Error("BlockedError.Reason vacío")
			}
		})
	}
}

func TestFetchReintentaTimeout(prueba *testing.T) {
	var pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		pageCalls.Add(1)
		time.Sleep(100 * time.Millisecond)
		_, _ = respuesta.Write([]byte("late"))
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
		prueba.Fatal("Fetch() debió fallar por timeout")
	}
	if pageCalls.Load() != 2 {
		prueba.Fatalf("calls = %d, want 2 (timeout + reintento)", pageCalls.Load())
	}
}

func TestFetchRespetaRobots(prueba *testing.T) {
	var pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			_, _ = respuesta.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		pageCalls.Add(1)
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer server.Close()

	_, err := testClient(0).Fetch(context.Background(), server.URL+"/private")
	if !errors.Is(err, crawler.ErrRobotsDisallowed) {
		prueba.Fatalf("error = %v, want ErrRobotsDisallowed", err)
	}
	if pageCalls.Load() != 0 {
		prueba.Errorf("se solicitó una URL prohibida %d veces", pageCalls.Load())
	}
}

func TestFetchRespetaRetryAfter(prueba *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		if calls.Add(1) == 1 {
			respuesta.Header().Set("Retry-After", "1")
			respuesta.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer server.Close()

	start := time.Now()
	response, err := testClient(1).Fetch(context.Background(), server.URL+"/retry")
	if err != nil {
		prueba.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusOK {
		prueba.Errorf("status = %d", response.Status)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		prueba.Errorf("Fetch() esperó %v; debía respetar Retry-After de 1 s", elapsed)
	}
}

func TestFetchRechazaRespuestasDemasiadoGrandes(prueba *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		_, _ = respuesta.Write([]byte("0123456789"))
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
		prueba.Fatalf("error = %v, want ErrResponseTooLarge", err)
	}
}
