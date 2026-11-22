// crawler.go encapsula el acceso HTTP del scraper: verificación de robots.txt,
// regulación de tasa por dominio, reintentos con espera exponencial y detección de bloqueos.
package extraccion

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

// ===== Certificados y Errores =====

// Acuenta no envía el certificado intermedio en el handshake TLS. El
// intermedio público de GlobalSign se agrega como ancla adicional para poder
// verificar la cadena sin desactivar la validación. Vence el 21/11/2028.
//
//go:embed certs/globalsign-rsa-ov-ssl-ca-2018.pem
var certificadoCAAdicional []byte

var (
	// ErrRobotsDisallowed indica que las directivas de robots.txt prohíben acceder a la URL.
	ErrRobotsDisallowed = errors.New("robots.txt no permite la URL")
	// ErrResponseTooLarge indica que el cuerpo HTTP excede el límite configurado en DOWNLOAD_MAXSIZE.
	ErrResponseTooLarge = errors.New("la respuesta supera DOWNLOAD_MAXSIZE")
)

// ===== Tipos de Respuesta y Error HTTP =====

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

// ===== Estructura de Cliente y Opciones =====

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

// Client descarga páginas respetando robots.txt, throttle por dominio,
// tamaño máximo y reintentos con backoff.
type Client struct {
	clienteHTTP      *http.Client
	agenteUsuario    string
	retraso          time.Duration
	tamanoMaximo     int64
	maximoReintentos int
	esperaMinima     time.Duration
	esperaMaxima     time.Duration
	registrador      *slog.Logger
	robots           *cacheRobots
	regulador        *reguladorTasa
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
		clienteHTTP:      nuevoClienteHTTP(opciones.Timeout),
		agenteUsuario:    opciones.UserAgent,
		retraso:          opciones.Delay,
		tamanoMaximo:     opciones.MaxSize,
		maximoReintentos: opciones.MaxRetries,
		esperaMinima:     opciones.BackoffMin,
		esperaMaxima:     opciones.BackoffMax,
		registrador:      opciones.Logger,
		regulador:        nuevoReguladorTasa(),
	}
	cliente.robots = nuevoCacheRobots(
		cliente.clienteHTTP,
		cliente.agenteUsuario,
		opciones.RobotsCacheTTL,
		opciones.RobotsFailureTTL,
		opciones.RobotsMaxBytes,
		cliente.registrador,
	)
	return cliente
}

// ===== Cliente HTTP =====

// nuevoClienteHTTP instancia un cliente HTTP con timeouts estrictos para crawling.
func nuevoClienteHTTP(tiempoEspera time.Duration) *http.Client {
	raices, errorObtenido := x509.SystemCertPool()
	if errorObtenido != nil || raices == nil {
		raices = x509.NewCertPool()
	}
	if len(certificadoCAAdicional) > 0 {
		raices.AppendCertsFromPEM(certificadoCAAdicional)
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

	permitido, retrasoRastreo, errorObtenido := cliente.robots.permitido(contexto, analizada)
	if errorObtenido != nil {
		cliente.registrador.Warn("scraper_robots_unavailable", "url", urlCruda, "error", errorObtenido)
	}
	if !permitido {
		return nil, fmt.Errorf("%w: %s", ErrRobotsDisallowed, urlCruda)
	}
	retraso := cliente.retraso
	if retrasoRastreo > retraso {
		retraso = retrasoRastreo
	}

	respuestasBloqueadas := 0
	for intento := 0; intento <= cliente.maximoReintentos; intento++ {
		if errorObtenido := cliente.regulador.esperar(contexto, analizada.Hostname(), retraso); errorObtenido != nil {
			return nil, errorObtenido
		}
		respuesta, errorObtenido := cliente.ejecutarPeticion(contexto, urlCruda)
		if errorObtenido != nil {
			if contexto.Err() != nil {
				return nil, contexto.Err()
			}
			if intento < cliente.maximoReintentos {
				if errorObtenido := cliente.esperarReintento(contexto, intento, 0); errorObtenido != nil {
					return nil, errorObtenido
				}
				continue
			}
			return nil, errorObtenido
		}

		if motivo := detectarBloqueo(respuesta.Status, respuesta.Body); motivo != "" {
			respuestasBloqueadas++
			cliente.registrador.Warn("scraper_blocked",
				"url", urlCruda,
				"status", respuesta.Status,
				"reason", motivo,
				"attempt", intento+1,
			)
			if intento < cliente.maximoReintentos {
				if errorObtenido := cliente.esperarReintento(contexto, intento, leerRetryAfter(respuesta.Header)); errorObtenido != nil {
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

		if respuesta.Status >= http.StatusInternalServerError {
			if intento < cliente.maximoReintentos {
				if errorObtenido := cliente.esperarReintento(contexto, intento, leerRetryAfter(respuesta.Header)); errorObtenido != nil {
					return nil, errorObtenido
				}
				continue
			}
			return nil, &HTTPStatusError{URL: urlCruda, Status: respuesta.Status}
		}
		if respuesta.Status >= http.StatusBadRequest && respuesta.Status != http.StatusNotFound {
			return nil, &HTTPStatusError{URL: urlCruda, Status: respuesta.Status}
		}
		return respuesta, nil
	}
	return nil, fmt.Errorf("no se pudo obtener %s", urlCruda)
}

// ===== Petición HTTP y Transporte =====

// ejecutarPeticion ejecuta la solicitud HTTP GET incorporando cabeceras de navegación.
func (cliente *Client) ejecutarPeticion(contexto context.Context, urlCruda string) (*Response, error) {
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodGet, urlCruda, nil)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("User-Agent", cliente.agenteUsuario)
	peticion.Header.Set("Accept", "text/html,application/xhtml+xml")

	respuesta, errorObtenido := cliente.clienteHTTP.Do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer respuesta.Body.Close()

	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, cliente.tamanoMaximo+1))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if int64(len(cuerpo)) > cliente.tamanoMaximo {
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

// esperarReintento calcula el tiempo de espera exponencial con variación aleatoria para reintentos.
func (cliente *Client) esperarReintento(contexto context.Context, intento int, esperaServidor time.Duration) error {
	espera := esperaServidor
	if espera <= 0 {
		base := cliente.retraso
		if base < cliente.esperaMinima {
			base = cliente.esperaMinima
		}
		exponencial := base * time.Duration(1<<min(intento, 7))
		// Jitter completo sobre la mitad superior: conserva el crecimiento
		// exponencial y evita que varios workers reintenten a la vez.
		mitad := exponencial / 2
		espera = mitad + time.Duration(rand.Int64N(int64(mitad)+1))
	}
	// Retry-After es una instrucción del servidor: se respeta incluso si es
	// mayor que el techo del backoff propio. El techo sólo aplica al jitter.
	if esperaServidor <= 0 && espera > cliente.esperaMaxima {
		espera = cliente.esperaMaxima
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

// leerRetryAfter interpreta la cabecera Retry-After en segundos o formato HTTP-date.
func leerRetryAfter(cabecera http.Header) time.Duration {
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

// ===== Detección de Bloqueos =====

// No se busca "captcha" como subcadena: las páginas normales de Cugat
// incluyen el CSS y la configuración de reCAPTCHA/WPForms, y se marcaban como
// bloqueadas sin estarlo. Los marcadores deben identificar el challenge, no
// cualquier script de terceros. Por eso "captcha" sólo cuenta si aparece en
// una instrucción de resolución o en un campo exacto del desafío.
var (
	marcadoresBloqueo = []string{
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
	patronCaptchaAislado = regexp.MustCompile(
		`(?i)(solve|complete|enter|verify|type|submit)\s+(the\s+)?captcha` +
			`|captcha\s+(required|verification|challenge|test|code)` +
			`|(id|name)\s*=\s*["']captcha["']`,
	)
)

// detectarBloqueo analiza la respuesta para detectar patrones de bloqueo o desafíos antibot.
func detectarBloqueo(estado int, cuerpo []byte) string {
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
	for _, marcador := range marcadoresBloqueo {
		if strings.Contains(minuscula, marcador) {
			return marcador
		}
	}
	if patronCaptchaAislado.MatchString(minuscula) {
		return "captcha"
	}
	return ""
}

// ===== Regulación de Tasa (Throttle) =====

// reguladorTasa gestiona la cadencia de peticiones por host para evitar saturaciones.
type reguladorTasa struct {
	candado   sync.Mutex
	siguiente map[string]time.Time
}

// nuevoReguladorTasa instancia un regulador de frecuencia de peticiones por intervalo.
func nuevoReguladorTasa() *reguladorTasa {
	return &reguladorTasa{siguiente: make(map[string]time.Time)}
}

// esperar reserva el siguiente turno del dominio. La reserva se hace bajo lock
// para que dos goroutines no calculen el mismo instante.
func (regulador *reguladorTasa) esperar(contexto context.Context, servidor string, retraso time.Duration) error {
	if retraso <= 0 {
		return nil
	}
	regulador.candado.Lock()
	ahora := time.Now()
	instanteSiguiente := regulador.siguiente[servidor]
	if instanteSiguiente.Before(ahora) {
		instanteSiguiente = ahora
	}
	regulador.siguiente[servidor] = instanteSiguiente.Add(retraso)
	regulador.candado.Unlock()

	espera := time.Until(instanteSiguiente)
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

// entradaRobots almacena las reglas y tiempos de crawl-delay para un dominio.
type entradaRobots struct {
	datos          *robotstxt.RobotsData
	retrasoRastreo time.Duration
	descargadoEn   time.Time
}

// cacheRobots mantiene en memoria las directivas de robots.txt por host.
type cacheRobots struct {
	candado       sync.Mutex
	entradas      map[string]*entradaRobots
	fallos        map[string]time.Time
	clienteHTTP   *http.Client
	agenteUsuario string
	vidaCache     time.Duration
	vidaFallo     time.Duration
	maximoBytes   int64
	registrador   *slog.Logger
}

// nuevoCacheRobots inicializa el caché de directivas de robots.txt.
func nuevoCacheRobots(
	cliente *http.Client,
	agenteUsuario string,
	vidaCache time.Duration,
	vidaFallo time.Duration,
	maximoBytes int64,
	registrador *slog.Logger,
) *cacheRobots {
	return &cacheRobots{
		entradas:      make(map[string]*entradaRobots),
		fallos:        make(map[string]time.Time),
		clienteHTTP:   cliente,
		agenteUsuario: agenteUsuario,
		vidaCache:     vidaCache,
		vidaFallo:     vidaFallo,
		maximoBytes:   maximoBytes,
		registrador:   registrador,
	}
}

// permitido determina si una URL cumple las reglas de exclusión de robots.txt.
func (cache *cacheRobots) permitido(contexto context.Context, objetivo *url.URL) (bool, time.Duration, error) {
	base := objetivo.Scheme + "://" + objetivo.Host
	cache.candado.Lock()
	entrada := cache.entradas[base]
	falladoEn, fallido := cache.fallos[base]
	cache.candado.Unlock()

	if entrada == nil || time.Since(entrada.descargadoEn) > cache.vidaCache {
		if fallido && time.Since(falladoEn) < cache.vidaFallo {
			// Fail-open sin repetir el aviso en cada página. Cugat, por
			// ejemplo, redirige robots.txt en bucle; sin esta caché el
			// worker registraría un warning por cada categoría.
			return true, 0, nil
		}
		obtenido, errorObtenido := cache.descargar(contexto, base)
		if errorObtenido != nil {
			if contexto.Err() == nil {
				cache.candado.Lock()
				cache.fallos[base] = time.Now()
				cache.candado.Unlock()
			}
			return true, 0, errorObtenido
		}
		entrada = obtenido
		cache.candado.Lock()
		cache.entradas[base] = entrada
		delete(cache.fallos, base)
		cache.candado.Unlock()
	}

	ruta := objetivo.RequestURI()
	if ruta == "" {
		ruta = "/"
	}
	return entrada.datos.TestAgent(ruta, cache.agenteUsuario), entrada.retrasoRastreo, nil
}

// descargar descarga y procesa el archivo robots.txt correspondiente a un host.
func (cache *cacheRobots) descargar(contexto context.Context, base string) (*entradaRobots, error) {
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodGet, base+"/robots.txt", nil)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("User-Agent", cache.agenteUsuario)
	respuesta, errorObtenido := cache.clienteHTTP.Do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer respuesta.Body.Close()

	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, cache.maximoBytes))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if respuesta.StatusCode >= http.StatusBadRequest {
		cuerpo = nil
	}
	datos, errorParseo := robotstxt.FromBytes(cuerpo)
	if errorParseo != nil {
		// Un robots.txt inválido no debe bloquear el crawl; se comporta como
		// ausente, igual que Scrapy ante un error de parseo.
		datos, _ = robotstxt.FromBytes(nil)
	}
	return &entradaRobots{
		datos:          datos,
		retrasoRastreo: analizarCrawlDelay(cuerpo, cache.agenteUsuario),
		descargadoEn:   time.Now(),
	}, nil
}

// analizarCrawlDelay busca la directiva Crawl-delay aplicable en robots.txt.
func analizarCrawlDelay(cuerpo []byte, agenteUsuario string) time.Duration {
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
			if !coincideAgente(agentes, agenteUsuario) {
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

// coincideAgente verifica si una directiva de User-agent aplica al cliente actual.
func coincideAgente(agentes []string, agenteUsuario string) bool {
	usuarioMinuscula := strings.ToLower(agenteUsuario)
	for _, agente := range agentes {
		if agente == "*" || strings.Contains(usuarioMinuscula, agente) || strings.Contains(agente, "tallerintegracion") {
			return true
		}
	}
	return false
}
