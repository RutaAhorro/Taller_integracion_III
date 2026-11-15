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
	if err := godotenv.Load(); err != nil {
		log.Printf("Aviso: no se encontró .env; se usará la configuración del entorno: %v", err)
	}
	configuracion, err := config.Cargar()
	if err != nil {
		return err
	}
	configuracionActiva = configuracion
	configurarClienteScraper(configuracion)

	sesionDiscord, err := discordgo.New("Bot " + configuracion.TokenDiscord)
	if err != nil {
		return fmt.Errorf("no se pudo crear la sesión de Discord: %w", err)
	}
	sesionDiscord.AddHandler(messageCreate)
	sesionDiscord.AddHandler(handleInteraction)
	sesionDiscord.Identify.Intents = discordgo.IntentsGuildMessages
	if err := sesionDiscord.Open(); err != nil {
		return fmt.Errorf("no se pudo abrir la conexión de Discord: %w", err)
	}
	defer func() {
		if err := sesionDiscord.Close(); err != nil {
			log.Printf("No se pudo cerrar la sesión de Discord: %v", err)
		}
	}()
	log.Println("Bot conectado a Discord!")

	router := gin.Default()
	router.POST("/webhooks/github", func(contextoHTTP *gin.Context) {
		evento := contextoHTTP.GetHeader("X-GitHub-Event")
		var cargaGitHub map[string]interface{}
		if err := contextoHTTP.BindJSON(&cargaGitHub); err != nil {
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
		Handler:           router,
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
	case err := <-resultadoServidor:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks: %w", err)
		}
	case <-contextoSenales.Done():
		contextoCierre, cancelarCierre := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelarCierre()
		if err := servidorHTTP.Shutdown(contextoCierre); err != nil {
			return fmt.Errorf("no se pudo cerrar el servidor de webhooks: %w", err)
		}
		if err := <-resultadoServidor; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks durante el cierre: %w", err)
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
		sendScraperMenu(sesionDiscord, mensajeDiscord.ChannelID)
	}

	if mensajeDiscord.Content == "!help" {
		helpMsg := "📚 **Lista de Comandos del Bot**\n\n" +
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

		sesionDiscord.ChannelMessageSend(mensajeDiscord.ChannelID, helpMsg)
	}
}

// ===== Notificaciones de GitHub =====

// handlePush notifica sobre nuevos commits.
func handlePush(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	pusherMap, ok := cargaGitHub["pusher"].(map[string]interface{})
	if !ok {
		return
	}
	pusherName := pusherMap["name"].(string)

	ref, _ := cargaGitHub["ref"].(string)
	branch := strings.Replace(ref, "refs/heads/", "", 1)

	isMerge := false
	var commitsMsg string

	if headCommit, ok := cargaGitHub["head_commit"].(map[string]interface{}); ok && headCommit != nil {
		message, _ := headCommit["message"].(string)
		if strings.HasPrefix(message, "Merge pull request") || strings.HasPrefix(message, "Merge branch") {
			isMerge = true
		}
	}

	if commits, ok := cargaGitHub["commits"].([]interface{}); ok && len(commits) > 0 {
		commitsMsg = "\n**Commits:**"
		// Limitar a 5 commits para no hacer spam si empujan muchos de golpe
		limit := len(commits)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			c := commits[i].(map[string]interface{})
			msg, _ := c["message"].(string)
			firstLine := strings.Split(msg, "\n")[0] // Solo la primera línea

			id, _ := c["id"].(string)
			shortID := id
			if len(id) > 7 {
				shortID = id[:7]
			}
			url, _ := c["url"].(string)

			// Formato: - [`shortID`](url) Mensaje
			commitsMsg += fmt.Sprintf("\n- [`%s`](<%s>) %s", shortID, url, firstLine)
		}
		if len(commits) > 5 {
			commitsMsg += fmt.Sprintf("\n*... y %d commits más.*", len(commits)-5)
		}
	}

	var msg string
	if isMerge {
		msg = fmt.Sprintf("🔀 **Merge Realizado** por `%s` en la rama `%s`.", pusherName, branch)
	} else {
		msg = fmt.Sprintf("🚀 **Nuevo Push** de `%s` en la rama `%s`.", pusherName, branch)
	}

	msg += commitsMsg
	sesionDiscord.ChannelMessageSend(canalID, msg)
}

// handlePullRequest notifica creación de PR, etiquetas, revisiones y merges.
func handlePullRequest(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	action, _ := cargaGitHub["action"].(string)

	prMap, ok := cargaGitHub["pull_request"].(map[string]interface{})
	if !ok {
		return
	}

	title, _ := prMap["title"].(string)
	url, _ := prMap["html_url"].(string)
	userMap, _ := prMap["user"].(map[string]interface{})
	author, _ := userMap["login"].(string)

	// Convertir PR number a float64 (JSON genérico) y luego a int
	numberFloat, ok := prMap["number"].(float64)
	number := 0
	if ok {
		number = int(numberFloat)
	}

	switch action {
	case "opened", "reopened":
		// Extraer Assignees
		var assigneesList []string
		if assignees, ok := prMap["assignees"].([]interface{}); ok {
			for _, a := range assignees {
				assigneeMap := a.(map[string]interface{})
				login := assigneeMap["login"].(string)
				assigneesList = append(assigneesList, mapGitHubToDiscord(login))
			}
		}

		// Extraer Requested Reviewers
		var reviewersList []string
		if reviewers, ok := prMap["requested_reviewers"].([]interface{}); ok {
			for _, r := range reviewers {
				reviewerMap := r.(map[string]interface{})
				login := reviewerMap["login"].(string)
				reviewersList = append(reviewersList, mapGitHubToDiscord(login))
			}
		}

		// Extraer Labels
		var labelsList []string
		if labels, ok := prMap["labels"].([]interface{}); ok {
			for _, l := range labels {
				labelMap := l.(map[string]interface{})
				name := labelMap["name"].(string)
				labelsList = append(labelsList, "`"+name+"`")
			}
		}

		msg := fmt.Sprintf("🛠️ **Nuevo Pull Request** por `%s`\n**Título**: %s\n**Link**: %s", author, title, url)

		if len(labelsList) > 0 {
			msg += fmt.Sprintf("\n**Etiquetas**: %s", strings.Join(labelsList, ", "))
		}

		if len(assigneesList) > 0 {
			msg += fmt.Sprintf("\n**Asignados**: %s", strings.Join(assigneesList, " "))
		}

		if len(reviewersList) > 0 {
			msg += fmt.Sprintf("\n**Reviewers Solicitados**: %s", strings.Join(reviewersList, " "))
		}

		sesionDiscord.ChannelMessageSend(canalID, msg)

	case "closed":
		senderMap, _ := cargaGitHub["sender"].(map[string]interface{})
		senderName, _ := senderMap["login"].(string)

		merged, _ := prMap["merged"].(bool)
		if merged {
			msg := fmt.Sprintf("✅ **Pull Request Aceptado (Merged)** por `%s`\n**Título**: %s\n**Link**: %s", senderName, title, url)
			sesionDiscord.ChannelMessageSend(canalID, msg)
		} else {
			msg := fmt.Sprintf("❌ **Pull Request Rechazado/Cerrado** por `%s`\n**Título**: %s\n**Link**: %s", senderName, title, url)
			sesionDiscord.ChannelMessageSend(canalID, msg)
		}

	case "review_requested":

		if reqRev, ok := cargaGitHub["requested_reviewer"].(map[string]interface{}); ok {
			reviewer := reqRev["login"].(string)
			reviewerDiscord := mapGitHubToDiscord(reviewer)

			mutexDebounceReviews.Lock()

			// Solo añadir si no existe ya para evitar duplicados en la lista (por si acaso GitHub manda eventos de mas)
			exists := false
			for _, usuario := range usuariosReviews[number] {
				if usuario == reviewerDiscord {
					exists = true
					break
				}
			}
			if !exists {
				usuariosReviews[number] = append(usuariosReviews[number], reviewerDiscord)
			}

			if temporizador, existeTemporizador := temporizadoresReviews[number]; existeTemporizador {
				temporizador.Stop()
			}

			// Acumular solicitudes repetidas de revisión antes de enviar una notificación.
			temporizadoresReviews[number] = time.AfterFunc(configuracionActiva.RetrasoDebounceReviews, func() {
				mutexDebounceReviews.Lock()
				usuarios := usuariosReviews[number]
				delete(usuariosReviews, number)
				delete(temporizadoresReviews, number)
				mutexDebounceReviews.Unlock()

				if len(usuarios) > 0 {
					// Las URL entre <> evitan que Discord genere la caja gigante de previsualización
					mensaje := fmt.Sprintf("👀 **Review Solicitado** en el PR #%d\nSe ha solicitado la revisión de %s.\n**Link**: <%s>", number, strings.Join(usuarios, ", "), url)
					sesionDiscord.ChannelMessageSend(canalID, mensaje)
				}
			})
			mutexDebounceReviews.Unlock()
		}

	case "labeled":
		if labelMap, ok := cargaGitHub["label"].(map[string]interface{}); ok {
			labelName := labelMap["name"].(string)
			msg := fmt.Sprintf("🏷️ **Nueva Etiqueta** en el PR #%d\nSe añadió la etiqueta `%s`.\n**Link**: <%s>", number, labelName, url)
			sesionDiscord.ChannelMessageSend(canalID, msg)
		}
	}
}

// ===== Mapeo de usuarios =====

// mapGitHubToDiscord traduce usuarios de GitHub a identificadores de Discord o nombres.
func mapGitHubToDiscord(usuarioGitHub string) string {
	// Diccionario estático de miembros del equipo
	users := map[string]string{
		"VichoMatus": "<@!537347874875506698>",
		"danirc2024": "<@!1221186570455879702>",
		"RCarrascoO": "<@!410177503592972288>",
		"chelo132":   "<@!467880145877991432>",
		"EsbanV":     "<@!265591689211674624>", // ID de Esban
		"Web":        "<@!265591689211674624>", // Mapeo para su otro nick de GitHub
	}

	if discordPing, exists := users[usuarioGitHub]; exists {
		return discordPing
	}

	// Si no está mapeado, retornar solo el nombre de Github resaltado
	return "`@" + usuarioGitHub + "`"
}

// ===== Notificaciones de revisiones y comentarios =====

// handlePullRequestReview notifica cuando alguien aprueba, rechaza o comenta un PR.
func handlePullRequestReview(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	action, _ := cargaGitHub["action"].(string)
	if action != "submitted" {
		return
	}

	reviewMap, ok := cargaGitHub["review"].(map[string]interface{})
	if !ok {
		return
	}

	prMap, ok := cargaGitHub["pull_request"].(map[string]interface{})
	if !ok {
		return
	}

	state, _ := reviewMap["state"].(string) // "approved", "changes_requested", "commented"
	body, _ := reviewMap["body"].(string)
	url, _ := reviewMap["html_url"].(string)

	userMap, _ := reviewMap["user"].(map[string]interface{})
	reviewer, _ := userMap["login"].(string)
	reviewerDiscord := mapGitHubToDiscord(reviewer)

	numberFloat, _ := prMap["number"].(float64)
	number := int(numberFloat)

	// Solo notificar si hay comentario o si es una aprobación/rechazo
	if body == "" && state == "commented" {
		return
	}

	var statusEmoji string
	var statusText string

	switch state {
	case "approved":
		statusEmoji = "✅"
		statusText = "aprobó"
	case "changes_requested":
		statusEmoji = "❌"
		statusText = "solicitó cambios en"
	case "commented":
		statusEmoji = "💬"
		statusText = "comentó en"
	default:
		statusEmoji = "📝"
		statusText = "revisó"
	}

	msg := fmt.Sprintf("%s **Review**: %s %s el PR #%d\n", statusEmoji, reviewerDiscord, statusText, number)
	if body != "" {
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		msg += fmt.Sprintf("> *\"%s\"*\n", body)
	}
	msg += fmt.Sprintf("**Link**: <%s>", url)

	sesionDiscord.ChannelMessageSend(canalID, msg)
}

// handleIssueComment notifica cuando alguien comenta en el hilo de un PR.
func handleIssueComment(sesionDiscord *discordgo.Session, canalID string, cargaGitHub map[string]interface{}) {
	action, _ := cargaGitHub["action"].(string)
	if action != "created" {
		return
	}

	issueMap, ok := cargaGitHub["issue"].(map[string]interface{})
	if !ok {
		return
	}
	if _, isPR := issueMap["pull_request"]; !isPR {
		return // Ignorar comentarios en issues normales por ahora
	}

	commentMap, ok := cargaGitHub["comment"].(map[string]interface{})
	if !ok {
		return
	}

	body, _ := commentMap["body"].(string)
	url, _ := commentMap["html_url"].(string)

	userMap, _ := commentMap["user"].(map[string]interface{})
	commenter, _ := userMap["login"].(string)
	commenterDiscord := mapGitHubToDiscord(commenter)

	numberFloat, _ := issueMap["number"].(float64)
	number := int(numberFloat)

	if len(body) > 300 {
		body = body[:300] + "..."
	}

	msg := fmt.Sprintf("💬 **Nuevo Comentario** de %s en el PR #%d\n> *\"%s\"*\n**Link**: <%s>", commenterDiscord, number, body, url)
	sesionDiscord.ChannelMessageSend(canalID, msg)
}
