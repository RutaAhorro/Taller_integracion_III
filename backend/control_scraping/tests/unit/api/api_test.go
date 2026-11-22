// Pruebas HTTP del router de Control con servicios sobre dobles en memoria:
// autenticación de servicio, CORS, formato de errores y contratos de cada ruta.
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/tests/dobles"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ===== Preparación =====

const (
	tokenServicio   = "ssssssssssssssssssssssssssssssss"
	origenPermitido = "https://frontend.example"
)

// escenario reúne el router y los dobles de una prueba.
type escenario struct {
	enrutador   *gin.Engine
	trabajos    *dobles.Trabajos
	catalogo    *dobles.Catalogo
	entregas    *dobles.Entregas
	ejecuciones *dobles.Ejecuciones
}

// nuevoEscenario crea la cadena 1 (Jumbo, activa) con la sucursal 10.
func nuevoEscenario() escenario {
	gin.SetMode(gin.TestMode)
	trabajos, catalogo, ejecuciones := dobles.NuevosTrabajos(), dobles.NuevoCatalogo(), &dobles.Ejecuciones{}
	catalogo.AgregarCadena(1, "Jumbo", true)
	catalogo.AgregarSucursal(10, 1)
	entregas := dobles.NuevasEntregas(trabajos)
	modulo := api.NuevoModulo(services.NuevoServicioTrabajos(trabajos, catalogo), services.NuevoServicioEntregas(entregas, catalogo), services.NuevoServicioEjecuciones(ejecuciones, ejecuciones))
	configuracion := config.Configuracion{TokenServicio: tokenServicio, OrigenesPermitidos: []string{origenPermitido}}
	return escenario{api.NuevoEnrutador(configuracion, modulo, nil, nil), trabajos, catalogo, entregas, ejecuciones}
}

// pedir envía una petición autenticada salvo que se indiquen otras cabeceras.
func (actual escenario) pedir(metodo, ruta, cuerpo string, cabeceras map[string]string) *httptest.ResponseRecorder {
	peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Authorization", "Bearer "+tokenServicio)
	for nombre, valor := range cabeceras {
		peticion.Header.Set(nombre, valor)
	}
	respuesta := httptest.NewRecorder()
	actual.enrutador.ServeHTTP(respuesta, peticion)
	return respuesta
}

// decodificar lee el cuerpo JSON de la respuesta.
func decodificar(prueba *testing.T, respuesta *httptest.ResponseRecorder) map[string]any {
	prueba.Helper()
	var cuerpo map[string]any
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorObtenido != nil {
		prueba.Fatalf("JSON inválido: %s", respuesta.Body)
	}
	return cuerpo
}

// ===== Autenticación, CORS y errores genéricos =====

// TestRutasExigenTokenServicio rechaza cada ruta sin el Bearer exacto.
func TestRutasExigenTokenServicio(prueba *testing.T) {
	actual := nuevoEscenario()
	identificador := uuid.NewString()
	rutas := [][2]string{
		{http.MethodPost, "/api/v1/scraper/trabajos"},
		{http.MethodGet, "/api/v1/scraper/trabajos/" + identificador},
		{http.MethodPut, "/api/v1/scraper/trabajos/" + identificador + "/finalizar"},
		{http.MethodPost, "/api/v1/scraper/trabajos/" + identificador + "/ejecutar"},
		{http.MethodPost, "/api/v1/scraper/trabajos/" + identificador + "/productos"},
		{http.MethodPost, "/api/v1/scraper/productos"},
	}
	cabeceras := map[string]string{
		"sin header":         "",
		"token incorrecto":   "Bearer " + strings.Repeat("x", 32),
		"sin esquema Bearer": tokenServicio,
		"espacio extra":      "Bearer  " + tokenServicio,
	}
	for _, ruta := range rutas {
		for nombre, valor := range cabeceras {
			prueba.Run(ruta[0]+" "+ruta[1]+" "+nombre, func(prueba *testing.T) {
				respuesta := actual.pedir(ruta[0], ruta[1], `{}`, map[string]string{"Authorization": valor, "Origin": origenPermitido})
				if respuesta.Code != http.StatusUnauthorized || respuesta.Header().Get("WWW-Authenticate") == "" {
					prueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
				}
			})
		}
	}
	if respuesta := actual.pedir(http.MethodGet, "/_scraping/live", "", map[string]string{"Authorization": ""}); respuesta.Code != http.StatusOK {
		prueba.Fatalf("liveness no debe exigir token: %d", respuesta.Code)
	}
}

// TestCORSRechazaAntesDelHandler responde 403 sin reflejar orígenes ajenos.
func TestCORSRechazaAntesDelHandler(prueba *testing.T) {
	actual := nuevoEscenario()
	casos := map[string]int{
		"https://evil.example":               http.StatusForbidden,
		"null":                               http.StatusForbidden,
		"https://frontend.example.evil.test": http.StatusForbidden,
		origenPermitido:                      http.StatusBadRequest,
		"":                                   http.StatusBadRequest,
	}
	for origen, esperado := range casos {
		prueba.Run(origen, func(prueba *testing.T) {
			respuesta := actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos", `{}`, map[string]string{"Origin": origen})
			if respuesta.Code != esperado {
				prueba.Fatal(respuesta.Code, respuesta.Body)
			}
			if esperado == http.StatusForbidden && respuesta.Header().Get("Access-Control-Allow-Origin") != "" {
				prueba.Fatal("origen ajeno reflejado")
			}
		})
	}
	preflight := actual.pedir(http.MethodOptions, "/api/v1/scraper/trabajos", "", map[string]string{"Origin": origenPermitido})
	if preflight.Code != http.StatusNoContent || preflight.Header().Get("Access-Control-Allow-Origin") != origenPermitido {
		prueba.Fatalf("preflight: %d %v", preflight.Code, preflight.Header())
	}
}

// TestRutasInexistentesUsanRespuestaError conserva el formato de 404 y 405.
func TestRutasInexistentesUsanRespuestaError(prueba *testing.T) {
	actual := nuevoEscenario()
	for ruta, esperado := range map[[2]string]int{
		{http.MethodGet, "/ruta-inexistente"}: http.StatusNotFound,
		{http.MethodGet, "/api/v1/auth/me"}:   http.StatusNotFound,
		{http.MethodPost, "/_scraping/live"}:  http.StatusMethodNotAllowed,
		{http.MethodGet, "/swagger/doc.json"}: http.StatusNotFound,
	} {
		respuesta := actual.pedir(ruta[0], ruta[1], "", nil)
		var cuerpo api.RespuestaError
		if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorObtenido != nil || respuesta.Code != esperado || cuerpo.Estado != esperado || cuerpo.Mensaje == "" {
			prueba.Fatalf("%v: %d %s", ruta, respuesta.Code, respuesta.Body)
		}
	}
}

// ===== Ciclo HTTP de un trabajo =====

// TestCicloHTTPDeUnTrabajo inicia, ingesta, consulta y finaliza un trabajo.
func TestCicloHTTPDeUnTrabajo(prueba *testing.T) {
	actual := nuevoEscenario()
	creado := actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":1}`, nil)
	if creado.Code != http.StatusCreated {
		prueba.Fatalf("crear: %d %s", creado.Code, creado.Body)
	}
	ruta := "/api/v1/scraper/trabajos/" + decodificar(prueba, creado)["id"].(string)
	clave := uuid.NewString()
	lote := `{"productos":[{"sku":"SKU-1","producto":"Arroz 1kg","precio_normal":1400}]}`
	ingesta := actual.pedir(http.MethodPost, ruta+"/productos", lote, map[string]string{"Idempotency-Key": clave})
	if ingesta.Code != http.StatusOK || ingesta.Header().Get("Idempotency-Key") != clave || decodificar(prueba, ingesta)["insertados"].(float64) != 1 {
		prueba.Fatalf("ingesta: %d %s", ingesta.Code, ingesta.Body)
	}
	consulta := actual.pedir(http.MethodGet, ruta, "", nil)
	if consulta.Code != http.StatusOK || decodificar(prueba, consulta)["elementos_extraidos"].(float64) != 1 {
		prueba.Fatalf("consulta: %d %s", consulta.Code, consulta.Body)
	}
	cierre := actual.pedir(http.MethodPut, ruta+"/finalizar", `{"estado":"completado"}`, nil)
	if cierre.Code != http.StatusOK || decodificar(prueba, cierre)["estado"] != domain.EstadoCompletado {
		prueba.Fatalf("cierre: %d %s", cierre.Code, cierre.Body)
	}
	if conflicto := actual.pedir(http.MethodPut, ruta+"/finalizar", `{"estado":"fallido"}`, nil); conflicto.Code != http.StatusConflict {
		prueba.Fatalf("cierre distinto: %d", conflicto.Code)
	}
	if tardio := actual.pedir(http.MethodPost, ruta+"/productos", lote, nil); tardio.Code != http.StatusConflict {
		prueba.Fatalf("lote en trabajo cerrado: %d %s", tardio.Code, tardio.Body)
	}
}

// TestValidacionesHTTP cubre los 400 y 404 de entradas inválidas.
func TestValidacionesHTTP(prueba *testing.T) {
	actual := nuevoEscenario()
	casos := []struct {
		nombre, metodo, ruta, cuerpo string
		cabeceras                    map[string]string
		esperado                     int
	}{
		{"trabajo inválido", http.MethodGet, "/api/v1/scraper/trabajos/invalido", "", nil, http.StatusBadRequest},
		{"trabajo inexistente", http.MethodGet, "/api/v1/scraper/trabajos/" + uuid.NewString(), "", nil, http.StatusNotFound},
		{"creación sin cadena", http.MethodPost, "/api/v1/scraper/trabajos", `{}`, nil, http.StatusBadRequest},
		{"cadena inexistente", http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":99}`, nil, http.StatusNotFound},
		{"finalización inválida", http.MethodPut, "/api/v1/scraper/trabajos/invalido/finalizar", `{}`, nil, http.StatusBadRequest},
		{"estado inválido", http.MethodPut, "/api/v1/scraper/trabajos/invalido/finalizar", `{"estado":"pausado"}`, nil, http.StatusBadRequest},
		{"lote vacío", http.MethodPost, "/api/v1/scraper/productos", `{"productos":[]}`, nil, http.StatusBadRequest},
		{"clave no UUID", http.MethodPost, "/api/v1/scraper/productos", `{}`, map[string]string{"Idempotency-Key": "x"}, http.StatusBadRequest},
		{"ejecución con ID inválido", http.MethodPost, "/api/v1/scraper/trabajos/invalido/ejecutar", `{}`, nil, http.StatusBadRequest},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			if respuesta := actual.pedir(caso.metodo, caso.ruta, caso.cuerpo, caso.cabeceras); respuesta.Code != caso.esperado {
				prueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
			}
		})
	}
}

// TestFuenteDeshabilitada responde 409 con su código al iniciar y al ejecutar.
func TestFuenteDeshabilitada(prueba *testing.T) {
	actual := nuevoEscenario()
	trabajoID := actual.trabajos.Agregar(1, "Jumbo")
	actual.catalogo.AgregarCadena(1, "Jumbo", false)
	for _, respuesta := range []*httptest.ResponseRecorder{
		actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":1}`, nil),
		actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos/"+trabajoID.String()+"/ejecutar", `{"spider":"jumbo_rsc"}`, nil),
	} {
		if respuesta.Code != http.StatusConflict || decodificar(prueba, respuesta)["codigo"] != "fuente_deshabilitada" {
			prueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
		}
	}
	if len(actual.ejecuciones.Registradas) != 0 {
		prueba.Fatal("no se debe encolar una fuente deshabilitada")
	}
}

// TestCatalogoCaidoResponde503y504 traduce la indisponibilidad de Catálogo.
func TestCatalogoCaidoResponde503y504(prueba *testing.T) {
	actual := nuevoEscenario()
	for errorCatalogo, esperado := range map[error]int{domain.ErrCatalogoNoDisponible: http.StatusServiceUnavailable, domain.ErrCatalogoTimeout: http.StatusGatewayTimeout} {
		actual.catalogo.Error = errorCatalogo
		if respuesta := actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos", `{"cadena_id":1}`, nil); respuesta.Code != esperado {
			prueba.Fatalf("%v: HTTP %d", errorCatalogo, respuesta.Code)
		}
	}
}

// TestEntregaPendienteResponde202 indica reintentar con la misma clave.
func TestEntregaPendienteResponde202(prueba *testing.T) {
	actual := nuevoEscenario()
	actual.entregas.LeaseOcupada = true
	lote := `{"supermercado":"Jumbo","productos":[{"sku":"SKU-1","producto":"Leche","precio_normal":1290}]}`
	respuesta := actual.pedir(http.MethodPost, "/api/v1/scraper/productos", lote, nil)
	if respuesta.Code != http.StatusAccepted || respuesta.Header().Get("Retry-After") != "2" || decodificar(prueba, respuesta)["codigo"] != "entrega_pendiente" {
		prueba.Fatalf("HTTP %d %v: %s", respuesta.Code, respuesta.Header(), respuesta.Body)
	}
}

// ===== Ejecución =====

// TestEjecutarGuardaEnOutbox acepta con 202 y arma el payload que lee Scraper Go.
func TestEjecutarGuardaEnOutbox(prueba *testing.T) {
	actual := nuevoEscenario()
	trabajoID := actual.trabajos.Agregar(1, "Jumbo")
	ruta := "/api/v1/scraper/trabajos/" + trabajoID.String() + "/ejecutar"
	cuerpo := `{"spider":" lider_rsc ","producto_url":"https://x.example/p","producto_id":"7","add_url":" https://www.jumbo.cl/despensa ","enrich_ean":true}`
	respuesta := actual.pedir(http.MethodPost, ruta, cuerpo, nil)
	if respuesta.Code != http.StatusAccepted || respuesta.Header().Get("X-Scraper-Queue") != "durable-stream" || decodificar(prueba, respuesta)["spider"] != "lider_rsc" {
		prueba.Fatalf("HTTP %d: %s", respuesta.Code, respuesta.Body)
	}
	if len(actual.ejecuciones.Registradas) != 1 || actual.ejecuciones.Registradas[0].TrabajoID != trabajoID {
		prueba.Fatalf("registradas=%+v", actual.ejecuciones.Registradas)
	}
	var carga map[string]any
	if errorObtenido := json.Unmarshal([]byte(actual.ejecuciones.Registradas[0].Payload), &carga); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	esperado := map[string]any{"trabajo_id": trabajoID.String(), "spider": "lider_rsc", "product_url": "https://x.example/p", "product_id": "7", "add_url": "https://www.jumbo.cl/despensa", "enrich_ean": true}
	for clave, valor := range esperado {
		if carga[clave] != valor {
			prueba.Errorf("%s=%v; esperado %v", clave, carga[clave], valor)
		}
	}
	if len(carga) != len(esperado) {
		prueba.Errorf("payload con claves extra: %v", carga)
	}
}

// TestEjecutarValidaSolicitud cubre spider por defecto, rechazos y estado del trabajo.
func TestEjecutarValidaSolicitud(prueba *testing.T) {
	actual := nuevoEscenario()
	ruta := "/api/v1/scraper/trabajos/" + actual.trabajos.Agregar(1, "Jumbo").String() + "/ejecutar"
	if respuesta := actual.pedir(http.MethodPost, ruta, `{}`, nil); respuesta.Code != http.StatusAccepted || decodificar(prueba, respuesta)["spider"] != "jumbo_rsc" {
		prueba.Fatalf("spider por defecto: %d %s", respuesta.Code, respuesta.Body)
	}
	for _, cuerpo := range []string{`{"spider":"otro"}`, `{"add_url":"   "}`, `{"producto_id":"7"}`, `no-json`} {
		if respuesta := actual.pedir(http.MethodPost, ruta, cuerpo, nil); respuesta.Code != http.StatusBadRequest {
			prueba.Fatalf("%s: HTTP %d", cuerpo, respuesta.Code)
		}
	}
	if len(actual.ejecuciones.Registradas) != 1 {
		prueba.Fatalf("sólo la solicitud válida se encola: %d", len(actual.ejecuciones.Registradas))
	}
	sinCadena := "/api/v1/scraper/trabajos/" + actual.trabajos.Agregar(99, "").String() + "/ejecutar"
	if respuesta := actual.pedir(http.MethodPost, sinCadena, `{}`, nil); respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("cadena inexistente: %d", respuesta.Code)
	}
	cerradoID := actual.trabajos.Agregar(1, "Jumbo")
	actual.trabajos.Guardados[cerradoID].Estado = domain.EstadoFallido
	if respuesta := actual.pedir(http.MethodPost, "/api/v1/scraper/trabajos/"+cerradoID.String()+"/ejecutar", `{}`, nil); respuesta.Code != http.StatusConflict {
		prueba.Fatalf("trabajo cerrado: %d", respuesta.Code)
	}
}
