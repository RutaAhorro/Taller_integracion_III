// articulos_test.go prueba los endpoints de artículos de una lista (SUP-241)
// contra el repositorio en memoria y el Catálogo falso de apoyo_test.go.
package unit_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// ===== Datos y auxiliares =====

const (
	productoLeche = "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e"
	productoPan   = "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	productoArroz = "3e4f5a6b-7c8d-4e9f-8a0b-1c2d3e4f5a6b"
	productoCafe  = "5a6b7c8d-9e0f-4a1b-8c2d-3e4f5a6b7c8d"
)

// rutaArticulos devuelve /api/v1/listas/{id}/articulos.
func rutaArticulos(idLista string) string { return api + "/" + idLista + "/articulos" }

// rutaArticulo devuelve /api/v1/listas/{id}/articulos/{articulo_id}.
func rutaArticulo(idLista string, articuloID any) string {
	return fmt.Sprintf("%s/%v", rutaArticulos(idLista), articuloID)
}

// agregar envía el alta y devuelve el código y el cuerpo decodificado.
func agregar(prueba *testing.T, clienteHTTP *cliente, idLista, productoID string, cantidad int) (int, map[string]any) {
	prueba.Helper()
	cuerpo := fmt.Sprintf(`{"producto_id": %q, "cantidad": %d}`, productoID, cantidad)
	respuesta := clienteHTTP.pedir(http.MethodPost, rutaArticulos(idLista), cuerpo, nil)
	return respuesta.Code, decodificar(prueba, respuesta.Body.Bytes())
}

// ===== Alta, suma, cambio y baja =====

func TestArticulosAltaSumaCambioYBaja(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "Semana")["id"].(string)

	// Alta: 201 con el estado y los datos de Catálogo.
	codigo, articulo := agregar(prueba, clienteHTTP, idLista, productoLeche, 2)
	if codigo != http.StatusCreated || articulo["producto_id"] != productoLeche || articulo["cantidad"] != 2.0 ||
		articulo["esta_comprado"] != false || articulo["estado_producto"] != listas.EstadoDisponible {
		prueba.Fatalf("alta: %d %v", codigo, articulo)
	}
	if producto, esObjeto := articulo["producto"].(map[string]any); !esObjeto || producto["nombre"] != "Leche entera 1 L" {
		prueba.Fatalf("alta sin datos del producto: %v", articulo["producto"])
	}
	idArticulo := articulo["id"]

	// Mismo producto otra vez: 200 y se suma la cantidad sobre el mismo artículo.
	codigo, sumado := agregar(prueba, clienteHTTP, idLista, productoLeche, 3)
	if codigo != http.StatusOK || sumado["id"] != idArticulo || sumado["cantidad"] != 5.0 {
		prueba.Fatalf("suma: %d %v", codigo, sumado)
	}

	// Cambiar cantidad y marcar comprado.
	respuesta := clienteHTTP.pedir(http.MethodPut, rutaArticulo(idLista, idArticulo), `{"cantidad": 1, "esta_comprado": true}`, nil)
	cambiado := decodificar(prueba, respuesta.Body.Bytes())
	if respuesta.Code != http.StatusOK || cambiado["cantidad"] != 1.0 || cambiado["esta_comprado"] != true {
		prueba.Fatalf("cambio: %d %v", respuesta.Code, cambiado)
	}

	// El detalle muestra el artículo con su estado en Catálogo.
	detalle := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil).Body.Bytes())
	articulos := detalle["articulos"].([]any)
	if detalle["total_articulos"] != 1.0 || len(articulos) != 1 ||
		articulos[0].(map[string]any)["estado_producto"] != listas.EstadoDisponible {
		prueba.Fatalf("detalle: %v", detalle)
	}

	// Baja: 204 y luego el artículo ya no existe.
	if respuesta := clienteHTTP.pedir(http.MethodDelete, rutaArticulo(idLista, idArticulo), "", nil); respuesta.Code != http.StatusNoContent {
		prueba.Fatalf("baja: %d", respuesta.Code)
	}
	if respuesta := clienteHTTP.pedir(http.MethodDelete, rutaArticulo(idLista, idArticulo), "", nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("baja repetida: %d", respuesta.Code)
	}
}

// ===== Validación del cuerpo =====

func TestArticulosRechazaCuerposInvalidos(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "Validación")["id"].(string)

	for _, cuerpo := range []string{
		`{}`,
		`{"cantidad": 1}`,
		`{"producto_id": "no-es-uuid", "cantidad": 1}`,
		`{"producto_id": "` + productoLeche + `"}`,
		`{"producto_id": "` + productoLeche + `", "cantidad": 0}`,
		`{"producto_id": "` + productoLeche + `", "cantidad": -2}`,
		`{"producto_id": "` + productoLeche + `", "cantidad": 1.5}`,
		`{"producto_id": "` + productoLeche + `", "cantidad": 1, "precio": 10}`,
	} {
		respuesta := clienteHTTP.pedir(http.MethodPost, rutaArticulos(idLista), cuerpo, nil)
		if respuesta.Code != http.StatusBadRequest || decodificar(prueba, respuesta.Body.Bytes())["mensaje"] != "Solicitud inválida." {
			prueba.Errorf("%s: %d %s", cuerpo, respuesta.Code, respuesta.Body)
		}
	}
	if clienteHTTP.catalogo.consultas != 0 {
		prueba.Fatalf("un cuerpo inválido no debe consultar a Catálogo (%d consultas)", clienteHTTP.catalogo.consultas)
	}

	// PUT sin campos o con cantidad inválida.
	_, articulo := agregar(prueba, clienteHTTP, idLista, productoLeche, 1)
	for _, cuerpo := range []string{`{}`, `{"cantidad": 0}`, `{"esta_comprado": "si"}`} {
		if respuesta := clienteHTTP.pedir(http.MethodPut, rutaArticulo(idLista, articulo["id"]), cuerpo, nil); respuesta.Code != http.StatusBadRequest {
			prueba.Errorf("PUT %s: %d", cuerpo, respuesta.Code)
		}
	}
}

// ===== Topes configurables =====

func TestArticulosRespetanLosTopes(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	for _, productoID := range []string{productoLeche, productoPan, productoArroz, productoCafe} {
		clienteHTTP.catalogo.producto(productoID, listas.EstadoDisponible)
	}
	idLista := crear(prueba, clienteHTTP, "Topes")["id"].(string)

	// Cantidad sobre el tope en el alta: 409 y no se guarda.
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoLeche, maxCantidadPrueba+1); codigo != http.StatusConflict {
		prueba.Fatalf("cantidad sobre el tope: %d %v", codigo, cuerpo)
	}

	// La suma tampoco puede pasar el tope.
	agregar(prueba, clienteHTTP, idLista, productoLeche, maxCantidadPrueba)
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoLeche, 1); codigo != http.StatusConflict {
		prueba.Fatalf("suma sobre el tope: %d %v", codigo, cuerpo)
	}

	// Máximo de artículos distintos por lista.
	agregar(prueba, clienteHTTP, idLista, productoPan, 1)
	agregar(prueba, clienteHTTP, idLista, productoArroz, 1)
	codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoCafe, 1)
	esperado := fmt.Sprintf("Alcanzaste el máximo de %d artículos en la lista.", maxArticulosPrueba)
	if codigo != http.StatusConflict || cuerpo["mensaje"] != esperado {
		prueba.Fatalf("tope de artículos: %d %v", codigo, cuerpo)
	}
}

// ===== Estados en Catálogo =====

func TestArticulosSegunCatalogo(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoPan, listas.EstadoSinStock)
	idLista := crear(prueba, clienteHTTP, "Catálogo")["id"].(string)

	// Producto inexistente: 422 y no se guarda.
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoLeche, 1); codigo != http.StatusUnprocessableEntity ||
		cuerpo["mensaje"] != "El producto no existe en el catálogo." {
		prueba.Fatalf("inexistente: %d %v", codigo, cuerpo)
	}

	// Sin stock: se guarda igual y se advierte con estado_producto (revisión de #133).
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoPan, 1); codigo != http.StatusCreated ||
		cuerpo["estado_producto"] != listas.EstadoSinStock {
		prueba.Fatalf("sin stock: %d %v", codigo, cuerpo)
	}

	// Catálogo caído al agregar: 503 y no se guarda nada.
	clienteHTTP.catalogo.producto(productoArroz, listas.EstadoDisponible)
	clienteHTTP.catalogo.caido = true
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoArroz, 1); codigo != http.StatusServiceUnavailable ||
		cuerpo["mensaje"] != "No se pudo verificar el producto. Intenta de nuevo." {
		prueba.Fatalf("catálogo caído al agregar: %d %v", codigo, cuerpo)
	}

	// Catálogo caído al mostrar: la lista se ve igual, con "desconocido".
	respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil)
	detalle := decodificar(prueba, respuesta.Body.Bytes())
	articulos := detalle["articulos"].([]any)
	if respuesta.Code != http.StatusOK || len(articulos) != 1 {
		prueba.Fatalf("detalle con catálogo caído: %d %v", respuesta.Code, detalle)
	}
	if articulo := articulos[0].(map[string]any); articulo["estado_producto"] != listas.EstadoDesconocido || articulo["producto"] != nil {
		prueba.Fatalf("artículo con catálogo caído: %v", articulo)
	}
}

// ===== Aislamiento por dueño =====

func TestArticulosDeListaAjenaResponden404(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "De A")["id"].(string)
	_, articulo := agregar(prueba, clienteHTTP, idLista, productoLeche, 1)

	clienteHTTP.jwt = tokenDe(prueba, usuarioB)
	if codigo, cuerpo := agregar(prueba, clienteHTTP, idLista, productoLeche, 1); codigo != http.StatusNotFound || cuerpo["mensaje"] != "La lista no existe." {
		prueba.Fatalf("B agrega a la lista de A: %d %v", codigo, cuerpo)
	}
	// La propiedad se comprueba antes que Catálogo: producto inexistente o
	// Catálogo caído tampoco revelan que la lista existe.
	if codigo, _ := agregar(prueba, clienteHTTP, idLista, productoArroz, 1); codigo != http.StatusNotFound {
		prueba.Fatalf("B agrega un producto inexistente a la lista de A: %d", codigo)
	}
	clienteHTTP.catalogo.caido = true
	if codigo, _ := agregar(prueba, clienteHTTP, idLista, productoLeche, 1); codigo != http.StatusNotFound {
		prueba.Fatalf("B agrega con Catálogo caído a la lista de A: %d", codigo)
	}
	clienteHTTP.catalogo.caido = false
	if respuesta := clienteHTTP.pedir(http.MethodPut, rutaArticulo(idLista, articulo["id"]), `{"cantidad": 9}`, nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("B cambia un artículo de A: %d", respuesta.Code)
	}
	if respuesta := clienteHTTP.pedir(http.MethodDelete, rutaArticulo(idLista, articulo["id"]), "", nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("B borra un artículo de A: %d", respuesta.Code)
	}

	// El artículo de A sigue igual.
	clienteHTTP.jwt = tokenDe(prueba, usuarioA)
	detalle := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil).Body.Bytes())
	if guardado := detalle["articulos"].([]any)[0].(map[string]any); guardado["cantidad"] != 1.0 {
		prueba.Fatalf("A perdió su artículo: %v", guardado)
	}

	// Un artículo de otra lista del mismo usuario tampoco se alcanza.
	otraLista := crear(prueba, clienteHTTP, "Otra de A")["id"].(string)
	if respuesta := clienteHTTP.pedir(http.MethodPut, rutaArticulo(otraLista, articulo["id"]), `{"cantidad": 9}`, nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("artículo de otra lista: %d", respuesta.Code)
	}
}

// ===== Rutas =====

func TestRutasDeArticulos(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	idLista := crear(prueba, clienteHTTP, "Rutas")["id"].(string)

	for _, idArticulo := range []string{"0", "-1", "abc", "1.5", "99999999999999999999"} {
		respuesta := clienteHTTP.pedir(http.MethodDelete, rutaArticulo(idLista, idArticulo), "", nil)
		if respuesta.Code != http.StatusNotFound || decodificar(prueba, respuesta.Body.Bytes())["mensaje"] != "El artículo no existe." {
			prueba.Errorf("id %q: %d %s", idArticulo, respuesta.Code, respuesta.Body)
		}
	}
	respuesta := clienteHTTP.pedir(http.MethodGet, rutaArticulos(idLista), "", nil)
	if respuesta.Code != http.StatusMethodNotAllowed || respuesta.Header().Get("Allow") != "POST" {
		prueba.Fatalf("GET de la colección: %d %q", respuesta.Code, respuesta.Header().Get("Allow"))
	}
	respuesta = clienteHTTP.pedir(http.MethodGet, rutaArticulo(idLista, 1), "", nil)
	if respuesta.Code != http.StatusMethodNotAllowed || respuesta.Header().Get("Allow") != "DELETE, PUT" {
		prueba.Fatalf("GET de un artículo: %d %q", respuesta.Code, respuesta.Header().Get("Allow"))
	}
	if respuesta := clienteHTTP.pedir(http.MethodGet, rutaArticulo(idLista, 1)+"/extra", "", nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("subruta extra: %d", respuesta.Code)
	}
	clienteHTTP.jwt = ""
	if respuesta := clienteHTTP.pedir(http.MethodPost, rutaArticulos(idLista), `{}`, nil); respuesta.Code != http.StatusUnauthorized {
		prueba.Fatalf("sin sesión: %d", respuesta.Code)
	}
}
