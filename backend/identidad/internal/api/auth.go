// Endpoints de cuenta propia: registro, login local y con Google, cierre de
// sesión y perfil.

package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
)

// ===== DTO HTTP =====

// PeticionRegistro DTO de entrada para la creación de cuenta
type PeticionRegistro struct {
	Correo         string `json:"correo" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	NombreCompleto string `json:"nombre_completo" binding:"required"`
}

// RespuestaRegistro DTO de salida sin exponer credenciales
type RespuestaRegistro struct {
	ID             string `json:"id"`
	Correo         string `json:"correo"`
	NombreCompleto string `json:"nombre_completo"`
	Rol            string `json:"rol"`
	EstaActivo     bool   `json:"esta_activo"`
	Mensaje        string `json:"mensaje"`
}

// PeticionLogin DTO de entrada para autenticación de usuario
type PeticionLogin struct {
	Correo   string `json:"correo" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RespuestaLogin DTO con la información básica del usuario autenticado
type RespuestaLogin struct {
	ID             string  `json:"id"`
	Correo         string  `json:"correo"`
	NombreCompleto string  `json:"nombre_completo"`
	Rol            string  `json:"rol"`
	EstaActivo     bool    `json:"esta_activo"`
	URLAvatar      *string `json:"url_avatar,omitempty"`
	Mensaje        string  `json:"mensaje"`
}

// ===== Sesión =====

// rolCanonico devuelve el rol normalizado para la respuesta y el JWT; si el
// valor almacenado no es reconocido lo devuelve tal cual.
func rolCanonico(rol string) string {
	if canonico, valido := domain.NormalizarRol(rol); valido {
		return canonico
	}
	return rol
}

const (
	cookieSesion = "jwt"
	// Duración de la sesión: 24h, igual que la expiración del JWT.
	duracionSesionSegundos = int(config.DuracionSesion / time.Second)
)

// ManejadorAuth gestiona las peticiones HTTP del módulo de autenticación
type ManejadorAuth struct {
	servicio     *services.ServicioAuth
	cookieSegura bool
	claveJWT     string
}

// NuevoManejadorAuth recibe el servicio y la clave JWT ya validada al iniciar.
func NuevoManejadorAuth(servicio *services.ServicioAuth, cookieSegura bool, clave string) *ManejadorAuth {
	return &ManejadorAuth{servicio: servicio, cookieSegura: cookieSegura, claveJWT: clave}
}

// emitirSesion firma el JWT y lo entrega en una cookie HttpOnly (no accesible
// desde JavaScript, protege contra XSS). Responde el error y retorna false si falla.
func (manejador *ManejadorAuth) emitirSesion(contexto *gin.Context, usuarioID, rol, proveedor string) bool {
	tokenTexto, errorObtenido := GenerarToken(usuarioID, rolCanonico(rol), proveedor, manejador.claveJWT)
	if errorObtenido != nil {
		ResponderError(contexto, http.StatusInternalServerError, "Error generando token de autorización.", errorObtenido)
		return false
	}
	contexto.SetSameSite(http.SameSiteLaxMode)
	contexto.SetCookie(cookieSesion, tokenTexto, duracionSesionSegundos, "/", "", manejador.cookieSegura, true)
	return true
}

// ===== Registro y login local =====

// RegistrarUsuario: registrar nuevo usuario con auto-login.
// Valida complejidad de contraseña, encripta con bcrypt, crea usuario y genera automáticamente cookie HttpOnly para inicio de sesión inmediato.
func (manejador *ManejadorAuth) RegistrarUsuario(contexto *gin.Context) {
	var peticion PeticionRegistro
	if errorObtenido := contexto.ShouldBindJSON(&peticion); errorObtenido != nil {
		ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos o incompletos.", errorObtenido)
		return
	}

	dto := services.RegistroDTO{
		Correo:         peticion.Correo,
		Password:       peticion.Password,
		NombreCompleto: peticion.NombreCompleto,
	}

	creado, errorOperacion := manejador.servicio.Registrar(contexto.Request.Context(), dto)
	if errorOperacion != nil {
		switch {
		case errors.Is(errorOperacion, services.ErrContrasenaInvalida), errors.Is(errorOperacion, services.ErrNombreInvalido):
			ResponderError(contexto, http.StatusBadRequest, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrCorreoDuplicado):
			ResponderError(contexto, http.StatusConflict, errorOperacion.Error(), errorOperacion)
		default:
			ResponderError(contexto, http.StatusInternalServerError, "Error interno procesando el registro.", errorOperacion)
		}
		return
	}

	// Auto-login tras el registro.
	if !manejador.emitirSesion(contexto, creado.ID.String(), creado.Rol, "local") {
		return
	}

	contexto.JSON(http.StatusCreated, RespuestaRegistro{
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
func (manejador *ManejadorAuth) LoginUsuario(contexto *gin.Context) {
	var peticion PeticionLogin
	if errorOperacion := contexto.ShouldBindJSON(&peticion); errorOperacion != nil {
		ResponderError(contexto, http.StatusBadRequest, "Datos de inicio de sesión inválidos o incompletos.", errorOperacion)
		return
	}

	usuario, errorObtenido := manejador.servicio.Login(contexto.Request.Context(), peticion.Correo, peticion.Password)
	if errorObtenido != nil {
		switch {
		case errors.Is(errorObtenido, services.ErrCredencialesInvalidas):
			ResponderError(contexto, http.StatusUnauthorized, "Credenciales incorrectas.", errorObtenido)
		case errors.Is(errorObtenido, services.ErrCuentaInactiva):
			ResponderError(contexto, http.StatusForbidden, errorObtenido.Error(), errorObtenido)
		default:
			ResponderError(contexto, http.StatusInternalServerError, "Error interno durante la autenticación.", errorObtenido)
		}
		return
	}

	if !manejador.emitirSesion(contexto, usuario.ID.String(), usuario.Rol, "local") {
		return
	}

	contexto.JSON(http.StatusOK, RespuestaLogin{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            rolCanonico(usuario.Rol),
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión exitoso.",
	})
}

// ===== Sesión y perfil =====

// CerrarSesion: cerrar sesión.
// Elimina la cookie HttpOnly de sesión. Es idempotente: responde 204 aunque no exista sesión.
func (manejador *ManejadorAuth) CerrarSesion(contexto *gin.Context) {
	// La cookie es HttpOnly: solo el servidor puede borrarla. MaxAge negativo la
	// expira con los mismos atributos con que se emitió.
	contexto.SetSameSite(http.SameSiteLaxMode)
	contexto.SetCookie(cookieSesion, "", -1, "/", "", manejador.cookieSegura, true)
	contexto.Status(http.StatusNoContent)
}

// PerfilUsuario: perfil del usuario autenticado.
// Endpoint protegido para verificar identidad y claims extraídos del token JWT.
func (manejador *ManejadorAuth) PerfilUsuario(contexto *gin.Context) {
	usuarioID, _ := contexto.Get("user_id")
	rol, _ := contexto.Get("rol")
	proveedor, _ := contexto.Get("provider")

	contexto.JSON(http.StatusOK, gin.H{
		"user_id":  usuarioID,
		"rol":      rol,
		"provider": proveedor,
		"mensaje":  "Acceso autorizado a ruta protegida con JWT.",
	})
}

// ActualizarPerfil: actualización parcial del perfil de usuario.
// Permite al usuario autenticado actualizar su nombre, correo, teléfono, dirección o contraseña. El ID se extrae directamente del token JWT (prevención IDOR).
func (manejador *ManejadorAuth) ActualizarPerfil(contexto *gin.Context) {
	// Prevención IDOR: Extraer identidad exclusivamente del contexto seguro inyectado por ExigirSesion
	valorUsuarioID, existe := contexto.Get("user_id")
	if !existe {
		ResponderError(contexto, http.StatusUnauthorized, "Acceso no autorizado.", errors.New("identificador de usuario no presente en sesión"))
		return
	}

	usuarioID, valido := valorUsuarioID.(string)
	if !valido || usuarioID == "" {
		ResponderError(contexto, http.StatusUnauthorized, "Identificador de usuario inválido.", errors.New("formato de user_id erróneo"))
		return
	}

	var peticion domain.ActualizarPerfilDTO
	if errorObtenido := contexto.ShouldBindJSON(&peticion); errorObtenido != nil {
		ResponderError(contexto, http.StatusBadRequest, "Datos de entrada inválidos para actualización de perfil.", errorObtenido)
		return
	}

	perfil, errorOperacion := manejador.servicio.ActualizarPerfil(contexto.Request.Context(), usuarioID, peticion)
	if errorOperacion != nil {
		switch {
		case errors.Is(errorOperacion, services.ErrContrasenaInvalida), errors.Is(errorOperacion, services.ErrCorreoInvalido), errors.Is(errorOperacion, services.ErrNombreInvalido):
			ResponderError(contexto, http.StatusBadRequest, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrCorreoDuplicado):
			ResponderError(contexto, http.StatusConflict, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrUsuarioNoEncontrado):
			ResponderError(contexto, http.StatusNotFound, errorOperacion.Error(), errorOperacion)
		case errors.Is(errorOperacion, services.ErrReautenticacionRequerida):
			ResponderError(contexto, http.StatusForbidden, errorOperacion.Error(), errorOperacion)
		default:
			ResponderError(contexto, http.StatusInternalServerError, "Error interno al actualizar perfil.", errorOperacion)
		}
		return
	}

	contexto.JSON(http.StatusOK, perfil)
}

// ===== Login con Google =====

// PeticionGoogle DTO de entrada para autenticación con Google
type PeticionGoogle struct {
	IDToken string `json:"id_token" binding:"required"`
}

// LoginGoogleUsuario: inicio de sesión con Google.
// Valida el token de Google y aplica lógica find-or-create para la cuenta de usuario.
func (manejador *ManejadorAuth) LoginGoogleUsuario(contexto *gin.Context) {
	var peticion PeticionGoogle
	if errorObtenido := contexto.ShouldBindJSON(&peticion); errorObtenido != nil {
		ResponderError(contexto, http.StatusBadRequest, "El id_token de Google es obligatorio.", errorObtenido)
		return
	}

	usuario, errorOperacion := manejador.servicio.LoginGoogle(contexto.Request.Context(), peticion.IDToken)
	if errorOperacion != nil {
		switch {
		case errors.Is(errorOperacion, services.ErrCredencialesInvalidas), errors.Is(errorOperacion, services.ErrCuentaInactiva):
			ResponderError(contexto, http.StatusUnauthorized, "Autenticación con Google fallida.", errorOperacion)
		case errors.Is(errorOperacion, services.ErrGoogleNoConfigurado):
			ResponderError(contexto, http.StatusServiceUnavailable, "Autenticación con Google temporalmente no disponible.", errorOperacion)
		default:
			ResponderError(contexto, http.StatusInternalServerError, "Error interno durante la autenticación con Google.", errorOperacion)
		}
		return
	}

	if !manejador.emitirSesion(contexto, usuario.ID.String(), usuario.Rol, "google") {
		return
	}

	contexto.JSON(http.StatusOK, RespuestaLogin{
		ID:             usuario.ID.String(),
		Correo:         usuario.Correo,
		NombreCompleto: usuario.NombreCompleto,
		Rol:            rolCanonico(usuario.Rol),
		EstaActivo:     usuario.EstaActivo,
		URLAvatar:      usuario.URLAvatar,
		Mensaje:        "Inicio de sesión con Google exitoso.",
	})
}
