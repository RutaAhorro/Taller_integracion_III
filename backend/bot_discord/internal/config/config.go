// Package config carga y valida, una sola vez al arrancar, la configuración del
// Bot de Discord. Falla con un mensaje claro cuando falta una variable
// obligatoria para que un despliegue mal configurado no llegue a ejecutarse.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ===== Nombres de variables de entorno =====

// Los nombres compartidos con otros servicios (SCRAPING_SERVICE_TOKEN y
// SCRAPER_API_BASE_URL) se fijan en docs/Guias/GUIA_CONFIGURACION.md y no se
// renombran desde este servicio.
const (
	variableTokenDiscord        = "DISCORD_BOT_TOKEN"
	variableCanalDiscord        = "DISCORD_CHANNEL_ID"
	variableCanalPush           = "DISCORD_PUSH_CHANNEL_ID"
	variableTokenServicio       = "SCRAPING_SERVICE_TOKEN"
	variableURLScraper          = "SCRAPER_API_BASE_URL"
	variablePuerto              = "PORT"
	variableTimeoutHTTP         = "SCRAPER_HTTP_TIMEOUT"
	variableTimeoutCabecera     = "BOT_HTTP_READ_HEADER_TIMEOUT"
	variableIntervaloConsulta   = "SCRAPER_POLL_INTERVAL"
	variableMaximoFallosMonitor = "SCRAPER_MONITOR_MAX_FAILURES"
	variableRetrasoDebounce     = "DISCORD_REVIEW_DEBOUNCE"
)

// ===== Límites y valores predeterminados =====

// Los valores por defecto y los máximos acotan lo configurable para que un
// error tipográfico (por ejemplo "15m" en lugar de "15s") falle al arrancar.
const (
	timeoutHTTPDefecto       = 15 * time.Second
	timeoutCabeceraDefecto   = 5 * time.Second
	intervaloConsultaDefecto = 5 * time.Second
	fallosMonitorDefecto     = 12
	retrasoDebounceDefecto   = 2500 * time.Millisecond
	puertoWebhookDefecto     = "8080"
	longitudMinimaToken      = 32
	timeoutMaximoScraper     = 2 * time.Minute
	timeoutCabeceraMaximo    = 30 * time.Second
	intervaloMaximoScraper   = time.Minute
	fallosMonitorMaximos     = 100
	retrasoDebounceMaximo    = 30 * time.Second
	fallosMonitorMinimos     = 1
	puertoMinimo             = 1
	puertoMaximo             = 65535
	caracteresEspacio        = " \t\r\n"
	rutaTrabajosScraper      = "/api/v1/scraper/trabajos"
	esquemaHTTP              = "http"
	esquemaHTTPS             = "https"
)

// Configuracion contiene los valores validados que necesita el proceso del bot.
type Configuracion struct {
	TokenDiscord           string
	CanalDiscordID         string
	CanalPushDiscordID     string
	TokenServicioScraping  string
	URLBaseScraper         string
	PuertoWebhook          string
	TimeoutHTTP            time.Duration
	TimeoutCabeceraHTTP    time.Duration
	IntervaloConsulta      time.Duration
	MaximoFallosMonitor    int
	RetrasoDebounceReviews time.Duration
}

// ===== Carga y validación de configuración =====

// Cargar lee y valida una sola vez las variables del proceso; el resto del bot
// recibe la estructura resultante en lugar de consultar el entorno.
func Cargar() (Configuracion, error) {
	configuracion := Configuracion{
		TokenDiscord:          strings.TrimSpace(os.Getenv(variableTokenDiscord)),
		CanalDiscordID:        strings.TrimSpace(os.Getenv(variableCanalDiscord)),
		CanalPushDiscordID:    strings.TrimSpace(os.Getenv(variableCanalPush)),
		TokenServicioScraping: os.Getenv(variableTokenServicio),
		URLBaseScraper:        strings.TrimSpace(os.Getenv(variableURLScraper)),
		PuertoWebhook:         valorODefecto(os.Getenv(variablePuerto), puertoWebhookDefecto),
	}
	if configuracion.TokenDiscord == "" {
		return Configuracion{}, fmt.Errorf("%s es obligatorio", variableTokenDiscord)
	}
	if configuracion.CanalDiscordID == "" {
		return Configuracion{}, fmt.Errorf("%s es obligatorio", variableCanalDiscord)
	}
	if configuracion.CanalPushDiscordID == "" {
		configuracion.CanalPushDiscordID = configuracion.CanalDiscordID
	}
	if len(configuracion.TokenServicioScraping) < longitudMinimaToken || strings.ContainsAny(configuracion.TokenServicioScraping, caracteresEspacio) {
		return Configuracion{}, fmt.Errorf("%s debe tener al menos %d caracteres y no contener espacios", variableTokenServicio, longitudMinimaToken)
	}
	if errorObtenido := validarURLScraper(configuracion.URLBaseScraper); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	configuracion.URLBaseScraper = strings.TrimSuffix(configuracion.URLBaseScraper, "/")
	if errorObtenido := validarPuerto(configuracion.PuertoWebhook); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	var errorObtenido error
	if configuracion.TimeoutHTTP, errorObtenido = cargarDuracionPositiva(variableTimeoutHTTP, timeoutHTTPDefecto, timeoutMaximoScraper); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if configuracion.TimeoutCabeceraHTTP, errorObtenido = cargarDuracionPositiva(variableTimeoutCabecera, timeoutCabeceraDefecto, timeoutCabeceraMaximo); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if configuracion.IntervaloConsulta, errorObtenido = cargarDuracionPositiva(variableIntervaloConsulta, intervaloConsultaDefecto, intervaloMaximoScraper); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if configuracion.MaximoFallosMonitor, errorObtenido = cargarEnteroAcotado(variableMaximoFallosMonitor, fallosMonitorDefecto, fallosMonitorMinimos, fallosMonitorMaximos); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if configuracion.RetrasoDebounceReviews, errorObtenido = cargarDuracionPositiva(variableRetrasoDebounce, retrasoDebounceDefecto, retrasoDebounceMaximo); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	return configuracion, nil
}

// ===== Validadores y valores auxiliares =====

// validarURLScraper exige que la URL apunte a la ruta de trabajos de Control y
// no incluya credenciales, query ni fragmento, para que el token viaje solo en
// la cabecera Authorization y nunca en la URL.
func validarURLScraper(valor string) error {
	direccion, errorObtenido := url.Parse(valor)
	if errorObtenido != nil || direccion == nil || (direccion.Scheme != esquemaHTTP && direccion.Scheme != esquemaHTTPS) ||
		direccion.Hostname() == "" || direccion.User != nil || direccion.RawQuery != "" ||
		direccion.ForceQuery || direccion.Fragment != "" || strings.TrimSuffix(direccion.Path, "/") != rutaTrabajosScraper {
		return fmt.Errorf("%s debe ser una URL HTTP(S) válida terminada en %s", variableURLScraper, rutaTrabajosScraper)
	}
	return nil
}

// validarPuerto evita abrir el servidor de webhooks en un puerto inválido.
func validarPuerto(valor string) error {
	puerto, errorObtenido := strconv.Atoi(valor)
	if errorObtenido != nil || puerto < puertoMinimo || puerto > puertoMaximo {
		return fmt.Errorf("%s debe estar entre %d y %d", variablePuerto, puertoMinimo, puertoMaximo)
	}
	return nil
}

// cargarDuracionPositiva lee una duración acotada o devuelve su valor documentado.
func cargarDuracionPositiva(nombre string, defecto, maximo time.Duration) (time.Duration, error) {
	valor := strings.TrimSpace(os.Getenv(nombre))
	if valor == "" {
		return defecto, nil
	}
	duracion, errorObtenido := time.ParseDuration(valor)
	if errorObtenido != nil || duracion <= 0 || duracion > maximo {
		return 0, fmt.Errorf("%s debe ser una duración positiva no superior a %s", nombre, maximo)
	}
	return duracion, nil
}

// cargarEnteroAcotado valida un entero configurable con límites explícitos.
func cargarEnteroAcotado(nombre string, defecto, minimo, maximo int) (int, error) {
	valor := strings.TrimSpace(os.Getenv(nombre))
	if valor == "" {
		return defecto, nil
	}
	numero, errorObtenido := strconv.Atoi(valor)
	if errorObtenido != nil || numero < minimo || numero > maximo {
		return 0, fmt.Errorf("%s debe ser un entero entre %d y %d", nombre, minimo, maximo)
	}
	return numero, nil
}

// valorODefecto devuelve el valor no vacío o el predeterminado documentado.
func valorODefecto(valor, defecto string) string {
	if valor == "" {
		return defecto
	}
	return valor
}
