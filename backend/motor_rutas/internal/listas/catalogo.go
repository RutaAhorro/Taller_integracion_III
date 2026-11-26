package listas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// =============================================================================
// Cliente de la consulta interna de disponibilidad (SUP-312)
// =============================================================================

// rutaDisponibilidad y rutaPreciosComparables son las rutas internas de
// Catálogo. No pasan por el Gateway.
const (
	rutaDisponibilidad     = "/internal/v1/productos/disponibilidad"
	rutaPreciosComparables = "/internal/v1/productos/precios-comparables"
)

// largoMaximoRespuesta acota lo que se lee de Catálogo: 1000 productos con sus
// datos caben de sobra en 4 MiB, y así una respuesta anómala no agota memoria.
const largoMaximoRespuesta = 4 << 20

// ErrCatalogoNoDisponible indica que Catálogo no respondió a tiempo o con un
// código distinto de 200. El llamador decide: al agregar no se guarda nada
// (503); al mostrar la lista, los artículos quedan como "desconocido".
var ErrCatalogoNoDisponible = errors.New("catálogo no disponible")

// ClienteCatalogo consulta disponibilidad y precios comparables por HTTP con
// token de servicio. Motor nunca lee tablas de Catálogo.
type ClienteCatalogo struct {
	urlDisponibilidad     string
	urlPreciosComparables string
	token                 string
	clienteHTTP           *http.Client
}

// NuevoClienteCatalogo arma el cliente con el origen interno de Catálogo
// (CATALOGO_INTERNAL_URL), su token de consulta (CATALOGO_CONSULTA_TOKEN) y
// el plazo máximo por llamada (RUTAS_CATALOGO_TIMEOUT).
func NuevoClienteCatalogo(origen, token string, plazo time.Duration) *ClienteCatalogo {
	return &ClienteCatalogo{
		urlDisponibilidad:     origen + rutaDisponibilidad,
		urlPreciosComparables: origen + rutaPreciosComparables,
		token:                 token,
		clienteHTTP:           &http.Client{Timeout: plazo},
	}
}

// respuestaDisponibilidad es la parte de la respuesta de Catálogo que usa
// Motor. Los campos que no se nombran aquí se ignoran.
type respuestaDisponibilidad struct {
	Productos []struct {
		ProductoID     string            `json:"producto_id"`
		EstadoProducto string            `json:"estado_producto"`
		Producto       *ProductoCatalogo `json:"producto"`
	} `json:"productos"`
}

// Consultar pide a Catálogo el estado de los productos en una sola llamada.
// Un id que Catálogo no devuelve se informa como "no_encontrado".
func (cliente *ClienteCatalogo) Consultar(contexto context.Context, productoIDs []string) (map[string]Disponibilidad, error) {
	// --- Petición ------------------------------------------------------------
	cuerpo, errorObtenido := json.Marshal(map[string][]string{"producto_ids": productoIDs})
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.urlDisponibilidad, bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+cliente.token)

	respuesta, errorObtenido := cliente.clienteHTTP.Do(peticion)
	if errorObtenido != nil {
		// Sólo el tipo: la URL del error no debe llegar a los logs con datos.
		return nil, fmt.Errorf("%w (%T)", ErrCatalogoNoDisponible, errorObtenido)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w (estado %d)", ErrCatalogoNoDisponible, respuesta.StatusCode)
	}

	// --- Respuesta -----------------------------------------------------------
	var datos respuestaDisponibilidad
	if errorObtenido := json.NewDecoder(io.LimitReader(respuesta.Body, largoMaximoRespuesta)).Decode(&datos); errorObtenido != nil {
		return nil, fmt.Errorf("%w (respuesta inválida)", ErrCatalogoNoDisponible)
	}
	resultado := make(map[string]Disponibilidad, len(productoIDs))
	for _, productoID := range productoIDs {
		resultado[productoID] = Disponibilidad{Estado: EstadoNoEncontrado}
	}
	for _, item := range datos.Productos {
		if _, pedido := resultado[item.ProductoID]; pedido {
			resultado[item.ProductoID] = Disponibilidad{Estado: item.EstadoProducto, Producto: item.Producto}
		}
	}
	return resultado, nil
}

// ConsultarPreciosComparables obtiene de Catálogo las sucursales físicas dentro
// del radio y las ofertas comparables por EAN de los productos solicitados.
// Nunca expone el token ni la URL en los errores devueltos.
func (cliente *ClienteCatalogo) ConsultarPreciosComparables(contexto context.Context, consulta ConsultaPreciosComparables) (RespuestaPreciosComparables, error) {
	cuerpo, errorObtenido := json.Marshal(consulta)
	if errorObtenido != nil {
		return RespuestaPreciosComparables{}, errorObtenido
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, http.MethodPost, cliente.urlPreciosComparables, bytes.NewReader(cuerpo))
	if errorObtenido != nil {
		return RespuestaPreciosComparables{}, errorObtenido
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+cliente.token)

	respuesta, errorObtenido := cliente.clienteHTTP.Do(peticion)
	if errorObtenido != nil {
		return RespuestaPreciosComparables{}, fmt.Errorf("%w (%T)", ErrCatalogoNoDisponible, errorObtenido)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return RespuestaPreciosComparables{}, fmt.Errorf("%w (estado %d)", ErrCatalogoNoDisponible, respuesta.StatusCode)
	}

	var datos RespuestaPreciosComparables
	if errorObtenido := json.NewDecoder(io.LimitReader(respuesta.Body, largoMaximoRespuesta)).Decode(&datos); errorObtenido != nil {
		return RespuestaPreciosComparables{}, fmt.Errorf("%w (respuesta inválida)", ErrCatalogoNoDisponible)
	}
	return datos, nil
}

// El compilador comprueba que ClienteCatalogo cumple ambas interfaces.
var (
	_ Catalogo        = (*ClienteCatalogo)(nil)
	_ CatalogoPrecios = (*ClienteCatalogo)(nil)
)
