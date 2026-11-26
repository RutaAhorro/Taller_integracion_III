// Equivalencias por EAN: valida el lote, agrupa por GTIN canónico y aplica las
// reglas de comparabilidad. La coincidencia es siempre exacta: no hay nombres,
// IA ni matching difuso.

package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
)

// ===== Errores y límites =====

// Errores del servicio de equivalencias. El adaptador HTTP los traduce a 400
// sin exponer detalles internos.
var (
	ErrLoteEquivalenciaVacio    = errors.New("la consulta de equivalencia requiere al menos un producto")
	ErrLoteEquivalenciaExcesivo = errors.New("la consulta de equivalencia supera el máximo de productos")
	ErrIDProductoEquivalencia   = errors.New("el identificador de producto no es un UUID válido")
)

// LimitesEquivalencia agrupa los valores configurables de la consulta: cantidad
// máxima de IDs y antigüedad máxima de una captura para publicar su precio.
type LimitesEquivalencia struct {
	MaximoProductos  int
	AntiguedadMaxima time.Duration
}

// Largos de GTIN aceptados y base del dígito verificador. Un código de otra
// longitud se considera inválido y nunca se usa para unir productos.
const (
	largoGTIN8        = 8
	largoGTIN12       = 12
	largoGTIN13       = 13
	largoGTIN14       = 14
	moduloVerificador = 10
)

// estadoEAN clasifica el código almacenado antes de decidir la equivalencia.
type estadoEAN int

const (
	eanValido estadoEAN = iota
	eanAusente
	eanInvalido
)

// interpretacionEAN conserva el GTIN canónico y su clasificación para no
// repetir el trabajo al construir la respuesta.
type interpretacionEAN struct {
	canonico string
	estado   estadoEAN
}

// ===== Servicio =====

// ServicioEquivalencias valida el lote y construye un grupo por producto.
type ServicioEquivalencias struct {
	repositorio domain.RepositorioEquivalencias
	limites     LimitesEquivalencia
	ahora       func() time.Time
}

// NuevoServicioEquivalencias inyecta el repositorio, los límites validados en
// config y el reloj. El reloj es una función para que las pruebas fijen la
// vigencia sin depender de la hora real.
func NuevoServicioEquivalencias(repositorio domain.RepositorioEquivalencias, limites LimitesEquivalencia, ahora func() time.Time) *ServicioEquivalencias {
	return &ServicioEquivalencias{repositorio: repositorio, limites: limites, ahora: ahora}
}

// Consultar devuelve un grupo por cada ID solicitado, en el mismo orden y sin
// repetidos. Un producto sin EAN válido o con presentaciones conflictivas sólo
// se compara consigo mismo; un ID inexistente se informa como no encontrado.
func (servicio *ServicioEquivalencias) Consultar(contexto context.Context, productoIDs []string) (*domain.RespuestaEquivalenciaDTO, error) {
	idsNormalizados, errorValidacion := servicio.normalizarYValidar(productoIDs)
	if errorValidacion != nil {
		return nil, errorValidacion
	}

	registros, errorConsulta := servicio.repositorio.ConsultarPorIDs(contexto, idsNormalizados)
	if errorConsulta != nil {
		return nil, errorConsulta
	}

	porProducto := make(map[string]domain.ProductoEquivalenciaRegistro, len(registros))
	interpretaciones := make(map[string]interpretacionEAN, len(registros))
	gruposPorEAN := make(map[string][]domain.ProductoEquivalenciaRegistro)
	for _, registro := range registros {
		interpretacion := interpretarEAN(registro.CodigoEAN)
		porProducto[registro.ProductoID] = registro
		interpretaciones[registro.ProductoID] = interpretacion
		if interpretacion.estado == eanValido {
			gruposPorEAN[interpretacion.canonico] = append(gruposPorEAN[interpretacion.canonico], registro)
		}
	}

	ahora := servicio.ahora()
	productos := make([]domain.EquivalenciaProductoDTO, 0, len(idsNormalizados))
	resumen := domain.ResumenEquivalenciaDTO{Solicitados: len(idsNormalizados)}
	for _, productoID := range idsNormalizados {
		item := servicio.construirProducto(productoID, porProducto, interpretaciones, gruposPorEAN, ahora)
		productos = append(productos, item)
		contarEstadoEquivalencia(&resumen, item.Estado)
	}

	return &domain.RespuestaEquivalenciaDTO{Productos: productos, Resumen: resumen}, nil
}

// construirProducto arma la respuesta de un ID y decide si el grupo es
// comparable. La oferta del propio producto siempre se conserva.
func (servicio *ServicioEquivalencias) construirProducto(productoID string, porProducto map[string]domain.ProductoEquivalenciaRegistro,
	interpretaciones map[string]interpretacionEAN, gruposPorEAN map[string][]domain.ProductoEquivalenciaRegistro,
	ahora time.Time) domain.EquivalenciaProductoDTO {
	registro, encontrado := porProducto[productoID]
	if !encontrado {
		return domain.EquivalenciaProductoDTO{
			ProductoID: productoID,
			Estado:     domain.EstadoEquivalenciaNoEncontrado,
			Ofertas:    []domain.OfertaEquivalenteDTO{},
		}
	}

	item := domain.EquivalenciaProductoDTO{
		ProductoID:   productoID,
		Presentacion: strings.TrimSpace(registro.Formato),
		Ofertas:      []domain.OfertaEquivalenteDTO{servicio.construirOferta(registro, ahora)},
	}

	interpretacion := interpretaciones[productoID]
	switch interpretacion.estado {
	case eanAusente:
		item.Estado = domain.EstadoEquivalenciaSinEAN
	case eanInvalido:
		item.Estado = domain.EstadoEquivalenciaEANInvalido
	default:
		eanCanonico := interpretacion.canonico
		item.EAN = &eanCanonico
		grupo := gruposPorEAN[eanCanonico]
		if hayConflictoPresentacion(grupo) {
			item.Estado = domain.EstadoEquivalenciaConflictoPresentacion
			break
		}
		item.Estado = domain.EstadoEquivalenciaComparable
		item.Ofertas = make([]domain.OfertaEquivalenteDTO, 0, len(grupo))
		for _, miembro := range grupo {
			item.Ofertas = append(item.Ofertas, servicio.construirOferta(miembro, ahora))
		}
		ordenarOfertas(item.Ofertas)
	}
	return item
}

// construirOferta publica sólo los datos de la captura más reciente cuando es
// vigente. Una captura ausente, futura o vieja deja los precios y el stock en
// nulo para que el consumidor no los use en un cálculo.
func (servicio *ServicioEquivalencias) construirOferta(registro domain.ProductoEquivalenciaRegistro, ahora time.Time) domain.OfertaEquivalenteDTO {
	oferta := domain.OfertaEquivalenteDTO{
		ProductoIDOferta: registro.ProductoID,
		SucursalID:       registro.SucursalID,
		Cadena:           registro.Cadena,
		Sucursal:         registro.Sucursal,
		Nombre:           registro.Nombre,
		Unidad:           strings.TrimSpace(registro.Formato),
		CapturadoEl:      registro.CapturadoEl,
	}
	if registro.CapturadoEl == nil || !precioNormalValido(registro.PrecioNormal) {
		oferta.EstadoPrecio = domain.EstadoPrecioSinCaptura
		oferta.CapturadoEl = nil
		return oferta
	}

	antiguedad := ahora.UTC().Sub(registro.CapturadoEl.UTC())
	if antiguedad < 0 || antiguedad > servicio.limites.AntiguedadMaxima {
		oferta.EstadoPrecio = domain.EstadoPrecioVencido
		return oferta
	}

	oferta.EstadoPrecio = domain.EstadoPrecioVigente
	oferta.EnStock = registro.EnStock
	oferta.PrecioNormal = registro.PrecioNormal
	oferta.PrecioOferta = registro.PrecioOferta
	oferta.Precio = precioEfectivo(registro.PrecioNormal, registro.PrecioOferta)
	return oferta
}

// ===== Reglas de EAN =====

// interpretarEAN deja sólo dígitos, valida largo y dígito verificador y
// devuelve el GTIN canónico de 14 dígitos. Rellenar con ceros a la izquierda
// permite que un UPC-A de 12 y su EAN-13 con cero inicial sean la misma clave,
// sin perder ceros significativos.
func interpretarEAN(valor *string) interpretacionEAN {
	if valor == nil || strings.TrimSpace(*valor) == "" {
		return interpretacionEAN{estado: eanAusente}
	}
	digitos := soloDigitos(*valor)
	if !largoGTINValido(len(digitos)) || !digitoVerificadorValido(digitos) {
		return interpretacionEAN{estado: eanInvalido}
	}
	return interpretacionEAN{canonico: rellenarGTIN14(digitos), estado: eanValido}
}

// soloDigitos descarta separadores u otros caracteres y conserva el orden.
func soloDigitos(texto string) string {
	var constructor strings.Builder
	constructor.Grow(len(texto))
	for _, caracter := range texto {
		if caracter >= '0' && caracter <= '9' {
			constructor.WriteRune(caracter)
		}
	}
	return constructor.String()
}

// largoGTINValido acepta los cuatro largos estándar de GTIN.
func largoGTINValido(largo int) bool {
	switch largo {
	case largoGTIN8, largoGTIN12, largoGTIN13, largoGTIN14:
		return true
	default:
		return false
	}
}

// digitoVerificadorValido aplica el algoritmo módulo 10 de GTIN. El dígito
// inmediatamente a la izquierda del verificador pesa 3 y los demás alternan.
func digitoVerificadorValido(digitos string) bool {
	suma := 0
	peso := 3
	for indice := len(digitos) - 2; indice >= 0; indice-- {
		suma += int(digitos[indice]-'0') * peso
		if peso == 3 {
			peso = 1
		} else {
			peso = 3
		}
	}
	verificadorEsperado := (moduloVerificador - suma%moduloVerificador) % moduloVerificador
	return int(digitos[len(digitos)-1]-'0') == verificadorEsperado
}

// rellenarGTIN14 completa con ceros a la izquierda sin recortar el código.
func rellenarGTIN14(digitos string) string {
	return strings.Repeat("0", largoGTIN14-len(digitos)) + digitos
}

// ===== Reglas de presentación =====

// hayConflictoPresentacion compara sólo formatos explícitos de peso, volumen o
// empaque. Un formato vacío o un marcador genérico de unidad de venta ("1 un",
// "1 unidad", "format") no prueba una presentación distinta; dos formatos
// específicos que difieren tras normalizar mayúsculas y espacios sí bloquean la
// equivalencia.
func hayConflictoPresentacion(grupo []domain.ProductoEquivalenciaRegistro) bool {
	presentaciones := make(map[string]bool, 2)
	for _, registro := range grupo {
		presentacion := normalizarPresentacion(registro.Formato)
		if esPresentacionGenerica(presentacion) {
			continue
		}
		presentaciones[presentacion] = true
		if len(presentaciones) > 1 {
			return true
		}
	}
	return false
}

// esPresentacionGenerica reconoce textos vacíos o marcadores genéricos de
// unidad de venta que algunos scrapers registran cuando no separan peso/volumen
// del título (p. ej. "1 un" en Santa Isabel). No contradicen un formato
// específico si el GTIN canónico es idéntico.
func esPresentacionGenerica(presentacion string) bool {
	switch presentacion {
	case "", "1 un", "1 un.", "1 unidad", "un", "un.", "unidad", "format":
		return true
	default:
		return false
	}
}

// normalizarPresentacion unifica mayúsculas y espacios sin interpretar unidades
// ni recurrir a coincidencia difusa.
func normalizarPresentacion(texto string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(texto))), " ")
}

// ordenarOfertas fija un orden estable por sucursal y luego por producto.
func ordenarOfertas(ofertas []domain.OfertaEquivalenteDTO) {
	sort.Slice(ofertas, func(indiceA, indiceB int) bool {
		if ofertas[indiceA].SucursalID != ofertas[indiceB].SucursalID {
			return ofertas[indiceA].SucursalID < ofertas[indiceB].SucursalID
		}
		return ofertas[indiceA].ProductoIDOferta < ofertas[indiceB].ProductoIDOferta
	})
}

// contarEstadoEquivalencia suma el estado al resumen de cobertura.
func contarEstadoEquivalencia(resumen *domain.ResumenEquivalenciaDTO, estado string) {
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

// ===== Validación del lote =====

// normalizarYValidar comprueba el tope, descarta espacios, valida los UUID y
// elimina repetidos conservando el orden de la primera aparición.
func (servicio *ServicioEquivalencias) normalizarYValidar(productoIDs []string) ([]string, error) {
	if len(productoIDs) == 0 {
		return nil, ErrLoteEquivalenciaVacio
	}
	if len(productoIDs) > servicio.limites.MaximoProductos {
		return nil, ErrLoteEquivalenciaExcesivo
	}

	idsNormalizados := make([]string, 0, len(productoIDs))
	vistos := make(map[string]bool, len(productoIDs))
	for _, productoID := range productoIDs {
		identificador := strings.TrimSpace(productoID)
		identificadorParseado, errorParseo := uuid.Parse(identificador)
		if errorParseo != nil {
			return nil, ErrIDProductoEquivalencia
		}
		identificadorCanonico := identificadorParseado.String()
		if vistos[identificadorCanonico] {
			continue
		}
		vistos[identificadorCanonico] = true
		idsNormalizados = append(idsNormalizados, identificadorCanonico)
	}
	return idsNormalizados, nil
}
