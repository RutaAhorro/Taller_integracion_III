// Command extract ejecuta un spider contra URLs concretas y escribe un
// producto por línea (JSONL) en stdout. No usa Redis, la API ni el spool:
// sirve para validar extracción contra HTML real sin encolar un trabajo.
package main

import (
	"bufio"
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
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("la URL no puede estar vacía")
	}
	*s = append(*s, value)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var urls stringList
	spiderName := flag.String("spider", spiders.JumboName, "spider a ejecutar")
	file := flag.String("file", "", "archivo con URLs, una por línea")
	product := flag.Bool("product", false, "tratar la URL como ficha de producto")
	enrichEAN := flag.Bool("enrich-ean", false, "consultar la ficha para completar EAN cuando falte (Jumbo)")
	maxPages := flag.Int("max-pages", spiders.DefaultMaxPages, "máximo de páginas por categoría")
	categoryDir := flag.String("category-dir", "", "directorio de categorías; por defecto backend/scraper_go/categories")
	delay := flag.Duration("delay", -1, "pausa entre requests; -1 usa SCRAPER_DOWNLOAD_DELAY_MS")
	timeout := flag.Duration("timeout", 0, "timeout HTTP; 0 usa SCRAPER_DOWNLOAD_TIMEOUT")
	retries := flag.Int("retries", -1, "reintentos HTTP; -1 usa SCRAPER_RETRY_TIMES")
	flag.Var(&urls, "url", "URL de categoría o de producto (se puede repetir)")
	flag.Parse()

	if !spiders.IsKnown(*spiderName) {
		return fmt.Errorf("spider inválido %q; usa uno de: %s", *spiderName, strings.Join(spiders.NewRegistry(nil, "", 1, nil).Names(), ", "))
	}
	if *maxPages < 1 {
		return fmt.Errorf("-max-pages debe ser mayor que cero")
	}

	collected, err := collectURLs(urls, *file)
	if err != nil {
		return err
	}
	if *product && len(collected) != 1 {
		return fmt.Errorf("-product requiere exactamente una URL")
	}

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	if *delay >= 0 {
		cfg.DownloadDelay = *delay
	}
	if *timeout > 0 {
		cfg.DownloadTimeout = *timeout
	}
	if *retries >= 0 {
		cfg.MaxRetries = *retries
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	httpClient := crawler.New(crawler.Options{
		UserAgent:  cfg.UserAgent,
		Delay:      cfg.DownloadDelay,
		Timeout:    cfg.DownloadTimeout,
		MaxSize:    cfg.MaxDownloadSize,
		MaxRetries: cfg.MaxRetries,
		Logger:     logger,
	})

	dir := *categoryDir
	if dir == "" {
		dir = cfg.CategoryDir
		if dir == "" || !directoryExists(dir) {
			for _, candidate := range []string{
				filepath.Join("backend", "scraper_go", "categories"),
				"categories",
			} {
				if directoryExists(candidate) {
					dir = candidate
					break
				}
			}
		}
	}
	if len(collected) > 0 && !*product {
		dir, err = temporaryCategoryDir(*spiderName, collected)
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
	}

	registry := spiders.NewRegistry(httpClient, dir, *maxPages, logger)
	spider, ok := registry.Get(*spiderName)
	if !ok {
		return fmt.Errorf("spider no registrado: %s", *spiderName)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	encoder := json.NewEncoder(os.Stdout)
	emitted := 0
	emit := func(item domain.Item) error {
		item = spiders.CompleteItem(item)
		if err := encoder.Encode(item); err != nil {
			return err
		}
		emitted++
		return nil
	}

	job := domain.Job{
		Spider:    *spiderName,
		EnrichEAN: *enrichEAN,
	}
	if *product {
		job.ProductURL = collected[0]
	}
	if err := spider.Run(ctx, job, emit); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "extraidos=%d\n", emitted)
	return nil
}

func collectURLs(values []string, file string) ([]string, error) {
	result := append([]string(nil), values...)
	if file == "" {
		return result, nil
	}
	handle, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", file, err)
	}
	defer handle.Close()

	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result = append(result, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("leer %s: %w", file, err)
	}
	return result, nil
}

func temporaryCategoryDir(spiderName string, urls []string) (string, error) {
	dir, err := os.MkdirTemp("", "scraper-extract-*")
	if err != nil {
		return "", err
	}
	file := filepath.Join(dir, categoryFileName(spiderName))
	body := strings.Join(urls, "\n") + "\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func categoryFileName(spiderName string) string {
	return strings.TrimSuffix(spiderName, "_rsc") + "_categories.txt"
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
