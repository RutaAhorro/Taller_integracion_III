package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/scraping"
)

// ===== Comandos de texto =====

const (
	comandoPing     = "!ping"
	comandoScrapear = "!scrapear"
	comandoAyuda    = "!help"

	mensajePong  = "Pong! El bot está vivo y respirando."
	mensajeAyuda = "📚 **Lista de Comandos del Bot**\n\n" +
		"**Scraping:**\n" +
		"`!scrapear` - Muestra el menú interactivo para seleccionar y ejecutar una araña de scraping (Jumbo, Lider, Santa Isabel).\n\n" +
		"**GitHub (Automático):**\n" +
		"No necesitas comandos para GitHub, el bot avisa automáticamente cuando hay:\n" +
		"- 🚀 Nuevos Pushes (con resumen de commits)\n" +
		"- 🛠️ Creación y Merge de Pull Requests\n" +
		"- 👀 Asignación de Reviewers y Etiquetas en PRs\n" +
		"- ✅ Aprobación o rechazo de PRs\n\n" +
		"**Otros:**\n" +
		"`!ping` - Para comprobar que sigo con vida."

	mensajeMenuScraping = "🤖 **Menú de Scraping**\nSelecciona el supermercado que deseas scrapear:"
)

// ManejarMensaje responde a los comandos de texto "!ping", "!scrapear" y
// "!help", ignorando los mensajes del propio bot para no responderse a sí mismo.
func (manejador *Manejador) ManejarMensaje(_ *discordgo.Session, mensaje *discordgo.MessageCreate) {
	if mensaje.Author.ID == manejador.sesion.State.User.ID {
		return
	}
	switch mensaje.Content {
	case comandoPing:
		manejador.enviarMensaje(mensaje.ChannelID, mensajePong)
	case comandoScrapear:
		manejador.enviarMenuScraping(mensaje.ChannelID)
	case comandoAyuda:
		manejador.enviarMensaje(mensaje.ChannelID, mensajeAyuda)
	}
}

// ===== Menú de scraping =====

// aspectoBoton es la apariencia de un botón del menú.
type aspectoBoton struct {
	estilo discordgo.ButtonStyle
	emoji  string
}

// aspectosCadena asigna un aspecto a cada cadena por su identificador. El menú
// recorre el catálogo de scraping, de modo que los botones y las llamadas a
// Control no puedan desalinearse; una cadena sin aspecto usa el estilo
// secundario y no lleva emoji.
var aspectosCadena = map[int]aspectoBoton{
	1: {discordgo.SuccessButton, "🐘"},
	2: {discordgo.PrimaryButton, "🛒"},
	4: {discordgo.DangerButton, "🛍️"},
	5: {discordgo.SuccessButton, "📉"},
	6: {discordgo.SecondaryButton, "🏪"},
}

// botonesPrimeraFila es el reparto del menú: tres cadenas en la primera fila y
// el resto, junto con "Todos a la vez", en la segunda.
const botonesPrimeraFila = 3

// enviarMenuScraping publica el menú interactivo con un botón por cadena y uno
// para lanzarlas todas.
func (manejador *Manejador) enviarMenuScraping(canalID string) {
	var primeraFila, segundaFila []discordgo.MessageComponent
	for indice, cadena := range scraping.Cadenas {
		aspecto, existe := aspectosCadena[cadena.ID]
		if !existe {
			aspecto = aspectoBoton{estilo: discordgo.SecondaryButton}
		}
		boton := discordgo.Button{
			Label:    cadena.Nombre,
			Style:    aspecto.estilo,
			CustomID: identificadorBotonScraping(cadena),
		}
		if aspecto.emoji != "" {
			boton.Emoji = &discordgo.ComponentEmoji{Name: aspecto.emoji}
		}
		if indice < botonesPrimeraFila {
			primeraFila = append(primeraFila, boton)
		} else {
			segundaFila = append(segundaFila, boton)
		}
	}
	segundaFila = append(segundaFila, discordgo.Button{
		Label:    "Todos a la vez",
		Style:    discordgo.PrimaryButton,
		CustomID: identificadorBotonTodos,
		Emoji:    &discordgo.ComponentEmoji{Name: "🚀"},
	})
	_, errorObtenido := manejador.sesion.ChannelMessageSendComplex(canalID, &discordgo.MessageSend{
		Content: mensajeMenuScraping,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: primeraFila},
			discordgo.ActionsRow{Components: segundaFila},
		},
	})
	if errorObtenido != nil {
		log.Printf("Error enviando menú: %v", errorObtenido)
	}
}
