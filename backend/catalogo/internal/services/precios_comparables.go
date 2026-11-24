// Precios comparables: aplica el radio y el tope de sucursales sobre las
// equivalencias exactas de SUP-244. Nunca inventa coordenadas ni precios y
// devuelve las ausencias como datos explícitos.

package services

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
)

// ===== Errores y límites =====

// Errores del servicio de precios comparables. El adaptador HTTP los traduce a
// códigos públicos sin exponer detalles internos.
var (
	ErrLotePreciosVacio        = errors.New("la consulta de precios comparables requiere al menos un producto")
	ErrLotePreciosExcesivo     = errors.New("la consulta de precios comparables supera el máximo de productos")
	ErrIDProductoPrecios       = errors.New("el identificador de producto no es un UUID válido")
	ErrCoordenadasPrecios      = errors.New("el origen requiere lat y lon numéricos dentro de rango")
	ErrRadioPrecios            = errors.New("el radio debe ser positivo y estar dentro del límite configurado")
	ErrMaximoSucursalesPrecios = errors.New("maximo_sucursales debe estar entre 1 y 10")
)

// LimitesPreciosComparables agrupa los valores validados en internal/config.
type LimitesPreciosComparables struct {
	MaximoProductos int
	RadioMaximoKm   float64
}

// maximoSucursalesPermitidas es el tope del contrato del motor (SUP-323). El
// cliente puede pedir menos; Catálogo nunca devuelve más.
const maximoSucursalesPermitidas = 10

// ===== Servicio =====

// consultaEquivalencias es la capacidad de SUP-244 que este servicio reutiliza.
// La interfaz mantiene el servicio probatable sin acoplarlo a la implementación.
type consultaEquivalencias interface {
	Consultar(contexto context.Context, productoIDs []string) (*domain.RespuestaEquivalenciaDTO, error)
}

// ServicioPreciosComparables combina candidatas cercanas con las ofertas ya
// resueltas por equivalencia exacta.
type ServicioPreciosComparables struct {
	repositorio   domain.RepositorioPreciosComparables
	equivalencias consultaEquivalencias
	limites       LimitesPreciosComparables
}

// NuevoServicioPreciosComparables inyecta la lectura geoespacial, el servicio
// de equivalencias y los límites de la consulta.
func NuevoServicioPreciosComparables(repositorio domain.RepositorioPreciosComparables, equivalencias consultaEquivalencias, limites LimitesPreciosComparables) *ServicioPreciosComparables {
	return &ServicioPreciosComparables{repositorio: repositorio, equivalencias: equivalencias, limites: limites}
}

// Consultar valida la solicitud, acota las sucursales por radio y publica
// sólo las ofertas de esas sucursales. Lo que no existe se informa en listas
// separadas: sin_ofertas y sucursales_excluidas.
func (servicio *ServicioPreciosComparables) Consultar(contexto context.Context, solicitud domain.ConsultaPreciosComparablesDTO) (*domain.RespuestaPreciosComparablesDTO, error) {
	productoIDs, latitud, longitud, radioKm, maximoSucursales, errorValidacion := servicio.validar(solicitud)
	if errorValidacion != nil {
		return nil, errorValidacion
	}

	candidatas, errorCandidatas := servicio.repositorio.ConsultarCercanas(contexto, latitud, longitud, radioKm, maximoSucursales)
	if errorCandidatas != nil {
		return nil, errorCandidatas
	}
	sucursales, errorSucursales := servicio.repositorio.ConsultarSucursales(contexto)
	if errorSucursales != nil {
		return nil, errorSucursales
	}
	equivalencias, errorEquivalencias := servicio.equivalencias.Consultar(contexto, productoIDs)
	if errorEquivalencias != nil {
		return nil, errorEquivalencias
	}

	permitidas := make(map[int]bool, len(candidatas))
	for _, candidata := range candidatas {
		permitidas[candidata.ID] = true
	}
	excluidas := clasificarExcluidas(sucursales)

	respuesta := &domain.RespuestaPreciosComparablesDTO{
		Sucursales:          convertirCandidatas(candidatas),
		SinOfertas:          []string{},
		SucursalesExcluidas: excluidas,
		Resumen: domain.ResumenPreciosComparablesDTO{
			Solicitados:          len(productoIDs),
			SucursalesCandidatas: len(candidatas),
			SucursalesExcluidas:  len(excluidas),
		},
	}

	for _, equivalencia := range equivalencias.Productos {
		ofertas := make([]domain.OfertaEquivalenteDTO, 0, len(equivalencia.Ofertas))
		for _, oferta := range equivalencia.Ofertas {
			if !permitidas[oferta.SucursalID] {
				continue
			}
			ofertas = append(ofertas, oferta)
			if oferta.Precio == nil {
				respuesta.Resumen.OfertasSinPrecio++
			}
		}

		contarEstadoPrecios(&respuesta.Resumen, equivalencia.Estado)
		if len(ofertas) == 0 {
			respuesta.SinOfertas = append(respuesta.SinOfertas, equivalencia.ProductoID)
			respuesta.Resumen.SinOferta++
			continue
		}
		respuesta.Resumen.ConOferta++
		respuesta.Productos = append(respuesta.Productos, domain.ProductoPreciosComparableDTO{
			ProductoID:   equivalencia.ProductoID,
			Estado:       equivalencia.Estado,
			EAN:          equivalencia.EAN,
			Presentacion: equivalencia.Presentacion,
			Ofertas:      ofertas,
		})
	}
	return respuesta, nil
}

// ===== Validación =====

// validar comprueba el lote, las coordenadas, el radio y el tope antes de
// consultar la base. Los UUID se canonizan y deduplican conservando el orden.
func (servicio *ServicioPreciosComparables) validar(solicitud domain.ConsultaPreciosComparablesDTO) ([]string, float64, float64, float64, int, error) {
	if len(solicitud.ProductoIDs) == 0 {
		return nil, 0, 0, 0, 0, ErrLotePreciosVacio
	}
	if len(solicitud.ProductoIDs) > servicio.limites.MaximoProductos {
		return nil, 0, 0, 0, 0, ErrLotePreciosExcesivo
	}
	idsNormalizados := make([]string, 0, len(solicitud.ProductoIDs))
	vistos := make(map[string]bool, len(solicitud.ProductoIDs))
	for _, productoID := range solicitud.ProductoIDs {
		identificador, errorParseo := uuid.Parse(strings.TrimSpace(productoID))
		if errorParseo != nil {
			return nil, 0, 0, 0, 0, ErrIDProductoPrecios
		}
		canonico := identificador.String()
		if vistos[canonico] {
			continue
		}
		vistos[canonico] = true
		idsNormalizados = append(idsNormalizados, canonico)
	}

	if solicitud.Latitud == nil || solicitud.Longitud == nil ||
		!coordenadaValida(*solicitud.Latitud, -90, 90) || !coordenadaValida(*solicitud.Longitud, -180, 180) {
		return nil, 0, 0, 0, 0, ErrCoordenadasPrecios
	}
	if solicitud.RadioKm == nil || math.IsNaN(*solicitud.RadioKm) || math.IsInf(*solicitud.RadioKm, 0) ||
		*solicitud.RadioKm <= 0 || *solicitud.RadioKm > servicio.limites.RadioMaximoKm {
		return nil, 0, 0, 0, 0, ErrRadioPrecios
	}
	if solicitud.MaximoSucursales == nil || *solicitud.MaximoSucursales < 1 ||
		*solicitud.MaximoSucursales > maximoSucursalesPermitidas {
		return nil, 0, 0, 0, 0, ErrMaximoSucursalesPrecios
	}

	return idsNormalizados, *solicitud.Latitud, *solicitud.Longitud, *solicitud.RadioKm, *solicitud.MaximoSucursales, nil
}

// coordenadaValida rechaza NaN, infinito y valores fuera del rango geográfico.
func coordenadaValida(valor float64, minimo, maximo float64) bool {
	return !math.IsNaN(valor) && !math.IsInf(valor, 0) && valor >= minimo && valor <= maximo
}

// ===== Ensamblado de la respuesta =====

// convertirCandidatas copia el resultado del repositorio al DTO público.
func convertirCandidatas(candidatas []domain.SucursalCercanaRegistro) []domain.SucursalCandidataDTO {
	respuesta := make([]domain.SucursalCandidataDTO, 0, len(candidatas))
	for _, candidata := range candidatas {
		respuesta = append(respuesta, domain.SucursalCandidataDTO{
			SucursalID:  candidata.ID,
			Nombre:      candidata.Nombre,
			Cadena:      candidata.Cadena,
			Lat:         candidata.Lat,
			Lon:         candidata.Lon,
			DistanciaKm: candidata.DistanciaKm,
		})
	}
	return respuesta
}

// clasificarExcluidas informa sucursales inactivas, sin coordenadas o con
// dirección de tienda sólo en línea. Una sucursal fuera del radio no se
// clasifica: simplemente no es candidata.
func clasificarExcluidas(sucursales []domain.SucursalRegistro) []domain.SucursalExcluidaDTO {
	excluidas := make([]domain.SucursalExcluidaDTO, 0)
	for _, sucursal := range sucursales {
		motivo := ""
		switch {
		case !sucursal.Activa:
			motivo = domain.MotivoSucursalInactiva
		case sucursal.Lat == nil || sucursal.Lon == nil:
			motivo = domain.MotivoSucursalSinCoordenadas
		case esSucursalOnline(sucursal):
			motivo = domain.MotivoSucursalOnline
		}
		if motivo != "" {
			excluidas = append(excluidas, domain.SucursalExcluidaDTO{
				SucursalID: sucursal.ID,
				Nombre:     sucursal.Nombre,
				Cadena:     sucursal.Cadena,
				Motivo:     motivo,
			})
		}
	}
	sort.Slice(excluidas, func(indiceA, indiceB int) bool {
		return excluidas[indiceA].SucursalID < excluidas[indiceB].SucursalID
	})
	return excluidas
}

// esSucursalOnline reconoce el placeholder histórico "Casa Matriz / Online".
// Tras la migración 13_ las filas 5 y 6 son tiendas físicas y no caen aquí.
func esSucursalOnline(sucursal domain.SucursalRegistro) bool {
	direccion := strings.ToLower(strings.TrimSpace(sucursal.Direccion))
	nombre := strings.ToLower(sucursal.Nombre)
	return direccion == "casa matriz / online" || strings.Contains(nombre, "online")
}

// contarEstadoPrecios traslada los estados de SUP-244 al resumen de cobertura
// de esta consulta.
func contarEstadoPrecios(resumen *domain.ResumenPreciosComparablesDTO, estado string) {
	switch estado {
	case domain.EstadoEquivalenciaComparable:
		resumen.Comparables++
	case domain.EstadoEquivalenciaSinEAN:
		resumen.SinEAN++
	case domain.EstadoEquivalenciaEANInvalido:
		resumen.EANInvalidos++
	case domain.EstadoEquivalenciaConflictoPresentacion:
		resumen.ConflictosPresentacion++
	case domain.EstadoEquivalenciaNoEncontrado:
		resumen.NoEncontrados++
	}
}
