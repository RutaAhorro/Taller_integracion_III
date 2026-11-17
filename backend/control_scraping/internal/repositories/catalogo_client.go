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

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
)

// catalogoClient implementa las llamadas internas de Control hacia Catálogo.
type catalogoClient struct {
	origen  string
	token   string
	cliente *http.Client
}

// ===== Configuración y llamadas HTTP a Catálogo =====

// NewCatalogoClient valida el origen y credenciales y limita los reintentos HTTP.
func NewCatalogoClient(origen, token string, tiempoEspera time.Duration) (domain.IngestaRepository, error) {
	direccion, errorObtenido := url.Parse(origen)
	if errorObtenido != nil || direccion == nil || (direccion.Scheme != "http" && direccion.Scheme != "https") || direccion.Hostname() == "" ||
		direccion.User != nil || (direccion.Path != "" && direccion.Path != "/") || direccion.RawQuery != "" || direccion.ForceQuery || direccion.Fragment != "" || strings.ContainsAny(origen, "?#\\ \t\r\n") {
		return nil, errors.New("CATALOGO_INTERNAL_URL debe ser un origen HTTP(S), sin credenciales, rutas ni parámetros")
	}
	if len(token) < 32 || strings.ContainsAny(token, " \t\r\n") {
		return nil, errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos 32 caracteres y no contener espacios")
	}
	if tiempoEspera <= 0 || tiempoEspera > 20*time.Second {
		return nil, errors.New("CATALOGO_INGEST_TIMEOUT debe ser positivo y no superar 20s")
	}
	transporte := http.DefaultTransport.(*http.Transport).Clone()
	transporte.Proxy = nil
	transporte.DialContext = (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transporte.TLSHandshakeTimeout = 2 * time.Second
	transporte.ResponseHeaderTimeout = tiempoEspera
	// Las tres operaciones son POST; ningún lote se reenvía automáticamente.
	transporte.DisableKeepAlives = true
	transporte.Protocols = new(http.Protocols)
	transporte.Protocols.SetHTTP1(true)
	return &catalogoClient{origen: strings.TrimSuffix(origen, "/"), token: token, cliente: &http.Client{
		Transport: transporte, Timeout: tiempoEspera,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// call ejecuta una operación interna y normaliza errores sin exponer detalles del upstream.
func (clienteCatalogo *catalogoClient) call(contexto context.Context, ruta string, carga, resultado any) (bool, error) {
	datos, errorObtenido := json.Marshal(carga)
	if errorObtenido != nil {
		return false, errorObtenido
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, clienteCatalogo.origen+"/internal/v1/"+ruta, bytes.NewReader(datos))
	if errorObtenido != nil {
		return false, services.ErrCatalogoNoDisponible
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+clienteCatalogo.token)
	if identificador, esValido := contexto.Value(requestIDKey{}).(string); esValido {
		peticion.Header.Set("X-Request-ID", identificador)
	}
	if peticion.Header.Get("X-Request-ID") == "" {
		peticion.Header.Set("X-Request-ID", domain.LoteID(contexto))
	}
	respuesta, errorObtenido := clienteCatalogo.cliente.Do(peticion)
	if errorObtenido != nil {
		if contexto.Err() != nil {
			return false, contexto.Err()
		}
		var tiempoEspera net.Error
		if errors.As(errorObtenido, &tiempoEspera) && tiempoEspera.Timeout() {
			return false, services.ErrCatalogoTimeout
		}
		return false, services.ErrCatalogoNoDisponible
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode == 404 {
		return false, nil
	}
	if respuesta.StatusCode == 409 {
		return false, services.ErrIdempotencia
	}
	if respuesta.StatusCode == 400 || respuesta.StatusCode == 413 {
		return false, services.ErrCatalogoRechazo
	}
	if respuesta.StatusCode != http.StatusOK {
		return false, services.ErrCatalogoNoDisponible
	}
	// No se propagan errores HTTP/SQL ni secretos del upstream al cliente público.
	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, 1<<20+1))
	if errorObtenido != nil || len(cuerpo) > 1<<20 || json.Unmarshal(cuerpo, resultado) != nil {
		return false, services.ErrCatalogoNoDisponible
	}
	return true, nil
}

// requestIDKey identifica el valor de trazabilidad guardado en un contexto.
type requestIDKey struct{}

// WithRequestID agrega al contexto el identificador de trazabilidad de la petición.
func WithRequestID(contexto context.Context, identificador string) context.Context {
	return context.WithValue(contexto, requestIDKey{}, identificador)
}

// ObtenerCadena resuelve en Catálogo la cadena asociada a una ingesta.
func (clienteCatalogo *catalogoClient) ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	var resultado domain.CadenaSupermercado
	encontrado, errorObtenido := clienteCatalogo.call(contexto, "cadenas/resolver", map[string]any{"cadena_id": cadenaID, "nombre": nombre}, &resultado)
	if errorObtenido != nil || !encontrado {
		return nil, errorObtenido
	}
	return &resultado, nil
}

// ObtenerSucursal resuelve en Catálogo la sucursal asociada a una ingesta.
func (clienteCatalogo *catalogoClient) ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigo *string) (*domain.SucursalSupermercado, error) {
	var resultado domain.SucursalSupermercado
	encontrado, errorObtenido := clienteCatalogo.call(contexto, "sucursales/resolver", map[string]any{"cadena_id": cadenaID, "sucursal_id": sucursalID, "codigo_sucursal": codigo}, &resultado)
	if errorObtenido != nil || !encontrado {
		return nil, errorObtenido
	}
	return &resultado, nil
}

// IngestarLote envía los productos a Catálogo y valida sus contadores de respuesta.
func (clienteCatalogo *catalogoClient) IngestarLote(contexto context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	var resultado domain.IngestaResultadoDTO
	carga := map[string]any{"sucursal_id": sucursalID, "productos": productos}
	if identificador := domain.LoteID(contexto); identificador != "" {
		carga["lote_id"] = identificador
	}
	encontrado, errorObtenido := clienteCatalogo.call(contexto, "ingestas", carga, &resultado)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if !encontrado || resultado.SucursalID != sucursalID || resultado.TotalRecibidos != len(productos) ||
		resultado.Insertados < 0 || resultado.Actualizados < 0 || resultado.PreciosRegistrados != resultado.Insertados+resultado.Actualizados ||
		resultado.Insertados+resultado.Actualizados > len(productos) {
		return nil, services.ErrCatalogoNoDisponible
	}
	if trabajoID != nil {
		identificador := trabajoID.String()
		resultado.TrabajoID = &identificador
	}
	return &resultado, nil
}
