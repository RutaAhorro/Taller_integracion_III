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

// Un 200 real con scripts públicos de Cloudflare/CDN no es un challenge: el
// cuerpo debe entregarse al spider en vez de abortar el raspado.
func TestFetchAcceptsPageWithCDNScripts(t *testing.T) {
	body := `<html><head><script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>` +
		`<script src="https://static.cloudflareinsights.com/beacon.min.js"></script></head>` +
		`<body><h1>Leche Entera</h1></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	response, err := testClient().Fetch(context.Background(), server.URL+"/producto")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if string(response.Body) != body {
		t.Errorf("body = %q", response.Body)
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
	if blocked.BlockedResponses != 2 {
		t.Errorf("BlockedResponses = %d, want 2 (intento inicial + reintento)", blocked.BlockedResponses)
	}
}

func TestFetchClassifiesBlockStatuses(t *testing.T) {
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

			client := New(Options{
				UserAgent:  "TallerIntegracionIII/1.0",
				Delay:      time.Millisecond,
				Timeout:    time.Second,
				MaxSize:    1 << 20,
				MaxRetries: 0,
				Logger:     testLogger(),
			})
			_, err := client.Fetch(context.Background(), server.URL+"/blocked")
			var blocked *BlockedError
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

func TestDetectBlockClassifiesStatusesAndBodies(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "403", status: http.StatusForbidden, want: "http_403"},
		{name: "429", status: http.StatusTooManyRequests, want: "http_429"},
		{name: "503", status: http.StatusServiceUnavailable, want: "http_503"},
		{name: "504", status: http.StatusGatewayTimeout, want: "http_504"},
		{name: "cloudflare", status: http.StatusOK, body: "cf-chl", want: "cf-chl"},
		{name: "just a moment", status: http.StatusOK, body: "Just a moment...", want: "just a moment"},
		{name: "access denied", status: http.StatusOK, body: "<h1>Access Denied</h1>", want: "access denied"},
		{name: "unusual traffic", status: http.StatusOK, body: "Our systems have detected unusual traffic", want: "unusual traffic"},
		{name: "standalone captcha", status: http.StatusOK, body: `<div id="captcha"></div>`, want: "captcha"},
		{name: "captcha word boundary", status: http.StatusOK, body: "please solve the captcha.", want: "captcha"},
		{name: "captcha verification", status: http.StatusOK, body: "captcha verification required", want: "captcha"},
		{name: "recaptcha is normal", status: http.StatusOK, body: "grecaptcha-badge reCAPTCHA", want: ""},
		{name: "wpforms captcha config is normal", status: http.StatusOK, body: `<div class="wpforms-recaptcha-container"></div><script>{"readOnlyDisallowedFields":["captcha","repeater"]}</script>`, want: ""},
		{name: "cdnjs cloudflare script is normal", status: http.StatusOK, body: `<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script><h1>Leche</h1>`, want: ""},
		{name: "cloudflare insights is normal", status: http.StatusOK, body: `<script src="https://static.cloudflareinsights.com/beacon.min.js"></script>`, want: ""},
		{name: "cloudflare word in product name is normal", status: http.StatusOK, body: `<h1>Set Cloudflare 4 unidades</h1>`, want: ""},
		{name: "404 keeps status for the spider", status: http.StatusNotFound, body: "Access Denied", want: ""},
		{name: "normal page", status: http.StatusOK, body: "<h1>Frutas</h1>", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectBlock(test.status, []byte(test.body)); got != test.want {
				t.Errorf("detectBlock() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "seconds", value: "3", want: 3 * time.Second},
		{name: "zero", value: "0", want: 0},
		{name: "negative", value: "-2", want: 0},
		{name: "invalid", value: "soon", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			if test.value != "" {
				header.Set("Retry-After", test.value)
			}
			if got := retryAfter(header); got != test.want {
				t.Errorf("retryAfter() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	start := time.Now()
	if err := testClient().backoff(context.Background(), 0, 20*time.Millisecond); err != nil {
		t.Fatalf("backoff() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("backoff() = %v, esperaba al menos Retry-After de 20ms", elapsed)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	header := http.Header{}
	when := time.Now().Add(2 * time.Second).UTC().Truncate(time.Second)
	header.Set("Retry-After", when.Format(http.TimeFormat))
	if got := retryAfter(header); got < time.Second || got > 3*time.Second {
		t.Fatalf("retryAfter() = %v, want ~2s", got)
	}
}

func TestFetchRetriesTimeout(t *testing.T) {
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

	client := New(Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    20 * time.Millisecond,
		MaxSize:    1 << 20,
		MaxRetries: 1,
		Logger:     testLogger(),
	})
	if _, err := client.Fetch(context.Background(), server.URL+"/slow"); err == nil {
		t.Fatal("Fetch() debió fallar por timeout")
	}
	if pageCalls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (timeout + reintento)", pageCalls.Load())
	}
}

func TestDetectBlockIgnoresRecaptchaOnNormalPage(t *testing.T) {
	body := []byte(`<html><head><title>Frutas y Verduras - Supermercado Cugat</title></head>` +
		`<body><style>/* Hides the reCAPTCHA on every page */ .grecaptcha-badge { display:none; }</style>` +
		`<div class="wpforms-recaptcha-container wpforms-is-recaptcha"></div>` +
		`<script>var config={"readOnlyDisallowedFields":["captcha","repeater"]};</script>` +
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
