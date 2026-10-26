package crawler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClient() *Client {
	return New(Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    2 * time.Second,
		MaxSize:    1 << 20,
		MaxRetries: 1,
		Logger:     testLogger(),
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRobotsCacheCachesFailure(t *testing.T) {
	var calls atomic.Int32
	cache := newRobotsCache(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("robots no disponible")
		}),
	}, "TallerIntegracionIII/1.0", testLogger())

	target, err := url.Parse("https://example.test/frutas")
	if err != nil {
		t.Fatal(err)
	}

	allowed, _, err := cache.Allowed(context.Background(), target)
	if err == nil {
		t.Fatal("la primera consulta debe devolver el error de robots")
	}
	if !allowed {
		t.Fatal("un fallo de robots debe permitir el crawl (fail-open)")
	}

	allowed, _, err = cache.Allowed(context.Background(), target)
	if err != nil {
		t.Fatalf("la segunda consulta no debe repetir el error: %v", err)
	}
	if !allowed {
		t.Fatal("la cache de fallos debe seguir permitiendo el crawl")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("robots.txt se consulto %d veces; se esperaba 1", got)
	}
}

func TestFetchReturns404ForSpiderPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	response, err := testClient().Fetch(context.Background(), server.URL+"/missing")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if response.Status != http.StatusNotFound {
		t.Errorf("status = %d", response.Status)
	}
}

func TestFetchRetriesTransientStatus(t *testing.T) {
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

	response, err := testClient().Fetch(context.Background(), server.URL+"/retry")
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

func TestFetchReportsPersistentBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Just a moment"))
	}))
	defer server.Close()

	_, err := testClient().Fetch(context.Background(), server.URL+"/blocked")
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("error = %T %v, want *BlockedError", err, err)
	}
}

func TestDetectBlockIgnoresRecaptchaOnNormalPage(t *testing.T) {
	body := []byte(`<html><head><title>Frutas y Verduras - Supermercado Cugat</title></head>` +
		`<body><style>/* Hides the reCAPTCHA on every page */ .grecaptcha-badge { display:none; }</style>` +
		`<div class="product">Palta Hass</div></body></html>`)
	if reason := detectBlock(http.StatusOK, body); reason != "" {
		t.Errorf("detectBlock() = %q, want empty", reason)
	}
}

func TestDetectBlockRecognizesPerimeterX(t *testing.T) {
	body := []byte(`<html><head><title>Robot or human?</title></head>` +
		`<body><div id="px-captcha"></div></body></html>`)
	if reason := detectBlock(http.StatusOK, body); reason == "" {
		t.Fatal("detectBlock() no detectó el challenge de PerimeterX")
	}
}

func TestFetchHonorsRobots(t *testing.T) {
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

	_, err := testClient().Fetch(context.Background(), server.URL+"/private")
	if !errors.Is(err, ErrRobotsDisallowed) {
		t.Fatalf("error = %v, want ErrRobotsDisallowed", err)
	}
	if pageCalls.Load() != 0 {
		t.Errorf("se solicitó una URL prohibida %d veces", pageCalls.Load())
	}
}
