package integration_test

import (
	"strings"
	"testing"
)

const testServiceToken = "ssssssssssssssssssssssssssssssss"

func prepareApplicationEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)
	t.Setenv("CATALOGO_INGEST_TIMEOUT", "")
	t.Setenv("SCRAPING_SWAGGER_ENABLED", "false")
}
