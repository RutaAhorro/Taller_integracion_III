package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScrapingCORSRejectsBeforeBusinessHandler(t *testing.T) {
	t.Setenv("SCRAPING_CORS_ALLOWED_ORIGINS", "https://frontend.example")
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)
	r, err := setupRouter()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		origin string
		status int
	}{
		{"https://evil.example", 403}, {"null", 403}, {"https://frontend.example.evil.test", 403},
		{"https://frontend.example", 400}, {"", 400},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/scraper/trabajos", strings.NewReader(`{}`))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+testServiceToken)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatal(response.Code, response.Body)
			}
			if tc.status == 403 && response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("origen ajeno reflejado")
			}
		})
	}
}

func TestRedisConfigurationNeverFallsBack(t *testing.T) {
	for _, value := range []string{"", "not-a-url", "http://user:do-not-leak@localhost"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("REDIS_URL", value)
			err := initRedis()
			if err == nil || strings.Contains(err.Error(), "do-not-leak") {
				t.Fatal(err)
			}
		})
	}
}
