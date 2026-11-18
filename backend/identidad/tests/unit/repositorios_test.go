// Comprueba la búsqueda pública del repositorio sin conectar a PostgreSQL.

package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestListarUsuariosEscapaComodinesLIKE verifica que contar y listar reciben
// parámetros literales: los caracteres del usuario no se vuelven comodines SQL.
func TestListarUsuariosEscapaComodinesLIKE(prueba *testing.T) {
	casos := []struct {
		entrada string
		patron  string
	}{
		{"normal", "%normal%"},
		{"100%", "%100!%%"},
		{"a_b", "%a!_b%"},
		{"x!y", "%x!!y%"},
		{"%_!", "%!%!_!!%"},
	}
	for _, caso := range casos {
		prueba.Run(caso.entrada, func(prueba *testing.T) {
			conexion, errorConexion := gorm.Open(postgres.New(postgres.Config{
				DSN: "host=127.0.0.1 user=app_identity dbname=prueba sslmode=disable",
			}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			if errorConexion != nil {
				prueba.Fatal(errorConexion)
			}
			consultas := 0
			errorRegistro := conexion.Callback().Query().After("gorm:query").Register("prueba:verificar_busqueda", func(consulta *gorm.DB) {
				consultas++
				if !strings.Contains(consulta.Statement.SQL.String(), "ESCAPE '!'") {
					prueba.Errorf("consulta sin escape explícito: %s", consulta.Statement.SQL.String())
				}
				parametros := consulta.Statement.Vars
				if len(parametros) != 2 || parametros[0] != caso.patron || parametros[1] != caso.patron {
					prueba.Errorf("patrones = %v; se esperaban dos parámetros %q", parametros, caso.patron)
				}
			})
			if errorRegistro != nil {
				prueba.Fatal(errorRegistro)
			}
			repositorio := repositories.NewAdminUsuarioRepository(conexion)
			if _, _, errorListado := repositorio.Listar(context.Background(), domain.FiltroUsuariosAdminDTO{Query: caso.entrada}); errorListado != nil {
				prueba.Fatal(errorListado)
			}
			if consultas != 2 {
				prueba.Fatalf("se esperaban COUNT y listado; se observaron %d consultas", consultas)
			}
		})
	}
}
