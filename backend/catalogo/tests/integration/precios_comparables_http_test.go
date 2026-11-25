package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// repositorioPreciosHTTP es el doble geoespacial de la prueba del adaptador
// HTTP. Cuenta las llamadas para comprobar que un rechazo no llega a la base.
type repositorioPreciosHTTP struct {
	candidatas     []domain.SucursalCercanaRegistro
	sucursales     []domain.SucursalRegistro
	llamadas       int
	radioRecibido  float64
	maximoRecibido int
}

func (repositorio *repositorioPreciosHTTP) ConsultarCercanas(_ context.Context, _, _, radioKm float64, maximo int) ([]domain.SucursalCercanaRegistro, error) {
	repositorio.llamadas++
	repositorio.radioRecibido = radioKm
	repositorio.maximoRecibido = maximo
	return repositorio.candidatas, nil
}

func (repositorio *repositorioPreciosHTTP) ConsultarSucursales(_ context.Context) ([]domain.SucursalRegistro, error) {
	return repositorio.sucursales, nil
}

// nuevoRouterPreciosHTTP arma sólo la ruta interna de SUP-317 sobre el router
// real, con el token de sólo lectura.
func nuevoRouterPreciosHTTP(repositorioPrecios *repositorioPreciosHTTP, repositorioEquivalencias *repositorioEquivalenciasHTTP, token string) *gin.Engine {
	momento := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	equivalencias := services.NuevoServicioEquivalencias(repositorioEquivalencias,
		services.LimitesEquivalencia{MaximoProductos: 100, AntiguedadMaxima: 48 * time.Hour},
		func() time.Time { return momento })
	servicio := services.NuevoServicioPreciosComparables(repositorioPrecios, equivalencias,
		services.LimitesPreciosComparables{MaximoProductos: 100, RadioMaximoKm: 50})
	return nuevoEnrutadorDePrueba(api.Dependencias{PreciosComparables: servicio, TokenConsulta: token})
}

// TestPreciosComparablesHTTPExigeTokenDeServicio confirma que la consulta no
// expone precios sin el token de lectura ni a un navegador.
func TestPreciosComparablesHTTPExigeTokenDeServicio(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	cuerpo := `{"producto_ids":["00000000-0000-4000-8000-000000000001"],"lat":-38.739,"lon":-72.598,"radio_km":10,"maximo_sucursales":5}`
	for _, caso := range []struct {
		nombre, autorizacion, origen string
		estado                       int
	}{
		{"sin token", "", "", http.StatusUnauthorized},
		{"token de ingesta", "Bearer " + strings.Repeat("i", 32), "", http.StatusUnauthorized},
		{"navegador rechazado", "Bearer " + token, "https://frontend.example", http.StatusForbidden},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorioPrecios := &repositorioPreciosHTTP{}
			enrutador := nuevoRouterPreciosHTTP(repositorioPrecios, &repositorioEquivalenciasHTTP{}, token)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/precios-comparables", strings.NewReader(cuerpo))
			peticion.Header.Set("Authorization", caso.autorizacion)
			peticion.Header.Set("Origin", caso.origen)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.estado || repositorioPrecios.llamadas != 0 {
				subprueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, repositorioPrecios.llamadas, respuesta.Body.String())
			}
		})
	}
}

// TestPreciosComparablesHTTPValidaCuerpo cubre el JSON estricto y los límites
// del lote, origen, radio y tope de sucursales.
func TestPreciosComparablesHTTPValidaCuerpo(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	identificador := "00000000-0000-4000-8000-000000000001"
	for _, caso := range []struct {
		nombre, cuerpo, errorCodigo string
	}{
		{"JSON inválido", `{`, "invalid_payload"},
		{"campo desconocido", `{"producto_ids":["` + identificador + `"],"lat":-38.7,"lon":-72.5,"radio_km":10,"maximo_sucursales":5,"extra":1}`, "invalid_payload"},
		{"contenido extra", `{"producto_ids":["` + identificador + `"],"lat":-38.7,"lon":-72.5,"radio_km":10,"maximo_sucursales":5} {}`, "invalid_payload"},
		{"lote vacío", `{"producto_ids":[],"lat":-38.7,"lon":-72.5,"radio_km":10,"maximo_sucursales":5}`, "invalid_batch"},
		{"UUID inválido", `{"producto_ids":["no-uuid"],"lat":-38.7,"lon":-72.5,"radio_km":10,"maximo_sucursales":5}`, "invalid_product_id"},
		{"sin coordenadas", `{"producto_ids":["` + identificador + `"],"radio_km":10,"maximo_sucursales":5}`, "invalid_consulta"},
		{"radio excesivo", `{"producto_ids":["` + identificador + `"],"lat":-38.7,"lon":-72.5,"radio_km":51,"maximo_sucursales":5}`, "invalid_consulta"},
		{"tope de sucursales", `{"producto_ids":["` + identificador + `"],"lat":-38.7,"lon":-72.5,"radio_km":10,"maximo_sucursales":11}`, "invalid_consulta"},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorioPrecios := &repositorioPreciosHTTP{}
			enrutador := nuevoRouterPreciosHTTP(repositorioPrecios, &repositorioEquivalenciasHTTP{}, token)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/precios-comparables", strings.NewReader(caso.cuerpo))
			peticion.Header.Set("Authorization", "Bearer "+token)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			var cuerpo struct {
				Error string `json:"error"`
			}
			if respuesta.Code != http.StatusBadRequest {
				subprueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
			}
			if errorJSON := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorJSON != nil || cuerpo.Error != caso.errorCodigo {
				subprueba.Fatalf("error=%q (%v), esperado %q", cuerpo.Error, errorJSON, caso.errorCodigo)
			}
			if repositorioPrecios.llamadas != 0 {
				subprueba.Fatalf("la solicitud inválida consultó la base %d veces", repositorioPrecios.llamadas)
			}
		})
	}
}

// TestPreciosComparablesHTTPRespondeCandidatasYFaltantes verifica la forma de
// la respuesta que consume el motor: sucursales, ofertas y ausencias.
func TestPreciosComparablesHTTPRespondeCandidatasYFaltantes(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	momento := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	identificador := "00000000-0000-4000-8000-000000000001"

	registro := registroEquivalenciaBaseHTTP(identificador, 1, "Jumbo")
	registro.CodigoEAN = referenciaCadena("0036000291452")
	registro.PrecioNormal = punteroDecimalHTTP(1290)
	registro.EnStock = punteroBooleanoHTTP(true)
	registro.CapturadoEl = &momento

	repositorioPrecios := &repositorioPreciosHTTP{
		candidatas: []domain.SucursalCercanaRegistro{{ID: 1, Nombre: "Jumbo Los Pablos", Cadena: "Jumbo", Lat: -38.741, Lon: -72.6, DistanciaKm: 0.28}},
		sucursales: []domain.SucursalRegistro{
			{ID: 1, Nombre: "Jumbo Los Pablos", Cadena: "Jumbo", Activa: true, Lat: punteroDecimalHTTP(-38.741), Lon: punteroDecimalHTTP(-72.6)},
			{ID: 2, Nombre: "Sin coordenadas", Cadena: "Cugat", Activa: true},
		},
	}
	repositorioEquivalencias := &repositorioEquivalenciasHTTP{registros: []domain.ProductoEquivalenciaRegistro{registro}}
	enrutador := nuevoRouterPreciosHTTP(repositorioPrecios, repositorioEquivalencias, token)

	cuerpo := `{"producto_ids":["` + identificador + `"],"lat":-38.739,"lon":-72.598,"radio_km":10,"maximo_sucursales":5}`
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/precios-comparables", strings.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)

	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
	var contenido domain.RespuestaPreciosComparablesDTO
	if errorJSON := json.Unmarshal(respuesta.Body.Bytes(), &contenido); errorJSON != nil {
		prueba.Fatal(errorJSON)
	}
	if repositorioPrecios.radioRecibido != 10 || repositorioPrecios.maximoRecibido != 5 {
		prueba.Fatalf("radio o tope incorrectos: %+v", repositorioPrecios)
	}
	if len(contenido.Sucursales) != 1 || contenido.Sucursales[0].SucursalID != 1 {
		prueba.Fatalf("sucursales incorrectas: %+v", contenido.Sucursales)
	}
	if len(contenido.Productos) != 1 || len(contenido.Productos[0].Ofertas) != 1 {
		prueba.Fatalf("ofertas incorrectas: %+v", contenido.Productos)
	}
	if len(contenido.SinOfertas) != 0 {
		prueba.Fatalf("no debía haber faltantes: %v", contenido.SinOfertas)
	}
	if len(contenido.SucursalesExcluidas) != 1 || contenido.SucursalesExcluidas[0].Motivo != domain.MotivoSucursalSinCoordenadas {
		prueba.Fatalf("exclusiones incorrectas: %+v", contenido.SucursalesExcluidas)
	}
	if contenido.Resumen.ConOferta != 1 || contenido.Resumen.OfertasSinPrecio != 0 {
		prueba.Fatalf("resumen incorrecto: %+v", contenido.Resumen)
	}
}

// registroEquivalenciaBaseHTTP y los punteros locales evitan depender de los
// dobles del paquete de pruebas unitarias.
func registroEquivalenciaBaseHTTP(productoID string, sucursalID int, cadena string) domain.ProductoEquivalenciaRegistro {
	return domain.ProductoEquivalenciaRegistro{
		ProductoID: productoID,
		SucursalID: sucursalID,
		Cadena:     cadena,
		Sucursal:   cadena + " Centro",
		Nombre:     "Producto de prueba",
		Formato:    "1 L",
	}
}

func punteroDecimalHTTP(valor float64) *float64 { return &valor }

func punteroBooleanoHTTP(valor bool) *bool { return &valor }
