// retencion_test.go verifica la comprobación de capacidad libre en el volumen
// y la eliminación segura de spools mediante flock.
package cola_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/google/uuid"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
)

// ===== Pruebas de Capacidad y Limpieza =====

// TestEnsureCapacityRejectsHugeExtra comprueba que EnsureCapacity devuelva ErrStoragePressure cuando falta espacio.
func TestEnsureCapacityRejectsHugeExtra(prueba *testing.T) {
	errorObtenido := cola.EnsureCapacity(prueba.TempDir(), 1, 1<<62)
	if !errors.Is(errorObtenido, cola.ErrStoragePressure) {
		prueba.Fatalf("EnsureCapacity() error = %v, esperado ErrStoragePressure", errorObtenido)
	}
}

// TestFreeBytesDevuelveUnValorPositivo comprueba la lectura de bytes disponibles con statfs.
func TestFreeBytesDevuelveUnValorPositivo(prueba *testing.T) {
	libre, errorObtenido := cola.FreeBytes(prueba.TempDir())
	if errorObtenido != nil {
		prueba.Fatalf("FreeBytes() error = %v", errorObtenido)
	}
	if libre <= 0 {
		prueba.Errorf("FreeBytes() = %d", libre)
	}
}

// TestCleanupSpoolRespetaElBloqueoDeEjecucion verifica que CleanupSpool no elimine un directorio con flock activo.
func TestCleanupSpoolRespetaElBloqueoDeEjecucion(prueba *testing.T) {
	directorioEstado := prueba.TempDir()
	trabajoID := uuid.NewString()
	directorio := filepath.Join(directorioEstado, trabajoID)
	if errorObtenido := os.MkdirAll(directorio, 0o700); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	bloqueo, errorObtenido := os.OpenFile(filepath.Join(directorio, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if errorObtenido := syscall.Flock(int(bloqueo.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	eliminado, errorObtenido := cola.CleanupSpool(directorioEstado, trabajoID)
	if errorObtenido != nil {
		prueba.Fatalf("CleanupSpool() error = %v", errorObtenido)
	}
	if eliminado {
		prueba.Fatal("CleanupSpool() borró un spool bloqueado")
	}

	if errorObtenido := syscall.Flock(int(bloqueo.Fd()), syscall.LOCK_UN); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if errorObtenido := bloqueo.Close(); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	eliminado, errorObtenido = cola.CleanupSpool(directorioEstado, trabajoID)
	if errorObtenido != nil {
		prueba.Fatalf("CleanupSpool() error = %v", errorObtenido)
	}
	if !eliminado {
		prueba.Fatal("CleanupSpool() no borró un spool libre")
	}
	if _, errorObtenido := os.Stat(directorio); !os.IsNotExist(errorObtenido) {
		prueba.Fatalf("el directorio todavía existe: %v", errorObtenido)
	}
}

// TestCleanupSpoolIgnoraUnSpoolInexistente verifica que CleanupSpool devuelva false sin error si el directorio no existe.
func TestCleanupSpoolIgnoraUnSpoolInexistente(prueba *testing.T) {
	eliminado, errorObtenido := cola.CleanupSpool(prueba.TempDir(), uuid.NewString())
	if errorObtenido != nil {
		prueba.Fatalf("CleanupSpool() error = %v", errorObtenido)
	}
	if eliminado {
		prueba.Fatal("CleanupSpool() informó una eliminación inexistente")
	}
}
