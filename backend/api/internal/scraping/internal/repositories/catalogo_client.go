package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping/internal/services"
	"github.com/google/uuid"
)

type catalogoClient struct {
	origin string
	token  string
	client *http.Client
}

func NewCatalogoClient(origin, token string, timeout time.Duration) (domain.IngestaRepository, error) {
	u, err := url.Parse(origin)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(origin, "?#\\ \t\r\n") {
		return nil, errors.New("CATALOGO_INTERNAL_URL debe ser un origen HTTP(S), sin credenciales, rutas ni parámetros")
	}
	if len(token) < 32 || strings.ContainsAny(token, " \t\r\n") {
		return nil, errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos 32 caracteres y no contener espacios")
	}
	if timeout <= 0 || timeout > 20*time.Second {
		return nil, errors.New("CATALOGO_INGEST_TIMEOUT debe ser positivo y no superar 20s")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 2 * time.Second
	transport.ResponseHeaderTimeout = timeout
	// Las tres operaciones son POST; ningún lote se reenvía automáticamente.
	transport.DisableKeepAlives = true
	transport.Protocols = new(http.Protocols)
	transport.Protocols.SetHTTP1(true)
	return &catalogoClient{origin: strings.TrimSuffix(origin, "/"), token: token, client: &http.Client{
		Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (r *catalogoClient) call(ctx context.Context, path string, payload, result any) (bool, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.origin+"/internal/v1/"+path, bytes.NewReader(data))
	if err != nil {
		return false, services.ErrCatalogoNoDisponible
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.token)
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		req.Header.Set("X-Request-ID", id)
	}
	response, err := r.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return false, services.ErrCatalogoTimeout
		}
		return false, services.ErrCatalogoNoDisponible
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, services.ErrCatalogoNoDisponible
	}
	// No se propagan errores HTTP/SQL ni secretos del upstream al cliente público.
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 || json.Unmarshal(body, result) != nil {
		return false, services.ErrCatalogoNoDisponible
	}
	return true, nil
}

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func (r *catalogoClient) ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	var result domain.CadenaSupermercado
	found, err := r.call(ctx, "cadenas/resolver", map[string]any{"cadena_id": cadenaID, "nombre": nombre}, &result)
	if err != nil || !found {
		return nil, err
	}
	return &result, nil
}
func (r *catalogoClient) ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigo *string) (*domain.SucursalSupermercado, error) {
	var result domain.SucursalSupermercado
	found, err := r.call(ctx, "sucursales/resolver", map[string]any{"cadena_id": cadenaID, "sucursal_id": sucursalID, "codigo_sucursal": codigo}, &result)
	if err != nil || !found {
		return nil, err
	}
	return &result, nil
}
func (r *catalogoClient) IngestarLote(ctx context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	var result domain.IngestaResultadoDTO
	found, err := r.call(ctx, "ingestas", map[string]any{"sucursal_id": sucursalID, "productos": productos}, &result)
	if err != nil {
		return nil, err
	}
	if !found || result.SucursalID != sucursalID || result.TotalRecibidos != len(productos) ||
		result.Insertados < 0 || result.Actualizados < 0 || result.PreciosRegistrados != result.Insertados+result.Actualizados ||
		result.Insertados+result.Actualizados > len(productos) {
		return nil, services.ErrCatalogoNoDisponible
	}
	if trabajoID != nil {
		id := trabajoID.String()
		result.TrabajoID = &id
	}
	return &result, nil
}
