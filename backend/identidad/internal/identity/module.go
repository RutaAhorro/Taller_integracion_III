// Package identity es la entrada pública del módulo de identidad.
// Sus entidades, casos de uso y adaptadores permanecen privados.
package identity

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Module struct {
	handler *handlers.AuthHandler
	redis   *redis.Client
}

func New(db *gorm.DB, rdb *redis.Client, secureCookie ...bool) *Module {
	repo := repositories.NewUsuarioRepository(db)
	return &Module{handler: handlers.NewAuthHandler(services.NewAuthService(repo), secureCookie...), redis: rdb}
}

func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	routes.RegistrarRutasAuth(group, m.handler, m.redis)
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
