package handlers

import (
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"net/http"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	httperrors "github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/httperrors"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// RegistroRequest DTO de entrada para la creación de cuenta
type RegistroRequest struct {
	Correo         string `json:"correo" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	NombreCompleto string `json:"nombre_completo" binding:"required"`
}

// RegistroResponse DTO de salida sin exponer credenciales
type RegistroResponse struct {
	ID             string `json:"id"`
	Correo         string `json:"correo"`
	NombreCompleto string `json:"nombre_completo"`
	Rol            string `json:"rol"`
	EstaActivo     bool   `json:"esta_activo"`
	Mensaje        string `json:"mensaje"`
}

// LoginRequest DTO de entrada para autenticación de usuario
type LoginRequest struct {
	Correo   string `json:"correo" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse DTO con la información básica del usuario autenticado
type LoginResponse struct {
	ID             string  `json:"id"`
	Correo         string  `json:"correo"`
	NombreCompleto string  `json:"nombre_completo"`
	Rol            string  `json:"rol"`
	EstaActivo     bool    `json:"esta_activo"`
	URLAvatar      *string `json:"url_avatar,omitempty"`
	Mensaje        string  `json:"mensaje"`
}

func rolCanonico(rol string) string {
	if canonico, ok := domain.NormalizarRol(rol); ok {
		return canonico
	}
	return rol
}

const (
	cookieSesion = "jwt"
	// Duración de la sesión: 24h, igual que la expiración del JWT.
	duracionSesionSegundos = int(config.DuracionSesion / time.Second)
)

// AuthHandler gestiona las peticiones HTTP del módulo de autenticación
type AuthHandler struct {
	authService  services.AuthService
	secureCookie bool
	claveJWT     string
}

// NewAuthHandler recibe el servicio y la clave JWT ya validada al iniciar.
func NewAuthHandler(servicio services.AuthService, cookieSegura bool, clave string) *AuthHandler {
	return &AuthHandler{authService: servicio, secureCookie: cookieSegura, claveJWT: clave}
}

// emitirSesion firma el JWT y lo entrega en una cookie HttpOnly (no accesible
// desde JavaScript, protege contra XSS). Responde el error y retorna false si falla.
func (manejador *AuthHandler) emitirSesion(contexto *gin.Context, userID, rol, provider string) bool {
	tokenString, err := security.GenerarToken(userID, rolCanonico(rol), provider, manejador.claveJWT)
	if err != nil {
		httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error generando token de autorización.", err)
		return false
	}
	contexto.SetSameSite(http.SameSiteLaxMode)
	contexto.SetCookie(cookieSesion, tokenString, duracionSesionSegundos, "/", "", manejador.secureCookie, true)
	return true
}

// RegistrarUsuario: registrar nuevo usuario con auto-login.
// Valida complejidad de contraseña, encripta con bcrypt, crea usuario y genera automáticamente cookie HttpOnly para inicio de sesión inmediato.
func (manejador *AuthHandler) RegistrarUsuario(contexto *gin.Context) {
	var req RegistroRequest
	if err := contexto.ShouldBindJSON(&req); err != nil {
		httperrors.ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos o incompletos.", err)
		return
	}

	dto := services.RegistroDTO{
		Correo:         req.Correo,
		Password:       req.Password,
		NombreCompleto: req.NombreCompleto,
	}

	creado, err := manejador.authService.Registrar(contexto.Request.Context(), dto)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordInvalido), errors.Is(err, services.ErrNombreInvalido):
			httperrors.ResponderError(contexto, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrCorreoDuplicado):
			httperrors.ResponderError(contexto, http.StatusConflict, err.Error(), err)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno procesando el registro.", err)
		}
		return
	}

	// Auto-login tras el registro.
	if !manejador.emitirSesion(contexto, creado.ID.String(), creado.Rol, "local") {
		return
	}

	contexto.JSON(http.StatusCreated, RegistroResponse{
		ID:             creado.ID.String(),
		Correo:         creado.Correo,
		NombreCompleto: creado.NombreCompleto,
		Rol:            rolCanonico(creado.Rol),
		EstaActivo:     creado.EstaActivo,
		Mensaje:        "Usuario registrado con éxito.",
	})
}

// LoginUsuario: inicio de sesión local de usuario.
// Verifica credenciales de acceso, valida cuenta activa y emite token JWT firmado.
func (manejador *AuthHandler) LoginUsuario(contexto *gin.Context) {
	var req LoginRequest
	if err := contexto.ShouldBindJSON(&req); err != nil {
		httperrors.ResponderError(contexto, http.StatusBadRequest, "Datos de inicio de sesión inválidos o incompletos.", err)
		return
	}

	usuario, err := manejador.authService.LoginWithContext(contexto.Request.Context(), req.Correo, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCredencialesInvalidas):
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "Credenciales incorrectas.", err)
		case errors.Is(err, services.ErrCuentaInactiva):
			httperrors.ResponderError(contexto, http.StatusForbidden, err.Error(), err)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno durante la autenticación.", err)
		}
		return
	}

	if !manejador.emitirSesion(contexto, usuario.ID.String(), usuario.Rol, "local") {
		return
	}

	contexto.JSON(http.StatusOK, LoginResponse{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            rolCanonico(usuario.Rol),
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión exitoso.",
	})
}

// CerrarSesion: cerrar sesión.
// Elimina la cookie HttpOnly de sesión. Es idempotente: responde 204 aunque no exista sesión.
func (manejador *AuthHandler) CerrarSesion(contexto *gin.Context) {
	// La cookie es HttpOnly: solo el servidor puede borrarla. MaxAge negativo la
	// expira con los mismos atributos con que se emitió.
	contexto.SetSameSite(http.SameSiteLaxMode)
	contexto.SetCookie(cookieSesion, "", -1, "/", "", manejador.secureCookie, true)
	contexto.Status(http.StatusNoContent)
}

// PerfilUsuario: perfil del usuario autenticado.
// Endpoint protegido para verificar identidad y claims extraídos del token JWT.
func (manejador *AuthHandler) PerfilUsuario(contexto *gin.Context) {
	userID, _ := contexto.Get("user_id")
	rol, _ := contexto.Get("rol")
	provider, _ := contexto.Get("provider")

	contexto.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"rol":      rol,
		"provider": provider,
		"mensaje":  "Acceso autorizado a ruta protegida con JWT.",
	})
}

// ActualizarPerfil: actualización parcial del perfil de usuario.
// Permite al usuario autenticado actualizar su nombre, correo, teléfono, dirección o contraseña. El ID se extrae directamente del token JWT (prevención IDOR).
func (manejador *AuthHandler) ActualizarPerfil(contexto *gin.Context) {
	// Prevención IDOR: Extraer identidad exclusivamente del contexto seguro inyectado por RequireAuth
	userIDVal, exists := contexto.Get("user_id")
	if !exists {
		httperrors.ResponderError(contexto, http.StatusUnauthorized, "Acceso no autorizado.", errors.New("identificador de usuario no presente en sesión"))
		return
	}

	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		httperrors.ResponderError(contexto, http.StatusUnauthorized, "Identificador de usuario inválido.", errors.New("formato de user_id erróneo"))
		return
	}

	var req domain.ActualizarPerfilDTO
	if err := contexto.ShouldBindJSON(&req); err != nil {
		httperrors.ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos para actualización de perfil.", err)
		return
	}

	perfil, err := manejador.authService.ActualizarPerfil(contexto.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordInvalido), errors.Is(err, services.ErrCorreoInvalido), errors.Is(err, services.ErrNombreInvalido):
			httperrors.ResponderError(contexto, http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, services.ErrCorreoDuplicado):
			httperrors.ResponderError(contexto, http.StatusConflict, err.Error(), err)
		case errors.Is(err, services.ErrUsuarioNoEncontrado):
			httperrors.ResponderError(contexto, http.StatusNotFound, err.Error(), err)
		case errors.Is(err, services.ErrReautenticacionRequerida):
			httperrors.ResponderError(contexto, http.StatusForbidden, err.Error(), err)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno al actualizar perfil.", err)
		}
		return
	}

	contexto.JSON(http.StatusOK, perfil)
}

// GoogleLoginRequest DTO de entrada para autenticación con Google
type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// GoogleLoginUsuario: inicio de sesión con Google.
// Valida el token de Google y aplica lógica find-or-create para la cuenta de usuario.
func (manejador *AuthHandler) GoogleLoginUsuario(contexto *gin.Context) {
	var req GoogleLoginRequest
	if err := contexto.ShouldBindJSON(&req); err != nil {
		httperrors.ResponderError(contexto, http.StatusBadRequest, "El id_token de Google es obligatorio.", err)
		return
	}

	usuario, err := manejador.authService.GoogleLogin(contexto.Request.Context(), req.IDToken)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCredencialesInvalidas), errors.Is(err, services.ErrCuentaInactiva):
			httperrors.ResponderError(contexto, http.StatusUnauthorized, "Autenticación con Google fallida.", err)
		case errors.Is(err, services.ErrGoogleNoConfigurado):
			httperrors.ResponderError(contexto, http.StatusServiceUnavailable, "Autenticación con Google temporalmente no disponible.", err)
		default:
			httperrors.ResponderError(contexto, http.StatusInternalServerError, "Error interno durante la autenticación con Google.", err)
		}
		return
	}

	if !manejador.emitirSesion(contexto, usuario.ID.String(), usuario.Rol, "google") {
		return
	}

	contexto.JSON(http.StatusOK, LoginResponse{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            rolCanonico(usuario.Rol),
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión con Google exitoso.",
	})
}
