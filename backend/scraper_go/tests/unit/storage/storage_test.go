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

func TestEnsureCapacityRejectsHugeExtra(prueba *testing.T) {
	err := storage.EnsureCapacity(prueba.TempDir(), 1, 1<<62)
	if !errors.Is(err, storage.ErrStoragePressure) {
		prueba.Fatalf("EnsureCapacity() error = %v, want ErrStoragePressure", err)
	}
}

func TestFreeBytesDevuelveUnValorPositivo(prueba *testing.T) {
	free, err := storage.FreeBytes(prueba.TempDir())
	if err != nil {
		prueba.Fatalf("FreeBytes() error = %v", err)
	}
	if free <= 0 {
		prueba.Errorf("FreeBytes() = %d", free)
	}
}

func TestCleanupSpoolRespetaElBloqueoDeEjecucion(prueba *testing.T) {
	stateDir := prueba.TempDir()
	workID := uuid.NewString()
	directory := filepath.Join(stateDir, workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		prueba.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		prueba.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		prueba.Fatal(err)
	}

	removed, err := storage.CleanupSpool(stateDir, workID)
	if err != nil {
		prueba.Fatalf("CleanupSpool() error = %v", err)
	}
	if removed {
		prueba.Fatal("CleanupSpool() borró un spool bloqueado")
	}

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		prueba.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		prueba.Fatal(err)
	}

	removed, err = storage.CleanupSpool(stateDir, workID)
	if err != nil {
		prueba.Fatalf("CleanupSpool() error = %v", err)
	}
	if !removed {
		prueba.Fatal("CleanupSpool() no borró un spool libre")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		prueba.Fatalf("el directorio todavía existe: %v", err)
	}
}

func TestCleanupSpoolIgnoraUnSpoolInexistente(prueba *testing.T) {
	removed, err := storage.CleanupSpool(prueba.TempDir(), uuid.NewString())
	if err != nil {
		prueba.Fatalf("CleanupSpool() error = %v", err)
	}
	if removed {
		prueba.Fatal("CleanupSpool() informó una eliminación inexistente")
	}
}
