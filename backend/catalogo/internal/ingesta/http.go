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
)

const MaxBodyBytes = 8 << 20

type Store interface {
	ObtenerCadena(context.Context, int, string) (*CadenaSupermercado, error)
	ObtenerSucursal(context.Context, int, *int, *string) (*SucursalSupermercado, error)
	IngestarLote(context.Context, Request) (*IngestaResultadoDTO, error)
}

type Config struct {
	Repository Store
	Token      string
}

// Register mantiene el contrato privado fuera de las rutas públicas y de JWT.
func Register(r *gin.Engine, cfg Config, logger *slog.Logger) {
	if cfg.Token == "" || cfg.Repository == nil {
		return
	} // modo lector cerrado
	if logger == nil {
		logger = slog.Default()
	}
	expected := sha256.Sum256([]byte("Bearer " + cfg.Token))
	group := r.Group("/internal/v1", func(c *gin.Context) {
		if c.GetHeader("Origin") != "" {
			c.AbortWithStatusJSON(403, gin.H{"error": "internal_origin_forbidden"})
			return
		}
		got := sha256.Sum256([]byte(c.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "internal_unauthorized"})
			return
		}
		c.Next()
	})
	fail := func(c *gin.Context, err error) {
		logger.Error("catalogo_ingesta_error", "request_id", c.Writer.Header().Get("X-Request-ID"), "error", err)
		c.JSON(500, gin.H{"error": "catalogo_internal_error"})
	}
	group.POST("/cadenas/resolver", func(c *gin.Context) {
		var input struct {
			CadenaID int    `json:"cadena_id"`
			Nombre   string `json:"nombre"`
		}
		if !decode(c, &input) {
			return
		}
		if input.CadenaID <= 0 && strings.TrimSpace(input.Nombre) == "" {
			c.JSON(400, gin.H{"error": "invalid_chain"})
			return
		}
		result, err := cfg.Repository.ObtenerCadena(c.Request.Context(), input.CadenaID, input.Nombre)
		if err != nil {
			fail(c, err)
			return
		}
		if result == nil {
			c.JSON(404, gin.H{"error": "chain_not_found"})
			return
		}
		c.JSON(200, result)
	})
	group.POST("/sucursales/resolver", func(c *gin.Context) {
		var input struct {
			CadenaID       int     `json:"cadena_id"`
			SucursalID     *int    `json:"sucursal_id"`
			CodigoSucursal *string `json:"codigo_sucursal"`
		}
		if !decode(c, &input) {
			return
		}
		if input.CadenaID <= 0 && (input.SucursalID == nil || *input.SucursalID <= 0) &&
			(input.CodigoSucursal == nil || strings.TrimSpace(*input.CodigoSucursal) == "") {
			c.JSON(404, gin.H{"error": "branch_not_found"})
			return
		}
		result, err := cfg.Repository.ObtenerSucursal(c.Request.Context(), input.CadenaID, input.SucursalID, input.CodigoSucursal)
		if err != nil {
			fail(c, err)
			return
		}
		if result == nil {
			c.JSON(404, gin.H{"error": "branch_not_found"})
			return
		}
		c.JSON(200, result)
	})
	group.POST("/ingestas", func(c *gin.Context) {
		var input Request
		if !decode(c, &input) {
			return
		}
		if input.SucursalID <= 0 || len(input.Productos) < 1 || len(input.Productos) > 1000 {
			c.JSON(400, gin.H{"error": "invalid_batch"})
			return
		}
		result, err := cfg.Repository.IngestarLote(c.Request.Context(), input)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func decode(c *gin.Context, input any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
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
		c.AbortWithStatusJSON(status, gin.H{"error": "invalid_payload"})
		return false
	}
	return true
}
