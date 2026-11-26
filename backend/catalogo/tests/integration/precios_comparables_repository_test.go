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

// crearRepositorioPreciosSeco inspecciona el SQL generado sin base real, igual
// que la prueba del repositorio de equivalencias.
func crearRepositorioPreciosSeco(prueba *testing.T) (domain.RepositorioPreciosComparables, *strings.Builder) {
	prueba.Helper()
	var consultas strings.Builder
	baseSQL := sql.OpenDB(conectorSQLSinBase{})
	prueba.Cleanup(func() { _ = baseSQL.Close() })
	baseDatos, errorObtenido := gorm.Open(tests.DummyDialector{}, &gorm.Config{
		ConnPool: baseSQL,
		Logger:   logger.New(log.New(&consultas, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if errorObtenido != nil {
		prueba.Fatalf("error inicializando dummy dialector: %v", errorObtenido)
	}
	return repositories.NuevoRepositorioPreciosComparables(baseDatos), &consultas
}

// TestPreciosComparablesRepositorioUsaIndiceGeoespacial comprueba que el radio
// se calcula con ST_DWithin sobre geography y que el ORDER BY usa la distancia
// en kilómetros con un tope acotado.
func TestPreciosComparablesRepositorioUsaIndiceGeoespacial(prueba *testing.T) {
	repositorio, consultas := crearRepositorioPreciosSeco(prueba)
	_, errorObtenido := repositorio.ConsultarCercanas(context.Background(), -38.739, -72.598, 10, 5)
	if errorObtenido != nil {
		prueba.Fatalf("ConsultarCercanas generó error: %v", errorObtenido)
	}

	consultaSQL := consultas.String()
	for _, fragmento := range []string{
		"scraper.sucursales_supermercado ss",
		"ss.esta_activa = true",
		"ss.lat IS NOT NULL AND ss.lon IS NOT NULL",
		"Casa Matriz / Online",
		"ST_DWithin(",
		"ST_MakePoint(ss.lon, ss.lat)",
		"4326)::public.geography",
		"ST_Distance(",
		"distancia_km",
		"ORDER BY distancia_km ASC",
		"LIMIT 5",
	} {
		if !strings.Contains(consultaSQL, fragmento) {
			prueba.Errorf("la consulta no contiene %q: %s", fragmento, consultaSQL)
		}
	}
}

// TestPreciosComparablesRepositorioLeeCoordenadasNulas comprueba que el
// inventario de sucursales conserva NULL para informar coordenadas faltantes.
func TestPreciosComparablesRepositorioLeeCoordenadasNulas(prueba *testing.T) {
	repositorio, consultas := crearRepositorioPreciosSeco(prueba)
	_, errorObtenido := repositorio.ConsultarSucursales(context.Background())
	if errorObtenido != nil {
		prueba.Fatalf("ConsultarSucursales generó error: %v", errorObtenido)
	}

	consultaSQL := consultas.String()
	for _, fragmento := range []string{
		"scraper.sucursales_supermercado ss",
		"ss.esta_activa AS activa",
		"COALESCE(ss.direccion, '') AS direccion",
		"ss.lat",
		"ss.lon",
	} {
		if !strings.Contains(consultaSQL, fragmento) {
			prueba.Errorf("la consulta no contiene %q: %s", fragmento, consultaSQL)
		}
	}
}
