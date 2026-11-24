// Rutas internas /internal/v1: token de servicio, cuerpo JSON estricto y
// handlers de ingesta (Control) y disponibilidad (Listas e IA). Responden
// {"error": "<código>"} y nunca pasan por Gateway.

package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ===== Constantes =====

// Clases SQLSTATE de datos inválidos (22) y violación de restricciones (23):
// el mismo documento no mejora reintentando, así que se responden como 400.
// Conexión, deadlock y fallos internos siguen siendo 500.
const (
	claseSQLDatosInvalidos = "22"
	claseSQLRestriccion    = "23"
)

// ===== Token de servicio =====

// exigirTokenServicio compara el Bearer en tiempo constante y rechaza toda
// petición de navegador (Origin) antes de llegar al handler. No usa JWT.
func exigirTokenServicio(token string) gin.HandlerFunc {
	esperado := sha256.Sum256([]byte("Bearer " + token))
	return func(contexto *gin.Context) {
		if contexto.GetHeader("Origin") != "" {
			contexto.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "internal_origin_forbidden"})
			return
		}
		recibido := sha256.Sum256([]byte(contexto.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(recibido[:], esperado[:]) != 1 {
			contexto.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "internal_unauthorized"})
			return
		}
		contexto.Next()
	}
}

// decodificarCuerpo lee un único objeto JSON estricto, sin campos desconocidos
// ni contenido extra, y corta el cuerpo en maximoBytes (413).
func decodificarCuerpo(contexto *gin.Context, destino any, maximoBytes int) bool {
	contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, int64(maximoBytes))
	decodificador := json.NewDecoder(contexto.Request.Body)
	decodificador.DisallowUnknownFields()
	errorDecodificacion := decodificador.Decode(destino)
	if errorDecodificacion == nil {
		var contenidoExtra any
		if decodificador.Decode(&contenidoExtra) != io.EOF {
			errorDecodificacion = errors.New("contenido JSON extra")
		}
	}
	if errorDecodificacion == nil {
		return true
	}
	estado := http.StatusBadRequest
	var errorTamano *http.MaxBytesError
	if errors.As(errorDecodificacion, &errorTamano) {
		estado = http.StatusRequestEntityTooLarge
	}
	contexto.AbortWithStatusJSON(estado, gin.H{"error": "invalid_payload"})
	return false
}

// ===== Ingesta =====

// registrarIngesta monta la resolución de cadenas y sucursales y la ingesta de
// lotes que usa Control de scraping.
func registrarIngesta(grupo *gin.RouterGroup, dependencias Dependencias) {
	repositorio, registrador := dependencias.Ingesta, dependencias.Registrador
	fallar := func(contexto *gin.Context, errorIngesta error) {
		registrador.Error("catalogo_ingesta_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"), "error", errorIngesta)
		var errorSQL interface{ SQLState() string }
		if errors.As(errorIngesta, &errorSQL) {
			codigo := errorSQL.SQLState()
			if strings.HasPrefix(codigo, claseSQLDatosInvalidos) || strings.HasPrefix(codigo, claseSQLRestriccion) {
				contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_catalog_data"})
				return
			}
		}
		contexto.JSON(http.StatusInternalServerError, gin.H{"error": "catalogo_internal_error"})
	}

	grupo.POST("/cadenas/resolver", func(contexto *gin.Context) {
		var solicitud struct {
			CadenaID int    `json:"cadena_id"`
			Nombre   string `json:"nombre"`
		}
		if !decodificarCuerpo(contexto, &solicitud, dependencias.MaximoBytesCuerpo) {
			return
		}
		if solicitud.CadenaID <= 0 && strings.TrimSpace(solicitud.Nombre) == "" {
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_chain"})
			return
		}
		cadena, errorCadena := repositorio.ObtenerCadena(contexto.Request.Context(), solicitud.CadenaID, solicitud.Nombre)
		if errorCadena != nil {
			fallar(contexto, errorCadena)
			return
		}
		if cadena == nil {
			contexto.JSON(http.StatusNotFound, gin.H{"error": "chain_not_found"})
			return
		}
		contexto.JSON(http.StatusOK, cadena)
	})

	grupo.POST("/sucursales/resolver", func(contexto *gin.Context) {
		var solicitud struct {
			CadenaID       int     `json:"cadena_id"`
			SucursalID     *int    `json:"sucursal_id"`
			CodigoSucursal *string `json:"codigo_sucursal"`
		}
		if !decodificarCuerpo(contexto, &solicitud, dependencias.MaximoBytesCuerpo) {
			return
		}
		sucursal, errorSucursal := repositorio.ObtenerSucursal(contexto.Request.Context(), solicitud.CadenaID, solicitud.SucursalID, solicitud.CodigoSucursal)
		if errorSucursal != nil {
			fallar(contexto, errorSucursal)
			return
		}
		if sucursal == nil {
			contexto.JSON(http.StatusNotFound, gin.H{"error": "branch_not_found"})
			return
		}
		contexto.JSON(http.StatusOK, sucursal)
	})

	grupo.POST("/ingestas", func(contexto *gin.Context) {
		var lote domain.LoteIngestaDTO
		if !decodificarCuerpo(contexto, &lote, dependencias.MaximoBytesCuerpo) {
			return
		}
		if lote.SucursalID <= 0 || len(lote.Productos) < 1 || len(lote.Productos) > dependencias.MaximoProductosPorLote {
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_batch"})
			return
		}
		if lote.LoteID != "" {
			identificador, errorParseo := uuid.Parse(lote.LoteID)
			if errorParseo != nil {
				contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_batch_id"})
				return
			}
			lote.LoteID = identificador.String()
		}
		resultado, errorLote := repositorio.IngestarLote(contexto.Request.Context(), lote)
		if errorLote != nil {
			if errors.Is(errorLote, domain.ErrIdempotencia) {
				contexto.JSON(http.StatusConflict, gin.H{"error": "idempotency_conflict"})
				return
			}
			fallar(contexto, errorLote)
			return
		}
		registrador.Info("catalogo_batch_confirmed", "lote_id", lote.LoteID, "request_id", contexto.Writer.Header().Get("X-Request-ID"), "sucursal_id", lote.SucursalID)
		contexto.JSON(http.StatusOK, resultado)
	})
}

// ===== Disponibilidad y equivalencias =====

// registrarDisponibilidad monta las consultas de sólo lectura de Listas, IA y
// Rutas, con un token distinto del de ingesta. Cada consulta se registra sólo si
// su servicio fue cableado.
func registrarDisponibilidad(grupo *gin.RouterGroup, dependencias Dependencias) {
	if dependencias.Disponibilidad != nil {
		registrarConsultaDisponibilidad(grupo, dependencias)
	}
	if dependencias.Equivalencias != nil {
		registrarConsultaEquivalencia(grupo, dependencias)
	}
}

// registrarConsultaDisponibilidad atiende la verificación de stock y precio de
// Listas e IA.
func registrarConsultaDisponibilidad(grupo *gin.RouterGroup, dependencias Dependencias) {
	servicio, registrador := dependencias.Disponibilidad, dependencias.Registrador
	grupo.POST("/productos/disponibilidad", func(contexto *gin.Context) {
		var solicitud domain.ConsultaDisponibilidadDTO
		if !decodificarCuerpo(contexto, &solicitud, dependencias.MaximoBytesCuerpo) {
			return
		}
		respuesta, errorConsulta := servicio.Consultar(contexto.Request.Context(), solicitud.ProductoIDs)
		switch {
		case errorConsulta == nil:
			contexto.JSON(http.StatusOK, respuesta)
		case errors.Is(errorConsulta, services.ErrLoteDisponibilidadVacio), errors.Is(errorConsulta, services.ErrLoteDisponibilidadExcesivo):
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_batch"})
		case errors.Is(errorConsulta, services.ErrIDProductoDisponibilidad):
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_id"})
		default:
			registrador.Error("catalogo_disponibilidad_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"), "error", errorConsulta)
			contexto.JSON(http.StatusInternalServerError, gin.H{"error": "catalogo_internal_error"})
		}
	})
}

// registrarConsultaEquivalencia atiende la agrupación por EAN que usa el motor
// de rutas para comparar el mismo producto entre sucursales.
func registrarConsultaEquivalencia(grupo *gin.RouterGroup, dependencias Dependencias) {
	servicio, registrador := dependencias.Equivalencias, dependencias.Registrador
	grupo.POST("/productos/equivalencias", func(contexto *gin.Context) {
		var solicitud domain.ConsultaEquivalenciaDTO
		if !decodificarCuerpo(contexto, &solicitud, dependencias.MaximoBytesCuerpo) {
			return
		}
		respuesta, errorConsulta := servicio.Consultar(contexto.Request.Context(), solicitud.ProductoIDs)
		switch {
		case errorConsulta == nil:
			contexto.JSON(http.StatusOK, respuesta)
		case errors.Is(errorConsulta, services.ErrLoteEquivalenciaVacio), errors.Is(errorConsulta, services.ErrLoteEquivalenciaExcesivo):
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_batch"})
		case errors.Is(errorConsulta, services.ErrIDProductoEquivalencia):
			contexto.JSON(http.StatusBadRequest, gin.H{"error": "invalid_product_id"})
		default:
			registrador.Error("catalogo_equivalencia_error", "request_id", contexto.Writer.Header().Get("X-Request-ID"), "error", errorConsulta)
			contexto.JSON(http.StatusInternalServerError, gin.H{"error": "catalogo_internal_error"})
		}
	})
}
