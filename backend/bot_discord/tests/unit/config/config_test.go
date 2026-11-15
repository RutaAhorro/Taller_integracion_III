// Package config_test verifica la carga y validación de configuración del bot.
package config_test

import (
	"strings"
	"testing"

	"github.com/equipo/bot_discord/internal/config"
)

// prepararEntornoConfig establece las variables obligatorias para cada caso.
func prepararEntornoConfig(t *testing.T) {
	t.Helper()
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("DISCORD_CHANNEL_ID", "canal-principal")
	t.Setenv("DISCORD_PUSH_CHANNEL_ID", "")
	t.Setenv("SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 32))
	t.Setenv("SCRAPER_API_BASE_URL", "http://api:8080/api/v1/scraper/trabajos")
	t.Setenv("PORT", "")
	t.Setenv("SCRAPER_HTTP_TIMEOUT", "")
	t.Setenv("BOT_HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("SCRAPER_POLL_INTERVAL", "")
	t.Setenv("SCRAPER_MONITOR_MAX_FAILURES", "")
	t.Setenv("DISCORD_REVIEW_DEBOUNCE", "")
}

// TestCargarValoresPredeterminados verifica los valores documentados sin entorno opcional.
func TestCargarValoresPredeterminados(t *testing.T) {
	prepararEntornoConfig(t)
	configuracion, err := config.Cargar()
	if err != nil {
		t.Fatal(err)
	}
	if configuracion.CanalPushDiscordID != configuracion.CanalDiscordID ||
		configuracion.PuertoWebhook != "8080" ||
		configuracion.TimeoutHTTP.String() != "15s" ||
		configuracion.TimeoutCabeceraHTTP.String() != "5s" ||
		configuracion.IntervaloConsulta.String() != "5s" ||
		configuracion.MaximoFallosMonitor != 12 ||
		configuracion.RetrasoDebounceReviews.String() != "2.5s" {
		t.Fatalf("valores predeterminados inesperados: %+v", configuracion)
	}
}

// TestCargarNormalizaURLScraper evita barras dobles al construir rutas de trabajos.
func TestCargarNormalizaURLScraper(t *testing.T) {
	prepararEntornoConfig(t)
	t.Setenv("SCRAPER_API_BASE_URL", "http://api:8080/api/v1/scraper/trabajos/")
	configuracion, err := config.Cargar()
	if err != nil {
		t.Fatal(err)
	}
	if configuracion.URLBaseScraper != "http://api:8080/api/v1/scraper/trabajos" {
		t.Fatalf("URL base inesperada: %q", configuracion.URLBaseScraper)
	}
}

// TestCargarRechazaConfiguracionesInvalidas verifica fallos explícitos al cargar.
func TestCargarRechazaConfiguracionesInvalidas(t *testing.T) {
	casos := []struct {
		nombre   string
		variable string
		valor    string
	}{
		{"token de servicio corto", "SCRAPING_SERVICE_TOKEN", "corto"},
		{"token de servicio con espacio", "SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 15) + " " + strings.Repeat("s", 17)},
		{"URL sin ruta contractual", "SCRAPER_API_BASE_URL", "http://api:8080"},
		{"URL con query", "SCRAPER_API_BASE_URL", "http://api:8080/api/v1/scraper/trabajos?token=secreto"},
		{"URL con credenciales", "SCRAPER_API_BASE_URL", "http://usuario@api:8080/api/v1/scraper/trabajos"},
		{"puerto fuera de rango", "PORT", "65536"},
		{"timeout inválido", "SCRAPER_HTTP_TIMEOUT", "-1s"},
		{"timeout de cabecera inválido", "BOT_HTTP_READ_HEADER_TIMEOUT", "31s"},
		{"sondeo inválido", "SCRAPER_POLL_INTERVAL", "no-es-duracion"},
		{"fallos fuera de rango", "SCRAPER_MONITOR_MAX_FAILURES", "0"},
		{"debounce fuera de rango", "DISCORD_REVIEW_DEBOUNCE", "31s"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			prepararEntornoConfig(t)
			t.Setenv(caso.variable, caso.valor)
			if _, err := config.Cargar(); err == nil {
				t.Fatal("se aceptó una configuración inválida")
			}
		})
	}
}

// TestCargarRequiereCredencialesDiscord comprueba que las credenciales obligatorias fallen rápido.
func TestCargarRequiereCredencialesDiscord(t *testing.T) {
	for _, variable := range []string{"DISCORD_BOT_TOKEN", "DISCORD_CHANNEL_ID"} {
		t.Run(variable, func(t *testing.T) {
			prepararEntornoConfig(t)
			t.Setenv(variable, "")
			if _, err := config.Cargar(); err == nil {
				t.Fatal("se aceptó una credencial Discord vacía")
			}
		})
	}
}
