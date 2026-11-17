package integration_test

import (
	"strings"
	"testing"
)

const testServiceToken = "ssssssssssssssssssssssssssssssss"

func prepareApplicationEnvironment(prueba *testing.T) {
	prueba.Helper()
	prueba.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("prueba", 32))
	prueba.Setenv("SCRAPING_SERVICE_TOKEN", testServiceToken)
	prueba.Setenv("CATALOGO_INGEST_TIMEOUT", "")
	prueba.Setenv("SCRAPING_SWAGGER_ENABLED", "false")
}
