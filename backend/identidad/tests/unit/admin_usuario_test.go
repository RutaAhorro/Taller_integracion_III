package unit_test

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

func newAdminUsuariosTestRouter(prueba *testing.T) (*gin.Engine, *mockUsuarioRepository) {
	prueba.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMockUsuarioRepository()
	handler := handlers.NewAdminUsuarioHandler(services.NewAdminUsuarioService(repo))
	router := gin.New()
	handlers.RegistrarRutasAdminUsuarios(router.Group("/api/v1"), handler, repo, claveJWTDePrueba)
	return router, repo
}

func asegurarActorAdmin(prueba *testing.T, repo *mockUsuarioRepository, rol string) string {
	prueba.Helper()
	actorID, ok := adminActorIDs[rol]
	if !ok {
		prueba.Fatalf("no hay actor de prueba para el rol %q", rol)
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

func adminRequest(prueba *testing.T, router *gin.Engine, repo *mockUsuarioRepository, method, path, rol, body string) *httptest.ResponseRecorder {
	prueba.Helper()
	actorID := asegurarActorAdmin(prueba, repo, rol)
	return adminRequestComo(prueba, router, actorID, rol, method, path, body)
}

func adminRequestComo(prueba *testing.T, router *gin.Engine, actorID, rol, method, path, body string) *httptest.ResponseRecorder {
	prueba.Helper()

	token, err := security.GenerarToken(actorID, rol, "local", claveJWTDePrueba)
	if err != nil {
		prueba.Fatalf("no se pudo generar el token de prueba: %v", err)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
	escritor := httptest.NewRecorder()
	router.ServeHTTP(escritor, req)
	return escritor
}

func TestAdminUsuariosRechazaNoAdminCon403(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)

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
		prueba.Run(tc.rol+" "+tc.method, func(prueba *testing.T) {
			escritor := adminRequest(prueba, router, repo, tc.method, tc.path, tc.rol, tc.body)
			if escritor.Code != http.StatusForbidden {
				prueba.Fatalf("se esperaba 403, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
			}
		})
	}
}

func TestAdminUsuariosSinSesionRetorna401(prueba *testing.T) {
	router, _ := newAdminUsuariosTestRouter(prueba)
	escritor := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/usuarios", nil)
	router.ServeHTTP(escritor, req)
	if escritor.Code != http.StatusUnauthorized {
		prueba.Fatalf("se esperaba 401, se obtuvo %d", escritor.Code)
	}
}

func TestAdminUsuariosListaConRolAdmin(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	repo.usuarios["activo@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "activo@uct.cl", NombreCompleto: "Activa", Rol: domain.RolRegistrado, EstaActivo: true}
	repo.usuarios["colab@uct.cl"] = &domain.Usuario{ID: uuid.New(), Correo: "colab@uct.cl", NombreCompleto: "Colaboradora", Rol: domain.RolColaborador, EstaActivo: false}

	escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &pagina); err != nil {
		prueba.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	usuarios := pagina.Data
	if len(usuarios) != 3 {
		prueba.Fatalf("se esperaban 3 usuarios (incluido el actor admin), se obtuvieron %d: %s", len(usuarios), escritor.Body.String())
	}
	if pagina.Paginacion.TotalRegistros != 3 || pagina.Paginacion.PaginaActual != 1 {
		prueba.Fatalf("paginación inesperada: %+v", pagina.Paginacion)
	}
	for _, usuarioLocal := range usuarios {
		if usuarioLocal.ID == "" || usuarioLocal.Correo == "" {
			prueba.Fatalf("usuario sin identidad o correo en el listado: %+v", usuarioLocal)
		}
	}
	if strings.Contains(escritor.Body.String(), "password_hash") {
		prueba.Fatal("el listado no debe exponer credenciales")
	}
}

func TestAdminUsuariosActivaDesactivaYAsignaColaborador(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	userID := uuid.New()
	repo.usuarios["objetivo@uct.cl"] = &domain.Usuario{
		ID: userID, Correo: "objetivo@uct.cl", NombreCompleto: "Objetivo", Rol: domain.RolRegistrado, EstaActivo: true,
	}

	escritor := adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":false,"rol":"colaborador"}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	var actualizado domain.UsuarioAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &actualizado); err != nil {
		prueba.Fatalf("respuesta inválida: %v", err)
	}
	if actualizado.Rol != domain.RolColaborador || actualizado.EstaActivo {
		prueba.Fatalf("cambios no aplicados en la respuesta: %+v", actualizado)
	}

	persistido := repo.usuarios["objetivo@uct.cl"]
	if persistido == nil || persistido.Rol != domain.RolColaborador || persistido.EstaActivo {
		prueba.Fatalf("cambios no persistidos: %+v", persistido)
	}

	// Reactivar la cuenta manteniendo el rol colaborador.
	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+userID.String(), domain.RolAdmin, `{"esta_activo":true}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200 al reactivar, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if !repo.usuarios["objetivo@uct.cl"].EstaActivo {
		prueba.Fatal("la cuenta no se reactivó")
	}
}

func TestAdminUsuariosValidaRolYDestino(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)

	escritor := adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"rol":"superadmin"}`)
	if escritor.Code != http.StatusBadRequest {
		prueba.Fatalf("rol inválido debía dar 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/no-es-uuid", domain.RolAdmin, `{"esta_activo":false}`)
	if escritor.Code != http.StatusBadRequest {
		prueba.Fatalf("identificador inválido debía dar 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{"esta_activo":false}`)
	if escritor.Code != http.StatusNotFound {
		prueba.Fatalf("usuario inexistente debía dar 404, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+uuid.NewString(), domain.RolAdmin, `{}`)
	if escritor.Code != http.StatusBadRequest {
		prueba.Fatalf("sin cambios debía dar 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
}

func TestAdminUsuariosRechazaFiltrosInvalidos(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	for _, query := range []string{
		"esta_activo=flase",
		"limit=abc",
		"limit=0",
		"limit=201",
		"page=0",
		"page=9223372036854775807&limit=200",
	} {
		prueba.Run(query, func(prueba *testing.T) {
			escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?"+query, domain.RolAdmin, "")
			if escritor.Code != http.StatusBadRequest {
				prueba.Fatalf("se esperaba 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
			}
		})
	}
}

func TestAdminUsuariosPaginaPorDefecto(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	for indice := 0; indice < 51; indice++ {
		correo := fmt.Sprintf("usuario%02d@uct.cl", indice)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios", domain.RolAdmin, "")
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}

	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &pagina); err != nil {
		prueba.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(pagina.Data) != 20 {
		prueba.Fatalf("por defecto se esperaban 20 usuarios, se obtuvieron %d", len(pagina.Data))
	}
	if pagina.Paginacion.TotalRegistros != 52 || pagina.Paginacion.TotalPaginas != 3 || pagina.Paginacion.Limite != 20 {
		prueba.Fatalf("metadatos de paginación inesperados: %+v", pagina.Paginacion)
	}
}

func TestAdminUsuariosRechazaLimiteExcesivo(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=999", domain.RolAdmin, "")
	if escritor.Code != http.StatusBadRequest {
		prueba.Fatalf("se esperaba 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
}

func TestAdminUsuariosMetricas(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	repo.usuarios["activo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "activo@uct.cl", NombreCompleto: "Activo", Rol: domain.RolRegistrado, EstaActivo: true,
	}
	repo.usuarios["inactivo@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "inactivo@uct.cl", NombreCompleto: "Inactivo", Rol: domain.RolRegistrado, EstaActivo: false,
	}

	escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios/metricas", domain.RolRegistrado, "")
	if escritor.Code != http.StatusForbidden {
		prueba.Fatalf("un no admin no debe leer métricas, se obtuvo %d", escritor.Code)
	}

	escritor = adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios/metricas", domain.RolAdmin, "")
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var metricas domain.MetricasUsuariosDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &metricas); err != nil {
		prueba.Fatalf("respuesta inválida: %v", err)
	}
	var total, activos int64
	for _, usuario := range repo.usuarios {
		total++
		if usuario.EstaActivo {
			activos++
		}
	}
	if metricas.TotalUsuarios != total || metricas.UsuariosActivos != activos {
		prueba.Fatalf("métricas incorrectas: %+v; esperado total=%d activos=%d", metricas, total, activos)
	}
}

func TestAdminUsuariosProtegeUltimoAdminActivo(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	adminID := uuid.New()
	repo.usuarios["unico-admin@uct.cl"] = &domain.Usuario{
		ID: adminID, Correo: "unico-admin@uct.cl", NombreCompleto: "Único Admin", Rol: domain.RolAdmin, EstaActivo: true,
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
		escritor := adminRequestComo(prueba, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), body)
		if escritor.Code != http.StatusConflict {
			prueba.Fatalf("se esperaba 409 al intentar dejar el sistema sin admins con %s, se obtuvo %d. Body: %s", body, escritor.Code, escritor.Body.String())
		}
		if !strings.Contains(escritor.Body.String(), domain.ErrUltimoAdmin.Error()) {
			prueba.Fatalf("el 409 debía exponer el mensaje de negocio del último admin: %s", escritor.Body.String())
		}
		if !repo.usuarios["unico-admin@uct.cl"].EstaActivo || repo.usuarios["unico-admin@uct.cl"].Rol != domain.RolAdmin {
			prueba.Fatal("el último admin no debía modificarse")
		}
	}

	// Con otro admin activo la operación sigue prohibida si el actor es el
	// propio usuario: nadie debe desactivarse o degradarse a sí mismo.
	segundoAdminID := uuid.New()
	repo.usuarios["segundo-admin@uct.cl"] = &domain.Usuario{
		ID: segundoAdminID, Correo: "segundo-admin@uct.cl", NombreCompleto: "Segundo Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	escritor := adminRequestComo(prueba, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"esta_activo":false}`)
	if escritor.Code != http.StatusConflict {
		prueba.Fatalf("un admin no debe desactivarse a sí mismo, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if !strings.Contains(escritor.Body.String(), services.ErrAutoModificacionAdmin.Error()) {
		prueba.Fatalf("el 409 debía explicar la auto-modificación: %s", escritor.Body.String())
	}
	if !repo.usuarios["unico-admin@uct.cl"].EstaActivo {
		prueba.Fatal("la auto-desactivación no debía aplicarse")
	}

	// Otro administrador sí puede desactivarlo.
	escritor = adminRequestComo(prueba, router, segundoAdminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"esta_activo":false}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("otro admin debía poder desactivar la cuenta, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if repo.usuarios["unico-admin@uct.cl"].EstaActivo {
		prueba.Fatal("el admin debía quedar inactivo")
	}
	if repo.usuarios["unico-admin@uct.cl"].Rol != domain.RolAdmin {
		prueba.Fatalf("el rol histórico debía normalizarse a admin, se obtuvo %q", repo.usuarios["unico-admin@uct.cl"].Rol)
	}
}

func TestAdminUsuariosRechazaAutoDegradacionPeroPermitePlan(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	adminID := uuid.New()
	repo.usuarios["self-admin@uct.cl"] = &domain.Usuario{
		ID: adminID, Correo: "self-admin@uct.cl", NombreCompleto: "Self Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}
	repo.usuarios["otro-admin@uct.cl"] = &domain.Usuario{
		ID: uuid.New(), Correo: "otro-admin@uct.cl", NombreCompleto: "Otro Admin", Rol: domain.RolAdmin, EstaActivo: true,
	}

	for _, body := range []string{`{"rol":"colaborador"}`, `{"rol":"registrado"}`} {
		escritor := adminRequestComo(prueba, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), body)
		if escritor.Code != http.StatusConflict {
			prueba.Fatalf("se esperaba 409 al auto-degradarse con %s, se obtuvo %d. Body: %s", body, escritor.Code, escritor.Body.String())
		}
		if repo.usuarios["self-admin@uct.cl"].Rol != domain.RolAdmin {
			prueba.Fatal("el rol propio no debía cambiar")
		}
	}

	// Cambiar el plan propio no afecta la autorización y debe permitirse.
	escritor := adminRequestComo(prueba, router, adminID.String(), domain.RolAdmin, http.MethodPut, "/api/v1/admin/usuarios/"+adminID.String(), `{"plan":"premium"}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("el plan propio debía actualizarse, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if repo.usuarios["self-admin@uct.cl"].Plan != domain.PlanPremium {
		prueba.Fatal("el plan propio no se persistió")
	}
}

func TestAdminUsuariosNoAuditaMutacionIdempotente(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	targetID := uuid.New()
	repo.usuarios["idempotente@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "idempotente@uct.cl", NombreCompleto: "Idempotente",
		Rol: domain.RolRegistrado, Plan: domain.PlanBasico, EstaActivo: true,
	}

	// Mismos valores: no hay cambio efectivo, no debe quedar auditoría.
	escritor := adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"rol":"registrado","esta_activo":true,"plan":"basico"}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if len(repo.auditorias) != 0 {
		prueba.Fatalf("un PUT idempotente no debía auditar, se obtuvieron %d registros", len(repo.auditorias))
	}

	// Un cambio real sí se audita una sola vez.
	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"plan":"premium"}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	if len(repo.auditorias) != 1 {
		prueba.Fatalf("se esperaba una auditoría, se obtuvieron %d", len(repo.auditorias))
	}
}

func TestAdminUsuariosFiltraPorTextoRolYEstado(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
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
		prueba.Run(caso.query, func(prueba *testing.T) {
			escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?"+caso.query, domain.RolAdmin, "")
			if escritor.Code != http.StatusOK {
				prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
			}
			var pagina domain.PaginaUsuariosAdminDTO
			if err := json.Unmarshal(escritor.Body.Bytes(), &pagina); err != nil {
				prueba.Fatalf("respuesta no es una página de usuarios: %v", err)
			}
			usuarios := pagina.Data
			if len(usuarios) == 0 {
				prueba.Fatalf("el filtro %s no devolvió usuarios", caso.query)
			}
			if caso.query == "esta_activo=true" && len(usuarios) != 3 {
				prueba.Fatalf("esta_activo=true debía devolver 3 usuarios (incluido el actor admin), devolvió %d", len(usuarios))
			}
			if caso.query != "esta_activo=true" && usuarios[0].Correo != caso.esperado {
				prueba.Fatalf("el filtro %s devolvió %q, se esperaba %q", caso.query, usuarios[0].Correo, caso.esperado)
			}
		})
	}
}

func TestAdminUsuariosPaginaYFueraDeRango(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	for indice := 0; indice < 3; indice++ {
		correo := fmt.Sprintf("usuario%02d@uct.cl", indice)
		repo.usuarios[correo] = &domain.Usuario{ID: uuid.New(), Correo: correo, NombreCompleto: "Usuario", Rol: domain.RolRegistrado, EstaActivo: true}
	}

	escritor := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=2", domain.RolAdmin, "")
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var pagina domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &pagina); err != nil {
		prueba.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(pagina.Data) != 2 {
		prueba.Fatalf("la segunda página debía traer 2 usuarios (incluido el actor admin), trajo %d", len(pagina.Data))
	}
	if pagina.Paginacion.PaginaActual != 2 || pagina.Paginacion.TotalPaginas != 2 {
		prueba.Fatalf("metadatos de la segunda página inesperados: %+v", pagina.Paginacion)
	}

	escritor = adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?limit=2&page=9", domain.RolAdmin, "")
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200 en página fuera de rango, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var vacia domain.PaginaUsuariosAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &vacia); err != nil {
		prueba.Fatalf("respuesta no es una página de usuarios: %v", err)
	}
	if len(vacia.Data) != 0 {
		prueba.Fatalf("la página fuera de rango debía quedar vacía, trajo %d", len(vacia.Data))
	}
}

func TestAdminUsuariosActualizaPlan(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	targetID := uuid.New()
	repo.usuarios["plan@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "plan@uct.cl", NombreCompleto: "Plan", Rol: domain.RolRegistrado,
		Plan: domain.PlanBasico, EstaActivo: true,
	}

	escritor := adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"plan":"premium"}`)
	if escritor.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
	var actualizado domain.UsuarioAdminDTO
	if err := json.Unmarshal(escritor.Body.Bytes(), &actualizado); err != nil {
		prueba.Fatalf("respuesta inválida: %v", err)
	}
	if actualizado.Plan != domain.PlanPremium {
		prueba.Fatalf("plan en respuesta: %q", actualizado.Plan)
	}
	if repo.usuarios["plan@uct.cl"].Plan != domain.PlanPremium {
		prueba.Fatalf("plan persistido: %q", repo.usuarios["plan@uct.cl"].Plan)
	}
	cambioPlan, ok := repo.auditorias[0].Cambios["plan"].(map[string]interface{})
	if !ok || cambioPlan["anterior"] != domain.PlanBasico || cambioPlan["nuevo"] != domain.PlanPremium {
		prueba.Fatalf("auditoría de plan inesperada: %+v", repo.auditorias[0].Cambios)
	}

	escritor = adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, `{"plan":"oro"}`)
	if escritor.Code != http.StatusBadRequest {
		prueba.Fatalf("plan inválido debía dar 400, se obtuvo %d. Body: %s", escritor.Code, escritor.Body.String())
	}
}

func TestAdminUsuariosRechazaCamposDesconocidos(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	targetID := uuid.New()
	repo.usuarios["campos@uct.cl"] = &domain.Usuario{
		ID: targetID, Correo: "campos@uct.cl", NombreCompleto: "Campos", Rol: domain.RolRegistrado,
		Plan: domain.PlanBasico, EstaActivo: true,
	}
	for _, body := range []string{
		`{"esta_activo":false,"id":"otro"}`,
		`{"esta_activo":false} {"esta_activo":true}`,
	} {
		escritor := adminRequest(prueba, router, repo, http.MethodPut, "/api/v1/admin/usuarios/"+targetID.String(), domain.RolAdmin, body)
		if escritor.Code != http.StatusBadRequest {
			prueba.Fatalf("se esperaba 400 con %s, se obtuvo %d. Body: %s", body, escritor.Code, escritor.Body.String())
		}
	}
}

// TestAdminUsuariosRechazaAliasAdministrador fija el contrato tras el barrido:
// "administrador" dejó de ser un rol válido y sólo se acepta "admin".
func TestAdminUsuariosRechazaAliasAdministrador(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	objetivoID := uuid.New()
	repo.usuarios["objetivo@uct.cl"] = &domain.Usuario{
		ID: objetivoID, Correo: "objetivo@uct.cl", NombreCompleto: "Objetivo", Rol: domain.RolRegistrado, EstaActivo: true,
	}
	for _, peticion := range []struct{ metodo, ruta, cuerpo string }{
		{http.MethodPut, "/api/v1/admin/usuarios/" + objetivoID.String(), `{"rol":"administrador"}`},
		{http.MethodGet, "/api/v1/admin/usuarios?rol=administrador", ""},
	} {
		respuesta := adminRequest(prueba, router, repo, peticion.metodo, peticion.ruta, domain.RolAdmin, peticion.cuerpo)
		if respuesta.Code != http.StatusBadRequest {
			prueba.Fatalf("%s %s: se esperaba 400, se obtuvo %d. Body: %s", peticion.metodo, peticion.ruta, respuesta.Code, respuesta.Body.String())
		}
	}
	if repo.usuarios["objetivo@uct.cl"].Rol != domain.RolRegistrado || len(repo.auditorias) != 0 {
		prueba.Fatal("un rol inválido no debe modificar la cuenta ni dejar auditoría")
	}
}

// TestAdminUsuariosIgnoraOffset fija el contrato tras el barrido: la paginación
// usa sólo page y limit; un offset heredado no altera la página devuelta.
func TestAdminUsuariosIgnoraOffset(prueba *testing.T) {
	router, repo := newAdminUsuariosTestRouter(prueba)
	respuesta := adminRequest(prueba, router, repo, http.MethodGet, "/api/v1/admin/usuarios?offset=50", domain.RolAdmin, "")
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("se esperaba 200, se obtuvo %d. Body: %s", respuesta.Code, respuesta.Body.String())
	}
	var pagina domain.PaginaUsuariosAdminDTO
	if errorLectura := json.Unmarshal(respuesta.Body.Bytes(), &pagina); errorLectura != nil || pagina.Paginacion.PaginaActual != 1 {
		prueba.Fatalf("offset no debía cambiar la página: %+v %v", pagina.Paginacion, errorLectura)
	}
}
