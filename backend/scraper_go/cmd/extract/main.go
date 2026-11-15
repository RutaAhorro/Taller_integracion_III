// Command extract ejecuta un spider contra URLs concretas y escribe un
// producto por línea (JSONL) en stdout. No usa Redis, la API ni el spool:
// sirve para validar extracción contra HTML real sin encolar un trabajo.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/input"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

type stringList []string

// ===== Argumentos CLI =====

// String implementa flag.Value formateando la lista de cadenas.
func (lista *stringList) String() string {
	return strings.Join(*lista, ",")
}

// Set implementa flag.Value acumulando valores pasados por línea de comandos.
func (lista *stringList) Set(valor string) error {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return fmt.Errorf("la URL no puede estar vacía")
	}
	*lista = append(*lista, valor)
	return nil
}

// ===== Ejecución =====

// main es el punto de entrada para extracción puntual desde terminal.
func main() {
	if errorObtenido := run(); errorObtenido != nil {
		fmt.Fprintln(os.Stderr, "error:", errorObtenido)
		os.Exit(1)
	}
}

// run ejecuta el bucle de procesamiento continuo o la tarea solicitada.
func run() error {
	var urls stringList
	nombreSpider := flag.String("spider", spiders.JumboName, "spider a ejecutar")
	archivo := flag.String("file", "", "archivo con URLs, una por línea")
	producto := flag.Bool("product", false, "tratar la URL como ficha de producto")
	enriquecerEAN := flag.Bool("enrich-ean", false, "consultar la ficha para completar EAN cuando falte (Jumbo)")
	maxPaginas := flag.Int("max-pages", config.DefaultMaxPages, "máximo de páginas por categoría")
	directorioCategorias := flag.String("category-dir", "", "directorio de categorías; por defecto backend/scraper_go/categories")
	retraso := flag.Duration("delay", -1, "pausa entre requests; -1 usa SCRAPER_DOWNLOAD_DELAY_MS")
	tiempoEspera := flag.Duration("timeout", 0, "timeout HTTP; 0 usa SCRAPER_DOWNLOAD_TIMEOUT")
	reintentos := flag.Int("retries", -1, "reintentos HTTP; -1 usa SCRAPER_RETRY_TIMES")
	flag.Var(&urls, "url", "URL de categoría o de producto (se puede repetir)")
	flag.Parse()

	if !spiders.IsKnown(*nombreSpider) {
		return fmt.Errorf("spider inválido %q; usa uno de: %s", *nombreSpider, strings.Join(spiders.Names(), ", "))
	}
	if *maxPaginas < 1 {
		return fmt.Errorf("-max-pages debe ser mayor que cero")
	}

	recolectados, errorObtenido := input.Collect(urls, *archivo)
	if errorObtenido != nil {
		return errorObtenido
	}
	if *producto && len(recolectados) != 1 {
		return fmt.Errorf("-product requiere exactamente una URL")
	}

	configuracion, errorObtenido := config.FromEnv()
	if errorObtenido != nil {
		return errorObtenido
	}
	if *retraso >= 0 {
		configuracion.DownloadDelay = *retraso
	}
	if *tiempoEspera > 0 {
		configuracion.DownloadTimeout = *tiempoEspera
	}
	if *reintentos >= 0 {
		configuracion.MaxRetries = *reintentos
	}

	registrador := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: configuracion.LogLevel}))
	clienteHTTP := crawler.New(crawler.Options{
		UserAgent:        configuracion.UserAgent,
		Delay:            configuracion.DownloadDelay,
		Timeout:          configuracion.DownloadTimeout,
		MaxSize:          configuracion.MaxDownloadSize,
		MaxRetries:       configuracion.MaxRetries,
		RobotsCacheTTL:   configuracion.RobotsCacheTTL,
		RobotsFailureTTL: configuracion.RobotsFailureTTL,
		RobotsMaxBytes:   configuracion.RobotsMaxBytes,
		BackoffMin:       configuracion.BackoffMin,
		BackoffMax:       configuracion.BackoffMax,
		Logger:           registrador,
	})

	directorio := *directorioCategorias
	if directorio == "" {
		directorio = configuracion.CategoryDir
		if directorio == "" || !directoryExists(directorio) {
			for _, candidato := range []string{
				filepath.Join("backend", "scraper_go", "categories"),
				"categories",
			} {
				if directoryExists(candidato) {
					directorio = candidato
					break
				}
			}
		}
	}

	registro, errorObtenido := spiders.NewRegistryFromEnv(clienteHTTP, directorio, *maxPaginas, registrador)
	if errorObtenido != nil {
		return errorObtenido
	}
	spider, valido := registro.Get(*nombreSpider)
	if !valido {
		return fmt.Errorf("spider no registrado: %s", *nombreSpider)
	}

	contexto, parada := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parada()

	codificador := json.NewEncoder(os.Stdout)
	emitidos := 0
	emitir := func(elemento domain.Item) error {
		elemento = spiders.CompleteItem(elemento)
		if errorObtenido := codificador.Encode(elemento); errorObtenido != nil {
			return errorObtenido
		}
		emitidos++
		return nil
	}

	trabajo := domain.Job{
		Spider:    *nombreSpider,
		EnrichEAN: *enriquecerEAN,
	}
	if *producto {
		trabajo.ProductURL = recolectados[0]
	} else {
		trabajo.CategoryURLs = recolectados
	}
	if errorObtenido := spider.Run(contexto, trabajo, emitir); errorObtenido != nil {
		return errorObtenido
	}
	fmt.Fprintf(os.Stderr, "extraidos=%d\n", emitidos)
	return nil
}

// ===== Utilidades =====

// directoryExists comprueba la existencia de un directorio en el sistema de archivos.
func directoryExists(ruta string) bool {
	informacion, errorObtenido := os.Stat(ruta)
	return errorObtenido == nil && informacion.IsDir()
}
