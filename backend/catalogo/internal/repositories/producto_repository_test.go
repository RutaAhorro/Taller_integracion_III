package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/utils/tests"
)

func TestOrdenSeguro_PreciosFaltantesNullsLast(t *testing.T) {
	testsCases := []struct {
		name        string
		filtro      domain.FiltroProductosDTO
		mustHave    []string
		mustNotHave []string
	}{
		{
			name:   "Por defecto (precio asc): NULLS LAST y sin convertir a 0",
			filtro: domain.FiltroProductosDTO{},
			mustHave: []string{
				"COALESCE(cp.precio_oferta, cp.precio_normal)",
				"ASC NULLS LAST",
				"pc.id ASC",
			},
			mustNotHave: []string{", 0)"},
		},
		{
			name: "Precio desc: DESC NULLS LAST",
			filtro: domain.FiltroProductosDTO{
				SortBy: "precio",
				Order:  "desc",
			},
			mustHave: []string{
				"COALESCE(cp.precio_oferta, cp.precio_normal)",
				"DESC NULLS LAST",
				"pc.id ASC",
			},
			mustNotHave: []string{", 0)"},
		},
		{
			name: "Nombre desc",
			filtro: domain.FiltroProductosDTO{
				SortBy: "nombre",
				Order:  "desc",
			},
			mustHave: []string{"pc.titulo_crudo DESC", "pc.id ASC"},
		},
		{
			name: "Marca asc",
			filtro: domain.FiltroProductosDTO{
				SortBy: "marca",
				Order:  "asc",
			},
			mustHave: []string{"pc.marca_cruda ASC", "pc.id ASC"},
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ordenSeguro(tc.filtro)
			for _, exp := range tc.mustHave {
				if !strings.Contains(got, exp) {
					t.Errorf("ordenSeguro() no contiene '%s', obtuvo: %s", exp, got)
				}
			}
			for _, notExp := range tc.mustNotHave {
				if strings.Contains(got, notExp) {
					t.Errorf("ordenSeguro() contiene '%s', obtuvo: %s", notExp, got)
				}
			}
		})
	}
}

func TestListar_OrdenPriorizaStockYPrecios(t *testing.T) {
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("error inicializando dummy dialector: %v", err)
	}

	repo := NewProductoRepository(db).(*gormProductoRepository)
	ctx := context.Background()

	filtro := domain.FiltroProductosDTO{
		Page:  1,
		Limit: 10,
	}

	q := repo.consultaBase(ctx, filtro)
	stmt := paginar(q.Select("pc.id").Order("pc.en_stock DESC").Order(ordenSeguro(filtro)), filtro)
	stmt.Statement.Build(clause.OrderBy{}.Name())

	orderSQL := stmt.Statement.SQL.String()
	if !strings.Contains(orderSQL, "pc.en_stock DESC") {
		t.Errorf("Listar no prioriza stock con pc.en_stock DESC. SQL: %s", orderSQL)
	}
	if !strings.Contains(orderSQL, "NULLS LAST") {
		t.Errorf("Listar no envía precios nulos al final con NULLS LAST. SQL: %s", orderSQL)
	}
}

func TestObtenerPorID_ValidacionUUIDYFormatoURN(t *testing.T) {
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("error inicializando dummy dialector: %v", err)
	}

	repo := NewProductoRepository(db).(*gormProductoRepository)
	ctx := context.Background()

	t.Run("ID no válido devuelve nil sin consultar BD", func(t *testing.T) {
		res, err := repo.ObtenerPorID(ctx, "invalido-123")
		if err != nil {
			t.Errorf("se esperaba nil error para ID no UUID, se obtuvo: %v", err)
		}
		if res != nil {
			t.Errorf("se esperaba nil detalle para ID no UUID, se obtuvo: %+v", res)
		}
	})

	t.Run("ID en formato URN es normalizado a canónico sin prefijo urn", func(t *testing.T) {
		urnID := "urn:uuid:00000000-0000-4000-8000-000000000002"
		expectedCanonical := "00000000-0000-4000-8000-000000000002"

		parsed, err := uuid.Parse(urnID)
		if err != nil {
			t.Fatalf("uuid.Parse no aceptó URN: %v", err)
		}
		if parsed.String() != expectedCanonical {
			t.Errorf("se esperaba canónico %s, se obtuvo %s", expectedCanonical, parsed.String())
		}

		// Verificamos que la query construida con el ID normalizado no contenga 'urn:' y use ?::uuid
		stmt := repo.consultaBase(ctx, domain.FiltroProductosDTO{}).Where("pc.id = ?::uuid", parsed.String())
		stmt.Statement.Build(clause.Where{}.Name())

		sqlGenerated := stmt.Statement.SQL.String()
		if strings.Contains(sqlGenerated, "urn:") {
			t.Errorf("la consulta generada contiene 'urn:' en el SQL: %s", sqlGenerated)
		}
		if len(stmt.Statement.Vars) != 1 || stmt.Statement.Vars[0] != expectedCanonical {
			t.Errorf("la variable enviada a la consulta debe ser el UUID canónico '%s', se envió: %v", expectedCanonical, stmt.Statement.Vars)
		}
	})
}
