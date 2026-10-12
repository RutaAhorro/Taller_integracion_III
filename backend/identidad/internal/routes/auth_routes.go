package routes

import (
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegistrarRutasAuth conserva los endpoints públicos del módulo de identidad.
func RegistrarRutasAuth(rg *gin.RouterGroup, authHandler *handlers.AuthHandler, rdb *redis.Client) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", middleware.RateLimitRegistro(rdb), authHandler.RegistrarUsuario)
		auth.POST("/login", middleware.RateLimitLogin(rdb), authHandler.LoginUsuario)
		auth.POST("/google", middleware.RateLimitGoogle(rdb), authHandler.GoogleLoginUsuario)
		// Rutas protegidas con interceptor JWT
		auth.GET("/me", middleware.RequireAuth(), authHandler.PerfilUsuario)
		auth.PUT("/me", middleware.RequireAuth(), authHandler.ActualizarPerfil)
	}
}
