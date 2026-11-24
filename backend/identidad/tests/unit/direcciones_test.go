// Direcciones guardadas del usuario: autenticación, validación de campos,
// aislamiento entre cuentas (404, nunca 403), límite por usuario y reglas de la
// dirección principal, sobre un repositorio en memoria que imita el real.

package unit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Repositorio en memoria =====

// direccionesEnMemoria implementa RepositorioDirecciones con las mismas reglas
// observables del repositorio SQL: filtro por dueño, límite, una principal y
// promoción al borrar. La concurrencia real se prueba contra PostgreSQL.
type direccionesEnMemoria struct {
	filas  []domain.DireccionUsuario
	reloj  time.Time
	fallos error
}

// nuevoRepositorioDirecciones crea el repositorio con un reloj determinista.
func nuevoRepositorioDirecciones() *direccionesEnMemoria {
	return &direccionesEnMemoria{reloj: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// delUsuario devuelve las filas del usuario en el orden documentado.
func (repositorio *direccionesEnMemoria) delUsuario(usuarioID string) []domain.DireccionUsuario {
	var propias []domain.DireccionUsuario
	for _, fila := range repositorio.filas {
		if fila.UsuarioID.String() == usuarioID {
			propias = append(propias, fila)
		}
	}
	sort.SliceStable(propias, func(indice, otro int) bool {
		if propias[indice].EsPrincipal != propias[otro].EsPrincipal {
			return propias[indice].EsPrincipal
		}
		return propias[indice].CreadoEl.Before(propias[otro].CreadoEl)
	})
	return propias
}

// Listar devuelve las direcciones del usuario.
func (repositorio *direccionesEnMemoria) Listar(_ context.Context, usuarioID string) ([]domain.DireccionUsuario, error) {
	return repositorio.delUsuario(usuarioID), repositorio.fallos
}

// Crear aplica límite, primera principal y desmarcado de la anterior.
func (repositorio *direccionesEnMemoria) Crear(_ context.Context, usuarioID string, direccion *domain.DireccionUsuario, limite int) error {
	if repositorio.fallos != nil {
		return repositorio.fallos
	}
	existentes := repositorio.delUsuario(usuarioID)
	if len(existentes) >= limite {
		return domain.ErrLimiteDirecciones
	}
	if len(existentes) == 0 {
		direccion.EsPrincipal = true
	} else if direccion.EsPrincipal {
		repositorio.desmarcar(usuarioID)
	}
	repositorio.reloj = repositorio.reloj.Add(time.Second)
	direccion.ID, direccion.UsuarioID, direccion.CreadoEl = uuid.New(), uuid.MustParse(usuarioID), repositorio.reloj
	repositorio.filas = append(repositorio.filas, *direccion)
	return nil
}

// desmarcar quita la principal del usuario.
func (repositorio *direccionesEnMemoria) desmarcar(usuarioID string) {
	for indice := range repositorio.filas {
		if repositorio.filas[indice].UsuarioID.String() == usuarioID {
			repositorio.filas[indice].EsPrincipal = false
		}
	}
}

// buscar devuelve el índice de la fila del usuario o -1 si no es suya.
func (repositorio *direccionesEnMemoria) buscar(usuarioID, direccionID string) int {
	for indice, fila := range repositorio.filas {
		if fila.UsuarioID.String() == usuarioID && fila.ID.String() == direccionID {
			return indice
		}
	}
	return -1
}

// Actualizar reemplaza los campos editables y cambia la principal si se pide.
func (repositorio *direccionesEnMemoria) Actualizar(_ context.Context, usuarioID, direccionID string, datos domain.DireccionUsuario, esPrincipal *bool) (*domain.DireccionUsuario, error) {
	posicion := repositorio.buscar(usuarioID, direccionID)
	if posicion < 0 {
		return nil, domain.ErrDireccionNoEncontrada
	}
	if esPrincipal != nil && !*esPrincipal && repositorio.filas[posicion].EsPrincipal {
		return nil, domain.ErrPrincipalObligatoria
	}
	if esPrincipal != nil && *esPrincipal && !repositorio.filas[posicion].EsPrincipal {
		repositorio.desmarcar(usuarioID)
		repositorio.filas[posicion].EsPrincipal = true
	}
	fila := &repositorio.filas[posicion]
	fila.Etiqueta, fila.TextoDireccion, fila.Comuna, fila.Lat, fila.Lon = datos.Etiqueta, datos.TextoDireccion, datos.Comuna, datos.Lat, datos.Lon
	copia := *fila
	return &copia, nil
}

// Eliminar borra la fila y promueve a la más antigua si era la principal.
func (repositorio *direccionesEnMemoria) Eliminar(_ context.Context, usuarioID, direccionID string) error {
	posicion := repositorio.buscar(usuarioID, direccionID)
	if posicion < 0 {
		return domain.ErrDireccionNoEncontrada
	}
	eraPrincipal := repositorio.filas[posicion].EsPrincipal
	repositorio.filas = append(repositorio.filas[:posicion], repositorio.filas[posicion+1:]...)
	if restantes := repositorio.delUsuario(usuarioID); eraPrincipal && len(restantes) > 0 {
		repositorio.filas[repositorio.buscar(usuarioID, restantes[0].ID.String())].EsPrincipal = true
	}
	return nil
}

// ===== Preparación =====

// escenarioDirecciones agrupa el router, el repositorio y dos cuentas.
type escenarioDirecciones struct {
	enrutador   *gin.Engine
	direcciones *direccionesEnMemoria
	usuarios    *mockUsuarioRepository
	primero     string
	segundo     string
}

// nuevoEscenarioDirecciones monta las rutas reales con dos usuarios registrados.
func nuevoEscenarioDirecciones(prueba *testing.T) *escenarioDirecciones {
	prueba.Helper()
	gin.SetMode(gin.TestMode)
	usuarios := newMockUsuarioRepository()
	direcciones := nuevoRepositorioDirecciones()
	escenario := &escenarioDirecciones{enrutador: gin.New(), direcciones: direcciones, usuarios: usuarios}
	for _, nombre := range []string{"uno", "dos"} {
		cuenta := &domain.Usuario{ID: uuid.New(), Correo: nombre + "@uct.cl", NombreCompleto: nombre, Rol: domain.RolRegistrado, EstaActivo: true}
		usuarios.usuarios[cuenta.Correo] = cuenta
		if nombre == "uno" {
			escenario.primero = cuenta.ID.String()
		} else {
			escenario.segundo = cuenta.ID.String()
		}
	}
	manejador := api.NuevoManejadorDirecciones(services.NuevoServicioDirecciones(direcciones))
	api.RegistrarRutasDirecciones(escenario.enrutador.Group("/api/v1"), manejador, usuarios, claveJWTDePrueba)
	return escenario
}

// enviar firma la petición con la cookie del usuario indicado; vacío = sin cookie.
func (escenario *escenarioDirecciones) enviar(prueba *testing.T, usuarioID, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	prueba.Helper()
	peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	peticion.Header.Set("Content-Type", "application/json")
	// Headers forjados: la identidad sólo puede salir del JWT.
	peticion.Header.Set("X-User-ID", escenario.segundo)
	if usuarioID != "" {
		token, errorObtenido := api.GenerarToken(usuarioID, domain.RolRegistrado, "local", claveJWTDePrueba)
		if errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		peticion.AddCookie(&http.Cookie{Name: "jwt", Value: token})
	}
	respuesta := httptest.NewRecorder()
	escenario.enrutador.ServeHTTP(respuesta, peticion)
	return respuesta
}

// crear guarda una dirección válida y devuelve su DTO; falla si no responde 201.
func (escenario *escenarioDirecciones) crear(prueba *testing.T, usuarioID, cuerpo string) domain.DireccionDTO {
	prueba.Helper()
	respuesta := escenario.enviar(prueba, usuarioID, http.MethodPost, rutaDirecciones, cuerpo)
	if respuesta.Code != http.StatusCreated {
		prueba.Fatalf("crear: %d %s", respuesta.Code, respuesta.Body)
	}
	var dto domain.DireccionDTO
	if errorObtenido := json.Unmarshal(respuesta.Body.Bytes(), &dto); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return dto
}

// listar devuelve las direcciones visibles para el usuario.
func (escenario *escenarioDirecciones) listar(prueba *testing.T, usuarioID string) []domain.DireccionDTO {
	prueba.Helper()
	respuesta := escenario.enviar(prueba, usuarioID, http.MethodGet, rutaDirecciones, "")
	var lista domain.ListaDireccionesDTO
	if respuesta.Code != http.StatusOK || json.Unmarshal(respuesta.Body.Bytes(), &lista) != nil {
		prueba.Fatalf("listar: %d %s", respuesta.Code, respuesta.Body)
	}
	return lista.Data
}

const (
	rutaDirecciones = "/api/v1/usuarios/me/direcciones"
	cuerpoValido    = `{"etiqueta":"Casa","texto_direccion":"Av. Alemania 0422","comuna":"Temuco","lat":-38.7359,"lon":-72.5904}`
)

// ===== Pruebas =====

// TestDireccionesExigenSesion comprueba que ningún método responde sin cookie válida.
func TestDireccionesExigenSesion(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	identificador := uuid.NewString()
	for _, caso := range []struct{ metodo, ruta string }{
		{http.MethodGet, rutaDirecciones}, {http.MethodPost, rutaDirecciones},
		{http.MethodPut, rutaDirecciones + "/" + identificador}, {http.MethodDelete, rutaDirecciones + "/" + identificador},
	} {
		prueba.Run(caso.metodo, func(prueba *testing.T) {
			if respuesta := escenario.enviar(prueba, "", caso.metodo, caso.ruta, cuerpoValido); respuesta.Code != http.StatusUnauthorized {
				prueba.Fatalf("sin cookie se esperaba 401 y fue %d", respuesta.Code)
			}
		})
	}
	// Una cuenta desactivada pierde el acceso aunque su JWT siga vigente.
	escenario.usuarios.buscarPorID(escenario.primero).EstaActivo = false
	if respuesta := escenario.enviar(prueba, escenario.primero, http.MethodGet, rutaDirecciones, ""); respuesta.Code != http.StatusUnauthorized {
		prueba.Fatalf("cuenta inactiva: %d", respuesta.Code)
	}
}

// TestDireccionesCicloCompleto recorre alta, listado, edición y borrado.
func TestDireccionesCicloCompleto(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	if lista := escenario.listar(prueba, escenario.primero); lista == nil || len(lista) != 0 {
		prueba.Fatalf("la lista vacía debe ser [] no null: %v", lista)
	}
	creada := escenario.crear(prueba, escenario.primero, cuerpoValido)
	if !creada.EsPrincipal || creada.Comuna != "Temuco" || creada.Lat == nil || *creada.Lat != -38.7359 {
		prueba.Fatalf("la primera dirección debe quedar principal y conservar sus datos: %+v", creada)
	}

	respuesta := escenario.enviar(prueba, escenario.primero, http.MethodPut, rutaDirecciones+"/"+creada.ID,
		`{"texto_direccion":"  Rudecindo Ortega 02950 ","comuna":"Temuco"}`)
	var editada domain.DireccionDTO
	if respuesta.Code != http.StatusOK || json.Unmarshal(respuesta.Body.Bytes(), &editada) != nil {
		prueba.Fatalf("actualizar: %d %s", respuesta.Code, respuesta.Body)
	}
	if editada.TextoDireccion != "Rudecindo Ortega 02950" || editada.Etiqueta != nil || editada.Lat != nil || editada.Lon != nil || !editada.EsPrincipal {
		prueba.Fatalf("PUT reemplaza los campos: omitir etiqueta y coordenadas las borra: %+v", editada)
	}
	if respuesta := escenario.enviar(prueba, escenario.primero, http.MethodDelete, rutaDirecciones+"/"+creada.ID, ""); respuesta.Code != http.StatusNoContent || respuesta.Body.Len() != 0 {
		prueba.Fatalf("borrar: %d %s", respuesta.Code, respuesta.Body)
	}
	if lista := escenario.listar(prueba, escenario.primero); len(lista) != 0 {
		prueba.Fatalf("la lista debe quedar vacía: %v", lista)
	}
}

// TestDireccionesAislamientoEntreUsuarios exige 404 (no 403) sobre filas ajenas
// y que ni siquiera un header forjado cambie de dueño.
func TestDireccionesAislamientoEntreUsuarios(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	ajena := escenario.crear(prueba, escenario.segundo, cuerpoValido)
	propia := escenario.crear(prueba, escenario.primero, cuerpoValido)

	if lista := escenario.listar(prueba, escenario.primero); len(lista) != 1 || lista[0].ID != propia.ID {
		prueba.Fatalf("el listado sólo puede incluir direcciones propias: %v", lista)
	}
	rutaAjena := rutaDirecciones + "/" + ajena.ID
	inexistente := rutaDirecciones + "/" + uuid.NewString()
	var cuerpoAjeno, cuerpoInexistente string
	for _, caso := range []struct{ metodo, cuerpo string }{{http.MethodPut, cuerpoValido}, {http.MethodDelete, ""}} {
		respuestaAjena := escenario.enviar(prueba, escenario.primero, caso.metodo, rutaAjena, caso.cuerpo)
		respuestaInexistente := escenario.enviar(prueba, escenario.primero, caso.metodo, inexistente, caso.cuerpo)
		if respuestaAjena.Code != http.StatusNotFound || respuestaInexistente.Code != http.StatusNotFound {
			prueba.Fatalf("%s: ajena=%d inexistente=%d; ambas deben ser 404", caso.metodo, respuestaAjena.Code, respuestaInexistente.Code)
		}
		cuerpoAjeno, cuerpoInexistente = respuestaAjena.Body.String(), respuestaInexistente.Body.String()
		if cuerpoAjeno != cuerpoInexistente {
			prueba.Fatalf("la respuesta no debe distinguir ajena de inexistente: %q vs %q", cuerpoAjeno, cuerpoInexistente)
		}
	}
	// La dirección ajena sigue intacta tras los intentos.
	if lista := escenario.listar(prueba, escenario.segundo); len(lista) != 1 || lista[0].TextoDireccion != "Av. Alemania 0422" {
		prueba.Fatalf("la dirección ajena fue alterada: %v", lista)
	}
	// El usuario no puede fijar el dueño desde el cuerpo.
	cuerpoForjado := `{"usuario_id":"` + escenario.segundo + `","texto_direccion":"x","comuna":"y"}`
	if respuesta := escenario.enviar(prueba, escenario.primero, http.MethodPost, rutaDirecciones, cuerpoForjado); respuesta.Code != http.StatusBadRequest {
		prueba.Fatalf("usuario_id en el cuerpo debe rechazarse: %d", respuesta.Code)
	}
}

// TestDireccionesValidaCampos cubre texto, comuna, etiqueta, coordenadas y JSON.
func TestDireccionesValidaCampos(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	casos := []struct{ nombre, cuerpo string }{
		{"texto vacío", `{"texto_direccion":"   ","comuna":"Temuco"}`},
		{"sin texto", `{"comuna":"Temuco"}`},
		{"texto demasiado largo", `{"texto_direccion":"` + strings.Repeat("a", 256) + `","comuna":"Temuco"}`},
		{"comuna vacía", `{"texto_direccion":"Calle 1","comuna":" "}`},
		{"sin comuna", `{"texto_direccion":"Calle 1"}`},
		{"comuna demasiado larga", `{"texto_direccion":"Calle 1","comuna":"` + strings.Repeat("c", 101) + `"}`},
		{"etiqueta demasiado larga", `{"texto_direccion":"Calle 1","comuna":"Temuco","etiqueta":"` + strings.Repeat("e", 101) + `"}`},
		{"carácter de control", `{"texto_direccion":"Calle\n1","comuna":"Temuco"}`},
		{"lat sin lon", `{"texto_direccion":"Calle 1","comuna":"Temuco","lat":-38.7}`},
		{"lon sin lat", `{"texto_direccion":"Calle 1","comuna":"Temuco","lon":-72.5}`},
		{"lat fuera de rango", `{"texto_direccion":"Calle 1","comuna":"Temuco","lat":90.5,"lon":0}`},
		{"lon fuera de rango", `{"texto_direccion":"Calle 1","comuna":"Temuco","lat":0,"lon":-180.1}`},
		{"lat como texto", `{"texto_direccion":"Calle 1","comuna":"Temuco","lat":"-38.7","lon":-72.5}`},
		{"campo desconocido", `{"texto_direccion":"Calle 1","comuna":"Temuco","id":"x"}`},
		{"cuerpo no JSON", `no es json`},
		{"dos objetos", `{"texto_direccion":"Calle 1","comuna":"Temuco"}{}`},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			if respuesta := escenario.enviar(prueba, escenario.primero, http.MethodPost, rutaDirecciones, caso.cuerpo); respuesta.Code != http.StatusBadRequest {
				prueba.Fatalf("se esperaba 400 y fue %d: %s", respuesta.Code, respuesta.Body)
			}
		})
	}
	if lista := escenario.listar(prueba, escenario.primero); len(lista) != 0 {
		prueba.Fatalf("una validación fallida no debe persistir nada: %v", lista)
	}

	prueba.Run("sin coordenadas, etiqueta vacía y límites exactos", func(prueba *testing.T) {
		cuerpo := `{"etiqueta":"  ","texto_direccion":"` + strings.Repeat("ñ", 255) + `","comuna":"` + strings.Repeat("c", 100) + `"}`
		dto := escenario.crear(prueba, escenario.primero, cuerpo)
		if dto.Lat != nil || dto.Lon != nil || dto.Etiqueta != nil {
			prueba.Fatalf("coordenadas y etiqueta vacía deben quedar en null: %+v", dto)
		}
	})
	prueba.Run("coordenadas en los extremos y redondeo", func(prueba *testing.T) {
		dto := escenario.crear(prueba, escenario.segundo, `{"texto_direccion":"Polo","comuna":"X","lat":-90,"lon":179.999999996}`)
		if dto.Lat == nil || *dto.Lat != -90 || dto.Lon == nil || *dto.Lon != 180 {
			prueba.Fatalf("extremos válidos y redondeo a 8 decimales: %+v", dto)
		}
	})
	prueba.Run("identificador de ruta inválido", func(prueba *testing.T) {
		// uuid.Parse acepta urn:uuid:… y llaves, pero PostgreSQL no: deben dar 400.
		identificador := uuid.NewString()
		invalidos := []string{"no-es-uuid", "urn:uuid:" + identificador, "%7B" + identificador + "%7D"}
		for _, metodo := range []string{http.MethodPut, http.MethodDelete} {
			for _, invalido := range invalidos {
				if respuesta := escenario.enviar(prueba, escenario.primero, metodo, rutaDirecciones+"/"+invalido, cuerpoValido); respuesta.Code != http.StatusBadRequest {
					prueba.Fatalf("%s con id %q: %d", metodo, invalido, respuesta.Code)
				}
			}
		}
	})
}

// TestDireccionesLimitePorUsuario comprueba el tope y que otro usuario no lo comparte.
func TestDireccionesLimitePorUsuario(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	for indice := 0; indice < domain.MaximoDireccionesPorUsuario; indice++ {
		escenario.crear(prueba, escenario.primero, cuerpoValido)
	}
	respuesta := escenario.enviar(prueba, escenario.primero, http.MethodPost, rutaDirecciones, cuerpoValido)
	if respuesta.Code != http.StatusConflict || !strings.Contains(respuesta.Body.String(), "máximo") {
		prueba.Fatalf("la dirección %d debe responder 409 con mensaje claro: %d %s", domain.MaximoDireccionesPorUsuario+1, respuesta.Code, respuesta.Body)
	}
	escenario.crear(prueba, escenario.segundo, cuerpoValido)
}

// TestDireccionesPrincipal cubre una sola principal, promoción manual y borrado.
func TestDireccionesPrincipal(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	primera := escenario.crear(prueba, escenario.primero, `{"texto_direccion":"A","comuna":"X","es_principal":false}`)
	if !primera.EsPrincipal {
		prueba.Fatal("la primera dirección es principal aunque se pida false")
	}
	segunda := escenario.crear(prueba, escenario.primero, `{"texto_direccion":"B","comuna":"X"}`)
	tercera := escenario.crear(prueba, escenario.primero, `{"texto_direccion":"C","comuna":"X","es_principal":true}`)
	if segunda.EsPrincipal || !tercera.EsPrincipal {
		prueba.Fatalf("marcar una nueva como principal desmarca la anterior: %+v %+v", segunda, tercera)
	}
	contarPrincipales := func() (cantidad int, principal string) {
		for _, dto := range escenario.listar(prueba, escenario.primero) {
			if dto.EsPrincipal {
				cantidad, principal = cantidad+1, dto.TextoDireccion
			}
		}
		return cantidad, principal
	}
	if cantidad, principal := contarPrincipales(); cantidad != 1 || principal != "C" {
		prueba.Fatalf("debe haber exactamente una principal (C): %d %s", cantidad, principal)
	}
	if lista := escenario.listar(prueba, escenario.primero); lista[0].TextoDireccion != "C" {
		prueba.Fatalf("la principal va primero: %v", lista)
	}

	// Promoción con PUT y rechazo de desmarcar la principal.
	respuesta := escenario.enviar(prueba, escenario.primero, http.MethodPut, rutaDirecciones+"/"+segunda.ID, `{"texto_direccion":"B","comuna":"X","es_principal":true}`)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("promover: %d %s", respuesta.Code, respuesta.Body)
	}
	if cantidad, principal := contarPrincipales(); cantidad != 1 || principal != "B" {
		prueba.Fatalf("la promovida debe ser la única principal: %d %s", cantidad, principal)
	}
	respuesta = escenario.enviar(prueba, escenario.primero, http.MethodPut, rutaDirecciones+"/"+segunda.ID, `{"texto_direccion":"B","comuna":"X","es_principal":false}`)
	if respuesta.Code != http.StatusConflict {
		prueba.Fatalf("desmarcar la principal debe ser 409: %d", respuesta.Code)
	}

	// Al borrar la principal se promueve la más antigua de las restantes (A).
	if respuesta := escenario.enviar(prueba, escenario.primero, http.MethodDelete, rutaDirecciones+"/"+segunda.ID, ""); respuesta.Code != http.StatusNoContent {
		prueba.Fatalf("borrar principal: %d", respuesta.Code)
	}
	if cantidad, principal := contarPrincipales(); cantidad != 1 || principal != "A" {
		prueba.Fatalf("debe promoverse la más antigua (A): %d %s", cantidad, principal)
	}
}

// TestDireccionesErrorInternoNoExponeDetalle comprueba que un fallo del
// repositorio responde 500 genérico sin filtrar el mensaje original.
func TestDireccionesErrorInternoNoExponeDetalle(prueba *testing.T) {
	escenario := nuevoEscenarioDirecciones(prueba)
	escenario.direcciones.fallos = gorm.ErrInvalidDB
	respuesta := escenario.enviar(prueba, escenario.primero, http.MethodGet, rutaDirecciones, "")
	if respuesta.Code != http.StatusInternalServerError || strings.Contains(respuesta.Body.String(), gorm.ErrInvalidDB.Error()) {
		prueba.Fatalf("500 genérico esperado: %d %s", respuesta.Code, respuesta.Body)
	}
}
