// Package identity es la entrada pública del módulo de identidad.
// Sus entidades, casos de uso y adaptadores permanecen privados.
package identity

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/mailer"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log/slog"
)

type Module struct {
	handler      *handlers.AuthHandler
	adminHandler *handlers.AdminUsuarioHandler
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

func New(db *gorm.DB, rdb *redis.Client, secureCookie ...bool) *Module {
	return NewConRecuperacion(db, rdb, Recuperacion{}, secureCookie...)
}

func NewConRecuperacion(db *gorm.DB, rdb *redis.Client, recuperacion Recuperacion, secureCookie ...bool) *Module {
	repo := repositories.NewUsuarioRepository(db)
	adminRepo := repositories.NewAdminUsuarioRepository(db)
	return &Module{
		handler:      handlers.NewAuthHandler(services.NewAuthService(repo), secureCookie...),
		adminHandler: handlers.NewAdminUsuarioHandler(services.NewAdminUsuarioService(adminRepo)),
		redis:        rdb,
		recuperacion: services.NewRecuperacionService(repo, repositories.NewRecuperacionRepository(db), recuperacion.Correo, recuperacion.URLRestablecer, recuperacion.Log),
	}
}

// Cerrar despacha las solicitudes de recuperación pendientes antes de apagar el proceso.
func (m *Module) Cerrar(ctx context.Context) error {
	return m.recuperacion.Cerrar(ctx)
}

func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasAuth(group, m.handler, m.redis)
	routes.RegistrarRutasAdminUsuarios(group, m.adminHandler)
}

// RequireAuth entrega el interceptor sin exponer claves, JWT ni entidades internas.
func (m *Module) RequireAuth() gin.HandlerFunc {
	return middleware.RequireAuth()
}

func (m *Module) RequireRole(roles ...string) gin.HandlerFunc {
	return middleware.RequireRole(roles...)
}

// VerifyStorage comprueba únicamente la tabla que utiliza Identidad.
func VerifyStorage(ctx context.Context, db *gorm.DB) error {
	var permitted bool
	err := db.WithContext(ctx).Raw(`SELECT
        has_schema_privilege(current_user, 'api', 'USAGE') AND
        has_table_privilege(current_user, 'api.usuarios', 'SELECT') AND
        has_table_privilege(current_user, 'api.usuarios', 'INSERT') AND
        has_table_privilege(current_user, 'api.usuarios', 'UPDATE')`).Scan(&permitted).Error
	if err != nil || !permitted {
		return errors.New("aplicar 06_propiedad_permisos.sql y permisos de Identidad antes de iniciar Identidad")
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
	if err != nil || !permitted {
		return errors.New("aplicar 05_tokens_recuperacion.sql y 06_propiedad_permisos.sql para activar la recuperación")
	}
	return nil
}
