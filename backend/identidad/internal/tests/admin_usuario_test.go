package tests

import (
	"context"
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

var adminActorIDs = map[string]string{
	domain.RolAdmin:       "11111111-1111-4111-8111-111111111111",
	domain.RolColaborador: "22222222-2222-4222-8222-222222222222",
	domain.RolRegistrado:  "33333333-3333-4333-8333-333333333333",
}

func newAdminUsuariosTestRouter(t *testing.T) (*gin.Engine, *mockUsuarioRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	handler := handlers.NewAdminUsuarioHandler(services.NewAdminUsuarioService(repo))
	router := gin.New()
	routes.RegistrarRutasAdminUsuarios(router.Group("/api/v1"), handler, repo)
	return router, repo
}

func asegurarActorAdmin(t *testing.T, repo *mockUsuarioRepository, rol string) string {
	t.Helper()
	actorID, ok := adminActorIDs[rol]
	if !ok {
		t.Fatalf("no hay actor de prueba para el rol %q", rol)
	}
	if existente, _ := repo.FindByID(context.Background(), actorID); existente == nil {
		repo.usuarios["zz-actor-"+rol+"@uct.cl"] = &domain.Usuario{
			ID:             uuid.MustParse(actorID),
			Correo:         "zz-actor-" + rol + "@uct.cl",
			NombreCompleto: "Actor " + rol,
			Rol:            rol,
			EstaActivo:     true,
		}
	}
	return actorID
}

func adminRequest(t *testing.T, router *gin.Engine, repo *mockUsuarioRepository, method, path, rol, body string) *httptest.ResponseRecorder {
	t.Helper()
	actorID := asegurarActorAdmin(t, repo, rol)
	return adminRequestComo(t, router, actorID, rol, method, path, body)
}

func adminRequestComo(t *testing.T, router *gin.Engine, actorID, rol, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	token, err := security.GenerarToken(actorID, rol, "local")
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
	router, repo := newAdminUsuariosTestRouter(t)

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
			w := adminRequest(t, router, repo, tc.method, tc.path, tc.rol, tc.body)
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

	w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	usuarios := pagina.Data
	if len(usuarios) != 3 {
		t.Fatalf("se esperaban 3 usuarios (incluido el actor admin), se obtuvieron %d: %s", len(usuarios), w.Body.String())
	}
	if pagina.Paginacion.TotalRegistros != 3 || pagina.Paginacion.PaginaActual != 1 {
		t.Fatalf("paginación inesperada: %+v", pagina.Paginacion)
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

	w := adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":false,"rol":"colaborador"}`)
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
	w = adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 al reactivar, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if !repo.usuarios["objetivo@uct.cl"].EstaActivo {
		t.Fatal("la cuenta no se reactivó")
	}
}

func TestAdminUsuariosValidaRolYDestino(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)

	w := adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"rol":"superadmin"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("rol inválido debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/no-es-uuid", domain.RolAdmin, `{"esta_activo":false}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("identificador inválido debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"esta_activo":false}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("usuario inexistente debía dar 404, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	w = adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("sin cambios debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUsuariosRechazaFiltrosInvalidos(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	for _, query := range []string{
		"esta_activo=flase",
		"limit=abc",
		"limit=0",
		"limit=201",
		"offset=-1",
		"offset=9223372036854775807&limit=1",
		"page=0",
		"page=2&offset=0",
		"page=9223372036854775807&limit=200",
	} {
		t.Run(query, func(t *testing.T) {
			w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios?"+query, domain.RolAdmin, "")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("se esperaba 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminUsuariosPaginaPorDefecto(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	for i := 0; i < 51; i++ {
		correo := fmt.Sprintf("usuario%02d@uct.cl", i)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(pagina.Data) != 20 {
		t.Fatalf("por defecto se esperaban 20 usuarios, se obtuvieron %d", len(pagina.Data))
	}
	if pagina.Paginacion.TotalRegistros != 52 || pagina.Paginacion.TotalPaginas != 3 || pagina.Paginacion.Limite != 20 {
		t.Fatalf("metadatos de paginación inesperados: %+v", pagina.Paginacion)
	}
}

func TestAdminUsuariosRechazaLimiteExcesivo(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=999", domain.RolAdmin, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUsuariosMetricas(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	repo.usuarios["activo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "activo@uct.cl", NombreCompleto: "Activo", Rol: domain.RolRegistrado, EstaActivo: true,
	}
	repo.usuarios["inactivo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "inactivo@uct.cl", NombreCompleto: "Inactivo", Rol: domain.RolRegistrado, EstaActivo: false,
	}

	w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios/metricas", domain.RolRegistrado, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("un no admin no debe leer métricas, se obtuvo %d", w.Code)
	}

	w = adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios/metricas", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var metricas domain.MetricasUsuariosDTO
	if err := json.Unmarshal(w.Body.Bytes(), &metricas); err != nil {
		t.Fatalf("respuesta inválida: %v", err)
	}
	var total, activos int64
	for _, usuario := range repo.usuarios {
		total++
		if usuario.EstaActivo {
			activos++
		}
	}
	if metricas.TotalUsuarios != total || metricas.UsuariosActivos != activos {
		t.Fatalf("métricas incorrectas: %+v; esperado total=%d activos=%d", metricas, total, activos)
	}
}

func TestAdminUsuariosProtegeUltimoAdminActivo(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	adminID := uuid.New()
	// El rol histórico "administrador" debe contar como admin para la guarda.
	repo.usuarios["unico-admin@uct.cl"] = &domain.Usuario{
		ID: adminID, Correo: "unico-admin@uct.cl", NombreCompleto: "Único Admin", Rol: domain.RolAdminLegacy, EstaActivo: true,
	}
	// Un admin inactivo no cuenta como respaldo para la guarda.
	repo.usuarios["admin-inactivo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "admin-inactivo@uct.cl", NombreCompleto: "Admin Inactivo", Rol: domain.RolAdmin, EstaActivo: false,
	}

	// El propio último admin es el actor para no agregar otro administrador.
	for _, body := range []string{
		`{"esta_activo":false}`,
		`{"rol":"registrado"}`,
		`{"rol":"colaborador"}`,
	} {
		w := adminRequestComo(t, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), body)
		if w.Code != http.StatusConflict {
			t.Fatalf("se esperaba 409 al intentar dejar el sistema sin admins con %s, se obtuvo %d. Body: %s", body, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), domain.ErrUltimoAdmin.Error()) {
			t.Fatalf("el 409 debía exponer el mensaje de negocio del último admin: %s", w.Body.String())
		}
		if !repo.usuarios["unico-admin@uct.cl"].EstaActivo || repo.usuarios["unico-admin@uct.cl"].Rol != domain.RolAdminLegacy {
			t.Fatal("el último admin no debía modificarse")
		}
	}

	// Con otro admin activo la operación sigue prohibida si el actor es el
	// propio usuario: nadie debe desactivarse o degradarse a sí mismo.
	segundoAdminID := uuid.New()
	repo.usuarios["segundo-admin@uct.cl"] = &domain.Usuario{
		ID: segundoAdminID, Correo: "segundo-admin@uct.cl", NombreCompleto: "Segundo Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	w := adminRequestComo(t, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"esta_activo":false}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("un admin no debe desactivarse a sí mismo, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), services.ErrAutoModificacionAdmin.Error()) {
		t.Fatalf("el 409 debía explicar la auto-modificación: %s", w.Body.String())
	}
	if !repo.usuarios["unico-admin@uct.cl"].EstaActivo {
		t.Fatal("la auto-desactivación no debía aplicarse")
	}

	// Otro administrador sí puede desactivarlo.
	w = adminRequestComo(t, router, segundoAdminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"esta_activo":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("otro admin debía poder desactivar la cuenta, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if repo.usuarios["unico-admin@uct.cl"].EstaActivo {
		t.Fatal("el admin debía quedar inactivo")
	}
	if repo.usuarios["unico-admin@uct.cl"].Rol != domain.RolAdmin {
		t.Fatalf("el rol histórico debía normalizarse a admin, se obtuvo %q", repo.usuarios["unico-admin@uct.cl"].Rol)
	}
}

func TestAdminUsuariosRechazaAutoDegradacionPeroPermitePlan(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	adminID := uuid.New()
	repo.usuarios["self-admin@uct.cl"] = &domain.Usuario{
		ID: adminID, Correo: "self-admin@uct.cl", NombreCompleto: "Self Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	repo.usuarios["otro-admin@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "otro-admin@uct.cl", NombreCompleto: "Otro Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}

	for _, body := range []string{`{"rol":"colaborador"}`, `{"rol":"registrado"}`} {
		w := adminRequestComo(t, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), body)
		if w.Code != http.StatusConflict {
			t.Fatalf("se esperaba 409 al auto-degradarse con %s, se obtuvo %d. Body: %s", body, w.Code, w.Body.String())
		}
		if repo.usuarios["self-admin@uct.cl"].Rol != domain.RolAdmin {
			t.Fatal("el rol propio no debía cambiar")
		}
	}

	// Cambiar el plan propio no afecta la autorización y debe permitirse.
	w := adminRequestComo(t, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"plan":"premium"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("el plan propio debía actualizarse, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	if repo.usuarios["self-admin@uct.cl"].Plan != domain.PlanPremium {
		t.Fatal("el plan propio no se persistió")
	}
}

func TestAdminUsuariosNormalizaAliasYAuditaElCambio(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	targetID := uuid.New()
	repo.usuarios["alias@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "alias@uct.cl", NombreCompleto: "Alias", Rol: domain.RolAdminLegacy, EstaActivo: true,
	}

	w := adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"rol":"administrador"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}

	var actualizado domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &actualizado); err != nil {
		t.Fatalf("respuesta inválida: %v", err)
	}
	if actualizado.Rol != domain.RolAdmin {
		t.Fatalf("la respuesta debía normalizar el rol a admin, se obtuvo %q", actualizado.Rol)
	}
	if repo.usuarios["alias@uct.cl"].Rol != domain.RolAdmin {
		t.Fatalf("el rol persistido debía ser admin, se obtuvo %q", repo.usuarios["alias@uct.cl"].Rol)
	}

	if len(repo.auditorias) != 1 {
		t.Fatalf("se esperaba una auditoría, se obtuvieron %d", len(repo.auditorias))
	}
	auditoria := repo.auditorias[0]
	if auditoria.ActorID.String() != adminActorIDs[domain.RolAdmin] {
		t.Fatalf("actor de auditoría incorrecto: %s", auditoria.ActorID)
	}
	if auditoria.ObjetivoID != targetID {
		t.Fatalf("objetivo de auditoría incorrecto: %s", auditoria.ObjetivoID)
	}
	cambioRol, ok := auditoria.Cambios["rol"].(map[string]interface{})
	if !ok {
		t.Fatalf("la auditoría no registró el cambio de rol: %+v", auditoria.Cambios)
	}
	if cambioRol["anterior"] != domain.RolAdminLegacy || cambioRol["nuevo"] != domain.RolAdmin {
		t.Fatalf("cambio de rol inesperado en auditoría: %+v", cambioRol)
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
			w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios?"+caso.query, domain.RolAdmin, "")
			if w.Code != http.StatusOK {
				t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
			}
			var pagina domain.PaginaUsuariosAdminDTO
			if err := json.Unmarshal(w.Body.Bytes(), &pagina); err != nil {
				t.Fatalf("respuesta no es una página de usuarios: %v", err)
			}
			usuarios := pagina.Data
			if len(usuarios) == 0 {
				t.Fatalf("el filtro %s no devolvió usuarios", caso.query)
			}
			if caso.query == "esta_activo=true" && len(usuarios) != 3 {
				t.Fatalf("esta_activo=true debía devolver 3 usuarios (incluido el actor admin), devolvió %d", len(usuarios))
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

	w := adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=2", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &pagina); err != nil {
		t.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(pagina.Data) != 2 {
		t.Fatalf("la segunda página debía traer 2 usuarios (incluido el actor admin), trajo %d", len(pagina.Data))
	}
	if pagina.Paginacion.PaginaActual != 2 || pagina.Paginacion.TotalPaginas != 2 {
		t.Fatalf("metadatos de la segunda página inesperados: %+v", pagina.Paginacion)
	}

	w = adminRequest(t, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=9", domain.RolAdmin, "")
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 en página fuera de rango, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var vacia domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &vacia); err != nil {
		t.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(vacia.Data) != 0 {
		t.Fatalf("la página fuera de rango debía quedar vacía, trajo %d", len(vacia.Data))
	}
}

func TestAdminUsuariosActualizaPlan(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	targetID := uuid.New()
	repo.usuarios["plan@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "plan@uct.cl", NombreCompleto: "Plan", Rol: domain.RolRegistrado,
		Plan: domain.PlanBasico, EstaActivo: true,
	}

	w := adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"plan":"premium"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
	var actualizado domain.UsuarioAdminDTO
	if err := json.Unmarshal(w.Body.Bytes(), &actualizado); err != nil {
		t.Fatalf("respuesta inválida: %v", err)
	}
	if actualizado.Plan != domain.PlanPremium {
		t.Fatalf("plan en respuesta: %q", actualizado.Plan)
	}
	if repo.usuarios["plan@uct.cl"].Plan != domain.PlanPremium {
		t.Fatalf("plan persistido: %q", repo.usuarios["plan@uct.cl"].Plan)
	}
	cambioPlan, ok := repo.auditorias[0].Cambios["plan"].(map[string]interface{})
	if !ok || cambioPlan["anterior"] != domain.PlanBasico || cambioPlan["nuevo"] != domain.PlanPremium {
		t.Fatalf("auditoría de plan inesperada: %+v", repo.auditorias[0].Cambios)
	}

	w = adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"plan":"oro"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("plan inválido debía dar 400, se obtuvo %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAdminUsuariosRechazaCamposDesconocidos(t *testing.T) {
	router, repo := newAdminUsuariosTestRouter(t)
	targetID := uuid.New()
	repo.usuarios["campos@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "campos@uct.cl", NombreCompleto: "Campos", Rol: domain.RolRegistrado,
		Plan: domain.PlanBasico, EstaActivo: true,
	}
	for _, body := range []string{
		`{"esta_activo":false,"id":"otro"}`,
		`{"esta_activo":false} {"esta_activo":true}`,
	} {
		w := adminRequest(t, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("se esperaba 400 con %s, se obtuvo %d. Body: %s", body, w.Code, w.Body.String())
		}
	}
}
