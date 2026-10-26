// Package crawler encapsula el acceso HTTP del scraper: robots.txt,
// throttling por dominio, reintentos y detección de bloqueos.
package crawler

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// Acuenta no envía el certificado intermedio en el handshake TLS. El
// intermedio público de GlobalSign se agrega como ancla adicional para poder
// verificar la cadena sin desactivar la validación. Vence el 21/11/2028.
//
//go:embed certs/globalsign-rsa-ov-ssl-ca-2018.pem
var extraCACert []byte

var (
	ErrRobotsDisallowed = errors.New("robots.txt no permite la URL")
	ErrResponseTooLarge = errors.New("la respuesta supera DOWNLOAD_MAXSIZE")
)

type Response struct {
	URL    *url.URL
	Status int
	Header http.Header
	Body   []byte
}

type BlockedError struct {
	URL    string
	Status int
	Reason string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("bloqueo detectado en %s (status=%d, motivo=%s)", e.URL, e.Status, e.Reason)
}

type HTTPStatusError struct {
	URL    string
	Status int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d en %s", e.Status, e.URL)
}

type Options struct {
	UserAgent  string
	Delay      time.Duration
	Timeout    time.Duration
	MaxSize    int64
	MaxRetries int
	Logger     *slog.Logger
}

type Client struct {
	http       *http.Client
	userAgent  string
	delay      time.Duration
	maxSize    int64
	maxRetries int
	logger     *slog.Logger
	robots     *robotsCache
	throttle   *throttle
}

func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxSize <= 0 {
		opts.MaxSize = 5 * 1024 * 1024
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	client := &Client{
		http:       newHTTPClient(opts.Timeout),
		userAgent:  opts.UserAgent,
		delay:      opts.Delay,
		maxSize:    opts.MaxSize,
		maxRetries: opts.MaxRetries,
		logger:     opts.Logger,
		throttle:   newThrottle(),
	}
	client.robots = newRobotsCache(client.http, client.userAgent, client.logger)
	return client
}

func newHTTPClient(timeout time.Duration) *http.Client {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if len(extraCACert) > 0 {
		roots.AppendCertsFromPEM(extraCACert)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS12,
	}
	// Scrapy usa HTTP/1.1. Forzarlo evita que algunos anti-bot rechacen la
	// huella HTTP/2 de Go.
	transport.ForceAttemptHTTP2 = false

	// El jar conserva las cookies entre robots.txt y la página, igual que Scrapy.
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   timeout,
	}
}

// Fetch aplica robots, throttle y reintentos. Un 404 se devuelve como
// respuesta para que el spider decida si termina la paginación; un bloqueo
// persistente se devuelve como BlockedError.
func (c *Client) Fetch(ctx context.Context, rawURL string) (*Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL no soportada: %s", rawURL)
	}

	allowed, crawlDelay, err := c.robots.Allowed(ctx, parsed)
	if err != nil {
		c.logger.Warn("scraper_robots_unavailable", "url", rawURL, "error", err)
	}
	if !allowed {
		return nil, fmt.Errorf("%w: %s", ErrRobotsDisallowed, rawURL)
	}
	delay := c.delay
	if crawlDelay > delay {
		delay = crawlDelay
	}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.throttle.Wait(ctx, parsed.Hostname(), delay); err != nil {
			return nil, err
		}
		response, err := c.doRequest(ctx, rawURL)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt < c.maxRetries {
				if err := c.backoff(ctx, attempt, 0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}

		if reason := detectBlock(response.Status, response.Body); reason != "" {
			c.logger.Warn("scraper_blocked",
				"url", rawURL,
				"status", response.Status,
				"reason", reason,
				"attempt", attempt+1,
			)
			if attempt < c.maxRetries {
				if err := c.backoff(ctx, attempt, retryAfter(response.Header)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &BlockedError{URL: rawURL, Status: response.Status, Reason: reason}
		}

		if response.Status >= 500 {
			if attempt < c.maxRetries {
				if err := c.backoff(ctx, attempt, retryAfter(response.Header)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &HTTPStatusError{URL: rawURL, Status: response.Status}
		}
		if response.Status >= 400 && response.Status != http.StatusNotFound {
			return nil, &HTTPStatusError{URL: rawURL, Status: response.Status}
		}
		return response, nil
	}
	return nil, fmt.Errorf("no se pudo obtener %s", rawURL)
}

func (c *Client) doRequest(ctx context.Context, rawURL string) (*Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, c.maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > c.maxSize {
		return nil, ErrResponseTooLarge
	}
	return &Response{
		URL:    response.Request.URL,
		Status: response.StatusCode,
		Header: response.Header,
		Body:   body,
	}, nil
}

func (c *Client) backoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	wait := retryAfter
	if wait <= 0 {
		base := c.delay
		if base < 10*time.Millisecond {
			base = 10 * time.Millisecond
		}
		wait = base * time.Duration(1<<min(attempt, 7))
	}
	if wait > 120*time.Second {
		wait = 120 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}

// No se busca "captcha" a secas: las páginas normales de Cugat incluyen el
// CSS de reCAPTCHA y se marcaban como bloqueadas sin estarlo. Los marcadores
// deben identificar el challenge, no cualquier script de terceros.
var blockMarkers = []string{
	"just a moment",
	"cf-chl",
	"challenge-platform",
	"unusual traffic",
	"enable javascript and cookies",
	"robot or human?",
	"px-captcha",
	"captcha-delivery",
	"verify you are human",
	"attention required!",
}

func detectBlock(status int, body []byte) string {
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Sprintf("http_%d", status)
	}
	if len(body) == 0 {
		return ""
	}
	lower := strings.ToLower(string(body))
	for _, marker := range blockMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}

type throttle struct {
	mu   sync.Mutex
	next map[string]time.Time
}

func newThrottle() *throttle {
	return &throttle{next: make(map[string]time.Time)}
}

// Wait reserva el siguiente turno del dominio. La reserva se hace bajo lock
// para que dos goroutines no calculen el mismo instante.
func (t *throttle) Wait(ctx context.Context, host string, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	t.mu.Lock()
	now := time.Now()
	next := t.next[host]
	if next.Before(now) {
		next = now
	}
	t.next[host] = next.Add(delay)
	t.mu.Unlock()

	wait := time.Until(next)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type robotsEntry struct {
	data       *robotstxt.RobotsData
	crawlDelay time.Duration
	fetched    time.Time
}

type robotsCache struct {
	mu        sync.Mutex
	entries   map[string]*robotsEntry
	http      *http.Client
	userAgent string
	logger    *slog.Logger
}

func newRobotsCache(client *http.Client, userAgent string, logger *slog.Logger) *robotsCache {
	return &robotsCache{
		entries:   make(map[string]*robotsEntry),
		http:      client,
		userAgent: userAgent,
		logger:    logger,
	}
}

func (r *robotsCache) Allowed(ctx context.Context, target *url.URL) (bool, time.Duration, error) {
	base := target.Scheme + "://" + target.Host
	r.mu.Lock()
	entry := r.entries[base]
	r.mu.Unlock()

	if entry == nil || time.Since(entry.fetched) > time.Hour {
		fetched, err := r.fetch(ctx, base)
		if err != nil {
			return true, 0, err
		}
		entry = fetched
		r.mu.Lock()
		r.entries[base] = entry
		r.mu.Unlock()
	}

	path := target.RequestURI()
	if path == "" {
		path = "/"
	}
	return entry.data.TestAgent(path, r.userAgent), entry.crawlDelay, nil
}

func (r *robotsCache) fetch(ctx context.Context, base string) (*robotsEntry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/robots.txt", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", r.userAgent)
	response, err := r.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		body = nil
	}
	data, parseErr := robotstxt.FromBytes(body)
	if parseErr != nil {
		// Un robots.txt inválido no debe bloquear el crawl; se comporta como
		// ausente, igual que Scrapy ante un error de parseo.
		data, _ = robotstxt.FromBytes(nil)
	}
	return &robotsEntry{
		data:       data,
		crawlDelay: parseCrawlDelay(body, r.userAgent),
		fetched:    time.Now(),
	}, nil
}

func parseCrawlDelay(body []byte, userAgent string) time.Duration {
	var (
		agents []string
		delay  time.Duration
	)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			agents = nil
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "user-agent:"):
			agents = append(agents, strings.TrimSpace(lower[len("user-agent:"):]))
		case strings.HasPrefix(lower, "crawl-delay:"):
			if !matchesAgent(agents, userAgent) {
				continue
			}
			raw := strings.TrimSpace(lower[len("crawl-delay:"):])
			seconds, err := strconv.ParseFloat(raw, 64)
			if err != nil || seconds <= 0 {
				continue
			}
			candidate := time.Duration(seconds * float64(time.Second))
			if candidate > delay {
				delay = candidate
			}
		}
	}
	return delay
}

func matchesAgent(agents []string, userAgent string) bool {
	lowerUser := strings.ToLower(userAgent)
	for _, agent := range agents {
		if agent == "*" || strings.Contains(lowerUser, agent) || strings.Contains(agent, "tallerintegracion") {
			return true
		}
	}
	return false
}
