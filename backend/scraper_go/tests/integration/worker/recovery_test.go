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

func preseed(prueba *testing.T, arnes *harness, seed func(*spool.Spool) error) {
	prueba.Helper()
	store, err := spool.Open(arnes.workID, arnes.cfg.StateDir, arnes.cfg.MinFreeBytes)
	if err != nil {
		prueba.Fatalf("spool.Open() error = %v", err)
	}
	if err := seed(store); err != nil {
		_ = store.Close()
		prueba.Fatalf("preparar spool: %v", err)
	}
	if err := store.Close(); err != nil {
		prueba.Fatalf("Close() error = %v", err)
	}
}

func TestDoesNotCrawlTerminalJob(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	arnes := newHarness(prueba, spider, nil)
	arnes.apiState.Lock()
	arnes.apiState.status = "completado"
	arnes.apiState.Unlock()

	arnes.runJob(prueba, arnes.jobPayload(""))

	if spider.runCount() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces para un trabajo terminal", spider.runCount())
	}
	arnes.apiState.Lock()
	finalized := arnes.apiState.finalized
	arnes.apiState.Unlock()
	if finalized != "" {
		prueba.Errorf("no se debe finalizar de nuevo un trabajo terminal: %q", finalized)
	}
	if length := arnes.streamLength(prueba); length != 0 {
		prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
	if _, err := os.Stat(filepath.Join(arnes.cfg.StateDir, arnes.workID)); !os.IsNotExist(err) {
		prueba.Errorf("un trabajo terminal no debe abrir el spool: %v", err)
	}
}

func TestResumesWithCrawlDoneWithoutCrawling(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	arnes := newHarness(prueba, spider, nil)
	preseed(prueba, arnes, func(store *spool.Spool) error {
		market := "Jumbo"
		if err := store.Add(domain.Product{Producto: "Producto recuperado", Precio: floatPtr(1000)}, &market); err != nil {
			return err
		}
		return store.SetProgress("crawl_done", 1)
	})

	arnes.runJob(prueba, arnes.jobPayload(""))

	if spider.runCount() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con crawl_done=1", spider.runCount())
	}
	arnes.apiState.Lock()
	products := len(arnes.apiState.products)
	finalized := arnes.apiState.finalized
	arnes.apiState.Unlock()
	if products != 1 {
		prueba.Errorf("productos entregados = %d, want 1", products)
	}
	if finalized != "completado" {
		prueba.Errorf("estado final = %q, want completado", finalized)
	}
	if length := arnes.streamLength(prueba); length != 0 {
		prueba.Errorf("stream length = %d, want 0 (mensaje ACK)", length)
	}
}

func TestStopsAfterCrawlAttempts(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	arnes := newHarness(prueba, spider, nil)
	preseed(prueba, arnes, func(store *spool.Spool) error {
		return store.SetProgress("crawl_attempts", arnes.cfg.CrawlAttempts)
	})

	arnes.runJob(prueba, arnes.jobPayload(""))

	if spider.runCount() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con los intentos agotados", spider.runCount())
	}
	arnes.apiState.Lock()
	finalized := arnes.apiState.finalized
	finalMessage := arnes.apiState.finalMessage
	arnes.apiState.Unlock()
	if finalized != "fallido" {
		prueba.Errorf("estado final = %q, want fallido", finalized)
	}
	if !strings.Contains(finalMessage, "crawl agotado") {
		prueba.Errorf("registro_errores = %q", finalMessage)
	}
	messages := arnes.deadLetters(prueba)
	if len(messages) != 1 || messages[0].Values["reason"] != "attempts_exhausted" {
		prueba.Errorf("dead letter = %+v, want reason=attempts_exhausted", messages)
	}
	if _, err := os.Stat(filepath.Join(arnes.cfg.StateDir, arnes.workID)); err != nil {
		prueba.Errorf("el spool debe conservarse cuando se agotan los intentos: %v", err)
	}
}

func TestRespectsExecutionLock(prueba *testing.T) {
	spider := &fakeSpider{name: spiders.JumboName}
	arnes := newHarness(prueba, spider, nil)

	directory := filepath.Join(arnes.cfg.StateDir, arnes.workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		prueba.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		prueba.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		prueba.Fatal(err)
	}
	defer func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}()

	arnes.runJob(prueba, arnes.jobPayload(""))

	if spider.runCount() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con el spool bloqueado", spider.runCount())
	}
	arnes.apiState.Lock()
	finalized := arnes.apiState.finalized
	arnes.apiState.Unlock()
	if finalized != "" {
		prueba.Errorf("no se debe finalizar un trabajo bloqueado: %q", finalized)
	}
	if pending := arnes.pendingMessages(prueba); pending != 1 {
		prueba.Errorf("mensajes pendientes = %d, want 1 (sin ACK)", pending)
	}
	if length := arnes.streamLength(prueba); length != 1 {
		prueba.Errorf("stream length = %d, want 1 (sin ACK)", length)
	}
}
