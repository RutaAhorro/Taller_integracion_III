// Package config carga y valida la configuración de ejecución del Bot de Discord.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

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
	rutaTrabajosScraper      = "/api/v1/scraper/trabajos"
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

// Cargar lee y valida una sola vez las variables del proceso.
func Cargar() (Configuracion, error) {
	configuracion := Configuracion{
		TokenDiscord:          strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN")),
		CanalDiscordID:        strings.TrimSpace(os.Getenv("DISCORD_CHANNEL_ID")),
		CanalPushDiscordID:    strings.TrimSpace(os.Getenv("DISCORD_PUSH_CHANNEL_ID")),
		TokenServicioScraping: os.Getenv("SCRAPING_SERVICE_TOKEN"),
		URLBaseScraper:        strings.TrimSpace(os.Getenv("SCRAPER_API_BASE_URL")),
		PuertoWebhook:         valorODefecto(os.Getenv("PORT"), puertoWebhookDefecto),
	}
	if configuracion.TokenDiscord == "" {
		return Configuracion{}, errors.New("DISCORD_BOT_TOKEN es obligatorio")
	}
	if configuracion.CanalDiscordID == "" {
		return Configuracion{}, errors.New("DISCORD_CHANNEL_ID es obligatorio")
	}
	if configuracion.CanalPushDiscordID == "" {
		configuracion.CanalPushDiscordID = configuracion.CanalDiscordID
	}
	if len(configuracion.TokenServicioScraping) < longitudMinimaToken || strings.ContainsAny(configuracion.TokenServicioScraping, " \t\r\n") {
		return Configuracion{}, errors.New("SCRAPING_SERVICE_TOKEN debe tener al menos 32 caracteres y no contener espacios")
	}
	if err := validarURLScraper(configuracion.URLBaseScraper); err != nil {
		return Configuracion{}, err
	}
	configuracion.URLBaseScraper = strings.TrimSuffix(configuracion.URLBaseScraper, "/")
	if err := validarPuerto(configuracion.PuertoWebhook); err != nil {
		return Configuracion{}, err
	}
	var err error
	if configuracion.TimeoutHTTP, err = cargarDuracionPositiva("SCRAPER_HTTP_TIMEOUT", timeoutHTTPDefecto, timeoutMaximoScraper); err != nil {
		return Configuracion{}, err
	}
	if configuracion.TimeoutCabeceraHTTP, err = cargarDuracionPositiva("BOT_HTTP_READ_HEADER_TIMEOUT", timeoutCabeceraDefecto, timeoutCabeceraMaximo); err != nil {
		return Configuracion{}, err
	}
	if configuracion.IntervaloConsulta, err = cargarDuracionPositiva("SCRAPER_POLL_INTERVAL", intervaloConsultaDefecto, intervaloMaximoScraper); err != nil {
		return Configuracion{}, err
	}
	if configuracion.MaximoFallosMonitor, err = cargarEnteroAcotado("SCRAPER_MONITOR_MAX_FAILURES", fallosMonitorDefecto, 1, fallosMonitorMaximos); err != nil {
		return Configuracion{}, err
	}
	if configuracion.RetrasoDebounceReviews, err = cargarDuracionPositiva("DISCORD_REVIEW_DEBOUNCE", retrasoDebounceDefecto, retrasoDebounceMaximo); err != nil {
		return Configuracion{}, err
	}
	return configuracion, nil
}

// validarURLScraper exige una URL pública del contrato HTTP configurado.
func validarURLScraper(valor string) error {
	direccion, err := url.Parse(valor)
	if err != nil || direccion == nil || (direccion.Scheme != "http" && direccion.Scheme != "https") ||
		direccion.Hostname() == "" || direccion.User != nil || direccion.RawQuery != "" ||
		direccion.ForceQuery || direccion.Fragment != "" || strings.TrimSuffix(direccion.Path, "/") != rutaTrabajosScraper {
		return errors.New("SCRAPER_API_BASE_URL debe ser una URL HTTP(S) válida terminada en /api/v1/scraper/trabajos")
	}
	return nil
}

// validarPuerto evita abrir el servidor de webhooks en un puerto inválido.
func validarPuerto(valor string) error {
	puerto, err := strconv.Atoi(valor)
	if err != nil || puerto < 1 || puerto > 65535 {
		return errors.New("PORT debe estar entre 1 y 65535")
	}
	return nil
}

// cargarDuracionPositiva lee una duración acotada o devuelve su valor documentado.
func cargarDuracionPositiva(nombre string, defecto, maximo time.Duration) (time.Duration, error) {
	valor := strings.TrimSpace(os.Getenv(nombre))
	if valor == "" {
		return defecto, nil
	}
	duracion, err := time.ParseDuration(valor)
	if err != nil || duracion <= 0 || duracion > maximo {
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
	numero, err := strconv.Atoi(valor)
	if err != nil || numero < minimo || numero > maximo {
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
