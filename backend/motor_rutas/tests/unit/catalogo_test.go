// catalogo_test.go prueba el cliente HTTP de la consulta de disponibilidad de
// Catálogo (SUP-312) contra un servidor falso.
package unit_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

const (
	tokenConsultaPrueba = "consulta-solo-pruebas-de-al-menos-32-bytes"
	plazoClientePrueba  = 100 * time.Millisecond
)

func TestClienteCatalogoConsultaEnLote(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Method != http.MethodPost || peticion.URL.Path != "/internal/v1/productos/disponibilidad" ||
			peticion.Header.Get("Authorization") != "Bearer "+tokenConsultaPrueba || peticion.Header.Get("Origin") != "" {
			respuesta.WriteHeader(http.StatusForbidden)
			return
		}
		var cuerpo struct {
			ProductoIDs []string `json:"producto_ids"`
		}
		if errorObtenido := json.NewDecoder(peticion.Body).Decode(&cuerpo); errorObtenido != nil || len(cuerpo.ProductoIDs) != 2 {
			respuesta.WriteHeader(http.StatusBadRequest)
			return
		}
		// Sólo devuelve el primero; el segundo debe quedar "no_encontrado".
		// Incluye campos que Motor no usa, para comprobar que no rompen la lectura.
		respuesta.Header().Set("Content-Type", "application/json")
		_, _ = respuesta.Write([]byte(`{"productos": [{"producto_id": "` + productoLeche + `",
            "estado_producto": "sin_stock", "en_stock": false, "estado_precio": "vigente",
            "producto": {"nombre": "Leche", "supermercado": "Lider", "precio": 1190, "sucursal_id": 7}}]}`))
	}))
	defer servidor.Close()

	cliente := listas.NuevoClienteCatalogo(servidor.URL, tokenConsultaPrueba, time.Second)
	estados, errorObtenido := cliente.Consultar(prueba.Context(), []string{productoLeche, productoPan})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	leche := estados[productoLeche]
	if leche.Estado != listas.EstadoSinStock || leche.Producto == nil || leche.Producto.Nombre != "Leche" ||
		leche.Producto.Precio == nil || *leche.Producto.Precio != 1190 {
		prueba.Fatalf("leche: %+v", leche)
	}
	if pan := estados[productoPan]; pan.Estado != listas.EstadoNoEncontrado || pan.Producto != nil {
		prueba.Fatalf("pan: %+v", pan)
	}
}

func TestClienteCatalogoFallaSinExponerElToken(prueba *testing.T) {
	casos := map[string]http.HandlerFunc{
		"estado 500": func(respuesta http.ResponseWriter, _ *http.Request) {
			respuesta.WriteHeader(http.StatusInternalServerError)
		},
		"JSON inválido": func(respuesta http.ResponseWriter, _ *http.Request) {
			_, _ = respuesta.Write([]byte(`{"productos": [`))
		},
		// Responde después del plazo del cliente. La pausa es acotada para que
		// servidor.Close() no quede esperando al handler.
		"plazo vencido": func(respuesta http.ResponseWriter, _ *http.Request) {
			time.Sleep(4 * plazoClientePrueba)
		},
	}
	for nombre, manejador := range casos {
		servidor := httptest.NewServer(manejador)
		cliente := listas.NuevoClienteCatalogo(servidor.URL, tokenConsultaPrueba, plazoClientePrueba)
		_, errorObtenido := cliente.Consultar(prueba.Context(), []string{productoLeche})
		servidor.Close()
		if !errors.Is(errorObtenido, listas.ErrCatalogoNoDisponible) {
			prueba.Errorf("%s: se esperaba ErrCatalogoNoDisponible, se obtuvo %v", nombre, errorObtenido)
			continue
		}
		if strings.Contains(errorObtenido.Error(), tokenConsultaPrueba) {
			prueba.Errorf("%s: el error expone el token", nombre)
		}
	}
}

func TestClienteCatalogoPreciosComparables(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Method != http.MethodPost || peticion.URL.Path != "/internal/v1/productos/precios-comparables" ||
			peticion.Header.Get("Authorization") != "Bearer "+tokenConsultaPrueba || peticion.Header.Get("Origin") != "" {
			respuesta.WriteHeader(http.StatusForbidden)
			return
		}
		var cuerpo listas.ConsultaPreciosComparables
		if errorObtenido := json.NewDecoder(peticion.Body).Decode(&cuerpo); errorObtenido != nil ||
			len(cuerpo.ProductoIDs) != 2 || cuerpo.RadioKm != 10 || cuerpo.MaximoSucursales != 6 {
			respuesta.WriteHeader(http.StatusBadRequest)
			return
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_, _ = respuesta.Write([]byte(`{
			"sucursales": [
				{"sucursal_id": 5, "nombre": "SuperBodega aCuenta Temuco Matta", "cadena": "A Cuenta", "lat": -38.7338383, "lon": -72.5806422, "distancia_km": 1.61}
			],
			"productos": [
				{
					"producto_id": "` + productoLeche + `",
					"estado": "comparable",
					"ofertas": [
						{"producto_id_oferta": "` + productoLeche + `", "sucursal_id": 5, "cadena": "A Cuenta", "sucursal": "SuperBodega aCuenta Temuco Matta", "nombre": "Leche 1 L", "unidad": "1 L", "precio": 990, "en_stock": true, "estado_precio": "vigente"}
					]
				}
			],
			"sin_ofertas": ["` + productoPan + `"]
		}`))
	}))
	defer servidor.Close()

	cliente := listas.NuevoClienteCatalogo(servidor.URL, tokenConsultaPrueba, time.Second)
	resultado, errorObtenido := cliente.ConsultarPreciosComparables(prueba.Context(), listas.ConsultaPreciosComparables{
		ProductoIDs:      []string{productoLeche, productoPan},
		Lat:              -38.7359,
		Lon:              -72.5904,
		RadioKm:          10,
		MaximoSucursales: 6,
	})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(resultado.Sucursales) != 1 || resultado.Sucursales[0].SucursalID != 5 {
		prueba.Fatalf("sucursales: %+v", resultado.Sucursales)
	}
	if len(resultado.Productos) != 1 || len(resultado.Productos[0].Ofertas) != 1 ||
		resultado.Productos[0].Ofertas[0].Precio == nil || *resultado.Productos[0].Ofertas[0].Precio != 990 {
		prueba.Fatalf("productos: %+v", resultado.Productos)
	}
	if len(resultado.SinOfertas) != 1 || resultado.SinOfertas[0] != productoPan {
		prueba.Fatalf("sin_ofertas: %+v", resultado.SinOfertas)
	}
}

func TestClienteCatalogoPreciosComparablesFallaSinExponerToken(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		respuesta.WriteHeader(http.StatusBadGateway)
	}))
	defer servidor.Close()

	cliente := listas.NuevoClienteCatalogo(servidor.URL, tokenConsultaPrueba, plazoClientePrueba)
	_, errorObtenido := cliente.ConsultarPreciosComparables(prueba.Context(), listas.ConsultaPreciosComparables{
		ProductoIDs:      []string{productoLeche},
		Lat:              -38.7359,
		Lon:              -72.5904,
		RadioKm:          10,
		MaximoSucursales: 10,
	})
	if !errors.Is(errorObtenido, listas.ErrCatalogoNoDisponible) {
		prueba.Fatalf("se esperaba ErrCatalogoNoDisponible, se obtuvo %v", errorObtenido)
	}
	if strings.Contains(errorObtenido.Error(), tokenConsultaPrueba) {
		prueba.Fatalf("el error expone el token")
	}
}
