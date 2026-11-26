// Handler de POST /api/v1/rutas/optimizar (MVP).
//
// Recibe una lista del usuario autenticado y un origen opcional (por defecto
// el centro de Temuco), consulta precios comparables en Catálogo con token
// interno, filtra ofertas sin precio o sin stock y delega el cálculo puro a
// internal/optimizacion sin costo de combustible.
package httpapi

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
	"github.com/google/uuid"
)

// =============================================================================
// Constantes y configuración del algoritmo
// =============================================================================

// Coordenadas por defecto cuando el cliente omite "origen": centro de Temuco
// (Plaza Aníbal Pinto), alineado con la referencia geográfica del proyecto.
const (
	latitudTemucoPorDefecto  = -38.7359
	longitudTemucoPorDefecto = -72.5904

	latitudMaximaGeo  = 90.0
	longitudMaximaGeo = 180.0
)

// OpcionesOptimizacion reúne los parámetros del cálculo de rutas inyectados
// desde internal/config. Si queda en valor cero (en pruebas que no la pasan),
// se aplican los valores por defecto del MVP.
type OpcionesOptimizacion struct {
	RadioKm         float64
	MaxSucursales   int
	MaxParadas      int
	MaxArticulos    int
	FactorDistancia float64
	UmbralAhorro    int
}

// opcionesOptimizacionPorDefecto son los valores iniciales del MVP.
var opcionesOptimizacionPorDefecto = OpcionesOptimizacion{
	RadioKm:         10,
	MaxSucursales:   10,
	MaxParadas:      3,
	MaxArticulos:    30,
	FactorDistancia: 1.3,
	UmbralAhorro:    500,
}

// resolverOpcionesOptimizacion completa los campos sin configurar sin pisar un
// UmbralAhorro explícitamente puesto en 0 cuando el resto viene informado.
func resolverOpcionesOptimizacion(opciones OpcionesOptimizacion) OpcionesOptimizacion {
	if opciones.MaxParadas == 0 && opciones.RadioKm == 0 && opciones.MaxSucursales == 0 &&
		opciones.MaxArticulos == 0 && opciones.FactorDistancia == 0 && opciones.UmbralAhorro == 0 {
		return opcionesOptimizacionPorDefecto
	}
	if opciones.RadioKm <= 0 {
		opciones.RadioKm = opcionesOptimizacionPorDefecto.RadioKm
	}
	if opciones.MaxSucursales < 1 {
		opciones.MaxSucursales = opcionesOptimizacionPorDefecto.MaxSucursales
	}
	if opciones.MaxParadas < 1 {
		opciones.MaxParadas = opcionesOptimizacionPorDefecto.MaxParadas
	}
	if opciones.MaxArticulos < 1 {
		opciones.MaxArticulos = opcionesOptimizacionPorDefecto.MaxArticulos
	}
	if opciones.FactorDistancia <= 0 {
		opciones.FactorDistancia = opcionesOptimizacionPorDefecto.FactorDistancia
	}
	return opciones
}

// =============================================================================
// Modelos de solicitud y respuesta HTTP
// =============================================================================

// solicitudOrigen permite distinguir "no enviado" de cero (0 es válido).
type solicitudOrigen struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// solicitudOptimizar es el cuerpo aceptado por POST /api/v1/rutas/optimizar.
// En el MVP se recibe únicamente lista_id y origen opcional.
type solicitudOptimizar struct {
	ListaID string           `json:"lista_id"`
	Origen  *solicitudOrigen `json:"origen"`
}

// origenRespuesta informa las coordenadas usadas y si se aplicó el valor por defecto.
type origenRespuesta struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	PorDefecto bool    `json:"por_defecto"`
}

// parametrosRespuesta expone los parámetros efectivos del cálculo MVP.
type parametrosRespuesta struct {
	FactorDistancia float64 `json:"factor_distancia"`
	MaxParadas      int     `json:"max_paradas"`
	UmbralAhorro    int     `json:"umbral_ahorro"`
	RadioKm         float64 `json:"radio_km"`
}

// respuestaOptimizar es la salida 200 OK del endpoint (contrato MVP).
type respuestaOptimizar struct {
	Estimado            bool                        `json:"estimado"`
	Origen              origenRespuesta             `json:"origen"`
	Parametros          parametrosRespuesta         `json:"parametros"`
	Ruta                optimizacion.Ruta           `json:"ruta"`
	BaseUnaTienda       *optimizacion.BaseUnaTienda `json:"base_una_tienda"`
	Ahorro              *int                        `json:"ahorro"`
	NoCubiertos         []optimizacion.NoCubierto   `json:"no_cubiertos"`
	CandidatasEvaluadas int                         `json:"candidatas_evaluadas"`
}

// =============================================================================
// Handler principal
// =============================================================================

// optimizarRuta calcula la combinación más barata de sucursales y el orden de
// visita por Haversine para los artículos pendientes de una lista del usuario.
func (manejador *handlerListas) optimizarRuta(respuesta http.ResponseWriter, peticion *http.Request) {
	var solicitud solicitudOptimizar
	if !leerJSON(respuesta, peticion, &solicitud) {
		return
	}

	listaIDParseado, errorUUID := uuid.Parse(strings.TrimSpace(solicitud.ListaID))
	if errorUUID != nil {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, "lista_id: debe ser un UUID válido")
		return
	}

	origenUsado, detalleOrigen := resolverOrigen(solicitud.Origen)
	if detalleOrigen != "" {
		escribirError(respuesta, http.StatusBadRequest, mensajeSolicitudInvalida, detalleOrigen)
		return
	}

	// --- Lectura de la lista del usuario (filtra por dueño) ------------------
	sesionUsuario := sesion(peticion)
	detalleLista, errorRepo := manejador.opciones.Repo.Obtener(peticion.Context(), sesionUsuario.UsuarioID, listaIDParseado.String())
	if errorRepo != nil {
		manejador.fallo(respuesta, peticion, errorRepo)
		return
	}
	if detalleLista == nil {
		escribirError(respuesta, http.StatusNotFound, mensajeListaNoExiste, "")
		return
	}

	// --- Artículos pendientes de compra --------------------------------------
	opcionesOpt := resolverOpcionesOptimizacion(manejador.opciones.Optimizacion)
	articulosPendientes, productoIDs := extraerArticulosPendientes(detalleLista.Articulos)
	if len(articulosPendientes) == 0 {
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeListaSinArticulos, "")
		return
	}
	if len(articulosPendientes) > opcionesOpt.MaxArticulos {
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeDemasiadosArticulos, "")
		return
	}

	// --- Consulta de precios comparables a Catálogo --------------------------
	if manejador.opciones.Precios == nil {
		escribirError(respuesta, http.StatusServiceUnavailable, mensajePreciosNoDisponibles, "")
		return
	}
	preciosCatalogo, errorCatalogo := manejador.opciones.Precios.ConsultarPreciosComparables(
		peticion.Context(),
		listas.ConsultaPreciosComparables{
			ProductoIDs:      productoIDs,
			Lat:              origenUsado.Lat,
			Lon:              origenUsado.Lon,
			RadioKm:          opcionesOpt.RadioKm,
			MaximoSucursales: opcionesOpt.MaxSucursales,
		},
	)
	if errorCatalogo != nil {
		manejador.opciones.Log.Warn("catalogo_precios_error", "error", errorCatalogo.Error())
		escribirError(respuesta, http.StatusServiceUnavailable, mensajePreciosNoDisponibles, "")
		return
	}
	if len(preciosCatalogo.Sucursales) == 0 {
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeSinSucursalesCercanas, "")
		return
	}

	// --- Cálculo de la ruta óptima (sin costo de combustible) ----------------
	entrada := construirEntradaOptimizacion(origenUsado, articulosPendientes, preciosCatalogo, opcionesOpt)
	resultado, errorCalculo := optimizacion.Calcular(entrada)
	if errorCalculo != nil {
		manejador.responderErrorCalculo(respuesta, peticion, errorCalculo)
		return
	}

	escribirJSON(respuesta, http.StatusOK, respuestaOptimizar{
		Estimado: true,
		Origen:   origenUsado,
		Parametros: parametrosRespuesta{
			FactorDistancia: opcionesOpt.FactorDistancia,
			MaxParadas:      opcionesOpt.MaxParadas,
			UmbralAhorro:    opcionesOpt.UmbralAhorro,
			RadioKm:         opcionesOpt.RadioKm,
		},
		Ruta:                resultado.Ruta,
		BaseUnaTienda:       resultado.BaseUnaTienda,
		Ahorro:              resultado.Ahorro,
		NoCubiertos:         resultado.NoCubiertos,
		CandidatasEvaluadas: resultado.CandidatasEvaluadas,
	})
}

// =============================================================================
// Adaptadores y validaciones auxiliares
// =============================================================================

// resolverOrigen devuelve el origen indicado o, si se omitió, el centro de
// Temuco. Si el origen viene incompleto o fuera de rango devuelve el detalle.
func resolverOrigen(origenPedido *solicitudOrigen) (origenRespuesta, string) {
	if origenPedido == nil {
		return origenRespuesta{
			Lat:        latitudTemucoPorDefecto,
			Lon:        longitudTemucoPorDefecto,
			PorDefecto: true,
		}, ""
	}
	if origenPedido.Lat == nil || origenPedido.Lon == nil {
		return origenRespuesta{}, "origen: requiere lat y lon numéricos"
	}
	latitud, longitud := *origenPedido.Lat, *origenPedido.Lon
	if math.IsNaN(latitud) || math.IsInf(latitud, 0) || math.Abs(latitud) > latitudMaximaGeo {
		return origenRespuesta{}, "origen.lat: debe estar entre -90 y 90"
	}
	if math.IsNaN(longitud) || math.IsInf(longitud, 0) || math.Abs(longitud) > longitudMaximaGeo {
		return origenRespuesta{}, "origen.lon: debe estar entre -180 y 180"
	}
	return origenRespuesta{Lat: latitud, Lon: longitud, PorDefecto: false}, ""
}

// extraerArticulosPendientes filtra los artículos no comprados con producto_id
// válido y devuelve también la lista deduplicada de IDs para Catálogo.
func extraerArticulosPendientes(articulos []listas.ArticuloDetalle) ([]optimizacion.Articulo, []string) {
	pendientes := make([]optimizacion.Articulo, 0, len(articulos))
	productoIDs := make([]string, 0, len(articulos))
	vistos := make(map[string]bool, len(articulos))

	for _, articulo := range articulos {
		if articulo.EstaComprado || articulo.ProductoID == nil {
			continue
		}
		productoID := strings.TrimSpace(*articulo.ProductoID)
		if productoID == "" || articulo.Cantidad < 1 {
			continue
		}
		pendientes = append(pendientes, optimizacion.Articulo{
			ProductoID: productoID,
			Cantidad:   articulo.Cantidad,
		})
		if !vistos[productoID] {
			vistos[productoID] = true
			productoIDs = append(productoIDs, productoID)
		}
	}
	return pendientes, productoIDs
}

// construirEntradaOptimizacion convierte la respuesta de Catálogo a la entrada
// pura del optimizador. Descarta explícitamente ofertas sin precio positivo o
// sin stock para que las cadenas sin precios se ignoren sin provocar error.
func construirEntradaOptimizacion(
	origen origenRespuesta,
	articulos []optimizacion.Articulo,
	precios listas.RespuestaPreciosComparables,
	opciones OpcionesOptimizacion,
) optimizacion.Entrada {
	sucursales := make([]optimizacion.Sucursal, 0, len(precios.Sucursales))
	for _, sucursal := range precios.Sucursales {
		sucursales = append(sucursales, optimizacion.Sucursal{
			ID:        sucursal.SucursalID,
			Nombre:    sucursal.Nombre,
			Cadena:    sucursal.Cadena,
			Ubicacion: optimizacion.Coordenada{Lat: sucursal.Lat, Lon: sucursal.Lon},
		})
	}

	ofertas := make([]optimizacion.Oferta, 0)
	for _, producto := range precios.Productos {
		for _, oferta := range producto.Ofertas {
			if oferta.EnStock == nil || !*oferta.EnStock {
				continue
			}
			if oferta.Precio == nil || math.IsNaN(*oferta.Precio) || math.IsInf(*oferta.Precio, 0) {
				continue
			}
			precioPesos := int(math.Round(*oferta.Precio))
			if precioPesos < 1 || strings.TrimSpace(oferta.ProductoIDOferta) == "" {
				continue
			}
			ofertas = append(ofertas, optimizacion.Oferta{
				ProductoID:       producto.ProductoID,
				ProductoIDOferta: oferta.ProductoIDOferta,
				Nombre:           oferta.Nombre,
				SucursalID:       oferta.SucursalID,
				Precio:           precioPesos,
				EnStock:          true,
			})
		}
	}

	return optimizacion.Entrada{
		Origen:     optimizacion.Coordenada{Lat: origen.Lat, Lon: origen.Lon},
		Articulos:  articulos,
		Sucursales: sucursales,
		Ofertas:    ofertas,
		Parametros: optimizacion.Parametros{
			MaxParadas:         opciones.MaxParadas,
			FactorDistancia:    opciones.FactorDistancia,
			RendimientoKmL:     0,
			PrecioCombustibleL: 0,
			UmbralAhorro:       opciones.UmbralAhorro,
		},
	}
}

// responderErrorCalculo traduce los errores de optimizacion.Calcular al código
// HTTP y envoltorio del contrato.
func (manejador *handlerListas) responderErrorCalculo(respuesta http.ResponseWriter, peticion *http.Request, errorCalculo error) {
	var sinCobertura *optimizacion.ErrorSinCobertura
	switch {
	case errors.Is(errorCalculo, optimizacion.ErrSinArticulos):
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeListaSinArticulos, "")
	case errors.Is(errorCalculo, optimizacion.ErrSinSucursales):
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeSinSucursalesCercanas, "")
	case errors.As(errorCalculo, &sinCobertura):
		escribirError(respuesta, http.StatusUnprocessableEntity, mensajeSinCoberturaRuta, sinCobertura.Error())
	default:
		manejador.fallo(respuesta, peticion, errorCalculo)
	}
}
