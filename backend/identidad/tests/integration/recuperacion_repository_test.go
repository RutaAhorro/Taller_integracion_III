// Tokens de recuperación contra PostgreSQL real y desechable; se omite sin
// IDENTIDAD_TEST_DB_URL e IDENTIDAD_TEST_ADMIN_URL.

package integration_test

import (
	"bytes"
	"context"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/server"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Integración con PostgreSQL real tras 05_tokens_recuperacion.sql y 06_propiedad_permisos.sql.
// Se omite salvo que existan IDENTIDAD_TEST_DB_URL (app_identity) e IDENTIDAD_TEST_ADMIN_URL (operador).
func TestRecuperacionRepositoryPostgres(prueba *testing.T) {
	dsn, adminDSN := os.Getenv("IDENTIDAD_TEST_DB_URL"), os.Getenv("IDENTIDAD_TEST_ADMIN_URL")
	if dsn == "" || adminDSN == "" {
		prueba.Skip("requiere IDENTIDAD_TEST_DB_URL e IDENTIDAD_TEST_ADMIN_URL")
	}
	contexto := context.Background()
	baseDatos, errorOperacion := repositories.Open(contexto, dsn, config.AjustesPredeterminados())
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	admin, errorOperacion := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
	if errorObtenido := server.VerificarRecuperacion(contexto, baseDatos); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	usuario := &domain.Usuario{Correo: "recuperacion-" + uuid.NewString()[:8] + "@example.com", NombreCompleto: "Prueba", EstaActivo: true}
	if errorObtenido := repositories.NewUsuarioRepository(baseDatos).Create(contexto, usuario); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { admin.Exec("DELETE FROM api.usuarios WHERE id = ?", usuario.ID) })

	repositorio := repositories.NewRecuperacionRepository(baseDatos)
	if cantidad, espera, errorObtenido := repositorio.SolicitudesRecientes(contexto, usuario.ID, time.Hour, time.Minute); errorObtenido != nil || cantidad != 0 || espera {
		prueba.Fatalf("sin tokens: %d %v %v", cantidad, espera, errorObtenido)
	}
	_, primero, _ := security.GenerarTokenRecuperacion()
	_, segundo, _ := security.GenerarTokenRecuperacion()
	for _, hash := range [][]byte{primero, segundo} {
		if errorObtenido := repositorio.EmitirToken(contexto, usuario.ID, hash, 30*time.Minute); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
	}
	if cantidad, espera, errorObtenido := repositorio.SolicitudesRecientes(contexto, usuario.ID, time.Hour, time.Minute); errorObtenido != nil || cantidad != 2 || !espera {
		prueba.Fatalf("dos tokens recientes: %d %v %v", cantidad, espera, errorObtenido)
	}

	var filas []struct {
		TokenHash []byte
		Vigente   bool
		Minutos   float64
	}
	if errorObtenido := admin.Raw(`SELECT token_hash, usado_el IS NULL AS vigente, extract(epoch FROM expira_el - creado_el) / 60 AS minutos
        FROM api.tokens_recuperacion WHERE usuario_id = ? ORDER BY creado_el, vigente`, usuario.ID).Scan(&filas).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(filas) != 2 || !bytes.Equal(filas[0].TokenHash, primero) || filas[0].Vigente || !bytes.Equal(filas[1].TokenHash, segundo) || !filas[1].Vigente || filas[1].Minutos < 29.9 || filas[1].Minutos > 30.1 {
		prueba.Fatalf("sólo el último token debe quedar vigente por 30 minutos: %+v", filas)
	}
	if errorObtenido := baseDatos.Exec("DELETE FROM api.tokens_recuperacion WHERE usuario_id = ?", usuario.ID).Error; errorObtenido == nil {
		prueba.Fatal("app_identity no debe poder borrar tokens")
	}
	if errorObtenido := repositorio.EmitirToken(contexto, usuario.ID, primero, 30*time.Minute); errorObtenido == nil {
		prueba.Fatal("un hash repetido debe rechazarse")
	}

	// Flujo completo con el servicio: el hash vigente en la base corresponde al token del correo.
	if errorObtenido := admin.Exec("UPDATE api.tokens_recuperacion SET creado_el = now() - interval '2 hours' WHERE usuario_id = ?", usuario.ID).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	// El transporte de desarrollo permite inspeccionar el correo sin enviar a terceros.
	var salidaCorreo bytes.Buffer
	correo, errorCorreo := mailer.Nuevo(config.Correo{Modo: config.CorreoLog}, slog.New(slog.NewJSONHandler(&salidaCorreo, nil)))
	if errorCorreo != nil {
		prueba.Fatal(errorCorreo)
	}
	servicio := services.NewRecuperacionService(repositories.NewUsuarioRepository(baseDatos), repositorio, correo, "https://frontend.example/restablecer-contrasena", nil, config.AjustesPredeterminados())
	if errorObtenido := servicio.Solicitar(usuario.Correo); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	contextoCierre, cancelarCierre := context.WithTimeout(contexto, 5*time.Second)
	defer cancelarCierre()
	if errorCierre := servicio.Cerrar(contextoCierre); errorCierre != nil {
		prueba.Fatal(errorCierre)
	}
	coincidencia := regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`).FindStringSubmatch(salidaCorreo.String())
	if len(coincidencia) != 2 || strings.Count(salidaCorreo.String(), `"msg":"correo_modo_log"`) != 1 {
		prueba.Fatal("se esperaba un único correo con su enlace de recuperación")
	}
	token := coincidencia[1]
	var vigentes int64
	admin.Raw("SELECT count(*) FROM api.tokens_recuperacion WHERE usuario_id = ? AND usado_el IS NULL AND token_hash = ?", usuario.ID, security.HashTokenRecuperacion(token)).Scan(&vigentes)
	if vigentes != 1 {
		prueba.Fatal("el token del correo no quedó vigente en la base")
	}
}
