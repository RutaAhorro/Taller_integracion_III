package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
)

// Handlers de los artículos de una lista (SUP-241), según
// docs/Contratos/CONTRATO_ARTICULOS_LISTA.md. El agregado en lote es SUP-318.

// =============================================================================
// Id del artículo (middleware por ruta)
// =============================================================================

// claveArticulo identifica el id de artículo guardado en el contexto.
type claveArticulo struct{}

// articuloID devuelve el id que conArticulo dejó en el contexto.
func articuloID(peticion *http.Request) int64 {
	return peticion.Context().Value(claveArticulo{}).(int64)
}

// conArticulo valida el {articulo_id} de la URL: un entero positivo. Si no lo
// es, responde 404 igual que un artículo inexistente o de otra lista.
func (manejador *handlerListas) conArticulo(idCrudo string, siguiente http.HandlerFunc) http.HandlerFunc {
	return func(respuesta http.ResponseWriter, peticion *http.Request) {
		idParseado, errorObtenido := strconv.ParseInt(idCrudo, 10, 64)
		if errorObtenido != nil || idParseado < 1 {
			escribirError(respuesta, http.StatusNotFound, mensajeArticuloNoExiste, "")
			return
		}
		contextoConArticulo := context.WithValue(peticion.Context(), claveArticulo{}, idParseado)
		siguiente(respuesta, peticion.WithContext(contextoConArticulo))
	}
}

// =============================================================================
// Validaciones
// =============================================================================

// validarCantidad responde y devuelve false si la cantidad no es válida:
// menor que 1 es un cuerpo inválido (400) y mayor que el tope, un conflicto (409).
func (manejador *handlerListas) validarCantidad(respuesta http.ResponseWriter, cantidad int) bool {
	if cantidad < 1 {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "cantidad: debe ser un entero mayor o igual a 1")
		return false
	}
	if cantidad > manejador.opciones.Limites.MaxCantidad {
		escribirError(respuesta, http.StatusConflict, manejador.mensajeMaxCantidad(), "")
		return false
	}
	return true
}

// mensajeMaxCantidad cita el tope configurado en vez de repetir un número.
func (manejador *handlerListas) mensajeMaxCantidad() string {
	return fmt.Sprintf("La cantidad máxima por artículo es %d.", manejador.opciones.Limites.MaxCantidad)
}

// mensajeMaxArticulos cita el tope configurado en vez de repetir un número.
func (manejador *handlerListas) mensajeMaxArticulos() string {
	return fmt.Sprintf("Alcanzaste el máximo de %d artículos en la lista.", manejador.opciones.Limites.MaxArticulos)
}

// =============================================================================
// Agregar, cambiar y quitar
// =============================================================================

// agregarArticulo: POST /api/v1/listas/{lista_id}/articulos con
// {"producto_id": "uuid", "cantidad": n}. Responde 201 si el producto es nuevo
// en la lista y 200 si ya estaba y se sumó la cantidad.
func (manejador *handlerListas) agregarArticulo(respuesta http.ResponseWriter, peticion *http.Request) {
	// --- Validar cuerpo ------------------------------------------------------
	var datos struct {
		ProductoID *string `json:"producto_id"`
		Cantidad   *int    `json:"cantidad"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if datos.ProductoID == nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "producto_id: obligatorio")
		return
	}
	productoID, errorObtenido := uuid.Parse(*datos.ProductoID)
	if errorObtenido != nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "producto_id: debe ser un UUID")
		return
	}
	if datos.Cantidad == nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "cantidad: obligatorio")
		return
	}
	if !manejador.validarCantidad(respuesta, *datos.Cantidad) {
		return
	}

	// --- La lista debe ser del usuario antes de consultar a Catálogo ---------
	// Así una lista ajena responde 404 aunque Catálogo falle o el producto no exista.
	esDelUsuario, errorObtenido := manejador.opciones.Repo.ListaEsDelUsuario(peticion.Context(),
		sesion(peticion).UsuarioID, listaID(peticion))
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if !esDelUsuario {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}

	// --- Verificar en Catálogo: si no responde, no se guarda nada ------------
	idProducto := productoID.String()
	estados, errorObtenido := manejador.opciones.Catalogo.Consultar(peticion.Context(), []string{idProducto})
	if errorObtenido != nil {
		manejador.opciones.Log.Warn("catalogo_no_disponible", "ruta", peticion.URL.Path, "error", errorObtenido.Error())
		escribirError(respuesta, http.StatusServiceUnavailable, mensajeCatalogoNoDisponible, "")
		return
	}
	disponibilidad := estados[idProducto]
	if disponibilidad.Estado == listas.EstadoNoEncontrado {
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeProductoNoExiste, "")
		return
	}
	// "sin_stock" y "desconocido" se guardan igual, con el estado a la vista:
	// el usuario puede planificar la compra aunque hoy no haya stock confirmado.

	// --- Guardar (sólo si la lista es del usuario) ---------------------------
	articulo, creado, errorObtenido := manejador.opciones.Repo.AgregarArticulo(peticion.Context(),
		sesion(peticion).UsuarioID, listaID(peticion), idProducto, *datos.Cantidad, manejador.opciones.Limites)
	switch {
	case errors.Is(errorObtenido, listas.ErrMaxArticulos):
		escribirError(respuesta, http.StatusConflict, manejador.mensajeMaxArticulos(), "")
		return
	case errors.Is(errorObtenido, listas.ErrMaxCantidad):
		escribirError(respuesta, http.StatusConflict, manejador.mensajeMaxCantidad(), "")
		return
	case errorObtenido != nil:
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	case articulo == nil:
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}

	estado := http.StatusOK
	if creado {
		estado = http.StatusCreated
	}
	escribirJSON(respuesta, estado, listas.ArticuloDetalle{
		Articulo:       *articulo,
		EstadoProducto: disponibilidad.Estado,
		Producto:       disponibilidad.Producto,
	})
}

// actualizarArticulo: PUT /api/v1/listas/{lista_id}/articulos/{articulo_id}
// con "cantidad" y/o "esta_comprado". No consulta a Catálogo: el producto no cambia.
func (manejador *handlerListas) actualizarArticulo(respuesta http.ResponseWriter, peticion *http.Request) {
	// --- Validar cuerpo ------------------------------------------------------
	var datos struct {
		Cantidad     *int  `json:"cantidad"`
		EstaComprado *bool `json:"esta_comprado"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if datos.Cantidad == nil && datos.EstaComprado == nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "cuerpo: indicar cantidad o esta_comprado")
		return
	}
	if datos.Cantidad != nil && !manejador.validarCantidad(respuesta, *datos.Cantidad) {
		return
	}

	// --- Guardar (sólo si el artículo está en esa lista del usuario) ---------
	articulo, errorObtenido := manejador.opciones.Repo.ActualizarArticulo(peticion.Context(),
		sesion(peticion).UsuarioID, listaID(peticion), articuloID(peticion), datos.Cantidad, datos.EstaComprado)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if articulo == nil {
		escribirError(respuesta, http.StatusNotFound, mensajeArticuloNoExiste, "")
		return
	}
	escribirJSON(respuesta, http.StatusOK, articulo)
}

// eliminarArticulo: DELETE /api/v1/listas/{lista_id}/articulos/{articulo_id}
// — responde 204 sin cuerpo.
func (manejador *handlerListas) eliminarArticulo(respuesta http.ResponseWriter, peticion *http.Request) {
	eliminado, errorObtenido := manejador.opciones.Repo.EliminarArticulo(peticion.Context(),
		sesion(peticion).UsuarioID, listaID(peticion), articuloID(peticion))
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if !eliminado {
		escribirError(respuesta, http.StatusNotFound, mensajeArticuloNoExiste, "")
		return
	}
	respuesta.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// Estado en Catálogo para el detalle de la lista
// =============================================================================

// completarEstados agrega a cada artículo su estado y datos en Catálogo con
// una sola consulta. Si Catálogo no responde, la lista se muestra igual con
// "desconocido": el usuario no pierde su lista por una falla de otro servicio.
// Un artículo sin producto_id (previo a la migración 11) queda "no_encontrado".
func (manejador *handlerListas) completarEstados(peticion *http.Request, detalle *listas.ListaDetalle) {
	productoIDs := make([]string, 0, len(detalle.Articulos))
	for _, articulo := range detalle.Articulos {
		if articulo.ProductoID != nil {
			productoIDs = append(productoIDs, *articulo.ProductoID)
		}
	}

	var estados map[string]listas.Disponibilidad
	catalogoRespondio := true
	if len(productoIDs) > 0 {
		var errorObtenido error
		estados, errorObtenido = manejador.opciones.Catalogo.Consultar(peticion.Context(), productoIDs)
		if errorObtenido != nil {
			manejador.opciones.Log.Warn("catalogo_no_disponible", "ruta", peticion.URL.Path, "error", errorObtenido.Error())
			catalogoRespondio = false
		}
	}

	for indice := range detalle.Articulos {
		articulo := &detalle.Articulos[indice]
		switch {
		case articulo.ProductoID == nil:
			articulo.EstadoProducto = listas.EstadoNoEncontrado
		case !catalogoRespondio:
			articulo.EstadoProducto = listas.EstadoDesconocido
		default:
			disponibilidad := estados[*articulo.ProductoID]
			articulo.EstadoProducto, articulo.Producto = disponibilidad.Estado, disponibilidad.Producto
		}
	}
}
