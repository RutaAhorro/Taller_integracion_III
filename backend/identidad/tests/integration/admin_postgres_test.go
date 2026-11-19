// Panel de administración contra PostgreSQL real y desechable: listado,
// cambios con auditoría, guarda del último admin y restricción de roles. Se
// omite sin IDENTIDAD_TEST_DB_URL (rol app_identity) e IDENTIDAD_TEST_ADMIN_URL
// (operador, sólo para preparar y limpiar datos).

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ===== Preparación =====

// basesDePrueba abre la conexión runtime (app_identity) y la de operador, o
// omite la prueba si no están configuradas.
func basesDePrueba(prueba *testing.T) (runtime *gorm.DB, operador *gorm.DB) {
	prueba.Helper()
	dsn, dsnOperador := os.Getenv("IDENTIDAD_TEST_DB_URL"), os.Getenv("IDENTIDAD_TEST_ADMIN_URL")
	if dsn == "" || dsnOperador == "" {
		prueba.Skip("requiere IDENTIDAD_TEST_DB_URL e IDENTIDAD_TEST_ADMIN_URL")
	}
	runtime, errorObtenido := repositories.Abrir(context.Background(), dsn, config.AjustesPredeterminados())
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	operador, errorObtenido = gorm.Open(postgres.Open(dsnOperador), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return runtime, operador
}

// crearCuenta registra una cuenta con el rol pedido. El prefijo único de la
// prueba permite filtrar el listado sin depender de otras filas de la base.
func crearCuenta(prueba *testing.T, runtime, operador *gorm.DB, prefijo, nombre, rol string) *domain.Usuario {
	prueba.Helper()
	usuario := &domain.Usuario{Correo: prefijo + "-" + nombre + "@example.com", NombreCompleto: prefijo + " " + nombre, Rol: rol, Plan: domain.PlanBasico, EstaActivo: true}
	if errorObtenido := repositories.NuevoRepositorioUsuarios(runtime).Crear(context.Background(), usuario); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() {
		operador.Exec("DELETE FROM api.auditoria_admin_usuarios WHERE objetivo_id = ? OR actor_id = ?", usuario.ID, usuario.ID)
		operador.Exec("DELETE FROM api.usuarios WHERE id = ?", usuario.ID)
	})
	return usuario
}

// ===== Prueba =====

// TestAdministracionPostgres ejercita el SQL del panel con el rol runtime real.
func TestAdministracionPostgres(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	if errorObtenido := repositories.VerificarAlmacenamiento(contexto, runtime); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prefijo := "admin-" + uuid.NewString()[:8]
	primerAdmin := crearCuenta(prueba, runtime, operador, prefijo, "uno", domain.RolAdmin)
	segundoAdmin := crearCuenta(prueba, runtime, operador, prefijo, "dos", domain.RolAdmin)
	objetivo := crearCuenta(prueba, runtime, operador, prefijo, "tres", domain.RolRegistrado)
	repositorio := repositories.NuevoRepositorioAdmin(runtime)

	prueba.Run("listado filtra y pagina", func(prueba *testing.T) {
		usuarios, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroUsuariosAdminDTO{Query: prefijo, Rol: domain.RolAdmin, Page: 2, Limit: 1})
		if errorObtenido != nil || total != 2 || len(usuarios) != 1 {
			prueba.Fatalf("se esperaban 2 admins y 1 fila en la página 2: %d %d %v", total, len(usuarios), errorObtenido)
		}
	})

	prueba.Run("cambio de rol queda auditado", func(prueba *testing.T) {
		if errorObtenido := repositorio.ActualizarConGuarda(contexto, objetivo.ID.String(), map[string]interface{}{"rol": domain.RolColaborador}, primerAdmin.ID.String()); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		var auditorias int64
		operador.Raw("SELECT count(*) FROM api.auditoria_admin_usuarios WHERE actor_id = ? AND objetivo_id = ?", primerAdmin.ID, objetivo.ID).Scan(&auditorias)
		if auditorias != 1 {
			prueba.Fatalf("se esperaba 1 auditoría, hay %d", auditorias)
		}
		// Repetir el mismo valor no debe inventar un cambio.
		if errorObtenido := repositorio.ActualizarConGuarda(contexto, objetivo.ID.String(), map[string]interface{}{"rol": domain.RolColaborador}, primerAdmin.ID.String()); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		operador.Raw("SELECT count(*) FROM api.auditoria_admin_usuarios WHERE objetivo_id = ?", objetivo.ID).Scan(&auditorias)
		if auditorias != 1 {
			prueba.Fatalf("un cambio idempotente dejó auditoría: %d", auditorias)
		}
	})

	prueba.Run("un admin no se degrada a sí mismo", func(prueba *testing.T) {
		errorObtenido := repositorio.ActualizarConGuarda(contexto, primerAdmin.ID.String(), map[string]interface{}{"esta_activo": false}, primerAdmin.ID.String())
		if !errors.Is(errorObtenido, domain.ErrAutoModificacionAdmin) {
			prueba.Fatalf("se esperaba ErrAutoModificacionAdmin, se obtuvo %v", errorObtenido)
		}
	})

	prueba.Run("no se desactiva al último admin activo", func(prueba *testing.T) {
		var otrosAdmins int64
		operador.Raw("SELECT count(*) FROM api.usuarios WHERE rol = 'admin' AND esta_activo AND id NOT IN (?, ?)", primerAdmin.ID, segundoAdmin.ID).Scan(&otrosAdmins)
		if otrosAdmins > 0 {
			prueba.Skip("la base tiene otros admins activos; este caso requiere una base desechable vacía")
		}
		if errorObtenido := repositorio.ActualizarConGuarda(contexto, segundoAdmin.ID.String(), map[string]interface{}{"rol": domain.RolRegistrado}, primerAdmin.ID.String()); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		errorObtenido := repositorio.ActualizarConGuarda(contexto, primerAdmin.ID.String(), map[string]interface{}{"esta_activo": false}, segundoAdmin.ID.String())
		if !errors.Is(errorObtenido, domain.ErrUltimoAdmin) {
			prueba.Fatalf("se esperaba ErrUltimoAdmin, se obtuvo %v", errorObtenido)
		}
	})

	prueba.Run("la base rechaza un rol desconocido", func(prueba *testing.T) {
		for _, rol := range []string{"administrador", " admin", "Admin"} {
			if errorObtenido := runtime.Exec("UPDATE api.usuarios SET rol = ? WHERE id = ?", rol, objetivo.ID).Error; errorObtenido == nil {
				prueba.Fatalf("usuarios_rol_check aceptó %q", rol)
			}
		}
	})

	prueba.Run("app_identity no borra auditoría", func(prueba *testing.T) {
		if errorObtenido := runtime.Exec("DELETE FROM api.auditoria_admin_usuarios WHERE objetivo_id = ?", objetivo.ID).Error; errorObtenido == nil {
			prueba.Fatal("app_identity no debe poder borrar la auditoría")
		}
	})
}
