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
