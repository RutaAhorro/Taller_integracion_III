package integration_test

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

// crearRepositorioEquivalenciasSeco arma la consulta de equivalencias sobre un
// controlador SQL vacío. Sólo se inspecciona el SQL generado, sin base real.
func crearRepositorioEquivalenciasSeco(prueba *testing.T) (domain.RepositorioEquivalencias, *strings.Builder) {
	prueba.Helper()
	var consultas strings.Builder
	baseSQL := sql.OpenDB(conectorSQLSinBase{})
	prueba.Cleanup(func() { _ = baseSQL.Close() })
	db, errorObtenido := gorm.Open(tests.DummyDialector{}, &gorm.Config{
		ConnPool: baseSQL,
		Logger:   logger.New(log.New(&consultas, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if errorObtenido != nil {
		prueba.Fatalf("error inicializando dummy dialector: %v", errorObtenido)
	}
	return repositories.NuevoRepositorioEquivalencias(db), &consultas
}

// TestEquivalenciasRepositorioNormalizaEANYLeeLaUltimaCaptura verifica que la
// consulta use la PK, el LATERAL de la última captura, el EAN normalizado sin
// perder ceros y el cast a uuid[].
func TestEquivalenciasRepositorioNormalizaEANYLeeLaUltimaCaptura(prueba *testing.T) {
	repositorio, consultas := crearRepositorioEquivalenciasSeco(prueba)
	_, errorObtenido := repositorio.ConsultarPorIDs(context.Background(), []string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
	})
	if errorObtenido != nil {
		prueba.Fatalf("ConsultarPorIDs generó error: %v", errorObtenido)
	}

	consultaSQL := consultas.String()
	for _, fragmento := range []string{
		"scraper.productos_crudos pc",
		"LEFT JOIN LATERAL",
		"FROM scraper.capturas_precios",
		"ORDER BY capturado_el DESC",
		"LIMIT 1",
		"regexp_replace",
		"'[^0-9]'",
		"lpad(",
		"14, '0')",
		"pc.id = ANY(",
		"::uuid[]",
		"pc.ean_gtin AS codigo_ean",
		"cp.esta_disponible AS en_stock",
		"cp.precio_normal",
		"cp.capturado_el",
	} {
		if !strings.Contains(consultaSQL, fragmento) {
			prueba.Errorf("la consulta no contiene %q: %s", fragmento, consultaSQL)
		}
	}
	if strings.Contains(consultaSQL, "pc.en_stock") {
		prueba.Errorf("el stock debe venir de la captura validada, no de pc.en_stock: %s", consultaSQL)
	}
	if strings.Contains(consultaSQL, "TRIM(LEADING '0'") {
		prueba.Errorf("el EAN no debe recortar ceros iniciales: %s", consultaSQL)
	}
	if !strings.Contains(consultaSQL, "precio_normal >= 100") {
		prueba.Errorf("la consulta debe ignorar capturas bajo PrecioMinimoValido (100): %s", consultaSQL)
	}
}
