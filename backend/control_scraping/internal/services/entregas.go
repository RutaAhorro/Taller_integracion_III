// Entrega durable de lotes a Catálogo: cada lote se registra una vez en SQL
// con su clave idempotente, se procesa bajo una lease y se confirma o se
// reprograma. La recuperación reintenta los lotes pendientes en segundo plano.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// plazoReprogramacion acota la liberación de la lease aunque la petición
// original ya se haya cancelado.
const plazoReprogramacion = 2 * time.Second

// ===== Servicio de entregas =====

// ServicioEntregas registra lotes y los entrega a Catálogo.
type ServicioEntregas struct {
	entregas domain.EntregaRepository
	catalogo domain.CatalogoRepository
}

// NuevoServicioEntregas compone el servicio con el ledger SQL y Catálogo.
func NuevoServicioEntregas(entregas domain.EntregaRepository, catalogo domain.CatalogoRepository) *ServicioEntregas {
	return &ServicioEntregas{entregas: entregas, catalogo: catalogo}
}

// ===== Registro de lotes =====

// IngestarProductos registra el lote una sola vez y lo entrega si obtiene la
// lease. Un lote ya confirmado devuelve el resultado guardado; uno en curso
// en otro proceso responde ErrEntregaPendiente para que el cliente reintente.
func (servicio *ServicioEntregas) IngestarProductos(contexto context.Context, trabajoID *string, entrada domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
	if len(entrada.Productos) == 0 {
		return nil, domain.ErrLoteVacio
	}
	if len(entrada.Productos) > domain.MaximoProductosPorLote {
		return nil, domain.ErrLoteExcedeMaximo
	}
	loteID, errorObtenido := uuid.Parse(entrada.LoteID)
	if errorObtenido != nil {
		return nil, domain.ErrUUIDInvalido
	}
	var trabajoUUID *uuid.UUID
	if trabajoID != nil {
		analizado, errorObtenido := uuid.Parse(strings.TrimSpace(*trabajoID))
		if errorObtenido != nil {
			return nil, domain.ErrUUIDInvalido
		}
		trabajoUUID = &analizado
	}
	datos, errorObtenido := json.Marshal(entrada)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	// La huella incluye el trabajo: la misma clave con otro trabajo es un
	// conflicto. Los nombres de campo forman parte de las huellas ya guardadas.
	datosHuella, errorObtenido := json.Marshal(struct {
		TrabajoID *uuid.UUID
		Input     domain.IngestaLoteDTO
	}{trabajoUUID, entrada})
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	huella := sha256.Sum256(datosHuella)
	entrega, errorObtenido := servicio.entregas.Registrar(contexto, &domain.Entrega{ID: loteID, TrabajoID: trabajoUUID, Payload: string(datos), PayloadHash: hex.EncodeToString(huella[:])})
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	switch entrega.Estado {
	case domain.EntregaConfirmada:
		var resultado domain.IngestaResultadoDTO
		errorObtenido = json.Unmarshal([]byte(*entrega.Resultado), &resultado)
		return &resultado, errorObtenido
	case domain.EntregaFallida:
		return nil, domain.ErrEntregaFallida
	}
	entrega, errorObtenido = servicio.entregas.Adquirir(contexto, &loteID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if entrega == nil {
		return nil, domain.ErrEntregaPendiente
	}
	return servicio.entregar(contexto, entrega)
}

// ===== Entrega y reintentos =====

// entregar resuelve y fija la sucursal, ingesta en Catálogo y confirma el
// contador. Ante un fallo libera la lease con backoff o marca la entrega como
// fallida si el rechazo es permanente.
func (servicio *ServicioEntregas) entregar(contexto context.Context, entrega *domain.Entrega) (*domain.IngestaResultadoDTO, error) {
	contexto = domain.ConLoteID(contexto, entrega.ID.String())
	var entrada domain.IngestaLoteDTO
	var resultado *domain.IngestaResultadoDTO
	errorObtenido := json.Unmarshal([]byte(entrega.Payload), &entrada)
	if errorObtenido == nil && entrega.SucursalID == nil {
		errorObtenido = servicio.fijarSucursal(contexto, entrega, entrada)
	}
	if errorObtenido == nil {
		resultado, errorObtenido = servicio.catalogo.IngestarLote(contexto, entrega.TrabajoID, *entrega.SucursalID, entrada.Productos)
	}
	catalogoConfirmo := errorObtenido == nil
	if catalogoConfirmo {
		if errorObtenido = servicio.entregas.Confirmar(contexto, entrega, *resultado); errorObtenido == nil {
			return resultado, nil
		}
		errorObtenido = domain.ErrContadorPendiente
	}
	permanente := errors.Is(errorObtenido, domain.ErrSucursalNoEncontrada) || errors.Is(errorObtenido, domain.ErrIdempotencia) || errors.Is(errorObtenido, domain.ErrCatalogoRechazo)
	codigo := domain.CodigoCatalogoNoDisponible
	if catalogoConfirmo {
		codigo = domain.CodigoContadorPendiente
	}
	if permanente {
		codigo = domain.CodigoLoteRechazado
	}
	contextoReintento, cancelar := context.WithTimeout(context.WithoutCancel(contexto), plazoReprogramacion)
	defer cancelar()
	if fallo := servicio.entregas.Reprogramar(contextoReintento, entrega, codigo, permanente); fallo != nil {
		slog.Error("ingesta_reprogramar_error", "lote_id", entrega.ID, "error", fallo)
	}
	slog.Warn("ingesta_pendiente", "lote_id", entrega.ID, "trabajo_id", entrega.TrabajoID, "intento", entrega.Intentos, "codigo", codigo)
	return nil, errorObtenido
}

// fijarSucursal resuelve la sucursal en Catálogo. Sin trabajo, la cadena se
// resuelve por el nombre del supermercado del lote; con trabajo, la cadena es
// la del trabajo y el nombre del lote no se usa.
func (servicio *ServicioEntregas) fijarSucursal(contexto context.Context, entrega *domain.Entrega, entrada domain.IngestaLoteDTO) error {
	cadenaID := entrega.CadenaID
	if cadenaID == 0 && entrada.Supermercado != nil {
		cadena, errorObtenido := servicio.catalogo.ObtenerCadena(contexto, 0, *entrada.Supermercado)
		if errorObtenido != nil {
			return errorObtenido
		}
		if cadena != nil {
			cadenaID = cadena.ID
		}
	}
	sucursal, errorObtenido := servicio.catalogo.ObtenerSucursal(contexto, cadenaID, entrada.SucursalID, entrada.CodigoSucursal)
	if errorObtenido != nil {
		return errorObtenido
	}
	if sucursal == nil {
		return domain.ErrSucursalNoEncontrada
	}
	sucursalFijada, errorObtenido := servicio.entregas.FijarSucursal(contexto, entrega.ID, sucursal.ID)
	entrega.SucursalID = &sucursalFijada
	return errorObtenido
}

// RecuperarEntregas procesa una entrega pendiente por llamada; las leases
// permiten ejecutar la recuperación en varias réplicas a la vez.
func (servicio *ServicioEntregas) RecuperarEntregas(contexto context.Context) {
	entrega, errorObtenido := servicio.entregas.Adquirir(contexto, nil)
	if errorObtenido != nil {
		slog.Error("ingesta_recuperacion_error", "error", errorObtenido)
		return
	}
	if entrega != nil {
		_, _ = servicio.entregar(contexto, entrega)
	}
}
