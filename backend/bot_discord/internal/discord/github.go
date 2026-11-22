package discord

import (
	"fmt"
	"strings"
	"time"
)

// ===== Constantes de GitHub =====

const (
	// Eventos del encabezado X-GitHub-Event que el bot atiende.
	eventoPing              = "ping"
	eventoPush              = "push"
	eventoPullRequest       = "pull_request"
	eventoPullRequestReview = "pull_request_review"
	eventoIssueComment      = "issue_comment"

	// Acciones de pull_request, pull_request_review e issue_comment.
	accionAbierto            = "opened"
	accionReabierto          = "reopened"
	accionCerrado            = "closed"
	accionRevisionSolicitada = "review_requested"
	accionEtiquetado         = "labeled"
	accionRevisionEnviada    = "submitted"
	accionComentarioCreado   = "created"

	// Estados de una revisión de PR.
	revisionAprobada           = "approved"
	revisionCambiosSolicitados = "changes_requested"
	revisionComentada          = "commented"

	prefijoReferenciaRama = "refs/heads/"
	// Máximos para no saturar el canal con pushes o comentarios largos.
	maximoCommitsMostrados     = 5
	maximoCaracteresComentario = 300
	largoIdentificadorCorto    = 7
)

// cargaGitHub es el JSON de un webhook. Se lee de forma genérica porque solo
// se usan unos pocos campos de payloads grandes y variables.
type cargaGitHub = map[string]interface{}

// ===== Lectura segura de payloads =====

// objeto devuelve el sub-objeto JSON de la clave, o nil si falta o no es un
// objeto. Leer de un mapa nil es seguro, así un payload incompleto produce
// textos vacíos en lugar de pánico.
func objeto(carga cargaGitHub, clave string) cargaGitHub {
	valor, _ := carga[clave].(map[string]interface{})
	return valor
}

// texto devuelve el valor de texto de la clave, o "" si falta o no es texto.
func texto(carga cargaGitHub, clave string) string {
	valor, _ := carga[clave].(string)
	return valor
}

// entero devuelve un número JSON (float64 en Go) como entero, o 0 si falta.
func entero(carga cargaGitHub, clave string) int {
	valor, _ := carga[clave].(float64)
	return int(valor)
}

// loginsDeLista devuelve el campo "login" de cada objeto de una lista JSON.
func loginsDeLista(carga cargaGitHub, clave string) []string {
	lista, _ := carga[clave].([]interface{})
	var logins []string
	for _, elemento := range lista {
		datos, _ := elemento.(map[string]interface{})
		logins = append(logins, texto(datos, "login"))
	}
	return logins
}

// truncarTexto recorta a un máximo de caracteres (no bytes, para no partir un
// carácter UTF-8) y agrega puntos suspensivos si hubo recorte.
func truncarTexto(valor string, maximo int) string {
	caracteres := []rune(valor)
	if len(caracteres) <= maximo {
		return valor
	}
	return string(caracteres[:maximo]) + "..."
}

// ===== Mapeo de usuarios =====

// usuariosDiscord traduce el login de GitHub de cada integrante a su mención
// de Discord. Es un dato del equipo, no una regla de negocio; vive aquí para
// que las notificaciones mencionen a las personas en lugar de mostrar el login.
var usuariosDiscord = map[string]string{
	"VichoMatus": "<@!537347874875506698>",
	"danirc2024": "<@!1221186570455879702>",
	"RCarrascoO": "<@!410177503592972288>",
	"chelo132":   "<@!467880145877991432>",
	"EsbanV":     "<@!265591689211674624>",
	"Web":        "<@!265591689211674624>", // Segundo nick de GitHub de EsbanV.
}

// mencionarUsuarioGitHub devuelve la mención de Discord del usuario o, si no
// está mapeado, su login resaltado.
func mencionarUsuarioGitHub(usuarioGitHub string) string {
	if mencion, existe := usuariosDiscord[usuarioGitHub]; existe {
		return mencion
	}
	return "`@" + usuarioGitHub + "`"
}

// mencionarLogins aplica mencionarUsuarioGitHub a cada login.
func mencionarLogins(logins []string) []string {
	menciones := make([]string, 0, len(logins))
	for _, login := range logins {
		menciones = append(menciones, mencionarUsuarioGitHub(login))
	}
	return menciones
}

// ===== Push =====

// notificarPush avisa de un push o de una fusión, con un resumen de los primeros commits.
func (manejador *Manejador) notificarPush(canalID string, carga cargaGitHub) {
	datosEmisor := objeto(carga, "pusher")
	if datosEmisor == nil {
		return
	}
	nombreEmisor := texto(datosEmisor, "name")
	rama := strings.Replace(texto(carga, "ref"), prefijoReferenciaRama, "", 1)

	mensajeCommitPrincipal := texto(objeto(carga, "head_commit"), "message")
	esFusion := strings.HasPrefix(mensajeCommitPrincipal, "Merge pull request") || strings.HasPrefix(mensajeCommitPrincipal, "Merge branch")

	var detalleCommits string
	if confirmaciones, _ := carga["commits"].([]interface{}); len(confirmaciones) > 0 {
		detalleCommits = "\n**Commits:**"
		limite := min(len(confirmaciones), maximoCommitsMostrados)
		for indice := 0; indice < limite; indice++ {
			commitActual, _ := confirmaciones[indice].(map[string]interface{})
			primeraLinea := strings.Split(texto(commitActual, "message"), "\n")[0]
			identificadorCorto := texto(commitActual, "id")
			if len(identificadorCorto) > largoIdentificadorCorto {
				identificadorCorto = identificadorCorto[:largoIdentificadorCorto]
			}
			// Formato: - [`shortID`](<url>) Mensaje; los <> evitan la previsualización.
			detalleCommits += fmt.Sprintf("\n- [`%s`](<%s>) %s", identificadorCorto, texto(commitActual, "url"), primeraLinea)
		}
		if len(confirmaciones) > maximoCommitsMostrados {
			detalleCommits += fmt.Sprintf("\n*... y %d commits más.*", len(confirmaciones)-maximoCommitsMostrados)
		}
	}

	titulo := "🚀 **Nuevo Push** de `%s` en la rama `%s`."
	if esFusion {
		titulo = "🔀 **Merge Realizado** por `%s` en la rama `%s`."
	}
	manejador.enviarMensaje(canalID, fmt.Sprintf(titulo, nombreEmisor, rama)+detalleCommits)
}

// ===== Pull requests =====

// notificarPullRequest atiende los eventos de pull_request: apertura, cierre,
// solicitud de revisión y etiquetado. Otras acciones se ignoran.
func (manejador *Manejador) notificarPullRequest(canalID string, carga cargaGitHub) {
	datosPR := objeto(carga, "pull_request")
	if datosPR == nil {
		return
	}
	switch texto(carga, "action") {
	case accionAbierto, accionReabierto:
		manejador.notificarPullRequestAbierto(canalID, datosPR)
	case accionCerrado:
		manejador.notificarPullRequestCerrado(canalID, carga, datosPR)
	case accionRevisionSolicitada:
		manejador.notificarRevisionSolicitada(canalID, carga, datosPR)
	case accionEtiquetado:
		if datosEtiqueta := objeto(carga, "label"); datosEtiqueta != nil {
			manejador.enviarMensaje(canalID, fmt.Sprintf("🏷️ **Nueva Etiqueta** en el PR #%d\nSe añadió la etiqueta `%s`.\n**Link**: <%s>",
				entero(datosPR, "number"), texto(datosEtiqueta, "name"), texto(datosPR, "html_url")))
		}
	}
}

// notificarPullRequestAbierto anuncia un PR nuevo con sus etiquetas, asignados y revisores.
func (manejador *Manejador) notificarPullRequestAbierto(canalID string, datosPR cargaGitHub) {
	mensaje := fmt.Sprintf("🛠️ **Nuevo Pull Request** por `%s`\n**Título**: %s\n**Link**: %s",
		texto(objeto(datosPR, "user"), "login"), texto(datosPR, "title"), texto(datosPR, "html_url"))
	mensaje += formatearEtiquetasPullRequest(datosPR)
	if asignados := mencionarLogins(loginsDeLista(datosPR, "assignees")); len(asignados) > 0 {
		mensaje += fmt.Sprintf("\n**Asignados**: %s", strings.Join(asignados, " "))
	}
	if revisores := mencionarLogins(loginsDeLista(datosPR, "requested_reviewers")); len(revisores) > 0 {
		mensaje += fmt.Sprintf("\n**Reviewers Solicitados**: %s", strings.Join(revisores, " "))
	}
	manejador.enviarMensaje(canalID, mensaje)
}

// notificarPullRequestCerrado distingue un PR fusionado de uno cerrado sin fusionar.
func (manejador *Manejador) notificarPullRequestCerrado(canalID string, carga, datosPR cargaGitHub) {
	remitente := texto(objeto(carga, "sender"), "login")
	titulo, direccion := texto(datosPR, "title"), texto(datosPR, "html_url")
	if fusionado, _ := datosPR["merged"].(bool); fusionado {
		manejador.enviarMensaje(canalID, fmt.Sprintf("✅ **Pull Request Aceptado (Merged)** por `%s`\n**Título**: %s\n**Link**: %s", remitente, titulo, direccion))
		return
	}
	manejador.enviarMensaje(canalID, fmt.Sprintf("❌ **Pull Request Rechazado/Cerrado** por `%s`\n**Título**: %s\n**Link**: %s", remitente, titulo, direccion))
}

// notificarRevisionSolicitada agrupa las solicitudes de revisión de un mismo PR.
// GitHub envía un evento por revisor; se espera un retraso configurable tras el
// último y se publica un solo aviso con todos los revisores, para no repetir
// el mensaje en el canal.
func (manejador *Manejador) notificarRevisionSolicitada(canalID string, carga, datosPR cargaGitHub) {
	revisorSolicitado := objeto(carga, "requested_reviewer")
	if revisorSolicitado == nil {
		return
	}
	numero := entero(datosPR, "number")
	direccion := texto(datosPR, "html_url")
	mencion := mencionarUsuarioGitHub(texto(revisorSolicitado, "login"))

	manejador.mutexRevisiones.Lock()
	defer manejador.mutexRevisiones.Unlock()

	yaPendiente := false
	for _, pendiente := range manejador.revisoresPendientesPR[numero] {
		if pendiente == mencion {
			yaPendiente = true
			break
		}
	}
	if !yaPendiente {
		manejador.revisoresPendientesPR[numero] = append(manejador.revisoresPendientesPR[numero], mencion)
	}
	if temporizador, existe := manejador.temporizadoresPR[numero]; existe {
		temporizador.Stop()
	}
	manejador.temporizadoresPR[numero] = time.AfterFunc(manejador.configuracion.RetrasoDebounceReviews, func() {
		manejador.mutexRevisiones.Lock()
		revisores := manejador.revisoresPendientesPR[numero]
		delete(manejador.revisoresPendientesPR, numero)
		delete(manejador.temporizadoresPR, numero)
		manejador.mutexRevisiones.Unlock()

		if len(revisores) > 0 {
			// Los <> en la URL evitan la caja gigante de previsualización de Discord.
			manejador.enviarMensaje(canalID, fmt.Sprintf("👀 **Review Solicitado** en el PR #%d\nSe ha solicitado la revisión de %s.\n**Link**: <%s>", numero, strings.Join(revisores, ", "), direccion))
		}
	})
}

// formatearEtiquetasPullRequest crea la línea opcional de etiquetas de un PR.
func formatearEtiquetasPullRequest(datosPR cargaGitHub) string {
	etiquetasCrudas, _ := datosPR["labels"].([]interface{})
	var etiquetas []string
	for _, etiquetaActual := range etiquetasCrudas {
		datosEtiqueta, _ := etiquetaActual.(map[string]interface{})
		if nombre := texto(datosEtiqueta, "name"); nombre != "" {
			etiquetas = append(etiquetas, "`"+nombre+"`")
		}
	}
	if len(etiquetas) == 0 {
		return ""
	}
	return fmt.Sprintf("\n**Etiquetas**: %s", strings.Join(etiquetas, ", "))
}

// ===== Revisiones y comentarios =====

// notificarRevision avisa cuando alguien aprueba, pide cambios o comenta un PR.
// Una revisión "commented" sin texto no se notifica porque no aporta información.
func (manejador *Manejador) notificarRevision(canalID string, carga cargaGitHub) {
	if texto(carga, "action") != accionRevisionEnviada {
		return
	}
	datosRevision := objeto(carga, "review")
	datosPR := objeto(carga, "pull_request")
	if datosRevision == nil || datosPR == nil {
		return
	}
	estado := texto(datosRevision, "state")
	cuerpo := texto(datosRevision, "body")
	if cuerpo == "" && estado == revisionComentada {
		return
	}

	var emojiEstado, textoEstado string
	switch estado {
	case revisionAprobada:
		emojiEstado, textoEstado = "✅", "aprobó"
	case revisionCambiosSolicitados:
		emojiEstado, textoEstado = "❌", "solicitó cambios en"
	case revisionComentada:
		emojiEstado, textoEstado = "💬", "comentó en"
	default:
		emojiEstado, textoEstado = "📝", "revisó"
	}
	mensaje := fmt.Sprintf("%s **Review**: %s %s el PR #%d\n", emojiEstado,
		mencionarUsuarioGitHub(texto(objeto(datosRevision, "user"), "login")), textoEstado, entero(datosPR, "number"))
	if cuerpo != "" {
		mensaje += fmt.Sprintf("> *\"%s\"*\n", truncarTexto(cuerpo, maximoCaracteresComentario))
	}
	mensaje += fmt.Sprintf("**Link**: <%s>", texto(datosRevision, "html_url"))
	manejador.enviarMensaje(canalID, mensaje)
}

// notificarComentario avisa de un comentario nuevo en el hilo de un PR. Los
// comentarios de issues normales se ignoran porque el bot solo sigue PRs.
func (manejador *Manejador) notificarComentario(canalID string, carga cargaGitHub) {
	if texto(carga, "action") != accionComentarioCreado {
		return
	}
	datosIncidencia := objeto(carga, "issue")
	if datosIncidencia == nil {
		return
	}
	if _, esPullRequest := datosIncidencia["pull_request"]; !esPullRequest {
		return
	}
	datosComentario := objeto(carga, "comment")
	if datosComentario == nil {
		return
	}
	mensaje := fmt.Sprintf("💬 **Nuevo Comentario** de %s en el PR #%d\n> *\"%s\"*\n**Link**: <%s>",
		mencionarUsuarioGitHub(texto(objeto(datosComentario, "user"), "login")), entero(datosIncidencia, "number"),
		truncarTexto(texto(datosComentario, "body"), maximoCaracteresComentario), texto(datosComentario, "html_url"))
	manejador.enviarMensaje(canalID, mensaje)
}
