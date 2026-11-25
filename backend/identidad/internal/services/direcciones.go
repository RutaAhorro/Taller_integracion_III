// Casos de uso de las direcciones guardadas del usuario: validación y
// normalización de campos y delegación al repositorio. La pertenencia al
// usuario se aplica siempre con el identificador de la sesión.

package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
)

// ===== Errores =====

var (
	// ErrDireccionInvalida envuelve cualquier incumplimiento de las reglas de campos.
	ErrDireccionInvalida = errors.New("dirección inválida")
	// ErrIDDireccionInvalido se emite cuando el identificador de ruta no es un UUID.
	ErrIDDireccionInvalido = errors.New("identificador de dirección inválido")
	// ErrDireccionNoEncontrada indica dirección inexistente o de otra cuenta.
	ErrDireccionNoEncontrada = domain.ErrDireccionNoEncontrada
	// ErrLimiteDirecciones indica que el usuario alcanzó su máximo.
	ErrLimiteDirecciones = domain.ErrLimiteDirecciones
	// ErrPrincipalObligatoria impide desmarcar la principal sin elegir otra.
	ErrPrincipalObligatoria = domain.ErrPrincipalObligatoria
	// ErrConflictoPrincipal señala una carrera por la dirección principal.
	ErrConflictoPrincipal = domain.ErrConflictoPrincipal
)

// ===== Contrato del servicio =====

// ServicioDirecciones reúne el CRUD de direcciones del usuario autenticado.
type ServicioDirecciones struct {
	repositorio domain.RepositorioDirecciones
}

// NuevoServicioDirecciones inicializa el servicio con su repositorio.
func NuevoServicioDirecciones(repositorio domain.RepositorioDirecciones) *ServicioDirecciones {
	return &ServicioDirecciones{repositorio: repositorio}
}

// ===== Casos de uso =====

// Listar devuelve las direcciones del usuario, principal primero.
func (servicio *ServicioDirecciones) Listar(contexto context.Context, usuarioID string) (*domain.ListaDireccionesDTO, error) {
	if errorObtenido := validarUUID(usuarioID, ErrIDUsuarioInvalido); errorObtenido != nil {
		return nil, errorObtenido
	}
	direcciones, errorObtenido := servicio.repositorio.Listar(contexto, usuarioID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	// Un arreglo vacío (no null) para que el cliente itere sin comprobar nil.
	lista := domain.ListaDireccionesDTO{Data: make([]domain.DireccionDTO, 0, len(direcciones))}
	for _, direccion := range direcciones {
		lista.Data = append(lista.Data, aDireccionDTO(direccion))
	}
	return &lista, nil
}

// Crear valida y guarda una dirección nueva del usuario.
func (servicio *ServicioDirecciones) Crear(contexto context.Context, usuarioID string, entrada domain.EntradaDireccionDTO) (*domain.DireccionDTO, error) {
	if errorObtenido := validarUUID(usuarioID, ErrIDUsuarioInvalido); errorObtenido != nil {
		return nil, errorObtenido
	}
	direccion, errorObtenido := validarEntradaDireccion(entrada)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	// La primera dirección queda principal aunque se pida lo contrario: lo
	// decide el repositorio dentro de la transacción, que es quien conoce el conteo.
	direccion.EsPrincipal = entrada.EsPrincipal != nil && *entrada.EsPrincipal
	if errorObtenido := servicio.repositorio.Crear(contexto, usuarioID, &direccion, domain.MaximoDireccionesPorUsuario); errorObtenido != nil {
		return nil, errorObtenido
	}
	dto := aDireccionDTO(direccion)
	return &dto, nil
}

// Actualizar reemplaza los campos editables de una dirección propia.
func (servicio *ServicioDirecciones) Actualizar(contexto context.Context, usuarioID, direccionID string, entrada domain.EntradaDireccionDTO) (*domain.DireccionDTO, error) {
	if errorObtenido := validarUUID(usuarioID, ErrIDUsuarioInvalido); errorObtenido != nil {
		return nil, errorObtenido
	}
	if errorObtenido := validarUUID(direccionID, ErrIDDireccionInvalido); errorObtenido != nil {
		return nil, errorObtenido
	}
	datos, errorObtenido := validarEntradaDireccion(entrada)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	actualizada, errorObtenido := servicio.repositorio.Actualizar(contexto, usuarioID, direccionID, datos, entrada.EsPrincipal)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	dto := aDireccionDTO(*actualizada)
	return &dto, nil
}

// Eliminar borra una dirección propia.
func (servicio *ServicioDirecciones) Eliminar(contexto context.Context, usuarioID, direccionID string) error {
	if errorObtenido := validarUUID(usuarioID, ErrIDUsuarioInvalido); errorObtenido != nil {
		return errorObtenido
	}
	if errorObtenido := validarUUID(direccionID, ErrIDDireccionInvalido); errorObtenido != nil {
		return errorObtenido
	}
	return servicio.repositorio.Eliminar(contexto, usuarioID, direccionID)
}

// ===== Validación =====

// validarUUID devuelve el error indicado si el texto no es un UUID en forma
// canónica (36 caracteres con guiones, sin espacios). uuid.Parse también acepta
// "urn:uuid:…" y llaves, que PostgreSQL rechaza: sin esta comparación esos ids
// llegarían a la consulta y responderían 500 en vez de 400.
func validarUUID(valor string, errorInvalido error) error {
	identificador, errorObtenido := uuid.Parse(valor)
	if errorObtenido != nil || identificador.String() != strings.ToLower(valor) {
		return errorInvalido
	}
	return nil
}

// validarEntradaDireccion normaliza (trim) y valida los campos editables y
// devuelve la entidad lista para persistir, sin id ni dueño. Las coordenadas se
// redondean a la precisión de la columna.
func validarEntradaDireccion(entrada domain.EntradaDireccionDTO) (domain.DireccionUsuario, error) {
	texto, errorObtenido := validarTextoDireccion("texto_direccion", entrada.TextoDireccion, domain.MaximoCaracteresTextoDireccion, true)
	if errorObtenido != nil {
		return domain.DireccionUsuario{}, errorObtenido
	}
	comuna, errorObtenido := validarTextoDireccion("comuna", entrada.Comuna, domain.MaximoCaracteresComuna, true)
	if errorObtenido != nil {
		return domain.DireccionUsuario{}, errorObtenido
	}
	direccion := domain.DireccionUsuario{TextoDireccion: texto, Comuna: comuna}

	// Una etiqueta vacía equivale a no tener etiqueta.
	if entrada.Etiqueta != nil {
		etiqueta, errorEtiqueta := validarTextoDireccion("etiqueta", *entrada.Etiqueta, domain.MaximoCaracteresEtiqueta, false)
		if errorEtiqueta != nil {
			return domain.DireccionUsuario{}, errorEtiqueta
		}
		if etiqueta != "" {
			direccion.Etiqueta = &etiqueta
		}
	}

	if (entrada.Lat == nil) != (entrada.Lon == nil) {
		return domain.DireccionUsuario{}, fmt.Errorf("%w: lat y lon deben enviarse juntas o ninguna", ErrDireccionInvalida)
	}
	if entrada.Lat != nil {
		latitud, longitud := *entrada.Lat, *entrada.Lon
		if !coordenadaEnRango(latitud, domain.LatitudMinima, domain.LatitudMaxima) {
			return domain.DireccionUsuario{}, fmt.Errorf("%w: lat debe estar entre %g y %g", ErrDireccionInvalida, domain.LatitudMinima, domain.LatitudMaxima)
		}
		if !coordenadaEnRango(longitud, domain.LongitudMinima, domain.LongitudMaxima) {
			return domain.DireccionUsuario{}, fmt.Errorf("%w: lon debe estar entre %g y %g", ErrDireccionInvalida, domain.LongitudMinima, domain.LongitudMaxima)
		}
		latitud, longitud = redondearCoordenada(latitud), redondearCoordenada(longitud)
		direccion.Lat, direccion.Lon = &latitud, &longitud
	}
	return direccion, nil
}

// validarTextoDireccion recorta el texto, rechaza caracteres de control y
// exige longitud (en caracteres, como VARCHAR) dentro del máximo. Si
// obligatorio, además no admite el texto vacío.
func validarTextoDireccion(campo, valor string, maximo int, obligatorio bool) (string, error) {
	limpio := strings.TrimSpace(valor)
	if obligatorio && limpio == "" {
		return "", fmt.Errorf("%w: %s es obligatoria", ErrDireccionInvalida, campo)
	}
	if utf8.RuneCountInString(limpio) > maximo {
		return "", fmt.Errorf("%w: %s no puede superar %d caracteres", ErrDireccionInvalida, campo, maximo)
	}
	if strings.IndexFunc(limpio, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: %s contiene caracteres de control", ErrDireccionInvalida, campo)
	}
	return limpio, nil
}

// coordenadaEnRango rechaza NaN, infinitos y valores fuera del intervalo.
func coordenadaEnRango(valor, minimo, maximo float64) bool {
	return !math.IsNaN(valor) && !math.IsInf(valor, 0) && valor >= minimo && valor <= maximo
}

// redondearCoordenada ajusta la coordenada a los decimales que guarda la base.
func redondearCoordenada(valor float64) float64 {
	factor := math.Pow10(domain.DecimalesCoordenadas)
	return math.Round(valor*factor) / factor
}

// aDireccionDTO convierte la entidad en su representación pública.
func aDireccionDTO(direccion domain.DireccionUsuario) domain.DireccionDTO {
	return domain.DireccionDTO{
		ID:             direccion.ID.String(),
		Etiqueta:       direccion.Etiqueta,
		TextoDireccion: direccion.TextoDireccion,
		Comuna:         direccion.Comuna,
		Lat:            direccion.Lat,
		Lon:            direccion.Lon,
		EsPrincipal:    direccion.EsPrincipal,
		CreadoEl:       direccion.CreadoEl,
	}
}
