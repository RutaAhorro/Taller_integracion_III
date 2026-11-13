package ingesta

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Store interface {
	ObtenerCadena(context.Context, int, string) (*CadenaSupermercado, error)
	ObtenerSucursal(context.Context, int, *int, *string) (*SucursalSupermercado, error)
	IngestarLote(context.Context, Request) (*IngestaResultadoDTO, error)
}

type Config struct {
	Repository Store
	Token      string
	// MaxProductos es el tope de productos por lote (CATALOGO_MAX_BATCH_PRODUCTOS).
	MaxProductos int
	// MaxBodyBytes es el tamaño máximo del cuerpo JSON en bytes (CATALOGO_MAX_BODY_BYTES).
	MaxBodyBytes int
}

// Register mantiene el contrato privado fuera de las rutas públicas y de JWT.
func Register(enrutador *gin.Engine, cfg Config, logger *slog.Logger) {
	if cfg.Token == "" || cfg.Repository == nil {
		return
	} // modo lector cerrado
	if cfg.MaxProductos < 1 {
		panic("ingesta.Config.MaxProductos debe ser positivo")
	}
	if cfg.MaxBodyBytes < 1 {
		panic("ingesta.Config.MaxBodyBytes debe ser positivo")
	}
	if logger == nil {
		logger = slog.Default()
	}
	expected := sha256.Sum256([]byte("Bearer " + cfg.Token))
	group := enrutador.Group("/internal/v1", func(contexto *gin.Context) {
		if contexto.GetHeader("Origin") != "" {
			contexto.AbortWithStatusJSON(403, gin.H{"error": "internal_origin_forbidden"})
			return
		}
		got := sha256.Sum256([]byte(contexto.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			contexto.AbortWithStatusJSON(401, gin.H{"error": "internal_unauthorized"})
			return
		}
		contexto.Next()
	})
	fail := func(contexto *gin.Context, err error) {
		logger.Error("catalogo_ingesta_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"), "error", err)
		var sqlError interface{ SQLState() string }
		if errors.As(err, &sqlError) {
			code := sqlError.SQLState()
			// Datos fuera de rango/formato y restricciones: el mismo documento
			// no mejora esperando. Conexión, deadlock y fallos internos siguen 500.
			if strings.HasPrefix(code, "22") || strings.HasPrefix(code, "23") {
				contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_catalog_data"})
				return
			}
		}
		contexto.JSON(500, gin.H{"error": "catalogo_internal_error"})
	}
	group.POST("/cadenas/resolver", func(contexto *gin.Context) {
		var input struct {
			CadenaID int    `json:"cadena_id"`
			Nombre   string `json:"nombre"`
		}
		if !decode(contexto, &input, cfg.MaxBodyBytes) {
			return
		}
		if input.CadenaID <= 0 && strings.TrimSpace(input.Nombre) == "" {
			contexto.JSON(400, gin.H{"error": "invalid_chain"})
			return
		}
		result, err := cfg.Repository.ObtenerCadena(contexto.Request.Context(), input.CadenaID, input.Nombre)
		if err != nil {
			fail(contexto, err)
			return
		}
		if result == nil {
			contexto.JSON(404, gin.H{"error": "chain_not_found"})
			return
		}
		contexto.JSON(200, result)
	})
	group.POST("/sucursales/resolver", func(contexto *gin.Context) {
		var input struct {
			CadenaID       int     `json:"cadena_id"`
			SucursalID     *int    `json:"sucursal_id"`
			CodigoSucursal *string `json:"codigo_sucursal"`
		}
		if !decode(contexto, &input, cfg.MaxBodyBytes) {
			return
		}
		if input.CadenaID <= 0 && (input.SucursalID == nil || *input.SucursalID <= 0) &&
			(input.CodigoSucursal == nil || strings.TrimSpace(*input.CodigoSucursal) == "") {
			contexto.JSON(404, gin.H{"error": "branch_not_found"})
			return
		}
		result, err := cfg.Repository.ObtenerSucursal(contexto.Request.Context(), input.CadenaID, input.SucursalID, input.CodigoSucursal)
		if err != nil {
			fail(contexto, err)
			return
		}
		if result == nil {
			contexto.JSON(404, gin.H{"error": "branch_not_found"})
			return
		}
		contexto.JSON(200, result)
	})
	group.POST("/ingestas", func(contexto *gin.Context) {
		var input Request
		if !decode(contexto, &input, cfg.MaxBodyBytes) {
			return
		}
		if input.SucursalID <= 0 || len(input.Productos) < 1 || len(input.Productos) > cfg.MaxProductos {
			contexto.JSON(400, gin.H{"error": "invalid_batch"})
			return
		}
		if input.LoteID != "" {
			id, err := uuid.Parse(input.LoteID)
			if err != nil {
				contexto.JSON(400, gin.H{"error": "invalid_batch_id"})
				return
			}
			input.LoteID = id.String()
		}
		result, err := cfg.Repository.IngestarLote(contexto.Request.Context(), input)
		if err != nil {
			if errors.Is(err, ErrIdempotencia) {
				contexto.JSON(409, gin.H{"error": "idempotency_conflict"})
				return
			}
			fail(contexto, err)
			return
		}
		logger.Info("catalogo_batch_confirmed", "lote_id", input.LoteID, "request_id", contexto.Writer.Header().Get("X-Request-ID"), "sucursal_id", input.SucursalID)
		contexto.JSON(http.StatusOK, result)
	})
}

// decode lee un único objeto JSON estricto y corta el cuerpo en maximoBytes
// para que un lote enorme no agote la memoria del proceso.
func decode(contexto *gin.Context, input any, maximoBytes int) bool {
	contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(maximoBytes))
	decoder := json.NewDecoder(contexto.Request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(input)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("trailing JSON")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		contexto.AbortWithStatusJSON(status, gin.H{"error": "invalid_payload"})
		return false
	}
	return true
}
