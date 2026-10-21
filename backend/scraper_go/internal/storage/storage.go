// Package storage administra la capacidad del volumen y la retención de los
// spools. Nunca borra un trabajo activo ni un mensaje sin ACK.
package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
)

var ErrStoragePressure = errors.New("spool sin espacio de reserva; se conserva para recuperación")

// EnsureCapacity exige que quede el espacio de reserva más el espacio extra
// que se va a escribir.
func EnsureCapacity(directory string, minFree, extra int64) error {
	free, err := FreeBytes(directory)
	if err != nil {
		return err
	}
	if free < minFree+extra {
		return ErrStoragePressure
	}
	return nil
}

// FreeBytes devuelve el espacio disponible para el usuario, no el tamaño
// total del volumen.
func FreeBytes(directory string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(directory, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// CleanupSpool borra un spool sólo si ningún proceso mantiene el flock de
// ejecución. Devuelve false si está bloqueado o ya no existe.
func CleanupSpool(stateDir, workID string) (bool, error) {
	parsed, err := uuid.Parse(workID)
	if err != nil {
		return false, nil
	}
	directory := filepath.Join(stateDir, parsed.String())
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, nil
	}

	lockPath := filepath.Join(directory, "execution.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer lock.Close()

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck

	if err := os.RemoveAll(directory); err != nil {
		return false, err
	}
	return true, nil
}

// Collector retira spools terminales. Ante una dependencia caída devuelve el
// error y conserva todos los archivos.
type Collector struct {
	stateDir  string
	minFree   int64
	retention time.Duration
	logger    *slog.Logger
}

func NewCollector(stateDir string, minFree int64, retention time.Duration, logger *slog.Logger) *Collector {
	return &Collector{stateDir: stateDir, minFree: minFree, retention: retention, logger: logger}
}

func (c *Collector) Collect(ctx context.Context, api *controlscraping.Client, queue *queue.Queue, limit int) (int, error) {
	if c.retention < 0 {
		return 0, fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	}
	if limit <= 0 {
		limit = 20
	}
	entries, err := os.ReadDir(c.stateDir)
	if err != nil {
		return 0, err
	}
	pressure, err := c.isPressure()
	if err != nil {
		return 0, err
	}

	deadline := time.Now().Add(5 * time.Second)
	removed := 0
	processed := 0
	for _, entry := range entries {
		if processed >= limit || time.Now().After(deadline) {
			break
		}
		if !entry.IsDir() {
			continue
		}
		workID := entry.Name()
		if _, err := uuid.Parse(workID); err != nil {
			continue
		}
		processed++

		status, err := api.GetWork(ctx, workID)
		if err != nil {
			return removed, err
		}
		state, _ := status["estado"].(string)
		if state != "completado" && state != "fallido" {
			continue
		}
		hasMessage, err := queue.HasMessage(ctx, workID)
		if err != nil {
			return removed, err
		}
		if hasMessage {
			continue
		}
		if state == "fallido" && !pressure {
			finished, ok := status["finalizado_el"].(string)
			if !ok || finished == "" {
				continue
			}
			finishedAt, err := parseTimestamp(finished)
			if err != nil {
				continue
			}
			if time.Since(finishedAt) < c.retention {
				continue
			}
		}
		wasRemoved, err := CleanupSpool(c.stateDir, workID)
		if err != nil {
			return removed, err
		}
		if !wasRemoved {
			continue
		}
		removed++
		if state == "fallido" {
			c.logger.Warn("worker_failed_spool_removed",
				"trabajo_id", workID,
				"storage_pressure", pressure,
			)
		}
		pressure, err = c.isPressure()
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (c *Collector) isPressure() (bool, error) {
	free, err := FreeBytes(c.stateDir)
	if err != nil {
		return false, err
	}
	return free < c.minFree, nil
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida: %q", value)
}
