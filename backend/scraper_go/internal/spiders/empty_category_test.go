package spiders

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

type categoryRunner interface {
	Name() string
	Run(context.Context, domain.Job, Emitter) error
}

// TestEmptyCategoryDoesNotAbortChain cubre las cinco cadenas: una categoría
// con respuesta 200 y cero productos debe terminar esa categoría y continuar
// con la siguiente, no abortar el crawl completo.
func TestEmptyCategoryDoesNotAbortChain(t *testing.T) {
	cases := []struct {
		name       string
		file       string
		first      string
		second     string
		firstCall  string
		secondCall string
		factory    func(Fetcher, string, int, *slog.Logger) categoryRunner
	}{
		{
			name:   "jumbo",
			file:   "jumbo_categories.txt",
			first:  "https://www.jumbo.cl/categoria-vacia",
			second: "https://www.jumbo.cl/frutas",
			factory: func(f Fetcher, dir string, pages int, logger *slog.Logger) categoryRunner {
				return NewJumbo(f, dir, pages, logger)
			},
		},
		{
			name:   "santa_isabel",
			file:   "santa_isabel_categories.txt",
			first:  "https://www.santaisabel.cl/categoria-vacia",
			second: "https://www.santaisabel.cl/frutas",
			factory: func(f Fetcher, dir string, pages int, logger *slog.Logger) categoryRunner {
				return NewSantaIsabel(f, dir, pages, logger)
			},
		},
		{
			name:   "cugat",
			file:   "cugat_categories.txt",
			first:  "https://cugat.cl/categoria-producto/vacia/",
			second: "https://cugat.cl/categoria-producto/bebidas/",
			factory: func(f Fetcher, dir string, pages int, logger *slog.Logger) categoryRunner {
				return NewCugat(f, dir, pages, logger)
			},
		},
		{
			name:       "acuenta",
			file:       "acuenta_categories.txt",
			first:      "https://www.acuenta.cl/ca/vacia/01",
			second:     "https://www.acuenta.cl/ca/frutas/02",
			firstCall:  "https://www.acuenta.cl/ca/vacia/01?currentPage=1",
			secondCall: "https://www.acuenta.cl/ca/frutas/02?currentPage=1",
			factory: func(f Fetcher, dir string, pages int, logger *slog.Logger) categoryRunner {
				return NewAcuenta(f, dir, pages, logger)
			},
		},
		{
			name:   "lider",
			file:   "lider_categories.txt",
			first:  "https://super.lider.cl/browse/vacia/12345678",
			second: "https://super.lider.cl/browse/frutas/87654321",
			factory: func(f Fetcher, dir string, pages int, logger *slog.Logger) categoryRunner {
				return NewLider(f, dir, pages, logger)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			categories := tc.first + "\n" + tc.second + "\n"
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(categories), 0o600); err != nil {
				t.Fatal(err)
			}
			firstCall := tc.firstCall
			if firstCall == "" {
				firstCall = tc.first
			}
			secondCall := tc.secondCall
			if secondCall == "" {
				secondCall = tc.second
			}
			fetcher := &fakeFetcher{responses: map[string]*crawler.Response{
				firstCall:  response(firstCall, `<html><body><h1>Sin productos</h1></body></html>`),
				secondCall: response(secondCall, `<html><body><h1>Sin productos</h1></body></html>`),
			}}
			spider := tc.factory(fetcher, dir, 100, quietLogger())
			if err := spider.Run(context.Background(), domain.Job{}, func(domain.Item) error { return nil }); err != nil {
				t.Fatalf("%s abortó el crawl por una categoría vacía: %v", spider.Name(), err)
			}
			if len(fetcher.calls) < 2 || fetcher.calls[0] != firstCall || fetcher.calls[1] != secondCall {
				t.Errorf("%s calls = %v, want la categoría vacía y luego la siguiente", spider.Name(), fetcher.calls)
			}
		})
	}
}
