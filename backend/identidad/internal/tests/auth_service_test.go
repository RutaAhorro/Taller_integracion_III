package tests

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
)

// mockUsuarioRepository implementa UsuarioRepository en memoria para pruebas unitarias
type mockUsuarioRepository struct {
	usuarios map[string]*domain.Usuario
}

func newMockUsuarioRepository() *mockUsuarioRepository {
	return &mockUsuarioRepository{
		usuarios: make(map[string]*domain.Usuario),
	}
}

func (m *mockUsuarioRepository) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	if u, ok := m.usuarios[email]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockUsuarioRepository) FindByID(ctx context.Context, id string) (*domain.Usuario, error) {
	for _, u := range m.usuarios {
		if u.ID.String() == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUsuarioRepository) Create(ctx context.Context, usuario *domain.Usuario) error {
	if usuario.ID == uuid.Nil {
		usuario.ID = uuid.New()
	}
	m.usuarios[usuario.Correo] = usuario
	return nil
}

func (m *mockUsuarioRepository) Actualizar(ctx context.Context, id string, datos map[string]interface{}) error {
	var target *domain.Usuario
	for _, u := range m.usuarios {
		if u.ID.String() == id {
			target = u
			break
		}
	}
	if target == nil {
		return errors.New("record not found")
	}

	if nombre, ok := datos["nombre_completo"].(string); ok {
		target.NombreCompleto = nombre
	}
	if correo, ok := datos["correo"].(string); ok {
		delete(m.usuarios, target.Correo)
		target.Correo = correo
		m.usuarios[correo] = target
	}
	if tel, ok := datos["telefono"].(string); ok {
		target.Telefono = &tel
	}
	if dir, ok := datos["direccion"].(string); ok {
		target.Direccion = &dir
	}
	if pwd, ok := datos["password_hash"].(string); ok {
		target.PasswordHash = &pwd
	} else if value, ok := datos["password_hash"]; ok && value == nil {
		target.PasswordHash = nil
	}
	if googleID, ok := datos["google_id"].(string); ok {
		target.GoogleID = &googleID
	}
	if activo, ok := datos["esta_activo"].(bool); ok {
		target.EstaActivo = activo
	}
	if rol, ok := datos["rol"].(string); ok {
		target.Rol = rol
	}
	return nil
}

func (m *mockUsuarioRepository) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, error) {
	var resultado []domain.Usuario
	patron := strings.ToLower(strings.TrimSpace(filtro.Query))
	for _, u := range m.usuarios {
		if filtro.Rol != "" && !strings.EqualFold(strings.TrimSpace(u.Rol), filtro.Rol) {
			continue
		}
		if filtro.EstaActivo != nil && u.EstaActivo != *filtro.EstaActivo {
			continue
		}
		if patron != "" && !strings.Contains(strings.ToLower(u.Correo), patron) && !strings.Contains(strings.ToLower(u.NombreCompleto), patron) {
			continue
		}
		resultado = append(resultado, *u)
	}
	// Orden estable para probar paginación sin depender del mapa.
	sort.Slice(resultado, func(i, j int) bool { return resultado[i].Correo < resultado[j].Correo })
	if filtro.Offset > 0 {
		if filtro.Offset >= len(resultado) {
			return nil, nil
		}
		resultado = resultado[filtro.Offset:]
	}
	if filtro.Limit > 0 && filtro.Limit < len(resultado) {
		resultado = resultado[:filtro.Limit]
	}
	return resultado, nil
}

func (m *mockUsuarioRepository) ContarAdminsActivos(ctx context.Context, excluirID string) (int64, error) {
	var total int64
	for _, u := range m.usuarios {
		if u.ID.String() == excluirID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(u.Rol), domain.RolAdmin) && u.EstaActivo {
			total++
		}
	}
	return total, nil
}

func TestAuthService_PasswordComplexity(t *testing.T) {
	repo := newMockUsuarioRepository()
	service := services.NewAuthService(repo)

	casos := []struct {
		nombre      string
		password    string
		esperaError bool
	}{
		{"Muy corta", "Ab1!", true},
		{"Sin mayuscula", "password123!", true},
		{"Sin numero", "PasswordSinNum!", true},
		{"Sin simbolo", "PasswordSinSimbolo123", true},
		{"Valida", "Vicho159107!", false},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			_, err := service.Registrar(context.Background(), services.RegistroDTO{
				Correo:         "test@uct.cl",
				Password:       tc.password,
				NombreCompleto: "Test User",
			})

			if tc.esperaError && err == nil {
				t.Errorf("Se esperaba error para la clave '%s', pero no ocurrió", tc.password)
			}
			if !tc.esperaError && err != nil {
				t.Errorf("No se esperaba error para la clave '%s', pero ocurrió: %v", tc.password, err)
			}
		})
	}
}

func TestAuthService_RegistroExitoso(t *testing.T) {
	repo := newMockUsuarioRepository()
	service := services.NewAuthService(repo)

	out, err := service.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "Vicente@Uct.CL ",
		Password:       "SuperClave2026#",
		NombreCompleto: "<script>Vicente Matu</script>",
	})

	if err != nil {
		t.Fatalf("Error inesperado en registro: %v", err)
	}

	if out.Correo != "vicente@uct.cl" {
		t.Errorf("Esperaba correo normalizado 'vicente@uct.cl', obtuve '%s'", out.Correo)
	}

	if out.EstaActivo != true {
		t.Errorf("Esperaba que EstaActivo fuera true por auto-login temporal, pero fue false")
	}

	if out.TokenVerificacion.String() == "" {
		t.Errorf("El token de verificacion no debe estar vacio")
	}

	// Verificar persistencia en repo
	u := repo.usuarios["vicente@uct.cl"]
	if u == nil {
		t.Fatalf("Usuario no persistido en el repositorio")
	}

	if u.PasswordHash == nil || !security.CheckPasswordHash("SuperClave2026#", *u.PasswordHash) {
		t.Errorf("El hash almacenado no coincide con la contraseña con bcrypt")
	}

	// Intento de registro duplicado
	_, errDuplicado := service.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "vicente@uct.cl",
		Password:       "SuperClave2026#",
		NombreCompleto: "Vicente Duplicado",
	})

	if !errors.Is(errDuplicado, services.ErrCorreoDuplicado) {
		t.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", errDuplicado)
	}
}

func TestAuthService_Login(t *testing.T) {
	repo := newMockUsuarioRepository()
	service := services.NewAuthService(repo)

	hashPass, _ := security.HashPassword("Password123#")

	// 1. Usuario activo normal
	userActivo := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "activo@uct.cl",
		PasswordHash:   &hashPass,
		NombreCompleto: "Usuario Activo",
		EstaActivo:     true,
	}
	repo.usuarios[userActivo.Correo] = userActivo

	// 2. Usuario con PasswordHash nulo (Login social previo con Google)
	userGoogle := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "google@uct.cl",
		PasswordHash:   nil,
		NombreCompleto: "Usuario Google",
		EstaActivo:     true,
	}
	repo.usuarios[userGoogle.Correo] = userGoogle

	// 3. Usuario inactivo pendiente de verificación
	userInactivo := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "inactivo@uct.cl",
		PasswordHash:   &hashPass,
		NombreCompleto: "Usuario Inactivo",
		EstaActivo:     false,
	}
	repo.usuarios[userInactivo.Correo] = userInactivo

	t.Run("Usuario inexistente", func(t *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "noexiste@uct.cl", "Password123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			t.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", err)
		}
	})

	t.Run("Usuario con PasswordHash nulo (sin panic)", func(t *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "google@uct.cl", "CualquierPass123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			t.Errorf("Esperaba ErrCredencialesInvalidas ante cuenta sin password local, obtuve %v", err)
		}
	})

	t.Run("Cuenta inactiva pendiente de confirmacion", func(t *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "inactivo@uct.cl", "Password123#")
		if !errors.Is(err, services.ErrCuentaInactiva) {
			t.Errorf("Esperaba ErrCuentaInactiva, obtuve %v", err)
		}
	})

	t.Run("Contraseña incorrecta", func(t *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "activo@uct.cl", "PasswordErronea123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			t.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", err)
		}
	})

	t.Run("Login exitoso", func(t *testing.T) {
		u, err := service.LoginWithContext(context.Background(), "ACTIVO@UCT.CL ", "Password123#")
		if err != nil {
			t.Fatalf("Login exitoso falló inesperadamente: %v", err)
		}
		if u.Correo != "activo@uct.cl" {
			t.Errorf("Esperaba correo activo@uct.cl, obtuve %s", u.Correo)
		}
	})
}

func TestAuthService_ActualizarPerfil(t *testing.T) {
	repo := newMockUsuarioRepository()
	service := services.NewAuthService(repo)

	hashPass, _ := security.HashPassword("Password123#")
	userID := uuid.New()
	userOriginal := &domain.Usuario{
		ID:             userID,
		Correo:         "original@uct.cl",
		PasswordHash:   &hashPass,
		NombreCompleto: "Nombre Original",
		Rol:            "registrado",
		EstaActivo:     true,
	}
	repo.usuarios[userOriginal.Correo] = userOriginal

	// Usuario secundario para probar conflicto de correo
	repo.usuarios["otro@uct.cl"] = &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "otro@uct.cl",
		PasswordHash:   &hashPass,
		NombreCompleto: "Otro Usuario",
		EstaActivo:     true,
	}

	t.Run("Actualizacion parcial de nombre, telefono y direccion", func(t *testing.T) {
		nuevoNombre := "Vicente Matus Actualizado"
		nuevoTel := "+56912345678"
		nuevaDir := "Av. Alemania 1234, Temuco"

		res, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
			Direccion:      &nuevaDir,
		})
		if err != nil {
			t.Fatalf("Error inesperado en actualizacion: %v", err)
		}

		if res.NombreCompleto != nuevoNombre {
			t.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, res.NombreCompleto)
		}
		if res.Telefono == nil || *res.Telefono != nuevoTel {
			t.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, res.Telefono)
		}
		if res.Direccion == nil || *res.Direccion != nuevaDir {
			t.Errorf("Esperaba direccion %s, obtuve %v", nuevaDir, res.Direccion)
		}
		if res.Correo != "original@uct.cl" {
			t.Errorf("El correo no debio haber cambiado")
		}
		if !res.EstaActivo {
			t.Errorf("EstaActivo no debio alterarse")
		}
	})

	t.Run("Actualizacion de contraseña con validacion", func(t *testing.T) {
		actual := "Password123#"
		passInvalida := "corta"
		_, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Password:       &passInvalida,
			PasswordActual: &actual,
		})
		if !errors.Is(err, services.ErrPasswordInvalido) {
			t.Errorf("Esperaba ErrPasswordInvalido, obtuve %v", err)
		}

		passValida := "NuevaClaveSegura2026!"
		res, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Password:       &passValida,
			PasswordActual: &actual,
		})
		if err != nil {
			t.Fatalf("Actualizacion con clave valida falló: %v", err)
		}
		if res == nil {
			t.Fatalf("Respuesta nula")
		}

		// Verificar que el hash se actualizo en el usuario
		u := repo.usuarios["original@uct.cl"]
		if !security.CheckPasswordHash("NuevaClaveSegura2026!", *u.PasswordHash) {
			t.Errorf("El hash en repositorio no coincide con la nueva contraseña")
		}
	})

	t.Run("Conflicto al intentar cambiar a correo existente", func(t *testing.T) {
		correoTomado := "otro@uct.cl"
		actual := "NuevaClaveSegura2026!"
		_, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Correo:         &correoTomado,
			PasswordActual: &actual,
		})
		if !errors.Is(err, services.ErrCorreoDuplicado) {
			t.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", err)
		}
	})

	t.Run("Usuario no existente", func(t *testing.T) {
		nombre := "Fantasma"
		_, err := service.ActualizarPerfil(context.Background(), uuid.New().String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nombre,
		})
		if !errors.Is(err, services.ErrUsuarioNoEncontrado) {
			t.Errorf("Esperaba ErrUsuarioNoEncontrado, obtuve %v", err)
		}
	})
}
