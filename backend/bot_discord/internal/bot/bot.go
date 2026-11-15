// Package bot integra Discord, notificaciones de GitHub y operaciones de scraping.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// ===== Estado y ciclo de vida =====

var (
	mutexDebounceReviews  sync.Mutex
	temporizadoresReviews = make(map[int]*time.Timer)
	usuariosReviews       = make(map[int][]string)
	configuracionActiva   config.Configuracion
)

// Run carga configuración, conecta Discord y mantiene el servidor HTTP activo.
func Run() error {
	if errorObtenido := godotenv.Load(); errorObtenido != nil {
		log.Printf("Aviso: no se encontró .env; se usará la configuración del entorno: %v", errorObtenido)
	}
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil {
		return errorObtenido
	}
	configuracionActiva = configuracion
	configurarClienteScraper(configuracion)

	sesionDiscord, errorObtenido := discordgo.New("Bot " + configuracion.TokenDiscord)
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo crear la sesión de Discord: %w", errorObtenido)
	}
	sesionDiscord.AddHandler(messageCreate)
	sesionDiscord.AddHandler(manejarInteraccion)
	sesionDiscord.Identify.Intents = discordgo.IntentsGuildMessages
	if errorObtenido := sesionDiscord.Open(); errorObtenido != nil {
		return fmt.Errorf("no se pudo abrir la conexión de Discord: %w", errorObtenido)
	}
	defer func() {
		if errorObtenido := sesionDiscord.Close(); errorObtenido != nil {
			log.Printf("No se pudo cerrar la sesión de Discord: %v", errorObtenido)
		}
	}()
	log.Println("Bot conectado a Discord!")

	enrutador := gin.Default()
	enrutador.POST("/webhooks/github", func(contextoHTTP *gin.Context) {
		evento := contextoHTTP.GetHeader("X-GitHub-Event")
		var cargaGitHub map[string]interface{}
		if errorObtenido := contextoHTTP.BindJSON(&cargaGitHub); errorObtenido != nil {
			contextoHTTP.JSON(http.StatusBadRequest, gin.H{"error": "Payload inválido"})
			return
		}
		switch evento {
		case "ping":
			contextoHTTP.JSON(http.StatusOK, gin.H{"status": "Ping recibido"})
			return
		case "push":
			handlePush(sesionDiscord, configuracion.CanalPushDiscordID, cargaGitHub)
		case "pull_request":
			handlePullRequest(sesionDiscord, configuracion.CanalDiscordID, cargaGitHub)
		case "pull_request_review":
			handlePullRequestReview(sesionDiscord, configuracion.CanalDiscordID, cargaGitHub)
		case "issue_comment":
			handleIssueComment(sesionDiscord, configuracion.CanalDiscordID, cargaGitHub)
		default:
			log.Printf("Evento de GitHub ignorado: %s", evento)
		}
		contextoHTTP.JSON(http.StatusOK, gin.H{"status": "Procesado"})
	})
	servidorHTTP := &http.Server{
		Addr:              ":" + configuracion.PuertoWebhook,
		Handler:           enrutador,
		ReadHeaderTimeout: configuracion.TimeoutCabeceraHTTP,
	}
	resultadoServidor := make(chan error, 1)
	go func() {
		log.Printf("Servidor Webhook escuchando en el puerto %s", configuracion.PuertoWebhook)
		resultadoServidor <- servidorHTTP.ListenAndServe()
	}()

	contextoSenales, cancelarSenales := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer cancelarSenales()
	select {
	case errorObtenido := <-resultadoServidor:
		if !errors.Is(errorObtenido, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks: %w", errorObtenido)
		}
	case <-contextoSenales.Done():
		contextoCierre, cancelarCierre := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelarCierre()
		if errorObtenido := servidorHTTP.Shutdown(contextoCierre); errorObtenido != nil {
			return fmt.Errorf("no se pudo cerrar el servidor de webhooks: %w", errorObtenido)
		}
		if errorObtenido := <-resultadoServidor; !errors.Is(errorObtenido, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks durante el cierre: %w", errorObtenido)
		}
	}
	return nil
}

// ===== Comandos de Discord =====

// messageCreate responde a comandos de texto simples.
func messageCreate(sesionDiscord *discordgo.Session, mensajeDiscord *discordgo.MessageCreate) {
	if mensajeDiscord.Author.ID == sesionDiscord.State.User.ID {
		return
	}

	if mensajeDiscord.Content == "!ping" {
		sesionDiscord.ChannelMessageSend(mensajeDiscord.ChannelID, "Pong! El bot está vivo y respirando.")
	}

	if mensajeDiscord.Content == "!scrapear" {
		enviarMenuScraper(sesionDiscord, mensajeDiscord.ChannelID)
	}

	if mensajeDiscord.Content == "!help" {
		mensajeAyuda := "📚 **Lista de Comandos del Bot**\n\n" +
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

		sesionDiscord.ChannelMessageSend(mensajeDiscord.ChannelID, mensajeAyuda)
	}
}

// ===== Notificaciones de GitHub =====

// handlePush notifica sobre nuevos commits.
func handlePush(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	datosEmisor, esValido := cargaGitHub["pusher"].(map[string]interface{})
	if !esValido {
		return
	}
	nombreEmisor := datosEmisor["name"].(string)

	referencia, _ := cargaGitHub["ref"].(string)
	rama := strings.Replace(referencia, "refs/heads/", "", 1)

	esFusion := false
	var mensajesCommits string

	if commitEncabezado, esValido := cargaGitHub["head_commit"].(map[string]interface{}); esValido && commitEncabezado != nil {
		mensaje, _ := commitEncabezado["message"].(string)
		if strings.HasPrefix(mensaje, "Merge pull request") || strings.HasPrefix(mensaje, "Merge branch") {
			esFusion = true
		}
	}

	if confirmaciones, esValido := cargaGitHub["commits"].([]interface{}); esValido && len(confirmaciones) > 0 {
		mensajesCommits = "\n**Commits:**"
		// Limitar a 5 commits para no hacer spam si empujan muchos de golpe
		limite := len(confirmaciones)
		if limite > 5 {
			limite = 5
		}
		for indice := 0; indice < limite; indice++ {
			commitActual := confirmaciones[indice].(map[string]interface{})
			mensaje, _ := commitActual["message"].(string)
			primeraLinea := strings.Split(mensaje, "\n")[0] // Solo la primera línea

			identificador, _ := commitActual["id"].(string)
			identificadorCorto := identificador
			if len(identificador) > 7 {
				identificadorCorto = identificador[:7]
			}
			direccion, _ := commitActual["url"].(string)

			// Formato: - [`shortID`](url) Mensaje
			mensajesCommits += fmt.Sprintf("\n- [`%s`](<%s>) %s", identificadorCorto, direccion, primeraLinea)
		}
		if len(confirmaciones) > 5 {
			mensajesCommits += fmt.Sprintf("\n*... y %d commits más.*", len(confirmaciones)-5)
		}
	}

	var mensaje string
	if esFusion {
		mensaje = fmt.Sprintf("🔀 **Merge Realizado** por `%s` en la rama `%s`.", nombreEmisor, rama)
	} else {
		mensaje = fmt.Sprintf("🚀 **Nuevo Push** de `%s` en la rama `%s`.", nombreEmisor, rama)
	}

	mensaje += mensajesCommits
	sesionDiscord.ChannelMessageSend(canalID, mensaje)
}

// handlePullRequest notifica creación de PR, etiquetas, revisiones y merges.
func handlePullRequest(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	accion, _ := cargaGitHub["action"].(string)

	datosPR, esValido := cargaGitHub["pull_request"].(map[string]interface{})
	if !esValido {
		return
	}

	titulo, _ := datosPR["title"].(string)
	direccion, _ := datosPR["html_url"].(string)
	datosUsuario, _ := datosPR["user"].(map[string]interface{})
	autor, _ := datosUsuario["login"].(string)

	// Convertir PR number a float64 (JSON genérico) y luego a int
	numeroDecimal, esValido := datosPR["number"].(float64)
	numero := 0
	if esValido {
		numero = int(numeroDecimal)
	}

	switch accion {
	case "opened", "reopened":
		// Extraer Assignees
		var usuariosAsignados []string
		if asignados, esValido := datosPR["assignees"].([]interface{}); esValido {
			for _, asignadoActual := range asignados {
				datosAsignado := asignadoActual.(map[string]interface{})
				nombreUsuario := datosAsignado["login"].(string)
				usuariosAsignados = append(usuariosAsignados, mapGitHubToDiscord(nombreUsuario))
			}
		}

		// Extraer Requested Reviewers
		var usuariosRevisores []string
		if revisores, esValido := datosPR["requested_reviewers"].([]interface{}); esValido {
			for _, revisorActual := range revisores {
				datosRevisor := revisorActual.(map[string]interface{})
				nombreUsuario := datosRevisor["login"].(string)
				usuariosRevisores = append(usuariosRevisores, mapGitHubToDiscord(nombreUsuario))
			}
		}

		// Extraer Labels
		var etiquetas []string
		if etiquetas, esValido := datosPR["labels"].([]interface{}); esValido {
			for _, etiquetaActual := range etiquetas {
				datosEtiqueta := etiquetaActual.(map[string]interface{})
				nombre := datosEtiqueta["name"].(string)
				etiquetas = append(etiquetas, "`"+nombre+"`")
			}
		}

		mensaje := fmt.Sprintf("🛠️ **Nuevo Pull Request** por `%s`\n**Título**: %s\n**Link**: %s", autor, titulo, direccion)

		if len(etiquetas) > 0 {
			mensaje += fmt.Sprintf("\n**Etiquetas**: %s", strings.Join(etiquetas, ", "))
		}

		if len(usuariosAsignados) > 0 {
			mensaje += fmt.Sprintf("\n**Asignados**: %s", strings.Join(usuariosAsignados, " "))
		}

		if len(usuariosRevisores) > 0 {
			mensaje += fmt.Sprintf("\n**Reviewers Solicitados**: %s", strings.Join(usuariosRevisores, " "))
		}

		sesionDiscord.ChannelMessageSend(canalID, mensaje)

	case "closed":
		datosRemitente, _ := cargaGitHub["sender"].(map[string]interface{})
		nombreRemitente, _ := datosRemitente["login"].(string)

		fusionado, _ := datosPR["merged"].(bool)
		if fusionado {
			mensaje := fmt.Sprintf("✅ **Pull Request Aceptado (Merged)** por `%s`\n**Título**: %s\n**Link**: %s", nombreRemitente, titulo, direccion)
			sesionDiscord.ChannelMessageSend(canalID, mensaje)
		} else {
			mensaje := fmt.Sprintf("❌ **Pull Request Rechazado/Cerrado** por `%s`\n**Título**: %s\n**Link**: %s", nombreRemitente, titulo, direccion)
			sesionDiscord.ChannelMessageSend(canalID, mensaje)
		}

	case "review_requested":

		if revisorSolicitado, esValido := cargaGitHub["requested_reviewer"].(map[string]interface{}); esValido {
			revisor := revisorSolicitado["login"].(string)
			usuarioDiscord := mapGitHubToDiscord(revisor)

			mutexDebounceReviews.Lock()

			// Solo añadir si no existe ya para evitar duplicados en la lista (por si acaso GitHub manda eventos de mas)
			existe := false
			for _, usuario := range usuariosReviews[numero] {
				if usuario == usuarioDiscord {
					existe = true
					break
				}
			}
			if !existe {
				usuariosReviews[numero] = append(usuariosReviews[numero], usuarioDiscord)
			}

			if temporizador, existeTemporizador := temporizadoresReviews[numero]; existeTemporizador {
				temporizador.Stop()
			}

			// Acumular solicitudes repetidas de revisión antes de enviar una notificación.
			temporizadoresReviews[numero] = time.AfterFunc(configuracionActiva.RetrasoDebounceReviews, func() {
				mutexDebounceReviews.Lock()
				usuarios := usuariosReviews[numero]
				delete(usuariosReviews, numero)
				delete(temporizadoresReviews, numero)
				mutexDebounceReviews.Unlock()

				if len(usuarios) > 0 {
					// Las URL entre <> evitan que Discord genere la caja gigante de previsualización
					mensaje := fmt.Sprintf("👀 **Review Solicitado** en el PR #%d\nSe ha solicitado la revisión de %s.\n**Link**: <%s>", numero, strings.Join(usuarios, ", "), direccion)
					sesionDiscord.ChannelMessageSend(canalID, mensaje)
				}
			})
			mutexDebounceReviews.Unlock()
		}

	case "labeled":
		if datosEtiqueta, esValido := cargaGitHub["label"].(map[string]interface{}); esValido {
			nombreEtiqueta := datosEtiqueta["name"].(string)
			mensaje := fmt.Sprintf("🏷️ **Nueva Etiqueta** en el PR #%d\nSe añadió la etiqueta `%s`.\n**Link**: <%s>", numero, nombreEtiqueta, direccion)
			sesionDiscord.ChannelMessageSend(canalID, mensaje)
		}
	}
}

// ===== Mapeo de usuarios =====

// mapGitHubToDiscord traduce usuarios de GitHub a identificadores de Discord o nombres.
func mapGitHubToDiscord(usuarioGitHub string) string {
	// Diccionario estático de miembros del equipo
	usuarios := map[string]string{
		"VichoMatus": "<@!537347874875506698>",
		"danirc2024": "<@!1221186570455879702>",
		"RCarrascoO": "<@!410177503592972288>",
		"chelo132":   "<@!467880145877991432>",
		"EsbanV":     "<@!265591689211674624>", // ID de Esban
		"Web":        "<@!265591689211674624>", // Mapeo para su otro nick de GitHub
	}

	if mencionDiscord, existe := usuarios[usuarioGitHub]; existe {
		return mencionDiscord
	}

	// Si no está mapeado, retornar solo el nombre de Github resaltado
	return "`@" + usuarioGitHub + "`"
}

// ===== Notificaciones de revisiones y comentarios =====

// handlePullRequestReview notifica cuando alguien aprueba, rechaza o comenta un PR.
func handlePullRequestReview(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	accion, _ := cargaGitHub["action"].(string)
	if accion != "submitted" {
		return
	}

	datosRevision, esValido := cargaGitHub["review"].(map[string]interface{})
	if !esValido {
		return
	}

	datosPR, esValido := cargaGitHub["pull_request"].(map[string]interface{})
	if !esValido {
		return
	}

	estado, _ := datosRevision["state"].(string) // "approved", "changes_requested", "commented"
	cuerpo, _ := datosRevision["body"].(string)
	direccion, _ := datosRevision["html_url"].(string)

	datosUsuario, _ := datosRevision["user"].(map[string]interface{})
	revisor, _ := datosUsuario["login"].(string)
	usuarioDiscord := mapGitHubToDiscord(revisor)

	numeroDecimal, _ := datosPR["number"].(float64)
	numero := int(numeroDecimal)

	// Solo notificar si hay comentario o si es una aprobación/rechazo
	if cuerpo == "" && estado == "commented" {
		return
	}

	var emojiEstado string
	var textoEstado string

	switch estado {
	case "approved":
		emojiEstado = "✅"
		textoEstado = "aprobó"
	case "changes_requested":
		emojiEstado = "❌"
		textoEstado = "solicitó cambios en"
	case "commented":
		emojiEstado = "💬"
		textoEstado = "comentó en"
	default:
		emojiEstado = "📝"
		textoEstado = "revisó"
	}

	mensaje := fmt.Sprintf("%s **Review**: %s %s el PR #%d\n", emojiEstado, usuarioDiscord, textoEstado, numero)
	if cuerpo != "" {
		if len(cuerpo) > 300 {
			cuerpo = cuerpo[:300] + "..."
		}
		mensaje += fmt.Sprintf("> *\"%s\"*\n", cuerpo)
	}
	mensaje += fmt.Sprintf("**Link**: <%s>", direccion)

	sesionDiscord.ChannelMessageSend(canalID, mensaje)
}

// handleIssueComment notifica cuando alguien comenta en el hilo de un PR.
func handleIssueComment(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	accion, _ := cargaGitHub["action"].(string)
	if accion != "created" {
		return
	}

	datosIncidencia, esValido := cargaGitHub["issue"].(map[string]interface{})
	if !esValido {
		return
	}
	if _, esPR := datosIncidencia["pull_request"]; !esPR {
		return // Ignorar comentarios en issues normales por ahora
	}

	datosComentario, esValido := cargaGitHub["comment"].(map[string]interface{})
	if !esValido {
		return
	}

	cuerpo, _ := datosComentario["body"].(string)
	direccion, _ := datosComentario["html_url"].(string)

	datosUsuario, _ := datosComentario["user"].(map[string]interface{})
	usuarioComentarista, _ := datosUsuario["login"].(string)
	usuarioDiscord := mapGitHubToDiscord(usuarioComentarista)

	numeroDecimal, _ := datosIncidencia["number"].(float64)
	numero := int(numeroDecimal)

	if len(cuerpo) > 300 {
		cuerpo = cuerpo[:300] + "..."
	}

	mensaje := fmt.Sprintf("💬 **Nuevo Comentario** de %s en el PR #%d\n> *\"%s\"*\n**Link**: <%s>", usuarioDiscord, numero, cuerpo, direccion)
	sesionDiscord.ChannelMessageSend(canalID, mensaje)
}
