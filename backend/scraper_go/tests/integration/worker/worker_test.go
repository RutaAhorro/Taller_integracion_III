package worker_test

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestProcessesJobEndToEnd(prueba *testing.T) {
	spider := &fakeSpider{
		name: spiders.SantaIsabelName,
		items: []domain.Item{
			{Product: domain.Product{Producto: "Leche", Categoria: "lacteos", Precio: floatPtr(1290)}, Supermercado: "Santa Isabel"},
			{Product: domain.Product{Producto: "Pan", Categoria: "panaderia", Precio: floatPtr(990)}, Supermercado: "Santa Isabel"},
		},
	}
	arnes := newHarness(prueba, spider, nil)
	arnes.runJob(prueba, arnes.jobPayload(""))

	arnes.apiState.Lock()
	finalized := arnes.apiState.finalized
	batches := arnes.apiState.batches
	products := len(arnes.apiState.products)
	supermarket := arnes.apiState.supermarket
	idempotency := arnes.apiState.idempotency
	arnes.apiState.Unlock()

	if finalized != "completado" {
		prueba.Errorf("estado final = %q, want completado", finalized)
	}
	if batches != 1 || products != 2 {
		prueba.Errorf("lotes = %d, productos = %d; want 1/2", batches, products)
	}
	if supermarket != "Santa Isabel" {
		prueba.Errorf("supermercado = %q", supermarket)
	}
	if idempotency == "" {
		prueba.Error("el lote debe enviar Idempotency-Key")
	}
	if length := arnes.streamLength(prueba); length != 0 {
		prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	if messages := arnes.deadLetters(prueba); len(messages) != 0 {
		prueba.Errorf("dead letter = %d mensajes, want 0", len(messages))
	}
	if spider.runCount() != 1 {
		prueba.Errorf("spider ejecutado %d veces, want 1", spider.runCount())
	}
}

func TestSendsInvalidPayloadsToDeadLetter(prueba *testing.T) {
	cases := map[string]func(arnes *harness) string{
		"json inválido": func(*harness) string {
			return `{`
		},
		"trabajo_id inválido": func(*harness) string {
			return `{"trabajo_id":"no-es-uuid","spider":"santa_isabel_rsc"}`
		},
		"spider desconocido": func(arnes *harness) string {
			return `{"trabajo_id":"` + arnes.workID + `","spider":"desconocido"}`
		},
		"product_id sin url": func(arnes *harness) string {
			return `{"trabajo_id":"` + arnes.workID + `","product_id":"123"}`
		},
	}
	for name, payload := range cases {
		prueba.Run(name, func(prueba *testing.T) {
			spider := &fakeSpider{name: spiders.SantaIsabelName}
			arnes := newHarness(prueba, spider, nil)
			arnes.runJob(prueba, payload(arnes))

			if spider.runCount() != 0 {
				prueba.Errorf("el spider se ejecutó %d veces con un payload inválido", spider.runCount())
			}
			arnes.apiState.Lock()
			finalized := arnes.apiState.finalized
			arnes.apiState.Unlock()
			if finalized != "" {
				prueba.Errorf("no se debe finalizar el trabajo: %q", finalized)
			}
			if length := arnes.streamLength(prueba); length != 0 {
				prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
			}
			messages := arnes.deadLetters(prueba)
			if len(messages) != 1 {
				prueba.Fatalf("dead letter = %d mensajes, want 1", len(messages))
			}
			if messages[0].Values["reason"] != "invalid_payload" {
				prueba.Errorf("reason = %v, want invalid_payload", messages[0].Values["reason"])
			}
		})
	}
}

func TestUsesDefaultSpiderWhenMissing(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	arnes := newHarness(prueba, spider, nil)
	arnes.runJob(prueba, `{"trabajo_id":"`+arnes.workID+`"}`)

	if spider.runCount() != 1 {
		prueba.Fatalf("spider ejecutado %d veces, want 1", spider.runCount())
	}
	if job := spider.lastJob(); job.Spider != spiders.JumboName {
		prueba.Errorf("Spider = %q, want %q", job.Spider, spiders.JumboName)
	}
}

func TestParsesEnrichEANFromPayload(prueba *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
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
	for _, tc := range cases {
		prueba.Run(tc.name, func(prueba *testing.T) {
			spider := &fakeSpider{name: spiders.SantaIsabelName}
			arnes := newHarness(prueba, spider, nil)
			arnes.runJob(prueba, arnes.jobPayload(`"enrich_ean":`+tc.value))
			if spider.runCount() != 1 {
				prueba.Fatalf("spider ejecutado %d veces, want 1", spider.runCount())
			}
			if got := spider.lastJob().EnrichEAN; got != tc.want {
				prueba.Errorf("EnrichEAN = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestFinalizesBlockedJob(prueba *testing.T) {
	spider := &fakeSpider{
		name: spiders.SantaIsabelName,
		err: &crawler.BlockedError{
			URL:              "https://www.santaisabel.cl/despensa",
			Status:           429,
			Reason:           "http_429",
			BlockedResponses: 3,
		},
	}
	arnes := newHarness(prueba, spider, nil)
	arnes.runJob(prueba, arnes.jobPayload(""))

	arnes.apiState.Lock()
	defer arnes.apiState.Unlock()
	if arnes.apiState.finalized != "fallido" {
		prueba.Errorf("estado final = %q, want fallido", arnes.apiState.finalized)
	}
	if !strings.Contains(arnes.apiState.finalMessage, "bloqueo: 3 respuestas bloqueadas") {
		prueba.Errorf("registro_errores = %q", arnes.apiState.finalMessage)
	}
	if arnes.apiState.blocked != 3 {
		prueba.Errorf("respuestas_bloqueadas = %d, want 3", arnes.apiState.blocked)
	}
	if arnes.apiState.batches != 0 {
		prueba.Errorf("lotes = %d, want 0", arnes.apiState.batches)
	}
	if length := arnes.streamLength(prueba); length != 0 {
		prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	messages := arnes.deadLetters(prueba)
	if len(messages) != 1 || messages[0].Values["reason"] != "blocked" {
		prueba.Errorf("dead letter = %+v, want reason=blocked", messages)
	}
}

func TestRejectsEmptyCrawl(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.CugatName}
	arnes := newHarness(prueba, spider, nil)
	arnes.runJob(prueba, arnes.jobPayload(""))

	arnes.apiState.Lock()
	defer arnes.apiState.Unlock()
	if arnes.apiState.finalized != "fallido" {
		prueba.Errorf("estado final = %q, want fallido", arnes.apiState.finalized)
	}
	if !strings.Contains(arnes.apiState.finalMessage, "sin productos") {
		prueba.Errorf("registro_errores = %q", arnes.apiState.finalMessage)
	}
	if arnes.apiState.batches != 0 {
		prueba.Errorf("lotes = %d, want 0", arnes.apiState.batches)
	}
	if length := arnes.streamLength(prueba); length != 0 {
		prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	messages := arnes.deadLetters(prueba)
	if len(messages) != 1 || messages[0].Values["reason"] != "no_products" {
		prueba.Errorf("dead letter = %+v, want reason=no_products", messages)
	}
}
