package integration_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

// crearRepositorioSeco captura las consultas públicas sin acceder a implementaciones internas.
// Las consultas generadas se capturan con un controlador SQL que devuelve filas vacías.
type conectorSQLSinBase struct{}

type conexionSQLSinBase struct{}

type filasSQLSinBase struct{}

func (conectorSQLSinBase) Connect(context.Context) (driver.Conn, error) {
	return conexionSQLSinBase{}, nil
}

func (conectorSQLSinBase) Driver() driver.Driver {
	return controladorSQLSinBase{}
}

func (controladorSQLSinBase) Open(string) (driver.Conn, error) {
	return conexionSQLSinBase{}, nil
}

type controladorSQLSinBase struct{}

func (conexionSQLSinBase) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("no se permiten consultas preparadas")
}

func (conexionSQLSinBase) Close() error { return nil }

func (conexionSQLSinBase) Begin() (driver.Tx, error) {
	return nil, errors.New("no se permiten transacciones")
}

func (conexionSQLSinBase) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return filasSQLSinBase{}, nil
}

func (filasSQLSinBase) Columns() []string         { return nil }
func (filasSQLSinBase) Close() error              { return nil }
func (filasSQLSinBase) Next([]driver.Value) error { return io.EOF }

func crearRepositorioSeco(prueba *testing.T) (domain.ProductoRepository, *strings.Builder) {
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
	return repositories.NewProductoRepository(db), &consultas
}

func TestOrdenSeguro_PreciosFaltantesNullsLast(prueba *testing.T) {
	casos := []struct {
		nombre       string
		filtro       domain.FiltroProductosDTO
		debeContener []string
		noContener   []string
	}{
		{
			nombre: "Por defecto (precio asc): NULLS LAST y sin convertir a 0",
			filtro: domain.FiltroProductosDTO{},
			debeContener: []string{
				"COALESCE(cp.precio_oferta, cp.precio_normal)",
				"ASC NULLS LAST",
				"pc.id ASC",
			},
			noContener: []string{", 0)"},
		},
		{
			nombre: "Precio desc: DESC NULLS LAST",
			filtro: domain.FiltroProductosDTO{SortBy: "precio", Order: "desc"},
			debeContener: []string{
				"COALESCE(cp.precio_oferta, cp.precio_normal)",
				"DESC NULLS LAST",
				"pc.id ASC",
			},
			noContener: []string{", 0)"},
		},
		{
			nombre:       "Nombre desc",
			filtro:       domain.FiltroProductosDTO{SortBy: "nombre", Order: "desc"},
			debeContener: []string{"pc.titulo_crudo DESC", "pc.id ASC"},
		},
		{
			nombre:       "Marca asc",
			filtro:       domain.FiltroProductosDTO{SortBy: "marca", Order: "asc"},
			debeContener: []string{"pc.marca_cruda ASC", "pc.id ASC"},
		},
	}

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio, consultas := crearRepositorioSeco(subprueba)
			_, _, errorObtenido := repositorio.ListarParaAdmin(context.Background(), caso.filtro)
			if errorObtenido != nil {
				subprueba.Fatalf("ListarParaAdmin generó error: %v", errorObtenido)
			}
			consultaSQL := consultas.String()
			indiceOrden := strings.LastIndex(consultaSQL, "ORDER BY")
			if indiceOrden >= 0 {
				consultaSQL = consultaSQL[indiceOrden:]
			}
			for _, esperado := range caso.debeContener {
				if !strings.Contains(consultaSQL, esperado) {
					subprueba.Errorf("ListarParaAdmin no contiene '%s', obtuvo: %s", esperado, consultaSQL)
				}
			}
			for _, noEsperado := range caso.noContener {
				if strings.Contains(consultaSQL, noEsperado) {
					subprueba.Errorf("ListarParaAdmin contiene '%s', obtuvo: %s", noEsperado, consultaSQL)
				}
			}
		})
	}
}

func TestListar_OrdenPriorizaStockYPrecios(prueba *testing.T) {
	repositorio, consultas := crearRepositorioSeco(prueba)
	_, _, errorObtenido := repositorio.Listar(context.Background(), domain.FiltroProductosDTO{Page: 3, Limit: 10})
	if errorObtenido != nil {
		prueba.Fatalf("Listar generó error: %v", errorObtenido)
	}

	consultaSQL := consultas.String()
	if !strings.Contains(consultaSQL, "pc.en_stock DESC") {
		prueba.Errorf("Listar no prioriza stock con pc.en_stock DESC. SQL: %s", consultaSQL)
	}
	if !strings.Contains(consultaSQL, "NULLS LAST") {
		prueba.Errorf("Listar no envía precios nulos al final con NULLS LAST. SQL: %s", consultaSQL)
	}
	if !strings.Contains(consultaSQL, "LIMIT 10") || !strings.Contains(consultaSQL, "OFFSET 20") {
		prueba.Errorf("Listar no aplica LIMIT 10 y OFFSET 20. SQL: %s", consultaSQL)
	}
}

func TestObtenerPorID_ValidacionUUIDYFormatoURN(prueba *testing.T) {
	repositorio, consultas := crearRepositorioSeco(prueba)
	contexto := context.Background()

	prueba.Run("ID no válido devuelve nil sin consultar BD", func(subprueba *testing.T) {
		resultado, errorObtenido := repositorio.ObtenerPorID(contexto, "invalido-123")
		if errorObtenido != nil {
			subprueba.Errorf("se esperaba nil error para ID no UUID, se obtuvo: %v", errorObtenido)
		}
		if resultado != nil {
			subprueba.Errorf("se esperaba nil detalle para ID no UUID, se obtuvo: %+v", resultado)
		}
		if consultas.Len() != 0 {
			subprueba.Errorf("un ID inválido consultó la BD: %s", consultas.String())
		}
	})

	prueba.Run("ID en formato URN es normalizado a canónico sin prefijo urn", func(subprueba *testing.T) {
		urnID := "urn:uuid:00000000-0000-4000-8000-000000000002"
		idCanonicoEsperado := "00000000-0000-4000-8000-000000000002"

		identificadorParseado, errorObtenido := uuid.Parse(urnID)
		if errorObtenido != nil {
			subprueba.Fatalf("uuid.Parse no aceptó URN: %v", errorObtenido)
		}
		if identificadorParseado.String() != idCanonicoEsperado {
			subprueba.Errorf("se esperaba canónico %s, se obtuvo %s", idCanonicoEsperado, identificadorParseado.String())
		}

		consultas.Reset()
		_, errorObtenido = repositorio.ObtenerPorID(contexto, urnID)
		if errorObtenido != nil {
			subprueba.Fatalf("ObtenerPorID generó error: %v", errorObtenido)
		}
		consultaSQL := consultas.String()
		if strings.Contains(consultaSQL, "urn:") {
			subprueba.Errorf("la consulta generada contiene 'urn:' en el SQL: %s", consultaSQL)
		}
		if !strings.Contains(consultaSQL, idCanonicoEsperado) || !strings.Contains(consultaSQL, "::uuid") {
			subprueba.Errorf("la consulta no usa el UUID canónico con cast uuid: %s", consultaSQL)
		}
	})

}
