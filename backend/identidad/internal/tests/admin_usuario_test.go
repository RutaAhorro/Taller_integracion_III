package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/security"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newAdminUsuariosTestRouter(t *testing.T) (*gin.Engine, *mockUsuarioRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	handler := handlers.NewAdminUsuarioHandler(services.NewAdminUsuarioService(repo))
	router := gin.New()
	routes.RegistrarRutasAdminUsuarios(router.Group("/api/v1"), handler)
	return router, repo
}

func adminRequest(t *testing.T, router *gin.Engine, method, path, rol, body string) *httptest.ResponseRecorder {
	t.Helper()

	token, err := security.GenerarToken(uuid.NewString(), rol, "local")
	if err != nil {
		t.Fatalf("no se pudo generar el token de prueba: %v", err)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdminUsuariosRechazaNoAdminCon403(t *testing.T) {
	router, _ := newAdminUsuariosTestRouter(t)

	for _, tc := range []struct {
		rol    string
		method string
		path   string
		body   string
	}{
		{"registrado", http.MethodGet, "/api/v1/admin/usuarios", ""},
		{"colaborador", http.MethodGet, "/api/v1/admin/usuarios", ""},
		{"registrado", http.MethodPut, "/api/v1/admin/usuarios/" + uuid.NewString(), `{"esta_activo":false}`},
		{"colaborador", http.MethodPut, "/api/v1/admin/usuarios/" + uuid.NewString(), `{"rol":"colaborador"}`},
	} {
		t.Run(tc.rol+" "+tc.method, func(t *testing.T) {
			w := adminRequest(t, router, tc.method, tc.path, tc.rol, tc.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("se esperaba 403, se obtuvo %d. Body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminUsuariosSinSesionRetorna401(t *testing.T) {
	router, _ := newAdminUsuariosTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usuarios", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("se esperaba 401, se obtuvo %d", w.Code)
	}
}

func TestAdminUsuariosListaConRolAdmin(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	repo.usuarios["activo@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "activo@uct.cl", NombreCompleto: "Activa", Rol: domain.RolRegistrado, EstaActivo: true}
	repo.usuarios["colab@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "colab@uct.cl", NombreCompleto: "Colaboradora", Rol: domain.RolColaborador, EstaActivo: false}

	w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var usuarios []domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &usuarios); err != nil {
		t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
	}
	if len(usuarios) != 2 {
		t.Fatalf("se esperaban 2 usuarios, se obtuvieron %d: %s", len(usuarios), w.Body.String())
	}
	for _, u := range usuarios {
		if u.ID == "" || u.Correo == "" {
			t.Fatalf("usuario sin identidad o correo en el listado: %+v", u)
		}
	}
	if strings.Contains(w.Body.String(), "password_hash") {
		t.Fatal("el listado no debe exponer credenciales")
	}
}

func TestAdminUsuariosActivaDesactivaYAsignaColaborador(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	userID := uuid.New()
	repo.usuarios["objetivo@uct.cl"] = &domain.Usuario{
		ID: userID, Correo: "objetivo@uct.cl", NombreCompleto: "Objetivo", Rol: domain.RolRegistrado, EstaActivo: true,
	}

	w := adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":false,"rol":"colaborador"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var actualizado domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &actualizado); err != nil {
		t.Fatalf("respuesta inválida: %v", err)
	}
	if actualizado.Rol != domain.RolColaborador || actualizado.EstaActivo {
		t.Fatalf("cambios no aplicados en la respuesta: %+v", actualizado)
	}

	persistido := repo.usuarios["objetivo@uct.cl"]
	if persistido == nil || persistido.Rol != domain.RolColaborador || persistido.EstaActivo {
		t.Fatalf("cambios no persistidos: %+v", persistido)
	}

	// Reactivar la cuenta manteniendo el rol colaborador.
	w = adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 al reactivar, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if !repo.usuarios["objetivo@uct.cl"].EstaActivo {
		t.Fatal("la cuenta no se reactivó")
	}
}

func TestAdminUsuariosValidaRolYDestino(t *testing.T) {
	router, _ := newAdminUsuariosTestRouter(t)

	w := adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"rol":"superadmin"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("rol inválido debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/no-es-uuid", domain.RolAdmin, `{"esta_activo":false}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("identificador inválido debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"esta_activo":false}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("usuario inexistente debía dar 404, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("sin cambios debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUsuariosRechazaFiltrosInvalidos(t *testing.T) {
	router, _ := newAdminUsuariosTestRouter(t)
	for _, query := range []string{
		"esta_activo=flase",
		"limit=abc",
		"limit=0",
		"offset=-1",
		"page=0",
		"page=2",
		"page=2&offset=0",
		"page=9223372036854775807&limit=200",
	} {
		t.Run(query, func(t *testing.T) {
			w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios?"+query, domain.RolAdmin, "")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("se esperaba 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminUsuariosSinLimiteDevuelveTodos(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	for i := 0; i < 51; i++ {
		correo := fmt.Sprintf("usuario%02d@uct.cl", i)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var usuarios []domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &usuarios); err != nil {
		t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
	}
	if len(usuarios) != 51 {
		t.Fatalf("sin limit se esperaban 51 usuarios, se obtuvieron %d", len(usuarios))
	}
}

func TestAdminUsuariosAcotaLimiteMaximo(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	for i := 0; i < 205; i++ {
		correo := fmt.Sprintf("usuario%03d@uct.cl", i)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios?limit=999", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var usuarios []domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &usuarios); err != nil {
		t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
	}
	if len(usuarios) != 200 {
		t.Fatalf("el límite máximo debía acotar a 200, se obtuvieron %d", len(usuarios))
	}
}

func TestAdminUsuariosProtegeUltimoAdminActivo(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	adminID := uuid.New()
	repo.usuarios["unico-admin@uct.cl"] = &domain.Usuario{
		ID: adminID, Correo: "unico-admin@uct.cl", NombreCompleto: "Único Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	// Un admin inactivo no cuenta como respaldo para la guarda.
	repo.usuarios["admin-inactivo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "admin-inactivo@uct.cl", NombreCompleto: "Admin Inactivo", Rol: domain.RolAdmin, EstaActivo: false,
	}

	for _, body := range []string{
		`{"esta_activo":false}`,
		`{"rol":"registrado"}`,
		`{"rol":"colaborador"}`,
	} {
		w := adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), domain.RolAdmin, body)
		if w.Code != http.StatusConflict {
			t.Fatalf("se esperaba 409 al intentar dejar el sistema sin admins con %s, se obtuvo %d. Body: %s", body, w.Code, w.Body.String())
		}
		if !repo.usuarios["unico-admin@uct.cl"].EstaActivo || repo.usuarios["unico-admin@uct.cl"].Rol != domain.RolAdmin {
			t.Fatal("el último admin no debía modificarse")
		}
	}

	// Con otro admin activo la operación sí se permite.
	repo.usuarios["segundo-admin@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "segundo-admin@uct.cl", NombreCompleto: "Segundo Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	w := adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), domain.RolAdmin, `{"esta_activo":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("con otro admin activo se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUsuariosFiltraPorTextoRolYEstado(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	repo.usuarios["ana@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "ana@uct.cl", NombreCompleto: "Ana Admin", Rol: domain.RolAdmin, EstaActivo: true}
	repo.usuarios["bruno@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "bruno@uct.cl", NombreCompleto: "Bruno Colaborador", Rol: domain.RolColaborador, EstaActivo: false}
	repo.usuarios["carla@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "carla@uct.cl", NombreCompleto: "Carla Registrada", Rol: domain.RolRegistrado, EstaActivo: true}

	casos := []struct {
		query    string
		esperado string
	}{
		{"q=BRU&rol=colaborador&esta_activo=false", "bruno@uct.cl"},
		{"rol=ADMIN", "ana@uct.cl"},
		{"esta_activo=true", "ana@uct.cl"},
	}

	for _, caso := range casos {
		t.Run(caso.query, func(t *testing.T) {
			w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios?"+caso.query, domain.RolAdmin, "")
			if w.Code != http.StatusOK {
				t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
			}
			var usuarios []domain.UsuarioAdminDTO
			if err := json.Unmarshal(w.Body.Bytes(), &usuarios); err != nil {
				t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
			}
			if len(usuarios) == 0 {
				t.Fatalf("el filtro %s no devolvió usuarios", caso.query)
			}
			if caso.query == "esta_activo=true" && len(usuarios) != 2 {
				t.Fatalf("esta_activo=true debía devolver 2 usuarios, devolvió %d", len(usuarios))
			}
			if caso.query != "esta_activo=true" && usuarios[0].Correo != caso.esperado {
				t.Fatalf("el filtro %s devolvió %q, se esperaba %q", caso.query, usuarios[0].Correo, caso.esperado)
			}
		})
	}
}

func TestAdminUsuariosPaginaYFueraDeRango(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	for i := 0; i < 3; i++ {
		correo := fmt.Sprintf("usuario%02d@uct.cl", i)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	w := adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=2", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var segundaPagina []domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &segundaPagina); err != nil {
		t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
	}
	if len(segundaPagina) != 1 {
		t.Fatalf("la segunda página debía traer 1 usuario, trajo %d", len(segundaPagina))
	}

	w = adminRequest(t, router, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=9", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 en página fuera de rango, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var vacia []domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &vacia); err != nil {
		t.Fatalf("respuesta no es un arreglo de usuarios: %v", err)
	}
	if len(vacia) != 0 {
		t.Fatalf("la página fuera de rango debía quedar vacía, trajo %d", len(vacia))
	}
}
