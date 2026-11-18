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

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
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
func New(opciones Options) *Client {
	configuracionPredeterminada := config.Default()
	if opciones.Timeout <= 0 {
		opciones.Timeout = configuracionPredeterminada.DownloadTimeout
	}
	if opciones.MaxSize <= 0 {
		opciones.MaxSize = configuracionPredeterminada.MaxDownloadSize
	}
	if opciones.MaxRetries < 0 {
		opciones.MaxRetries = 0
	}
	if opciones.RobotsCacheTTL <= 0 {
		opciones.RobotsCacheTTL = configuracionPredeterminada.RobotsCacheTTL
	}
	if opciones.RobotsFailureTTL <= 0 {
		opciones.RobotsFailureTTL = configuracionPredeterminada.RobotsFailureTTL
	}
	if opciones.RobotsMaxBytes <= 0 {
		opciones.RobotsMaxBytes = configuracionPredeterminada.RobotsMaxBytes
	}
	if opciones.BackoffMin <= 0 {
		opciones.BackoffMin = configuracionPredeterminada.BackoffMin
	}
	if opciones.BackoffMax <= 0 {
		opciones.BackoffMax = configuracionPredeterminada.BackoffMax
	}
	if opciones.Logger == nil {
		opciones.Logger = slog.Default()
	}
	cliente := &Client{
		http:       newHTTPClient(opciones.Timeout),
		userAgent:  opciones.UserAgent,
		delay:      opciones.Delay,
		maxSize:    opciones.MaxSize,
		maxRetries: opciones.MaxRetries,
		backoffMin: opciones.BackoffMin,
		backoffMax: opciones.BackoffMax,
		logger:     opciones.Logger,
		throttle:   newThrottle(),
	}
	cliente.robots = newRobotsCache(
		cliente.http,
		cliente.userAgent,
		opciones.RobotsCacheTTL,
		opciones.RobotsFailureTTL,
		opciones.RobotsMaxBytes,
		cliente.logger,
	)
	return cliente
}

// ===== Cliente HTTP =====

// newHTTPClient instancia un cliente HTTP con timeouts estrictos para crawling.
func newHTTPClient(tiempoEspera time.Duration) *http.Client {
	raices, errorObtenido := x509.SystemCertPool()
	if errorObtenido != nil || raices == nil {
		raices = x509.NewCertPool()
	}
	if len(extraCACert) > 0 {
		raices.AppendCertsFromPEM(extraCACert)
	}

	transporte := http.DefaultTransport.(*http.Transport).Clone()
	transporte.TLSClientConfig = &tls.Config{
		RootCAs:    raices,
		MinVersion: tls.VersionTLS12,
	}
	// Scrapy usa HTTP/1.1. Forzarlo evita que algunos anti-bot rechacen la
	// huella HTTP/2 de Go.
	transporte.ForceAttemptHTTP2 = false

	// El jar conserva las cookies entre robots.txt y la página, igual que Scrapy.
	galletas, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: transporte,
		Jar:       galletas,
		Timeout:   tiempoEspera,
	}
}

// ===== Método Público Fetch =====

// Fetch aplica robots, throttle y reintentos. Un 404 se devuelve como
// respuesta para que el spider decida si termina la paginación; un bloqueo
// persistente se devuelve como BlockedError.
func (cliente *Client) Fetch(contexto context.Context, urlCruda string) (*Response, error) {
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if analizada.Scheme != "http" && analizada.Scheme != "https" {
		return nil, fmt.Errorf("URL no soportada: %s", urlCruda)
	}

	permitido, retrasoRastreo, errorObtenido := cliente.robots.Allowed(contexto, analizada)
	if errorObtenido != nil {
		cliente.logger.Warn("scraper_robots_unavailable", "url", urlCruda, "error", errorObtenido)
	}
	if !permitido {
		return nil, fmt.Errorf("%w: %s", ErrRobotsDisallowed, urlCruda)
	}
	retraso := cliente.delay
	if retrasoRastreo > retraso {
		retraso = retrasoRastreo
	}

	respuestasBloqueadas := 0
	for intento := 0; intento <= cliente.maxRetries; intento++ {
		if errorObtenido := cliente.throttle.Wait(contexto, analizada.Hostname(), retraso); errorObtenido != nil {
			return nil, errorObtenido
		}
		respuesta, errorObtenido := cliente.doRequest(contexto, urlCruda)
		if errorObtenido != nil {
			if contexto.Err() != nil {
				return nil, contexto.Err()
			}
			if intento < cliente.maxRetries {
				if errorObtenido := cliente.backoff(contexto, intento, 0); errorObtenido != nil {
					return nil, errorObtenido
				}
				continue
			}
			return nil, errorObtenido
		}

		if motivo := detectBlock(respuesta.Status, respuesta.Body); motivo != "" {
			respuestasBloqueadas++
			cliente.logger.Warn("scraper_blocked",
				"url", urlCruda,
				"status", respuesta.Status,
				"reason", motivo,
				"attempt", intento+1,
			)
			if intento < cliente.maxRetries {
				if errorObtenido := cliente.backoff(contexto, intento, retryAfter(respuesta.Header)); errorObtenido != nil {
					return nil, errorObtenido
				}
				continue
			}
			return nil, &BlockedError{
				URL:              urlCruda,
				Status:           respuesta.Status,
				Reason:           motivo,
				BlockedResponses: respuestasBloqueadas,
			}
		}

		if respuesta.Status >= 500 {
			if intento < cliente.maxRetries {
				if errorObtenido := cliente.backoff(contexto, intento, retryAfter(respuesta.Header)); errorObtenido != nil {
					return nil, errorObtenido
				}
				continue
			}
			return nil, &HTTPStatusError{URL: urlCruda, Status: respuesta.Status}
		}
		if respuesta.Status >= 400 && respuesta.Status != http.StatusNotFound {
			return nil, &HTTPStatusError{URL: urlCruda, Status: respuesta.Status}
		}
		return respuesta, nil
	}
	return nil, fmt.Errorf("no se pudo obtener %s", urlCruda)
}

// ===== Petición HTTP y Transporte =====

// doRequest ejecuta la solicitud HTTP GET incorporando cabeceras de navegación.
func (cliente *Client) doRequest(contexto context.Context, urlCruda string) (*Response, error) {
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodGet, urlCruda, nil)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("User-Agent", cliente.userAgent)
	peticion.Header.Set("Accept", "text/html,application/xhtml+xml")

	respuesta, errorObtenido := cliente.http.Do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer respuesta.Body.Close()

	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, cliente.maxSize+1))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if int64(len(cuerpo)) > cliente.maxSize {
		return nil, ErrResponseTooLarge
	}
	return &Response{
		URL:    respuesta.Request.URL,
		Status: respuesta.StatusCode,
		Header: respuesta.Header,
		Body:   cuerpo,
	}, nil
}

// ===== Reintentos y Espera =====

// backoff calcula el tiempo de espera exponencial con variación aleatoria para reintentos.
func (cliente *Client) backoff(contexto context.Context, intento int, esperaReintento time.Duration) error {
	espera := esperaReintento
	if espera <= 0 {
		base := cliente.delay
		if base < cliente.backoffMin {
			base = cliente.backoffMin
		}
		exponencial := base * time.Duration(1<<min(intento, 7))
		// Jitter completo sobre la mitad superior: conserva el crecimiento
		// exponencial y evita que varios workers reintenten a la vez.
		mitad := exponencial / 2
		espera = mitad + time.Duration(rand.Int64N(int64(mitad)+1))
	}
	// Retry-After es una instrucción del servidor: se respeta incluso si es
	// mayor que el techo del backoff propio. El techo sólo aplica al jitter.
	if esperaReintento <= 0 && espera > cliente.backoffMax {
		espera = cliente.backoffMax
	}
	temporizador := time.NewTimer(espera)
	defer temporizador.Stop()
	select {
	case <-contexto.Done():
		return contexto.Err()
	case <-temporizador.C:
		return nil
	}
}

// retryAfter interpreta la cabecera Retry-After en segundos o formato HTTP-date.
func retryAfter(cabecera http.Header) time.Duration {
	valor := strings.TrimSpace(cabecera.Get("Retry-After"))
	if valor == "" {
		return 0
	}
	if segundos, errorObtenido := strconv.Atoi(valor); errorObtenido == nil {
		if segundos < 0 {
			return 0
		}
		return time.Duration(segundos) * time.Second
	}
	if cuando, errorObtenido := http.ParseTime(valor); errorObtenido == nil {
		if espera := time.Until(cuando); espera > 0 {
			return espera
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
func detectBlock(estado int, cuerpo []byte) string {
	switch estado {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Sprintf("http_%d", estado)
	}
	// Los marcadores de challenge sólo se evalúan en respuestas 200: son las
	// que un CDN/WAF usa para servir la página de verificación sin cambiar el
	// status. En 4xx/5xx no bloqueantes (por ejemplo 404) el cuerpo se deja al
	// spider, que decide si es fin de paginación o un producto inexistente.
	if estado != http.StatusOK || len(cuerpo) == 0 {
		return ""
	}
	minuscula := strings.ToLower(string(cuerpo))
	for _, marcador := range blockMarkers {
		if strings.Contains(minuscula, marcador) {
			return marcador
		}
	}
	if standaloneCaptcha.MatchString(minuscula) {
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
func (regulador *throttle) Wait(contexto context.Context, servidor string, retraso time.Duration) error {
	if retraso <= 0 {
		return nil
	}
	regulador.mu.Lock()
	ahora := time.Now()
	siguiente := regulador.next[servidor]
	if siguiente.Before(ahora) {
		siguiente = ahora
	}
	regulador.next[servidor] = siguiente.Add(retraso)
	regulador.mu.Unlock()

	espera := time.Until(siguiente)
	if espera <= 0 {
		return nil
	}
	temporizador := time.NewTimer(espera)
	defer temporizador.Stop()
	select {
	case <-contexto.Done():
		return contexto.Err()
	case <-temporizador.C:
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
	cliente *http.Client,
	agenteUsuario string,
	vidaCache time.Duration,
	vidaFallo time.Duration,
	maximoBytes int64,
	registrador *slog.Logger,
) *robotsCache {
	return &robotsCache{
		entries:    make(map[string]*robotsEntry),
		failures:   make(map[string]time.Time),
		http:       cliente,
		userAgent:  agenteUsuario,
		cacheTTL:   vidaCache,
		failureTTL: vidaFallo,
		maxBytes:   maximoBytes,
		logger:     registrador,
	}
}

// Allowed determina si una URL cumple las reglas de exclusión de robots.txt.
func (cacheRobots *robotsCache) Allowed(contexto context.Context, objetivo *url.URL) (bool, time.Duration, error) {
	base := objetivo.Scheme + "://" + objetivo.Host
	cacheRobots.mu.Lock()
	entrada := cacheRobots.entries[base]
	falladoEn, fallido := cacheRobots.failures[base]
	cacheRobots.mu.Unlock()

	if entrada == nil || time.Since(entrada.fetched) > cacheRobots.cacheTTL {
		if fallido && time.Since(falladoEn) < cacheRobots.failureTTL {
			// Fail-open sin repetir el aviso en cada pagina. Cugat, por
			// ejemplo, redirige robots.txt en bucle; sin esta cache el
			// worker registraria un warning por cada categoria.
			return true, 0, nil
		}
		obtenido, errorObtenido := cacheRobots.fetch(contexto, base)
		if errorObtenido != nil {
			if contexto.Err() == nil {
				cacheRobots.mu.Lock()
				cacheRobots.failures[base] = time.Now()
				cacheRobots.mu.Unlock()
			}
			return true, 0, errorObtenido
		}
		entrada = obtenido
		cacheRobots.mu.Lock()
		cacheRobots.entries[base] = entrada
		delete(cacheRobots.failures, base)
		cacheRobots.mu.Unlock()
	}

	ruta := objetivo.RequestURI()
	if ruta == "" {
		ruta = "/"
	}
	return entrada.data.TestAgent(ruta, cacheRobots.userAgent), entrada.crawlDelay, nil
}

// fetch descarga y procesa el archivo robots.txt correspondiente a un host.
func (cacheRobots *robotsCache) fetch(contexto context.Context, base string) (*robotsEntry, error) {
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodGet, base+"/robots.txt", nil)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("User-Agent", cacheRobots.userAgent)
	respuesta, errorObtenido := cacheRobots.http.Do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer respuesta.Body.Close()

	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, cacheRobots.maxBytes))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if respuesta.StatusCode >= 400 {
		cuerpo = nil
	}
	datos, errorParseo := robotstxt.FromBytes(cuerpo)
	if errorParseo != nil {
		// Un robots.txt inválido no debe bloquear el crawl; se comporta como
		// ausente, igual que Scrapy ante un error de parseo.
		datos, _ = robotstxt.FromBytes(nil)
	}
	return &robotsEntry{
		data:       datos,
		crawlDelay: parseCrawlDelay(cuerpo, cacheRobots.userAgent),
		fetched:    time.Now(),
	}, nil
}

// parseCrawlDelay busca la directiva Crawl-delay aplicable en robots.txt.
func parseCrawlDelay(cuerpo []byte, agenteUsuario string) time.Duration {
	var (
		agentes []string
		retraso time.Duration
	)
	for _, linea := range strings.Split(string(cuerpo), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			agentes = nil
			continue
		}
		minuscula := strings.ToLower(linea)
		switch {
		case strings.HasPrefix(minuscula, "user-agent:"):
			agentes = append(agentes, strings.TrimSpace(minuscula[len("user-agent:"):]))
		case strings.HasPrefix(minuscula, "crawl-delay:"):
			if !matchesAgent(agentes, agenteUsuario) {
				continue
			}
			crudo := strings.TrimSpace(minuscula[len("crawl-delay:"):])
			segundos, errorObtenido := strconv.ParseFloat(crudo, 64)
			if errorObtenido != nil || segundos <= 0 {
				continue
			}
			candidato := time.Duration(segundos * float64(time.Second))
			if candidato > retraso {
				retraso = candidato
			}
		}
	}
	return retraso
}

// matchesAgent verifica si una directiva de User-agent aplica al cliente actual.
func matchesAgent(agentes []string, agenteUsuario string) bool {
	usuarioMinuscula := strings.ToLower(agenteUsuario)
	for _, agente := range agentes {
		if agente == "*" || strings.Contains(usuarioMinuscula, agente) || strings.Contains(agente, "tallerintegracion") {
			return true
		}
	}
	return false
}
