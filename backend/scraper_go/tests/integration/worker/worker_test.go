// worker_test.go verifica el flujo de extremo a extremo del worker: ingesta de
// lotes, validación de payloads, bloqueo HTTP y rechazo de crawls vacíos.
package worker_test

import (
	"net/http"
	"strings"
	"testing"

	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Ejecución de Extremo a Extremo =====

func TestProcessesJobEndToEnd(prueba *testing.T) {
	spider := &spiderFalso{
		nombre: extraccion.SantaIsabelName,
		elementos: []dominio.Item{
			{Product: dominio.Product{Producto: "Leche", Categoria: "lacteos", Precio: punteroFlotante(1290)}, Supermercado: "Santa Isabel"},
			{Product: dominio.Product{Producto: "Pan", Categoria: "panaderia", Precio: punteroFlotante(990)}, Supermercado: "Santa Isabel"},
		},
	}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	arnes.estadoControl.Lock()
	finalizado := arnes.estadoControl.finalizado
	lotes := arnes.estadoControl.lotes
	productos := len(arnes.estadoControl.productos)
	supermercado := arnes.estadoControl.supermercado
	idempotencia := arnes.estadoControl.idempotencia
	arnes.estadoControl.Unlock()

	if finalizado != "completado" {
		prueba.Errorf("estado final = %q, esperado completado", finalizado)
	}
	if lotes != 1 || productos != 2 {
		prueba.Errorf("lotes = %d, productos = %d; esperado 1/2", lotes, productos)
	}
	if supermercado != "Santa Isabel" {
		prueba.Errorf("supermercado = %q", supermercado)
	}
	if idempotencia == "" {
		prueba.Error("el lote debe enviar Idempotency-Key")
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
		prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
	}
	if mensajes := arnes.mensajesMuertos(prueba); len(mensajes) != 0 {
		prueba.Errorf("dead letter = %d mensajes, esperado 0", len(mensajes))
	}
	if spider.cantidadEjecuciones() != 1 {
		prueba.Errorf("spider ejecutado %d veces, esperado 1", spider.cantidadEjecuciones())
	}
}

// ===== Validación de Payloads y Parámetros =====

func TestSendsInvalidPayloadsToDeadLetter(prueba *testing.T) {
	casos := map[string]func(arnes *arnesWorker) string{
		"json inválido": func(*arnesWorker) string {
			return `{`
		},
		"trabajo_id inválido": func(*arnesWorker) string {
			return `{"trabajo_id":"no-es-uuid","spider":"santa_isabel_rsc"}`
		},
		"spider desconocido": func(arnes *arnesWorker) string {
			return `{"trabajo_id":"` + arnes.trabajoID + `","spider":"desconocido"}`
		},
		"product_id sin url": func(arnes *arnesWorker) string {
			return `{"trabajo_id":"` + arnes.trabajoID + `","product_id":"123"}`
		},
	}
	for nombreCaso, construirCarga := range casos {
		prueba.Run(nombreCaso, func(prueba *testing.T) {
			spider := &spiderFalso{nombre: extraccion.SantaIsabelName}
			arnes := nuevoArnesWorker(prueba, spider, nil)
			arnes.ejecutarTrabajo(prueba, construirCarga(arnes))

			if spider.cantidadEjecuciones() != 0 {
				prueba.Errorf("el spider se ejecutó %d veces con un payload inválido", spider.cantidadEjecuciones())
			}
			arnes.estadoControl.Lock()
			finalizado := arnes.estadoControl.finalizado
			arnes.estadoControl.Unlock()
			if finalizado != "" {
				prueba.Errorf("no se debe finalizar el trabajo: %q", finalizado)
			}
			if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
				prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
			}
			mensajes := arnes.mensajesMuertos(prueba)
			if len(mensajes) != 1 {
				prueba.Fatalf("dead letter = %d mensajes, esperado 1", len(mensajes))
			}
			if mensajes[0].Values["reason"] != "invalid_payload" {
				prueba.Errorf("reason = %v, esperado invalid_payload", mensajes[0].Values["reason"])
			}
		})
	}
}

func TestUsesDefaultSpiderWhenMissing(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.JumboName}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	arnes.ejecutarTrabajo(prueba, `{"trabajo_id":"`+arnes.trabajoID+`"}`)

	if spider.cantidadEjecuciones() != 1 {
		prueba.Fatalf("spider ejecutado %d veces, esperado 1", spider.cantidadEjecuciones())
	}
	if trabajo := spider.trabajoRecibido(); trabajo.Spider != extraccion.JumboName {
		prueba.Errorf("Spider = %q, esperado %q", trabajo.Spider, extraccion.JumboName)
	}
}

func TestParsesEnrichEANFromPayload(prueba *testing.T) {
	casos := []struct {
		nombre   string
		valor    string
		esperado bool
	}{
		{"true", `true`, true},
		{"string true", `"true"`, true},
		{"string 1", `"1"`, true},
		{"string yes", `"yes"`, true},
		{"false", `false`, false},
		{"string false", `"false"`, false},
		{"cero", `0`, false},
		{"uno numérico", `1`, true},
		{"nulo", `null`, false},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			spider := &spiderFalso{nombre: extraccion.SantaIsabelName}
			arnes := nuevoArnesWorker(prueba, spider, nil)
			arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(`"enrich_ean":`+caso.valor))
			if spider.cantidadEjecuciones() != 1 {
				prueba.Fatalf("spider ejecutado %d veces, esperado 1", spider.cantidadEjecuciones())
			}
			if obtenido := spider.trabajoRecibido().EnrichEAN; obtenido != caso.esperado {
				prueba.Errorf("EnrichEAN = %t, esperado %t", obtenido, caso.esperado)
			}
		})
	}
}

// ===== Cierre por Bloqueo o Crawl Vacío =====

func TestFinalizesBlockedJob(prueba *testing.T) {
	spider := &spiderFalso{
		nombre: extraccion.SantaIsabelName,
		errorForzado: &extraccion.BlockedError{
			URL:              "https://www.santaisabel.cl/despensa",
			Status:           http.StatusTooManyRequests,
			Reason:           "http_429",
			BlockedResponses: 3,
		},
	}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	arnes.estadoControl.Lock()
	defer arnes.estadoControl.Unlock()
	if arnes.estadoControl.finalizado != "fallido" {
		prueba.Errorf("estado final = %q, esperado fallido", arnes.estadoControl.finalizado)
	}
	if !strings.Contains(arnes.estadoControl.mensajeFinal, "bloqueo: 3 respuestas bloqueadas") {
		prueba.Errorf("registro_errores = %q", arnes.estadoControl.mensajeFinal)
	}
	if arnes.estadoControl.bloqueadas != 3 {
		prueba.Errorf("respuestas_bloqueadas = %d, esperado 3", arnes.estadoControl.bloqueadas)
	}
	if arnes.estadoControl.lotes != 0 {
		prueba.Errorf("lotes = %d, esperado 0", arnes.estadoControl.lotes)
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
		prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
	}
	mensajes := arnes.mensajesMuertos(prueba)
	if len(mensajes) != 1 || mensajes[0].Values["reason"] != "blocked" {
		prueba.Errorf("dead letter = %+v, esperado reason=blocked", mensajes)
	}
}

func TestRejectsEmptyCrawl(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.CugatName}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	arnes.estadoControl.Lock()
	defer arnes.estadoControl.Unlock()
	if arnes.estadoControl.finalizado != "fallido" {
		prueba.Errorf("estado final = %q, esperado fallido", arnes.estadoControl.finalizado)
	}
	if !strings.Contains(arnes.estadoControl.mensajeFinal, "sin productos") {
		prueba.Errorf("registro_errores = %q", arnes.estadoControl.mensajeFinal)
	}
	if arnes.estadoControl.lotes != 0 {
		prueba.Errorf("lotes = %d, esperado 0", arnes.estadoControl.lotes)
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
		prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
	}
	mensajes := arnes.mensajesMuertos(prueba)
	if len(mensajes) != 1 || mensajes[0].Values["reason"] != "no_products" {
		prueba.Errorf("dead letter = %+v, esperado reason=no_products", mensajes)
	}
}
