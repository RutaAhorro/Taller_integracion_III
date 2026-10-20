package tests

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/database"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/identity"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Integración con PostgreSQL real tras 05_tokens_recuperacion.sql y 06_propiedad_permisos.sql.
// Se omite salvo que existan IDENTIDAD_TEST_DB_URL (app_identity) e IDENTIDAD_TEST_ADMIN_URL (operador).
func TestRecuperacionRepositoryPostgres(t *testing.T) {
	dsn, adminDSN := os.Getenv("IDENTIDAD_TEST_DB_URL"), os.Getenv("IDENTIDAD_TEST_ADMIN_URL")
	if dsn == "" || adminDSN == "" {
		t.Skip("requiere IDENTIDAD_TEST_DB_URL e IDENTIDAD_TEST_ADMIN_URL")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.VerificarRecuperacion(ctx, db); err != nil {
		t.Fatal(err)
	}
	usuario := &domain.Usuario{Correo: "recuperacion-" + uuid.NewString()[:8] + "@example.com", NombreCompleto: "Prueba", EstaActivo: true}
	if err := repositories.NewUsuarioRepository(db).Create(ctx, usuario); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DELETE FROM api.usuarios WHERE id = ?", usuario.ID) })

	repo := repositories.NewRecuperacionRepository(db)
	if n, espera, err := repo.SolicitudesRecientes(ctx, usuario.ID, time.Hour, time.Minute); err != nil || n != 0 || espera {
		t.Fatalf("sin tokens: %d %v %v", n, espera, err)
	}
	_, primero, _ := security.GenerarTokenRecuperacion()
	_, segundo, _ := security.GenerarTokenRecuperacion()
	for _, hash := range [][]byte{primero, segundo} {
		if err := repo.EmitirToken(ctx, usuario.ID, hash, 30*time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if n, espera, err := repo.SolicitudesRecientes(ctx, usuario.ID, time.Hour, time.Minute); err != nil || n != 2 || !espera {
		t.Fatalf("dos tokens recientes: %d %v %v", n, espera, err)
	}

	var filas []struct {
		TokenHash []byte
		Vigente   bool
		Minutos   float64
	}
	if err := admin.Raw(`SELECT token_hash, usado_el IS NULL AS vigente, extract(epoch FROM expira_el - creado_el) / 60 AS minutos
        FROM api.tokens_recuperacion WHERE usuario_id = ? ORDER BY creado_el, vigente`, usuario.ID).Scan(&filas).Error; err != nil {
		t.Fatal(err)
	}
	if len(filas) != 2 || !bytes.Equal(filas[0].TokenHash, primero) || filas[0].Vigente || !bytes.Equal(filas[1].TokenHash, segundo) || !filas[1].Vigente || filas[1].Minutos < 29.9 || filas[1].Minutos > 30.1 {
		t.Fatalf("sólo el último token debe quedar vigente por 30 minutos: %+v", filas)
	}
	if err := db.Exec("DELETE FROM api.tokens_recuperacion WHERE usuario_id = ?", usuario.ID).Error; err == nil {
		t.Fatal("app_identity no debe poder borrar tokens")
	}
	if err := repo.EmitirToken(ctx, usuario.ID, primero, 30*time.Minute); err == nil {
		t.Fatal("un hash repetido debe rechazarse")
	}

	// Flujo completo con el servicio: el hash vigente en la base corresponde al token del correo.
	if err := admin.Exec("UPDATE api.tokens_recuperacion SET creado_el = now() - interval '2 hours' WHERE usuario_id = ?", usuario.ID).Error; err != nil {
		t.Fatal(err)
	}
	correo := &correoFalso{}
	servicio := services.NewRecuperacionService(repositories.NewUsuarioRepository(db), repo, correo, urlRestablecer, nil)
	if err := servicio.Solicitar(usuario.Correo); err != nil {
		t.Fatal(err)
	}
	procesarPendientes(t, servicio)
	if len(correo.enviados) != 1 {
		t.Fatalf("esperaba un correo: %+v", correo.enviados)
	}
	token := tokenEnEnlace.FindStringSubmatch(correo.enviados[0].Texto)[1]
	var vigentes int64
	admin.Raw("SELECT count(*) FROM api.tokens_recuperacion WHERE usuario_id = ? AND usado_el IS NULL AND token_hash = ?", usuario.ID, security.HashTokenRecuperacion(token)).Scan(&vigentes)
	if vigentes != 1 {
		t.Fatal("el token del correo no quedó vigente en la base")
	}
}
