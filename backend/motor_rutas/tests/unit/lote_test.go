// lote_test.go prueba el agregado en lote de artículos (SUP-318) contra el
// repositorio en memoria y el Catálogo falso de apoyo_test.go. Topes de
// prueba: 3 artículos por lista y 10 envases por artículo.
package unit_test

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// ===== Auxiliares =====

// claveLote es una Idempotency-Key válida para las pruebas.
const claveLote = "6a1f0d2e-3b4c-4d5e-8f90-a1b2c3d4e5f6"

// rutaLote devuelve /api/v1/listas/{id}/articulos/lote.
func rutaLote(idLista string) string { return rutaArticulos(idLista) + "/lote" }

// cuerpoLote arma {"articulos": [...]} con pares producto/cantidad.
func cuerpoLote(pares ...any) string {
	lineas := make([]string, 0, len(pares)/2)
	for indice := 0; indice < len(pares); indice += 2 {
		lineas = append(lineas, fmt.Sprintf(`{"producto_id": %q, "cantidad": %d}`, pares[indice], pares[indice+1]))
	}
	return `{"articulos": [` + strings.Join(lineas, ", ") + `]}`
}

// enviarLote envía el lote con la clave indicada ("" la omite).
func enviarLote(prueba *testing.T, clienteHTTP *cliente, idLista, clave, cuerpo string) (int, map[string]any) {
	prueba.Helper()
	cabeceras := map[string]string{}
	if clave != "" {
		cabeceras["Idempotency-Key"] = clave
	}
	respuesta := clienteHTTP.pedir(http.MethodPost, rutaLote(idLista), cuerpo, cabeceras)
	return respuesta.Code, decodificar(prueba, respuesta.Body.Bytes())
}

// cantidades resume los artículos del detalle como producto_id → cantidad.
func cantidades(detalle map[string]any) map[string]float64 {
	resultado := map[string]float64{}
	for _, crudo := range detalle["articulos"].([]any) {
		articulo := crudo.(map[string]any)
		resultado[articulo["producto_id"].(string)] = articulo["cantidad"].(float64)
	}
	return resultado
}

// ===== Éxito, suma de repetidos e idempotencia =====

func TestLoteAgregaSumaYEsIdempotente(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	clienteHTTP.catalogo.producto(productoPan, listas.EstadoSinStock)
	idLista := crear(prueba, clienteHTTP, "Receta")["id"].(string)
	if codigo, _ := agregar(prueba, clienteHTTP, idLista, productoLeche, 1); codigo != http.StatusCreated {
		prueba.Fatalf("alta previa: %d", codigo)
	}

	// Leche ya estaba (1) y viene dos veces en el lote (2+1): queda en 4.
	// Pan sin stock se guarda con advertencia, igual que en el alta de uno.
	cuerpo := cuerpoLote(productoLeche, 2, productoPan, 3, productoLeche, 1)
	codigo, detalle := enviarLote(prueba, clienteHTTP, idLista, claveLote, cuerpo)
	if codigo != http.StatusOK || detalle["id"] != idLista || detalle["total_articulos"] != 2.0 {
		prueba.Fatalf("lote: %d %v", codigo, detalle)
	}
	if obtenidas := cantidades(detalle); obtenidas[productoLeche] != 4 || obtenidas[productoPan] != 3 {
		prueba.Fatalf("cantidades: %v", obtenidas)
	}
	for _, crudo := range detalle["articulos"].([]any) {
		articulo := crudo.(map[string]any)
		if articulo["producto_id"] == productoPan && articulo["estado_producto"] != listas.EstadoSinStock {
			prueba.Fatalf("pan sin advertencia: %v", articulo)
		}
	}

	// Reintento con la misma clave (aunque cambie el orden): no suma otra vez
	// ni vuelve a validar en Catálogo antes de responder.
	consultasAntes := clienteHTTP.catalogo.consultas
	reordenado := cuerpoLote(productoPan, 3, productoLeche, 3)
	codigo, repetido := enviarLote(prueba, clienteHTTP, idLista, claveLote, reordenado)
	if codigo != http.StatusOK || cantidades(repetido)[productoLeche] != 4 || cantidades(repetido)[productoPan] != 3 {
		prueba.Fatalf("reintento: %d %v", codigo, repetido)
	}
	if clienteHTTP.catalogo.consultas != consultasAntes+1 { // sólo la del detalle
		prueba.Fatalf("el reintento validó otra vez en Catálogo")
	}

	// Misma clave con otro contenido: 409 idempotency_conflict, sin cambios.
	codigo, conflicto := enviarLote(prueba, clienteHTTP, idLista, claveLote, cuerpoLote(productoLeche, 1))
	if codigo != http.StatusConflict || conflicto["codigo"] != "idempotency_conflict" || conflicto["estado"] != 409.0 {
		prueba.Fatalf("clave reutilizada: %d %v", codigo, conflicto)
	}
	respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil)
	if obtenidas := cantidades(decodificar(prueba, respuesta.Body.Bytes())); obtenidas[productoLeche] != 4 {
		prueba.Fatalf("la clave reutilizada cambió la lista: %v", obtenidas)
	}

	// Una clave nueva sí vuelve a sumar.
	codigo, otraVez := enviarLote(prueba, clienteHTTP, idLista, "otra-clave", cuerpoLote(productoLeche, 1))
	if codigo != http.StatusOK || cantidades(otraVez)[productoLeche] != 5 {
		prueba.Fatalf("clave nueva: %d %v", codigo, otraVez)
	}
}

// ===== Todo o nada =====

func TestLoteTodoONada(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	clienteHTTP.catalogo.producto(productoPan, listas.EstadoDisponible)
	clienteHTTP.catalogo.producto(productoArroz, listas.EstadoDisponible)
	clienteHTTP.catalogo.producto(productoCafe, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "Carrito")["id"].(string)
	agregar(prueba, clienteHTTP, idLista, productoLeche, 8)

	casos := []struct {
		nombre     string
		cuerpo     string
		estado     int
		rechazados map[string]string // producto_id → motivo; nil si detalle no es arreglo
	}{
		{"producto inexistente", cuerpoLote(productoPan, 1, "7f000000-0000-4000-8000-000000000000", 1), http.StatusConflict,
			map[string]string{"7f000000-0000-4000-8000-000000000000": listas.MotivoNoEncontrado}},
		{"supera la cantidad al sumar con la lista", cuerpoLote(productoPan, 1, productoLeche, 3), http.StatusConflict,
			map[string]string{productoLeche: listas.MotivoCantidadMaxima}},
		{"supera la cantidad al sumar repetidos", cuerpoLote(productoPan, 6, productoPan, 5), http.StatusConflict,
			map[string]string{productoPan: listas.MotivoCantidadMaxima}},
		// Reproducción de la revisión de #176: sin saturar, la suma desborda y
		// guarda 1 con 200.
		{"la suma de repetidos desborda", cuerpoLote(productoPan, math.MaxInt, productoPan, math.MaxInt, productoPan, 3), http.StatusConflict,
			map[string]string{productoPan: listas.MotivoCantidadMaxima}},
		{"supera el máximo de artículos", cuerpoLote(productoPan, 1, productoArroz, 1, productoCafe, 1), http.StatusConflict, nil},
	}
	for indice, caso := range casos {
		codigo, cuerpo := enviarLote(prueba, clienteHTTP, idLista, fmt.Sprintf("clave-%d", indice), caso.cuerpo)
		if codigo != caso.estado {
			prueba.Fatalf("%s: %d %v", caso.nombre, codigo, cuerpo)
		}
		if caso.rechazados != nil {
			detalle, esArreglo := cuerpo["detalle"].([]any)
			if !esArreglo || len(detalle) != len(caso.rechazados) || cuerpo["mensaje"] != "No se pudo agregar el lote." {
				prueba.Fatalf("%s: detalle %v", caso.nombre, cuerpo)
			}
			for _, crudo := range detalle {
				rechazo := crudo.(map[string]any)
				if caso.rechazados[rechazo["producto_id"].(string)] != rechazo["motivo"] {
					prueba.Fatalf("%s: rechazo %v", caso.nombre, rechazo)
				}
			}
		}
	}

	// Ningún intento fallido dejó cambios: sólo la leche con 8.
	respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil)
	if obtenidas := cantidades(decodificar(prueba, respuesta.Body.Bytes())); len(obtenidas) != 1 || obtenidas[productoLeche] != 8 {
		prueba.Fatalf("quedaron cambios a medias: %v", obtenidas)
	}

	// Un lote rechazado no registra su clave: corregido, se puede reenviar.
	if codigo, _ := enviarLote(prueba, clienteHTTP, idLista, "clave-1", cuerpoLote(productoPan, 1, productoLeche, 2)); codigo != http.StatusOK {
		prueba.Fatalf("reenvío corregido: %d", codigo)
	}
}

// ===== Catálogo caído =====

func TestLoteSinCatalogoNoGuardaNada(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "Semana")["id"].(string)
	clienteHTTP.catalogo.caido = true

	codigo, cuerpo := enviarLote(prueba, clienteHTTP, idLista, claveLote, cuerpoLote(productoLeche, 1))
	if codigo != http.StatusServiceUnavailable || cuerpo["mensaje"] != "No se pudo verificar el producto. Intenta de nuevo." {
		prueba.Fatalf("catálogo caído: %d %v", codigo, cuerpo)
	}
	if detalle, _ := clienteHTTP.repo.Obtener(prueba.Context(), usuarioA, idLista); len(detalle.Articulos) != 0 {
		prueba.Fatalf("se guardó sin verificar: %v", detalle.Articulos)
	}
}

// ===== Validación de cabecera y cuerpo =====

func TestLoteRechazaPeticionesInvalidas(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	idLista := crear(prueba, clienteHTTP, "Semana")["id"].(string)
	valido := cuerpoLote(productoLeche, 1)

	casos := []struct{ nombre, clave, cuerpo string }{
		{"sin Idempotency-Key", "", valido},
		{"clave con espacios", "clave con espacios", valido},
		{"clave demasiado larga", strings.Repeat("a", 129), valido},
		{"sin articulos", claveLote, `{}`},
		{"articulos vacío", claveLote, `{"articulos": []}`},
		{"más líneas que el máximo", claveLote, cuerpoLote(productoLeche, 1, productoPan, 1, productoArroz, 1, productoCafe, 1)},
		{"producto_id mal formado", claveLote, cuerpoLote("no-es-uuid", 1)},
		{"sin cantidad", claveLote, fmt.Sprintf(`{"articulos": [{"producto_id": %q}]}`, productoLeche)},
		{"cantidad cero", claveLote, cuerpoLote(productoLeche, 0)},
		{"campo extra", claveLote, fmt.Sprintf(`{"articulos": [{"producto_id": %q, "cantidad": 1, "precio": 1}]}`, productoLeche)},
	}
	for _, caso := range casos {
		if codigo, cuerpo := enviarLote(prueba, clienteHTTP, idLista, caso.clave, caso.cuerpo); codigo != http.StatusBadRequest || cuerpo["mensaje"] != "Solicitud inválida." {
			prueba.Fatalf("%s: %d %v", caso.nombre, codigo, cuerpo)
		}
	}
	if clienteHTTP.catalogo.consultas != 0 {
		prueba.Fatalf("una petición inválida no debe consultar a Catálogo (%d consultas)", clienteHTTP.catalogo.consultas)
	}
	// Otro método sobre /lote responde 405 con el envoltorio común.
	if respuesta := clienteHTTP.pedir(http.MethodGet, rutaLote(idLista), "", nil); respuesta.Code != http.StatusMethodNotAllowed || respuesta.Header().Get("Allow") != "POST" {
		prueba.Fatalf("GET lote: %d %q", respuesta.Code, respuesta.Header().Get("Allow"))
	}
}

// ===== Aislamiento por dueño =====

func TestLoteEnListaAjenaResponde404(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.catalogo.producto(productoLeche, listas.EstadoDisponible)
	idLista := crear(prueba, clienteHTTP, "De A")["id"].(string)
	if codigo, _ := enviarLote(prueba, clienteHTTP, idLista, claveLote, cuerpoLote(productoLeche, 1)); codigo != http.StatusOK {
		prueba.Fatalf("lote de A: %d", codigo)
	}

	// B reutiliza la misma clave sobre la lista de A: 404, sin ver el lote.
	clienteHTTP.jwt = tokenDe(prueba, usuarioB)
	if codigo, cuerpo := enviarLote(prueba, clienteHTTP, idLista, claveLote, cuerpoLote(productoLeche, 1)); codigo != http.StatusNotFound || cuerpo["mensaje"] != "La lista no existe." {
		prueba.Fatalf("lista ajena: %d %v", codigo, cuerpo)
	}
	// Con Catálogo caído tampoco se revela que la lista existe.
	clienteHTTP.catalogo.caido = true
	if codigo, _ := enviarLote(prueba, clienteHTTP, idLista, "otra-clave", cuerpoLote(productoLeche, 1)); codigo != http.StatusNotFound {
		prueba.Fatalf("lista ajena con Catálogo caído: %d", codigo)
	}
	clienteHTTP.catalogo.caido = false
	if codigo, _ := enviarLote(prueba, clienteHTTP, "no-es-uuid", claveLote, cuerpoLote(productoLeche, 1)); codigo != http.StatusNotFound {
		prueba.Fatalf("id inválido: %d", codigo)
	}
}
