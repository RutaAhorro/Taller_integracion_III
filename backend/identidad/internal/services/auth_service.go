package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/utils"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

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

// RegistroDTO contiene los parámetros requeridos para el registro de un usuario
type RegistroDTO struct {
	Correo         string
	Password       string
	NombreCompleto string
}

// UsuarioCreadoDTO representa el resultado de un registro exitoso sin datos sensibles
type UsuarioCreadoDTO struct {
	ID                uuid.UUID
	Correo            string
	NombreCompleto    string
	Rol               string
	EstaActivo        bool
	TokenVerificacion uuid.UUID
	Mensaje           string
}

// AuthService define los casos de uso para la autenticación e identidad de usuarios
type AuthService interface {
	Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error)
	Login(correo, password string) (*domain.Usuario, error)
	LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error)
	GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error)
	ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)
}

type authService struct {
	usuarioRepo    domain.UsuarioRepository
	validateGoogle func(context.Context, string, string) (*idtoken.Payload, error)
}

// NewAuthService inicializa el servicio inyectando el repositorio de usuarios
func NewAuthService(usuarioRepo domain.UsuarioRepository) AuthService {
	return &authService{
		usuarioRepo:    usuarioRepo,
		validateGoogle: idtoken.Validate,
	}
}

// validarComplejidadPassword exige al menos 8 caracteres, una mayúscula, un número y un símbolo
func validarComplejidadPassword(password string) error {
	if len(password) > 72 {
		return fmt.Errorf("%w: longitud máxima de 72 bytes", ErrPasswordInvalido)
	}
	if len(password) < 8 {
		return fmt.Errorf("%w: longitud mínima de 8 caracteres", ErrPasswordInvalido)
	}

	var tieneMayuscula, tieneNumero, tieneSimbolo bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			tieneMayuscula = true
		case unicode.IsDigit(r):
			tieneNumero = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
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

// validarEmail verifica que el correo tenga un formato sintácticamente válido
func validarEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address != "" && strings.Contains(email, "@") && strings.Contains(email, ".")
}

// Registrar ejecuta el caso de uso completo de registro con validaciones de seguridad
func (s *authService) Registrar(ctx context.Context, input RegistroDTO) (*UsuarioCreadoDTO, error) {
	// 1. Validar complejidad de contraseña antes de computar hash
	if err := validarComplejidadPassword(input.Password); err != nil {
		return nil, err
	}

	// 2. Normalizar y sanitizar entradas
	correoNormalizado := strings.ToLower(strings.TrimSpace(input.Correo))
	nombreSanitizado := utils.SanitizarInputBusqueda(input.NombreCompleto)
	if nombreSanitizado == "" {
		return nil, ErrNombreInvalido
	}

	// 3. Comprobar unicidad del correo electrónico
	existente, err := s.usuarioRepo.FindByEmail(ctx, correoNormalizado)
	if err != nil {
		return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", err)
	}
	if existente != nil {
		return nil, ErrCorreoDuplicado
	}

	// 4. Generar hash bcrypt de la contraseña
	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("error al generar hash seguro de contraseña: %w", err)
	}

	// 5. Generar token de verificación UUID
	tokenVerificacion := uuid.New()

	// 6. Instanciar modelo de dominio
	nuevoUsuario := &domain.Usuario{
		Correo:            correoNormalizado,
		PasswordHash:      &hash,
		NombreCompleto:    nombreSanitizado,
		Rol:               "registrado",
		CuotaTokensIA:     1000,
		EstaActivo:        true, // Activado temporalmente según requerimiento de auto-login
		TokenVerificacion: &tokenVerificacion,
	}

	// 7. Persistir en repositorio
	if err := s.usuarioRepo.Create(ctx, nuevoUsuario); err != nil {
		return nil, fmt.Errorf("error al persistir el nuevo usuario: %w", err)
	}

	// 8. Construir DTO de respuesta
	return &UsuarioCreadoDTO{
		ID:                nuevoUsuario.ID,
		Correo:            nuevoUsuario.Correo,
		NombreCompleto:    nuevoUsuario.NombreCompleto,
		Rol:               nuevoUsuario.Rol,
		EstaActivo:        nuevoUsuario.EstaActivo,
		TokenVerificacion: tokenVerificacion,
		Mensaje:           "Usuario registrado con éxito.",
	}, nil
}

// Login ejecuta la autenticación de un usuario con contexto por defecto
func (s *authService) Login(correo, password string) (*domain.Usuario, error) {
	return s.LoginWithContext(context.Background(), correo, password)
}

// LoginWithContext ejecuta la autenticación verificando credenciales y estado de forma segura
func (s *authService) LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error) {
	// 1. Buscar al usuario por correo usando el repositorio
	correoNormalizado := strings.ToLower(strings.TrimSpace(correo))
	usuario, err := s.usuarioRepo.FindByEmail(ctx, correoNormalizado)
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", err)
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
	if !security.CheckPasswordHash(password, *usuario.PasswordHash) {
		return nil, ErrCredencialesInvalidas
	}

	return usuario, nil
}

func (s *authService) GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		return nil, ErrGoogleNoConfigurado
	}

	googleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	payload, err := s.validateGoogle(googleCtx, tokenGoogle, clientID)
	if err != nil || payload == nil {
		return nil, ErrCredencialesInvalidas
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return nil, ErrCredencialesInvalidas
	}
	email, emailOK := payload.Claims["email"].(string)
	verified, verifiedOK := payload.Claims["email_verified"].(bool)
	googleID := payload.Subject
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailOK || !verifiedOK || !verified || !validarEmail(email) || googleID == "" {
		return nil, ErrCredencialesInvalidas
	}
	name, _ := payload.Claims["name"].(string)
	name = utils.SanitizarInputBusqueda(name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	var picture *string
	// El avatar es opcional. Omitir valores incompatibles con VARCHAR(500)
	// conserva el login sin persistir una URL cortada o UTF-8 inválido.
	if pic, ok := payload.Claims["picture"].(string); ok && utf8.ValidString(pic) && utf8.RuneCountInString(pic) <= 500 {
		picture = &pic
	}

	usuario, err := s.usuarioRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", err)
	}

	if usuario == nil {
		nuevoUsuario := &domain.Usuario{
			ID:             uuid.New(),
			Correo:         email,
			NombreCompleto: name,
			URLAvatar:      picture,
			GoogleID:       &googleID,
			Rol:            "registrado",
			EstaActivo:     true,
			CuotaTokensIA:  1000,
		}

		err = s.usuarioRepo.Create(ctx, nuevoUsuario)
		if errors.Is(err, ErrCorreoDuplicado) {
			// Otro request pudo crear la misma cuenta entre find y create.
			usuario, err = s.usuarioRepo.FindByEmail(ctx, email)
			if err != nil {
				return nil, fmt.Errorf("error al consultar registro concurrente de Google: %w", err)
			}
			if usuario == nil {
				return nil, ErrCredencialesInvalidas
			}
			return s.validateGoogleAccount(ctx, usuario, googleID)
		}
		if err != nil {
			return nil, fmt.Errorf("error al registrar usuario con Google: %w", err)
		}
		return nuevoUsuario, nil
	}

	return s.validateGoogleAccount(ctx, usuario, googleID)
}

// ActualizarPerfil procesa las actualizaciones parciales del perfil de usuario
func (s *authService) ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrUsuarioNoEncontrado
	}

	// 1. Verificar existencia del usuario actual
	usuarioActual, err := s.usuarioRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al verificar usuario: %w", err)
	}
	if usuarioActual == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	datos := make(map[string]interface{})

	// Cambiar correo o contraseña exige reautenticación con la contraseña actual.
	// Las cuentas sin contraseña local (Google) no pueden hacerlo aquí hasta que
	// exista la confirmación por correo.
	cambiaCorreo := input.Correo != nil && strings.ToLower(strings.TrimSpace(*input.Correo)) != usuarioActual.Correo
	if cambiaCorreo || input.Password != nil {
		if usuarioActual.PasswordHash == nil || *usuarioActual.PasswordHash == "" ||
			input.PasswordActual == nil || !security.CheckPasswordHash(*input.PasswordActual, *usuarioActual.PasswordHash) {
			return nil, ErrReautenticacionRequerida
		}
	}

	// 2. Procesar NombreCompleto si viene presente
	if input.NombreCompleto != nil {
		nombreSanitizado := utils.SanitizarInputBusqueda(*input.NombreCompleto)
		if nombreSanitizado == "" {
			return nil, ErrNombreInvalido
		}
		datos["nombre_completo"] = nombreSanitizado
	}

	// 3. Procesar Correo si viene presente
	if input.Correo != nil {
		correoNormalizado := strings.ToLower(strings.TrimSpace(*input.Correo))
		if !validarEmail(correoNormalizado) {
			return nil, ErrCorreoInvalido
		}
		// Validar unicidad si el correo ha cambiado
		if correoNormalizado != usuarioActual.Correo {
			existente, err := s.usuarioRepo.FindByEmail(ctx, correoNormalizado)
			if err != nil {
				return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", err)
			}
			if existente != nil && existente.ID.String() != userID {
				return nil, ErrCorreoDuplicado
			}
			datos["correo"] = correoNormalizado
		}
	}

	// 4. Procesar Teléfono si viene presente
	if input.Telefono != nil {
		datos["telefono"] = strings.TrimSpace(*input.Telefono)
	}

	// 5. Procesar Dirección si viene presente
	if input.Direccion != nil {
		datos["direccion"] = strings.TrimSpace(*input.Direccion)
	}

	// 6. Procesar Contraseña si viene presente
	if input.Password != nil {
		if err := validarComplejidadPassword(*input.Password); err != nil {
			return nil, err
		}
		hash, err := security.HashPassword(*input.Password)
		if err != nil {
			return nil, fmt.Errorf("error al generar hash de contraseña: %w", err)
		}
		datos["password_hash"] = hash
	}

	// 7. Seguridad: Prohibir explícitamente alterar esta_activo, rol o cuota desde este caso de uso
	delete(datos, "esta_activo")
	delete(datos, "rol")
	delete(datos, "cuota_tokens_ia")

	// 8. Si hay cambios dinámicos, persistir
	if len(datos) > 0 {
		datos["actualizado_el"] = time.Now()
		if err := s.usuarioRepo.Actualizar(ctx, userID, datos); err != nil {
			return nil, fmt.Errorf("error al actualizar el perfil en base de datos: %w", err)
		}
	}

	// 9. Consultar datos actualizados para retornar DTO consistente
	actualizado, err := s.usuarioRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario actualizado: %w", err)
	}
	if actualizado == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	return &domain.PerfilUsuarioDTO{
		ID:             actualizado.ID.String(),
		Correo:         actualizado.Correo,
		NombreCompleto: actualizado.NombreCompleto,
		Telefono:       actualizado.Telefono,
		Direccion:      actualizado.Direccion,
		Rol:            actualizado.Rol,
		EstaActivo:     actualizado.EstaActivo,
		Mensaje:        "Perfil actualizado exitosamente.",
	}, nil
}

// La validación criptográfica sigue siendo la de Google en producción. Esta
// entrada permite probar claims y fallos sin credenciales ni acceso externo.
func NewAuthServiceWithGoogleValidator(repo domain.UsuarioRepository, validate func(context.Context, string, string) (*idtoken.Payload, error)) AuthService {
	return &authService{usuarioRepo: repo, validateGoogle: validate}
}

// validateGoogleAccount acepta el login con Google sobre una cuenta existente.
//
// MEDIDA PROVISORIA (hasta implementar la confirmación de correo): hoy el
// registro local no verifica el correo, así que cualquiera puede registrar el
// correo de otra persona antes que ella (pre-account hijacking). Como Google sí
// verificó el correo (email_verified), en la primera vinculación se guarda el
// google_id y se invalida la contraseña local, que nunca demostró ser del dueño.
// Cuando exista `correo_verificado`, la contraseña debe conservarse si el correo
// local ya estaba verificado, y ambos métodos de acceso pueden convivir.
func (s *authService) validateGoogleAccount(ctx context.Context, user *domain.Usuario, subject string) (*domain.Usuario, error) {
	if !user.EstaActivo {
		return nil, ErrCuentaInactiva
	}
	if user.GoogleID != nil {
		if *user.GoogleID != subject {
			return nil, ErrCredencialesInvalidas
		}
		return user, nil
	}
	datos := map[string]interface{}{"google_id": subject, "password_hash": nil, "actualizado_el": time.Now()}
	if err := s.usuarioRepo.Actualizar(ctx, user.ID.String(), datos); err != nil {
		if errors.Is(err, ErrCorreoDuplicado) {
			// google_id es único: ya está vinculado a otra cuenta.
			return nil, ErrCredencialesInvalidas
		}
		return nil, fmt.Errorf("error al vincular cuenta con Google: %w", err)
	}
	user.GoogleID = &subject
	user.PasswordHash = nil
	return user, nil
}
