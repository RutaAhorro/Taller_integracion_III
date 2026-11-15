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
	"sort"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/queue"
)

// ===== Capacidad del Volumen =====

// ErrStoragePressure indica que el volumen no conserva la reserva mínima
// configurada.
var ErrStoragePressure = errors.New("spool sin espacio de reserva; se conserva para recuperación")

// EnsureCapacity exige que quede el espacio de reserva más el espacio extra
// que se va a escribir.
func EnsureCapacity(directorio string, minimoLibre, extra int64) error {
	libre, errorObtenido := FreeBytes(directorio)
	if errorObtenido != nil {
		return errorObtenido
	}
	if libre < minimoLibre+extra {
		return ErrStoragePressure
	}
	return nil
}

// FreeBytes devuelve el espacio disponible para el usuario, no el tamaño
// total del volumen.
func FreeBytes(directorio string) (int64, error) {
	var infoArchivo syscall.Statfs_t
	if errorObtenido := syscall.Statfs(directorio, &infoArchivo); errorObtenido != nil {
		return 0, errorObtenido
	}
	return int64(infoArchivo.Bavail) * int64(infoArchivo.Bsize), nil
}

// ===== Limpieza de Spools =====

// CleanupSpool borra un spool sólo si ningún proceso mantiene el flock de
// ejecución. Devuelve false si está bloqueado o ya no existe.
func CleanupSpool(directorioEstado, trabajoID string) (bool, error) {
	analizada, errorObtenido := uuid.Parse(trabajoID)
	if errorObtenido != nil {
		return false, nil
	}
	directorio := filepath.Join(directorioEstado, analizada.String())
	informacion, errorObtenido := os.Lstat(directorio)
	if errors.Is(errorObtenido, os.ErrNotExist) {
		return false, nil
	}
	if errorObtenido != nil {
		return false, errorObtenido
	}
	if informacion.Mode()&os.ModeSymlink != 0 || !informacion.IsDir() {
		return false, nil
	}

	rutaBloqueo := filepath.Join(directorio, "execution.lock")
	bloqueo, errorObtenido := os.OpenFile(rutaBloqueo, os.O_CREATE|os.O_RDWR, 0o600)
	if errorObtenido != nil {
		return false, errorObtenido
	}
	defer bloqueo.Close()

	if errorObtenido := syscall.Flock(int(bloqueo.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errorObtenido != nil {
		if errors.Is(errorObtenido, syscall.EWOULDBLOCK) || errors.Is(errorObtenido, syscall.EAGAIN) {
			return false, nil
		}
		return false, errorObtenido
	}
	defer syscall.Flock(int(bloqueo.Fd()), syscall.LOCK_UN) //nolint:errcheck

	if errorObtenido := os.RemoveAll(directorio); errorObtenido != nil {
		return false, errorObtenido
	}
	return true, nil
}

// ===== Recolector =====

// Collector retira spools terminales. Ante una dependencia caída devuelve el
// error y conserva todos los archivos.
type Collector struct {
	stateDir  string
	minFree   int64
	retention time.Duration
	timeout   time.Duration
	logger    *slog.Logger
	// cursor rota la revisión entre llamadas. Sin él, los primeros 20
	// directorios podrían acaparar todos los turnos y los spools antiguos
	// nunca se recolectarían.
	cursor string
}

// NewCollector construye el recolector que retira spools terminales según la
// retención y la presión de disco.
func NewCollector(
	directorioEstado string,
	minimoLibre int64,
	retencion time.Duration,
	tiempoEspera time.Duration,
	registrador *slog.Logger,
) *Collector {
	return &Collector{
		stateDir:  directorioEstado,
		minFree:   minimoLibre,
		retention: retencion,
		timeout:   tiempoEspera,
		logger:    registrador,
	}
}

// Collect revisa hasta limit spools, rotando desde el cursor, y borra los de
// trabajos terminales que ya no tienen mensaje pendiente. Los fallidos se
// conservan durante la retención salvo que haya presión de disco. Devuelve
// cuántos spools eliminó.
func (recolector *Collector) Collect(contexto context.Context, api *controlscraping.Client, cola *queue.Queue, limite int) (int, error) {
	if recolector.retention < 0 {
		return 0, fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	}
	if limite <= 0 {
		limite = 20
	}
	entradas, errorObtenido := os.ReadDir(recolector.stateDir)
	if errorObtenido != nil {
		return 0, errorObtenido
	}
	presion, errorObtenido := recolector.isPressure()
	if errorObtenido != nil {
		return 0, errorObtenido
	}

	var nombres []string
	for _, entrada := range entradas {
		if !entrada.IsDir() {
			continue
		}
		nombre := entrada.Name()
		if _, errorObtenido := uuid.Parse(nombre); errorObtenido != nil {
			continue
		}
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	if len(nombres) == 0 {
		return 0, nil
	}
	inicio := 0
	for indice, nombre := range nombres {
		if nombre > recolector.cursor {
			inicio = indice
			break
		}
	}

	fechaLimite := time.Now().Add(recolector.timeout)
	eliminado := 0
	procesados := 0
	for desplazamiento := 0; desplazamiento < len(nombres) && procesados < limite; desplazamiento++ {
		if time.Now().After(fechaLimite) {
			break
		}
		trabajoID := nombres[(inicio+desplazamiento)%len(nombres)]
		recolector.cursor = trabajoID
		procesados++

		estado, errorObtenido := api.GetWork(contexto, trabajoID)
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
		estadoTrabajo, _ := estado["estado"].(string)
		if estadoTrabajo != "completado" && estadoTrabajo != "fallido" {
			continue
		}
		tieneMensaje, errorObtenido := cola.HasMessage(contexto, trabajoID)
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
		if tieneMensaje {
			continue
		}
		if estadoTrabajo == "fallido" && !presion {
			terminado, valido := estado["finalizado_el"].(string)
			if !valido || terminado == "" {
				continue
			}
			terminadoEn, errorObtenido := parseTimestamp(terminado)
			if errorObtenido != nil {
				continue
			}
			if time.Since(terminadoEn) < recolector.retention {
				continue
			}
		}
		fueEliminado, errorObtenido := CleanupSpool(recolector.stateDir, trabajoID)
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
		if !fueEliminado {
			continue
		}
		eliminado++
		if estadoTrabajo == "fallido" {
			recolector.logger.Warn("worker_failed_spool_removed",
				"trabajo_id", trabajoID,
				"storage_pressure", presion,
			)
		}
		presion, errorObtenido = recolector.isPressure()
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
	}
	return eliminado, nil
}

// isPressure comprueba si el espacio libre en disco es menor al umbral de seguridad.
func (recolector *Collector) isPressure() (bool, error) {
	libre, errorObtenido := FreeBytes(recolector.stateDir)
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return libre < recolector.minFree, nil
}

// parseTimestamp convierte la marca de tiempo de un nombre de archivo a time.Time.
func parseTimestamp(valor string) (time.Time, error) {
	for _, disposicion := range []string{time.RFC3339Nano, time.RFC3339} {
		if analizada, errorObtenido := time.Parse(disposicion, valor); errorObtenido == nil {
			return analizada, nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida: %q", valor)
}
