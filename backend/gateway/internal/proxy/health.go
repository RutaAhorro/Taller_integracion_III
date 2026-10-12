package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// Diagnostics are separate from readiness: an unavailable domain must not
// withdraw Gateway endpoints for other domains from the Kubernetes Service.
func dependencyChecks(ctx context.Context, client *http.Client, c config.Config, id string) map[string]string {
	results := make(chan map[string]string, 3)
	probe := func(origin *url.URL, path, service string, dependencies map[string]string) {
		checks := map[string]string{service: "error"}
		for _, output := range dependencies {
			checks[output] = "error"
		}
		target := *origin
		target.Path, target.RawPath, target.RawQuery, target.ForceQuery = path, "", "", false
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		request.Header.Set("X-Request-ID", id)
		response, err := client.Do(request)
		if err == nil {
			defer response.Body.Close()
			var health struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if response.StatusCode == 200 && json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&health) == nil && health.Status == "ok" {
				checks[service] = "ok"
				for input, output := range dependencies {
					if health.Checks[input] == "ok" {
						checks[output] = "ok"
					}
				}
			}
		}
		results <- checks
	}
	if c.Scraping != nil {
		go probe(c.Scraping, "/_scraping/ready", "scraping", map[string]string{"database": "scraping_database", "redis": "scraping_redis"})
	} else {
		go probe(c.Backend, "/api/v1/health", "backend", map[string]string{"database": "database", "redis": "redis"})
	}
	count := 1
	if c.Catalogo != nil {
		count++
		go probe(c.Catalogo, "/_catalogo/ready", "catalogo", map[string]string{"database": "catalogo_database"})
	}
	if c.Identidad != nil {
		count++
		go probe(c.Identidad, "/_identidad/ready", "identidad", map[string]string{"database": "identidad_database", "redis": "identidad_redis"})
	}

	checks := make(map[string]string)
	for i := 0; i < count; i++ {
		for key, value := range <-results {
			checks[key] = value
		}
	}
	// Alias de diagnóstico conservados para clientes anteriores al corte.
	if c.Scraping != nil {
		checks["backend"], checks["database"], checks["redis"] = checks["scraping"], checks["scraping_database"], checks["scraping_redis"]
	}
	return checks
}
