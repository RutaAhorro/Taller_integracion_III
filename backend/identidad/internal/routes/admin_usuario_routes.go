package routes

import (
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegistrarRutasAdminUsuarios expone la administración de cuentas en
// /api/v1/admin/usuarios. Toda petición exige sesión JWT y rol admin.
func RegistrarRutasAdminUsuarios(rg *gin.RouterGroup, handler *handlers.AdminUsuarioHandler) {
	admin := rg.Group("/admin/usuarios", middleware.RequireAuth(), middleware.RequireRole("admin"))
	{
		admin.GET("", handler.ListarUsuarios)
		admin.PUT("/:id", handler.ActualizarUsuario)
	}
}
