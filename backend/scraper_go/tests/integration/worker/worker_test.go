package worker_test

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

func TestProcessesJobEndToEnd(t *testing.T) {
	spider := &fakeSpider{
		name: spiders.SantaIsabelName,
		items: []domain.Item{
			{Product: domain.Product{Producto: "Leche", Categoria: "lacteos", Precio: floatPtr(1290)}, Supermercado: "Santa Isabel"},
			{Product: domain.Product{Producto: "Pan", Categoria: "panaderia", Precio: floatPtr(990)}, Supermercado: "Santa Isabel"},
		},
	}
	h := newHarness(t, spider, nil)
	h.runJob(t, h.jobPayload(""))

	h.apiState.Lock()
	finalized := h.apiState.finalized
	batches := h.apiState.batches
	products := len(h.apiState.products)
	supermarket := h.apiState.supermarket
	idempotency := h.apiState.idempotency
	h.apiState.Unlock()

	if finalized != "completado" {
		t.Errorf("estado final = %q, want completado", finalized)
	}
	if batches != 1 || products != 2 {
		t.Errorf("lotes = %d, productos = %d; want 1/2", batches, products)
	}
	if supermarket != "Santa Isabel" {
		t.Errorf("supermercado = %q", supermarket)
	}
	if idempotency == "" {
		t.Error("el lote debe enviar Idempotency-Key")
	}
	if length := h.streamLength(t); length != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	if messages := h.deadLetters(t); len(messages) != 0 {
		t.Errorf("dead letter = %d mensajes, want 0", len(messages))
	}
	if spider.runCount() != 1 {
		t.Errorf("spider ejecutado %d veces, want 1", spider.runCount())
	}
}

func TestSendsInvalidPayloadsToDeadLetter(t *testing.T) {
	cases := map[string]func(h *harness) string{
		"json inválido": func(*harness) string {
			return `{`
		},
		"trabajo_id inválido": func(*harness) string {
			return `{"trabajo_id":"no-es-uuid","spider":"santa_isabel_rsc"}`
		},
		"spider desconocido": func(h *harness) string {
			return `{"trabajo_id":"` + h.workID + `","spider":"desconocido"}`
		},
		"product_id sin url": func(h *harness) string {
			return `{"trabajo_id":"` + h.workID + `","product_id":"123"}`
		},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			spider := &fakeSpider{name: spiders.SantaIsabelName}
			h := newHarness(t, spider, nil)
			h.runJob(t, payload(h))

			if spider.runCount() != 0 {
				t.Errorf("el spider se ejecutó %d veces con un payload inválido", spider.runCount())
			}
			h.apiState.Lock()
			finalized := h.apiState.finalized
			h.apiState.Unlock()
			if finalized != "" {
				t.Errorf("no se debe finalizar el trabajo: %q", finalized)
			}
			if length := h.streamLength(t); length != 0 {
				t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
			}
			messages := h.deadLetters(t)
			if len(messages) != 1 {
				t.Fatalf("dead letter = %d mensajes, want 1", len(messages))
			}
			if messages[0].Values["reason"] != "invalid_payload" {
				t.Errorf("reason = %v, want invalid_payload", messages[0].Values["reason"])
			}
		})
	}
}

func TestUsesDefaultSpiderWhenMissing(t *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	h := newHarness(t, spider, nil)
	h.runJob(t, `{"trabajo_id":"`+h.workID+`"}`)

	if spider.runCount() != 1 {
		t.Fatalf("spider ejecutado %d veces, want 1", spider.runCount())
	}
	if job := spider.lastJob(); job.Spider != spiders.JumboName {
		t.Errorf("Spider = %q, want %q", job.Spider, spiders.JumboName)
	}
}

func TestParsesEnrichEANFromPayload(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			spider := &fakeSpider{name: spiders.SantaIsabelName}
			h := newHarness(t, spider, nil)
			h.runJob(t, h.jobPayload(`"enrich_ean":`+tc.value))
			if spider.runCount() != 1 {
				t.Fatalf("spider ejecutado %d veces, want 1", spider.runCount())
			}
			if got := spider.lastJob().EnrichEAN; got != tc.want {
				t.Errorf("EnrichEAN = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestFinalizesBlockedJob(t *testing.T) {
	spider := &fakeSpider{
		name: spiders.SantaIsabelName,
		err: &crawler.BlockedError{
			URL:              "https://www.santaisabel.cl/despensa",
			Status:           429,
			Reason:           "http_429",
			BlockedResponses: 3,
		},
	}
	h := newHarness(t, spider, nil)
	h.runJob(t, h.jobPayload(""))

	h.apiState.Lock()
	defer h.apiState.Unlock()
	if h.apiState.finalized != "fallido" {
		t.Errorf("estado final = %q, want fallido", h.apiState.finalized)
	}
	if !strings.Contains(h.apiState.finalMessage, "bloqueo: 3 respuestas bloqueadas") {
		t.Errorf("registro_errores = %q", h.apiState.finalMessage)
	}
	if h.apiState.blocked != 3 {
		t.Errorf("respuestas_bloqueadas = %d, want 3", h.apiState.blocked)
	}
	if h.apiState.batches != 0 {
		t.Errorf("lotes = %d, want 0", h.apiState.batches)
	}
	if length := h.streamLength(t); length != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	messages := h.deadLetters(t)
	if len(messages) != 1 || messages[0].Values["reason"] != "blocked" {
		t.Errorf("dead letter = %+v, want reason=blocked", messages)
	}
}

func TestRejectsEmptyCrawl(t *testing.T) {
	spider := &fakeSpider{name: spiders.CugatName}
	h := newHarness(t, spider, nil)
	h.runJob(t, h.jobPayload(""))

	h.apiState.Lock()
	defer h.apiState.Unlock()
	if h.apiState.finalized != "fallido" {
		t.Errorf("estado final = %q, want fallido", h.apiState.finalized)
	}
	if !strings.Contains(h.apiState.finalMessage, "sin productos") {
		t.Errorf("registro_errores = %q", h.apiState.finalMessage)
	}
	if h.apiState.batches != 0 {
		t.Errorf("lotes = %d, want 0", h.apiState.batches)
	}
	if length := h.streamLength(t); length != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	messages := h.deadLetters(t)
	if len(messages) != 1 || messages[0].Values["reason"] != "no_products" {
		t.Errorf("dead letter = %+v, want reason=no_products", messages)
	}
}
