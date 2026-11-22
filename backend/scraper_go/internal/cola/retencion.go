// retencion.go administra la capacidad del volumen y la retención de los
// spools. Nunca borra un trabajo activo ni un mensaje sin ACK.
package cola

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
)

// ===== Capacidad del Volumen =====

// DefaultCollectLimit define el número máximo por defecto de spools evaluados por pasada.
const DefaultCollectLimit = 20

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
	directorioEstado string
	minimoLibre      int64
	retencion        time.Duration
	tiempoEspera     time.Duration
	registrador      *slog.Logger
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
	if registrador == nil {
		registrador = slog.Default()
	}
	return &Collector{
		directorioEstado: directorioEstado,
		minimoLibre:      minimoLibre,
		retencion:        retencion,
		tiempoEspera:     tiempoEspera,
		registrador:      registrador,
	}
}

// Collect revisa hasta limite spools, rotando desde el cursor, y borra los de
// trabajos terminales que ya no tienen mensaje pendiente. Los fallidos se
// conservan durante la retención salvo que haya presión de disco. Devuelve
// cuántos spools eliminó.
func (recolector *Collector) Collect(contexto context.Context, api *Client, clienteCola *Queue, limite int) (int, error) {
	if recolector.retencion < 0 {
		return 0, fmt.Errorf("SCRAPER_FAILED_RETENTION_SECONDS no puede ser negativo")
	}
	if limite <= 0 {
		limite = DefaultCollectLimit
	}
	entradas, errorObtenido := os.ReadDir(recolector.directorioEstado)
	if errorObtenido != nil {
		return 0, errorObtenido
	}
	presion, errorObtenido := recolector.hayPresionDisco()
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

	fechaLimite := time.Now().Add(recolector.tiempoEspera)
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
		tieneMensaje, errorObtenido := clienteCola.HasMessage(contexto, trabajoID)
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
			terminadoEn, errorObtenido := analizarMarcaTiempo(terminado)
			if errorObtenido != nil {
				continue
			}
			if time.Since(terminadoEn) < recolector.retencion {
				continue
			}
		}
		fueEliminado, errorObtenido := CleanupSpool(recolector.directorioEstado, trabajoID)
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
		if !fueEliminado {
			continue
		}
		eliminado++
		if estadoTrabajo == "fallido" {
			recolector.registrador.Warn("worker_failed_spool_removed",
				"trabajo_id", trabajoID,
				"storage_pressure", presion,
			)
		}
		presion, errorObtenido = recolector.hayPresionDisco()
		if errorObtenido != nil {
			return eliminado, errorObtenido
		}
	}
	return eliminado, nil
}

// hayPresionDisco comprueba si el espacio libre en disco es menor al umbral de seguridad.
func (recolector *Collector) hayPresionDisco() (bool, error) {
	libre, errorObtenido := FreeBytes(recolector.directorioEstado)
	if errorObtenido != nil {
		return false, errorObtenido
	}
	return libre < recolector.minimoLibre, nil
}

// analizarMarcaTiempo convierte una marca de tiempo RFC3339 a time.Time.
func analizarMarcaTiempo(valor string) (time.Time, error) {
	for _, disposicion := range []string{time.RFC3339Nano, time.RFC3339} {
		if analizada, errorObtenido := time.Parse(disposicion, valor); errorObtenido == nil {
			return analizada, nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida: %q", valor)
}
