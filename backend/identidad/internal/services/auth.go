// Package services contiene los casos de uso de Identidad: registro, login,
// Google, perfil, administración de cuentas y recuperación de contraseña.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

// ===== Errores =====

var (
	ErrGoogleNoConfigurado = errors.New("login Google no configurado")
	// ErrContrasenaInvalida se emite cuando la contraseña no cumple las reglas de complejidad
	ErrContrasenaInvalida = errors.New("la contraseña debe tener al menos 8 caracteres, una letra mayúscula, un número y un símbolo especial")
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

// ValidadorGoogle verifica criptográficamente un id_token contra el client ID.
// En producción es idtoken.Validate; las pruebas inyectan claims sin red.
type ValidadorGoogle func(contexto context.Context, tokenGoogle, clienteGoogle string) (*idtoken.Payload, error)

// ServicioAuth reúne registro, login local y con Google, y edición del perfil.
type ServicioAuth struct {
	usuarios      domain.RepositorioUsuarios
	validarGoogle ValidadorGoogle
	clienteGoogle string
	plazoGoogle   time.Duration
}

// NuevoServicioAuth recibe el validador de Google, su client ID y su plazo ya
// leídos al arrancar: el servicio nunca consulta el entorno.
func NuevoServicioAuth(repositorio domain.RepositorioUsuarios, validarGoogle ValidadorGoogle, clienteGoogle string, plazoGoogle time.Duration) *ServicioAuth {
	return &ServicioAuth{usuarios: repositorio, validarGoogle: validarGoogle, clienteGoogle: clienteGoogle, plazoGoogle: plazoGoogle}
}

// ===== Validaciones y valores iniciales =====

// validarComplejidadContrasena exige al menos 8 caracteres, una mayúscula, un número y un símbolo
func validarComplejidadContrasena(contrasena string) error {
	if len(contrasena) > MaximoBytesContrasena {
		return fmt.Errorf("%w: longitud máxima de 72 bytes", ErrContrasenaInvalida)
	}
	if len(contrasena) < MinimoBytesContrasena {
		return fmt.Errorf("%w: longitud mínima de 8 caracteres", ErrContrasenaInvalida)
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
		return fmt.Errorf("%w: debe incluir al menos una letra mayúscula", ErrContrasenaInvalida)
	}
	if !tieneNumero {
		return fmt.Errorf("%w: debe incluir al menos un número", ErrContrasenaInvalida)
	}
	if !tieneSimbolo {
		return fmt.Errorf("%w: debe incluir al menos un símbolo especial", ErrContrasenaInvalida)
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

// formatoCorreoValido verifica que el correo tenga un formato sintácticamente válido
func formatoCorreoValido(direccionCorreo string) bool {
	valorNumerico, errorObtenido := mail.ParseAddress(direccionCorreo)
	return errorObtenido == nil && valorNumerico.Address != "" && strings.Contains(direccionCorreo, "@") && strings.Contains(direccionCorreo, ".")
}

// registrarUltimoAcceso deja constancia del acceso sin bloquear el login si
// falla la escritura: la autenticación ya fue validada.
func (servicio *ServicioAuth) registrarUltimoAcceso(contexto context.Context, usuario *domain.Usuario) {
	if usuario == nil {
		return
	}
	ahora := time.Now()
	if errorObtenido := servicio.usuarios.Actualizar(contexto, usuario.ID.String(), map[string]interface{}{"ultimo_acceso": ahora}); errorObtenido != nil {
		slog.Warn("identidad_ultimo_acceso", "user_id", usuario.ID.String(), "error", errorObtenido)
		return
	}
	usuario.UltimoAcceso = &ahora
}

// ===== Registro y login local =====

// Registrar ejecuta el caso de uso completo de registro con validaciones de seguridad
func (servicio *ServicioAuth) Registrar(contexto context.Context, entrada RegistroDTO) (*domain.Usuario, error) {
	// La política se valida antes del hash: bcrypt cuesta CPU y no debe gastarse
	// en una contraseña que se va a rechazar.
	if errorOperacion := validarComplejidadContrasena(entrada.Password); errorOperacion != nil {
		return nil, errorOperacion
	}

	correoNormalizado := normalizarCorreo(entrada.Correo)
	nombreSanitizado := SanitizarNombre(entrada.NombreCompleto)
	if nombreSanitizado == "" {
		return nil, ErrNombreInvalido
	}

	existente, errorObtenido := servicio.usuarios.BuscarPorCorreo(contexto, correoNormalizado)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", errorObtenido)
	}
	if existente != nil {
		return nil, ErrCorreoDuplicado
	}

	hash, errorObtenido := HashearContrasena(entrada.Password)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al generar hash seguro de contraseña: %w", errorObtenido)
	}

	nuevoUsuario := nuevoUsuarioRegistrado(correoNormalizado, nombreSanitizado)
	nuevoUsuario.PasswordHash = &hash

	if errorOperacion := servicio.usuarios.Crear(contexto, nuevoUsuario); errorOperacion != nil {
		return nil, fmt.Errorf("error al persistir el nuevo usuario: %w", errorOperacion)
	}

	// El handler arma la respuesta pública, igual que en el login.
	return nuevoUsuario, nil
}

// Login ejecuta la autenticación verificando credenciales y estado de forma segura
func (servicio *ServicioAuth) Login(contexto context.Context, correo, contrasena string) (*domain.Usuario, error) {
	correoNormalizado := normalizarCorreo(correo)
	usuario, errorObtenido := servicio.usuarios.BuscarPorCorreo(contexto, correoNormalizado)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", errorObtenido)
	}
	if usuario == nil {
		// Error genérico para no revelar si el correo existe o no
		return nil, ErrCredencialesInvalidas
	}

	// Una cuenta creada con Google no tiene contraseña local: se rechaza con el
	// mismo error genérico, sin desreferenciar el hash nulo ni llamar a bcrypt.
	if usuario.PasswordHash == nil || *usuario.PasswordHash == "" {
		return nil, ErrCredencialesInvalidas
	}

	if !usuario.EstaActivo {
		return nil, ErrCuentaInactiva
	}

	if !ContrasenaCoincide(contrasena, *usuario.PasswordHash) {
		return nil, ErrCredencialesInvalidas
	}

	servicio.registrarUltimoAcceso(contexto, usuario)
	return usuario, nil
}

// ===== Login con Google =====

// LoginGoogle valida el id_token de Google, exige correo verificado y busca o
// crea la cuenta; una creación concurrente se resuelve como login.
func (servicio *ServicioAuth) LoginGoogle(contexto context.Context, tokenGoogle string) (*domain.Usuario, error) {
	clienteGoogle := servicio.clienteGoogle
	if clienteGoogle == "" {
		return nil, ErrGoogleNoConfigurado
	}

	contextoGoogle, cancelar := context.WithTimeout(contexto, servicio.plazoGoogle)
	defer cancelar()
	datosGoogle, errorObtenido := servicio.validarGoogle(contextoGoogle, tokenGoogle, clienteGoogle)
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
	if !correoValido || !verificadoValido || !verificado || !formatoCorreoValido(direccionCorreo) || googleID == "" {
		return nil, ErrCredencialesInvalidas
	}
	nombre, _ := datosGoogle.Claims["name"].(string)
	nombre = SanitizarNombre(nombre)
	if nombre == "" {
		nombre = strings.SplitN(direccionCorreo, "@", 2)[0]
	}
	var urlAvatar *string
	// El avatar es opcional. Omitir valores incompatibles con VARCHAR(500)
	// conserva el login sin persistir una URL cortada o UTF-8 inválido.
	if imagen, valido := datosGoogle.Claims["picture"].(string); valido && utf8.ValidString(imagen) && utf8.RuneCountInString(imagen) <= domain.MaximoCaracteresAvatar {
		urlAvatar = &imagen
	}

	usuario, errorObtenido := servicio.usuarios.BuscarPorCorreo(contexto, direccionCorreo)
	if errorObtenido != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", errorObtenido)
	}

	if usuario == nil {
		nuevoUsuario := nuevoUsuarioRegistrado(direccionCorreo, nombre)
		nuevoUsuario.ID = uuid.New()
		nuevoUsuario.URLAvatar = urlAvatar
		nuevoUsuario.GoogleID = &googleID

		errorObtenido = servicio.usuarios.Crear(contexto, nuevoUsuario)
		if errorObtenido == nil {
			return nuevoUsuario, nil
		}
		if !errors.Is(errorObtenido, ErrCorreoDuplicado) {
			return nil, fmt.Errorf("error al registrar usuario con Google: %w", errorObtenido)
		}
		// Otro request pudo crear la misma cuenta entre find y create:
		// se continúa como login sobre la cuenta existente.
		usuario, errorObtenido = servicio.usuarios.BuscarPorCorreo(contexto, direccionCorreo)
		if errorObtenido != nil {
			return nil, fmt.Errorf("error al consultar registro concurrente de Google: %w", errorObtenido)
		}
		if usuario == nil {
			return nil, ErrCredencialesInvalidas
		}
	}

	usuario, errorObtenido = servicio.validarCuentaGoogle(contexto, usuario, googleID)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	servicio.registrarUltimoAcceso(contexto, usuario)
	return usuario, nil
}

// ===== Perfil =====

// ActualizarPerfil procesa las actualizaciones parciales del perfil de usuario
func (servicio *ServicioAuth) ActualizarPerfil(contexto context.Context, usuarioID string, entrada domain.ActualizarPerfilDTO) (*domain.PerfilUsuarioDTO, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	if usuarioID == "" {
		return nil, ErrUsuarioNoEncontrado
	}

	usuarioActual, errorObtenido := servicio.usuarios.BuscarPorID(contexto, usuarioID)
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
			entrada.PasswordActual == nil || !ContrasenaCoincide(*entrada.PasswordActual, *usuarioActual.PasswordHash) {
			return nil, ErrReautenticacionRequerida
		}
	}

	if entrada.NombreCompleto != nil {
		nombreSanitizado := SanitizarNombre(*entrada.NombreCompleto)
		if nombreSanitizado == "" {
			return nil, ErrNombreInvalido
		}
		datos["nombre_completo"] = nombreSanitizado
	}

	if entrada.Correo != nil {
		correoNormalizado := normalizarCorreo(*entrada.Correo)
		if !formatoCorreoValido(correoNormalizado) {
			return nil, ErrCorreoInvalido
		}
		// Validar unicidad si el correo ha cambiado
		if correoNormalizado != usuarioActual.Correo {
			existente, errorOperacion := servicio.usuarios.BuscarPorCorreo(contexto, correoNormalizado)
			if errorOperacion != nil {
				return nil, fmt.Errorf("error consultando disponibilidad del correo: %w", errorOperacion)
			}
			if existente != nil && existente.ID.String() != usuarioID {
				return nil, ErrCorreoDuplicado
			}
			datos["correo"] = correoNormalizado
		}
	}

	if entrada.Telefono != nil {
		datos["telefono"] = strings.TrimSpace(*entrada.Telefono)
	}

	if entrada.Direccion != nil {
		datos["direccion"] = strings.TrimSpace(*entrada.Direccion)
	}

	if entrada.Password != nil {
		if errorOperacion := validarComplejidadContrasena(*entrada.Password); errorOperacion != nil {
			return nil, errorOperacion
		}
		hash, errorInterno := HashearContrasena(*entrada.Password)
		if errorInterno != nil {
			return nil, fmt.Errorf("error al generar hash de contraseña: %w", errorInterno)
		}
		datos["password_hash"] = hash
	}

	// Seguridad: datos solo recibe las claves de arriba; este caso de uso nunca
	// escribe esta_activo, rol ni cuota_tokens_ia.

	if len(datos) > 0 {
		datos["actualizado_el"] = time.Now()
		if errorOperacion := servicio.usuarios.Actualizar(contexto, usuarioID, datos); errorOperacion != nil {
			return nil, fmt.Errorf("error al actualizar el perfil en base de datos: %w", errorOperacion)
		}
	}

	actualizado, errorObtenido := servicio.usuarios.BuscarPorID(contexto, usuarioID)
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

// validarCuentaGoogle acepta el login con Google sobre una cuenta existente.
//
// MEDIDA PROVISORIA (hasta implementar la confirmación de correo): hoy el
// registro local no verifica el correo, así que cualquiera puede registrar el
// correo de otra persona antes que ella (pre-account hijacking). Como Google sí
// verificó el correo (email_verified), en la primera vinculación se guarda el
// google_id y se invalida la contraseña local, que nunca demostró ser del dueño.
// Cuando exista `correo_verificado`, la contraseña debe conservarse si el correo
// local ya estaba verificado, y ambos métodos de acceso pueden convivir.
func (servicio *ServicioAuth) validarCuentaGoogle(contexto context.Context, usuario *domain.Usuario, sujeto string) (*domain.Usuario, error) {
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
	if errorObtenido := servicio.usuarios.Actualizar(contexto, usuario.ID.String(), datos); errorObtenido != nil {
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

// ===== Saneamiento de nombres =====

var caracteresNoPermitidos = regexp.MustCompile(`[^a-zA-Z0-9\sñÑáéíóúÁÉÍÓÚ\-]`)

// maximoCaracteresNombre conserva la política vigente de los perfiles.
const maximoCaracteresNombre = 100

// SanitizarNombre limita nombres a 100 caracteres y filtra su alfabeto.
// La seguridad SQL depende de consultas parametrizadas; HTML debe escaparse al mostrarlo.
func SanitizarNombre(entrada string) string {
	limpio := strings.TrimSpace(entrada)

	// Se cuenta en runas, no en bytes, para no cortar una letra con tilde por la mitad.
	runas := []rune(limpio)
	if len(runas) > maximoCaracteresNombre {
		limpio = string(runas[:maximoCaracteresNombre])
	}

	// Sólo letras (con tilde y ñ), números, espacios y guiones: el alfabeto
	// heredado excluye signos como ;, comillas o corchetes.
	limpio = caracteresNoPermitidos.ReplaceAllString(limpio, "")

	return strings.TrimSpace(limpio)
}

// ===== Contraseñas =====

// Límites de la política vigente y del formato bcrypt; no se alteran en el barrido.
const (
	MinimoBytesContrasena = 8
	MaximoBytesContrasena = 72
)

// HashearContrasena genera un hash seguro a partir de una contraseña en texto plano utilizando bcrypt
func HashearContrasena(contrasena string) (string, error) {
	cantidadBytes, errorObtenido := bcrypt.GenerateFromPassword([]byte(contrasena), bcrypt.DefaultCost)
	if errorObtenido != nil {
		return "", errorObtenido
	}
	return string(cantidadBytes), nil
}

// ContrasenaCoincide compara una contraseña en texto plano contra un hash bcrypt almacenado
func ContrasenaCoincide(contrasena, hash string) bool {
	errorObtenido := bcrypt.CompareHashAndPassword([]byte(hash), []byte(contrasena))
	return errorObtenido == nil
}
