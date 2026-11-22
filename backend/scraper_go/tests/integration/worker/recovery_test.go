// recovery_test.go verifica la recuperación ante fallos del worker: trabajos
// terminales, reanudación desde spool con crawl_done=1, límite de intentos y
// respeto por execution.lock.
package worker_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	cola "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/cola"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Auxiliar de Presembrado =====

func presembrarSpool(prueba *testing.T, arnes *arnesWorker, sembrar func(*cola.Spool) error) {
	prueba.Helper()
	almacen, errorApertura := cola.Open(arnes.trabajoID, arnes.configuracion.StateDir, arnes.configuracion.MinFreeBytes)
	if errorApertura != nil {
		prueba.Fatalf("cola.Open() error = %v", errorApertura)
	}
	if errorSembrado := sembrar(almacen); errorSembrado != nil {
		_ = almacen.Close()
		prueba.Fatalf("preparar spool: %v", errorSembrado)
	}
	if errorCierre := almacen.Close(); errorCierre != nil {
		prueba.Fatalf("Close() error = %v", errorCierre)
	}
}

// ===== Pruebas de Recuperación y Concurrencia =====

func TestDoesNotCrawlTerminalJob(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.JumboName}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	arnes.estadoControl.Lock()
	arnes.estadoControl.estado = "completado"
	arnes.estadoControl.Unlock()

	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	if spider.cantidadEjecuciones() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces para un trabajo terminal", spider.cantidadEjecuciones())
	}
	arnes.estadoControl.Lock()
	finalizado := arnes.estadoControl.finalizado
	arnes.estadoControl.Unlock()
	if finalizado != "" {
		prueba.Errorf("no se debe finalizar de nuevo un trabajo terminal: %q", finalizado)
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
		prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
	}
	if _, errorEstadistica := os.Stat(filepath.Join(arnes.configuracion.StateDir, arnes.trabajoID)); !os.IsNotExist(errorEstadistica) {
		prueba.Errorf("un trabajo terminal no debe abrir el spool: %v", errorEstadistica)
	}
}

func TestResumesWithCrawlDoneWithoutCrawling(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.JumboName}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	presembrarSpool(prueba, arnes, func(almacen *cola.Spool) error {
		supermercado := "Jumbo"
		if errorAdicion := almacen.Add(dominio.Product{Producto: "Producto recuperado", Precio: punteroFlotante(1000)}, &supermercado); errorAdicion != nil {
			return errorAdicion
		}
		return almacen.SetProgress("crawl_done", 1)
	})

	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	if spider.cantidadEjecuciones() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con crawl_done=1", spider.cantidadEjecuciones())
	}
	arnes.estadoControl.Lock()
	productos := len(arnes.estadoControl.productos)
	finalizado := arnes.estadoControl.finalizado
	arnes.estadoControl.Unlock()
	if productos != 1 {
		prueba.Errorf("productos entregados = %d, esperado 1", productos)
	}
	if finalizado != "completado" {
		prueba.Errorf("estado final = %q, esperado completado", finalizado)
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 0 {
		prueba.Errorf("stream length = %d, esperado 0 (mensaje ACK)", longitud)
	}
}

func TestStopsAfterCrawlAttempts(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.JumboName}
	arnes := nuevoArnesWorker(prueba, spider, nil)
	presembrarSpool(prueba, arnes, func(almacen *cola.Spool) error {
		return almacen.SetProgress("crawl_attempts", arnes.configuracion.CrawlAttempts)
	})

	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	if spider.cantidadEjecuciones() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con los intentos agotados", spider.cantidadEjecuciones())
	}
	arnes.estadoControl.Lock()
	finalizado := arnes.estadoControl.finalizado
	mensajeFinal := arnes.estadoControl.mensajeFinal
	arnes.estadoControl.Unlock()
	if finalizado != "fallido" {
		prueba.Errorf("estado final = %q, esperado fallido", finalizado)
	}
	if !strings.Contains(mensajeFinal, "crawl agotado") {
		prueba.Errorf("registro_errores = %q", mensajeFinal)
	}
	mensajes := arnes.mensajesMuertos(prueba)
	if len(mensajes) != 1 || mensajes[0].Values["reason"] != "attempts_exhausted" {
		prueba.Errorf("dead letter = %+v, esperado reason=attempts_exhausted", mensajes)
	}
	if _, errorEstadistica := os.Stat(filepath.Join(arnes.configuracion.StateDir, arnes.trabajoID)); errorEstadistica != nil {
		prueba.Errorf("el spool debe conservarse cuando se agotan los intentos: %v", errorEstadistica)
	}
}

func TestRespectsExecutionLock(prueba *testing.T) {
	spider := &spiderFalso{nombre: extraccion.JumboName}
	arnes := nuevoArnesWorker(prueba, spider, nil)

	directorio := filepath.Join(arnes.configuracion.StateDir, arnes.trabajoID)
	if errorDirectorio := os.MkdirAll(directorio, 0o700); errorDirectorio != nil {
		prueba.Fatal(errorDirectorio)
	}
	archivoBloqueo, errorApertura := os.OpenFile(filepath.Join(directorio, "execution.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if errorApertura != nil {
		prueba.Fatal(errorApertura)
	}
	defer archivoBloqueo.Close()
	if errorFlock := syscall.Flock(int(archivoBloqueo.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errorFlock != nil {
		prueba.Fatal(errorFlock)
	}
	defer func() {
		_ = syscall.Flock(int(archivoBloqueo.Fd()), syscall.LOCK_UN)
	}()

	arnes.ejecutarTrabajo(prueba, arnes.cargaTrabajo(""))

	if spider.cantidadEjecuciones() != 0 {
		prueba.Errorf("el spider se ejecutó %d veces con el spool bloqueado", spider.cantidadEjecuciones())
	}
	arnes.estadoControl.Lock()
	finalizado := arnes.estadoControl.finalizado
	arnes.estadoControl.Unlock()
	if finalizado != "" {
		prueba.Errorf("no se debe finalizar un trabajo bloqueado: %q", finalizado)
	}
	if pendientes := arnes.mensajesPendientes(prueba); pendientes != 1 {
		prueba.Errorf("mensajes pendientes = %d, esperado 1 (sin ACK)", pendientes)
	}
	if longitud := arnes.longitudFlujo(prueba); longitud != 1 {
		prueba.Errorf("stream length = %d, esperado 1 (sin ACK)", longitud)
	}
}
