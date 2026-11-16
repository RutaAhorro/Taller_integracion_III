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

// ===== Registro durable de lotes =====

// ingestarDurable registra el lote una sola vez y lo procesa bajo una lease.
func (servicio *scraperService) ingestarDurable(contexto context.Context, trabajoID *string, entrada domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
	if len(entrada.Productos) == 0 {
		return nil, ErrLoteVacio
	}
	if len(entrada.Productos) > 1000 {
		return nil, ErrLoteExcedeMaximo
	}
	identificador, errorObtenido := uuid.Parse(entrada.LoteID)
	if errorObtenido != nil {
		return nil, ErrUUIDInvalido
	}
	var trabajoUUID *uuid.UUID
	if trabajoID != nil {
		analizado, errorObtenido := uuid.Parse(strings.TrimSpace(*trabajoID))
		if errorObtenido != nil {
			return nil, ErrUUIDInvalido
		}
		trabajoUUID = &analizado
	}
	datos, errorObtenido := json.Marshal(entrada)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	hashDatos, errorObtenido := json.Marshal(struct {
		TrabajoID *uuid.UUID
		Input     domain.IngestaLoteDTO
	}{trabajoUUID, entrada})
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	huella := sha256.Sum256(hashDatos)
	entrega, errorObtenido := servicio.entregas.Registrar(contexto, &domain.Entrega{ID: identificador, TrabajoID: trabajoUUID, Payload: string(datos), PayloadHash: hex.EncodeToString(huella[:])})
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if entrega.Estado == "confirmada" {
		var resultado domain.IngestaResultadoDTO
		errorObtenido = json.Unmarshal([]byte(*entrega.Resultado), &resultado)
		return &resultado, errorObtenido
	}
	if entrega.Estado == "fallida" {
		return nil, ErrEntregaFallida
	}
	entrega, errorObtenido = servicio.entregas.Adquirir(contexto, &identificador)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if entrega == nil {
		return nil, ErrEntregaPendiente
	}
	return servicio.entregar(contexto, entrega)
}

// ===== Resolución, entrega y reintentos =====

// entregar resuelve la sucursal, ingesta en Catálogo y confirma o reprograma el lote.
func (servicio *scraperService) entregar(contexto context.Context, entrega *domain.Entrega) (*domain.IngestaResultadoDTO, error) {
	contexto = domain.ConLoteID(contexto, entrega.ID.String())
	var entrada domain.IngestaLoteDTO
	var resultado *domain.IngestaResultadoDTO
	errorObtenido := json.Unmarshal([]byte(entrega.Payload), &entrada)
	if errorObtenido == nil && entrega.SucursalID == nil {
		cadenaObtenida := entrega.CadenaID
		if cadenaObtenida == 0 && entrada.Supermercado != nil {
			var cadena *domain.CadenaSupermercado
			cadena, errorObtenido = servicio.ingesta.ObtenerCadena(contexto, 0, *entrada.Supermercado)
			if errorObtenido == nil && cadena != nil {
				cadenaObtenida = cadena.ID
			}
		}
		if errorObtenido == nil {
			var sucursal *domain.SucursalSupermercado
			sucursal, errorObtenido = servicio.ingesta.ObtenerSucursal(contexto, cadenaObtenida, entrada.SucursalID, entrada.CodigoSucursal)
			if errorObtenido == nil && sucursal == nil {
				errorObtenido = ErrSucursalNoEncontrada
			}
			if errorObtenido == nil {
				var identificador int
				identificador, errorObtenido = servicio.entregas.FijarSucursal(contexto, entrega.ID, sucursal.ID)
				entrega.SucursalID = &identificador
			}
		}
	}
	if errorObtenido == nil {
		resultado, errorObtenido = servicio.ingesta.IngestarLote(contexto, entrega.TrabajoID, *entrega.SucursalID, entrada.Productos)
	}
	confirmado := errorObtenido == nil
	if confirmado {
		errorObtenido = servicio.entregas.Confirmar(contexto, entrega, *resultado)
		if errorObtenido == nil {
			return resultado, nil
		}
		errorObtenido = ErrContadorPendiente
	}
	permanente := errors.Is(errorObtenido, ErrSucursalNoEncontrada) || errors.Is(errorObtenido, ErrIdempotencia) || errors.Is(errorObtenido, ErrCatalogoRechazo)
	codigo := "catalogo_unavailable"
	if confirmado {
		codigo = "contador_pendiente"
	}
	if permanente {
		codigo = "lote_rechazado"
	}
	// No usar el contexto cancelado de una petición para liberar/reprogramar.
	contextoReintento, cancelar := context.WithTimeout(context.WithoutCancel(contexto), 2*time.Second)
	defer cancelar()
	if fallo := servicio.entregas.Reprogramar(contextoReintento, entrega, codigo, permanente); fallo != nil {
		slog.Error("ingesta_reprogramar_error", "lote_id", entrega.ID, "error", fallo)
	}
	slog.Warn("ingesta_pendiente", "lote_id", entrega.ID, "trabajo_id", entrega.TrabajoID, "intento", entrega.Intentos, "codigo", codigo)
	return nil, errorObtenido
}

// Recuperar entrega una entrada por tick; las leases permiten varias API.
func (servicio *scraperService) Recuperar(contexto context.Context) {
	if servicio.entregas == nil {
		return
	}
	entrega, errorObtenido := servicio.entregas.Adquirir(contexto, nil)
	if errorObtenido != nil {
		slog.Error("ingesta_recuperacion_error", "error", errorObtenido)
		return
	}
	if entrega != nil {
		_, _ = servicio.entregar(contexto, entrega)
	}
}
