package storage_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/storage"
)

func TestEnsureCapacityRejectsHugeExtra(t *testing.T) {
	err := storage.EnsureCapacity(t.TempDir(), 1, 1<<62)
	if !errors.Is(err, storage.ErrStoragePressure) {
		t.Fatalf("EnsureCapacity() error = %v, want ErrStoragePressure", err)
	}
}

func TestFreeBytesDevuelveUnValorPositivo(t *testing.T) {
	free, err := storage.FreeBytes(t.TempDir())
	if err != nil {
		t.Fatalf("FreeBytes() error = %v", err)
	}
	if free <= 0 {
		t.Errorf("FreeBytes() = %d", free)
	}
}

func TestCleanupSpoolRespetaElBloqueoDeEjecucion(t *testing.T) {
	stateDir := t.TempDir()
	workID := uuid.NewString()
	directory := filepath.Join(stateDir, workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}

	removed, err := storage.CleanupSpool(stateDir, workID)
	if err != nil {
		t.Fatalf("CleanupSpool() error = %v", err)
	}
	if removed {
		t.Fatal("CleanupSpool() borró un spool bloqueado")
	}

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}

	removed, err = storage.CleanupSpool(stateDir, workID)
	if err != nil {
		t.Fatalf("CleanupSpool() error = %v", err)
	}
	if !removed {
		t.Fatal("CleanupSpool() no borró un spool libre")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("el directorio todavía existe: %v", err)
	}
}

func TestCleanupSpoolIgnoraUnSpoolInexistente(t *testing.T) {
	removed, err := storage.CleanupSpool(t.TempDir(), uuid.NewString())
	if err != nil {
		t.Fatalf("CleanupSpool() error = %v", err)
	}
	if removed {
		t.Fatal("CleanupSpool() informó una eliminación inexistente")
	}
}
