package bot

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCrearRespuestaScrapingMasivoSinTrabajosOmiteComponentes(prueba *testing.T) {
	respuesta := crearRespuestaScrapingMasivo(nil)
	if len(respuesta.Components) != 0 {
		prueba.Fatalf("la respuesta sin trabajos no debe incluir componentes: %+v", respuesta.Components)
	}
	if !strings.Contains(respuesta.Content, "No se pudo iniciar ningún trabajo") {
		prueba.Fatalf("la respuesta debe explicar que no se iniciaron trabajos: %q", respuesta.Content)
	}
}

func TestCrearRespuestaScrapingMasivoConTrabajosIncluyeBotones(prueba *testing.T) {
	respuesta := crearRespuestaScrapingMasivo([]discordgo.Button{{Label: "Actualizar Jumbo", CustomID: "refresh-id"}})
	if len(respuesta.Components) != 1 {
		prueba.Fatalf("la respuesta debe incluir una fila de componentes: %+v", respuesta.Components)
	}
	if !strings.Contains(respuesta.Content, "Se inició `1` trabajo") {
		prueba.Fatalf("la respuesta debe indicar la cantidad de trabajos: %q", respuesta.Content)
	}
}

// roundTripperFunc adapta una función para simular las respuestas HTTP de Discord.
type adaptadorRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip satisface http.RoundTripper para las pruebas de interacción.
func (transportador adaptadorRoundTripper) RoundTrip(peticion *http.Request) (*http.Response, error) {
	return transportador(peticion)
}

func TestIniciarScrapingEnDMRespondeDespuesDeDiferir(prueba *testing.T) {
	sesionDiscord, errorObtenido := discordgo.New("Bot token-de-prueba")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	var rutasSolicitadas []string
	var cuerposDiscord []string
	sesionDiscord.Client = &http.Client{Transport: adaptadorRoundTripper(func(peticion *http.Request) (*http.Response, error) {
		rutasSolicitadas = append(rutasSolicitadas, peticion.URL.Path)
		codigoEstado := http.StatusNoContent
		cuerpoRespuesta := ""
		if strings.Contains(peticion.URL.Path, "/webhooks/") {
			cuerpoPeticion, errorLectura := io.ReadAll(peticion.Body)
			if errorLectura != nil {
				return nil, errorLectura
			}
			cuerposDiscord = append(cuerposDiscord, string(cuerpoPeticion))
			codigoEstado = http.StatusOK
			cuerpoRespuesta = `{}`
		}
		return &http.Response{
			StatusCode: codigoEstado,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(cuerpoRespuesta)),
			Request:    peticion,
		}, nil
	})}
	interaccion := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID: "interaccion-prueba", AppID: "aplicacion-prueba", Token: "token-prueba",
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: "scrape_1_jumbo_rsc"},
	}}

	iniciarScraping(sesionDiscord, interaccion)

	if len(rutasSolicitadas) != 2 || !strings.HasSuffix(rutasSolicitadas[0], "/callback") || !strings.Contains(rutasSolicitadas[1], "/webhooks/") {
		prueba.Fatalf("se esperaba diferir y responder la interacción: %v", rutasSolicitadas)
	}
	if len(cuerposDiscord) != 1 || !strings.Contains(cuerposDiscord[0], "No se pudo identificar tu usuario de Discord") {
		prueba.Fatalf("la interacción diferida debe recibir un mensaje de error: %v", cuerposDiscord)
	}
}
