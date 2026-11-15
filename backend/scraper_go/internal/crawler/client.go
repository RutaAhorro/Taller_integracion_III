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
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
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

// Response es la respuesta HTTP aceptada: URL final, estado, cabeceras y
// cuerpo.
type Response struct {
	URL    *url.URL
	Status int
	Header http.Header
	Body   []byte
}

// BlockedError indica que el sitio bloqueó la petición por estado HTTP o por
// un challenge.
type BlockedError struct {
	URL    string
	Status int
	Reason string
	// BlockedResponses cuenta las respuestas bloqueadas de esta URL,
	// incluyendo reintentos. El worker la incluye en el cierre del trabajo
	// para que un fallo por bloqueo no se confunda con un crawl vacío o un
	// error de red.
	BlockedResponses int
}

// Error implementa error con URL, status y motivo del bloqueo.
func (errorEstado *BlockedError) Error() string {
	return fmt.Sprintf("bloqueo detectado en %s (status=%d, motivo=%s)", errorEstado.URL, errorEstado.Status, errorEstado.Reason)
}

// HTTPStatusError representa un estado HTTP que el crawler no puede
// recuperar: 5xx tras agotar los reintentos o 4xx distinto de 404.
type HTTPStatusError struct {
	URL    string
	Status int
}

// Error implementa error con el estado y la URL.
func (errorEstado *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d en %s", errorEstado.Status, errorEstado.URL)
}

// Options agrupa la configuración del crawler: identidad, tiempos, tamaño
// máximo, reintentos, robots.txt y backoff.
type Options struct {
	UserAgent        string
	Delay            time.Duration
	Timeout          time.Duration
	MaxSize          int64
	MaxRetries       int
	RobotsCacheTTL   time.Duration
	RobotsFailureTTL time.Duration
	RobotsMaxBytes   int64
	BackoffMin       time.Duration
	BackoffMax       time.Duration
	Logger           *slog.Logger
}

// ===== Estructura de Cliente y Opciones =====

// Client descarga páginas respetando robots.txt, throttle por dominio,
// tamaño máximo y reintentos con backoff.
type Client struct {
	http       *http.Client
	userAgent  string
	delay      time.Duration
	maxSize    int64
	maxRetries int
	backoffMin time.Duration
	backoffMax time.Duration
	logger     *slog.Logger
	robots     *robotsCache
	throttle   *throttle
}

// ===== Constructor =====

// New construye el cliente y aplica valores seguros a los campos de Options
// que lleguen en cero.
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
	if opts.RobotsCacheTTL <= 0 {
		opts.RobotsCacheTTL = time.Hour
	}
	if opts.RobotsFailureTTL <= 0 {
		opts.RobotsFailureTTL = 10 * time.Minute
	}
	if opts.RobotsMaxBytes <= 0 {
		opts.RobotsMaxBytes = 512 * 1024
	}
	if opts.BackoffMin <= 0 {
		opts.BackoffMin = 10 * time.Millisecond
	}
	if opts.BackoffMax <= 0 {
		opts.BackoffMax = 120 * time.Second
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
		backoffMin: opts.BackoffMin,
		backoffMax: opts.BackoffMax,
		logger:     opts.Logger,
		throttle:   newThrottle(),
	}
	client.robots = newRobotsCache(
		client.http,
		client.userAgent,
		opts.RobotsCacheTTL,
		opts.RobotsFailureTTL,
		opts.RobotsMaxBytes,
		client.logger,
	)
	return client
}

// ===== Cliente HTTP =====

// newHTTPClient instancia un cliente HTTP con timeouts estrictos para crawling.
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

// ===== Método Público Fetch =====

// Fetch aplica robots, throttle y reintentos. Un 404 se devuelve como
// respuesta para que el spider decida si termina la paginación; un bloqueo
// persistente se devuelve como BlockedError.
func (cliente *Client) Fetch(ctx context.Context, rawURL string) (*Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL no soportada: %s", rawURL)
	}

	allowed, crawlDelay, err := cliente.robots.Allowed(ctx, parsed)
	if err != nil {
		cliente.logger.Warn("scraper_robots_unavailable", "url", rawURL, "error", err)
	}
	if !allowed {
		return nil, fmt.Errorf("%w: %s", ErrRobotsDisallowed, rawURL)
	}
	delay := cliente.delay
	if crawlDelay > delay {
		delay = crawlDelay
	}

	blockedResponses := 0
	for attempt := 0; attempt <= cliente.maxRetries; attempt++ {
		if err := cliente.throttle.Wait(ctx, parsed.Hostname(), delay); err != nil {
			return nil, err
		}
		response, err := cliente.doRequest(ctx, rawURL)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt < cliente.maxRetries {
				if err := cliente.backoff(ctx, attempt, 0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}

		if reason := detectBlock(response.Status, response.Body); reason != "" {
			blockedResponses++
			cliente.logger.Warn("scraper_blocked",
				"url", rawURL,
				"status", response.Status,
				"reason", reason,
				"attempt", attempt+1,
			)
			if attempt < cliente.maxRetries {
				if err := cliente.backoff(ctx, attempt, retryAfter(response.Header)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &BlockedError{
				URL:              rawURL,
				Status:           response.Status,
				Reason:           reason,
				BlockedResponses: blockedResponses,
			}
		}

		if response.Status >= 500 {
			if attempt < cliente.maxRetries {
				if err := cliente.backoff(ctx, attempt, retryAfter(response.Header)); err != nil {
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

// ===== Petición HTTP y Transporte =====

// doRequest ejecuta la solicitud HTTP GET incorporando cabeceras de navegación.
func (cliente *Client) doRequest(ctx context.Context, rawURL string) (*Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", cliente.userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	response, err := cliente.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, cliente.maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > cliente.maxSize {
		return nil, ErrResponseTooLarge
	}
	return &Response{
		URL:    response.Request.URL,
		Status: response.StatusCode,
		Header: response.Header,
		Body:   body,
	}, nil
}

// ===== Reintentos y Espera =====

// backoff calcula el tiempo de espera exponencial con variación aleatoria para reintentos.
func (cliente *Client) backoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	wait := retryAfter
	if wait <= 0 {
		base := cliente.delay
		if base < cliente.backoffMin {
			base = cliente.backoffMin
		}
		exponential := base * time.Duration(1<<min(attempt, 7))
		// Jitter completo sobre la mitad superior: conserva el crecimiento
		// exponencial y evita que varios workers reintenten a la vez.
		half := exponential / 2
		wait = half + time.Duration(rand.Int64N(int64(half)+1))
	}
	// Retry-After es una instrucción del servidor: se respeta incluso si es
	// mayor que el techo del backoff propio. El techo sólo aplica al jitter.
	if retryAfter <= 0 && wait > cliente.backoffMax {
		wait = cliente.backoffMax
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

// retryAfter interpreta la cabecera Retry-After en segundos o formato HTTP-date.
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

// No se busca "captcha" como subcadena: las páginas normales de Cugat
// incluyen el CSS y la configuración de reCAPTCHA/WPForms, y se marcaban como
// bloqueadas sin estarlo. Los marcadores deben identificar el challenge, no
// cualquier script de terceros. Por eso "captcha" sólo cuenta si aparece en
// una instrucción de resolución o en un campo exacto del desafío.
var (
	blockMarkers = []string{
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
		"access denied",
	}
	standaloneCaptcha = regexp.MustCompile(
		`(?i)(solve|complete|enter|verify|type|submit)\s+(the\s+)?captcha` +
			`|captcha\s+(required|verification|challenge|test|code)` +
			`|(id|name)\s*=\s*["']captcha["']`,
	)
)

// ===== Detección de Bloqueos =====

// detectBlock analiza la respuesta para detectar patrones de bloqueo o desafíos antibot.
func detectBlock(status int, body []byte) string {
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Sprintf("http_%d", status)
	}
	// Los marcadores de challenge sólo se evalúan en respuestas 200: son las
	// que un CDN/WAF usa para servir la página de verificación sin cambiar el
	// status. En 4xx/5xx no bloqueantes (por ejemplo 404) el cuerpo se deja al
	// spider, que decide si es fin de paginación o un producto inexistente.
	if status != http.StatusOK || len(body) == 0 {
		return ""
	}
	lower := strings.ToLower(string(body))
	for _, marker := range blockMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	if standaloneCaptcha.MatchString(lower) {
		return "captcha"
	}
	return ""
}

// ===== Regulación de Tasa (Throttle) =====

// throttle gestiona la cadencia de peticiones por host para evitar saturaciones.
type throttle struct {
	mu   sync.Mutex
	next map[string]time.Time
}

// newThrottle instancia un regulador de frecuencia de peticiones por intervalo.
func newThrottle() *throttle {
	return &throttle{next: make(map[string]time.Time)}
}

// Wait reserva el siguiente turno del dominio. La reserva se hace bajo lock
// para que dos goroutines no calculen el mismo instante.
func (regulador *throttle) Wait(ctx context.Context, host string, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	regulador.mu.Lock()
	now := time.Now()
	next := regulador.next[host]
	if next.Before(now) {
		next = now
	}
	regulador.next[host] = next.Add(delay)
	regulador.mu.Unlock()

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

// ===== Gestión de Robots.txt =====

// robotsEntry almacena las reglas y tiempos de crawl-delay para un dominio.
type robotsEntry struct {
	data       *robotstxt.RobotsData
	crawlDelay time.Duration
	fetched    time.Time
}

// robotsCache mantiene en memoria las directivas de robots.txt por host.
type robotsCache struct {
	mu         sync.Mutex
	entries    map[string]*robotsEntry
	failures   map[string]time.Time
	http       *http.Client
	userAgent  string
	cacheTTL   time.Duration
	failureTTL time.Duration
	maxBytes   int64
	logger     *slog.Logger
}

// newRobotsCache inicializa el caché de directivas de robots.txt.
func newRobotsCache(
	client *http.Client,
	userAgent string,
	cacheTTL time.Duration,
	failureTTL time.Duration,
	maxBytes int64,
	logger *slog.Logger,
) *robotsCache {
	return &robotsCache{
		entries:    make(map[string]*robotsEntry),
		failures:   make(map[string]time.Time),
		http:       client,
		userAgent:  userAgent,
		cacheTTL:   cacheTTL,
		failureTTL: failureTTL,
		maxBytes:   maxBytes,
		logger:     logger,
	}
}

// Allowed determina si una URL cumple las reglas de exclusión de robots.txt.
func (cacheRobots *robotsCache) Allowed(ctx context.Context, target *url.URL) (bool, time.Duration, error) {
	base := target.Scheme + "://" + target.Host
	cacheRobots.mu.Lock()
	entry := cacheRobots.entries[base]
	failedAt, failed := cacheRobots.failures[base]
	cacheRobots.mu.Unlock()

	if entry == nil || time.Since(entry.fetched) > cacheRobots.cacheTTL {
		if failed && time.Since(failedAt) < cacheRobots.failureTTL {
			// Fail-open sin repetir el aviso en cada pagina. Cugat, por
			// ejemplo, redirige robots.txt en bucle; sin esta cache el
			// worker registraria un warning por cada categoria.
			return true, 0, nil
		}
		fetched, err := cacheRobots.fetch(ctx, base)
		if err != nil {
			if ctx.Err() == nil {
				cacheRobots.mu.Lock()
				cacheRobots.failures[base] = time.Now()
				cacheRobots.mu.Unlock()
			}
			return true, 0, err
		}
		entry = fetched
		cacheRobots.mu.Lock()
		cacheRobots.entries[base] = entry
		delete(cacheRobots.failures, base)
		cacheRobots.mu.Unlock()
	}

	path := target.RequestURI()
	if path == "" {
		path = "/"
	}
	return entry.data.TestAgent(path, cacheRobots.userAgent), entry.crawlDelay, nil
}

// fetch descarga y procesa el archivo robots.txt correspondiente a un host.
func (cacheRobots *robotsCache) fetch(ctx context.Context, base string) (*robotsEntry, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/robots.txt", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", cacheRobots.userAgent)
	response, err := cacheRobots.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, cacheRobots.maxBytes))
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
		crawlDelay: parseCrawlDelay(body, cacheRobots.userAgent),
		fetched:    time.Now(),
	}, nil
}

// parseCrawlDelay busca la directiva Crawl-delay aplicable en robots.txt.
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

// matchesAgent verifica si una directiva de User-agent aplica al cliente actual.
func matchesAgent(agents []string, userAgent string) bool {
	lowerUser := strings.ToLower(userAgent)
	for _, agent := range agents {
		if agent == "*" || strings.Contains(lowerUser, agent) || strings.Contains(agent, "tallerintegracion") {
			return true
		}
	}
	return false
}
