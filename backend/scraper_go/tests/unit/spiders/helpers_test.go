package spiders_test

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/crawler"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/spiders"
)

type fakeFetcher struct {
	responses map[string]*crawler.Response
	errs      map[string]error
	calls     []string
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Response, error) {
	f.calls = append(f.calls, rawURL)
	if err, ok := f.errs[rawURL]; ok {
		return nil, err
	}
	if response, ok := f.responses[rawURL]; ok {
		return response, nil
	}
	return &crawler.Response{URL: mustURL(rawURL), Status: 404}, nil
}

func response(rawURL, body string) *crawler.Response {
	return &crawler.Response{URL: mustURL(rawURL), Status: 200, Body: []byte(body)}
}

func mustURL(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return parsed
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type categoryRunner interface {
	Name() string
	Run(context.Context, domain.Job, spiders.Emitter) error
}

func collect(t *testing.T, spider categoryRunner, job domain.Job) []domain.Item {
	t.Helper()
	var items []domain.Item
	if err := spider.Run(context.Background(), job, func(item domain.Item) error {
		items = append(items, item)
		return nil
	}); err != nil {
		t.Fatalf("%s.Run() error = %v", spider.Name(), err)
	}
	return items
}
