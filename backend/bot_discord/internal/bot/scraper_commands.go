package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/config"
)

// ===== Configuración y cliente HTTP de scraping =====

var (
	urlBaseScraper     string
	tokenScraping      string
	clienteHTTPScraper = &http.Client{}
)

// configurarClienteScraper aplica la configuración validada antes de iniciar el bot.
func configurarClienteScraper(configuracion config.Configuracion) {
	urlBaseScraper = configuracion.URLBaseScraper
	tokenScraping = configuracion.TokenServicioScraping
	clienteHTTPScraper = &http.Client{Timeout: configuracion.TimeoutHTTP}
}

// obtenerURLBaseScraper devuelve la ruta de Control configurada al iniciar.
func obtenerURLBaseScraper() string { return urlBaseScraper }

// ejecutarSolicitudScraper envía una petición HTTP autenticada a Control.
func ejecutarSolicitudScraper(metodo, direccion string, cuerpo io.Reader) (*http.Response, error) {
	peticion, errorObtenido := http.NewRequest(metodo, direccion, cuerpo)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if cuerpo != nil {
		peticion.Header.Set("Content-Type", "application/json")
	}
	peticion.Header.Set("Authorization", "Bearer "+tokenScraping)
	return clienteHTTPScraper.Do(peticion)
}

// enviarPOSTScraper envía un cuerpo JSON por POST a Control.
func enviarPOSTScraper(direccion string, cuerpo io.Reader) (*http.Response, error) {
	return ejecutarSolicitudScraper(http.MethodPost, direccion, cuerpo)
}

// enviarGETScraper consulta Control mediante GET.
func enviarGETScraper(direccion string) (*http.Response, error) {
	return ejecutarSolicitudScraper(http.MethodGet, direccion, nil)
}

// ===== Lectura de respuestas HTTP =====

// EstadoTrabajo contiene los campos de trabajo que el bot muestra a sus usuarios.
type EstadoTrabajo struct {
	Estado             string
	Cadena             string
	ElementosExtraidos int
	RegistroErrores    string
}

// LeerEstadoTrabajo valida una respuesta HTTP de estado de trabajo.
func LeerEstadoTrabajo(respuestaHTTP *http.Response) (EstadoTrabajo, error) {
	if respuestaHTTP.StatusCode != http.StatusOK {
		return EstadoTrabajo{}, fmt.Errorf("HTTP %d", respuestaHTTP.StatusCode)
	}
	var cuerpo map[string]interface{}
	if errorObtenido := json.NewDecoder(respuestaHTTP.Body).Decode(&cuerpo); errorObtenido != nil {
		return EstadoTrabajo{}, fmt.Errorf("respuesta inválida: %w", errorObtenido)
	}
	estado, estadoValido := cuerpo["estado"].(string)
	cadena, cadenaValida := cuerpo["cadena_nombre"].(string)
	if !estadoValido || !cadenaValida {
		return EstadoTrabajo{}, fmt.Errorf("respuesta incompleta")
	}
	resultado := EstadoTrabajo{Estado: estado, Cadena: cadena}
	if elementos, existe := cuerpo["elementos_extraidos"].(float64); existe {
		resultado.ElementosExtraidos = int(elementos)
	}
	if mensajeErrores, existe := cuerpo["registro_errores"].(string); existe {
		resultado.RegistroErrores = mensajeErrores
	}
	return resultado, nil
}

// LeerIDTrabajo valida y extrae el identificador de un trabajo recién creado.
func LeerIDTrabajo(respuestaHTTP *http.Response) (string, bool) {
	var cuerpo map[string]interface{}
	if errorObtenido := json.NewDecoder(respuestaHTTP.Body).Decode(&cuerpo); errorObtenido != nil {
		return "", false
	}
	identificador, existe := cuerpo["id"].(string)
	return identificador, existe && identificador != ""
}

// ===== Comandos e interacción para scraping =====

// enviarMenuScraper presenta opciones de supermercados disponibles en Control.
func enviarMenuScraper(sesionDiscord *discordgo.Session, canalID string) {
	mensaje := &discordgo.MessageSend{
		Content: "🤖 **Menú de Scraping**\nSelecciona el supermercado que deseas scrapear:",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Jumbo",
						Style:    discordgo.SuccessButton,
						CustomID: "scrape_1_jumbo_rsc",
						Emoji:    &discordgo.ComponentEmoji{Name: "🐘"},
					},
					discordgo.Button{
						Label:    "Lider",
						Style:    discordgo.PrimaryButton,
						CustomID: "scrape_2_lider_rsc",
						Emoji:    &discordgo.ComponentEmoji{Name: "🛒"},
					},
					discordgo.Button{
						Label:    "Santa Isabel",
						Style:    discordgo.DangerButton,
						CustomID: "scrape_4_santa_isabel_rsc",
						Emoji:    &discordgo.ComponentEmoji{Name: "🛍️"},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "A Cuenta",
						Style:    discordgo.SuccessButton,
						CustomID: "scrape_5_acuenta_rsc",
						Emoji:    &discordgo.ComponentEmoji{Name: "📉"},
					},
					discordgo.Button{
						Label:    "Cugat",
						Style:    discordgo.SecondaryButton,
						CustomID: "scrape_6_cugat_rsc",
						Emoji:    &discordgo.ComponentEmoji{Name: "🏪"},
					},
					discordgo.Button{
						Label:    "Todos a la vez",
						Style:    discordgo.PrimaryButton,
						CustomID: "scrape_0_todos",
						Emoji:    &discordgo.ComponentEmoji{Name: "🚀"},
					},
				},
			},
		},
	}
	_, errorObtenido := sesionDiscord.ChannelMessageSendComplex(canalID, mensaje)
	if errorObtenido != nil {
		log.Printf("Error enviando menú: %v", errorObtenido)
	}
}

// manejarInteraccion enruta componentes de scraping y actualización de estado.
func manejarInteraccion(sesionDiscord *discordgo.Session, interaccion *discordgo.InteractionCreate) {
	if interaccion.Type != discordgo.InteractionMessageComponent {
		return
	}

	identificadorComponente := interaccion.MessageComponentData().CustomID

	if strings.HasPrefix(identificadorComponente, "refresh_") {
		manejarActualizacionEstado(sesionDiscord, interaccion)
		return
	}

	if strings.HasPrefix(identificadorComponente, "scrape_") {
		iniciarScraping(sesionDiscord, interaccion)
		return
	}
}

// iniciarScraping interpreta la selección y solicita la ejecución a Control.
func iniciarScraping(sesionDiscord *discordgo.Session, interaccion *discordgo.InteractionCreate) {
	sesionDiscord.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	partesID := strings.SplitN(strings.TrimPrefix(interaccion.MessageComponentData().CustomID, "scrape_"), "_", 2)
	if len(partesID) != 2 {
		return
	}
	var cadenaID int
	if _, errorObtenido := fmt.Sscanf(partesID[0], "%d", &cadenaID); errorObtenido != nil {
		log.Printf("No se pudo interpretar el identificador de cadena %q: %v", partesID[0], errorObtenido)
		return
	}
	if interaccion.Member == nil || interaccion.Member.User == nil {
		log.Printf("La interacción de scraping no contiene un miembro de Discord")
		return
	}
	usuarioID := interaccion.Member.User.ID
	if cadenaID == 0 {
		iniciarTrabajosMasivos(sesionDiscord, interaccion, usuarioID)
		return
	}
	nombreCadena := nombreCadenaScraper(cadenaID, partesID[1])
	trabajoID, errorObtenido := crearTrabajoScraper(cadenaID, usuarioID)
	if errorObtenido != nil {
		responderSeguimiento(sesionDiscord, interaccion, fmt.Sprintf("❌ No se pudo crear el trabajo para %s: %v", nombreCadena, errorObtenido))
		return
	}
	if errorObtenido := ejecutarTrabajoScraper(trabajoID, partesID[1]); errorObtenido != nil {
		responderSeguimiento(sesionDiscord, interaccion, fmt.Sprintf("❌ No se pudo iniciar el scraper %s: %v", partesID[1], errorObtenido))
		return
	}
	sesionDiscord.FollowupMessageCreate(interaccion.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("🚀 **¡Scraping de %s iniciado!**\nID del trabajo: `%s`\nLa araña `%s` está recolectando productos.", nombreCadena, trabajoID, partesID[1]),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Actualizar Estado", Style: discordgo.SecondaryButton, CustomID: "refresh_" + trabajoID, Emoji: &discordgo.ComponentEmoji{Name: "🔄"}},
			}},
		},
	})
	go monitorearTrabajoYNotificar(sesionDiscord, interaccion.ChannelID, trabajoID, nombreCadena, usuarioID)
}

// ===== Configuración de cadenas disponibles =====

// cadenaScraper identifica la fuente y el rastreador usados por las opciones del bot.
type cadenaScraper struct {
	cadenaID         int
	nombreRastreador string
	nombreCadena     string
}

// cadenasScraper contiene las opciones contractuales expuestas por los botones.
var cadenasScraper = []cadenaScraper{
	{cadenaID: 1, nombreRastreador: "jumbo_rsc", nombreCadena: "Jumbo"},
	{cadenaID: 2, nombreRastreador: "lider_rsc", nombreCadena: "Lider"},
	{cadenaID: 4, nombreRastreador: "santa_isabel_rsc", nombreCadena: "Santa Isabel"},
	{cadenaID: 5, nombreRastreador: "acuenta_rsc", nombreCadena: "A Cuenta"},
	{cadenaID: 6, nombreRastreador: "cugat_rsc", nombreCadena: "Cugat"},
}

// iniciarTrabajosMasivos solicita un trabajo por cada cadena y presenta los aceptados.
func iniciarTrabajosMasivos(sesionDiscord *discordgo.Session, interaccion *discordgo.InteractionCreate, usuarioID string) {
	var trabajosExitosos []discordgo.Button
	for _, cadena := range cadenasScraper {
		trabajoID, errorObtenido := crearTrabajoScraper(cadena.cadenaID, usuarioID)
		if errorObtenido == nil {
			errorObtenido = ejecutarTrabajoScraper(trabajoID, cadena.nombreRastreador)
		}
		if errorObtenido != nil {
			log.Printf("No se pudo iniciar scraping de %s: %v", cadena.nombreCadena, errorObtenido)
			continue
		}
		trabajosExitosos = append(trabajosExitosos, discordgo.Button{
			Label: "Actualizar " + cadena.nombreCadena, Style: discordgo.SecondaryButton,
			CustomID: "refresh_" + trabajoID, Emoji: &discordgo.ComponentEmoji{Name: "🔄"},
		})
		go monitorearTrabajoYNotificar(sesionDiscord, interaccion.ChannelID, trabajoID, cadena.nombreCadena, usuarioID)
	}
	contenido := fmt.Sprintf("🚀 **¡Scraping masivo iniciado!**\nSe iniciaron `%d` trabajos. Usa los botones para revisar su estado.", len(trabajosExitosos))
	sesionDiscord.FollowupMessageCreate(interaccion.Interaction, true, &discordgo.WebhookParams{
		Content: contenido,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: botonesComoComponentes(trabajosExitosos)},
		},
	})
}

// nombreCadenaScraper obtiene el nombre legible de la cadena elegida.
func nombreCadenaScraper(cadenaID int, nombrePredeterminado string) string {
	for _, cadena := range cadenasScraper {
		if cadena.cadenaID == cadenaID {
			return cadena.nombreCadena
		}
	}
	return nombrePredeterminado
}

// ===== Creación y ejecución de trabajos =====

// crearTrabajoScraper registra un trabajo y valida la respuesta de Control.
func crearTrabajoScraper(cadenaID int, usuarioID string) (string, error) {
	cuerpoJSON, errorObtenido := json.Marshal(map[string]interface{}{
		"cadena_id": cadenaID, "disparado_por_usuario_id": usuarioID,
	})
	if errorObtenido != nil {
		return "", fmt.Errorf("no se pudo serializar la creación: %w", errorObtenido)
	}
	respuestaHTTP, errorObtenido := enviarPOSTScraper(obtenerURLBaseScraper(), bytes.NewReader(cuerpoJSON))
	if errorObtenido != nil {
		return "", fmt.Errorf("falló la comunicación con Control: %w", errorObtenido)
	}
	defer respuestaHTTP.Body.Close()
	if respuestaHTTP.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("Control respondió HTTP %d al crear", respuestaHTTP.StatusCode)
	}
	trabajoID, valido := LeerIDTrabajo(respuestaHTTP)
	if !valido {
		return "", fmt.Errorf("Control devolvió un identificador de trabajo inválido")
	}
	return trabajoID, nil
}

// ejecutarTrabajoScraper encola un rastreador en el trabajo previamente creado.
func ejecutarTrabajoScraper(trabajoID, nombreRastreador string) error {
	cuerpoJSON, errorObtenido := json.Marshal(map[string]string{"spider": nombreRastreador})
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo serializar la ejecución: %w", errorObtenido)
	}
	respuestaHTTP, errorObtenido := enviarPOSTScraper(
		fmt.Sprintf("%s/%s/ejecutar", obtenerURLBaseScraper(), trabajoID),
		bytes.NewReader(cuerpoJSON),
	)
	if errorObtenido != nil {
		return fmt.Errorf("falló la comunicación con Control: %w", errorObtenido)
	}
	defer respuestaHTTP.Body.Close()
	if respuestaHTTP.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Control respondió HTTP %d al ejecutar", respuestaHTTP.StatusCode)
	}
	return nil
}

// responderSeguimiento notifica un error de scraping en la respuesta diferida.
func responderSeguimiento(sesionDiscord *discordgo.Session, interaccion *discordgo.InteractionCreate, mensaje string) {
	if _, errorObtenido := sesionDiscord.FollowupMessageCreate(interaccion.Interaction, true, &discordgo.WebhookParams{Content: mensaje}); errorObtenido != nil {
		log.Printf("No se pudo responder a la interacción de scraping: %v", errorObtenido)
	}
}

// botonesComoComponentes adapta botones de Discord al tipo de componente requerido.
func botonesComoComponentes(botones []discordgo.Button) []discordgo.MessageComponent {
	componentes := make([]discordgo.MessageComponent, 0, len(botones))
	for _, boton := range botones {
		componentes = append(componentes, boton)
	}
	return componentes
}

// ===== Seguimiento y consulta del estado =====

// monitorearTrabajoYNotificar consulta periódicamente el trabajo y notifica su resultado.
func monitorearTrabajoYNotificar(sesionDiscord *discordgo.Session, canalID, trabajoID, nombreCadena, usuarioID string) {
	fallos := 0
	for {
		time.Sleep(configuracionActiva.IntervaloConsulta)

		respuestaHTTP, errorObtenido := enviarGETScraper(fmt.Sprintf("%s/%s", obtenerURLBaseScraper(), trabajoID))
		var trabajo EstadoTrabajo
		if errorObtenido == nil {
			trabajo, errorObtenido = LeerEstadoTrabajo(respuestaHTTP)
			if errorCierre := respuestaHTTP.Body.Close(); errorCierre != nil {
				log.Printf("No se pudo cerrar la respuesta del trabajo %s: %v", trabajoID, errorCierre)
			}
		}
		if errorObtenido != nil {
			fallos++
			if fallos >= configuracionActiva.MaximoFallosMonitor {
				log.Printf("monitor_abandonado trabajo_id=%s error=%v", trabajoID, errorObtenido)
				if _, errorDiscord := sesionDiscord.ChannelMessageSend(canalID, fmt.Sprintf("⚠️ <@%s>, dejé de seguir el scraping de **%s** (`%s`) tras %d consultas fallidas (%v). Usa **Actualizar Estado** para revisarlo.", usuarioID, nombreCadena, trabajoID, fallos, errorObtenido)); errorDiscord != nil {
					log.Printf("No se pudo notificar el abandono del seguimiento del trabajo %s: %v", trabajoID, errorDiscord)
				}
				return
			}
			continue
		}
		fallos = 0

		estado := trabajo.Estado
		if estado == "completado" || estado == "fallido" {
			extraidos := trabajo.ElementosExtraidos

			emoji := "✅"
			resultado := "completado con éxito"
			if estado == "fallido" {
				emoji = "❌"
				resultado = "fallado brutalmente"
			}

			mensajeNotificacion := fmt.Sprintf("%s <@%s>, ¡Te aviso que tu scraping de **%s** ha %s!\n", emoji, usuarioID, nombreCadena, resultado)
			if estado == "completado" {
				mensajeNotificacion += fmt.Sprintf("📊 Se extrajeron **%d** productos y ya están en la base de datos.", extraidos)
			} else if trabajo.RegistroErrores != "" {
				mensajeNotificacion += fmt.Sprintf("🛑 Motivo: `%s`", trabajo.RegistroErrores)
			}

			if _, errorDiscord := sesionDiscord.ChannelMessageSend(canalID, mensajeNotificacion); errorDiscord != nil {
				log.Printf("No se pudo notificar el resultado del trabajo %s: %v", trabajoID, errorDiscord)
			}
			break
		}
	}
}

// manejarActualizacionEstado actualiza la respuesta Discord con el estado del trabajo.
func manejarActualizacionEstado(sesionDiscord *discordgo.Session, interaccion *discordgo.InteractionCreate) {
	identificadorComponente := interaccion.MessageComponentData().CustomID
	trabajoID := strings.TrimPrefix(identificadorComponente, "refresh_")

	// Obtener estado
	respuestaHTTP, errorObtenido := enviarGETScraper(fmt.Sprintf("%s/%s", obtenerURLBaseScraper(), trabajoID))
	if errorObtenido != nil {
		sesionDiscord.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Error de red al consultar el estado",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	defer respuestaHTTP.Body.Close()

	trabajo, errorObtenido := LeerEstadoTrabajo(respuestaHTTP)
	if errorObtenido != nil {
		sesionDiscord.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❌ No se pudo consultar el estado (%v). Verifica las credenciales del servicio.", errorObtenido),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	estadoTrabajo, elementosExtraidos, nombreCadena := trabajo.Estado, trabajo.ElementosExtraidos, trabajo.Cadena

	emojiEstado := "⏳"
	if estadoTrabajo == "completado" {
		emojiEstado = "✅"
	} else if estadoTrabajo == "fallido" {
		emojiEstado = "❌"
	}

	// Generar barrita de progreso visual (simulada basada en un estimado, digamos 5000)
	// Si no sabemos el total, mostramos los extraidos en texto
	progresoVisual := ""
	if estadoTrabajo == "en_progreso" {
		progresoVisual = fmt.Sprintf("\n📊 Progreso: **%d productos extraídos**", elementosExtraidos)
	}

	nuevoMensaje := fmt.Sprintf("%s **Estado del Scraping: %s**\nID: `%s`\nSupermercado: **%s**\nEstado: `%s`%s\nÚltima actualización: <t:%d:R>",
		emojiEstado, nombreCadena, trabajoID, nombreCadena, estadoTrabajo, progresoVisual, time.Now().Unix())

	// Mantener el botón si sigue en progreso
	var componentes []discordgo.MessageComponent
	if estadoTrabajo == "en_progreso" || estadoTrabajo == "encolado" {
		componentes = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Actualizar Estado",
						Style:    discordgo.SecondaryButton,
						CustomID: identificadorComponente,
						Emoji: &discordgo.ComponentEmoji{
							Name: "🔄",
						},
					},
				},
			},
		}
	}

	sesionDiscord.InteractionRespond(interaccion.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    nuevoMensaje,
			Components: componentes,
		},
	})
}
