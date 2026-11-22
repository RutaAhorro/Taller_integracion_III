// Package discord_test verifica los comandos, interacciones y notificaciones del
// bot sin conectarse a Discord ni a Control reales: la sesión usa un transporte
// HTTP falso y Control se simula con httptest.
package discord_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/config"
	"github.com/equipo/bot_discord/internal/discord"
	"github.com/equipo/bot_discord/internal/scraping"
)

const (
	tokenServicioPrueba = "token-de-servicio-de-prueba-0123456789"
	canalPrincipalID    = "canal-principal"
	canalPushID         = "canal-push"
	rutaControlPrueba   = "/api/v1/scraper/trabajos"
	// intervaloLargo evita que los seguimientos en segundo plano consulten Control durante una prueba.
	intervaloLargo = time.Hour
)

// llamadaDiscord es una petición HTTP que el bot hizo hacia la API de Discord.
type llamadaDiscord struct {
	ruta   string
	cuerpo string
}

// registroDiscord acumula, de forma segura entre goroutines, las llamadas a Discord.
type registroDiscord struct {
	mutex    sync.Mutex
	llamadas []llamadaDiscord
}

// copiar devuelve una copia de las llamadas registradas hasta ahora.
func (registro *registroDiscord) copiar() []llamadaDiscord {
	registro.mutex.Lock()
	defer registro.mutex.Unlock()
	return append([]llamadaDiscord(nil), registro.llamadas...)
}

// cuerpos devuelve los cuerpos enviados a rutas que contienen el fragmento dado.
func (registro *registroDiscord) cuerpos(fragmentoRuta string) []string {
	var cuerpos []string
	for _, llamada := range registro.copiar() {
		if strings.Contains(llamada.ruta, fragmentoRuta) {
			cuerpos = append(cuerpos, llamada.cuerpo)
		}
	}
	return cuerpos
}

// transportadorFalso adapta una función a http.RoundTripper.
type transportadorFalso func(*http.Request) (*http.Response, error)

// RoundTrip satisface http.RoundTripper.
func (transportador transportadorFalso) RoundTrip(peticion *http.Request) (*http.Response, error) {
	return transportador(peticion)
}

// nuevaSesionFalsa crea una sesión de Discord cuyas peticiones se registran y
// se responden localmente.
func nuevaSesionFalsa(prueba *testing.T) (*discordgo.Session, *registroDiscord) {
	prueba.Helper()
	sesion, errorObtenido := discordgo.New("Bot token-de-prueba")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	sesion.State.User = &discordgo.User{ID: "bot-id"}
	registro := &registroDiscord{}
	sesion.Client = &http.Client{Transport: transportadorFalso(func(peticion *http.Request) (*http.Response, error) {
		cuerpoPeticion, errorLectura := io.ReadAll(peticion.Body)
		if errorLectura != nil {
			return nil, errorLectura
		}
		registro.mutex.Lock()
		registro.llamadas = append(registro.llamadas, llamadaDiscord{ruta: peticion.URL.Path, cuerpo: string(cuerpoPeticion)})
		registro.mutex.Unlock()
		estado, cuerpo := http.StatusOK, `{}`
		if strings.HasSuffix(peticion.URL.Path, "/callback") {
			estado, cuerpo = http.StatusNoContent, ``
		}
		return &http.Response{StatusCode: estado, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(cuerpo)), Request: peticion}, nil
	})}
	return sesion, registro
}

// configuracionPrueba devuelve una configuración válida con tiempos aptos para pruebas.
func configuracionPrueba() config.Configuracion {
	return config.Configuracion{
		CanalDiscordID:         canalPrincipalID,
		CanalPushDiscordID:     canalPushID,
		TokenServicioScraping:  tokenServicioPrueba,
		TimeoutHTTP:            5 * time.Second,
		TimeoutCabeceraHTTP:    time.Second,
		IntervaloConsulta:      intervaloLargo,
		MaximoFallosMonitor:    1,
		RetrasoDebounceReviews: 30 * time.Millisecond,
	}
}

// nuevoManejadorPrueba arma un Manejador con sesión falsa y el Control indicado.
func nuevoManejadorPrueba(prueba *testing.T, controlHTTP http.Handler) (*discord.Manejador, *registroDiscord) {
	prueba.Helper()
	servidor := httptest.NewServer(controlHTTP)
	prueba.Cleanup(servidor.Close)
	configuracion := configuracionPrueba()
	control, errorObtenido := scraping.NuevoCliente(servidor.URL+rutaControlPrueba, tokenServicioPrueba, configuracion.TimeoutHTTP)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	contexto, cancelar := context.WithCancel(context.Background())
	prueba.Cleanup(cancelar)
	sesion, registro := nuevaSesionFalsa(prueba)
	manejador, errorObtenido := discord.NuevoManejador(contexto, configuracion, sesion, control)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return manejador, registro
}

// controlSiempre devuelve un Control simulado que responde igual a todo.
func controlSiempre(estadoHTTP int, cuerpo string) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		respuesta.WriteHeader(estadoHTTP)
		_, _ = io.WriteString(respuesta, cuerpo)
	})
}
