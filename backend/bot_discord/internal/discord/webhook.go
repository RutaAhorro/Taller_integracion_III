package discord

import (
	"encoding/json"
	"log"
	"net/http"
)

// ===== Servidor de webhooks =====

const (
	rutaWebhookGitHub     = "POST /webhooks/github"
	cabeceraEventoGitHub  = "X-GitHub-Event"
	cabeceraTipoContenido = "Content-Type"
	tipoContenidoJSON     = "application/json"
)

// ServidorWebhook crea el servidor HTTP que recibe los webhooks de GitHub. El
// límite para leer cabeceras evita conexiones lentas que retengan recursos
// indefinidamente.
func (manejador *Manejador) ServidorWebhook() *http.Server {
	enrutador := http.NewServeMux()
	enrutador.HandleFunc(rutaWebhookGitHub, manejador.atenderWebhookGitHub)
	return &http.Server{
		Addr:              ":" + manejador.configuracion.PuertoWebhook,
		Handler:           enrutador,
		ReadHeaderTimeout: manejador.configuracion.TimeoutCabeceraHTTP,
	}
}

// escribirJSON responde con un objeto JSON y el estado HTTP indicado.
func escribirJSON(respuesta http.ResponseWriter, estado int, cuerpo map[string]string) {
	respuesta.Header().Set(cabeceraTipoContenido, tipoContenidoJSON)
	respuesta.WriteHeader(estado)
	if errorObtenido := json.NewEncoder(respuesta).Encode(cuerpo); errorObtenido != nil {
		log.Printf("No se pudo escribir la respuesta del webhook: %v", errorObtenido)
	}
}

// atenderWebhookGitHub decodifica el evento de GitHub y lo deriva al
// notificador que corresponde según la cabecera X-GitHub-Event. Los eventos no
// soportados se aceptan con 200 para que GitHub no los marque como fallidos.
func (manejador *Manejador) atenderWebhookGitHub(respuesta http.ResponseWriter, peticion *http.Request) {
	evento := peticion.Header.Get(cabeceraEventoGitHub)
	var carga cargaGitHub
	if errorObtenido := json.NewDecoder(peticion.Body).Decode(&carga); errorObtenido != nil {
		escribirJSON(respuesta, http.StatusBadRequest, map[string]string{"error": "Payload inválido"})
		return
	}
	switch evento {
	case eventoPing:
		escribirJSON(respuesta, http.StatusOK, map[string]string{"status": "Ping recibido"})
		return
	case eventoPush:
		manejador.notificarPush(manejador.configuracion.CanalPushDiscordID, carga)
	case eventoPullRequest:
		manejador.notificarPullRequest(manejador.configuracion.CanalDiscordID, carga)
	case eventoPullRequestReview:
		manejador.notificarRevision(manejador.configuracion.CanalDiscordID, carga)
	case eventoIssueComment:
		manejador.notificarComentario(manejador.configuracion.CanalDiscordID, carga)
	default:
		log.Printf("Evento de GitHub ignorado: %s", evento)
	}
	escribirJSON(respuesta, http.StatusOK, map[string]string{"status": "Procesado"})
}
