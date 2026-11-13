package httpapi

import (
	"encoding/json"
	"net/http"
)

// =============================================================================
// Envoltorio de errores
// =============================================================================

// cuerpoError es el formato común de error de los servicios Go del proyecto:
//
//	{"estado": 404, "mensaje": "La lista no existe.", "detalle": "..."}
//
// "detalle" es opcional y nunca repite valores enviados por el cliente.
type cuerpoError struct {
	Estado  int    `json:"estado"`
	Mensaje string `json:"mensaje"`
	Detalle string `json:"detalle,omitempty"`
}

// =============================================================================
// Mensajes para el usuario
// =============================================================================

// Los textos son parte del contrato: el frontend puede mostrarlos tal cual.
const (
	mensajeSolicitudInvalida = "Solicitud inválida."
	mensajeListaNoExiste     = "La lista no existe."
	mensajeRutaNoExiste      = "La ruta o recurso solicitado no existe."
	mensajeMetodoNoPermitido = "El método HTTP utilizado no está permitido para este recurso."
	mensajeErrorInterno      = "Ocurrió un error interno en el servidor."
	mensajeSinCookie         = "Cookie de autenticación requerida."
	mensajeTokenInvalido     = "Token inválido o expirado."
	mensajeOrigenNoPermitido = "Origen no permitido."

	largoMaximoDetalle = 500 // recorta "detalle" para no devolver textos enormes
)

// =============================================================================
// Escritura de respuestas
// =============================================================================

// escribirJSON responde con el código de estado y el cuerpo serializado a JSON.
func escribirJSON(respuesta http.ResponseWriter, estado int, cuerpo any) {
	respuesta.Header().Set("Content-Type", "application/json")
	respuesta.WriteHeader(estado)
	_ = json.NewEncoder(respuesta).Encode(cuerpo)
}

// escribirError responde con el envoltorio común de errores.
func escribirError(respuesta http.ResponseWriter, estado int, mensaje, detalle string) {
	if len(detalle) > largoMaximoDetalle {
		detalle = detalle[:largoMaximoDetalle]
	}
	escribirJSON(respuesta, estado, cuerpoError{Estado: estado, Mensaje: mensaje, Detalle: detalle})
}
