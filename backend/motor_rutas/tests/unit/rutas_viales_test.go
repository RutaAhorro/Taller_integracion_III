// Pruebas HTTP de origen, perfil y geometría: repositorio aislado en memoria.
package unit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/httpapi"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/vial"
)

// vialFalso distingue fallos del proveedor de una matriz sin conectividad.
type vialFalso struct {
	fallar        bool
	vacia         bool
	fallarTrazado bool
}

// proveedorObservado solo registra errores ya sanitizados del cliente, nunca claves ni cuerpos.
type proveedorObservado struct {
	vial.Proveedor
	prueba *testing.T
}

// Matriz informa la fase que falla en la comprobación opt-in.
func (observado proveedorObservado) Matriz(contexto context.Context, modo string, origen optimizacion.Coordenada, sucursales []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error) {
	resultado, errorObtenido := observado.Proveedor.Matriz(contexto, modo, origen, sucursales)
	if errorObtenido != nil {
		observado.prueba.Logf("Matrix (%s): %v", modo, errorObtenido)
	}
	return resultado, errorObtenido
}

// Trazar distingue un fallo de geometría de un fallo de matriz.
func (observado proveedorObservado) Trazar(contexto context.Context, modo string, origen optimizacion.Coordenada, ruta *optimizacion.Ruta) error {
	errorObtenido := observado.Proveedor.Trazar(contexto, modo, origen, ruta)
	if errorObtenido != nil {
		observado.prueba.Logf("Directions (%s): %v", modo, errorObtenido)
	}
	return errorObtenido
}

// Trazar permite simular una caída posterior a Matrix sin devolver una ruta a medias.
func (falso vialFalso) Trazar(contexto context.Context, modo string, origen optimizacion.Coordenada, ruta *optimizacion.Ruta) error {
	if falso.fallarTrazado {
		return errors.New("sin geometría")
	}
	return nil
}

// Matriz genera distancias controladas exclusivamente para pruebas.
func (falso vialFalso) Matriz(contexto context.Context, modo string, origen optimizacion.Coordenada, sucursales []optimizacion.Sucursal) (map[[2]int]optimizacion.TrayectoVial, error) {
	if falso.fallar {
		return nil, errors.New("sin proveedor")
	}
	matriz := map[[2]int]optimizacion.TrayectoVial{}
	if falso.vacia {
		return matriz, nil
	}
	puntos := append([]optimizacion.Sucursal{{ID: 0}}, sucursales...)
	for _, desde := range puntos {
		for _, hasta := range puntos {
			if desde.ID != hasta.ID {
				matriz[[2]int{desde.ID, hasta.ID}] = optimizacion.TrayectoVial{DistanciaKm: 1, DuracionSegundos: 60, Geometrias: []string{"_p~iF~ps|U_ulLnnqC_mqNvxq`@"}}
			}
		}
	}
	return matriz, nil
}

// preciosVialesPrueba usa puntos públicos de Temuco y un precio controlado.
func preciosVialesPrueba() *preciosFalso {
	return &preciosFalso{respuesta: listas.RespuestaPreciosComparables{
		Sucursales: sucursalesTemucoPrueba()[:2],
		Productos: []listas.ProductoPreciosComparable{{
			ProductoID: productoLeche, Estado: "comparable", Ofertas: []listas.OfertaComparable{
				{ProductoIDOferta: productoLeche, SucursalID: 5, Precio: ptrFloat(1100), EnStock: ptrBool(true)},
				{ProductoIDOferta: productoLeche, SucursalID: 6, Precio: ptrFloat(1200), EnStock: ptrBool(true)},
			},
		}},
	}}
}

// comprobarEndpointVial ejecuta todo el handler con autorización y datos aislados.
func comprobarEndpointVial(prueba *testing.T, proveedor vial.Proveedor, modo string) int {
	prueba.Helper()
	clienteHTTP := nuevoClienteRutas(prueba, preciosVialesPrueba(), httpapi.OpcionesOptimizacion{}, proveedor)
	listaID := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioA, []listas.ArticuloLote{{ProductoID: productoLeche, Cantidad: 1}})
	respuesta := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaID+`","medio_transporte":"`+modo+`","origen":{"lat":-38.7359,"lon":-72.5904}}`, nil)
	if respuesta.Code == http.StatusOK {
		var datos struct {
			Fuente string `json:"fuente_recorrido"`
			Modo   string `json:"medio_transporte"`
			Ruta   struct {
				Total  int `json:"total"`
				Tramos []struct {
					Geometrias []string `json:"geometrias"`
				} `json:"tramos"`
			} `json:"ruta"`
			Origen struct {
				PorDefecto bool `json:"por_defecto"`
			} `json:"origen"`
			Parametros struct {
				MaxParadas int `json:"max_paradas"`
			} `json:"parametros"`
		}
		if json.Unmarshal(respuesta.Body.Bytes(), &datos) != nil {
			prueba.Fatal("JSON inválido")
		}
		if datos.Fuente != "ors_osm" || datos.Modo != modo || datos.Origen.PorDefecto || datos.Parametros.MaxParadas != 2 ||
			len(datos.Ruta.Tramos[0].Geometrias) == 0 || datos.Ruta.Total != 1100 {
			prueba.Fatalf("respuesta vial incorrecta: %s", respuesta.Body)
		}
	}
	return respuesta.Code
}

// TestEndpointVial valida contrato nuevo, rechazo de perfiles y fallos sin respaldo.
func TestEndpointVial(prueba *testing.T) {
	for _, modo := range []string{"vehiculo", "bicicleta", "pie"} {
		if codigo := comprobarEndpointVial(prueba, vialFalso{}, modo); codigo != 200 {
			prueba.Fatalf("%s: %d", modo, codigo)
		}
	}
	if codigo := comprobarEndpointVial(prueba, nil, "vehiculo"); codigo != 503 {
		prueba.Fatal(codigo)
	}
	if codigo := comprobarEndpointVial(prueba, vialFalso{fallar: true}, "pie"); codigo != 503 {
		prueba.Fatal(codigo)
	}
	if codigo := comprobarEndpointVial(prueba, vialFalso{vacia: true}, "pie"); codigo != 422 {
		prueba.Fatal(codigo)
	}
	if codigo := comprobarEndpointVial(prueba, vialFalso{fallarTrazado: true}, "pie"); codigo != 503 {
		prueba.Fatal(codigo)
	}
	for _, modo := range []string{"scooter", "publico", "CAR"} {
		if codigo := comprobarEndpointVial(prueba, vialFalso{}, modo); codigo != 400 {
			prueba.Fatal(codigo)
		}
	}
}

// TestEndpointConORSReal es opt-in: consume tres consultas Matrix y tres Directions.
func TestEndpointConORSReal(prueba *testing.T) {
	if os.Getenv("ORS_VIAL_TEST") != "1" {
		prueba.Skip("requiere ORS_VIAL_TEST=1 y configuración ORS; consume cuota")
	}
	urlORS, clave := os.Getenv("RUTAS_ORS_URL"), os.Getenv("RUTAS_ORS_API_KEY")
	if urlORS == "" || clave == "" {
		prueba.Fatal("falta configuración ORS, sin imprimir credenciales")
	}
	for _, modo := range []string{"vehiculo", "bicicleta", "pie"} {
		if codigo := comprobarEndpointVial(prueba, proveedorObservado{Proveedor: vial.Nuevo(urlORS, clave), prueba: prueba}, modo); codigo != 200 {
			prueba.Fatalf("%s: %d", modo, codigo)
		}
	}
}
