// Package controlscraping implementa el cliente HTTP del worker contra
// Control de Scraping, manteniendo el contrato de idempotencia del worker de
// Python.
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

// Error representa un rechazo de la API. Permanent indica que reintentar el
// mismo lote no va a cambiar el resultado.
type Error struct {
	Message string
	Status  int
	Code    string
	Err     error
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (status=%d, codigo=%s)", e.Message, e.Status, e.Code)
	}
	return fmt.Sprintf("%s (status=%d)", e.Message, e.Status)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Permanent() bool {
	switch e.Status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusRequestEntityTooLarge:
		return true
	}
	switch e.Code {
	case "idempotency_conflict", "entrega_fallida", "entregas_fallidas", "lote_rechazado":
		return true
	}
	return false
}

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

func New(endpoint, token string, timeout time.Duration) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		token:    token,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type ingestPayload struct {
	Productos      []domain.Product `json:"productos"`
	Supermercado   *string          `json:"supermercado,omitempty"`
	SucursalID     *int             `json:"sucursal_id,omitempty"`
	CodigoSucursal string           `json:"codigo_sucursal,omitempty"`
}

// IngestBatch envía un lote y valida el recibo cuando se usa una clave de
// idempotencia, igual que ScraperAPIClient.ingest_batch.
func (c *Client) IngestBatch(
	ctx context.Context,
	products []domain.Product,
	supermarket *string,
	workID string,
	batchID string,
) (map[string]any, error) {
	if len(products) == 0 {
		return map[string]any{"total_recibidos": 0}, nil
	}

	payload := ingestPayload{Productos: limitProducts(products)}
	if supermarket != nil && *supermarket != "" {
		payload.Supermercado = supermarket
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("serializar lote: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ingestionEndpoint(workID), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	if batchID != "" {
		request.Header.Set("Idempotency-Key", batchID)
	}

	responseBody, err := c.do(request)
	if err != nil {
		return nil, err
	}
	if len(responseBody) == 0 {
		if batchID != "" {
			return nil, &Error{Message: "La API no confirmó el lote"}
		}
		return map[string]any{}, nil
	}

	var result map[string]any
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, &Error{Message: "La API respondió con JSON inválido"}
	}
	if result["codigo"] == "entrega_pendiente" {
		return nil, &Error{
			Message: "Entrega pendiente de confirmación",
			Status:  http.StatusAccepted,
			Code:    "entrega_pendiente",
		}
	}
	if batchID != "" {
		total, ok := asInt(result["total_recibidos"])
		if !ok || total != len(products) {
			return nil, &Error{Message: "La API devolvió un recibo de lote inconsistente"}
		}
		registered, okRegistered := asInt(result["precios_registrados"])
		inserted, okInserted := asInt(result["insertados"])
		updated, okUpdated := asInt(result["actualizados"])
		if !okRegistered || !okInserted || !okUpdated || registered != inserted+updated {
			return nil, &Error{Message: "La API devolvió un recibo de lote inconsistente"}
		}
	}
	return result, nil
}

// FinalizeWork cierra un trabajo y exige que la API confirme id y estado.
func (c *Client) FinalizeWork(
	ctx context.Context,
	workID string,
	status string,
	extractedItems *int,
	blockedResponses *int,
	errorMessage string,
) (map[string]any, error) {
	payload := map[string]any{"estado": status}
	if extractedItems != nil {
		payload["elementos_extraidos"] = *extractedItems
	}
	if blockedResponses != nil {
		payload["respuestas_bloqueadas"] = *blockedResponses
	}
	if errorMessage != "" {
		payload["registro_errores"] = errorMessage
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base()+"/trabajos/"+workID+"/finalizar", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	responseBody, err := c.do(request)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, &Error{Message: "La API respondió con JSON inválido al cerrar el trabajo"}
	}
	if fmt.Sprint(result["id"]) != workID || result["estado"] != status {
		return nil, &Error{Message: "La API no confirmó el cierre del trabajo"}
	}
	return result, nil
}

// GetWork consulta el estado de un trabajo y valida su consistencia.
func (c *Client) GetWork(ctx context.Context, workID string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/trabajos/"+workID, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	body, err := c.do(request)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &Error{Message: "No se pudo consultar trabajo"}
	}
	if fmt.Sprint(result["id"]) != workID {
		return nil, &Error{Message: "Estado de trabajo inconsistente"}
	}
	switch result["estado"] {
	case "en_progreso", "completado", "fallido":
		return result, nil
	default:
		return nil, &Error{Message: "Estado de trabajo inconsistente"}
	}
}

func (c *Client) do(request *http.Request) ([]byte, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return nil, &Error{Message: "No se pudo conectar con la API", Err: err}
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if readErr != nil {
		return nil, &Error{Message: "No se pudo leer la respuesta de la API", Err: readErr}
	}
	if response.StatusCode >= http.StatusBadRequest {
		code := ""
		var detail map[string]any
		if json.Unmarshal(body, &detail) == nil {
			if value, ok := detail["codigo"].(string); ok {
				code = value
			}
		}
		return nil, &Error{
			Message: "La API rechazó la solicitud",
			Status:  response.StatusCode,
			Code:    code,
		}
	}
	return body, nil
}

func (c *Client) base() string {
	return strings.TrimSuffix(c.endpoint, "/productos")
}

func (c *Client) ingestionEndpoint(workID string) string {
	if workID != "" {
		return c.base() + "/trabajos/" + workID + "/productos"
	}
	if strings.HasSuffix(c.endpoint, "/productos") {
		return c.endpoint
	}
	return c.endpoint + "/productos"
}

// limitProducts ajusta los campos a los límites de scraper.productos_crudos.
// Los spiders pueden entregar descripciones comerciales más largas que el
// varchar del esquema; recortar en el límite de entrega evita que Catálogo
// rechace el lote completo.
func limitProducts(products []domain.Product) []domain.Product {
	limited := make([]domain.Product, len(products))
	for index, product := range products {
		product.Producto = normalize.Truncar(product.Producto, 255)
		product.SKU = normalize.TruncarPtr(product.SKU, 100)
		product.EanGtin = normalize.TruncarPtr(product.EanGtin, 50)
		product.Marca = normalize.TruncarPtr(product.Marca, 100)
		product.Categoria = normalize.Truncar(product.Categoria, 100)
		product.FormatoCrudo = normalize.TruncarPtr(product.FormatoCrudo, 100)
		product.MecanicaPromocion = normalize.TruncarPtr(product.MecanicaPromocion, 255)
		limited[index] = product
	}
	return limited
}

func asInt(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number {
		return 0, false
	}
	return int(number), true
}
