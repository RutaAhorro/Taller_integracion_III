// Pruebas HTTP del borrador: sesión, cuenta esperada, validación y conflictos sin BD.
package unit_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/httpapi"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// ===== Doble observable =====

// carritosObservados comprueba que HTTP sólo utiliza la cuenta autenticada y no consulta tras rechazos.
type carritosObservados struct {
	usuario       string
	llamadas      int
	errorConsulta error
}

// ObtenerCarrito registra el dueño que recibió del middleware.
func (repositorio *carritosObservados) ObtenerCarrito(_ context.Context, usuarioID string) (listas.Carrito, error) {
	repositorio.usuario, repositorio.llamadas = usuarioID, repositorio.llamadas+1
	return listas.Carrito{Articulos: []listas.ArticuloCarrito{}}, repositorio.errorConsulta
}

// GuardarCarrito registra los datos de autorización, no simula persistencia SQL.
func (repositorio *carritosObservados) GuardarCarrito(contexto context.Context, usuarioID string, _ int64, _ []listas.ArticuloCarrito) (listas.Carrito, error) {
	return repositorio.ObtenerCarrito(contexto, usuarioID)
}

// PrepararCarrito usa el mismo registro para probar el despacho autenticado.
func (repositorio *carritosObservados) PrepararCarrito(contexto context.Context, usuarioID string, _ int64, _ listas.EnvioCarrito) (listas.Carrito, error) {
	return repositorio.ObtenerCarrito(contexto, usuarioID)
}

// ConfirmarCarrito permite probar errores sin llamar al Catálogo real.
func (repositorio *carritosObservados) ConfirmarCarrito(contexto context.Context, usuarioID, _ string, _ listas.Limites, _ listas.Catalogo) (listas.Carrito, error) {
	return repositorio.ObtenerCarrito(contexto, usuarioID)
}

// clienteCarrito conserva el helper de HTTP existente y añade el repositorio específico.
func clienteCarrito(prueba *testing.T, repositorio *carritosObservados) *cliente {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.manejador = httpapi.Nuevo(httpapi.Opciones{
		Repo: clienteHTTP.repo, Carritos: repositorio, Catalogo: clienteHTTP.catalogo,
		JWTSecret: secreto, OrigenesCORS: []string{origen},
		Limites: listas.LimitesContrato, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return clienteHTTP
}

// ===== Autenticación y contrato =====

// TestCarritoExigeSesionYCuentaCoherente cubre también cookies cambiadas por otra pestaña.
func TestCarritoExigeSesionYCuentaCoherente(prueba *testing.T) {
	repositorio := &carritosObservados{}
	clienteHTTP := clienteCarrito(prueba, repositorio)
	for _, ruta := range []string{api + "/carrito", api + "/carrito/preparar", api + "/carrito/confirmar"} {
		metodo := http.MethodPost
		if ruta == api+"/carrito" {
			metodo = http.MethodGet
		}
		clienteHTTP.jwt = ""
		if respuesta := clienteHTTP.pedir(metodo, ruta, "", nil); respuesta.Code != http.StatusUnauthorized {
			prueba.Fatal(respuesta.Code)
		}
		clienteHTTP.jwt = tokenDe(prueba, usuarioA)
		if respuesta := clienteHTTP.pedir(metodo, ruta, "", map[string]string{"X-Carrito-Cuenta": usuarioB}); respuesta.Code != http.StatusUnauthorized {
			prueba.Fatal(respuesta.Code)
		}
	}
	if repositorio.llamadas != 0 {
		prueba.Fatal("una sesión incoherente alcanzó la persistencia")
	}
	respuesta := clienteHTTP.pedir(http.MethodGet, api+"/carrito", "", map[string]string{"X-Carrito-Cuenta": usuarioA})
	if respuesta.Code != http.StatusOK || repositorio.usuario != usuarioA || respuesta.Header().Get("Cache-Control") != "no-store" {
		prueba.Fatal(respuesta.Code, repositorio.usuario)
	}
}

// TestCarritoValidaVersionYArticulos impide cantidades inválidas, dueños y precios en el JSON.
func TestCarritoValidaVersionYArticulos(prueba *testing.T) {
	repositorio := &carritosObservados{}
	clienteHTTP := clienteCarrito(prueba, repositorio)
	for _, cuerpo := range []string{
		`{}`, `{"version":null,"articulos":[]}`, `{"version":-1,"articulos":[]}`,
		`{"version":0,"articulos":null}`, `{"version":0,"articulos":[],"usuario_id":"otro"}`,
		`{"version":0,"articulos":[{"producto_id":"no-uuid","cantidad":1}]}`,
		`{"version":0,"articulos":[{"producto_id":"33333333-3333-4333-8333-333333333333","cantidad":100}]}`,
		`{"version":0,"articulos":[{"producto_id":"33333333-3333-4333-8333-333333333333","cantidad":1.5}]}`,
		`{"version":0,"articulos":[{"producto_id":"33333333-3333-4333-8333-333333333333","cantidad":1,"precio":5}]}`,
		`{"version":0,"articulos":[{"producto_id":"33333333-3333-4333-8333-333333333333","cantidad":1},{"producto_id":"33333333-3333-4333-8333-333333333333","cantidad":1}]}`,
	} {
		respuesta := clienteHTTP.pedir(http.MethodPut, api+"/carrito", cuerpo, map[string]string{"X-Carrito-Cuenta": usuarioA})
		if respuesta.Code != http.StatusBadRequest {
			prueba.Fatalf("%d: %s", respuesta.Code, respuesta.Body.String())
		}
	}
	if repositorio.llamadas != 0 {
		prueba.Fatal("un cuerpo inválido alcanzó la persistencia")
	}
	respuesta := clienteHTTP.pedir(http.MethodPut, api+"/carrito", `{"version":0,"articulos":[]}`, map[string]string{"X-Carrito-Cuenta": usuarioA})
	if respuesta.Code != http.StatusOK {
		prueba.Fatal(respuesta.Body.String())
	}
	repositorio.errorConsulta = listas.ErrVersionCarrito
	respuesta = clienteHTTP.pedir(http.MethodPut, api+"/carrito", `{"version":1,"articulos":[]}`, map[string]string{"X-Carrito-Cuenta": usuarioA})
	if respuesta.Code != http.StatusConflict {
		prueba.Fatal(respuesta.Code)
	}
}
