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

// crearRepositorioDisponibilidadSeco arma la consulta de disponibilidad sobre
// un controlador SQL vacío. Sólo se inspecciona el SQL generado, sin base real.
func crearRepositorioDisponibilidadSeco(prueba *testing.T) (domain.RepositorioDisponibilidad, *strings.Builder) {
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
	return repositories.NuevoRepositorioDisponibilidad(db), &consultas
}

// TestDisponibilidadRepositorioUsaLaCapturaMasReciente verifica que la consulta
// por lote use la PK, el LATERAL de la última captura y el cast a uuid[].
func TestDisponibilidadRepositorioUsaLaCapturaMasReciente(prueba *testing.T) {
	repositorio, consultas := crearRepositorioDisponibilidadSeco(prueba)
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
		"pc.id = ANY(",
		"::uuid[]",
		"cp.esta_disponible AS en_stock",
		"cp.precio_normal",
		"cp.capturado_el",
	} {
		if !strings.Contains(consultaSQL, fragmento) {
			prueba.Errorf("la consulta no contiene %q: %s", fragmento, consultaSQL)
		}
	}
	if strings.Contains(consultaSQL, "COALESCE(cp.esta_disponible") {
		prueba.Errorf("el stock NULL no debe convertirse en disponible: %s", consultaSQL)
	}
	if strings.Contains(consultaSQL, "pc.en_stock") {
		prueba.Errorf("el stock debe venir de la captura validada, no de pc.en_stock: %s", consultaSQL)
	}
	if !strings.Contains(consultaSQL, "precio_normal >= 100") {
		prueba.Errorf("la consulta debe ignorar capturas bajo PrecioMinimoValido (100): %s", consultaSQL)
	}
}
