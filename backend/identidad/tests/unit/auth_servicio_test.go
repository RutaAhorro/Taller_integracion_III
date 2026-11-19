// Casos de uso de registro, login y perfil sobre un repositorio en memoria.

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

	"github.com/gin-gonic/gin"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// mockUsuarioRepository implementa RepositorioUsuarios en memoria para pruebas unitarias
type mockUsuarioRepository struct {
	usuarios   map[string]*domain.Usuario
	auditorias []domain.AuditoriaAdminUsuario
}

// newMockUsuarioRepository crea el repositorio en memoria compartido por las pruebas.
func newMockUsuarioRepository() *mockUsuarioRepository {
	return &mockUsuarioRepository{
		usuarios: make(map[string]*domain.Usuario),
	}
}

// BuscarPorCorreo busca por correo exacto, como la consulta real.
func (repositorio *mockUsuarioRepository) BuscarPorCorreo(contexto context.Context, direccionCorreo string) (*domain.Usuario, error) {
	if usuarioLocal, valido := repositorio.usuarios[direccionCorreo]; valido {
		return usuarioLocal, nil
	}
	return nil, nil
}

// BuscarPorID busca por identificador.
func (repositorio *mockUsuarioRepository) BuscarPorID(contexto context.Context, id string) (*domain.Usuario, error) {
	for _, usuarioLocal := range repositorio.usuarios {
		if usuarioLocal.ID.String() == id {
			return usuarioLocal, nil
		}
	}
	return nil, nil
}

// Crear asigna un ID si falta y rechaza correos duplicados.
func (repositorio *mockUsuarioRepository) Crear(contexto context.Context, usuario *domain.Usuario) error {
	if usuario.ID == uuid.Nil {
		usuario.ID = uuid.New()
	}
	repositorio.usuarios[usuario.Correo] = usuario
	return nil
}

// Contar devuelve el total y las cuentas activas.
func (repositorio *mockUsuarioRepository) Contar(contexto context.Context) (int64, int64, error) {
	var total, activos int64
	for _, usuarioLocal := range repositorio.usuarios {
		total++
		if usuarioLocal.EstaActivo {
			activos++
		}
	}
	return total, activos, nil
}

// buscarPorID localiza la cuenta sin copiarla, para modificarla en el lugar.
func (repositorio *mockUsuarioRepository) buscarPorID(id string) *domain.Usuario {
	for _, usuarioLocal := range repositorio.usuarios {
		if usuarioLocal.ID.String() == id {
			return usuarioLocal
		}
	}
	return nil
}

// aplicarDatos replica el Updates de GORM sobre las columnas conocidas.
func (repositorio *mockUsuarioRepository) aplicarDatos(objetivo *domain.Usuario, datos map[string]interface{}) {
	if nombre, valido := datos["nombre_completo"].(string); valido {
		objetivo.NombreCompleto = nombre
	}
	if correo, valido := datos["correo"].(string); valido {
		delete(repositorio.usuarios, objetivo.Correo)
		objetivo.Correo = correo
		repositorio.usuarios[correo] = objetivo
	}
	if telefono, valido := datos["telefono"].(string); valido {
		objetivo.Telefono = &telefono
	}
	if directorio, valido := datos["direccion"].(string); valido {
		objetivo.Direccion = &directorio
	}
	if contrasena, existe := datos["password_hash"].(string); existe {
		objetivo.PasswordHash = &contrasena
	} else if valor, valido := datos["password_hash"]; valido && valor == nil {
		objetivo.PasswordHash = nil
	}
	if googleID, valido := datos["google_id"].(string); valido {
		objetivo.GoogleID = &googleID
	}
	if activo, valido := datos["esta_activo"].(bool); valido {
		objetivo.EstaActivo = activo
	}
	if rol, valido := datos["rol"].(string); valido {
		objetivo.Rol = rol
	}
	if plan, valido := datos["plan"].(string); valido {
		objetivo.Plan = plan
	}
	if ultimo, valido := datos["ultimo_acceso"].(time.Time); valido {
		objetivo.UltimoAcceso = &ultimo
	}
}

// Actualizar aplica cambios parciales y falla si la cuenta no existe.
func (repositorio *mockUsuarioRepository) Actualizar(contexto context.Context, id string, datos map[string]interface{}) error {
	objetivo := repositorio.buscarPorID(id)
	if objetivo == nil {
		return gorm.ErrRecordNotFound
	}
	repositorio.aplicarDatos(objetivo, datos)
	return nil
}

// ActualizarConGuarda reproduce la guarda del último admin y la auditoría
// del repositorio real, para probar el servicio sin PostgreSQL.
func (repositorio *mockUsuarioRepository) ActualizarConGuarda(contexto context.Context, id string, datos map[string]interface{}, actorID string) error {
	actorUUID, errorObtenido := uuid.Parse(strings.TrimSpace(actorID))
	if errorObtenido != nil {
		return errors.New("actor de auditoría inválido")
	}

	objetivo := repositorio.buscarPorID(id)
	if objetivo == nil {
		return gorm.ErrRecordNotFound
	}

	rolActual, _ := domain.NormalizarRol(objetivo.Rol)
	if _, cambiaRol := datos["rol"]; !cambiaRol {
		if canonico, valido := domain.NormalizarRol(objetivo.Rol); valido && strings.TrimSpace(objetivo.Rol) != canonico {
			datos["rol"] = canonico
		}
	}

	dejaDeSerAdmin := false
	if rolActual == domain.RolAdmin && objetivo.EstaActivo {
		if activo, valido := datos["esta_activo"].(bool); valido && !activo {
			dejaDeSerAdmin = true
		}
		if nuevoRol, valido := datos["rol"].(string); valido {
			if canonico, _ := domain.NormalizarRol(nuevoRol); canonico != domain.RolAdmin {
				dejaDeSerAdmin = true
			}
		}
	}

	if dejaDeSerAdmin {
		var otros int64
		for _, usuarioLocal := range repositorio.usuarios {
			if usuarioLocal.ID == objetivo.ID {
				continue
			}
			rol, valido := domain.NormalizarRol(usuarioLocal.Rol)
			if valido && rol == domain.RolAdmin && usuarioLocal.EstaActivo {
				otros++
			}
		}
		if otros == 0 {
			return domain.ErrUltimoAdmin
		}
		if actorUUID == objetivo.ID {
			return domain.ErrAutoModificacionAdmin
		}
	}

	anteriorRol := objetivo.Rol
	anteriorActivo := objetivo.EstaActivo
	anteriorPlan := objetivo.Plan
	repositorio.aplicarDatos(objetivo, datos)

	cambios := map[string]interface{}{}
	if nuevo, valido := datos["rol"]; valido {
		if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(anteriorRol) {
			cambios["rol"] = map[string]interface{}{"anterior": anteriorRol, "nuevo": nuevo}
		}
	}
	if nuevo, valido := datos["esta_activo"]; valido {
		if activo, esBool := nuevo.(bool); !esBool || activo != anteriorActivo {
			cambios["esta_activo"] = map[string]interface{}{"anterior": anteriorActivo, "nuevo": nuevo}
		}
	}
	if nuevo, valido := datos["plan"]; valido {
		if texto, esTexto := nuevo.(string); !esTexto || strings.TrimSpace(texto) != strings.TrimSpace(anteriorPlan) {
			cambios["plan"] = map[string]interface{}{"anterior": anteriorPlan, "nuevo": nuevo}
		}
	}
	// Igual que el repositorio real: un PUT idempotente no genera auditoría.
	if len(cambios) == 0 {
		return nil
	}
	repositorio.auditorias = append(repositorio.auditorias, domain.AuditoriaAdminUsuario{
		ActorID:    actorUUID,
		ObjetivoID: objetivo.ID,
		Cambios:    cambios,
	})
	return nil
}

// Listar aplica filtros, orden y paginación como la consulta real.
func (repositorio *mockUsuarioRepository) Listar(contexto context.Context, filtro domain.FiltroUsuariosAdminDTO) ([]domain.Usuario, int64, error) {
	var resultado []domain.Usuario
	patron := strings.ToLower(strings.TrimSpace(filtro.Query))
	rolFiltro := ""
	if filtro.Rol != "" {
		if canonico, valido := domain.NormalizarRol(filtro.Rol); valido {
			rolFiltro = canonico
		} else {
			rolFiltro = strings.ToLower(strings.TrimSpace(filtro.Rol))
		}
	}
	for _, usuarioLocal := range repositorio.usuarios {
		if rolFiltro != "" {
			rolUsuario, valido := domain.NormalizarRol(usuarioLocal.Rol)
			if !valido {
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
	if desplazamiento := filtro.Desplazamiento(); desplazamiento > 0 {
		if desplazamiento >= len(resultado) {
			return nil, total, nil
		}
		resultado = resultado[desplazamiento:]
	}
	if filtro.Limit > 0 && filtro.Limit < len(resultado) {
		resultado = resultado[:filtro.Limit]
	}
	return resultado, total, nil
}

// ContarAdminsActivos cuenta admins activos distintos de excluirID.
func (repositorio *mockUsuarioRepository) ContarAdminsActivos(contexto context.Context, excluirID string) (int64, error) {
	var total int64
	for _, usuarioLocal := range repositorio.usuarios {
		if usuarioLocal.ID.String() == excluirID {
			continue
		}
		if rol, valido := domain.NormalizarRol(usuarioLocal.Rol); valido && rol == domain.RolAdmin && usuarioLocal.EstaActivo {
			total++
		}
	}
	return total, nil
}

func TestServicioAuthComplejidadContrasena(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	servicio := nuevoServicioAuth(repositorio)

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

	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			_, errorObtenido := servicio.Registrar(context.Background(), services.RegistroDTO{
				Correo:         "test@uct.cl",
				Password:       caso.password,
				NombreCompleto: "Test User",
			})

			if caso.esperaError && errorObtenido == nil {
				prueba.Errorf("Se esperaba error para la clave '%s', pero no ocurrió", caso.password)
			}
			if !caso.esperaError && errorObtenido != nil {
				prueba.Errorf("No se esperaba error para la clave '%s', pero ocurrió: %v", caso.password, errorObtenido)
			}
		})
	}
}

func TestServicioAuthRegistroExitoso(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	servicio := nuevoServicioAuth(repositorio)

	resultado, errorObtenido := servicio.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "Vicente@Uct.CL ",
		Password:       "SuperClave2026#",
		NombreCompleto: "<script>Vicente Matu</script>",
	})

	if errorObtenido != nil {
		prueba.Fatalf("Error inesperado en registro: %v", errorObtenido)
	}

	if resultado.Correo != "vicente@uct.cl" {
		prueba.Errorf("Esperaba correo normalizado 'vicente@uct.cl', obtuve '%s'", resultado.Correo)
	}

	if resultado.EstaActivo != true {
		prueba.Errorf("Esperaba que EstaActivo fuera true por auto-login temporal, pero fue false")
	}

	// Verificar persistencia en repo
	usuarioLocal := repositorio.usuarios["vicente@uct.cl"]
	if usuarioLocal == nil {
		prueba.Fatalf("Usuario no persistido en el repositorio")
	}

	if usuarioLocal.PasswordHash == nil || !services.ContrasenaCoincide("SuperClave2026#", *usuarioLocal.PasswordHash) {
		prueba.Errorf("El hash almacenado no coincide con la contraseña con bcrypt")
	}
	if usuarioLocal.Plan != domain.PlanBasico || usuarioLocal.UltimoAcceso == nil {
		prueba.Errorf("El registro debía iniciar con plan básico y último acceso: %+v", usuarioLocal)
	}

	// Intento de registro duplicado
	_, errorDuplicado := servicio.Registrar(context.Background(), services.RegistroDTO{
		Correo:         "vicente@uct.cl",
		Password:       "SuperClave2026#",
		NombreCompleto: "Vicente Duplicado",
	})

	if !errors.Is(errorDuplicado, services.ErrCorreoDuplicado) {
		prueba.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", errorDuplicado)
	}
}

func TestServicioAuthLogin(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	servicio := nuevoServicioAuth(repositorio)

	hashContrasena, _ := services.HashearContrasena("Password123#")

	usuarioActivo := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "activo@uct.cl",
		PasswordHash:   &hashContrasena,
		NombreCompleto: "Usuario Activo",
		EstaActivo:     true,
	}
	repositorio.usuarios[usuarioActivo.Correo] = usuarioActivo

	// Cuenta creada con Google: sin contraseña local.
	usuarioGoogle := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "google@uct.cl",
		PasswordHash:   nil,
		NombreCompleto: "Usuario Google",
		EstaActivo:     true,
	}
	repositorio.usuarios[usuarioGoogle.Correo] = usuarioGoogle

	usuarioInactivo := &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "inactivo@uct.cl",
		PasswordHash:   &hashContrasena,
		NombreCompleto: "Usuario Inactivo",
		EstaActivo:     false,
	}
	repositorio.usuarios[usuarioInactivo.Correo] = usuarioInactivo

	prueba.Run("Usuario inexistente", func(prueba *testing.T) {
		_, errorObtenido := servicio.Login(context.Background(), "noexiste@uct.cl", "Password123#")
		if !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", errorObtenido)
		}
	})

	prueba.Run("Usuario con PasswordHash nulo (sin panic)", func(prueba *testing.T) {
		_, errorObtenido := servicio.Login(context.Background(), "google@uct.cl", "CualquierPass123#")
		if !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas ante cuenta sin password local, obtuve %v", errorObtenido)
		}
	})

	prueba.Run("Cuenta inactiva pendiente de confirmacion", func(prueba *testing.T) {
		_, errorObtenido := servicio.Login(context.Background(), "inactivo@uct.cl", "Password123#")
		if !errors.Is(errorObtenido, services.ErrCuentaInactiva) {
			prueba.Errorf("Esperaba ErrCuentaInactiva, obtuve %v", errorObtenido)
		}
	})

	prueba.Run("Contraseña incorrecta", func(prueba *testing.T) {
		_, errorObtenido := servicio.Login(context.Background(), "activo@uct.cl", "PasswordErronea123#")
		if !errors.Is(errorObtenido, services.ErrCredencialesInvalidas) {
			prueba.Errorf("Esperaba ErrCredencialesInvalidas, obtuve %v", errorObtenido)
		}
	})

	prueba.Run("Login exitoso", func(prueba *testing.T) {
		usuarioLocal, errorObtenido := servicio.Login(context.Background(), "ACTIVO@UCT.CL ", "Password123#")
		if errorObtenido != nil {
			prueba.Fatalf("Login exitoso falló inesperadamente: %v", errorObtenido)
		}
		if usuarioLocal.Correo != "activo@uct.cl" {
			prueba.Errorf("Esperaba correo activo@uct.cl, obtuve %s", usuarioLocal.Correo)
		}
		if usuarioLocal.UltimoAcceso == nil {
			prueba.Error("El login debía registrar ultimo_acceso")
		}
	})
}

func TestServicioAuthActualizarPerfil(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	servicio := nuevoServicioAuth(repositorio)

	hashContrasena, _ := services.HashearContrasena("Password123#")
	usuarioID := uuid.New()
	usuarioOriginal := &domain.Usuario{
		ID:             usuarioID,
		Correo:         "original@uct.cl",
		PasswordHash:   &hashContrasena,
		NombreCompleto: "Nombre Original",
		Rol:            "registrado",
		EstaActivo:     true,
	}
	repositorio.usuarios[usuarioOriginal.Correo] = usuarioOriginal

	// Usuario secundario para probar conflicto de correo
	repositorio.usuarios["otro@uct.cl"] = &domain.Usuario{
		ID:             uuid.New(),
		Correo:         "otro@uct.cl",
		PasswordHash:   &hashContrasena,
		NombreCompleto: "Otro Usuario",
		EstaActivo:     true,
	}

	prueba.Run("Actualizacion parcial de nombre, telefono y direccion", func(prueba *testing.T) {
		nuevoNombre := "Vicente Matus Actualizado"
		nuevoTel := "+56912345678"
		nuevaDir := "Av. Alemania 1234, Temuco"

		resultado, errorObtenido := servicio.ActualizarPerfil(context.Background(), usuarioID.String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nuevoNombre,
			Telefono:       &nuevoTel,
			Direccion:      &nuevaDir,
		})
		if errorObtenido != nil {
			prueba.Fatalf("Error inesperado en actualizacion: %v", errorObtenido)
		}

		if resultado.NombreCompleto != nuevoNombre {
			prueba.Errorf("Esperaba nombre %s, obtuve %s", nuevoNombre, resultado.NombreCompleto)
		}
		if resultado.Telefono == nil || *resultado.Telefono != nuevoTel {
			prueba.Errorf("Esperaba telefono %s, obtuve %v", nuevoTel, resultado.Telefono)
		}
		if resultado.Direccion == nil || *resultado.Direccion != nuevaDir {
			prueba.Errorf("Esperaba direccion %s, obtuve %v", nuevaDir, resultado.Direccion)
		}
		if resultado.Correo != "original@uct.cl" {
			prueba.Errorf("El correo no debio haber cambiado")
		}
		if !resultado.EstaActivo {
			prueba.Errorf("EstaActivo no debio alterarse")
		}
	})

	prueba.Run("Actualizacion de contraseña con validacion", func(prueba *testing.T) {
		actual := "Password123#"
		contrasenaInvalida := "corta"
		_, errorObtenido := servicio.ActualizarPerfil(context.Background(), usuarioID.String(), domain.ActualizarPerfilDTO{
			Password:       &contrasenaInvalida,
			PasswordActual: &actual,
		})
		if !errors.Is(errorObtenido, services.ErrContrasenaInvalida) {
			prueba.Errorf("Esperaba ErrContrasenaInvalida, obtuve %v", errorObtenido)
		}

		contrasenaValida := "NuevaClaveSegura2026!"
		resultado, errorObtenido := servicio.ActualizarPerfil(context.Background(), usuarioID.String(), domain.ActualizarPerfilDTO{
			Password:       &contrasenaValida,
			PasswordActual: &actual,
		})
		if errorObtenido != nil {
			prueba.Fatalf("Actualizacion con clave valida falló: %v", errorObtenido)
		}
		if resultado == nil {
			prueba.Fatalf("Respuesta nula")
		}

		// Verificar que el hash se actualizo en el usuario
		usuarioLocal := repositorio.usuarios["original@uct.cl"]
		if !services.ContrasenaCoincide("NuevaClaveSegura2026!", *usuarioLocal.PasswordHash) {
			prueba.Errorf("El hash en repositorio no coincide con la nueva contraseña")
		}
	})

	prueba.Run("Conflicto al intentar cambiar a correo existente", func(prueba *testing.T) {
		correoTomado := "otro@uct.cl"
		actual := "NuevaClaveSegura2026!"
		_, errorObtenido := servicio.ActualizarPerfil(context.Background(), usuarioID.String(), domain.ActualizarPerfilDTO{
			Correo:         &correoTomado,
			PasswordActual: &actual,
		})
		if !errors.Is(errorObtenido, services.ErrCorreoDuplicado) {
			prueba.Errorf("Esperaba ErrCorreoDuplicado, obtuve %v", errorObtenido)
		}
	})

	prueba.Run("Usuario no existente", func(prueba *testing.T) {
		nombre := "Fantasma"
		_, errorObtenido := servicio.ActualizarPerfil(context.Background(), uuid.New().String(), domain.ActualizarPerfilDTO{
			NombreCompleto: &nombre,
		})
		if !errors.Is(errorObtenido, services.ErrUsuarioNoEncontrado) {
			prueba.Errorf("Esperaba ErrUsuarioNoEncontrado, obtuve %v", errorObtenido)
		}
	})
}

func TestPasswordBeyondBcryptLimitIsValidationError(prueba *testing.T) {
	servicio := nuevoServicioAuth(newMockUsuarioRepository())
	_, errorObtenido := servicio.Registrar(prueba.Context(), services.RegistroDTO{Correo: "fixture@example.test", NombreCompleto: "Fixture", Password: "A1!" + strings.Repeat("a", 70)})
	if !errors.Is(errorObtenido, services.ErrContrasenaInvalida) {
		prueba.Fatal(errorObtenido)
	}
}

// W4 (#99): con una cookie robada no se puede cambiar correo ni contraseña.
func TestActualizarPerfilExigeContrasenaActual(prueba *testing.T) {
	repositorio := newMockUsuarioRepository()
	servicio := nuevoServicioAuth(repositorio)
	hash, _ := services.HashearContrasena("Password123#")
	local := &domain.Usuario{ID: uuid.New(), Correo: "local@uct.cl", PasswordHash: &hash, NombreCompleto: "Local", EstaActivo: true}
	googleID := "google-only"
	sso := &domain.Usuario{ID: uuid.New(), Correo: "sso@uct.cl", GoogleID: &googleID, NombreCompleto: "SSO", EstaActivo: true}
	repositorio.usuarios[local.Correo] = local
	repositorio.usuarios[sso.Correo] = sso

	nueva := "OtraClaveSegura2026!"
	correo := "nuevo@uct.cl"
	mismoCorreo := " LOCAL@uct.cl "
	incorrecta := "NoEsLaClave123#"
	correcta := "Password123#"
	for _, caso := range []struct {
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
		prueba.Run(caso.name, func(prueba *testing.T) {
			antes := caso.user.PasswordHash
			_, errorObtenido := servicio.ActualizarPerfil(context.Background(), caso.user.ID.String(), caso.input)
			if !errors.Is(errorObtenido, caso.wantErr) {
				prueba.Fatalf("error %v; esperado %v", errorObtenido, caso.wantErr)
			}
			if caso.wantErr != nil && caso.user.PasswordHash != antes {
				prueba.Fatal("la contraseña cambió sin reautenticación")
			}
		})
	}
	if repositorio.usuarios["local@uct.cl"] == nil {
		prueba.Fatal("el correo cambió sin reautenticación")
	}
}

func TestProfileNameValidationPreservesStoredName(prueba *testing.T) {
	for _, nombre := range []string{"@@@;;;", " \t ", "@ @"} {
		prueba.Run(nombre, func(prueba *testing.T) {
			repositorio := newMockUsuarioRepository()
			usuario := &domain.Usuario{ID: uuid.New(), Correo: "profile@example.test", NombreCompleto: "Original"}
			repositorio.usuarios[usuario.Correo] = usuario
			manejador := api.NuevoManejadorAuth(nuevoServicioAuth(repositorio), false, claveJWTDePrueba)
			enrutador := gin.New()
			enrutador.PUT("/profile", func(contexto *gin.Context) { contexto.Set("user_id", usuario.ID.String()) }, manejador.ActualizarPerfil)
			cuerpo, _ := json.Marshal(map[string]string{"nombre_completo": nombre})
			solicitud := httptest.NewRequest(http.MethodPut, "/profile", strings.NewReader(string(cuerpo)))
			solicitud.Header.Set("Content-Type", "application/json")
			respuestaHTTP := httptest.NewRecorder()
			enrutador.ServeHTTP(respuestaHTTP, solicitud)
			if respuestaHTTP.Code != http.StatusBadRequest || usuario.NombreCompleto != "Original" {
				prueba.Fatalf("status=%d name=%q body=%s", respuestaHTTP.Code, usuario.NombreCompleto, respuestaHTTP.Body)
			}
		})
	}
}

func TestNameSanitizationPreservesUnicodeBoundaries(prueba *testing.T) {
	for _, caso := range []struct{ name, input, want string }{
		{"multibyte boundary", strings.Repeat("a", 99) + "ñz", strings.Repeat("a", 99) + "ñ"},
		{"one hundred accents", strings.Repeat("á", 101), strings.Repeat("á", 100)},
		{"allowed alphabet", "  María Ñúñez-23  ", "María Ñúñez-23"},
		{"invalid UTF8", "A\xffB", "AB"},
		{"filtered whitespace", "@ @", ""},
	} {
		prueba.Run(caso.name, func(prueba *testing.T) {
			obtenido := services.SanitizarNombre(caso.input)
			if obtenido != caso.want || !utf8.ValidString(obtenido) {
				prueba.Fatalf("got=%q want=%q", obtenido, caso.want)
			}
		})
	}
}
