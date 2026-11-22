package discord

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/scraping"
)

// ===== Identificadores de componentes =====

// Los CustomID de los botones forman parte del contrato visible con los
// mensajes ya publicados en Discord: "scrape_<cadenaID>_<rastreador>" lanza una
// cadena, "scrape_0_todos" lanza todas y "refresh_<trabajoID>" consulta un trabajo.
const (
	prefijoBotonScraping     = "scrape_"
	prefijoBotonActualizar   = "refresh_"
	identificadorBotonTodos  = "scrape_0_todos"
	cadenaIDTodas            = 0
	partesIdentificadorBoton = 2
)

// identificadorBotonScraping construye el CustomID del botón de una cadena.
func identificadorBotonScraping(cadena scraping.Cadena) string {
	return fmt.Sprintf("%s%d_%s", prefijoBotonScraping, cadena.ID, cadena.NombreRastreador)
}

// ===== Textos de respuesta =====

const (
	mensajeOpcionNoReconocida = "❌ No se reconoció la opción de scraping seleccionada."
	mensajeCadenaNoReconocida = "❌ No se pudo interpretar la cadena seleccionada."
	mensajeSinUsuario         = "❌ No se pudo identificar tu usuario de Discord para iniciar el scraping."
	mensajeSinTrabajos        = "❌ No se pudo iniciar ningún trabajo de scraping. Control puede no estar disponible; inténtalo nuevamente más tarde."
	mensajeErrorRed           = "❌ Error de red al consultar el estado"
	textoBotonActualizar      = "Actualizar Estado"
	emojiActualizar           = "🔄"
)

// ===== Interacciones =====

// ManejarInteraccion enruta los botones del bot: iniciar scraping o actualizar
// el estado de un trabajo. Ignora cualquier otra interacción.
func (manejador *Manejador) ManejarInteraccion(_ *discordgo.Session, interaccion *discordgo.InteractionCreate) {
	if interaccion.Type != discordgo.InteractionMessageComponent {
		return
	}
	identificadorComponente := interaccion.MessageComponentData().CustomID
	switch {
	case strings.HasPrefix(identificadorComponente, prefijoBotonActualizar):
		manejador.actualizarEstado(interaccion)
	case strings.HasPrefix(identificadorComponente, prefijoBotonScraping):
		manejador.iniciarScraping(interaccion)
	}
}

// responderSeguimiento publica un mensaje en la respuesta diferida de la interacción.
func responderSeguimiento(sesion *discordgo.Session, interaccion *discordgo.InteractionCreate, mensaje string) {
	if _, errorObtenido := sesion.FollowupMessageCreate(interaccion.Interaction, true, &discordgo.WebhookParams{Content: mensaje}); errorObtenido != nil {
		log.Printf("No se pudo responder a la interacción de scraping: %v", errorObtenido)
	}
}

// botonActualizar crea el botón que consulta el estado de un trabajo.
func botonActualizar(etiqueta, trabajoID string) discordgo.Button {
	return discordgo.Button{
		Label: etiqueta, Style: discordgo.SecondaryButton,
		CustomID: prefijoBotonActualizar + trabajoID, Emoji: &discordgo.ComponentEmoji{Name: emojiActualizar},
	}
}

// ===== Inicio de scraping =====

// iniciarScraping interpreta el botón pulsado y pide a Control crear y
// ejecutar el trabajo. Primero difiere la respuesta porque Control puede
// tardar más que los tres segundos que Discord concede para responder.
func (manejador *Manejador) iniciarScraping(interaccion *discordgo.InteractionCreate) {
	if errorObtenido := manejador.sesion.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); errorObtenido != nil {
		log.Printf("No se pudo diferir la interacción de scraping: %v", errorObtenido)
		return
	}
	partesID := strings.SplitN(strings.TrimPrefix(interaccion.MessageComponentData().CustomID, prefijoBotonScraping), "_", partesIdentificadorBoton)
	if len(partesID) != partesIdentificadorBoton {
		responderSeguimiento(manejador.sesion, interaccion, mensajeOpcionNoReconocida)
		return
	}
	nombreRastreador := partesID[1]
	var cadenaID int
	if _, errorObtenido := fmt.Sscanf(partesID[0], "%d", &cadenaID); errorObtenido != nil {
		log.Printf("No se pudo interpretar el identificador de cadena %q: %v", partesID[0], errorObtenido)
		responderSeguimiento(manejador.sesion, interaccion, mensajeCadenaNoReconocida)
		return
	}
	if interaccion.Member == nil || interaccion.Member.User == nil {
		log.Printf("La interacción de scraping no contiene un miembro de Discord")
		responderSeguimiento(manejador.sesion, interaccion, mensajeSinUsuario)
		return
	}
	usuarioID := interaccion.Member.User.ID
	if cadenaID == cadenaIDTodas {
		manejador.iniciarTrabajosMasivos(interaccion, usuarioID)
		return
	}
	nombreCadena := scraping.NombreCadena(cadenaID, nombreRastreador)
	trabajoID, errorObtenido := manejador.control.CrearTrabajo(manejador.contexto, cadenaID, usuarioID)
	if errorObtenido != nil {
		responderSeguimiento(manejador.sesion, interaccion, fmt.Sprintf("❌ No se pudo crear el trabajo para %s: %v", nombreCadena, errorObtenido))
		return
	}
	if errorObtenido := manejador.control.EjecutarTrabajo(manejador.contexto, trabajoID, nombreRastreador); errorObtenido != nil {
		responderSeguimiento(manejador.sesion, interaccion, fmt.Sprintf("❌ No se pudo iniciar el scraper %s: %v", nombreRastreador, errorObtenido))
		return
	}
	if _, errorObtenido := manejador.sesion.FollowupMessageCreate(interaccion.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("🚀 **¡Scraping de %s iniciado!**\nID del trabajo: `%s`\nLa araña `%s` está recolectando productos.", nombreCadena, trabajoID, nombreRastreador),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{botonActualizar(textoBotonActualizar, trabajoID)}},
		},
	}); errorObtenido != nil {
		log.Printf("No se pudo enviar la confirmación del scraping %s: %v", trabajoID, errorObtenido)
	}
	go manejador.monitorearTrabajo(interaccion.ChannelID, trabajoID, nombreCadena, usuarioID)
}

// iniciarTrabajosMasivos solicita un trabajo por cada cadena del catálogo y
// presenta solo los que Control aceptó.
func (manejador *Manejador) iniciarTrabajosMasivos(interaccion *discordgo.InteractionCreate, usuarioID string) {
	var botonesAceptados []discordgo.Button
	for _, cadena := range scraping.Cadenas {
		trabajoID, errorObtenido := manejador.control.CrearTrabajo(manejador.contexto, cadena.ID, usuarioID)
		if errorObtenido == nil {
			errorObtenido = manejador.control.EjecutarTrabajo(manejador.contexto, trabajoID, cadena.NombreRastreador)
		}
		if errorObtenido != nil {
			log.Printf("No se pudo iniciar scraping de %s: %v", cadena.Nombre, errorObtenido)
			continue
		}
		botonesAceptados = append(botonesAceptados, botonActualizar("Actualizar "+cadena.Nombre, trabajoID))
		go manejador.monitorearTrabajo(interaccion.ChannelID, trabajoID, cadena.Nombre, usuarioID)
	}
	parametros := crearRespuestaScrapingMasivo(botonesAceptados)
	if _, errorObtenido := manejador.sesion.FollowupMessageCreate(interaccion.Interaction, true, parametros); errorObtenido != nil {
		log.Printf("No se pudo enviar el resultado del scraping masivo: %v", errorObtenido)
	}
}

// crearRespuestaScrapingMasivo arma el mensaje final y omite los componentes
// cuando ningún trabajo se inició, porque Discord rechaza filas vacías.
func crearRespuestaScrapingMasivo(botonesAceptados []discordgo.Button) *discordgo.WebhookParams {
	if len(botonesAceptados) == 0 {
		return &discordgo.WebhookParams{Content: mensajeSinTrabajos}
	}
	contenido := fmt.Sprintf("🚀 **¡Scraping masivo iniciado!**\nSe iniciaron `%d` trabajos. Usa los botones para revisar su estado.", len(botonesAceptados))
	if len(botonesAceptados) == 1 {
		contenido = "🚀 **¡Scraping iniciado!**\nSe inició `1` trabajo. Usa el botón para revisar su estado."
	}
	componentes := make([]discordgo.MessageComponent, 0, len(botonesAceptados))
	for _, boton := range botonesAceptados {
		componentes = append(componentes, boton)
	}
	return &discordgo.WebhookParams{
		Content:    contenido,
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: componentes}},
	}
}

// ===== Seguimiento y consulta del estado =====

// monitorearTrabajo consulta periódicamente un trabajo y avisa al usuario
// cuando termina. Abandona tras un máximo de consultas fallidas seguidas para
// no sondear indefinidamente a un Control caído, y se detiene al apagar el bot.
func (manejador *Manejador) monitorearTrabajo(canalID, trabajoID, nombreCadena, usuarioID string) {
	fallosConsecutivos := 0
	for {
		select {
		case <-manejador.contexto.Done():
			return
		case <-time.After(manejador.configuracion.IntervaloConsulta):
		}
		trabajo, errorObtenido := manejador.control.ConsultarEstado(manejador.contexto, trabajoID)
		if errorObtenido != nil {
			fallosConsecutivos++
			if fallosConsecutivos >= manejador.configuracion.MaximoFallosMonitor {
				log.Printf("monitor_abandonado trabajo_id=%s error=%v", trabajoID, errorObtenido)
				manejador.enviarMensaje(canalID, fmt.Sprintf("⚠️ <@%s>, dejé de seguir el scraping de **%s** (`%s`) tras %d consultas fallidas (%v). Usa **Actualizar Estado** para revisarlo.", usuarioID, nombreCadena, trabajoID, fallosConsecutivos, errorObtenido))
				return
			}
			continue
		}
		fallosConsecutivos = 0
		if trabajo.Estado != scraping.EstadoCompletado && trabajo.Estado != scraping.EstadoFallido {
			continue
		}
		manejador.enviarMensaje(canalID, mensajeResultadoTrabajo(trabajo, nombreCadena, usuarioID))
		return
	}
}

// mensajeResultadoTrabajo redacta el aviso final de un trabajo terminado.
func mensajeResultadoTrabajo(trabajo scraping.EstadoTrabajo, nombreCadena, usuarioID string) string {
	emoji := "✅"
	resultado := "completado con éxito"
	if trabajo.Estado == scraping.EstadoFallido {
		emoji = "❌"
		resultado = "fallado brutalmente"
	}
	mensaje := fmt.Sprintf("%s <@%s>, ¡Te aviso que tu scraping de **%s** ha %s!\n", emoji, usuarioID, nombreCadena, resultado)
	if trabajo.Estado == scraping.EstadoCompletado {
		mensaje += fmt.Sprintf("📊 Se extrajeron **%d** productos y ya están en la base de datos.", trabajo.ElementosExtraidos)
	} else if trabajo.RegistroErrores != "" {
		mensaje += fmt.Sprintf("🛑 Motivo: `%s`", trabajo.RegistroErrores)
	}
	return mensaje
}

// responderEfimero contesta una interacción con un mensaje visible solo para quien pulsó el botón.
func responderEfimero(sesion *discordgo.Session, interaccion *discordgo.InteractionCreate, mensaje string) {
	if errorObtenido := sesion.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: mensaje, Flags: discordgo.MessageFlagsEphemeral},
	}); errorObtenido != nil {
		log.Printf("No se pudo responder a la interacción: %v", errorObtenido)
	}
}

// actualizarEstado reemplaza el mensaje del botón pulsado con el estado actual
// del trabajo, manteniendo el botón mientras siga pendiente.
func (manejador *Manejador) actualizarEstado(interaccion *discordgo.InteractionCreate) {
	identificadorComponente := interaccion.MessageComponentData().CustomID
	trabajoID := strings.TrimPrefix(identificadorComponente, prefijoBotonActualizar)

	trabajo, errorObtenido := manejador.control.ConsultarEstado(manejador.contexto, trabajoID)
	if errorObtenido != nil {
		if errors.Is(errorObtenido, scraping.ErrComunicacion) {
			responderEfimero(manejador.sesion, interaccion, mensajeErrorRed)
			return
		}
		responderEfimero(manejador.sesion, interaccion, fmt.Sprintf("❌ No se pudo consultar el estado (%v). Verifica las credenciales del servicio.", errorObtenido))
		return
	}

	emojiEstado := "⏳"
	switch trabajo.Estado {
	case scraping.EstadoCompletado:
		emojiEstado = "✅"
	case scraping.EstadoFallido:
		emojiEstado = "❌"
	}
	// Control solo informa cuántos elementos lleva; no hay total, así que se
	// muestra el conteo en lugar de una barra de progreso.
	progreso := ""
	if trabajo.Estado == scraping.EstadoEnProgreso {
		progreso = fmt.Sprintf("\n📊 Progreso: **%d productos extraídos**", trabajo.ElementosExtraidos)
	}
	mensaje := fmt.Sprintf("%s **Estado del Scraping: %s**\nID: `%s`\nSupermercado: **%s**\nEstado: `%s`%s\nÚltima actualización: <t:%d:R>",
		emojiEstado, trabajo.Cadena, trabajoID, trabajo.Cadena, trabajo.Estado, progreso, time.Now().Unix())

	var componentes []discordgo.MessageComponent
	if trabajo.Estado == scraping.EstadoEnProgreso || trabajo.Estado == scraping.EstadoEncolado {
		componentes = []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{botonActualizar(textoBotonActualizar, trabajoID)}},
		}
	}
	if errorObtenido := manejador.sesion.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Content: mensaje, Components: componentes},
	}); errorObtenido != nil {
		log.Printf("No se pudo actualizar el estado del trabajo %s: %v", trabajoID, errorObtenido)
	}
}
