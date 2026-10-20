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

func TestAdminUsuariosIdEnCuerpo(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	userID := uuid.New()
	repo.usuarios["cuerpo@uct.cl"] = &domain.Usuario{ID: userID, Correo: "cuerpo@uct.cl", NombreCompleto: "Cuerpo", Rol: domain.RolRegistrado, EstaActivo: true}

	payload := `{"id":"` + userID.String() + `","rol":"colaborador"}`
	w := adminRequest(t, router, http.MethodPut, "/api/v1/admin/usuarios", domain.RolAdmin, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 con id en el cuerpo, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if repo.usuarios["cuerpo@uct.cl"].Rol != domain.RolColaborador {
		t.Fatal("el rol colaborador no se persistió")
	}
}

func TestAdminUsuariosAplicaLimitePorDefecto(t *testing.T) {
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
	if len(usuarios) != 50 {
		t.Fatalf("se esperaba el límite por defecto de 50, se obtuvieron %d", len(usuarios))
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
