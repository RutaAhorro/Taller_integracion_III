package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/auth"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
)

// =============================================================================
// Handler de listas
// =============================================================================

// largoMaximoCuerpo limita el JSON recibido a 64 KB.
const largoMaximoCuerpo = 64 << 10

// handlerListas agrupa los handlers de /api/v1/listas y las sondas.
type handlerListas struct{ opciones Opciones }

// =============================================================================
// Sesión e id de la lista (middlewares por ruta)
// =============================================================================

// claveSesion y claveID identifican valores guardados en el contexto de la petición.
type (
	claveSesion struct{}
	claveID     struct{}
)

// sesion devuelve la sesión que conSesion dejó en el contexto.
func sesion(peticion *http.Request) auth.Sesion {
	return peticion.Context().Value(claveSesion{}).(auth.Sesion)
}

// listaID devuelve el id de lista (UUID normalizado) que conID dejó en el contexto.
func listaID(peticion *http.Request) string {
	return peticion.Context().Value(claveID{}).(string)
}

// conSesion exige una cookie `jwt` válida. El usuario sale SOLO de esa cookie:
// se ignoran cabeceras como X-User-ID o Authorization y cualquier campo del cuerpo.
func (manejador *handlerListas) conSesion(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(respuesta http.ResponseWriter, peticion *http.Request) {
		token := ""
		if cookie, errorObtenido := peticion.Cookie("jwt"); errorObtenido == nil {
			token = strings.TrimSpace(cookie.Value)
		}
		if token == "" {
			escribirError(respuesta, http.StatusUnauthorized, mensajeSinCookie, "")
			return
		}

		sesionUsuario, errorObtenido := auth.Validar(token, manejador.opciones.JWTSecret)
		if errorObtenido != nil {
			escribirError(respuesta, http.StatusUnauthorized, mensajeTokenInvalido, "")
			return
		}

		contextoConSesion := context.WithValue(peticion.Context(), claveSesion{}, sesionUsuario)
		siguiente(respuesta, peticion.WithContext(contextoConSesion))
	}
}

// conID valida el {lista_id} de la URL. Un id mal formado responde igual que
// una lista inexistente o ajena (404), para no revelar qué listas existen.
func (manejador *handlerListas) conID(idCrudo string, siguiente http.HandlerFunc) http.HandlerFunc {
	return func(respuesta http.ResponseWriter, peticion *http.Request) {
		idParseado, errorObtenido := uuid.Parse(idCrudo)
		if errorObtenido != nil {
			escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
			return
		}
		contextoConID := context.WithValue(peticion.Context(), claveID{}, idParseado.String())
		siguiente(respuesta, peticion.WithContext(contextoConID))
	}
}

// =============================================================================
// Lectura del cuerpo JSON
// =============================================================================

// leerJSON decodifica el cuerpo en destino. Exige:
//   - como máximo largoMaximoCuerpo bytes;
//   - UTF-8 válido;
//   - un único objeto JSON, sin campos desconocidos ni contenido sobrante.
//
// Si falla, ya respondió 400 y devuelve false. Los mensajes nombran el campo,
// nunca el valor recibido.
func leerJSON(respuesta http.ResponseWriter, peticion *http.Request, destino any) bool {
	cuerpoCrudo, errorObtenido := io.ReadAll(http.MaxBytesReader(respuesta, peticion.Body, largoMaximoCuerpo))
	if errorObtenido != nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "El cuerpo excede el tamaño permitido.")
		return false
	}
	if !utf8.Valid(cuerpoCrudo) {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "El cuerpo no es JSON válido.")
		return false
	}

	decodificador := json.NewDecoder(bytes.NewReader(cuerpoCrudo))
	decodificador.DisallowUnknownFields()
	if errorObtenido := decodificador.Decode(destino); errorObtenido != nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, detalleErrorJSON(errorObtenido))
		return false
	}
	// Tras el objeto solo puede quedar el fin del cuerpo. More() no sirve aquí:
	// devuelve false ante un '}' o ']' sobrante, así que {"a":1}} se aceptaría.
	// Una segunda decodificación debe devolver exactamente io.EOF.
	var sobrante json.RawMessage
	if errorObtenido := decodificador.Decode(&sobrante); !errors.Is(errorObtenido, io.EOF) {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "El cuerpo no es JSON válido.")
		return false
	}
	return true
}

// detalleErrorJSON traduce el error del decodificador a un texto para el cliente.
func detalleErrorJSON(errorObtenido error) string {
	var errorTipo *json.UnmarshalTypeError
	switch {
	case errors.As(errorObtenido, &errorTipo) && errorTipo.Field != "":
		return errorTipo.Field + ": tipo inválido"
	case strings.HasPrefix(errorObtenido.Error(), "json: unknown field "):
		return "cuerpo: campo no permitido"
	default:
		return "El cuerpo no es JSON válido."
	}
}

// fallo registra un error inesperado (solo su tipo, sin datos) y responde 500.
func (manejador *handlerListas) fallo(respuesta http.ResponseWriter, peticion *http.Request, errorObtenido error) {
	manejador.opciones.Log.Error("error_interno", "ruta", peticion.URL.Path, "tipo", fmt.Sprintf("%T", errorObtenido))
	escribirError(respuesta, http.StatusInternalServerError, mensajeErrorInterno, "")
}

// =============================================================================
// Sondas de salud
// =============================================================================

// live: GET /_rutas/live — el proceso está vivo (no revisa la base).
func (manejador *handlerListas) live(respuesta http.ResponseWriter, _ *http.Request) {
	escribirJSON(respuesta, http.StatusOK, map[string]string{"status": "ok", "service": "motor_rutas"})
}

// ready: GET /_rutas/ready y /health — listo para recibir tráfico si la base responde.
func (manejador *handlerListas) ready(respuesta http.ResponseWriter, peticion *http.Request) {
	baseSana := manejador.opciones.Ping == nil || manejador.opciones.Ping(peticion.Context())
	estado, codigo := "ok", http.StatusOK
	if !baseSana {
		estado, codigo = "error", http.StatusServiceUnavailable
	}
	escribirJSON(respuesta, codigo, map[string]any{
		"status":  estado,
		"service": "motor_rutas",
		"checks":  map[string]string{"database": estado},
	})
}

// =============================================================================
// CRUD de listas
// =============================================================================

// listar: GET /api/v1/listas — listas del usuario, de la más reciente a la más antigua.
func (manejador *handlerListas) listar(respuesta http.ResponseWriter, peticion *http.Request) {
	usuarioID := sesion(peticion).UsuarioID
	listasUsuario, errorObtenido := manejador.opciones.Repo.Listar(peticion.Context(), usuarioID, manejador.opciones.MaxListas)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	escribirJSON(respuesta, http.StatusOK, map[string][]listas.Lista{"listas": listasUsuario})
}

// crear: POST /api/v1/listas con {"nombre": "..."} — responde 201 con la lista creada.
func (manejador *handlerListas) crear(respuesta http.ResponseWriter, peticion *http.Request) {
	// --- Validar cuerpo ------------------------------------------------------
	var datos struct {
		Nombre *string `json:"nombre"` // puntero para distinguir "ausente" de "vacío"
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if datos.Nombre == nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "nombre: obligatorio")
		return
	}
	nombre, errorObtenido := listas.NormalizarNombre(*datos.Nombre)
	if errorObtenido != nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "nombre: "+errorObtenido.Error())
		return
	}

	// --- Respetar el tope de listas por usuario ------------------------------
	usuarioID := sesion(peticion).UsuarioID
	totalListas, errorObtenido := manejador.opciones.Repo.Contar(peticion.Context(), usuarioID)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if totalListas >= manejador.opciones.MaxListas {
		mensaje := fmt.Sprintf("Alcanzaste el máximo de %d listas.", manejador.opciones.MaxListas)
		escribirError(respuesta, http.StatusConflict, mensaje, "")
		return
	}

	// --- Guardar -------------------------------------------------------------
	lista, errorObtenido := manejador.opciones.Repo.Crear(peticion.Context(), usuarioID, nombre)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	escribirJSON(respuesta, http.StatusCreated, lista)
}

// obtener: GET /api/v1/listas/{lista_id} — la lista con sus artículos.
func (manejador *handlerListas) obtener(respuesta http.ResponseWriter, peticion *http.Request) {
	detalle, errorObtenido := manejador.opciones.Repo.Obtener(peticion.Context(), sesion(peticion).UsuarioID, listaID(peticion))
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if detalle == nil {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}
	escribirJSON(respuesta, http.StatusOK, detalle)
}

// actualizar: PUT /api/v1/listas/{lista_id} con "nombre" y/o "estado".
func (manejador *handlerListas) actualizar(respuesta http.ResponseWriter, peticion *http.Request) {
	// --- Validar cuerpo ------------------------------------------------------
	var datos struct {
		Nombre *string `json:"nombre"`
		Estado *string `json:"estado"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if datos.Nombre == nil && datos.Estado == nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "cuerpo: indicar nombre o estado")
		return
	}
	if datos.Nombre != nil {
		nombre, errorObtenido := listas.NormalizarNombre(*datos.Nombre)
		if errorObtenido != nil {
			escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "nombre: "+errorObtenido.Error())
			return
		}
		datos.Nombre = &nombre
	}
	if datos.Estado != nil && !listas.Estados[*datos.Estado] {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "estado: debe ser activa, completada o archivada")
		return
	}

	// --- Guardar (solo si la lista es del usuario) ---------------------------
	lista, errorObtenido := manejador.opciones.Repo.Actualizar(peticion.Context(), sesion(peticion).UsuarioID,
		listaID(peticion), datos.Nombre, datos.Estado)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if lista == nil {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}
	escribirJSON(respuesta, http.StatusOK, lista)
}

// eliminar: DELETE /api/v1/listas/{lista_id} — responde 204 sin cuerpo.
func (manejador *handlerListas) eliminar(respuesta http.ResponseWriter, peticion *http.Request) {
	eliminada, errorObtenido := manejador.opciones.Repo.Eliminar(peticion.Context(), sesion(peticion).UsuarioID, listaID(peticion))
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if !eliminada {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}
	respuesta.WriteHeader(http.StatusNoContent)
}
