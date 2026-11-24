package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// repositorioEquivalenciasHTTP es el doble de persistencia para las pruebas del
// adaptador HTTP. Registra llamadas para comprobar que un lote inválido no
// llega a la base.
type repositorioEquivalenciasHTTP struct {
	registros     []domain.ProductoEquivalenciaRegistro
	errorConsulta error
	llamadas      int
	idsRecibidos  []string
}

func (repositorio *repositorioEquivalenciasHTTP) ConsultarPorIDs(_ context.Context, productoIDs []string) ([]domain.ProductoEquivalenciaRegistro, error) {
	repositorio.llamadas++
	repositorio.idsRecibidos = append([]string(nil), productoIDs...)
	if repositorio.errorConsulta != nil {
		return nil, repositorio.errorConsulta
	}
	return repositorio.registros, nil
}

// referenciaCadena devuelve un puntero a una copia del texto recibido.
func referenciaCadena(valor string) *string {
	return &valor
}

// nuevoRouterEquivalenciasHTTP arma sólo la ruta interna para aislar la prueba
// de las rutas públicas y del Gateway.
func nuevoRouterEquivalenciasHTTP(repositorio *repositorioEquivalenciasHTTP, token string, maxProductos int, ahora time.Time) *gin.Engine {
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: maxProductos, AntiguedadMaxima: 48 * time.Hour},
		func() time.Time { return ahora })
	return nuevoEnrutadorDePrueba(api.Dependencias{Equivalencias: servicio, TokenConsulta: token})
}

// TestEquivalenciaHTTPExigeTokenDeServicio cubre los rechazos de seguridad
// antes de leer o consultar datos.
func TestEquivalenciaHTTPExigeTokenDeServicio(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	cuerpo := `{"producto_ids":["00000000-0000-4000-8000-000000000001"]}`
	for _, caso := range []struct {
		nombre, autorizacion, origen string
		estado, llamadas             int
	}{
		{"sin token", "", "", http.StatusUnauthorized, 0},
		{"token incorrecto", "Bearer user-jwt", "", http.StatusUnauthorized, 0},
		{"navegador rechazado", "Bearer " + token, "https://frontend.example", http.StatusForbidden, 0},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &repositorioEquivalenciasHTTP{}
			enrutador := nuevoRouterEquivalenciasHTTP(repositorio, token, 1000, momento)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", strings.NewReader(cuerpo))
			peticion.Header.Set("Authorization", caso.autorizacion)
			peticion.Header.Set("Origin", caso.origen)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.estado || repositorio.llamadas != caso.llamadas {
				subprueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, repositorio.llamadas, respuesta.Body.String())
			}
		})
	}
}

// TestEquivalenciaHTTPValidaCuerpo comprueba el contrato estricto del cuerpo y
// el tope configurable de IDs.
func TestEquivalenciaHTTPValidaCuerpo(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("t", 32)
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	identificador := "00000000-0000-4000-8000-000000000001"
	cuerpoTresIDs, _ := json.Marshal(map[string][]string{"producto_ids": []string{identificador, identificador, identificador}})

	for _, caso := range []struct {
		nombre string
		cuerpo string
		estado int
	}{
		{"JSON inválido", `{`, http.StatusBadRequest},
		{"campo desconocido", `{"producto_ids":["` + identificador + `"],"cantidad":2}`, http.StatusBadRequest},
		{"dos documentos", `{"producto_ids":["` + identificador + `"]} {}`, http.StatusBadRequest},
		{"lote vacío", `{"producto_ids":[]}`, http.StatusBadRequest},
		{"lote sobre el tope", string(cuerpoTresIDs), http.StatusBadRequest},
		{"identificador inválido", `{"producto_ids":["no-es-uuid"]}`, http.StatusBadRequest},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &repositorioEquivalenciasHTTP{}
			enrutador := nuevoRouterEquivalenciasHTTP(repositorio, token, 2, momento)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", strings.NewReader(caso.cuerpo))
			peticion.Header.Set("Authorization", "Bearer "+token)
			respuesta := httptest.NewRecorder()
			enrutador.ServeHTTP(respuesta, peticion)
			if respuesta.Code != caso.estado {
				subprueba.Fatalf("estado=%d; esperado=%d cuerpo=%s", respuesta.Code, caso.estado, respuesta.Body.String())
			}
			if repositorio.llamadas != 0 {
				subprueba.Fatalf("un cuerpo inválido consultó la base: %v", repositorio.idsRecibidos)
			}
		})
	}
}

// TestEquivalenciaHTTPRechazaCuerpoExcesivo verifica que el límite de bytes se
// aplique antes de decodificar el lote.
func TestEquivalenciaHTTPRechazaCuerpoExcesivo(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	repositorio := &repositorioEquivalenciasHTTP{}
	enrutador := nuevoRouterEquivalenciasHTTP(repositorio, token, 1000, time.Now())
	cuerpo := `{"producto_ids":["` + strings.Repeat("x", bytesPorCuerpoDePrueba) + `"]}`
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", strings.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusRequestEntityTooLarge || repositorio.llamadas != 0 {
		prueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, repositorio.llamadas, respuesta.Body.String())
	}
}

// TestEquivalenciaHTTPDevuelveGrupos verifica el contrato de respuesta y la
// deduplicación en una petición válida.
func TestEquivalenciaHTTPDevuelveGrupos(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-2 * time.Hour)
	productoLiderID := "00000000-0000-4000-8000-000000000001"
	productoJumboID := "00000000-0000-4000-8000-000000000002"
	sinEANID := "00000000-0000-4000-8000-000000000003"
	inexistenteID := "00000000-0000-4000-8000-000000000004"
	precioNormal := 1290.0
	precioOferta := 990.0
	enStockVerdadero := true

	repositorio := &repositorioEquivalenciasHTTP{registros: []domain.ProductoEquivalenciaRegistro{
		{
			ProductoID: productoLiderID, CodigoEAN: referenciaCadena("0036000291452"), Nombre: "Leche entera 1 L",
			Marca: "Colún", Formato: "1 L", SucursalID: 2, Cadena: "Lider", Sucursal: "Lider Ñuñoa",
			EnStock: &enStockVerdadero, PrecioNormal: &precioNormal, PrecioOferta: &precioOferta, CapturadoEl: &fresco,
		},
		{
			ProductoID: productoJumboID, CodigoEAN: referenciaCadena("036000291452"), Nombre: "Leche entera 1 L",
			Marca: "Colún", Formato: "1 L", SucursalID: 1, Cadena: "Jumbo", Sucursal: "Jumbo Centro",
			EnStock: &enStockVerdadero, PrecioNormal: &precioNormal, CapturadoEl: &fresco,
		},
		{
			ProductoID: sinEANID, Nombre: "Arroz 1 kg", Marca: "Tucapel", Formato: "1 kg",
			SucursalID: 3, Cadena: "Santa Isabel", Sucursal: "Santa Isabel Ñuñoa",
			EnStock: &enStockVerdadero, PrecioNormal: &precioNormal, CapturadoEl: &fresco,
		},
	}}
	enrutador := nuevoRouterEquivalenciasHTTP(repositorio, token, 1000, momento)
	cuerpo, _ := json.Marshal(map[string][]string{"producto_ids": []string{productoLiderID, sinEANID, inexistenteID, productoLiderID}})
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", bytes.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}

	var resultado domain.RespuestaEquivalenciaDTO
	if errorDecodificacion := json.Unmarshal(respuesta.Body.Bytes(), &resultado); errorDecodificacion != nil {
		prueba.Fatal(errorDecodificacion)
	}
	if len(resultado.Productos) != 3 {
		prueba.Fatalf("se esperaban 3 productos únicos, se obtuvieron %d", len(resultado.Productos))
	}
	if resultado.Productos[0].Estado != domain.EstadoEquivalenciaComparable || resultado.Productos[0].EAN == nil ||
		*resultado.Productos[0].EAN != "00036000291452" || len(resultado.Productos[0].Ofertas) != 2 {
		prueba.Fatalf("grupo comparable incorrecto: %+v", resultado.Productos[0])
	}
	if resultado.Productos[1].Estado != domain.EstadoEquivalenciaSinEAN || len(resultado.Productos[1].Ofertas) != 1 {
		prueba.Fatalf("producto sin EAN incorrecto: %+v", resultado.Productos[1])
	}
	if resultado.Productos[2].Estado != domain.EstadoEquivalenciaNoEncontrado || len(resultado.Productos[2].Ofertas) != 0 {
		prueba.Fatalf("producto inexistente incorrecto: %+v", resultado.Productos[2])
	}
	if resultado.Resumen.Solicitados != 3 || resultado.Resumen.Comparables != 1 ||
		resultado.Resumen.SinEAN != 1 || resultado.Resumen.NoEncontrados != 1 {
		prueba.Fatalf("resumen incorrecto: %+v", resultado.Resumen)
	}

	var crudo map[string]any
	if errorDecodificacion := json.Unmarshal(respuesta.Body.Bytes(), &crudo); errorDecodificacion != nil {
		prueba.Fatal(errorDecodificacion)
	}
	if _, existe := crudo["productos"]; !existe {
		prueba.Fatal("la respuesta no incluye productos")
	}
}

// TestEquivalenciaHTTPOcultaErrorDePersistencia verifica que un fallo SQL se
// transforme en 500 genérico sin filtrar el detalle interno.
func TestEquivalenciaHTTPOcultaErrorDePersistencia(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	repositorio := &repositorioEquivalenciasHTTP{errorConsulta: errors.New("secret SQL productos_crudos")}
	enrutador := nuevoRouterEquivalenciasHTTP(repositorio, token, 1000, time.Now())
	cuerpo := `{"producto_ids":["00000000-0000-4000-8000-000000000001"]}`
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", strings.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusInternalServerError ||
		strings.Contains(respuesta.Body.String(), "secret") ||
		strings.Contains(respuesta.Body.String(), "productos_crudos") {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
}

// TestEquivalenciaHTTPNoSeRegistraSinServicio confirma que la ruta no existe si
// el servicio de equivalencias no se cableó.
func TestEquivalenciaHTTPNoSeRegistraSinServicio(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	enrutador := nuevoEnrutadorDePrueba(api.Dependencias{TokenConsulta: token})
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/equivalencias", strings.NewReader(`{"producto_ids":[]}`))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("estado=%d; se esperaba 404 sin servicio", respuesta.Code)
	}
}
