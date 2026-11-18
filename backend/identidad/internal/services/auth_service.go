// Package services contiene los casos de uso de Identidad: registro, login,
// Google, perfil, administración de cuentas y recuperación de contraseña.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

// ===== Errores =====

var (
	ErrGoogleNoConfigurado = errors.New("login Google no configurado")
	// ErrPasswordInvalido se emite cuando la contraseña no cumple las reglas de complejidad
	ErrPasswordInvalido = errors.New("la contraseña debe tener al menos 8 caracteres, una letra mayúscula, un número y un símbolo especial")
	// ErrCorreoDuplicado se emite cuando ya existe un usuario con el mismo correo
	ErrCorreoDuplicado = domain.ErrCorreoDuplicado
	// ErrCredencialesInvalidas se emite genéricamente cuando el correo o contraseña son incorrectos
	ErrCredencialesInvalidas = errors.New("Credenciales incorrectas")
	// ErrCuentaInactiva se emite cuando la cuenta aún no ha sido activada mediante correo
	ErrCuentaInactiva = errors.New("La cuenta requiere verificación de correo")
	// ErrUsuarioNoEncontrado se emite cuando el usuario no existe en la base de datos
	ErrUsuarioNoEncontrado = errors.New("usuario no encontrado")
	// ErrCorreoInvalido se emite cuando el correo no tiene un formato válido
	ErrCorreoInvalido = errors.New("el formato del correo electrónico es inválido")
	ErrNombreInvalido = errors.New("el nombre completo no puede quedar vacío")
	// ErrReautenticacionRequerida se emite al cambiar correo o contraseña sin
	// confirmar la contraseña actual (o en cuentas sin contraseña local).
	ErrReautenticacionRequerida = errors.New("se requiere la contraseña actual para cambiar el correo o la contraseña")
)

// ===== Contrato del servicio =====

// RegistroDTO contiene los parámetros requeridos para el registro de un usuario
type RegistroDTO struct {
	Correo         string
	Password       string
	NombreCompleto string
}

// AuthService define los casos de uso para la autenticación e identidad de usuarios
type AuthService interface {
	Registrar(contexto context.Context, entrada RegistroDTO) (*domain.Usuario, error)
	LoginWithContext(contexto context.Context, correo, contrasena string) (*domain.Usuario, error)
	GoogleLogin(contexto context.Context, tokenGoogle string) (*domain.Usuario, error)
	ActualizarPerfil(contexto context.Context, usuarioID string, entrada domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)
}

// ValidadorGoogle verifica criptográficamente un id_token contra el client ID.
// En producción es idtoken.Validate; las pruebas inyectan claims sin red.
type ValidadorGoogle func(contexto context.Context, tokenGoogle, clienteGoogle string) (*idtoken.Payload, error)

// authService implementa AuthService.
type authService struct {
	usuarioRepo    domain.UsuarioRepository
	validateGoogle ValidadorGoogle
	googleClientID string
	googleTimeout  time.Duration
}

// NewAuthService recibe el validador de Google, su client ID y su plazo ya
// leídos al arrancar: el servicio nunca consulta el entorno.
func NewAuthService(repositorio domain.UsuarioRepository, validarGoogle ValidadorGoogle, clienteGoogle string, plazoGoogle time.Duration) AuthService {
	return &authService{usuarioRepo: repositorio, validateGoogle: validarGoogle, googleClientID: clienteGoogle, googleTimeout: plazoGoogle}
}

// ===== Validaciones y valores iniciales =====

// validarComplejidadPassword exige al menos 8 caracteres, una mayúscula, un número y un símbolo
func validarComplejidadPassword(contrasena string) error {
	if len(contrasena) > security.MaximoBytesPassword {
		return fmt.Errorf("%w: longitud máxima de 72 bytes", ErrPasswordInvalido)
	}
	if len(contrasena) < security.MinimoBytesPassword {
		return fmt.Errorf("%w: longitud mínima de 8 caracteres", ErrPasswordInvalido)
	}

	var tieneMayuscula, tieneNumero, tieneSimbolo bool
	for _, caracter := range contrasena {
		switch {
		case unicode.IsUpper(caracter):
			tieneMayuscula = true
		case unicode.IsDigit(caracter):
			tieneNumero = true
		case unicode.IsPunct(caracter) || unicode.IsSymbol(caracter):
			tieneSimbolo = true
		}
	}

	if !tieneMayuscula {
		return fmt.Errorf("%w: debe incluir al menos una letra mayúscula", ErrPasswordInvalido)
	}
	if !tieneNumero {
		return fmt.Errorf("%w: debe incluir al menos un número", ErrPasswordInvalido)
	}
	if !tieneSimbolo {
		return fmt.Errorf("%w: debe incluir al menos un símbolo especial", ErrPasswordInvalido)
	}

	return nil
}

// normalizarCorreo deja el correo en la forma en que se guarda y se compara.
func normalizarCorreo(correo string) string {
	return strings.ToLower(strings.TrimSpace(correo))
}

// nuevoUsuarioRegistrado aplica los valores iniciales comunes a toda cuenta nueva.
func nuevoUsuarioRegistrado(correo, nombre string) *domain.Usuario {
	ahora := time.Now()
	return &domain.Usuario{
		Correo:         correo,
		NombreCompleto: nombre,
		Rol:            domain.RolRegistrado,
		Plan:           domain.PlanBasico,
		CuotaTokensIA:  domain.CuotaTokensInicial,
		EstaActivo:     true, // Activado temporalmente según requerimiento de auto-login
		UltimoAcceso:   &ahora,
	}
}

// validarEmail verifica que el correo tenga un formato sintácticamente válido
func validarEmail(direccionCorreo string) bool {
	valorNumerico, errorObtenido := mail.ParseAddress(direccionCorreo)
	return errorObtenido == nil && valorNumerico.Address != "" && strings.Contains(direccionCorreo, "@") && strings.Contains(direccionCorreo, ".")
}

// registrarUltimoAcceso deja constancia del acceso sin bloquear el login si
// falla la escritura: la autenticación ya fue validada.
func (servicio *authService) registrarUltimoAcceso(contexto context.Context, usuario *domain.Usuario) {
	if usuario == nil {
		return
	}
	ahora := time.Now()
	if errorObtenido := servicio.usuarioRepo.Actualizar(contexto, usuario.ID.String(), map[string]interface{}{"ultimo_acceso": ahora}); errorObtenido != nil {
		slog.Warn("identidad_ultimo_acceso", "user_id", usuario.ID.String(), "error", errorObtenido)
		return
	}
	usuario.UltimoAcceso = &ahora
}

// ===== Registro y login local =====

// Registrar ejecuta el caso de uso completo de registro con validaciones de seguridad
func (servicio *authService) Registrar(contexto context.Context, entrada RegistroDTO) (*domain.Usuario, error) {
	// 1. Validar complejidad de contraseña antes de computar hash
	if errorOperacion := validarComplejidadPassword(entrada.Password); errorOperacion != nil {
		return nil, errorOperacion
	}

	// 2. Normalizar y sanitizar entradas
	correoNormalizado := normalizarCorreo(entrada.Correo)
	nombreSanitizado := SanitizarInputBusqueda(entrada.NombreCompleto)
	if nombreSanitizado == "" {
		return nil, ErrNombreInvalido
	}

	// 3. Comprobar unicidad del correo electrónico
	existente, errorObtenido := servicio.usuarioRepo.FindByEmail(contexto, correoNormalizado)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", errorObtenido)
	}
	if existente != nil {
		return nil, ErrCorreoDuplicado
	}

	// 4. Generar hash bcrypt de la contraseña
	hash, errorObtenido := security.HashPassword(entrada.Password)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al generar hash seguro de contraseña: %w", errorObtenido)
	}

	// 5. Instanciar modelo de dominio
	nuevoUsuario := nuevoUsuarioRegistrado(correoNormalizado, nombreSanitizado)
	nuevoUsuario.PasswordHash = &hash

	// 6. Persistir en repositorio
	if errorOperacion := servicio.usuarioRepo.Create(contexto, nuevoUsuario); errorOperacion != nil {
		return nil, fmt.Errorf("error al persistir el nuevo usuario: %w", errorOperacion)
	}

	// El handler arma la respuesta pública, igual que en el login.
	return nuevoUsuario, nil
}

// LoginWithContext ejecuta la autenticación verificando credenciales y estado de forma segura
func (servicio *authService) LoginWithContext(contexto context.Context, correo, contrasena string) (*domain.Usuario, error) {
	// 1. Buscar al usuario por correo usando el repositorio
	correoNormalizado := normalizarCorreo(correo)
	usuario, errorObtenido := servicio.usuarioRepo.FindByEmail(contexto, correoNormalizado)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", errorObtenido)
	}
	if usuario == nil {
		// Error genérico para no revelar si el correo existe o no
		return nil, ErrCredencialesInvalidas
	}

	// 2. Prevención de Panic por puntero nulo:
	// Si el usuario no tiene contraseña local (PasswordHash nulo o vacío), NO llamar a bcrypt y retornar de inmediato
	if usuario.PasswordHash == nil || *usuario.PasswordHash == "" {
		return nil, ErrCredencialesInvalidas
	}

	// 3. Validación de Activación: verificar si EstaActivo es true
	if !usuario.EstaActivo {
		return nil, ErrCuentaInactiva
	}

	// 4. Validación de Credenciales: comparar contraseña con security.CheckPasswordHash de forma segura
	if !security.CheckPasswordHash(contrasena, *usuario.PasswordHash) {
		return nil, ErrCredencialesInvalidas
	}

	servicio.registrarUltimoAcceso(contexto, usuario)
	return usuario, nil
}

// ===== Login con Google =====

// GoogleLogin valida el id_token de Google, exige correo verificado y busca o
// crea la cuenta; una creación concurrente se resuelve como login.
func (servicio *authService) GoogleLogin(contexto context.Context, tokenGoogle string) (*domain.Usuario, error) {
	clienteGoogle := servicio.googleClientID
	if clienteGoogle == "" {
		return nil, ErrGoogleNoConfigurado
	}

	contextoGoogle, cancelar := context.WithTimeout(contexto, servicio.googleTimeout)
	defer cancelar()
	datosGoogle, errorObtenido := servicio.validateGoogle(contextoGoogle, tokenGoogle, clienteGoogle)
	if errorObtenido != nil || datosGoogle == nil {
		return nil, ErrCredencialesInvalidas
	}
	if datosGoogle.Issuer != "accounts.google.com" && datosGoogle.Issuer != "https://accounts.google.com" {
		return nil, ErrCredencialesInvalidas
	}
	direccionCorreo, correoValido := datosGoogle.Claims["email"].(string)
	verificado, verificadoValido := datosGoogle.Claims["email_verified"].(bool)
	googleID := datosGoogle.Subject
	direccionCorreo = normalizarCorreo(direccionCorreo)
	if !correoValido || !verificadoValido || !verificado || !validarEmail(direccionCorreo) || googleID == "" {
		return nil, ErrCredencialesInvalidas
	}
	nombre, _ := datosGoogle.Claims["name"].(string)
	nombre = SanitizarInputBusqueda(nombre)
	if nombre == "" {
		nombre = strings.SplitN(direccionCorreo, "@", 2)[0]
	}
	var urlAvatar *string
	// El avatar es opcional. Omitir valores incompatibles con VARCHAR(500)
	// conserva el login sin persistir una URL cortada o UTF-8 inválido.
	if imagen, valido := datosGoogle.Claims["picture"].(string); valido && utf8.ValidString(imagen) && utf8.RuneCountInString(imagen) <= domain.MaximoCaracteresAvatar {
		urlAvatar = &imagen
	}

	usuario, errorObtenido := servicio.usuarioRepo.FindByEmail(contexto, direccionCorreo)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", errorObtenido)
	}

	if usuario == nil {
		nuevoUsuario := nuevoUsuarioRegistrado(direccionCorreo, nombre)
		nuevoUsuario.ID = uuid.New()
		nuevoUsuario.URLAvatar = urlAvatar
		nuevoUsuario.GoogleID = &googleID

		errorObtenido = servicio.usuarioRepo.Create(contexto, nuevoUsuario)
		if errorObtenido == nil {
			return nuevoUsuario, nil
		}
		if !errors.Is(errorObtenido, ErrCorreoDuplicado) {
			return nil, fmt.Errorf("error al registrar usuario con Google: %w", errorObtenido)
		}
		// Otro request pudo crear la misma cuenta entre find y create:
		// se continúa como login sobre la cuenta existente.
		usuario, errorObtenido = servicio.usuarioRepo.FindByEmail(contexto, direccionCorreo)
		if errorObtenido != nil {
			return nil, fmt.Errorf("error al consultar registro concurrente de Google: %w", errorObtenido)
		}
		if usuario == nil {
			return nil, ErrCredencialesInvalidas
		}
	}

	usuario, errorObtenido = servicio.validateGoogleAccount(contexto, usuario, googleID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	servicio.registrarUltimoAcceso(contexto, usuario)
	return usuario, nil
}

// ===== Perfil =====

// ActualizarPerfil procesa las actualizaciones parciales del perfil de usuario
func (servicio *authService) ActualizarPerfil(contexto context.Context, usuarioID string, entrada domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	if usuarioID == "" {
		return nil, ErrUsuarioNoEncontrado
	}

	// 1. Verificar existencia del usuario actual
	usuarioActual, errorObtenido := servicio.usuarioRepo.FindByID(contexto, usuarioID)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al verificar usuario: %w", errorObtenido)
	}
	if usuarioActual == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	datos := make(map[string]interface{})

	// Cambiar correo o contraseña exige reautenticación con la contraseña actual.
	// Las cuentas sin contraseña local (Google) no pueden hacerlo aquí hasta que
	// exista la confirmación por correo.
	cambiaCorreo := entrada.Correo != nil && normalizarCorreo(*entrada.Correo) != usuarioActual.Correo
	if cambiaCorreo || entrada.Password != nil {
		if usuarioActual.PasswordHash == nil || *usuarioActual.PasswordHash == "" ||
			entrada.PasswordActual == nil || !security.CheckPasswordHash(*entrada.PasswordActual, *usuarioActual.PasswordHash) {
			return nil, ErrReautenticacionRequerida
		}
	}

	// 2. Procesar NombreCompleto si viene presente
	if entrada.NombreCompleto != nil {
		nombreSanitizado := SanitizarInputBusqueda(*entrada.NombreCompleto)
		if nombreSanitizado == "" {
			return nil, ErrNombreInvalido
		}
		datos["nombre_completo"] = nombreSanitizado
	}

	// 3. Procesar Correo si viene presente
	if entrada.Correo != nil {
		correoNormalizado := normalizarCorreo(*entrada.Correo)
		if !validarEmail(correoNormalizado) {
			return nil, ErrCorreoInvalido
		}
		// Validar unicidad si el correo ha cambiado
		if correoNormalizado != usuarioActual.Correo {
			existente, errorOperacion := servicio.usuarioRepo.FindByEmail(contexto, correoNormalizado)
			if errorOperacion != nil {
				return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", errorOperacion)
			}
			if existente != nil && existente.ID.String() != usuarioID {
				return nil, ErrCorreoDuplicado
			}
			datos["correo"] = correoNormalizado
		}
	}

	// 4. Procesar Teléfono si viene presente
	if entrada.Telefono != nil {
		datos["telefono"] = strings.TrimSpace(*entrada.Telefono)
	}

	// 5. Procesar Dirección si viene presente
	if entrada.Direccion != nil {
		datos["direccion"] = strings.TrimSpace(*entrada.Direccion)
	}

	// 6. Procesar Contraseña si viene presente
	if entrada.Password != nil {
		if errorOperacion := validarComplejidadPassword(*entrada.Password); errorOperacion != nil {
			return nil, errorOperacion
		}
		hash, errorInterno := security.HashPassword(*entrada.Password)
		if errorInterno != nil {
			return nil, fmt.Errorf("error al generar hash de contraseña: %w", errorInterno)
		}
		datos["password_hash"] = hash
	}

	// Seguridad: datos solo recibe las claves de arriba; este caso de uso nunca
	// escribe esta_activo, rol ni cuota_tokens_ia.

	// 7. Si hay cambios dinámicos, persistir
	if len(datos) > 0 {
		datos["actualizado_el"] = time.Now()
		if errorOperacion := servicio.usuarioRepo.Actualizar(contexto, usuarioID, datos); errorOperacion != nil {
			return nil, fmt.Errorf("error al actualizar el perfil en base de datos: %w", errorOperacion)
		}
	}

	// 8. Consultar datos actualizados para retornar DTO consistente
	actualizado, errorObtenido := servicio.usuarioRepo.FindByID(contexto, usuarioID)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al consultar usuario actualizado: %w", errorObtenido)
	}
	if actualizado == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	rol := actualizado.Rol
	if canonico, valido := domain.NormalizarRol(actualizado.Rol); valido {
		rol = canonico
	}
	return &domain.PerfilUsuarioDTO{
		ID:             actualizado.ID.String(),
		Correo:         actualizado.Correo,
		NombreCompleto: actualizado.NombreCompleto,
		Telefono:       actualizado.Telefono,
		Direccion:      actualizado.Direccion,
		Rol:            rol,
		EstaActivo:     actualizado.EstaActivo,
		Mensaje:        "Perfil actualizado exitosamente.",
	}, nil
}

// ===== Vinculación de cuentas Google =====

// validateGoogleAccount acepta el login con Google sobre una cuenta existente.
//
// MEDIDA PROVISORIA (hasta implementar la confirmación de correo): hoy el
// registro local no verifica el correo, así que cualquiera puede registrar el
// correo de otra persona antes que ella (pre-account hijacking). Como Google sí
// verificó el correo (email_verified), en la primera vinculación se guarda el
// google_id y se invalida la contraseña local, que nunca demostró ser del dueño.
// Cuando exista `correo_verificado`, la contraseña debe conservarse si el correo
// local ya estaba verificado, y ambos métodos de acceso pueden convivir.
func (servicio *authService) validateGoogleAccount(contexto context.Context, usuario *domain.Usuario, sujeto string) (*domain.Usuario, error) {
	if !usuario.EstaActivo {
		return nil, ErrCuentaInactiva
	}
	if usuario.GoogleID != nil {
		if *usuario.GoogleID != sujeto {
			return nil, ErrCredencialesInvalidas
		}
		return usuario, nil
	}
	datos := map[string]interface{}{"google_id": sujeto, "password_hash": nil, "actualizado_el": time.Now()}
	if errorObtenido := servicio.usuarioRepo.Actualizar(contexto, usuario.ID.String(), datos); errorObtenido != nil {
		if errors.Is(errorObtenido, ErrCorreoDuplicado) {
			// google_id es único: ya está vinculado a otra cuenta.
			return nil, ErrCredencialesInvalidas
		}
		return nil, fmt.Errorf("error al vincular cuenta con Google: %w", errorObtenido)
	}
	usuario.GoogleID = &sujeto
	usuario.PasswordHash = nil
	return usuario, nil
}
