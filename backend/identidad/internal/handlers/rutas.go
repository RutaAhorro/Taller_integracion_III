// Registro de rutas de Identidad: une cada endpoint con sus middlewares.

package handlers

import (
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ===== Autenticación y perfil =====

// RegistrarRutasAuth monta /auth/*. Registro, login y Google llevan contador
// por IP; las rutas de perfil revalidan la cuenta en api.usuarios además del JWT.
func RegistrarRutasAuth(grupo *gin.RouterGroup, manejador *AuthHandler, redisCliente *redis.Client, repositorio domain.UsuarioRepository, configuracion config.Config) {
	ajustes := configuracion.Ajustes
	sesion := middleware.RequireAuth(repositorio, configuracion.JWTSecret)
	auth := grupo.Group("/auth")
	auth.POST("/register", middleware.RateLimitRegistro(redisCliente, ajustes), manejador.RegistrarUsuario)
	auth.POST("/login", middleware.RateLimitLogin(redisCliente, ajustes), manejador.LoginUsuario)
	auth.POST("/google", middleware.RateLimitGoogle(redisCliente, ajustes), manejador.GoogleLoginUsuario)
	auth.POST("/logout", manejador.CerrarSesion)
	auth.GET("/me", sesion, manejador.PerfilUsuario)
	auth.PUT("/me", sesion, manejador.ActualizarPerfil)
}

// ===== Administración de cuentas =====

// RegistrarRutasAdminUsuarios monta /admin/usuarios. Toda petición exige
// sesión JWT vigente y rol admin.
func RegistrarRutasAdminUsuarios(grupo *gin.RouterGroup, manejador *AdminUsuarioHandler, repositorio domain.UsuarioRepository, claveJWT string) {
	admin := grupo.Group("/admin/usuarios", middleware.RequireAuth(repositorio, claveJWT), middleware.RequireRole(domain.RolAdmin))
	admin.GET("", manejador.ListarUsuarios)
	admin.GET("/metricas", manejador.Metricas)
	admin.PUT("/:id", manejador.ActualizarUsuario)
}
