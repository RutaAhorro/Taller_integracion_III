// Direcciones de usuario contra PostgreSQL real y desechable, con el rol
// runtime app_identity: CRUD, una sola principal (incluida la carrera), límite,
// restricciones de la migración 11, permisos y aislamiento entre cuentas a
// través del router completo. Se omite sin IDENTIDAD_TEST_DB_URL (app_identity)
// e IDENTIDAD_TEST_ADMIN_URL (operador, sólo para preparar y limpiar).

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Preparación =====

const (
	// concurrenciaPrueba es la cantidad de operaciones simultáneas por caso; se
	// mantiene bajo MaximoDireccionesPorUsuario para no mezclar con el límite.
	concurrenciaPrueba = 8
	// claveJWTIntegracion firma las sesiones del router completo en estas pruebas.
	claveJWTIntegracion = "sup239-integration-only-key-at-least-32-bytes"
)

// nuevaDireccion arma una dirección válida con un texto distinguible.
func nuevaDireccion(texto string, esPrincipal bool) *domain.DireccionUsuario {
	return &domain.DireccionUsuario{TextoDireccion: texto, Comuna: "Temuco", EsPrincipal: esPrincipal}
}

// contarPrincipales devuelve cuántas principales tiene el usuario según el operador.
func contarPrincipales(prueba *testing.T, operador *gorm.DB, usuarioID uuid.UUID) int64 {
	prueba.Helper()
	var principales int64
	if errorObtenido := operador.Raw("SELECT count(*) FROM api.direcciones_usuario WHERE usuario_id = ? AND es_principal", usuarioID).Scan(&principales).Error; errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return principales
}

// ===== Repositorio =====

// TestDireccionesRepositorioPostgres ejercita las reglas del repositorio SQL.
func TestDireccionesRepositorioPostgres(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	if errorObtenido := repositories.VerificarAlmacenamiento(contexto, runtime); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prefijo := "dir-" + uuid.NewString()[:8]
	dueno := crearCuenta(prueba, runtime, operador, prefijo, "uno", domain.RolRegistrado)
	otro := crearCuenta(prueba, runtime, operador, prefijo, "dos", domain.RolRegistrado)
	repositorio := repositories.NuevoRepositorioDirecciones(runtime)
	duenoID, otroID := dueno.ID.String(), otro.ID.String()

	prueba.Run("la primera queda principal y el orden es determinista", func(prueba *testing.T) {
		primera, segunda, tercera := nuevaDireccion("A", false), nuevaDireccion("B", false), nuevaDireccion("C", false)
		for _, direccion := range []*domain.DireccionUsuario{primera, segunda, tercera} {
			if errorObtenido := repositorio.Crear(contexto, duenoID, direccion, domain.MaximoDireccionesPorUsuario); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
		}
		if !primera.EsPrincipal || segunda.EsPrincipal || tercera.EsPrincipal || primera.ID == uuid.Nil || primera.CreadoEl.IsZero() {
			prueba.Fatalf("la primera es principal y recibe id y fecha: %+v", primera)
		}
		lista, errorObtenido := repositorio.Listar(contexto, duenoID)
		if errorObtenido != nil || len(lista) != 3 || lista[0].TextoDireccion != "A" || lista[1].TextoDireccion != "B" || lista[2].TextoDireccion != "C" {
			prueba.Fatalf("orden principal y luego antigüedad: %+v %v", lista, errorObtenido)
		}
	})

	prueba.Run("marcar principal desmarca la anterior en la misma operación", func(prueba *testing.T) {
		nueva := nuevaDireccion("D", true)
		if errorObtenido := repositorio.Crear(contexto, duenoID, nueva, domain.MaximoDireccionesPorUsuario); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		if principales := contarPrincipales(prueba, operador, dueno.ID); principales != 1 {
			prueba.Fatalf("debe haber una principal, hay %d", principales)
		}
		lista, _ := repositorio.Listar(contexto, duenoID)
		if lista[0].ID != nueva.ID {
			prueba.Fatalf("la nueva principal va primero: %+v", lista[0])
		}
	})

	prueba.Run("actualizar reemplaza campos, escribe NULL y promueve", func(prueba *testing.T) {
		latitud, longitud, etiqueta := -38.7359, -72.5904, "Casa"
		lista, _ := repositorio.Listar(contexto, duenoID)
		objetivo := lista[len(lista)-1]
		verdadero := true
		actualizada, errorObtenido := repositorio.Actualizar(contexto, duenoID, objetivo.ID.String(),
			domain.DireccionUsuario{Etiqueta: &etiqueta, TextoDireccion: "Nueva", Comuna: "Padre Las Casas", Lat: &latitud, Lon: &longitud}, &verdadero)
		if errorObtenido != nil || !actualizada.EsPrincipal || actualizada.Lat == nil || *actualizada.Lat != latitud || *actualizada.Lon != longitud || actualizada.Comuna != "Padre Las Casas" {
			prueba.Fatalf("actualización: %+v %v", actualizada, errorObtenido)
		}
		if principales := contarPrincipales(prueba, operador, dueno.ID); principales != 1 {
			prueba.Fatalf("tras promover debe haber una principal, hay %d", principales)
		}
		sinCoordenadas, errorObtenido := repositorio.Actualizar(contexto, duenoID, objetivo.ID.String(), domain.DireccionUsuario{TextoDireccion: "Nueva", Comuna: "X"}, nil)
		if errorObtenido != nil || sinCoordenadas.Lat != nil || sinCoordenadas.Lon != nil || sinCoordenadas.Etiqueta != nil || !sinCoordenadas.EsPrincipal {
			prueba.Fatalf("omitir coordenadas las borra y es_principal nil conserva: %+v %v", sinCoordenadas, errorObtenido)
		}
		falso := false
		if _, errorObtenido := repositorio.Actualizar(contexto, duenoID, objetivo.ID.String(), domain.DireccionUsuario{TextoDireccion: "Nueva", Comuna: "X"}, &falso); !errors.Is(errorObtenido, domain.ErrPrincipalObligatoria) {
			prueba.Fatalf("desmarcar la principal: %v", errorObtenido)
		}
	})

	prueba.Run("borrar la principal promueve a la más antigua", func(prueba *testing.T) {
		lista, _ := repositorio.Listar(contexto, duenoID)
		masAntigua := lista[1]
		for _, candidata := range lista[1:] {
			if candidata.CreadoEl.Before(masAntigua.CreadoEl) || (candidata.CreadoEl.Equal(masAntigua.CreadoEl) && candidata.ID.String() < masAntigua.ID.String()) {
				masAntigua = candidata
			}
		}
		if errorObtenido := repositorio.Eliminar(contexto, duenoID, lista[0].ID.String()); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		restantes, _ := repositorio.Listar(contexto, duenoID)
		if len(restantes) != 3 || !restantes[0].EsPrincipal || restantes[0].ID != masAntigua.ID {
			prueba.Fatalf("debía promoverse %s: %+v", masAntigua.ID, restantes)
		}
		if principales := contarPrincipales(prueba, operador, dueno.ID); principales != 1 {
			prueba.Fatalf("debe haber una principal, hay %d", principales)
		}
	})

	prueba.Run("aislamiento: otra cuenta no ve, edita ni borra", func(prueba *testing.T) {
		lista, _ := repositorio.Listar(contexto, duenoID)
		ajena := lista[0].ID.String()
		if propias, errorObtenido := repositorio.Listar(contexto, otroID); errorObtenido != nil || len(propias) != 0 {
			prueba.Fatalf("el otro usuario no debe ver filas ajenas: %v %v", propias, errorObtenido)
		}
		if _, errorObtenido := repositorio.Actualizar(contexto, otroID, ajena, domain.DireccionUsuario{TextoDireccion: "hack", Comuna: "X"}, nil); !errors.Is(errorObtenido, domain.ErrDireccionNoEncontrada) {
			prueba.Fatalf("actualizar ajena: %v", errorObtenido)
		}
		if errorObtenido := repositorio.Eliminar(contexto, otroID, ajena); !errors.Is(errorObtenido, domain.ErrDireccionNoEncontrada) {
			prueba.Fatalf("borrar ajena: %v", errorObtenido)
		}
		if errorObtenido := repositorio.Eliminar(contexto, duenoID, uuid.NewString()); !errors.Is(errorObtenido, domain.ErrDireccionNoEncontrada) {
			prueba.Fatalf("borrar inexistente: %v", errorObtenido)
		}
		despues, _ := repositorio.Listar(contexto, duenoID)
		if len(despues) != len(lista) {
			prueba.Fatal("los intentos ajenos no deben alterar las filas")
		}
	})

	prueba.Run("el límite se respeta", func(prueba *testing.T) {
		llenable := crearCuenta(prueba, runtime, operador, prefijo, "lleno", domain.RolRegistrado)
		for indice := 0; indice < 3; indice++ {
			if errorObtenido := repositorio.Crear(contexto, llenable.ID.String(), nuevaDireccion("L", false), 3); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
		}
		if errorObtenido := repositorio.Crear(contexto, llenable.ID.String(), nuevaDireccion("L", false), 3); !errors.Is(errorObtenido, domain.ErrLimiteDirecciones) {
			prueba.Fatalf("se esperaba ErrLimiteDirecciones: %v", errorObtenido)
		}
	})
}

// ===== Concurrencia =====

// TestDireccionesConcurrenciaPostgres comprueba que las carreras no producen
// dos principales ni superan el límite, sin recurrir a errores 500.
func TestDireccionesConcurrenciaPostgres(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	contexto := context.Background()
	repositorio := repositories.NuevoRepositorioDirecciones(runtime)
	prefijo := "conc-" + uuid.NewString()[:8]

	prueba.Run("altas simultáneas todas marcadas principal", func(prueba *testing.T) {
		cuenta := crearCuenta(prueba, runtime, operador, prefijo, "altas", domain.RolRegistrado)
		errores := make(chan error, concurrenciaPrueba)
		var espera sync.WaitGroup
		for indice := 0; indice < concurrenciaPrueba; indice++ {
			espera.Add(1)
			go func() {
				defer espera.Done()
				errores <- repositorio.Crear(contexto, cuenta.ID.String(), nuevaDireccion("P", true), domain.MaximoDireccionesPorUsuario)
			}()
		}
		espera.Wait()
		close(errores)
		for errorObtenido := range errores {
			if errorObtenido != nil {
				prueba.Fatalf("ninguna alta concurrente debe fallar: %v", errorObtenido)
			}
		}
		if principales := contarPrincipales(prueba, operador, cuenta.ID); principales != 1 {
			prueba.Fatalf("debe haber exactamente una principal, hay %d", principales)
		}
	})

	prueba.Run("promociones simultáneas de filas distintas", func(prueba *testing.T) {
		cuenta := crearCuenta(prueba, runtime, operador, prefijo, "promo", domain.RolRegistrado)
		var identificadores []string
		for indice := 0; indice < concurrenciaPrueba; indice++ {
			direccion := nuevaDireccion("Q", false)
			if errorObtenido := repositorio.Crear(contexto, cuenta.ID.String(), direccion, domain.MaximoDireccionesPorUsuario); errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			identificadores = append(identificadores, direccion.ID.String())
		}
		verdadero := true
		errores := make(chan error, len(identificadores))
		var espera sync.WaitGroup
		for _, identificador := range identificadores {
			espera.Add(1)
			go func() {
				defer espera.Done()
				_, errorObtenido := repositorio.Actualizar(contexto, cuenta.ID.String(), identificador, domain.DireccionUsuario{TextoDireccion: "Q", Comuna: "X"}, &verdadero)
				errores <- errorObtenido
			}()
		}
		espera.Wait()
		close(errores)
		for errorObtenido := range errores {
			if errorObtenido != nil {
				prueba.Fatalf("ninguna promoción concurrente debe fallar: %v", errorObtenido)
			}
		}
		if principales := contarPrincipales(prueba, operador, cuenta.ID); principales != 1 {
			prueba.Fatalf("debe haber exactamente una principal, hay %d", principales)
		}
	})

	prueba.Run("el límite resiste altas simultáneas", func(prueba *testing.T) {
		cuenta := crearCuenta(prueba, runtime, operador, prefijo, "limite", domain.RolRegistrado)
		const intentos, limite = 12, 5
		resultados := make(chan error, intentos)
		var espera sync.WaitGroup
		for indice := 0; indice < intentos; indice++ {
			espera.Add(1)
			go func() {
				defer espera.Done()
				resultados <- repositorio.Crear(contexto, cuenta.ID.String(), nuevaDireccion("R", false), limite)
			}()
		}
		espera.Wait()
		close(resultados)
		aceptadas, rechazadas := 0, 0
		for errorObtenido := range resultados {
			switch {
			case errorObtenido == nil:
				aceptadas++
			case errors.Is(errorObtenido, domain.ErrLimiteDirecciones):
				rechazadas++
			default:
				prueba.Fatalf("error inesperado: %v", errorObtenido)
			}
		}
		if aceptadas != limite || rechazadas != intentos-limite {
			prueba.Fatalf("aceptadas=%d rechazadas=%d; se esperaban %d y %d", aceptadas, rechazadas, limite, intentos-limite)
		}
	})
}

// ===== Restricciones y permisos =====

// TestDireccionesRestriccionesYPermisosPostgres comprueba que la base misma
// rechaza datos inválidos y que app_identity tiene sólo los permisos previstos.
func TestDireccionesRestriccionesYPermisosPostgres(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	cuenta := crearCuenta(prueba, runtime, operador, "res-"+uuid.NewString()[:8], "uno", domain.RolRegistrado)

	insertar := func(sqlFila string, parametros ...any) error {
		return runtime.Exec("INSERT INTO api.direcciones_usuario (usuario_id, texto_direccion, comuna, lat, lon, es_principal) VALUES "+sqlFila, parametros...).Error
	}
	prueba.Run("la base rechaza datos inválidos", func(prueba *testing.T) {
		casos := map[string]error{
			"lat sin lon":         insertar("(?, 'x', 'y', 10, NULL, false)", cuenta.ID),
			"lat fuera de rango":  insertar("(?, 'x', 'y', 91, 0, false)", cuenta.ID),
			"lon fuera de rango":  insertar("(?, 'x', 'y', 0, -181, false)", cuenta.ID),
			"comuna en blanco":    insertar("(?, 'x', '   ', 0, 0, false)", cuenta.ID),
			"es_principal NULL":   insertar("(?, 'x', 'y', 0, 0, NULL)", cuenta.ID),
			"usuario inexistente": insertar("(?, 'x', 'y', 0, 0, false)", uuid.New()),
		}
		for nombre, errorObtenido := range casos {
			if errorObtenido == nil {
				prueba.Errorf("%s: la base lo aceptó", nombre)
			}
		}
		if errorObtenido := insertar("(?, 'x', 'y', NULL, NULL, false)", cuenta.ID); errorObtenido != nil {
			prueba.Fatalf("sin coordenadas debe ser válido: %v", errorObtenido)
		}
	})
	prueba.Run("el índice único parcial impide dos principales", func(prueba *testing.T) {
		if errorObtenido := insertar("(?, 'a', 'y', 0, 0, true)", cuenta.ID); errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		if errorObtenido := insertar("(?, 'b', 'y', 0, 0, true)", cuenta.ID); errorObtenido == nil || !strings.Contains(errorObtenido.Error(), "uq_direcciones_usuario_principal") {
			prueba.Fatalf("se esperaba violación del índice de principal: %v", errorObtenido)
		}
	})
	prueba.Run("permisos exactos de app_identity", func(prueba *testing.T) {
		permisos := map[string]bool{}
		for _, permiso := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var concedido bool
			if errorObtenido := runtime.Raw("SELECT has_table_privilege(current_user, 'api.direcciones_usuario', ?)", permiso).Scan(&concedido).Error; errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			permisos[permiso] = concedido
		}
		esperados := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}
		for permiso, concedido := range permisos {
			if concedido != esperados[permiso] {
				prueba.Errorf("%s = %v; se esperaba %v", permiso, concedido, esperados[permiso])
			}
		}
	})
}

// ===== HTTP completo =====

// TestDireccionesHTTPAislamientoPostgres usa el router real con la base real:
// dos cuentas, sesión por cookie y respuesta 404 para filas ajenas.
func TestDireccionesHTTPAislamientoPostgres(prueba *testing.T) {
	runtime, operador := basesDePrueba(prueba)
	prefijo := "http-" + uuid.NewString()[:8]
	primero := crearCuenta(prueba, runtime, operador, prefijo, "uno", domain.RolRegistrado)
	segundo := crearCuenta(prueba, runtime, operador, prefijo, "dos", domain.RolRegistrado)
	configuracion := config.Configuracion{ClaveJWT: claveJWTIntegracion, Ajustes: config.AjustesPredeterminados()}
	enrutador := api.NuevoEnrutador(configuracion, api.NuevoModulo(runtime, nil, configuracion), nil, nil, nil)

	enviar := func(cuenta *domain.Usuario, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
		prueba.Helper()
		peticion := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
		peticion.Header.Set("Content-Type", "application/json")
		token, errorObtenido := api.GenerarToken(cuenta.ID.String(), cuenta.Rol, "local", claveJWTIntegracion)
		if errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
		peticion.AddCookie(&http.Cookie{Name: "jwt", Value: token})
		respuesta := httptest.NewRecorder()
		enrutador.ServeHTTP(respuesta, peticion)
		return respuesta
	}
	const ruta = "/api/v1/usuarios/me/direcciones"

	creada := enviar(primero, http.MethodPost, ruta, `{"etiqueta":"Casa","texto_direccion":"Av. Alemania 0422","comuna":"Temuco","lat":-38.7359,"lon":-72.5904}`)
	var dto domain.DireccionDTO
	if creada.Code != http.StatusCreated || json.Unmarshal(creada.Body.Bytes(), &dto) != nil || !dto.EsPrincipal || dto.Lat == nil || *dto.Lat != -38.7359 {
		prueba.Fatalf("crear: %d %s", creada.Code, creada.Body)
	}
	if respuesta := enviar(segundo, http.MethodGet, ruta, ""); respuesta.Code != http.StatusOK || !strings.Contains(respuesta.Body.String(), `"data":[]`) {
		prueba.Fatalf("el segundo usuario no ve nada: %d %s", respuesta.Code, respuesta.Body)
	}
	for _, caso := range []struct{ metodo, cuerpo string }{
		{http.MethodPut, `{"texto_direccion":"hack","comuna":"X"}`}, {http.MethodDelete, ""},
	} {
		if respuesta := enviar(segundo, caso.metodo, ruta+"/"+dto.ID, caso.cuerpo); respuesta.Code != http.StatusNotFound {
			prueba.Fatalf("%s sobre dirección ajena debe ser 404 y fue %d", caso.metodo, respuesta.Code)
		}
	}
	if respuesta := enviar(primero, http.MethodGet, ruta, ""); !strings.Contains(respuesta.Body.String(), "Av. Alemania 0422") {
		prueba.Fatalf("la dirección propia debe seguir intacta: %s", respuesta.Body)
	}
	if respuesta := enviar(primero, http.MethodDelete, ruta+"/"+dto.ID, ""); respuesta.Code != http.StatusNoContent {
		prueba.Fatalf("borrar propia: %d", respuesta.Code)
	}
}
