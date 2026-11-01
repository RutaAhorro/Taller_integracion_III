// Package identity es la entrada pública del módulo de identidad.
// Sus entidades, casos de uso y adaptadores permanecen privados.
package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Module arma Identidad completa (repositorios, servicios y handlers) y es lo
// único que main y server ven del servicio.
type Module struct {
	handler      *handlers.AuthHandler
	adminHandler *handlers.AdminUsuarioHandler
	repo         domain.UsuarioRepository
	redis        *redis.Client
	// recuperacion emite tokens y despacha su correo (SUP-184). El endpoint que la
	// expone (POST /auth/recuperar) es SUP-233.
	recuperacion services.RecuperacionService
}

// Recuperacion configura SUP-184. Sin Correo o sin URLRestablecer queda apagada:
// Solicitar sólo valida el formato y no emite tokens ni correos.
type Recuperacion struct {
	Correo         mailer.Mailer
	URLRestablecer string
	Log            *slog.Logger
}

// New crea el módulo con la recuperación de contraseña apagada.
func New(db *gorm.DB, rdb *redis.Client, secureCookie ...bool) *Module {
	return NewConRecuperacion(db, rdb, Recuperacion{}, secureCookie...)
}

// NewConRecuperacion crea el módulo con recuperación de contraseña; es el que usa main.
func NewConRecuperacion(db *gorm.DB, rdb *redis.Client, recuperacion Recuperacion, secureCookie ...bool) *Module {
	repo := repositories.NewUsuarioRepository(db)
	adminRepo := repositories.NewAdminUsuarioRepository(db)
	return &Module{
		handler:      handlers.NewAuthHandler(services.NewAuthService(repo), secureCookie...),
		adminHandler: handlers.NewAdminUsuarioHandler(services.NewAdminUsuarioService(adminRepo)),
		repo:         repo,
		redis:        rdb,
		recuperacion: services.NewRecuperacionService(repo, repositories.NewRecuperacionRepository(db), recuperacion.Correo, recuperacion.URLRestablecer, recuperacion.Log),
	}
}

// Cerrar despacha las solicitudes de recuperación pendientes antes de apagar el proceso.
func (m *Module) Cerrar(ctx context.Context) error {
	return m.recuperacion.Cerrar(ctx)
}

// RegisterRoutes monta /auth/* y /admin/usuarios/* bajo group.
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasAuth(group, m.handler, m.redis, m.repo)
	routes.RegistrarRutasAdminUsuarios(group, m.adminHandler, m.repo)
}

// RequireAuth entrega el interceptor sin exponer claves, JWT ni entidades internas.
func (m *Module) RequireAuth() gin.HandlerFunc {
	return middleware.RequireAuth(m.repo)
}

// RequireRole exige que el JWT validado por RequireAuth tenga alguno de los roles.
func (m *Module) RequireRole(roles ...string) gin.HandlerFunc {
	return middleware.RequireRole(roles...)
}

// VerifyStorage comprueba las tablas que utiliza Identidad, incluida la
// auditoría administrativa agregada en SUP-242.
func VerifyStorage(ctx context.Context, db *gorm.DB) error {
	var permitted bool
	err := db.WithContext(ctx).Raw(`SELECT
        has_schema_privilege(current_user, 'api', 'USAGE') AND
        has_table_privilege(current_user, 'api.usuarios', 'SELECT') AND
        has_table_privilege(current_user, 'api.usuarios', 'INSERT') AND
        has_table_privilege(current_user, 'api.usuarios', 'UPDATE') AND
        to_regclass('api.auditoria_admin_usuarios') IS NOT NULL AND
        has_table_privilege(current_user, 'api.auditoria_admin_usuarios', 'SELECT') AND
        has_table_privilege(current_user, 'api.auditoria_admin_usuarios', 'INSERT') AND
        has_sequence_privilege(current_user, 'api.auditoria_admin_usuarios_id_seq', 'USAGE') AND
        (SELECT COUNT(*) FROM information_schema.columns
            WHERE table_schema = 'api' AND table_name = 'usuarios'
              AND column_name IN ('plan', 'ultimo_acceso')) = 2`).Scan(&permitted).Error
	if err != nil {
		// Conexión, timeout o credenciales: no es un problema de migraciones.
		return fmt.Errorf("no se pudo verificar el almacenamiento de Identidad: %w", err)
	}
	if !permitted {
		return errors.New("aplicar 06_propiedad_permisos.sql, 08_normalizar_rol_admin_y_auditoria.sql y 09_plan_y_ultimo_acceso.sql antes de iniciar Identidad")
	}
	return nil
}

// VerificarRecuperacion comprueba api.tokens_recuperacion. A diferencia de VerifyStorage,
// su ausencia no impide iniciar: el login sigue disponible y sólo se apaga la recuperación.
func VerificarRecuperacion(ctx context.Context, db *gorm.DB) error {
	var permitted bool
	err := db.WithContext(ctx).Raw(`SELECT to_regclass('api.tokens_recuperacion') IS NOT NULL AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'SELECT') AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'INSERT') AND
        has_table_privilege(current_user, 'api.tokens_recuperacion', 'UPDATE')`).Scan(&permitted).Error
	if err != nil {
		return fmt.Errorf("no se pudo verificar la recuperación: %w", err)
	}
	if !permitted {
		return errors.New("aplicar 05_tokens_recuperacion.sql y 06_propiedad_permisos.sql para activar la recuperación")
	}
	return nil
}
