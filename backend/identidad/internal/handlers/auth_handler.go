package handlers

import (
	"errors"
	"net/http"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	middleware "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// RegistroRequest DTO de entrada para la creación de cuenta
type RegistroRequest struct {
	Correo         string `json:"correo" binding:"required,email" example:"usuario.test@uct.cl"`
	Password       string `json:"password" binding:"required" example:"PasswordSegura123!"`
	NombreCompleto string `json:"nombre_completo" binding:"required" example:"Vicente Matu"`
}

// RegistroResponse DTO de salida sin exponer credenciales
type RegistroResponse struct {
	ID             string `json:"id" example:"a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"`
	Correo         string `json:"correo" example:"usuario.test@uct.cl"`
	NombreCompleto string `json:"nombre_completo" example:"Vicente Matu"`
	Rol            string `json:"rol" example:"registrado"`
	EstaActivo     bool   `json:"esta_activo" example:"true"`
	Mensaje        string `json:"mensaje" example:"Usuario registrado con éxito."`
}

// LoginRequest DTO de entrada para autenticación de usuario
type LoginRequest struct {
	Correo   string `json:"correo" binding:"required,email" example:"usuario.test@uct.cl"`
	Password string `json:"password" binding:"required" example:"PasswordSegura123!"`
}

// LoginResponse DTO con la información básica del usuario autenticado
type LoginResponse struct {
	ID             string  `json:"id" example:"a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"`
	Correo         string  `json:"correo" example:"usuario.test@uct.cl"`
	NombreCompleto string  `json:"nombre_completo" example:"Vicente Matu"`
	Rol            string  `json:"rol" example:"registrado"`
	EstaActivo     bool    `json:"esta_activo" example:"true"`
	URLAvatar      *string `json:"url_avatar,omitempty" example:"https://ejemplo.com/avatar.jpg"`
	Mensaje        string  `json:"mensaje" example:"Inicio de sesión exitoso."`
}

// AuthHandler gestiona las peticiones HTTP del módulo de autenticación
type AuthHandler struct {
	authService  services.AuthService
	secureCookie bool
}

// NewAuthHandler construye el handler inyectando el servicio de autenticación
func NewAuthHandler(authService services.AuthService, secureCookie ...bool) *AuthHandler {
	secure := len(secureCookie) > 0 && secureCookie[0]
	return &AuthHandler{
		authService:  authService,
		secureCookie: secure,
	}
}

// RegistrarUsuario godoc
// @Summary      Registrar nuevo usuario con auto-login
// @Description  Valida complejidad de contraseña, encripta con bcrypt, crea usuario y genera automáticamente cookie HttpOnly para inicio de sesión inmediato
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload body RegistroRequest true "Datos de registro"
// @Success      201  {object}  RegistroResponse
// @Failure      400  {object}  middleware.RespuestaError
// @Failure      409  {object}  middleware.RespuestaError
// @Failure      500  {object}  middleware.RespuestaError
// @Router       /api/v1/auth/register [post]
func (h *AuthHandler) RegistrarUsuario(c *gin.Context) {
	var req RegistroRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ResponderError(c, http.StatusBadRequest, "Datos de entrada inválidos o incompletos.", err)
		return
	}

	dto := services.RegistroDTO{
		Correo:         req.Correo,
		Password:       req.Password,
		NombreCompleto: req.NombreCompleto,
	}

	creado, err := h.authService.Registrar(c.Request.Context(), dto)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordInvalido), errors.Is(err, services.ErrNombreInvalido):
			middleware.ResponderError(c, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrCorreoDuplicado):
			middleware.ResponderError(c, http.StatusConflict, err.Error(), err)
		default:
			middleware.ResponderError(c, http.StatusInternalServerError, "Error interno procesando el registro.", err)
		}
		return
	}

	// Auto-login: emisión del JWT centralizado e inyección de cookie HttpOnly
	tokenString, err := security.GenerarToken(creado.ID.String(), creado.Rol, "local")
	if err != nil {
		middleware.ResponderError(c, http.StatusInternalServerError, "Error generando token de autorización tras registro.", err)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jwt", tokenString, 86400, "/", "", h.secureCookie, true)

	c.JSON(http.StatusCreated, RegistroResponse{
		ID:             creado.ID.String(),
		Correo:         creado.Correo,
		NombreCompleto: creado.NombreCompleto,
		Rol:            creado.Rol,
		EstaActivo:     creado.EstaActivo,
		Mensaje:        creado.Mensaje,
	})
}

// LoginUsuario godoc
// @Summary      Inicio de sesión local de usuario
// @Description  Verifica credenciales de acceso, valida cuenta activa y emite token JWT firmado
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload body LoginRequest true "Credenciales de acceso"
// @Success      200  {object}  LoginResponse
// @Failure      400  {object}  middleware.RespuestaError
// @Failure      401  {object}  middleware.RespuestaError
// @Failure      403  {object}  middleware.RespuestaError
// @Failure      429  {object}  middleware.RespuestaError
// @Failure      500  {object}  middleware.RespuestaError
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) LoginUsuario(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ResponderError(c, http.StatusBadRequest, "Datos de inicio de sesión inválidos o incompletos.", err)
		return
	}

	usuario, err := h.authService.LoginWithContext(c.Request.Context(), req.Correo, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCredencialesInvalidas):
			middleware.ResponderError(c, http.StatusUnauthorized, "Credenciales incorrectas.", err)
		case errors.Is(err, services.ErrCuentaInactiva):
			middleware.ResponderError(c, http.StatusForbidden, err.Error(), err)
		default:
			middleware.ResponderError(c, http.StatusInternalServerError, "Error interno durante la autenticación.", err)
		}
		return
	}

	// Emisión del JWT centralizado (provider siempre es 'local' en login nativo)
	tokenString, err := security.GenerarToken(usuario.ID.String(), usuario.Rol, "local")
	if err != nil {
		middleware.ResponderError(c, http.StatusInternalServerError, "Error generando token de autorización.", err)
		return
	}

	// Inyectar cookie segura HttpOnly contra ataques XSS (duración: 24h = 86400s)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jwt", tokenString, 86400, "/", "", h.secureCookie, true)

	c.JSON(http.StatusOK, LoginResponse{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            usuario.Rol,
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión exitoso.",
	})
}

// PerfilUsuario godoc
// @Summary      Perfil del usuario autenticado
// @Description  Endpoint protegido para verificar identidad y claims extraídos del token JWT
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  middleware.RespuestaError
// @Router       /api/v1/auth/me [get]
func (h *AuthHandler) PerfilUsuario(c *gin.Context) {
	userID, _ := c.Get("user_id")
	rol, _ := c.Get("rol")
	provider, _ := c.Get("provider")

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"rol":      rol,
		"provider": provider,
		"mensaje":  "Acceso autorizado a ruta protegida con JWT.",
	})
}

// ActualizarPerfil godoc
// @Summary      Actualización parcial del perfil de usuario
// @Description  Permite al usuario autenticado actualizar su nombre, correo, teléfono, dirección o contraseña. El ID se extrae directamente del token JWT (prevención IDOR).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload body domain.ActualizarPerfilDTO true "Campos a actualizar"
// @Success      200  {object}  domain.PerfilUsuarioDTO
// @Failure      400  {object}  middleware.RespuestaError
// @Failure      401  {object}  middleware.RespuestaError
// @Failure      409  {object}  middleware.RespuestaError
// @Failure      500  {object}  middleware.RespuestaError
// @Router       /api/v1/auth/me [put]
func (h *AuthHandler) ActualizarPerfil(c *gin.Context) {
	// Prevención IDOR: Extraer identidad exclusivamente del contexto seguro inyectado por RequireAuth
	userIDVal, exists := c.Get("user_id")
	if !exists {
		middleware.ResponderError(c, http.StatusUnauthorized, "Acceso no autorizado.", errors.New("identificador de usuario no presente en sesión"))
		return
	}

	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		middleware.ResponderError(c, http.StatusUnauthorized, "Identificador de usuario inválido.", errors.New("formato de user_id erróneo"))
		return
	}

	var req domain.ActualizarPerfilDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ResponderError(c, http.StatusBadRequest, "Datos de entrada inválidos para actualización de perfil.", err)
		return
	}

	perfil, err := h.authService.ActualizarPerfil(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordInvalido), errors.Is(err, services.ErrCorreoInvalido), errors.Is(err, services.ErrNombreInvalido):
			middleware.ResponderError(c, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrCorreoDuplicado):
			middleware.ResponderError(c, http.StatusConflict, err.Error(), err)
		case errors.Is(err, services.ErrUsuarioNoEncontrado):
			middleware.ResponderError(c, http.StatusNotFound, err.Error(), err)
		case errors.Is(err, services.ErrReautenticacionRequerida):
			middleware.ResponderError(c, http.StatusForbidden, err.Error(), err)
		default:
			middleware.ResponderError(c, http.StatusInternalServerError, "Error interno al actualizar perfil.", err)
		}
		return
	}

	c.JSON(http.StatusOK, perfil)
}

// GoogleLoginRequest DTO de entrada para autenticación con Google
type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// GoogleLoginUsuario godoc
// @Summary      Inicio de sesión con Google
// @Description  Valida el token de Google y aplica lógica find-or-create para la cuenta de usuario
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload body GoogleLoginRequest true "Token JWT de Google"
// @Success      200  {object}  LoginResponse
// @Failure      400  {object}  middleware.RespuestaError
// @Failure      401  {object}  middleware.RespuestaError
// @Failure      500  {object}  middleware.RespuestaError
// @Failure      503  {object}  middleware.RespuestaError
// @Router       /api/v1/auth/google [post]
func (h *AuthHandler) GoogleLoginUsuario(c *gin.Context) {
	var req GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ResponderError(c, http.StatusBadRequest, "El id_token de Google es obligatorio.", err)
		return
	}

	usuario, err := h.authService.GoogleLogin(c.Request.Context(), req.IDToken)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCredencialesInvalidas), errors.Is(err, services.ErrCuentaInactiva):
			middleware.ResponderError(c, http.StatusUnauthorized, "Autenticación con Google fallida.", err)
		case errors.Is(err, services.ErrGoogleNoConfigurado):
			middleware.ResponderError(c, http.StatusServiceUnavailable, "Autenticación con Google temporalmente no disponible.", err)
		default:
			middleware.ResponderError(c, http.StatusInternalServerError, "Error interno durante la autenticación con Google.", err)
		}
		return
	}

	// Emisión del JWT centralizado
	tokenString, err := security.GenerarToken(usuario.ID.String(), usuario.Rol, "google")
	if err != nil {
		middleware.ResponderError(c, http.StatusInternalServerError, "Error generando token de autorización local.", err)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("jwt", tokenString, 86400, "/", "", h.secureCookie, true)

	c.JSON(http.StatusOK, LoginResponse{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            usuario.Rol,
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión con Google exitoso.",
	})
}
