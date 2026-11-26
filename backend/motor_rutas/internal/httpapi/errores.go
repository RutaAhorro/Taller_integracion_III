package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// =============================================================================
// Envoltorio de errores
// =============================================================================

// cuerpoError es el formato común de error de los servicios Go del proyecto:
//
//	{"estado": 404, "mensaje": "La lista no existe.", "detalle": "..."}
//
// "detalle" es opcional y nunca repite valores enviados por el cliente. Suele
// ser un texto; el 409 del lote (SUP-318) lo usa como arreglo de rechazos.
// "codigo" es opcional: un identificador estable para que el cliente distinga
// un error sin comparar el mensaje (hoy sólo "idempotency_conflict").
type cuerpoError struct {
	Estado  int    `json:"estado"`
	Codigo  string `json:"codigo,omitempty"`
	Mensaje string `json:"mensaje"`
	Detalle any    `json:"detalle,omitempty"`
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

	// Artículos (SUP-241), con los textos del contrato de artículos.
	mensajeArticuloNoExiste     = "El artículo no existe."
	mensajeProductoNoExiste     = "El producto no existe en el catálogo."
	mensajeCatalogoNoDisponible = "No se pudo verificar el producto. Intenta de nuevo."

	// Lote (SUP-318).
	mensajeLoteRechazado       = "No se pudo agregar el lote."
	mensajeClaveReutilizada    = "La clave de idempotencia ya se usó con otro lote."
	codigoConflictoIdempotente = "idempotency_conflict"

	// Optimización de rutas (MVP / SUP-316).
	mensajeListaSinArticulos     = "La lista no tiene artículos por comprar."
	mensajeDemasiadosArticulos   = "La lista supera el máximo de artículos para calcular una ruta."
	mensajeSinSucursalesCercanas = "No hay supermercados cercanos para calcular una ruta."
	mensajeSinCoberturaRuta      = "No se encontró una forma de comprar la lista en las tiendas cercanas."
	mensajePreciosNoDisponibles  = "No se pudo consultar precios. Intenta de nuevo."

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
	cuerpo := cuerpoError{Estado: estado, Mensaje: mensaje}
	if detalle != "" {
		cuerpo.Detalle = detalle
	}
	escribirJSON(respuesta, estado, cuerpo)
}

// escribirRechazosLote responde el 409 del lote con "detalle" como arreglo
// [{producto_id, motivo}], para que el frontend marque cada producto sin
// interpretar texto. Los ids son los que el propio usuario envió.
func escribirRechazosLote(respuesta http.ResponseWriter, rechazados []listas.Rechazo) {
	escribirJSON(respuesta, http.StatusConflict, cuerpoError{
		Estado: http.StatusConflict, Mensaje: mensajeLoteRechazado, Detalle: rechazados})
}
