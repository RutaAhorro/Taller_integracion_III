package unit_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func decodificar(prueba *testing.T, cuerpo []byte) map[string]any {
	prueba.Helper()
	var respuestaJSON map[string]any
	if errorObtenido := json.Unmarshal(cuerpo, &respuestaJSON); errorObtenido != nil {
		prueba.Fatalf("respuesta no es JSON: %s", cuerpo)
	}
	return respuestaJSON
}

func crear(prueba *testing.T, clienteHTTP *cliente, nombre string) map[string]any {
	prueba.Helper()
	cuerpo, _ := json.Marshal(map[string]string{"nombre": nombre})
	respuesta := clienteHTTP.pedir(http.MethodPost, api, string(cuerpo), nil)
	if respuesta.Code != http.StatusCreated {
		prueba.Fatalf("crear: %d %s", respuesta.Code, respuesta.Body)
	}
	return decodificar(prueba, respuesta.Body.Bytes())
}

func TestCRUDCompletoDelDueno(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	creada := crear(prueba, clienteHTTP, "  Compras   de la semana  ")
	if creada["nombre"] != "Compras de la semana" || creada["estado"] != "activa" || creada["total_articulos"] != 0.0 {
		prueba.Fatalf("lista creada: %v", creada)
	}
	if !strings.HasSuffix(creada["creada_el"].(string), "Z") {
		prueba.Fatalf("creada_el sin UTC: %v", creada["creada_el"])
	}
	idLista := creada["id"].(string)

	detalle := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil).Body.Bytes())
	if articulos, esArreglo := detalle["articulos"].([]any); !esArreglo || len(articulos) != 0 {
		prueba.Fatalf("articulos debe ser [] y no null: %v", detalle["articulos"])
	}

	respuesta := clienteHTTP.pedir(http.MethodPut, api+"/"+idLista, `{"nombre": "Asado ñandú", "estado": "completada"}`, nil)
	actualizada := decodificar(prueba, respuesta.Body.Bytes())
	if respuesta.Code != 200 || actualizada["nombre"] != "Asado ñandú" || actualizada["estado"] != "completada" {
		prueba.Fatalf("actualizar: %d %v", respuesta.Code, actualizada)
	}

	todas := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, api, "", nil).Body.Bytes())["listas"].([]any)
	if len(todas) != 1 || todas[0].(map[string]any)["id"] != idLista {
		prueba.Fatalf("listar: %v", todas)
	}
	if respuesta := clienteHTTP.pedir(http.MethodGet, api+"/", "", nil); respuesta.Code != 200 {
		prueba.Fatalf("barra final: %d", respuesta.Code)
	}
	if respuesta := clienteHTTP.pedir(http.MethodDelete, api+"/"+idLista, "", nil); respuesta.Code != http.StatusNoContent {
		prueba.Fatalf("eliminar: %d", respuesta.Code)
	}
	if respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+idLista, "", nil); respuesta.Code != 404 {
		prueba.Fatalf("tras eliminar: %d", respuesta.Code)
	}
}

func TestListasVaciasSonArregloYNoNull(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	if cuerpo := clienteHTTP.pedir(http.MethodGet, api, "", nil).Body.String(); !strings.Contains(cuerpo, `"listas":[]`) {
		prueba.Fatalf("respuesta: %s", cuerpo)
	}
}

func TestListasAjenasResponden404SinModificarse(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	idLista := crear(prueba, clienteHTTP, "De A")["id"].(string)
	clienteHTTP.jwt = tokenDe(prueba, usuarioB)
	if listasAjenas := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, api, "", nil).Body.Bytes())["listas"].([]any); len(listasAjenas) != 0 {
		prueba.Fatalf("B ve listas de A: %v", listasAjenas)
	}
	for _, caso := range []struct{ metodo, cuerpo string }{{"GET", ""}, {"PUT", `{"nombre": "robada"}`}, {"DELETE", ""}} {
		respuesta := clienteHTTP.pedir(caso.metodo, api+"/"+idLista, caso.cuerpo, nil)
		if respuesta.Code != 404 || strings.TrimSpace(respuesta.Body.String()) != `{"estado":404,"mensaje":"La lista no existe."}` {
			prueba.Fatalf("%s: %d %s", caso.metodo, respuesta.Code, respuesta.Body)
		}
	}
	if clienteHTTP.repo.filas[idLista].lista.Nombre != "De A" {
		prueba.Fatal("la lista ajena se modificó")
	}
}

func TestIDMalFormadoEs404(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	for _, ruta := range []string{"no-es-uuid", "1", "%27%20OR%201=1%20--"} {
		if respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+ruta, "", nil); respuesta.Code != 404 {
			prueba.Fatalf("%s: %d", ruta, respuesta.Code)
		}
	}
}

func TestValidacionConEnvoltorioYSinEcoDelValor(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	malos := []string{`{}`, `{"nombre": ""}`, `{"nombre": "   "}`, fmt.Sprintf(`{"nombre": %q}`, strings.Repeat("x", 151)),
		`{"nombre": "ok", "usuario_id": "` + usuarioB + `"}`, `{"nombre": 5}`, `{"nombre": null}`, `[]`, `{"nombre": "a"} {}`}
	for _, cuerpo := range malos {
		respuesta := clienteHTTP.pedir(http.MethodPost, api, cuerpo, nil)
		respuestaJSON := decodificar(prueba, respuesta.Body.Bytes())
		if respuesta.Code != 400 || respuestaJSON["estado"] != 400.0 || respuestaJSON["mensaje"] != "Solicitud inválida." {
			prueba.Fatalf("%s: %d %v", cuerpo, respuesta.Code, respuestaJSON)
		}
		if strings.Contains(respuesta.Body.String(), usuarioB) {
			prueba.Fatalf("eco del valor recibido: %s", respuesta.Body)
		}
	}
	idLista := crear(prueba, clienteHTTP, "Semana")["id"].(string)
	for _, cuerpo := range []string{`{}`, `{"estado": "borrada"}`, `{"nombre": null, "estado": null}`} {
		if respuesta := clienteHTTP.pedir(http.MethodPut, api+"/"+idLista, cuerpo, nil); respuesta.Code != 400 {
			prueba.Fatalf("PUT %s: %d", cuerpo, respuesta.Code)
		}
	}
}

func TestJSONMalFormadoUsaEnvoltorio(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	respuesta := clienteHTTP.pedir(http.MethodPost, api, "{\"nombre\": \"s\xe1bado\"}", nil)
	if respuesta.Code != 400 || decodificar(prueba, respuesta.Body.Bytes())["mensaje"] != "Solicitud inválida." {
		prueba.Fatalf("UTF-8 corrupto: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestNombreSinCaracteresDeControl(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	if nombre := crear(prueba, clienteHTTP, "Feria\u0000​ del\tsábado")["nombre"]; nombre != "Feria del sábado" {
		prueba.Fatalf("nombre: %q", nombre)
	}
	if nombre := crear(prueba, clienteHTTP, strings.Repeat("ñ", 150))["nombre"]; nombre != strings.Repeat("ñ", 150) {
		prueba.Fatal("150 caracteres con ñ deben caber: se cuentan caracteres, no bytes")
	}
}

func TestTopeDeListasPorUsuario(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	for indice := 0; indice < 100; indice++ {
		crear(prueba, clienteHTTP, fmt.Sprint("lista ", indice))
	}
	if respuesta := clienteHTTP.pedir(http.MethodPost, api, `{"nombre": "una más"}`, nil); respuesta.Code != http.StatusConflict {
		prueba.Fatalf("tope: %d", respuesta.Code)
	}
	clienteHTTP.jwt = tokenDe(prueba, usuarioB)
	crear(prueba, clienteHTTP, "otro usuario no se ve afectado")
}

func TestRutasYMetodosDesconocidosUsanEnvoltorio(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	if respuestaJSON := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, "/api/v1/otra", "", nil).Body.Bytes()); respuestaJSON["estado"] != 404.0 {
		prueba.Fatalf("ruta desconocida: %v", respuestaJSON)
	}
	if respuesta := clienteHTTP.pedir(http.MethodGet, api+"/"+usuarioA+"/extra", "", nil); respuesta.Code != 404 {
		prueba.Fatalf("subruta: %d", respuesta.Code)
	}
	respuesta := clienteHTTP.pedir(http.MethodPatch, api, `{}`, nil)
	if respuesta.Code != 405 || decodificar(prueba, respuesta.Body.Bytes())["estado"] != 405.0 || respuesta.Header().Get("Allow") != "GET, POST" {
		prueba.Fatalf("405: %d %s %q", respuesta.Code, respuesta.Body, respuesta.Header().Get("Allow"))
	}
}

func TestRechazaTokensInseguros(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	firmaHS256 := jwt.SigningMethodHS256
	casos := map[string]string{
		"firma ajena":        token(prueba, firmaHS256, []byte("otra-clave-de-al-menos-treinta-y-dos-bytes"), nil),
		"algoritmo distinto": token(prueba, jwt.SigningMethodHS512, []byte(secreto), nil),
		"alg none":           token(prueba, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, nil),
		"expirado":           token(prueba, firmaHS256, []byte(secreto), map[string]any{"exp": 1}),
		"sin expiración":     token(prueba, firmaHS256, []byte(secreto), map[string]any{"exp": nil}),
		"nbf futuro":         token(prueba, firmaHS256, []byte(secreto), map[string]any{"nbf": int64(1) << 31}),
		"sin rol":            token(prueba, firmaHS256, []byte(secreto), map[string]any{"rol": nil}),
		"rol en blanco":      token(prueba, firmaHS256, []byte(secreto), map[string]any{"rol": "  "}),
		"sin provider":       token(prueba, firmaHS256, []byte(secreto), map[string]any{"provider": nil}),
		"user_id no uuid":    token(prueba, firmaHS256, []byte(secreto), map[string]any{"user_id": "admin"}),
		"user_id no texto":   token(prueba, firmaHS256, []byte(secreto), map[string]any{"user_id": 5}),
		"demasiado largo":    token(prueba, firmaHS256, []byte(secreto), map[string]any{"relleno": strings.Repeat("x", 5000)}),
	}
	for nombre, valor := range casos {
		clienteHTTP.jwt = valor
		if respuesta := clienteHTTP.pedir(http.MethodGet, api, "", nil); respuesta.Code != 401 {
			prueba.Errorf("%s: %d", nombre, respuesta.Code)
		}
	}
	clienteHTTP.jwt = tokenDe(prueba, usuarioA)
	if respuesta := clienteHTTP.pedir(http.MethodGet, api, "", nil); respuesta.Code != 200 {
		prueba.Fatalf("token válido: %d", respuesta.Code)
	}
}

func TestIgnoraIdentidadEnHeaders(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	valido := clienteHTTP.jwt
	clienteHTTP.jwt = ""
	respuesta := clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"X-User-ID": usuarioA, "Authorization": "Bearer " + valido})
	if respuesta.Code != 401 || decodificar(prueba, respuesta.Body.Bytes())["mensaje"] != "Cookie de autenticación requerida." {
		prueba.Fatalf("%d %s", respuesta.Code, respuesta.Body)
	}
}

func TestCORSListaBlanca(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	respuesta := clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"Origin": origen})
	if respuesta.Header().Get("Access-Control-Allow-Origin") != origen || respuesta.Header().Get("Access-Control-Allow-Credentials") != "true" {
		prueba.Fatalf("origen permitido sin cabeceras: %v", respuesta.Header())
	}
	if respuesta := clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"Origin": "https://ajeno.example"}); respuesta.Code != 403 {
		prueba.Fatalf("origen ajeno: %d", respuesta.Code)
	}
	preflight := clienteHTTP.pedir(http.MethodOptions, api, "", map[string]string{"Origin": origen, "Access-Control-Request-Method": "DELETE"})
	if preflight.Code != 204 || !strings.Contains(preflight.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		prueba.Fatalf("preflight: %d %v", preflight.Code, preflight.Header())
	}
}

func TestRequestIDSePropagaOGenera(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	if idLista := clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"X-Request-ID": "abc-123"}).Header().Get("X-Request-ID"); idLista != "abc-123" {
		prueba.Fatalf("propagado: %q", idLista)
	}
	generado := clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"X-Request-ID": "malo id;<script>"}).Header().Get("X-Request-ID")
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(generado) {
		prueba.Fatalf("generado: %q", generado)
	}
	// También las respuestas de CORS rechazado llevan el identificador.
	if clienteHTTP.pedir(http.MethodGet, api, "", map[string]string{"Origin": "https://ajeno.example"}).Header().Get("X-Request-ID") == "" {
		prueba.Fatal("403 de CORS sin X-Request-ID")
	}
}

func TestSondas(prueba *testing.T) {
	clienteHTTP := nuevoCliente(prueba)
	clienteHTTP.jwt = ""
	if respuestaJSON := decodificar(prueba, clienteHTTP.pedir(http.MethodGet, "/_rutas/live", "", nil).Body.Bytes()); respuestaJSON["status"] != "ok" {
		prueba.Fatalf("live: %v", respuestaJSON)
	}
	respuesta := clienteHTTP.pedir(http.MethodGet, "/_rutas/ready", "", nil)
	respuestaJSON := decodificar(prueba, respuesta.Body.Bytes())
	if respuesta.Code != 200 || respuestaJSON["checks"].(map[string]any)["database"] != "ok" {
		prueba.Fatalf("ready: %d %v", respuesta.Code, respuestaJSON)
	}
}
