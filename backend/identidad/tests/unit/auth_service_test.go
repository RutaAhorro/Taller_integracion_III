package unit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/gin-gonic/gin"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// mockUsuarioRepository implementa UsuarioRepository en memoria para pruebas unitarias
type mockUsuarioRepository struct {
	usuarios   map[string]*domain.Usuario
	auditorias []domain.AuditoriaAdminUsuario
}

func newMockUsuarioRepository() *mockUsuarioRepository {
	return &mockUsuarioRepository{
		usuarios: make(map[string]*domain.Usuario),
	}
}

func (mensaje *mockUsuarioRepository) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	if usuarioLocal, ok := mensaje.usuarios[email]; ok {
		return usuarioLocal, nil
	}
	return nil, nil
}

func (mensaje *mockUsuarioRepository) FindByID(ctx context.Context, id string) (*domain.Usuario, error) {
	for _, usuarioLocal := range mensaje.usuarios {
		if usuarioLocal.ID.String() == id {
			return usuarioLocal, nil
		}
	}
	return nil, nil
}

func (mensaje *mockUsuarioRepository) Create(ctx context.Context, usuario *domain.Usuario) error {
	if usuario.ID == uuid.Nil {
		usuario.ID = uuid.New()
	}
	mensaje.usuarios[usuario.Correo] = usuario
	return nil
}

func (mensaje *mockUsuarioRepository) Contar(ctx context.Context) (int64, int64, error) {
	var total, activos int64
	for _, usuarioLocal := range mensaje.usuarios {
		total++
		if usuarioLocal.EstaActivo {
			activos++
		}
	}
	return total, activos, nil
}

func (mensaje *mockUsuarioRepository) buscarPorID(id string) *domain.Usuario {
	for _, usuarioLocal := range mensaje.usuarios {
		if usuarioLocal.ID.String() == id {
			return usuarioLocal
		}
	}
	return nil
}

func (mensaje *mockUsuarioRepository) aplicarDatos(target *domain.Usuario, datos map[string]interface{}) {
	if nombre, ok := datos["nombre_completo"].(string); ok {
		target.NombreCompleto = nombre
	}
	if correo, ok := datos["correo"].(string); ok {
		delete(mensaje.usuarios, target.Correo)
		target.Correo = correo
		mensaje.usuarios[correo] = target
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
	if plan, ok := datos["plan"].(string); ok {
		target.Plan = plan
	}
	if ultimo, ok := datos["ultimo_acceso"].(time.Time); ok {
		target.UltimoAcceso = &ultimo
	}
}

func (mensaje *mockUsuarioRepository) Actualizar(ctx context.Context, id string, datos map[string]interface{}) error {
	target := mensaje.buscarPorID(id)
	if target == nil {
		return gorm.ErrRecordNotFound
	}
	mensaje.aplicarDatos(target, datos)
	return nil
}

func (mensaje *mockUsuarioRepository) ActualizarConGuarda(ctx context.Context, id string, datos map[string]interface{}, actorID string) error {
	actorUUID, err := uuid.Parse(strings.TrimSpace(actorID))
	if err != nil {
		return errors.New("actor de auditoría inválido")
	}

	target := mensaje.buscarPorID(id)
	if target == nil {
		return gorm.ErrRecordNotFound
	}

	rolActual, _ := domain.NormalizarRol(target.Rol)
	if _, cambiaRol := datos["rol"]; !cambiaRol {
		if canonico, ok := domain.NormalizarRol(target.Rol); ok && strings.TrimSpace(target.Rol) != canonico {
			datos["rol"] = canonico
		}
	}

	dejaDeSerAdmin := false
	if rolActual == domain.RolAdmin && target.EstaActivo {
		if activo, ok := datos["esta_activo"].(bool); ok && !activo {
			dejaDeSerAdmin = true
		}
		if nuevoRol, ok := datos["rol"].(string); ok {
			if canonico, _ := domain.NormalizarRol(nuevoRol); canonico != domain.RolAdmin {
				dejaDeSerAdmin = true
			}
		}
	}

	if dejaDeSerAdmin {
		var otros int64
		for _, usuarioLocal := range mensaje.usuarios {
			if usuarioLocal.ID == target.ID {
				continue
			}
			rol, ok := domain.NormalizarRol(usuarioLocal.Rol)
			if ok && rol == domain.RolAdmin && usuarioLocal.EstaActivo {
				otros++
			}
		}
		if otros == 0 {
			return domain.ErrUltimoAdmin
		}
		if actorUUID == target.ID {
			return domain.ErrAutoModificacionAdmin
		}
	}

	anteriorRol := target.Rol
	anteriorActivo := target.EstaActivo
	anteriorPlan := target.Plan
	mensaje.aplicarDatos(target, datos)

	cambios := map[string]interface{}{}
	if nuevo, ok := datos["rol"]; ok {
		if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(anteriorRol) {
			cambios["rol"] = map[string]interface{}{"anterior": anteriorRol, "nuevo": nuevo}
		}
	}
	if nuevo, ok := datos["esta_activo"]; ok {
		if activo, esBool := nuevo.(bool); !esBool || activo != anteriorActivo {
			cambios["esta_activo"] = map[string]interface{}{"anterior": anteriorActivo, "nuevo": nuevo}
		}
	}
	if nuevo, ok := datos["plan"]; ok {
		if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(anteriorPlan) {
			cambios["plan"] = map[string]interface{}{"anterior": anteriorPlan, "nuevo": nuevo}
		}
	}
	// Igual que el repositorio real: un PUT idempotente no genera auditoría.
	if len(cambios) == 0 {
		return nil
	}
	mensaje.auditorias = append(mensaje.auditorias, domain.AuditoriaAdminUsuario{
		ActorID:    actorUUID,
		ObjetivoID: target.ID,
		Cambios:    cambios,
	})
	return nil
}

func (mensaje *mockUsuarioRepository) Listar(ctx context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error) {
	var resultado []domain.Usuario
	patron := strings.ToLower(strings.TrimSpace(filtro.Query))
	rolFiltro := ""
	if filtro.Rol != "" {
		if canonico, ok := domain.NormalizarRol(filtro.Rol); ok {
			rolFiltro = canonico
		} else {
			rolFiltro = strings.ToLower(strings.TrimSpace(filtro.Rol))
		}
	}
	for _, usuarioLocal := range mensaje.usuarios {
		if rolFiltro != "" {
			rolUsuario, ok := domain.NormalizarRol(usuarioLocal.Rol)
			if !ok {
				rolUsuario = strings.ToLower(strings.TrimSpace(usuarioLocal.Rol))
			}
			if rolUsuario != rolFiltro {
				continue
			}
		}
		if filtro.EstaActivo != nil && usuarioLocal.EstaActivo != *filtro.EstaActivo {
			continue
		}
		if patron != "" && !strings.Contains(strings.ToLower(usuarioLocal.Correo), patron) && !strings.Contains(strings.ToLower(usuarioLocal.NombreCompleto), patron) {
			continue
		}
		resultado = append(resultado, *usuarioLocal)
	}
	// Orden estable para probar paginación sin depender del mapa.
	sort.Slice(resultado, func(indice, indiceSecundario int) bool {
		return resultado[indice].Correo < resultado[indiceSecundario].Correo
	})
	total := int64(len(resultado))
	if filtro.Offset > 0 {
		if filtro.Offset >= len(resultado) {
			return nil, total, nil
		}
		resultado = resultado[filtro.Offset:]
	}
	if filtro.Limit > 0 && filtro.Limit < len(resultado) {
		resultado = resultado[:filtro.Limit]
	}
	return resultado, total, nil
}

func (mensaje *mockUsuarioRepository) ContarAdminsActivos(ctx context.Context, excluirID string) (int64, error) {
	var total int64
	for _, usuarioLocal := range mensaje.usuarios {
		if usuarioLocal.ID.String() == excluirID {
			continue
		}
		if rol, ok := domain.NormalizarRol(usuarioLocal.Rol); ok && rol == domain.RolAdmin && usuarioLocal.EstaActivo {
			total++
		}
	}
	return total, nil
}

func TestAuthService_PasswordComplexity(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	service := nuevoServicioAuth(repo)

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
		prueba.Run(tc.nombre, func(prueba *testing.T) {
			_, err := service.Registrar(context.Background(), services.RegistroDTO{
				Correo:         "test@uct.cl",
				Password:       tc.password,
				NombreCompleto: "Test User",
			})

			if tc.esperaError && err == nil {
				prueba.Errorf("Se esperaba error para la clave '%s', pero no ocurrió", tc.password)
			}
			if !tc.esperaError && err != nil {
				prueba.Errorf("No se esperaba error para la clave '%s', pero ocurrió: %v", tc.password, err)
			}
		})
	}
}

func TestAuthService_RegistroExitoso(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	service := nuevoServicioAuth(repo)

	out, err := service.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "Vicente@Uct.CL ",
		Password:       "SuperClave2026#",
		NombreCompleto: "<script>Vicente Matu</script>",
	})

	if err != nil {
		prueba.Fatalf("Error inesperado en registro: %v", err)
	}

	if out.Correo != "vicente@uct.cl" {
		prueba.Errorf("Esperaba correo normalizado 'vicente@uct.cl', obtuve '%s'", out.Correo)
	}

	if out.EstaActivo != true {
		prueba.Errorf("Esperaba que EstaActivo fuera true por auto-login temporal, pero fue false")
	}

	// Verificar persistencia en repo
	usuarioLocal := repo.usuarios["vicente@uct.cl"]
	if usuarioLocal == nil {
		prueba.Fatalf("Usuario no persistido en el repositorio")
	}

	if usuarioLocal.PasswordHash == nil || !security.CheckPasswordHash("SuperClave2026#", *usuarioLocal.PasswordHash) {
		prueba.Errorf("El hash almacenado no coincide con la contraseña con bcrypt")
	}
	if usuarioLocal.Plan != domain.PlanBasico || usuarioLocal.UltimoAcceso == nil {
		prueba.Errorf("El registro debía iniciar con plan básico y último acceso: %+v", usuarioLocal)
	}

	// Intento de registro duplicado
	_, errDuplicado := service.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "vicente@uct.cl",
		Password:       "SuperClave2026#",
		NombreCompleto: "Vicente Duplicado",
	})

	if !errors.Is(errDuplicado, services.ErrCorreoDuplicado) {
		prueba.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", errDuplicado)
	}
}

func TestAuthService_Login(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	service := nuevoServicioAuth(repo)

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

	prueba.Run("Usuario inexistente", func(prueba *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "noexiste@uct.cl", "Password123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", err)
		}
	})

	prueba.Run("Usuario con PasswordHash nulo (sin panic)", func(prueba *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "google@uct.cl", "CualquierPass123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas ante cuenta sin password local, obtuve %v", err)
		}
	})

	prueba.Run("Cuenta inactiva pendiente de confirmacion", func(prueba *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "inactivo@uct.cl", "Password123#")
		if !errors.Is(err, services.ErrCuentaInactiva) {
			prueba.Errorf("Esperaba ErrCuentaInactiva, obtuve %v", err)
		}
	})

	prueba.Run("Contraseña incorrecta", func(prueba *testing.T) {
		_, err := service.LoginWithContext(context.Background(), "activo@uct.cl", "PasswordErronea123#")
		if !errors.Is(err, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", err)
		}
	})

	prueba.Run("Login exitoso", func(prueba *testing.T) {
		usuarioLocal, err := service.LoginWithContext(context.Background(), "ACTIVO@UCT.CL ", "Password123#")
		if err != nil {
			prueba.Fatalf("Login exitoso falló inesperadamente: %v", err)
		}
		if usuarioLocal.Correo != "activo@uct.cl" {
			prueba.Errorf("Esperaba correo activo@uct.cl, obtuve %s", usuarioLocal.Correo)
		}
		if usuarioLocal.UltimoAcceso == nil {
			prueba.Error("El login debía registrar ultimo_acceso")
		}
	})
}

func TestAuthService_ActualizarPerfil(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	service := nuevoServicioAuth(repo)

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

	prueba.Run("Actualizacion parcial de nombre, telefono y direccion", func(prueba *testing.T) {
		nuevoNombre := "Vicente Matus Actualizado"
		nuevoTel := "+56912345678"
		nuevaDir := "Av. Alemania 1234, Temuco"

		res, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
			Direccion:      &nuevaDir,
		})
		if err != nil {
			prueba.Fatalf("Error inesperado en actualizacion: %v", err)
		}

		if res.NombreCompleto != nuevoNombre {
			prueba.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, res.NombreCompleto)
		}
		if res.Telefono == nil || *res.Telefono != nuevoTel {
			prueba.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, res.Telefono)
		}
		if res.Direccion == nil || *res.Direccion != nuevaDir {
			prueba.Errorf("Esperaba direccion %s, obtuve %v", nuevaDir, res.Direccion)
		}
		if res.Correo != "original@uct.cl" {
			prueba.Errorf("El correo no debio haber cambiado")
		}
		if !res.EstaActivo {
			prueba.Errorf("EstaActivo no debio alterarse")
		}
	})

	prueba.Run("Actualizacion de contraseña con validacion", func(prueba *testing.T) {
		actual := "Password123#"
		passInvalida := "corta"
		_, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Password:       &passInvalida,
			PasswordActual: &actual,
		})
		if !errors.Is(err, services.ErrPasswordInvalido) {
			prueba.Errorf("Esperaba ErrPasswordInvalido, obtuve %v", err)
		}

		passValida := "NuevaClaveSegura2026!"
		res, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Password:       &passValida,
			PasswordActual: &actual,
		})
		if err != nil {
			prueba.Fatalf("Actualizacion con clave valida falló: %v", err)
		}
		if res == nil {
			prueba.Fatalf("Respuesta nula")
		}

		// Verificar que el hash se actualizo en el usuario
		usuarioLocal := repo.usuarios["original@uct.cl"]
		if !security.CheckPasswordHash("NuevaClaveSegura2026!", *usuarioLocal.PasswordHash) {
			prueba.Errorf("El hash en repositorio no coincide con la nueva contraseña")
		}
	})

	prueba.Run("Conflicto al intentar cambiar a correo existente", func(prueba *testing.T) {
		correoTomado := "otro@uct.cl"
		actual := "NuevaClaveSegura2026!"
		_, err := service.ActualizarPerfil(context.Background(), userID.String(), domain.ActualizarPerfilDTO{
			Correo:         &correoTomado,
			PasswordActual: &actual,
		})
		if !errors.Is(err, services.ErrCorreoDuplicado) {
			prueba.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", err)
		}
	})

	prueba.Run("Usuario no existente", func(prueba *testing.T) {
		nombre := "Fantasma"
		_, err := service.ActualizarPerfil(context.Background(), uuid.New().String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nombre,
		})
		if !errors.Is(err, services.ErrUsuarioNoEncontrado) {
			prueba.Errorf("Esperaba ErrUsuarioNoEncontrado, obtuve %v", err)
		}
	})
}

func TestPasswordBeyondBcryptLimitIsValidationError(prueba *testing.T) {
	servicio := nuevoServicioAuth(newMockUsuarioRepository())
	_, err := servicio.Registrar(prueba.Context(), services.RegistroDTO{Correo: "fixture@example.test", NombreCompleto: "Fixture", Password: "A1!" + strings.Repeat("a", 70)})
	if !errors.Is(err, services.ErrPasswordInvalido) {
		prueba.Fatal(err)
	}
}

// W4 (#99): con una cookie robada no se puede cambiar correo ni contraseña.
func TestActualizarPerfilExigeContrasenaActual(prueba *testing.T) {
	repo := newMockUsuarioRepository()
	service := nuevoServicioAuth(repo)
	hash, _ := security.HashPassword("Password123#")
	local := &domain.Usuario{ID: uuid.New(), Correo: "local@uct.cl", PasswordHash: &hash, NombreCompleto: "Local", EstaActivo: true}
	googleID := "google-only"
	sso := &domain.Usuario{ID: uuid.New(), Correo: "sso@uct.cl", GoogleID: &googleID, NombreCompleto: "SSO", EstaActivo: true}
	repo.usuarios[local.Correo] = local
	repo.usuarios[sso.Correo] = sso

	nueva := "OtraClaveSegura2026!"
	correo := "nuevo@uct.cl"
	mismoCorreo := " LOCAL@uct.cl "
	incorrecta := "NoEsLaClave123#"
	correcta := "Password123#"
	for _, tc := range []struct {
		name    string
		user    *domain.Usuario
		input   domain.ActualizarPerfilDTO
		wantErr error
	}{
		{"contraseña sin actual", local, domain.ActualizarPerfilDTO{Password: &nueva}, services.ErrReautenticacionRequerida},
		{"correo sin actual", local, domain.ActualizarPerfilDTO{Correo: &correo}, services.ErrReautenticacionRequerida},
		{"actual incorrecta", local, domain.ActualizarPerfilDTO{Password: &nueva, PasswordActual: &incorrecta}, services.ErrReautenticacionRequerida},
		{"cuenta Google sin contraseña", sso, domain.ActualizarPerfilDTO{Password: &nueva, PasswordActual: &correcta}, services.ErrReautenticacionRequerida},
		{"mismo correo no es un cambio", local, domain.ActualizarPerfilDTO{Correo: &mismoCorreo}, nil},
	} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			antes := tc.user.PasswordHash
			_, err := service.ActualizarPerfil(context.Background(), tc.user.ID.String(), tc.input)
			if !errors.Is(err, tc.wantErr) {
				prueba.Fatalf("error %v; esperado %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && tc.user.PasswordHash != antes {
				prueba.Fatal("la contraseña cambió sin reautenticación")
			}
		})
	}
	if repo.usuarios["local@uct.cl"] == nil {
		prueba.Fatal("el correo cambió sin reautenticación")
	}
}

func TestProfileNameValidationPreservesStoredName(prueba *testing.T) {
	for _, name := range []string{"@@@;;;", " \t ", "@ @"} {
		prueba.Run(name, func(prueba *testing.T) {
			repo := newMockUsuarioRepository()
			user := &domain.Usuario{ID: uuid.New(), Correo: "profile@example.test", NombreCompleto: "Original"}
			repo.usuarios[user.Correo] = user
			manejador := handlers.NewAuthHandler(nuevoServicioAuth(repo), false, claveJWTDePrueba)
			peticion := gin.New()
			peticion.PUT("/profile", func(contexto *gin.Context) { contexto.Set("user_id", user.ID.String()) }, manejador.ActualizarPerfil)
			body, _ := json.Marshal(map[string]string{"nombre_completo": name})
			request := httptest.NewRequest(http.MethodPut, "/profile", strings.NewReader(string(body)))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			peticion.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || user.NombreCompleto != "Original" {
				prueba.Fatalf("status=%d name=%q body=%s", response.Code, user.NombreCompleto, response.Body)
			}
		})
	}
}

func TestNameSanitizationPreservesUnicodeBoundaries(prueba *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"multibyte boundary", strings.Repeat("a", 99) + "ñz", strings.Repeat("a", 99) + "ñ"},
		{"one hundred accents", strings.Repeat("á", 101), strings.Repeat("á", 100)},
		{"allowed alphabet", "  María Ñúñez-23  ", "María Ñúñez-23"},
		{"invalid UTF8", "A\xffB", "AB"},
		{"filtered whitespace", "@ @", ""},
	} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			got := services.SanitizarInputBusqueda(tc.input)
			if got != tc.want || !utf8.ValidString(got) {
				prueba.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}
