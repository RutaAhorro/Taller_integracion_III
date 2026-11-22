// Cliente HTTP de la API interna de Catálogo (/internal/v1/*). Control nunca
// lee tablas de Catálogo por SQL: resuelve cadenas y sucursales y entrega
// lotes por este cliente.
package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/google/uuid"
)

// ===== Rutas y límites del contrato interno =====

const (
	prefijoInterno       = "/internal/v1/"
	rutaResolverCadena   = "cadenas/resolver"
	rutaResolverSucursal = "sucursales/resolver"
	rutaIngestas         = "ingestas"
	// plazoConexionCatalogo acota DNS + TCP y el handshake TLS por separado del
	// plazo total configurado.
	plazoConexionCatalogo = 2 * time.Second
	keepAliveTCPCatalogo  = 30 * time.Second
	// maximoBytesRespuesta impide que una respuesta anómala agote memoria.
	maximoBytesRespuesta = 1 << 20
)

// ===== Cliente =====

// clienteCatalogo implementa domain.CatalogoRepository.
type clienteCatalogo struct {
	origen  string
	token   string
	cliente *http.Client
}

// NuevoClienteCatalogo crea el cliente con HTTP/1.1, sin keep-alive, sin proxy
// ni redirecciones: las tres operaciones son POST y ningún lote se reenvía
// automáticamente. origen y token ya fueron validados por config.
func NuevoClienteCatalogo(origen, token string, plazo time.Duration) domain.CatalogoRepository {
	transporte := http.DefaultTransport.(*http.Transport).Clone()
	transporte.Proxy = nil
	transporte.DialContext = (&net.Dialer{Timeout: plazoConexionCatalogo, KeepAlive: keepAliveTCPCatalogo}).DialContext
	transporte.TLSHandshakeTimeout = plazoConexionCatalogo
	transporte.ResponseHeaderTimeout = plazo
	transporte.DisableKeepAlives = true
	transporte.Protocols = new(http.Protocols)
	transporte.Protocols.SetHTTP1(true)
	return &clienteCatalogo{origen: origen, token: token, cliente: &http.Client{
		Transport: transporte, Timeout: plazo,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// llamar ejecuta una operación interna. Devuelve false sin error ante 404 y
// traduce el resto de fallos a errores de dominio, sin exponer detalles del
// upstream (SQL, secretos) al cliente público.
func (cliente *clienteCatalogo) llamar(contexto context.Context, ruta string, carga, resultado any) (bool, error) {
	datos, errorObtenido := json.Marshal(carga)
	if errorObtenido != nil {
		return false, errorObtenido
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.origen+prefijoInterno+ruta, bytes.NewReader(datos))
	if errorObtenido != nil {
		return false, domain.ErrCatalogoNoDisponible
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+cliente.token)
	// Sin X-Request-ID de la petición original (recuperación en segundo plano)
	// se usa la clave del lote para correlacionar.
	identificadorPeticion := domain.IDPeticion(contexto)
	if identificadorPeticion == "" {
		identificadorPeticion = domain.LoteID(contexto)
	}
	peticion.Header.Set("X-Request-ID", identificadorPeticion)
	respuesta, errorObtenido := cliente.cliente.Do(peticion)
	if errorObtenido != nil {
		if contexto.Err() != nil {
			return false, contexto.Err()
		}
		var errorRed net.Error
		if errors.As(errorObtenido, &errorRed) && errorRed.Timeout() {
			return false, domain.ErrCatalogoTimeout
		}
		return false, domain.ErrCatalogoNoDisponible
	}
	defer respuesta.Body.Close()
	switch respuesta.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return false, nil
	case http.StatusConflict:
		return false, domain.ErrIdempotencia
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return false, domain.ErrCatalogoRechazo
	default:
		return false, domain.ErrCatalogoNoDisponible
	}
	cuerpo, errorObtenido := io.ReadAll(io.LimitReader(respuesta.Body, maximoBytesRespuesta+1))
	if errorObtenido != nil || len(cuerpo) > maximoBytesRespuesta || json.Unmarshal(cuerpo, resultado) != nil {
		return false, domain.ErrCatalogoNoDisponible
	}
	return true, nil
}

// ===== Operaciones =====

// ObtenerCadena resuelve una cadena por ID o nombre; nil si Catálogo no la conoce.
func (cliente *clienteCatalogo) ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	var resultado domain.CadenaSupermercado
	encontrado, errorObtenido := cliente.llamar(contexto, rutaResolverCadena, map[string]any{"cadena_id": cadenaID, "nombre": nombre}, &resultado)
	if errorObtenido != nil || !encontrado {
		return nil, errorObtenido
	}
	return &resultado, nil
}

// ObtenerSucursal resuelve una sucursal por ID o código; nil si no existe.
func (cliente *clienteCatalogo) ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigo *string) (*domain.SucursalSupermercado, error) {
	var resultado domain.SucursalSupermercado
	encontrado, errorObtenido := cliente.llamar(contexto, rutaResolverSucursal, map[string]any{"cadena_id": cadenaID, "sucursal_id": sucursalID, "codigo_sucursal": codigo}, &resultado)
	if errorObtenido != nil || !encontrado {
		return nil, errorObtenido
	}
	return &resultado, nil
}

// IngestarLote envía los productos con la clave estable del lote y valida que
// los contadores de la respuesta sean coherentes antes de confirmarlos.
func (cliente *clienteCatalogo) IngestarLote(contexto context.Context, trabajoID *uuid.UUID, sucursalID int, productos []domain.ProductoScrapeadoDTO) (*domain.IngestaResultadoDTO, error) {
	var resultado domain.IngestaResultadoDTO
	carga := map[string]any{"sucursal_id": sucursalID, "productos": productos}
	if identificador := domain.LoteID(contexto); identificador != "" {
		carga["lote_id"] = identificador
	}
	encontrado, errorObtenido := cliente.llamar(contexto, rutaIngestas, carga, &resultado)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	procesados := resultado.Insertados + resultado.Actualizados
	if !encontrado || resultado.SucursalID != sucursalID || resultado.TotalRecibidos != len(productos) ||
		resultado.Insertados < 0 || resultado.Actualizados < 0 || resultado.PreciosRegistrados != procesados || procesados > len(productos) {
		return nil, domain.ErrCatalogoNoDisponible
	}
	if trabajoID != nil {
		identificador := trabajoID.String()
		resultado.TrabajoID = &identificador
	}
	return &resultado, nil
}
