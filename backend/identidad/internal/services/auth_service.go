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

// AuthService define los casos de uso para la autenticación e identidad de usuarios
type AuthService interface {
	Registrar(ctx context.Context, input RegistroDTO) (*domain.Usuario, error)
	LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error)
	GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error)
	ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error)
}

// ValidadorGoogle verifica criptográficamente un id_token contra el client ID.
// En producción es idtoken.Validate; las pruebas inyectan claims sin red.
type ValidadorGoogle func(contexto context.Context, tokenGoogle, clienteGoogle string) (*idtoken.Payload, error)

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

// validarComplejidadPassword exige al menos 8 caracteres, una mayúscula, un número y un símbolo
func validarComplejidadPassword(password string) error {
	if len(password) > security.MaximoBytesPassword {
		return fmt.Errorf("%w: longitud máxima de 72 bytes", ErrPasswordInvalido)
	}
	if len(password) < security.MinimoBytesPassword {
		return fmt.Errorf("%w: longitud mínima de 8 caracteres", ErrPasswordInvalido)
	}

	var tieneMayuscula, tieneNumero, tieneSimbolo bool
	for _, caracter := range password {
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
func validarEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address != "" && strings.Contains(email, "@") && strings.Contains(email, ".")
}

// registrarUltimoAcceso deja constancia del acceso sin bloquear el login si
// falla la escritura: la autenticación ya fue validada.
func (servicio *authService) registrarUltimoAcceso(ctx context.Context, usuario *domain.Usuario) {
	if usuario == nil {
		return
	}
	ahora := time.Now()
	if err := servicio.usuarioRepo.Actualizar(ctx, usuario.ID.String(), map[string]interface{}{"ultimo_acceso": ahora}); err != nil {
		slog.Warn("identidad_ultimo_acceso", "user_id", usuario.ID.String(), "error", err)
		return
	}
	usuario.UltimoAcceso = &ahora
}

// Registrar ejecuta el caso de uso completo de registro con validaciones de seguridad
func (servicio *authService) Registrar(ctx context.Context, input RegistroDTO) (*domain.Usuario, error) {
	// 1. Validar complejidad de contraseña antes de computar hash
	if err := validarComplejidadPassword(input.Password); err != nil {
		return nil, err
	}

	// 2. Normalizar y sanitizar entradas
	correoNormalizado := normalizarCorreo(input.Correo)
	nombreSanitizado := SanitizarInputBusqueda(input.NombreCompleto)
	if nombreSanitizado == "" {
		return nil, ErrNombreInvalido
	}

	// 3. Comprobar unicidad del correo electrónico
	existente, err := servicio.usuarioRepo.FindByEmail(ctx, correoNormalizado)
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

	// 5. Instanciar modelo de dominio
	nuevoUsuario := nuevoUsuarioRegistrado(correoNormalizado, nombreSanitizado)
	nuevoUsuario.PasswordHash = &hash

	// 6. Persistir en repositorio
	if err := servicio.usuarioRepo.Create(ctx, nuevoUsuario); err != nil {
		return nil, fmt.Errorf("error al persistir el nuevo usuario: %w", err)
	}

	// El handler arma la respuesta pública, igual que en el login.
	return nuevoUsuario, nil
}

// LoginWithContext ejecuta la autenticación verificando credenciales y estado de forma segura
func (servicio *authService) LoginWithContext(ctx context.Context, correo, password string) (*domain.Usuario, error) {
	// 1. Buscar al usuario por correo usando el repositorio
	correoNormalizado := normalizarCorreo(correo)
	usuario, err := servicio.usuarioRepo.FindByEmail(ctx, correoNormalizado)
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

	servicio.registrarUltimoAcceso(ctx, usuario)
	return usuario, nil
}

func (servicio *authService) GoogleLogin(ctx context.Context, tokenGoogle string) (*domain.Usuario, error) {
	clientID := servicio.googleClientID
	if clientID == "" {
		return nil, ErrGoogleNoConfigurado
	}

	googleCtx, cancel := context.WithTimeout(ctx, servicio.googleTimeout)
	defer cancel()
	payload, err := servicio.validateGoogle(googleCtx, tokenGoogle, clientID)
	if err != nil || payload == nil {
		return nil, ErrCredencialesInvalidas
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return nil, ErrCredencialesInvalidas
	}
	email, emailOK := payload.Claims["email"].(string)
	verified, verifiedOK := payload.Claims["email_verified"].(bool)
	googleID := payload.Subject
	email = normalizarCorreo(email)
	if !emailOK || !verifiedOK || !verified || !validarEmail(email) || googleID == "" {
		return nil, ErrCredencialesInvalidas
	}
	name, _ := payload.Claims["name"].(string)
	name = SanitizarInputBusqueda(name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	var picture *string
	// El avatar es opcional. Omitir valores incompatibles con VARCHAR(500)
	// conserva el login sin persistir una URL cortada o UTF-8 inválido.
	if pic, ok := payload.Claims["picture"].(string); ok && utf8.ValidString(pic) && utf8.RuneCountInString(pic) <= domain.MaximoCaracteresAvatar {
		picture = &pic
	}

	usuario, err := servicio.usuarioRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", err)
	}

	if usuario == nil {
		nuevoUsuario := nuevoUsuarioRegistrado(email, name)
		nuevoUsuario.ID = uuid.New()
		nuevoUsuario.URLAvatar = picture
		nuevoUsuario.GoogleID = &googleID

		err = servicio.usuarioRepo.Create(ctx, nuevoUsuario)
		if err == nil {
			return nuevoUsuario, nil
		}
		if !errors.Is(err, ErrCorreoDuplicado) {
			return nil, fmt.Errorf("error al registrar usuario con Google: %w", err)
		}
		// Otro request pudo crear la misma cuenta entre find y create:
		// se continúa como login sobre la cuenta existente.
		usuario, err = servicio.usuarioRepo.FindByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("error al consultar registro concurrente de Google: %w", err)
		}
		if usuario == nil {
			return nil, ErrCredencialesInvalidas
		}
	}

	usuario, err = servicio.validateGoogleAccount(ctx, usuario, googleID)
	if err != nil {
		return nil, err
	}
	servicio.registrarUltimoAcceso(ctx, usuario)
	return usuario, nil
}

// ActualizarPerfil procesa las actualizaciones parciales del perfil de usuario
func (servicio *authService) ActualizarPerfil(ctx context.Context, userID string, input domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrUsuarioNoEncontrado
	}

	// 1. Verificar existencia del usuario actual
	usuarioActual, err := servicio.usuarioRepo.FindByID(ctx, userID)
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
	cambiaCorreo := input.Correo != nil && normalizarCorreo(*input.Correo) != usuarioActual.Correo
	if cambiaCorreo || input.Password != nil {
		if usuarioActual.PasswordHash == nil || *usuarioActual.PasswordHash == "" ||
			input.PasswordActual == nil || !security.CheckPasswordHash(*input.PasswordActual, *usuarioActual.PasswordHash) {
			return nil, ErrReautenticacionRequerida
		}
	}

	// 2. Procesar NombreCompleto si viene presente
	if input.NombreCompleto != nil {
		nombreSanitizado := SanitizarInputBusqueda(*input.NombreCompleto)
		if nombreSanitizado == "" {
			return nil, ErrNombreInvalido
		}
		datos["nombre_completo"] = nombreSanitizado
	}

	// 3. Procesar Correo si viene presente
	if input.Correo != nil {
		correoNormalizado := normalizarCorreo(*input.Correo)
		if !validarEmail(correoNormalizado) {
			return nil, ErrCorreoInvalido
		}
		// Validar unicidad si el correo ha cambiado
		if correoNormalizado != usuarioActual.Correo {
			existente, err := servicio.usuarioRepo.FindByEmail(ctx, correoNormalizado)
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

	// Seguridad: datos solo recibe las claves de arriba; este caso de uso nunca
	// escribe esta_activo, rol ni cuota_tokens_ia.

	// 7. Si hay cambios dinámicos, persistir
	if len(datos) > 0 {
		datos["actualizado_el"] = time.Now()
		if err := servicio.usuarioRepo.Actualizar(ctx, userID, datos); err != nil {
			return nil, fmt.Errorf("error al actualizar el perfil en base de datos: %w", err)
		}
	}

	// 8. Consultar datos actualizados para retornar DTO consistente
	actualizado, err := servicio.usuarioRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario actualizado: %w", err)
	}
	if actualizado == nil {
		return nil, ErrUsuarioNoEncontrado
	}

	rol := actualizado.Rol
	if canonico, ok := domain.NormalizarRol(actualizado.Rol); ok {
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

// validateGoogleAccount acepta el login con Google sobre una cuenta existente.
//
// MEDIDA PROVISORIA (hasta implementar la confirmación de correo): hoy el
// registro local no verifica el correo, así que cualquiera puede registrar el
// correo de otra persona antes que ella (pre-account hijacking). Como Google sí
// verificó el correo (email_verified), en la primera vinculación se guarda el
// google_id y se invalida la contraseña local, que nunca demostró ser del dueño.
// Cuando exista `correo_verificado`, la contraseña debe conservarse si el correo
// local ya estaba verificado, y ambos métodos de acceso pueden convivir.
func (servicio *authService) validateGoogleAccount(ctx context.Context, user *domain.Usuario, subject string) (*domain.Usuario, error) {
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
	if err := servicio.usuarioRepo.Actualizar(ctx, user.ID.String(), datos); err != nil {
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
