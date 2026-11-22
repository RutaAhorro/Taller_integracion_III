// cliente_control_test.go verifica el contrato HTTP del cliente hacia Control
// de Scraping: ingesta por lotes, idempotencia, redirecciones y cierre de trabajos.
package cola_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Auxiliares de Prueba =====

type cargaIngestaPrueba struct {
	Productos    []dominio.Product `json:"productos"`
	Supermercado *string           `json:"supermercado"`
}

// punteroFlotante crea un puntero a float64 para pruebas de precios.
func punteroFlotante(valor float64) *float64 {
	return &valor
}

// reciboValido construye un recibo de ingesta consistente para cantidad productos.
func reciboValido(cantidad int) map[string]any {
	return map[string]any{
		"total_recibidos":     cantidad,
		"precios_registrados": cantidad,
		"insertados":          cantidad,
		"actualizados":        0,
	}
}

// valorCadena desreferencia un puntero a string o devuelve cadena vacía si es nil.
func valorCadena(puntero *string) string {
	if puntero == nil {
		return ""
	}
	return *puntero
}

// ===== Pruebas de Ingesta de Lotes =====

// TestIngestBatchAndFinalizeWork verifica el flujo completo de envío de lote y cierre de trabajo.
func TestIngestBatchAndFinalizeWork(prueba *testing.T) {
	var (
		recibido     cargaIngestaPrueba
		idempotencia string
		cierre       map[string]any
	)
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		switch {
		case peticion.Method == http.MethodPost && peticion.URL.Path == "/api/v1/scraper/trabajos/work-1/productos":
			if obtenido := peticion.Header.Get("Authorization"); obtenido != "Bearer token" {
				prueba.Errorf("Authorization = %q", obtenido)
			}
			idempotencia = peticion.Header.Get("Idempotency-Key")
			if errorObtenido := json.NewDecoder(peticion.Body).Decode(&recibido); errorObtenido != nil {
				prueba.Errorf("decodificar carga: %v", errorObtenido)
			}
			respuesta.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(respuesta).Encode(reciboValido(len(recibido.Productos)))
		case peticion.Method == http.MethodPut && peticion.URL.Path == "/api/v1/scraper/trabajos/work-1/finalizar":
			if errorObtenido := json.NewDecoder(peticion.Body).Decode(&cierre); errorObtenido != nil {
				prueba.Errorf("decodificar finalizar: %v", errorObtenido)
			}
			respuesta.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(respuesta).Encode(map[string]any{"id": "work-1", "estado": "completado"})
		default:
			http.NotFound(respuesta, peticion)
		}
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "token", 2*time.Second)
	supermercado := "Jumbo"
	productos := []dominio.Product{
		{Producto: "Leche", Categoria: "lacteos", Precio: punteroFlotante(1290)},
		{Producto: "Pan", Categoria: "panaderia"},
	}
	resultado, errorObtenido := cliente.IngestBatch(context.Background(), productos, &supermercado, "work-1", "batch-1")
	if errorObtenido != nil {
		prueba.Fatalf("IngestBatch() error = %v", errorObtenido)
	}
	if idempotencia != "batch-1" {
		prueba.Errorf("Idempotency-Key = %q", idempotencia)
	}
	if recibido.Supermercado == nil || *recibido.Supermercado != "Jumbo" {
		prueba.Errorf("supermercado = %v", recibido.Supermercado)
	}
	if len(recibido.Productos) != 2 || recibido.Productos[1].Precio != nil {
		prueba.Errorf("productos = %+v", recibido.Productos)
	}
	if resultado["insertados"] != float64(2) {
		prueba.Errorf("resultado = %+v", resultado)
	}

	bloqueados := 4
	cerrado, errorObtenido := cliente.FinalizeWork(context.Background(), "work-1", "completado", nil, &bloqueados, "")
	if errorObtenido != nil {
		prueba.Fatalf("FinalizeWork() error = %v", errorObtenido)
	}
	if cerrado["estado"] != "completado" {
		prueba.Errorf("estado = %v", cerrado["estado"])
	}
	if cierre["respuestas_bloqueadas"] != float64(4) {
		prueba.Errorf("respuestas_bloqueadas = %v", cierre["respuestas_bloqueadas"])
	}
}

// TestIngestBatchLimitaLasColumnasDeLaBase verifica el recorte defensivo de campos antes de serializar.
func TestIngestBatchLimitaLasColumnasDeLaBase(prueba *testing.T) {
	var recibido cargaIngestaPrueba
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if errorObtenido := json.NewDecoder(peticion.Body).Decode(&recibido); errorObtenido != nil {
			prueba.Errorf("decodificar carga: %v", errorObtenido)
		}
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(reciboValido(len(recibido.Productos)))
	}))
	defer servidor.Close()

	textoLargo := strings.Repeat("á", 400)
	sku := strings.Repeat("s", 120)
	ean := strings.Repeat("9", 60)
	urlProducto := "https://www.santaisabel.cl/producto-1993238/p"
	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	_, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{
		Producto:          textoLargo,
		SKU:               &sku,
		EanGtin:           &ean,
		Marca:             &textoLargo,
		Categoria:         textoLargo,
		FormatoCrudo:      &textoLargo,
		MecanicaPromocion: &textoLargo,
	}, {
		Producto:    "Agua saborizada",
		URLProducto: &urlProducto,
	}}, nil, "work-1", "batch-1")
	if errorObtenido != nil {
		prueba.Fatalf("IngestBatch() error = %v", errorObtenido)
	}

	if len(recibido.Productos) != 2 {
		prueba.Fatalf("productos = %d", len(recibido.Productos))
	}
	producto := recibido.Productos[0]
	verificaciones := []struct {
		nombre   string
		obtenido string
		maximo   int
	}{
		{"producto", producto.Producto, dominio.MaxProductoLength},
		{"sku", valorCadena(producto.SKU), dominio.MaxCampoCortoLength},
		{"ean_gtin", valorCadena(producto.EanGtin), dominio.MaxEANLength},
		{"marca", valorCadena(producto.Marca), dominio.MaxCampoCortoLength},
		{"categoria", producto.Categoria, dominio.MaxCampoCortoLength},
		{"formato_crudo", valorCadena(producto.FormatoCrudo), dominio.MaxCampoCortoLength},
		{"mecanica_promocion", valorCadena(producto.MecanicaPromocion), dominio.MaxProductoLength},
	}
	for _, verificacion := range verificaciones {
		if obtenido := len([]rune(verificacion.obtenido)); obtenido != verificacion.maximo {
			prueba.Errorf("%s tiene %d caracteres, esperado %d", verificacion.nombre, obtenido, verificacion.maximo)
		}
	}
	if obtenido := recibido.Productos[1].SKU; obtenido == nil || *obtenido != "1993238" {
		prueba.Errorf("sku derivado de URL = %v, esperado 1993238", obtenido)
	}
}

// TestIngestBatchVacioNoLlamaALaAPI verifica que un lote sin productos retorne de inmediato sin tráfico HTTP.
func TestIngestBatchVacioNoLlamaALaAPI(prueba *testing.T) {
	llamadas := 0
	servidor := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		llamadas++
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	resultado, errorObtenido := cliente.IngestBatch(context.Background(), nil, nil, "work-1", "batch-1")
	if errorObtenido != nil {
		prueba.Fatalf("IngestBatch() error = %v", errorObtenido)
	}
	if llamadas != 0 {
		prueba.Errorf("llamadas = %d, esperado 0", llamadas)
	}
	if resultado["total_recibidos"] != 0 {
		prueba.Errorf("resultado = %+v", resultado)
	}
}

// TestIngestBatchRechazaRecibosInconsistentes comprueba la validación de contadores del recibo de idempotencia.
func TestIngestBatchRechazaRecibosInconsistentes(prueba *testing.T) {
	casos := map[string]map[string]any{
		"total distinto": {
			"total_recibidos":     0,
			"precios_registrados": 0,
			"insertados":          0,
			"actualizados":        0,
		},
		"sin campos": {"total_recibidos": 1},
		"suma rota": {
			"total_recibidos":     1,
			"precios_registrados": 2,
			"insertados":          1,
			"actualizados":        0,
		},
	}
	for nombre, cuerpo := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(respuesta).Encode(cuerpo)
			}))
			defer servidor.Close()

			cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
			_, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			if errorObtenido == nil {
				prueba.Fatal("IngestBatch() debió rechazar un recibo inconsistente")
			}
		})
	}
}

// TestIngestBatchRechazaJSONInvalido verifica que una respuesta no JSON produzca un error tipado.
func TestIngestBatchRechazaJSONInvalido(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		_, _ = respuesta.Write([]byte("no es json"))
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1"); errorObtenido == nil {
		prueba.Fatal("IngestBatch() debió rechazar un JSON inválido")
	}
}

// TestIngestBatchEntregaPendienteEsTransitoria verifica que HTTP 202 con codigo entrega_pendiente devuelva Error no permanente.
func TestIngestBatchEntregaPendienteEsTransitoria(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		respuesta.Header().Set("Content-Type", "application/json")
		respuesta.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(respuesta).Encode(map[string]any{"codigo": "entrega_pendiente"})
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	_, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
	var errorAPI *cola.Error
	if !errors.As(errorObtenido, &errorAPI) {
		prueba.Fatalf("error = %T %v, esperado *cola.Error", errorObtenido, errorObtenido)
	}
	if errorAPI.Status != http.StatusAccepted || errorAPI.Code != "entrega_pendiente" {
		prueba.Errorf("Error = %+v, esperado status=202 codigo=entrega_pendiente", errorAPI)
	}
	if errorAPI.Permanent() {
		prueba.Error("entrega_pendiente no debe clasificarse como error permanente")
	}
}

// TestIngestBatchCuerpoVacioConYSiIdempotencia verifica el comportamiento ante HTTP 200 sin cuerpo.
func TestIngestBatchCuerpoVacioConYSiIdempotencia(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		respuesta.WriteHeader(http.StatusOK)
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1"); errorObtenido == nil {
		prueba.Fatal("IngestBatch() con loteID debió fallar ante respuesta vacía")
	}
	resultado, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "")
	if errorObtenido != nil || len(resultado) != 0 {
		prueba.Fatalf("IngestBatch() sin loteID = %v, %v; esperado mapa vacío sin error", resultado, errorObtenido)
	}
}

// TestControlClientNoSigueRedirecciones verifica que el cliente HTTP detenga las redirecciones 302.
func TestControlClientNoSigueRedirecciones(prueba *testing.T) {
	seguido := false
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/destino" {
			seguido = true
			respuesta.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(respuesta, peticion, "/destino", http.StatusFound)
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	_, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
	if errorObtenido == nil {
		prueba.Fatal("IngestBatch() no debió aceptar una redirección 302 como recibo válido")
	}
	if seguido {
		prueba.Fatal("el cliente siguió la redirección HTTP 302")
	}
}

// TestPermanentAPIError verifica la clasificación de errores permanentes por código HTTP y código de contrato.
func TestPermanentAPIError(prueba *testing.T) {
	casos := []struct {
		nombre   string
		estado   int
		codigo   string
		esperado bool
	}{
		{"bad request", http.StatusBadRequest, "", true},
		{"not found", http.StatusNotFound, "", true},
		{"too large", http.StatusRequestEntityTooLarge, "", true},
		{"conflicto de idempotencia", http.StatusConflict, "idempotency_conflict", true},
		{"entrega fallida", http.StatusUnprocessableEntity, "entrega_fallida", true},
		{"entregas fallidas", http.StatusUnprocessableEntity, "entregas_fallidas", true},
		{"lote rechazado", http.StatusUnprocessableEntity, "lote_rechazado", true},
		{"servicio caído", http.StatusServiceUnavailable, "", false},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				respuesta.WriteHeader(caso.estado)
				cuerpo := map[string]any{}
				if caso.codigo != "" {
					cuerpo["codigo"] = caso.codigo
				}
				_ = json.NewEncoder(respuesta).Encode(cuerpo)
			}))
			defer servidor.Close()

			cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
			_, errorObtenido := cliente.IngestBatch(context.Background(), []dominio.Product{{Producto: "Leche"}}, nil, "work-1", "batch-1")
			var errorAPI *cola.Error
			if !errors.As(errorObtenido, &errorAPI) {
				prueba.Fatalf("error = %T %v, esperado *Error", errorObtenido, errorObtenido)
			}
			if obtenido := errorAPI.Permanent(); obtenido != caso.esperado {
				prueba.Errorf("Permanent() = %t, esperado %t (error=%v)", obtenido, caso.esperado, errorAPI)
			}
		})
	}
}

// ===== Pruebas de Consulta y Cierre de Trabajos =====

// TestGetWorkValidaEstadoYIdentidad comprueba la validación de id, estado y JSON en GetWork.
func TestGetWorkValidaEstadoYIdentidad(prueba *testing.T) {
	casos := []struct {
		nombre      string
		carga       map[string]any
		cuerpoCrudo string
		esperaFallo bool
	}{
		{"en progreso", map[string]any{"id": "work-1", "estado": "en_progreso"}, "", false},
		{"completado", map[string]any{"id": "work-1", "estado": "completado"}, "", false},
		{"id ajeno", map[string]any{"id": "otro", "estado": "en_progreso"}, "", true},
		{"estado desconocido", map[string]any{"id": "work-1", "estado": "pendiente"}, "", true},
		{"json inválido", nil, "no-es-json", true},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				respuesta.Header().Set("Content-Type", "application/json")
				if caso.cuerpoCrudo != "" {
					_, _ = respuesta.Write([]byte(caso.cuerpoCrudo))
					return
				}
				_ = json.NewEncoder(respuesta).Encode(caso.carga)
			}))
			defer servidor.Close()

			cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
			_, errorObtenido := cliente.GetWork(context.Background(), "work-1")
			if caso.esperaFallo && errorObtenido == nil {
				prueba.Fatal("GetWork() debió rechazar la respuesta")
			}
			if !caso.esperaFallo && errorObtenido != nil {
				prueba.Fatalf("GetWork() error = %v", errorObtenido)
			}
		})
	}
}

// TestFinalizeWorkEnviaCamposYExigeConfirmacion verifica elementos_extraidos, registro_errores y validación de respuesta.
func TestFinalizeWorkEnviaCamposYExigeConfirmacion(prueba *testing.T) {
	var cuerpoRecibido map[string]any
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		_ = json.NewDecoder(peticion.Body).Decode(&cuerpoRecibido)
		respuesta.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(respuesta).Encode(map[string]any{"id": "work-1", "estado": "fallido"})
	}))
	defer servidor.Close()

	cliente := cola.NewControlClient(servidor.URL+"/api/v1/scraper/productos", "", time.Second)
	extraidos := 15
	bloqueados := 2
	_, errorObtenido := cliente.FinalizeWork(context.Background(), "work-1", "fallido", &extraidos, &bloqueados, "fallo de prueba")
	if errorObtenido != nil {
		prueba.Fatalf("FinalizeWork() error = %v", errorObtenido)
	}
	if cuerpoRecibido["elementos_extraidos"] != float64(15) {
		prueba.Errorf("elementos_extraidos = %v, esperado 15", cuerpoRecibido["elementos_extraidos"])
	}
	if cuerpoRecibido["respuestas_bloqueadas"] != float64(2) {
		prueba.Errorf("respuestas_bloqueadas = %v, esperado 2", cuerpoRecibido["respuestas_bloqueadas"])
	}
	if cuerpoRecibido["registro_errores"] != "fallo de prueba" {
		prueba.Errorf("registro_errores = %v, esperado 'fallo de prueba'", cuerpoRecibido["registro_errores"])
	}

	if _, errorObtenido := cliente.FinalizeWork(context.Background(), "work-1", "completado", nil, nil, ""); errorObtenido == nil {
		prueba.Fatal("FinalizeWork() debió rechazar un cierre cuyo estado no coincide")
	}

	servidorInvalido := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		_, _ = respuesta.Write([]byte("{invalido"))
	}))
	defer servidorInvalido.Close()
	clienteInvalido := cola.NewControlClient(servidorInvalido.URL+"/api/v1/scraper/productos", "", time.Second)
	if _, errorObtenido := clienteInvalido.FinalizeWork(context.Background(), "work-1", "completado", nil, nil, ""); errorObtenido == nil {
		prueba.Fatal("FinalizeWork() debió rechazar JSON inválido")
	}
}
