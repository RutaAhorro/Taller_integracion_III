// Package controlscraping implementa el cliente HTTP del worker contra
// Control de Scraping con contrato de idempotencia para la entrega de lotes.
package controlscraping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/normalize"
)

// ===== Errores del Contrato =====

// Error representa un rechazo de la API. Permanent indica que reintentar el
// mismo lote no va a cambiar el resultado.
type Error struct {
	Message string
	Status  int
	Code    string
	Err     error
}

// Error implementa error y muestra el código del contrato cuando existe.
func (errorEstado *Error) Error() string {
	if errorEstado.Code != "" {
		return fmt.Sprintf("%s (status=%d, codigo=%s)", errorEstado.Message, errorEstado.Status, errorEstado.Code)
	}
	return fmt.Sprintf("%s (status=%d)", errorEstado.Message, errorEstado.Status)
}

// Unwrap devuelve el error de red subyacente, si lo hay.
func (errorEstado *Error) Unwrap() error {
	return errorEstado.Err
}

// Permanent indica si reintentar el mismo lote no cambiará el resultado.
func (errorEstado *Error) Permanent() bool {
	switch errorEstado.Status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusRequestEntityTooLarge:
		return true
	}
	switch errorEstado.Code {
	case "idempotency_conflict", "entrega_fallida", "entregas_fallidas", "lote_rechazado":
		return true
	}
	return false
}

// ===== Estructura del Cliente =====

// Client entrega lotes y consulta trabajos en Control de Scraping. No sigue
// redirecciones y es seguro para uso concurrente.
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// ===== Constructor =====

// New construye el cliente con el endpoint de ingesta, el token Bearer y el
// timeout de cada llamada.
func New(puntoAcceso, token string, tiempoEspera time.Duration) *Client {
	return &Client{
		endpoint: strings.TrimRight(puntoAcceso, "/"),
		token:    token,
		http: &http.Client{
			Timeout: tiempoEspera,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ===== Ingesta de Lotes =====

// ingestPayload define el cuerpo de la petición de ingesta hacia control de scraping.
type ingestPayload struct {
	Productos      []domain.Product `json:"productos"`
	Supermercado   *string          `json:"supermercado,omitempty"`
	SucursalID     *int             `json:"sucursal_id,omitempty"`
	CodigoSucursal string           `json:"codigo_sucursal,omitempty"`
}

// IngestBatch envía un lote y valida el recibo cuando se usa una clave de
// idempotencia, igual que ScraperAPIClient.ingest_batch.
func (cliente *Client) IngestBatch(
	contexto context.Context,
	productos []domain.Product,
	supermercado *string,
	trabajoID string,
	loteID string,
) (map[string]any, error) {
	if len(productos) == 0 {
		return map[string]any{"total_recibidos": 0}, nil
	}

	carga := ingestPayload{Productos: limitProducts(productos)}
	if supermercado != nil && *supermercado != "" {
		carga.Supermercado = supermercado
	}
	cuerpo, errorObtenido := json.Marshal(carga)
	if errorObtenido != nil {
		return nil, fmt.Errorf("serializar lote: %w", errorObtenido)
	}

	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.ingestionEndpoint(trabajoID), bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("Accept", "application/json")
	peticion.Header.Set("Content-Type", "application/json")
	if cliente.token != "" {
		peticion.Header.Set("Authorization", "Bearer "+cliente.token)
	}
	if loteID != "" {
		peticion.Header.Set("Idempotency-Key", loteID)
	}

	cuerpoRespuesta, errorObtenido := cliente.do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if len(cuerpoRespuesta) == 0 {
		if loteID != "" {
			return nil, &Error{Message: "La API no confirmó el lote"}
		}
		return map[string]any{}, nil
	}

	var resultado map[string]any
	if errorObtenido := json.Unmarshal(cuerpoRespuesta, &resultado); errorObtenido != nil {
		return nil, &Error{Message: "La API respondió con JSON inválido"}
	}
	if resultado["codigo"] == "entrega_pendiente" {
		return nil, &Error{
			Message: "Entrega pendiente de confirmación",
			Status:  http.StatusAccepted,
			Code:    "entrega_pendiente",
		}
	}
	if loteID != "" {
		total, valido := asInt(resultado["total_recibidos"])
		if !valido || total != len(productos) {
			return nil, &Error{Message: "La API devolvió un recibo de lote inconsistente"}
		}
		registrado, validoRegistrado := asInt(resultado["precios_registrados"])
		insertado, validoInsertado := asInt(resultado["insertados"])
		actualizado, validoActualizado := asInt(resultado["actualizados"])
		if !validoRegistrado || !validoInsertado || !validoActualizado || registrado != insertado+actualizado {
			return nil, &Error{Message: "La API devolvió un recibo de lote inconsistente"}
		}
	}
	return resultado, nil
}

// ===== Cierre y Consulta de Trabajos =====

// FinalizeWork cierra un trabajo y exige que la API confirme id y estado.
func (cliente *Client) FinalizeWork(
	contexto context.Context,
	trabajoID string,
	estado string,
	elementosExtraidos *int,
	respuestasBloqueadas *int,
	mensajeError string,
) (map[string]any, error) {
	carga := map[string]any{"estado": estado}
	if elementosExtraidos != nil {
		carga["elementos_extraidos"] = *elementosExtraidos
	}
	if respuestasBloqueadas != nil {
		carga["respuestas_bloqueadas"] = *respuestasBloqueadas
	}
	if mensajeError != "" {
		carga["registro_errores"] = mensajeError
	}
	cuerpo, errorObtenido := json.Marshal(carga)
	if errorObtenido != nil {
		return nil, errorObtenido
	}

	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPut, cliente.base()+"/trabajos/"+trabajoID+"/finalizar", bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("Accept", "application/json")
	peticion.Header.Set("Content-Type", "application/json")
	if cliente.token != "" {
		peticion.Header.Set("Authorization", "Bearer "+cliente.token)
	}

	cuerpoRespuesta, errorObtenido := cliente.do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	var resultado map[string]any
	if errorObtenido := json.Unmarshal(cuerpoRespuesta, &resultado); errorObtenido != nil {
		return nil, &Error{Message: "La API respondió con JSON inválido al cerrar el trabajo"}
	}
	if fmt.Sprint(resultado["id"]) != trabajoID || resultado["estado"] != estado {
		return nil, &Error{Message: "La API no confirmó el cierre del trabajo"}
	}
	return resultado, nil
}

// GetWork consulta el estado de un trabajo y valida su consistencia.
func (cliente *Client) GetWork(contexto context.Context, trabajoID string) (map[string]any, error) {
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodGet, cliente.base()+"/trabajos/"+trabajoID, nil)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("Accept", "application/json")
	if cliente.token != "" {
		peticion.Header.Set("Authorization", "Bearer "+cliente.token)
	}

	cuerpo, errorObtenido := cliente.do(peticion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	var resultado map[string]any
	if errorObtenido := json.Unmarshal(cuerpo, &resultado); errorObtenido != nil {
		return nil, &Error{Message: "No se pudo consultar trabajo"}
	}
	if fmt.Sprint(resultado["id"]) != trabajoID {
		return nil, &Error{Message: "Estado de trabajo inconsistente"}
	}
	switch resultado["estado"] {
	case "en_progreso", "completado", "fallido":
		return resultado, nil
	default:
		return nil, &Error{Message: "Estado de trabajo inconsistente"}
	}
}

// ===== Transporte HTTP =====

// do realiza peticiones HTTP seguras hacia la API de control de scraping.
func (cliente *Client) do(peticion *http.Request) ([]byte, error) {
	respuesta, errorObtenido := cliente.http.Do(peticion)
	if errorObtenido != nil {
		return nil, &Error{Message: "No se pudo conectar con la API", Err: errorObtenido}
	}
	defer respuesta.Body.Close()

	cuerpo, errorLectura := io.ReadAll(io.LimitReader(respuesta.Body, 4<<20))
	if errorLectura != nil {
		return nil, &Error{Message: "No se pudo leer la respuesta de la API", Err: errorLectura}
	}
	if respuesta.StatusCode >= http.StatusBadRequest {
		codigo := ""
		var detalle map[string]any
		if json.Unmarshal(cuerpo, &detalle) == nil {
			if valor, valido := detalle["codigo"].(string); valido {
				codigo = valor
			}
		}
		return nil, &Error{
			Message: "La API rechazó la solicitud",
			Status:  respuesta.StatusCode,
			Code:    codigo,
		}
	}
	return cuerpo, nil
}

// ===== Rutas del Contrato =====

// base retorna la URL base limpia de la API de control de scraping.
func (cliente *Client) base() string {
	return strings.TrimSuffix(cliente.endpoint, "/productos")
}

// ingestionEndpoint resuelve el endpoint completo para el envío de lotes.
func (cliente *Client) ingestionEndpoint(trabajoID string) string {
	if trabajoID != "" {
		return cliente.base() + "/trabajos/" + trabajoID + "/productos"
	}
	if strings.HasSuffix(cliente.endpoint, "/productos") {
		return cliente.endpoint
	}
	return cliente.endpoint + "/productos"
}

// ===== Utilidades =====

// limitProducts ajusta los campos a los límites de scraper.productos_crudos.
// CompleteItem ya los recorta antes de guardarlos en el spool; este paso queda
// como defensa para lotes reenviados o construidos fuera del worker.
func limitProducts(productos []domain.Product) []domain.Product {
	limitado := make([]domain.Product, len(productos))
	for indice, producto := range productos {
		limitado[indice] = normalize.LimitarProducto(producto)
	}
	return limitado
}

// asInt convierte valores genéricos numéricos al tipo int.
func asInt(valor any) (int, bool) {
	numero, valido := valor.(float64)
	if !valido || math.Trunc(numero) != numero {
		return 0, false
	}
	return int(numero), true
}
