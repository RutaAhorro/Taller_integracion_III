package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
)

// Handler del agregado en lote de artículos (SUP-318), según la sección 3.4 de
// docs/Contratos/CONTRATO_ARTICULOS_LISTA.md. Lo usan la confirmación de
// receta (SUP-329) y el carrito: todo o nada, con Idempotency-Key obligatorio.

// =============================================================================
// Idempotency-Key
// =============================================================================

// cabeceraIdempotencia es la cabecera que el Gateway ya reenvía en escrituras.
const cabeceraIdempotencia = "Idempotency-Key"

// claveIdempotenciaValida acepta un UUID u otro identificador opaco de hasta
// 128 caracteres seguros, el largo de la columna clave de la migración 12.
var claveIdempotenciaValida = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// =============================================================================
// Agregar en lote
// =============================================================================

// agregarLote: POST /api/v1/listas/{lista_id}/articulos/lote con
// {"articulos": [{"producto_id": "uuid", "cantidad": n}, ...]}.
//
// Orden de las comprobaciones, para que un reintento sea barato y seguro:
//  1. cabecera y cuerpo válidos (400);
//  2. clave ya aplicada con el mismo lote: responde el detalle sin volver a
//     consultar Catálogo ni sumar (si el producto desapareció entre tanto,
//     el reintento igual recibe el éxito que ya ocurrió);
//  3. Catálogo confirma que existen todos (503 si no responde, 409 si falta alguno);
//  4. el repositorio guarda todo o nada respetando los topes (409).
//
// Responde 200 con la lista completa (misma forma que GET /listas/{id}).
func (manejador *handlerListas) agregarLote(respuesta http.ResponseWriter, peticion *http.Request) {
	// --- Validar cabecera y cuerpo -------------------------------------------
	clave := peticion.Header.Get(cabeceraIdempotencia)
	if !claveIdempotenciaValida.MatchString(clave) {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "Idempotency-Key: obligatoria, hasta 128 caracteres [A-Za-z0-9._:-]")
		return
	}
	articulos, valido := manejador.leerLote(respuesta, peticion)
	if !valido {
		return
	}
	lote := listas.Lote{Clave: clave, Articulos: listas.NormalizarLote(articulos)}
	lote.Huella = listas.HuellaDe(lote.Articulos)

	// Sumar repetidos puede superar el tope aunque cada línea sea válida.
	var rechazados []listas.Rechazo
	for _, articulo := range lote.Articulos {
		if articulo.Cantidad > manejador.opciones.Limites.MaxCantidad {
			rechazados = append(rechazados, listas.Rechazo{ProductoID: articulo.ProductoID, Motivo: listas.MotivoCantidadMaxima})
		}
	}
	if len(rechazados) > 0 {
		escribirRechazosLote(respuesta, rechazados)
		return
	}

	usuarioID, idLista := sesion(peticion).UsuarioID, listaID(peticion)

	// --- Reintento con la misma clave ----------------------------------------
	huella, yaAplicado, errorObtenido := manejador.opciones.Repo.HuellaLote(peticion.Context(), usuarioID, idLista, clave)
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if yaAplicado {
		if huella != lote.Huella {
			manejador.escribirClaveReutilizada(respuesta)
			return
		}
		manejador.responderDetalle(respuesta, peticion)
		return
	}

	// --- Verificar en Catálogo: si no responde, no se guarda nada ------------
	productoIDs := make([]string, 0, len(lote.Articulos))
	for _, articulo := range lote.Articulos {
		productoIDs = append(productoIDs, articulo.ProductoID)
	}
	estados, errorObtenido := manejador.opciones.Catalogo.Consultar(peticion.Context(), productoIDs)
	if errorObtenido != nil {
		manejador.opciones.Log.Warn("catalogo_no_disponible", "ruta", peticion.URL.Path, "error", errorObtenido.Error())
		escribirError(respuesta, http.StatusServiceUnavailable, mensajeCatalogoNoDisponible, "")
		return
	}
	for _, productoID := range productoIDs {
		// "sin_stock" y "desconocido" se guardan, igual que en el alta de uno.
		if estados[productoID].Estado == listas.EstadoNoEncontrado {
			rechazados = append(rechazados, listas.Rechazo{ProductoID: productoID, Motivo: listas.MotivoNoEncontrado})
		}
	}
	if len(rechazados) > 0 {
		escribirRechazosLote(respuesta, rechazados)
		return
	}

	// --- Guardar todo o nada -------------------------------------------------
	resultado, errorObtenido := manejador.opciones.Repo.AgregarLote(peticion.Context(), usuarioID, idLista, lote, manejador.opciones.Limites)
	switch {
	case errors.Is(errorObtenido, listas.ErrMaxArticulos):
		escribirError(respuesta, http.StatusConflict, manejador.mensajeMaxArticulos(), "")
		return
	case errors.Is(errorObtenido, listas.ErrClaveReutilizada):
		manejador.escribirClaveReutilizada(respuesta)
		return
	case errorObtenido != nil:
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	case !resultado.ListaExiste:
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	case len(resultado.Rechazados) > 0:
		escribirRechazosLote(respuesta, resultado.Rechazados)
		return
	}
	manejador.responderDetalle(respuesta, peticion)
}

// =============================================================================
// Auxiliares
// =============================================================================

// leerLote decodifica y valida el cuerpo. Cada línea exige un UUID y una
// cantidad >= 1; el lote, entre 1 y el máximo de artículos por lista (más no
// cabría en ninguna lista). Responde 400 y devuelve false si algo falla.
func (manejador *handlerListas) leerLote(respuesta http.ResponseWriter, peticion *http.Request) ([]listas.ArticuloLote, bool) {
	var datos struct {
		Articulos []struct {
			ProductoID *string `json:"producto_id"`
			Cantidad   *int    `json:"cantidad"`
		} `json:"articulos"`
	}
	if !leerJSON(respuesta, peticion, &datos) {
		return nil, false
	}
	maximo := manejador.opciones.Limites.MaxArticulos
	if len(datos.Articulos) < 1 || len(datos.Articulos) > maximo {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida,
			fmt.Sprintf("articulos: debe tener entre 1 y %d elementos", maximo))
		return nil, false
	}

	articulos := make([]listas.ArticuloLote, 0, len(datos.Articulos))
	for indice, linea := range datos.Articulos {
		if linea.ProductoID == nil {
			escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, fmt.Sprintf("articulos[%d].producto_id: obligatorio", indice))
			return nil, false
		}
		productoID, errorObtenido := uuid.Parse(*linea.ProductoID)
		if errorObtenido != nil {
			escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, fmt.Sprintf("articulos[%d].producto_id: debe ser un UUID", indice))
			return nil, false
		}
		if linea.Cantidad == nil || *linea.Cantidad < 1 {
			escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, fmt.Sprintf("articulos[%d].cantidad: debe ser un entero mayor o igual a 1", indice))
			return nil, false
		}
		// uuid.String() deja el id en minúsculas: la misma clave de suma y de huella.
		articulos = append(articulos, listas.ArticuloLote{ProductoID: productoID.String(), Cantidad: *linea.Cantidad})
	}
	return articulos, true
}

// escribirClaveReutilizada responde 409 con codigo "idempotency_conflict": la
// clave ya se usó en esta lista con otro contenido. Es un conflicto con un
// envío anterior, no un cuerpo mal formado; el cliente debe generar una clave
// nueva por cada envío distinto.
func (manejador *handlerListas) escribirClaveReutilizada(respuesta http.ResponseWriter) {
	escribirJSON(respuesta, http.StatusConflict, cuerpoError{
		Estado: http.StatusConflict, Codigo: codigoConflictoIdempotente, Mensaje: mensajeClaveReutilizada})
}

// responderDetalle devuelve la lista actualizada con el estado de cada
// artículo en Catálogo, igual que GET /listas/{id}.
func (manejador *handlerListas) responderDetalle(respuesta http.ResponseWriter, peticion *http.Request) {
	detalle, errorObtenido := manejador.opciones.Repo.Obtener(peticion.Context(), sesion(peticion).UsuarioID, listaID(peticion))
	if errorObtenido != nil {
		manejador.fallo(respuesta, peticion, errorObtenido)
		return
	}
	if detalle == nil {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}
	manejador.completarEstados(peticion, detalle)
	escribirJSON(respuesta, http.StatusOK, detalle)
}
