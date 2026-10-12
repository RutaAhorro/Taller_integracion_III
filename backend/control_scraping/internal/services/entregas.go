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

func (s *scraperService) ingestarDurable(ctx context.Context, trabajoID *string, input domain.IngestaLoteDTO) (*domain.IngestaResultadoDTO, error) {
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
	e, err := s.entregas.Registrar(ctx, &domain.Entrega{ID: id, TrabajoID: jobID, Payload: string(data), PayloadHash: hex.EncodeToString(hash[:])})
	if err != nil {
		return nil, err
	}
	if e.Estado == "confirmada" {
		var result domain.IngestaResultadoDTO
		err = json.Unmarshal([]byte(*e.Resultado), &result)
		return &result, err
	}
	if e.Estado == "fallida" {
		return nil, ErrEntregaFallida
	}
	e, err = s.entregas.Adquirir(ctx, &id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrEntregaPendiente
	}
	return s.entregar(ctx, e)
}

func (s *scraperService) entregar(ctx context.Context, e *domain.Entrega) (*domain.IngestaResultadoDTO, error) {
	ctx = domain.ConLoteID(ctx, e.ID.String())
	var input domain.IngestaLoteDTO
	var result *domain.IngestaResultadoDTO
	err := json.Unmarshal([]byte(e.Payload), &input)
	if err == nil && e.SucursalID == nil {
		chain := e.CadenaID
		if chain == 0 && input.Supermercado != nil {
			var c *domain.CadenaSupermercado
			c, err = s.ingesta.ObtenerCadena(ctx, 0, *input.Supermercado)
			if err == nil && c != nil {
				chain = c.ID
			}
		}
		if err == nil {
			var branch *domain.SucursalSupermercado
			branch, err = s.ingesta.ObtenerSucursal(ctx, chain, input.SucursalID, input.CodigoSucursal)
			if err == nil && branch == nil {
				err = ErrSucursalNoEncontrada
			}
			if err == nil {
				var id int
				id, err = s.entregas.FijarSucursal(ctx, e.ID, branch.ID)
				e.SucursalID = &id
			}
		}
	}
	if err == nil {
		result, err = s.ingesta.IngestarLote(ctx, e.TrabajoID, *e.SucursalID, input.Productos)
	}
	confirmed := err == nil
	if confirmed {
		err = s.entregas.Confirmar(ctx, e, *result)
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
	if failure := s.entregas.Reprogramar(retryCtx, e, code, permanent); failure != nil {
		slog.Error("ingesta_reprogramar_error", "lote_id", e.ID, "error", failure)
	}
	slog.Warn("ingesta_pendiente", "lote_id", e.ID, "trabajo_id", e.TrabajoID, "intento", e.Intentos, "codigo", code)
	return nil, err
}

// Recuperar entrega una entrada por tick; las leases permiten varias API.
func (s *scraperService) Recuperar(ctx context.Context) {
	if s.entregas == nil {
		return
	}
	e, err := s.entregas.Adquirir(ctx, nil)
	if err != nil {
		slog.Error("ingesta_recuperacion_error", "error", err)
		return
	}
	if e != nil {
		_, _ = s.entregar(ctx, e)
	}
}
