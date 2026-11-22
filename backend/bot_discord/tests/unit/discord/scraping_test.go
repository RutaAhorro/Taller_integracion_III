package discord_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// nuevaInteraccionBoton simula la pulsación de un botón; con usuario vacío la
// interacción llega sin miembro, como ocurre en un mensaje directo.
func nuevaInteraccionBoton(identificador, usuarioID string) *discordgo.InteractionCreate {
	interaccion := &discordgo.Interaction{
		ID: "interaccion-prueba", AppID: "aplicacion-prueba", Token: "token-prueba", ChannelID: canalPrincipalID,
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: identificador},
	}
	if usuarioID != "" {
		interaccion.Member = &discordgo.Member{User: &discordgo.User{ID: usuarioID}}
	}
	return &discordgo.InteractionCreate{Interaction: interaccion}
}

// TestIniciarScrapingEnDMRespondeDespuesDeDiferir: sin miembro de Discord se difiere y luego se responde el error.
func TestIniciarScrapingEnDMRespondeDespuesDeDiferir(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusInternalServerError, ``))
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("scrape_1_jumbo_rsc", ""))

	llamadas := registro.copiar()
	if len(llamadas) != 2 || !strings.HasSuffix(llamadas[0].ruta, "/callback") || !strings.Contains(llamadas[1].ruta, "/webhooks/") {
		prueba.Fatalf("se esperaba diferir y responder la interacción: %v", llamadas)
	}
	if !strings.Contains(llamadas[1].cuerpo, "No se pudo identificar tu usuario de Discord") {
		prueba.Fatalf("la interacción diferida debe recibir un mensaje de error: %v", llamadas[1].cuerpo)
	}
}

// TestScrapingMasivoSinTrabajosOmiteComponentes: si Control rechaza todo, el mensaje lo explica y no hay botones.
func TestScrapingMasivoSinTrabajosOmiteComponentes(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusInternalServerError, ``))
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("scrape_0_todos", "usuario-1"))

	cuerpos := registro.cuerpos("/webhooks/")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "No se pudo iniciar ningún trabajo") || !strings.Contains(cuerpos[0], `"components":null`) {
		prueba.Fatalf("respuesta masiva sin trabajos inesperada: %v", cuerpos)
	}
}

// controlAceptaSoloElPrimero crea y ejecuta el primer trabajo y rechaza los siguientes.
func controlAceptaSoloElPrimero() http.Handler {
	var creaciones atomic.Int32
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		switch {
		case peticion.Method == http.MethodPost && strings.HasSuffix(peticion.URL.Path, "/ejecutar"):
			respuesta.WriteHeader(http.StatusAccepted)
		case peticion.Method == http.MethodPost && creaciones.Add(1) == 1:
			respuesta.WriteHeader(http.StatusCreated)
			_, _ = respuesta.Write([]byte(`{"id":"trabajo-1"}`))
		default:
			respuesta.WriteHeader(http.StatusInternalServerError)
		}
	})
}

// TestScrapingMasivoConUnTrabajoIncluyeBoton verifica el texto singular y el botón de actualización.
func TestScrapingMasivoConUnTrabajoIncluyeBoton(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlAceptaSoloElPrimero())
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("scrape_0_todos", "usuario-1"))

	cuerpos := registro.cuerpos("/webhooks/")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "Se inició `1` trabajo") ||
		!strings.Contains(cuerpos[0], "refresh_trabajo-1") || !strings.Contains(cuerpos[0], "Actualizar Jumbo") {
		prueba.Fatalf("respuesta masiva con un trabajo inesperada: %v", cuerpos)
	}
}

// TestScrapingDeUnaCadenaConfirmaConIDDeTrabajo verifica el flujo de una sola cadena.
func TestScrapingDeUnaCadenaConfirmaConIDDeTrabajo(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlAceptaSoloElPrimero())
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("scrape_2_lider_rsc", "usuario-1"))

	cuerpos := registro.cuerpos("/webhooks/")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "Scraping de Lider iniciado") || !strings.Contains(cuerpos[0], "trabajo-1") {
		prueba.Fatalf("confirmación inesperada: %v", cuerpos)
	}
}

// TestActualizarEstadoInformaErroresSinPanic: ante un 401 de Control se responde un mensaje efímero.
func TestActualizarEstadoInformaErroresSinPanic(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusUnauthorized, `{}`))
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("refresh_trabajo-1", "usuario-1"))

	cuerpos := registro.cuerpos("/callback")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "No se pudo consultar el estado") || !strings.Contains(cuerpos[0], "HTTP 401") {
		prueba.Fatalf("mensaje de error inesperado: %v", cuerpos)
	}
}

// TestActualizarEstadoMantieneBotonEnProgreso verifica el contenido y el botón de un trabajo en curso.
func TestActualizarEstadoMantieneBotonEnProgreso(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{"estado":"en_progreso","cadena_nombre":"Jumbo","elementos_extraidos":10}`))
	manejador.ManejarInteraccion(nil, nuevaInteraccionBoton("refresh_trabajo-1", "usuario-1"))

	cuerpos := registro.cuerpos("/callback")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "10 productos extraídos") || !strings.Contains(cuerpos[0], "refresh_trabajo-1") {
		prueba.Fatalf("estado en progreso inesperado: %v", cuerpos)
	}
}

// TestComandosDeTexto verifica !ping, !help, !scrapear (menú con los botones del contrato) y que se ignore al propio bot.
func TestComandosDeTexto(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	enviar := func(autorID, contenido string) {
		manejador.ManejarMensaje(nil, &discordgo.MessageCreate{Message: &discordgo.Message{
			Author: &discordgo.User{ID: autorID}, Content: contenido, ChannelID: canalPrincipalID,
		}})
	}
	enviar("bot-id", "!ping")
	enviar("persona", "hola")
	if len(registro.copiar()) != 0 {
		prueba.Fatal("el bot no debe responder a sí mismo ni a mensajes sin comando")
	}

	enviar("persona", "!ping")
	enviar("persona", "!help")
	enviar("persona", "!scrapear")
	cuerpos := registro.cuerpos("/channels/" + canalPrincipalID + "/messages")
	if len(cuerpos) != 3 {
		prueba.Fatalf("se esperaban tres mensajes: %v", cuerpos)
	}
	if !strings.Contains(cuerpos[0], "Pong!") || !strings.Contains(cuerpos[1], "Lista de Comandos del Bot") {
		prueba.Fatalf("respuestas de !ping o !help inesperadas: %v", cuerpos[:2])
	}
	for _, identificador := range []string{
		"scrape_1_jumbo_rsc", "scrape_2_lider_rsc", "scrape_4_santa_isabel_rsc",
		"scrape_5_acuenta_rsc", "scrape_6_cugat_rsc", "scrape_0_todos",
	} {
		if !strings.Contains(cuerpos[2], identificador) {
			prueba.Fatalf("el menú no contiene el botón %s: %s", identificador, cuerpos[2])
		}
	}
}
