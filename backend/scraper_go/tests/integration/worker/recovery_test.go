package worker_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spool"
)

func preseed(t *testing.T, h *harness, seed func(*spool.Spool) error) {
	t.Helper()
	store, err := spool.Open(h.workID, h.cfg.StateDir, h.cfg.MinFreeBytes)
	if err != nil {
		t.Fatalf("spool.Open() error = %v", err)
	}
	if err := seed(store); err != nil {
		_ = store.Close()
		t.Fatalf("preparar spool: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDoesNotCrawlTerminalJob(t *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	h := newHarness(t, spider, nil)
	h.apiState.Lock()
	h.apiState.status = "completado"
	h.apiState.Unlock()

	h.runJob(t, h.jobPayload(""))

	if spider.runCount() != 0 {
		t.Errorf("el spider se ejecutó %d veces para un trabajo terminal", spider.runCount())
	}
	h.apiState.Lock()
	finalized := h.apiState.finalized
	h.apiState.Unlock()
	if finalized != "" {
		t.Errorf("no se debe finalizar de nuevo un trabajo terminal: %q", finalized)
	}
	if length := h.streamLength(t); length != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	if _, err := os.Stat(filepath.Join(h.cfg.StateDir, h.workID)); !os.IsNotExist(err) {
		t.Errorf("un trabajo terminal no debe abrir el spool: %v", err)
	}
}

func TestResumesWithCrawlDoneWithoutCrawling(t *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	h := newHarness(t, spider, nil)
	preseed(t, h, func(store *spool.Spool) error {
		market := "Jumbo"
		if err := store.Add(domain.Product{Producto: "Producto recuperado", Precio: floatPtr(1000)}, &market); err != nil {
			return err
		}
		return store.SetProgress("crawl_done", 1)
	})

	h.runJob(t, h.jobPayload(""))

	if spider.runCount() != 0 {
		t.Errorf("el spider se ejecutó %d veces con crawl_done=1", spider.runCount())
	}
	h.apiState.Lock()
	products := len(h.apiState.products)
	finalized := h.apiState.finalized
	h.apiState.Unlock()
	if products != 1 {
		t.Errorf("productos entregados = %d, want 1", products)
	}
	if finalized != "completado" {
		t.Errorf("estado final = %q, want completado", finalized)
	}
	if length := h.streamLength(t); length != 0 {
		t.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
}

func TestStopsAfterCrawlAttempts(t *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	h := newHarness(t, spider, nil)
	preseed(t, h, func(store *spool.Spool) error {
		return store.SetProgress("crawl_attempts", h.cfg.CrawlAttempts)
	})

	h.runJob(t, h.jobPayload(""))

	if spider.runCount() != 0 {
		t.Errorf("el spider se ejecutó %d veces con los intentos agotados", spider.runCount())
	}
	h.apiState.Lock()
	finalized := h.apiState.finalized
	finalMessage := h.apiState.finalMessage
	h.apiState.Unlock()
	if finalized != "fallido" {
		t.Errorf("estado final = %q, want fallido", finalized)
	}
	if !strings.Contains(finalMessage, "crawl agotado") {
		t.Errorf("registro_errores = %q", finalMessage)
	}
	messages := h.deadLetters(t)
	if len(messages) != 1 || messages[0].Values["reason"] != "attempts_exhausted" {
		t.Errorf("dead letter = %+v, want reason=attempts_exhausted", messages)
	}
	if _, err := os.Stat(filepath.Join(h.cfg.StateDir, h.workID)); err != nil {
		t.Errorf("el spool debe conservarse cuando se agotan los intentos: %v", err)
	}
}

func TestRespectsExecutionLock(t *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	h := newHarness(t, spider, nil)

	directory := filepath.Join(h.cfg.StateDir, h.workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}()

	h.runJob(t, h.jobPayload(""))

	if spider.runCount() != 0 {
		t.Errorf("el spider se ejecutó %d veces con el spool bloqueado", spider.runCount())
	}
	h.apiState.Lock()
	finalized := h.apiState.finalized
	h.apiState.Unlock()
	if finalized != "" {
		t.Errorf("no se debe finalizar un trabajo bloqueado: %q", finalized)
	}
	if pending := h.pendingMessages(t); pending != 1 {
		t.Errorf("mensajes pendientes = %d, want 1 (sin ACK)", pending)
	}
	if length := h.streamLength(t); length != 1 {
		t.Errorf("stream length = %d, want 1 (sin ACK)", length)
	}
}
