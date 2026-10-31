package storage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/google/uuid"
)

func TestEnsureCapacityRejectsHugeExtra(t *testing.T) {
	err := EnsureCapacity(t.TempDir(), 1, 1<<62)
	if !errors.Is(err, ErrStoragePressure) {
		t.Fatalf("EnsureCapacity() error = %v, want ErrStoragePressure", err)
	}
}

func TestCleanupSpoolRespectsExecutionLock(t *testing.T) {
	stateDir := t.TempDir()
	workID := uuid.NewString()
	directory := filepath.Join(stateDir, workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(directory, "execution.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanupSpool(stateDir, workID)
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

	removed, err = CleanupSpool(stateDir, workID)
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
