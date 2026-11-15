package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
	"log/slog"
	"strings"
	"time"
)

func (servicio *scraperService) ingestarDurable(ctx context.Context, trabajoID *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
	if len(input.Productos) == 0 {
		return nil, ErrLoteVacio
	}
	if len(input.Productos) > 1000 {
		return nil, ErrLoteExcedeMaximo
	}
	id, err := uuid.Parse(input.LoteID)
	if err != nil {
		return nil, ErrUUIDInvalido
	}
	var jobID *uuid.UUID
	if trabajoID != nil {
		parsed, err := uuid.Parse(strings.TrimSpace(*trabajoID))
		if err != nil {
			return nil, ErrUUIDInvalido
		}
		jobID = &parsed
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	hashData, err := json.Marshal(struct {
		TrabajoID *uuid.UUID
		Input     domain.IngestaLoteDTO
	}{jobID, input})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(hashData)
	entrega, err := servicio.entregas.Registrar(ctx, &domain.Entrega{ID: id, TrabajoID: jobID, Payload: string(data), PayloadHash: hex.EncodeToString(hash[:])})
	if err != nil {
		return nil, err
	}
	if entrega.Estado == "confirmada" {
		var result domain.IngestaResultadoDTO
		err = json.Unmarshal([]byte(*entrega.Resultado), &result)
		return &result, err
	}
	if entrega.Estado == "fallida" {
		return nil, ErrEntregaFallida
	}
	entrega, err = servicio.entregas.Adquirir(ctx, &id)
	if err != nil {
		return nil, err
	}
	if entrega == nil {
		return nil, ErrEntregaPendiente
	}
	return servicio.entregar(ctx, entrega)
}

func (servicio *scraperService) entregar(ctx context.Context, entrega *domain.Entrega) (*domain.IngestaResultadoDTO, error) {
	ctx = domain.ConLoteID(ctx, entrega.ID.String())
	var input domain.IngestaLoteDTO
	var result *domain.IngestaResultadoDTO
	err := json.Unmarshal([]byte(entrega.Payload), &input)
	if err == nil && entrega.SucursalID == nil {
		chain := entrega.CadenaID
		if chain == 0 && input.Supermercado != nil {
			var cadena *domain.CadenaSupermercado
			cadena, err = servicio.ingesta.ObtenerCadena(ctx, 0, *input.Supermercado)
			if err == nil && cadena != nil {
				chain = cadena.ID
			}
		}
		if err == nil {
			var branch *domain.SucursalSupermercado
			branch, err = servicio.ingesta.ObtenerSucursal(ctx, chain, input.SucursalID, input.CodigoSucursal)
			if err == nil && branch == nil {
				err = ErrSucursalNoEncontrada
			}
			if err == nil {
				var id int
				id, err = servicio.entregas.FijarSucursal(ctx, entrega.ID, branch.ID)
				entrega.SucursalID = &id
			}
		}
	}
	if err == nil {
		result, err = servicio.ingesta.IngestarLote(ctx, entrega.TrabajoID, *entrega.SucursalID, input.Productos)
	}
	confirmed := err == nil
	if confirmed {
		err = servicio.entregas.Confirmar(ctx, entrega, *result)
		if err == nil {
			return result, nil
		}
		err = ErrContadorPendiente
	}
	permanent := errors.Is(err, ErrSucursalNoEncontrada) || errors.Is(err, ErrIdempotencia) || errors.Is(err, ErrCatalogoRechazo)
	code := "catalogo_unavailable"
	if confirmed {
		code = "contador_pendiente"
	}
	if permanent {
		code = "lote_rechazado"
	}
	// No usar el contexto cancelado de una petición para liberar/reprogramar.
	retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if failure := servicio.entregas.Reprogramar(retryCtx, entrega, code, permanent); failure != nil {
		slog.Error("ingesta_reprogramar_error", "lote_id", entrega.ID, "error", failure)
	}
	slog.Warn("ingesta_pendiente", "lote_id", entrega.ID, "trabajo_id", entrega.TrabajoID, "intento", entrega.Intentos, "codigo", code)
	return nil, err
}

// Recuperar entrega una entrada por tick; las leases permiten varias API.
func (servicio *scraperService) Recuperar(ctx context.Context) {
	if servicio.entregas == nil {
		return
	}
	entrega, err := servicio.entregas.Adquirir(ctx, nil)
	if err != nil {
		slog.Error("ingesta_recuperacion_error", "error", err)
		return
	}
	if entrega != nil {
		_, _ = servicio.entregar(ctx, entrega)
	}
}
