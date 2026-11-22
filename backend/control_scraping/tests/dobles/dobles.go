// Package dobles implementa en memoria los contratos de domain para las
// pruebas unitarias de services y api. Cada prueba crea los pocos datos que
// necesita; aquí no hay datos de ejemplo.
package dobles

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// ===== Trabajos =====

// Trabajos implementa domain.TrabajoRepository sobre un mapa.
type Trabajos struct {
	Guardados map[uuid.UUID]*domain.TrabajoScraper
}

// NuevosTrabajos crea el repositorio vacío.
func NuevosTrabajos() *Trabajos {
	return &Trabajos{Guardados: map[uuid.UUID]*domain.TrabajoScraper{}}
}

// Agregar guarda un trabajo en progreso de la cadena indicada y devuelve su ID.
func (repositorio *Trabajos) Agregar(cadenaID int, cadenaNombre string) uuid.UUID {
	identificador := uuid.New()
	repositorio.Guardados[identificador] = &domain.TrabajoScraper{ID: identificador, CadenaID: cadenaID, CadenaNombre: cadenaNombre, Estado: domain.EstadoEnProgreso, IniciadoEl: time.Now().Add(-time.Minute)}
	return identificador
}

// CrearTrabajo guarda el trabajo recibido.
func (repositorio *Trabajos) CrearTrabajo(_ context.Context, trabajo *domain.TrabajoScraper) error {
	repositorio.Guardados[trabajo.ID] = trabajo
	return nil
}

// ObtenerTrabajoPorID devuelve una copia del trabajo o nil si no existe.
func (repositorio *Trabajos) ObtenerTrabajoPorID(_ context.Context, identificador uuid.UUID) (*domain.TrabajoScraper, error) {
	trabajo, existe := repositorio.Guardados[identificador]
	if !existe {
		return nil, nil
	}
	copia := *trabajo
	return &copia, nil
}

// FinalizarTrabajo aplica el cierre como lo hace el repositorio SQL sin entregas.
func (repositorio *Trabajos) FinalizarTrabajo(_ context.Context, identificador uuid.UUID, estado string, elementosExtraidos *int, respuestasBloqueadas *int, registroErrores *string) (*domain.TrabajoScraper, error) {
	trabajo, existe := repositorio.Guardados[identificador]
	if !existe {
		return nil, nil
	}
	ahora := time.Now()
	trabajo.Estado, trabajo.FinalizadoEl, trabajo.RegistroErrores = estado, &ahora, registroErrores
	if elementosExtraidos != nil {
		trabajo.ElementosExtraidos = *elementosExtraidos
	}
	if respuestasBloqueadas != nil {
		trabajo.RespuestasBloqueadas = *respuestasBloqueadas
	}
	copia := *trabajo
	return &copia, nil
}

// ===== Catálogo =====

// Catalogo implementa domain.CatalogoRepository. Error, si se define, lo
// devuelve cualquier operación; ErrorIngesta sólo IngestarLote.
type Catalogo struct {
	Cadenas      map[int]*domain.CadenaSupermercado
	Sucursales   map[int]*domain.SucursalSupermercado
	Error        error
	ErrorIngesta error
	Ingestas     int
}

// NuevoCatalogo crea un Catálogo vacío.
func NuevoCatalogo() *Catalogo {
	return &Catalogo{Cadenas: map[int]*domain.CadenaSupermercado{}, Sucursales: map[int]*domain.SucursalSupermercado{}}
}

// AgregarCadena registra una cadena con su estado de activación.
func (catalogo *Catalogo) AgregarCadena(identificador int, nombre string, activa bool) {
	catalogo.Cadenas[identificador] = &domain.CadenaSupermercado{ID: identificador, Nombre: nombre, EstaActiva: activa}
}

// AgregarSucursal registra una sucursal de una cadena.
func (catalogo *Catalogo) AgregarSucursal(identificador, cadenaID int) {
	catalogo.Sucursales[identificador] = &domain.SucursalSupermercado{ID: identificador, CadenaID: cadenaID}
}

// ObtenerCadena busca por ID o, si es 0, por nombre sin distinguir mayúsculas.
func (catalogo *Catalogo) ObtenerCadena(_ context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	if catalogo.Error != nil {
		return nil, catalogo.Error
	}
	if cadena, existe := catalogo.Cadenas[cadenaID]; existe {
		return cadena, nil
	}
	for _, cadena := range catalogo.Cadenas {
		if nombre != "" && strings.EqualFold(cadena.Nombre, nombre) {
			return cadena, nil
		}
	}
	return nil, nil
}

// ObtenerSucursal busca por ID o devuelve la primera sucursal de la cadena.
func (catalogo *Catalogo) ObtenerSucursal(_ context.Context, cadenaID int, sucursalID *int, _ *string) (*domain.SucursalSupermercado, error) {
	if catalogo.Error != nil {
		return nil, catalogo.Error
	}
	if sucursalID != nil {
		return catalogo.Sucursales[*sucursalID], nil
	}
	for _, sucursal := range catalogo.Sucursales {
		if sucursal.CadenaID == cadenaID {
			return sucursal, nil
		}
	}
	return nil, nil
}

// IngestarLote devuelve contadores coherentes: todos los productos insertados.
func (catalogo *Catalogo) IngestarLote(_ context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	if catalogo.Error != nil {
		return nil, catalogo.Error
	}
	if catalogo.ErrorIngesta != nil {
		return nil, catalogo.ErrorIngesta
	}
	catalogo.Ingestas++
	resultado := &domain.IngestaResultadoDTO{SucursalID: sucursalID, TotalRecibidos: len(productos), Insertados: len(productos), PreciosRegistrados: len(productos)}
	if trabajoID != nil {
		texto := trabajoID.String()
		resultado.TrabajoID = &texto
	}
	return resultado, nil
}

// ===== Entregas =====

// Reprogramacion registra una llamada a Reprogramar.
type Reprogramacion struct {
	Codigo     string
	Permanente bool
}

// Entregas implementa domain.EntregaRepository en memoria. ErrorConfirmar
// simula que Catálogo confirmó pero el contador local no se pudo sumar;
// LeaseOcupada, que otro proceso tiene tomadas todas las entregas.
type Entregas struct {
	Guardadas        map[uuid.UUID]*domain.Entrega
	Trabajos         *Trabajos
	ErrorConfirmar   error
	LeaseOcupada     bool
	Reprogramaciones []Reprogramacion
}

// NuevasEntregas crea el ledger vacío; trabajos permite sumar contadores.
func NuevasEntregas(trabajos *Trabajos) *Entregas {
	return &Entregas{Guardadas: map[uuid.UUID]*domain.Entrega{}, Trabajos: trabajos}
}

// Registrar guarda la entrega una vez y detecta otra huella con la misma clave.
func (repositorio *Entregas) Registrar(_ context.Context, entrega *domain.Entrega) (*domain.Entrega, error) {
	if entrega.TrabajoID != nil {
		trabajo, existe := repositorio.Trabajos.Guardados[*entrega.TrabajoID]
		if !existe {
			return nil, domain.ErrTrabajoNoEncontrado
		}
		if domain.TrabajoTerminado(trabajo.Estado) {
			return nil, domain.ErrTrabajoYaFinalizado
		}
		entrega.CadenaID = trabajo.CadenaID
	}
	if existente, existe := repositorio.Guardadas[entrega.ID]; existe {
		if existente.PayloadHash != entrega.PayloadHash {
			return nil, domain.ErrIdempotencia
		}
		copia := *existente
		return &copia, nil
	}
	nueva := *entrega
	nueva.Estado = domain.EntregaPendiente
	repositorio.Guardadas[nueva.ID] = &nueva
	copia := nueva
	return &copia, nil
}

// Adquirir entrega la pendiente indicada (o cualquiera) con una lease nueva.
func (repositorio *Entregas) Adquirir(_ context.Context, identificador *uuid.UUID) (*domain.Entrega, error) {
	if repositorio.LeaseOcupada {
		return nil, nil
	}
	for clave, entrega := range repositorio.Guardadas {
		if entrega.Estado != domain.EntregaPendiente || (identificador != nil && clave != *identificador) {
			continue
		}
		token := uuid.New()
		entrega.LeaseToken = &token
		entrega.Intentos++
		copia := *entrega
		return &copia, nil
	}
	return nil, nil
}

// FijarSucursal conserva la primera sucursal resuelta.
func (repositorio *Entregas) FijarSucursal(_ context.Context, identificador uuid.UUID, sucursalID int) (int, error) {
	entrega := repositorio.Guardadas[identificador]
	if entrega.SucursalID == nil {
		entrega.SucursalID = &sucursalID
	}
	return *entrega.SucursalID, nil
}

// Confirmar marca la entrega y suma el contador del trabajo.
func (repositorio *Entregas) Confirmar(_ context.Context, entrega *domain.Entrega, resultado domain.IngestaResultadoDTO) error {
	if repositorio.ErrorConfirmar != nil {
		return repositorio.ErrorConfirmar
	}
	codificado, errorObtenido := json.Marshal(resultado)
	if errorObtenido != nil {
		return errorObtenido
	}
	texto := string(codificado)
	guardada := repositorio.Guardadas[entrega.ID]
	guardada.Estado, guardada.Resultado = domain.EntregaConfirmada, &texto
	if entrega.TrabajoID != nil {
		repositorio.Trabajos.Guardados[*entrega.TrabajoID].ElementosExtraidos += resultado.Insertados + resultado.Actualizados
	}
	return nil
}

// Reprogramar registra el código y deja la entrega pendiente o fallida.
func (repositorio *Entregas) Reprogramar(_ context.Context, entrega *domain.Entrega, codigo string, permanente bool) error {
	repositorio.Reprogramaciones = append(repositorio.Reprogramaciones, Reprogramacion{Codigo: codigo, Permanente: permanente})
	if permanente {
		repositorio.Guardadas[entrega.ID].Estado = domain.EntregaFallida
	}
	return nil
}

// ===== Ejecuciones =====

// Ejecuciones implementa domain.EjecucionRepository y domain.PublicadorTrabajos.
type Ejecuciones struct {
	mutex         sync.Mutex
	Registradas   []domain.Ejecucion
	Publicadas    []domain.Ejecucion
	ErrorPublicar error
}

// RegistrarEjecucion guarda la ejecución.
func (repositorio *Ejecuciones) RegistrarEjecucion(_ context.Context, ejecucion domain.Ejecucion) error {
	repositorio.mutex.Lock()
	defer repositorio.mutex.Unlock()
	repositorio.Registradas = append(repositorio.Registradas, ejecucion)
	return nil
}

// Pendientes devuelve las ejecuciones registradas.
func (repositorio *Ejecuciones) Pendientes(context.Context) ([]domain.Ejecucion, error) {
	repositorio.mutex.Lock()
	defer repositorio.mutex.Unlock()
	return append([]domain.Ejecucion(nil), repositorio.Registradas...), nil
}

// PosponerPublicacion no hace nada: el doble no modela tiempos.
func (repositorio *Ejecuciones) PosponerPublicacion(context.Context, uuid.UUID) error { return nil }

// Publicar registra la ejecución o devuelve ErrorPublicar.
func (repositorio *Ejecuciones) Publicar(_ context.Context, ejecucion domain.Ejecucion) error {
	repositorio.mutex.Lock()
	defer repositorio.mutex.Unlock()
	if repositorio.ErrorPublicar != nil {
		return repositorio.ErrorPublicar
	}
	repositorio.Publicadas = append(repositorio.Publicadas, ejecucion)
	return nil
}
