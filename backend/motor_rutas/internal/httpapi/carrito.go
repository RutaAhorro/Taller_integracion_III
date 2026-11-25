// API del borrador de la cuenta: sin IDs de usuario en URLs ni SQL cruzado.
package httpapi

import (
	"errors"
	"net/http"
	"sort"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
)

// ===== Respuestas y errores =====

// responderCarrito añade disponibilidad actual sin guardar precios en el borrador.
func (manejador *handlerListas) responderCarrito(respuesta http.ResponseWriter, peticion *http.Request, carrito listas.Carrito, errorObtenido error) {
	switch {
	case errors.Is(errorObtenido, listas.ErrVersionCarrito):
		escribirError(respuesta, http.StatusConflict, "El carrito cambió en otra pestaña o dispositivo. Revisa ambas versiones antes de continuar.", "version_carrito")
	case errors.Is(errorObtenido, listas.ErrDestinoCarrito):
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
	case errors.Is(errorObtenido, listas.ErrCarritoVacio):
		escribirError(respuesta, http.StatusConflict, "Agrega productos antes de guardar el carrito en una lista.", "")
	case errors.Is(errorObtenido, listas.ErrCatalogoNoDisponible):
		escribirError(respuesta, http.StatusServiceUnavailable, mensajeCatalogoNoDisponible, "")
	case errorObtenido != nil:
		manejador.fallo(respuesta, peticion, errorObtenido)
	default:
		// El GET y cada escritura incluyen los datos actuales cuando Catálogo responde.
		productoIDs := make([]string, 0, len(carrito.Articulos))
		for _, articulo := range carrito.Articulos {
			productoIDs = append(productoIDs, articulo.ProductoID)
		}
		disponibilidad := map[string]listas.Disponibilidad{}
		if len(productoIDs) > 0 {
			if consultada, errorCatalogo := manejador.opciones.Catalogo.Consultar(peticion.Context(), productoIDs); errorCatalogo == nil {
				disponibilidad = consultada
			}
		}
		respuesta.Header().Set("Cache-Control", "no-store")
		escribirJSON(respuesta, http.StatusOK, struct {
			listas.Carrito
			Disponibilidad map[string]listas.Disponibilidad `json:"disponibilidad"`
		}{carrito, disponibilidad})
	}
}

// repoCarrito mantiene compatibles los dobles de listas; producción configura ambos repositorios.
func (manejador *handlerListas) repoCarrito(respuesta http.ResponseWriter) listas.RepositorioCarrito {
	if manejador.opciones.Carritos == nil {
		escribirError(respuesta, http.StatusServiceUnavailable, "El carrito persistente no está habilitado.", "")
	}
	return manejador.opciones.Carritos
}

// ===== Lectura y edición =====

// conSesionCarrito impide que una pestaña antigua guarde bajo la cookie de una cuenta recién cambiada.
// La cabecera sólo comprueba coherencia: la identidad sigue saliendo exclusivamente del JWT.
func (manejador *handlerListas) conSesionCarrito(siguiente http.HandlerFunc) http.HandlerFunc {
	return manejador.conSesion(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Header.Get("X-Carrito-Cuenta") != sesion(peticion).UsuarioID {
			escribirError(respuesta, http.StatusUnauthorized, "La cuenta de esta pestaña no coincide con la sesión. Vuelve a iniciar sesión para recuperar tu carrito.", "")
			return
		}
		siguiente(respuesta, peticion)
	})
}

// obtenerCarrito recupera únicamente el borrador del usuario validado por cookie.
func (manejador *handlerListas) obtenerCarrito(respuesta http.ResponseWriter, peticion *http.Request) {
	repositorio := manejador.repoCarrito(respuesta)
	if repositorio == nil {
		return
	}
	carrito, errorObtenido := repositorio.ObtenerCarrito(peticion.Context(), sesion(peticion).UsuarioID)
	manejador.responderCarrito(respuesta, peticion, carrito, errorObtenido)
}

// guardarCarrito valida topes y UUIDs; nunca acepta un envío, un precio ni un dueño desde el cuerpo.
func (manejador *handlerListas) guardarCarrito(respuesta http.ResponseWriter, peticion *http.Request) {
	repositorio := manejador.repoCarrito(respuesta)
	if repositorio == nil {
		return
	}
	var datos struct {
		Version   *int64                   `json:"version"`
		Articulos []listas.ArticuloCarrito `json:"articulos"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if datos.Version == nil || *datos.Version < 0 || datos.Articulos == nil || len(datos.Articulos) > manejador.opciones.Limites.MaxArticulos {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "version y articulos: obligatorios y dentro de los límites")
		return
	}
	vistos := map[string]bool{}
	for indice, articulo := range datos.Articulos {
		productoID, errorObtenido := uuid.Parse(articulo.ProductoID)
		if errorObtenido != nil || articulo.Cantidad < 1 || articulo.Cantidad > manejador.opciones.Limites.MaxCantidad || vistos[productoID.String()] {
			escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "articulos: UUIDs distintos y cantidades entre 1 y el máximo de envases")
			return
		}
		vistos[productoID.String()] = true
		datos.Articulos[indice].ProductoID = productoID.String()
	}
	sort.Slice(datos.Articulos, func(izquierda, derecha int) bool {
		return datos.Articulos[izquierda].ProductoID < datos.Articulos[derecha].ProductoID
	})
	carrito, errorObtenido := repositorio.GuardarCarrito(peticion.Context(), sesion(peticion).UsuarioID, *datos.Version, datos.Articulos)
	manejador.responderCarrito(respuesta, peticion, carrito, errorObtenido)
}

// prepararCarrito hace durable la clave antes de modificar la lista de destino.
func (manejador *handlerListas) prepararCarrito(respuesta http.ResponseWriter, peticion *http.Request) {
	repositorio := manejador.repoCarrito(respuesta)
	if repositorio == nil {
		return
	}
	var datos struct {
		Version *int64 `json:"version"`
		ListaID string `json:"lista_id"`
		Clave   string `json:"clave"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	listaID, errorObtenido := uuid.Parse(datos.ListaID)
	if errorObtenido != nil || datos.Version == nil || *datos.Version < 0 || !claveIdempotenciaValida.MatchString(datos.Clave) {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "version, lista_id y clave: obligatorios y válidos")
		return
	}
	carrito, errorObtenido := repositorio.PrepararCarrito(peticion.Context(), sesion(peticion).UsuarioID, *datos.Version, listas.EnvioCarrito{ListaID: listaID.String(), Clave: datos.Clave})
	manejador.responderCarrito(respuesta, peticion, carrito, errorObtenido)
}

// confirmarCarrito no recibe cantidades: confirma la copia que el servidor congeló.
func (manejador *handlerListas) confirmarCarrito(respuesta http.ResponseWriter, peticion *http.Request) {
	repositorio := manejador.repoCarrito(respuesta)
	if repositorio == nil {
		return
	}
	var datos struct {
		Clave string `json:"clave"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return
	}
	if !claveIdempotenciaValida.MatchString(datos.Clave) {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "clave: obligatoria y válida")
		return
	}
	carrito, errorObtenido := repositorio.ConfirmarCarrito(peticion.Context(), sesion(peticion).UsuarioID, datos.Clave, manejador.opciones.Limites, manejador.opciones.Catalogo)
	manejador.responderCarrito(respuesta, peticion, carrito, errorObtenido)
}
