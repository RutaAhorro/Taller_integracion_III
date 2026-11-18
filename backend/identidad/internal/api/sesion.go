// Sesión de Identidad: emisión y validación del JWT de la cookie `jwt`,
// revalidación de la cuenta en api.usuarios y autorización por rol.

package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ===== Contrato del token =====

// maximoBytesJWT limita el formato recibido, no la duración del contrato.
const maximoBytesJWT = 4096

// JWTClaims define los datos del payload del token JWT centralizado
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Rol      string `json:"rol"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

// ===== Emisión y validación =====

// GenerarToken emite un JWT firmado con HS256 conteniendo estrictamente user_id, rol, provider y expiración a 24 horas.
// La clave es la validada al arrancar: nunca se lee del entorno ni existe clave de respaldo.
func GenerarToken(usuarioID, rol, proveedor, clave string) (string, error) {
	if usuarioID == "" {
		return "", errors.New("el usuarioID no puede estar vacío")
	}

	ahora := time.Now()
	datosToken := JWTClaims{
		UserID:   usuarioID,
		Rol:      rol,
		Provider: proveedor,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(ahora.Add(config.DuracionSesion)),
			IssuedAt:  jwt.NewNumericDate(ahora),
			NotBefore: jwt.NewNumericDate(ahora),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, datosToken)
	if clave == "" {
		return "", errors.New("JWT_SECRET no configurado")
	}
	tokenTexto, errorObtenido := token.SignedString([]byte(clave))
	if errorObtenido != nil {
		return "", fmt.Errorf("error al firmar token JWT: %w", errorObtenido)
	}

	return tokenTexto, nil
}

// ValidarToken parsea y verifica la firma criptográfica y expiración del JWT
func ValidarToken(tokenTexto, clave string) (*jwt.Token, error) {
	if len(tokenTexto) > maximoBytesJWT {
		return nil, errors.New("token inválido")
	}
	if clave == "" {
		return nil, errors.New("JWT_SECRET no configurado")
	}
	secreto := []byte(clave)
	token, errorObtenido := jwt.ParseWithClaims(tokenTexto, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secreto, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	if errorObtenido != nil {
		return nil, fmt.Errorf("token inválido: %w", errorObtenido)
	}

	datosToken, valido := token.Claims.(*JWTClaims)
	if !valido || strings.TrimSpace(datosToken.UserID) == "" || strings.TrimSpace(datosToken.Rol) == "" || strings.TrimSpace(datosToken.Provider) == "" {
		return nil, errors.New("claims de identidad incompletos")
	}
	return token, nil
}

// ===== Sesión =====

// RequireAuth intercepta la petición HTTP validando el token en la cookie
// segura HttpOnly 'jwt' y revalida en api.usuarios que la cuenta siga activa,
// usando el rol actual de la base de datos: así una desactivación o degradación
// surte efecto sin esperar a que expire el JWT.
//
// El repositorio es obligatorio (fail-closed): una ruta protegida no puede
// omitir por accidente la revalidación de sesión. Sin él sólo se validaría la
// firma del JWT, que sigue vigente hasta 24 h después de desactivar la cuenta.
func RequireAuth(repositorio domain.UsuarioRepository, clave string) gin.HandlerFunc {
	if repositorio == nil {
		panic("RequireAuth: el repositorio de usuarios es obligatorio")
	}

	return func(contexto *gin.Context) {
		tokenTexto, errorOperacion := contexto.Cookie("jwt")
		if errorOperacion != nil || strings.TrimSpace(tokenTexto) == "" {
			ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Cookie de autenticación requerida.",
				errors.New("cookie 'jwt' ausente o no autorizada"),
			)
			return
		}

		token, errorOperacion := ValidarToken(strings.TrimSpace(tokenTexto), clave)
		if errorOperacion != nil || token == nil || !token.Valid {
			ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Token inválido o expirado.",
				errorOperacion,
			)
			return
		}

		datosToken, existe := token.Claims.(*JWTClaims)
		if !existe {
			ResponderError(
				contexto,
				http.StatusUnauthorized,
				"Claims del token inválidos.",
				errors.New("no fue posible procesar la información de identidad del token"),
			)
			return
		}

		usuarioID := strings.TrimSpace(datosToken.UserID)
		rol := datosToken.Rol

		if _, errorObtenido := uuid.Parse(usuarioID); errorObtenido != nil {
			ResponderError(contexto, http.StatusUnauthorized, "Token inválido o expirado.", errorObtenido)
			return
		}

		usuario, errorOperacion := repositorio.FindByID(contexto.Request.Context(), usuarioID)
		if errorOperacion != nil {
			ResponderError(contexto, http.StatusInternalServerError, "No se pudo validar la sesión.", errorOperacion)
			return
		}
		if usuario == nil || !usuario.EstaActivo {
			ResponderError(contexto, http.StatusUnauthorized, "La sesión ya no está activa.", nil)
			return
		}

		// El rol vigente es el de la base de datos, no el que quedó en el
		// JWT. Los alias históricos se normalizan a "admin".
		if canonico, valido := domain.NormalizarRol(usuario.Rol); valido {
			rol = canonico
		} else {
			rol = strings.ToLower(strings.TrimSpace(usuario.Rol))
		}
		usuarioID = usuario.ID.String()

		// Inyectar identidad vigente en el contexto de Gin para las rutas protegidas.
		contexto.Set("user_id", usuarioID)
		contexto.Set("rol", rol)
		contexto.Set("provider", datosToken.Provider)

		contexto.Next()
	}
}

// ===== Rol =====

// RequireRole valida que el usuario autenticado posea al menos uno de los roles permitidos en la petición.
// Si no se encuentra el rol o no coincide con los autorizados, interrumpe el flujo retornando HTTP 403 Forbidden.
func RequireRole(rolesPermitidos ...string) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		valorRol, existe := contexto.Get("rol")
		if !existe {
			ResponderError(
				contexto,
				http.StatusForbidden,
				"Acceso denegado: rol de usuario no identificado en la sesión.",
				errors.New("rol ausente en el contexto de autenticación"),
			)
			return
		}

		rolUsuario, valido := valorRol.(string)
		if !valido || strings.TrimSpace(rolUsuario) == "" {
			ResponderError(
				contexto,
				http.StatusForbidden,
				"Acceso denegado: rol de usuario inválido.",
				errors.New("formato de rol erróneo o vacío"),
			)
			return
		}

		rolUsuarioLimpio := strings.ToLower(strings.TrimSpace(rolUsuario))
		autorizado := false

		for _, rol := range rolesPermitidos {
			if rolUsuarioLimpio == strings.ToLower(strings.TrimSpace(rol)) {
				autorizado = true
				break
			}
		}

		if !autorizado {
			ResponderError(
				contexto,
				http.StatusForbidden,
				"Acceso denegado: permisos insuficientes para acceder a este recurso.",
				errors.New("el rol del usuario no tiene autorización para esta ruta"),
			)
			return
		}

		contexto.Next()
	}
}
