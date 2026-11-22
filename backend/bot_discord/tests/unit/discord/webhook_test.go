package discord_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// enviarWebhook entrega un evento de GitHub al servidor de webhooks del bot.
func enviarWebhook(prueba *testing.T, manejadorHTTP http.Handler, evento, cuerpo string) *httptest.ResponseRecorder {
	prueba.Helper()
	peticion := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(cuerpo))
	peticion.Header.Set("X-GitHub-Event", evento)
	respuesta := httptest.NewRecorder()
	manejadorHTTP.ServeHTTP(respuesta, peticion)
	return respuesta
}

// TestWebhookPullRequestMuestraEtiquetas verifica que un PR abierto incluya sus etiquetas en el canal principal.
func TestWebhookPullRequestMuestraEtiquetas(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	respuesta := enviarWebhook(prueba, manejador.ServidorWebhook().Handler, "pull_request", `{
		"action":"opened",
		"pull_request":{"title":"Titulo","html_url":"https://x/1","number":1,"user":{"login":"alguien"},
			"labels":[{"name":"bug"},{"name":"prioridad-alta"}]}}`)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado inesperado: %d", respuesta.Code)
	}
	cuerpos := registro.cuerpos("/channels/" + canalPrincipalID + "/messages")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "**Etiquetas**: `bug`, `prioridad-alta`") {
		prueba.Fatalf("no se mostraron las etiquetas del PR: %v", cuerpos)
	}
}

// TestWebhookPushUsaCanalPush verifica el canal de push y el límite de commits mostrados.
func TestWebhookPushUsaCanalPush(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	commits := strings.Repeat(`{"id":"0123456789","message":"m","url":"u"},`, 6)
	enviarWebhook(prueba, manejador.ServidorWebhook().Handler, "push",
		`{"pusher":{"name":"ana"},"ref":"refs/heads/develop","commits":[`+commits[:len(commits)-1]+`]}`)
	cuerpos := registro.cuerpos("/channels/" + canalPushID + "/messages")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], "Nuevo Push") || !strings.Contains(cuerpos[0], "y 1 commits más") {
		prueba.Fatalf("notificación de push inesperada: %v", cuerpos)
	}
}

// TestWebhookRespuestasHTTP cubre ping, evento ignorado, JSON inválido y método no permitido.
func TestWebhookRespuestasHTTP(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	manejadorHTTP := manejador.ServidorWebhook().Handler
	casos := []struct {
		nombre, evento, cuerpo string
		estado                 int
		contiene               string
	}{
		{"ping", "ping", `{}`, http.StatusOK, "Ping recibido"},
		{"evento ignorado", "star", `{}`, http.StatusOK, "Procesado"},
		{"JSON inválido", "push", `no-json`, http.StatusBadRequest, "Payload inválido"},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			respuesta := enviarWebhook(prueba, manejadorHTTP, caso.evento, caso.cuerpo)
			if respuesta.Code != caso.estado || !strings.Contains(respuesta.Body.String(), caso.contiene) {
				prueba.Fatalf("respuesta inesperada: %d %s", respuesta.Code, respuesta.Body.String())
			}
		})
	}
	peticion := httptest.NewRequest(http.MethodGet, "/webhooks/github", nil)
	respuesta := httptest.NewRecorder()
	manejadorHTTP.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusMethodNotAllowed {
		prueba.Fatalf("GET debe rechazarse: %d", respuesta.Code)
	}
	if len(registro.copiar()) != 0 {
		prueba.Fatal("ping, eventos ignorados y errores no deben publicar mensajes")
	}
}

// TestWebhookPayloadIncompletoNoEntraEnPanico: campos ausentes o de tipo erróneo producen textos vacíos, no un panic.
func TestWebhookPayloadIncompletoNoEntraEnPanico(prueba *testing.T) {
	manejador, _ := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	manejadorHTTP := manejador.ServidorWebhook().Handler
	for evento, cuerpo := range map[string]string{
		"push":                `{"pusher":{},"commits":[1,"x"]}`,
		"pull_request":        `{"action":"opened","pull_request":{"assignees":[null,{"login":5}]}}`,
		"pull_request_review": `{"action":"submitted","review":{"state":"approved"},"pull_request":{}}`,
		"issue_comment":       `{"action":"created","issue":{"pull_request":{}},"comment":{}}`,
	} {
		if respuesta := enviarWebhook(prueba, manejadorHTTP, evento, cuerpo); respuesta.Code != http.StatusOK {
			prueba.Fatalf("%s: estado %d", evento, respuesta.Code)
		}
	}
}

// TestWebhookComentarioLargoSeTrunca verifica el recorte a 300 caracteres de los comentarios.
func TestWebhookComentarioLargoSeTrunca(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	enviarWebhook(prueba, manejador.ServidorWebhook().Handler, "issue_comment",
		`{"action":"created","issue":{"number":3,"pull_request":{}},"comment":{"body":"`+strings.Repeat("ñ", 400)+`","html_url":"u","user":{"login":"x"}}}`)
	cuerpos := registro.cuerpos("/channels/" + canalPrincipalID + "/messages")
	if len(cuerpos) != 1 || !strings.Contains(cuerpos[0], strings.Repeat("ñ", 300)+"...") || strings.Contains(cuerpos[0], strings.Repeat("ñ", 301)) {
		prueba.Fatalf("el comentario no se truncó a 300 caracteres: %v", cuerpos)
	}
}

// TestWebhookAgrupaSolicitudesDeRevision verifica que varias solicitudes de revisión del mismo PR generen un solo aviso.
func TestWebhookAgrupaSolicitudesDeRevision(prueba *testing.T) {
	manejador, registro := nuevoManejadorPrueba(prueba, controlSiempre(http.StatusOK, `{}`))
	manejadorHTTP := manejador.ServidorWebhook().Handler
	for _, revisor := range []string{"VichoMatus", "RCarrascoO", "VichoMatus"} {
		enviarWebhook(prueba, manejadorHTTP, "pull_request", `{"action":"review_requested","requested_reviewer":{"login":"`+revisor+`"},"pull_request":{"number":9,"html_url":"u"}}`)
	}
	limite := time.Now().Add(2 * time.Second)
	for len(registro.copiar()) == 0 && time.Now().Before(limite) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	cuerpos := registro.cuerpos("/channels/" + canalPrincipalID + "/messages")
	if len(cuerpos) != 1 || strings.Count(cuerpos[0], "537347874875506698") != 1 || !strings.Contains(cuerpos[0], "410177503592972288") {
		prueba.Fatalf("se esperaba un solo aviso con ambos revisores sin duplicados: %v", cuerpos)
	}
}
