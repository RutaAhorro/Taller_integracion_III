package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/consulta"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/gin-gonic/gin"
)

// repositorioDisponibilidadHTTP es el doble de persistencia para las pruebas
// del adaptador HTTP. Registra llamadas para comprobar que un lote inválido no
// llega a la base.
type repositorioDisponibilidadHTTP struct {
	registros     []domain.ProductoDisponibilidadRegistro
	errorConsulta error
	llamadas      int
	idsRecibidos  []string
}

func (repositorio *repositorioDisponibilidadHTTP) ConsultarPorIDs(_ context.Context, productoIDs []string) ([]domain.ProductoDisponibilidadRegistro, error) {
	repositorio.llamadas++
	repositorio.idsRecibidos = append([]string(nil), productoIDs...)
	if repositorio.errorConsulta != nil {
		return nil, repositorio.errorConsulta
	}
	return repositorio.registros, nil
}

// nuevoRouterDisponibilidadHTTP arma sólo la ruta interna para aislar la prueba
// de las rutas públicas y del Gateway.
func nuevoRouterDisponibilidadHTTP(repositorio *repositorioDisponibilidadHTTP, token string, maxProductos, maxBodyBytes int, ahora time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)
	servicio := services.NewDisponibilidadService(repositorio,
		services.LimitesDisponibilidad{MaxProductos: maxProductos, AntiguedadMaxima: 48 * time.Hour},
		func() time.Time { return ahora })
	enrutador := gin.New()
	consulta.Register(enrutador, consulta.Config{
		Service:      servicio,
		Token:        token,
		MaxBodyBytes: maxBodyBytes,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return enrutador
}

// TestDisponibilidadHTTPExigeTokenDeServicio cubre los rechazos de seguridad
// antes de leer o consultar datos.
func TestDisponibilidadHTTPExigeTokenDeServicio(prueba *testing.T) {
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
			repositorio := &repositorioDisponibilidadHTTP{}
			enrutador := nuevoRouterDisponibilidadHTTP(repositorio, token, 1000, bytesPorCuerpoDePrueba, momento)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", strings.NewReader(cuerpo))
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

// TestDisponibilidadHTTPValidaCuerpo comprueba el contrato estricto del cuerpo
// y el tope configurable de IDs.
func TestDisponibilidadHTTPValidaCuerpo(prueba *testing.T) {
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
			repositorio := &repositorioDisponibilidadHTTP{}
			enrutador := nuevoRouterDisponibilidadHTTP(repositorio, token, 2, bytesPorCuerpoDePrueba, momento)
			peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", strings.NewReader(caso.cuerpo))
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

// TestDisponibilidadHTTPRechazaCuerpoExcesivo verifica que el límite de bytes
// se aplique antes de decodificar el lote.
func TestDisponibilidadHTTPRechazaCuerpoExcesivo(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	repositorio := &repositorioDisponibilidadHTTP{}
	enrutador := nuevoRouterDisponibilidadHTTP(repositorio, token, 1000, 16, time.Now())
	cuerpo := `{"producto_ids":["00000000-0000-4000-8000-000000000001"]}`
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", strings.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusRequestEntityTooLarge || repositorio.llamadas != 0 {
		prueba.Fatalf("estado=%d llamadas=%d cuerpo=%s", respuesta.Code, repositorio.llamadas, respuesta.Body.String())
	}
}

// TestDisponibilidadHTTPDevuelveEstados verifica el contrato de respuesta y la
// deduplicación en una petición válida.
func TestDisponibilidadHTTPDevuelveEstados(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-2 * time.Hour)
	viejo := momento.Add(-10 * 24 * time.Hour)
	disponibleID := "00000000-0000-4000-8000-000000000001"
	sinStockID := "00000000-0000-4000-8000-000000000004"
	vencidoID := "00000000-0000-4000-8000-000000000002"
	inexistenteID := "00000000-0000-4000-8000-000000000003"
	precioNormal := 1290.0
	precioOferta := 990.0
	enStockVerdadero := true
	enStockFalso := false

	repositorio := &repositorioDisponibilidadHTTP{registros: []domain.ProductoDisponibilidadRegistro{
		{
			ProductoID: disponibleID, Nombre: "Leche entera 1 L", Marca: "Colún", Categoria: "Lácteos",
			Supermercado: "Lider", SucursalID: 1, Sucursal: "Lider Ñuñoa", Unidad: "1 L",
			URLImagen: "https://imagen", EnStock: &enStockVerdadero,
			PrecioNormal: &precioNormal, PrecioOferta: &precioOferta, CapturadoEl: &fresco,
		},
		{
			ProductoID: sinStockID, Nombre: "Arroz 1 kg", Marca: "Tucapel", Categoria: "Despensa",
			Supermercado: "Santa Isabel", SucursalID: 3, Sucursal: "Santa Isabel Ñuñoa", Unidad: "1 kg",
			EnStock: &enStockFalso, PrecioNormal: &precioNormal, CapturadoEl: &fresco,
		},
		{
			ProductoID: vencidoID, Nombre: "Aceite 1 L", Marca: "Chef", Categoria: "Despensa",
			Supermercado: "Jumbo", SucursalID: 2, Sucursal: "Jumbo Centro", Unidad: "1 L",
			EnStock: &enStockVerdadero, PrecioNormal: &precioNormal, CapturadoEl: &viejo,
		},
	}}
	enrutador := nuevoRouterDisponibilidadHTTP(repositorio, token, 1000, bytesPorCuerpoDePrueba, momento)
	cuerpo, _ := json.Marshal(map[string][]string{"producto_ids": []string{disponibleID, sinStockID, vencidoID, inexistenteID, disponibleID}})
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", bytes.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}

	var resultado domain.RespuestaDisponibilidadDTO
	if errorDecodificacion := json.Unmarshal(respuesta.Body.Bytes(), &resultado); errorDecodificacion != nil {
		prueba.Fatal(errorDecodificacion)
	}
	if len(resultado.Productos) != 4 {
		prueba.Fatalf("se esperaban 4 productos únicos, se obtuvieron %d", len(resultado.Productos))
	}
	if resultado.Productos[0].EstadoProducto != domain.EstadoProductoDisponible ||
		resultado.Productos[0].Producto == nil || resultado.Productos[0].Producto.Precio == nil ||
		*resultado.Productos[0].Producto.Precio != 990 {
		prueba.Fatalf("producto disponible incorrecto: %+v", resultado.Productos[0])
	}
	if resultado.Productos[1].EstadoProducto != domain.EstadoProductoSinStock ||
		resultado.Productos[1].EnStock == nil || *resultado.Productos[1].EnStock {
		prueba.Fatalf("producto sin stock incorrecto: %+v", resultado.Productos[1])
	}
	if resultado.Productos[2].EstadoProducto != domain.EstadoProductoDesconocido ||
		resultado.Productos[2].EnStock != nil || resultado.Productos[2].Producto == nil ||
		resultado.Productos[2].Producto.Precio != nil ||
		resultado.Productos[2].EstadoPrecio != domain.EstadoPrecioVencido {
		prueba.Fatalf("producto vencido incorrecto: %+v", resultado.Productos[2])
	}
	if resultado.Productos[3].EstadoProducto != domain.EstadoProductoNoEncontrado ||
		resultado.Productos[3].Producto != nil {
		prueba.Fatalf("producto inexistente incorrecto: %+v", resultado.Productos[3])
	}
}

// TestDisponibilidadHTTPOcultaErrorDePersistencia verifica que un fallo SQL se
// transforme en 500 genérico sin filtrar el detalle interno.
func TestDisponibilidadHTTPOcultaErrorDePersistencia(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	repositorio := &repositorioDisponibilidadHTTP{errorConsulta: errors.New("secret SQL productos_crudos")}
	enrutador := nuevoRouterDisponibilidadHTTP(repositorio, token, 1000, bytesPorCuerpoDePrueba, time.Now())
	cuerpo := `{"producto_ids":["00000000-0000-4000-8000-000000000001"]}`
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", strings.NewReader(cuerpo))
	peticion.Header.Set("Authorization", "Bearer "+token)
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusInternalServerError ||
		strings.Contains(respuesta.Body.String(), "secret") ||
		strings.Contains(respuesta.Body.String(), "productos_crudos") {
		prueba.Fatalf("estado=%d cuerpo=%s", respuesta.Code, respuesta.Body.String())
	}
}

// TestDisponibilidadHTTPNoSeRegistraSinToken confirma el modo lector: sin
// CATALOGO_CONSULTA_TOKEN la ruta de consulta no existe.
func TestDisponibilidadHTTPNoSeRegistraSinToken(prueba *testing.T) {
	gin.SetMode(gin.TestMode)
	enrutador := gin.New()
	consulta.Register(enrutador, consulta.Config{
		Service:      services.NewDisponibilidadService(&repositorioDisponibilidadHTTP{}, services.LimitesDisponibilidad{MaxProductos: 10, AntiguedadMaxima: time.Hour}, time.Now),
		Token:        "",
		MaxBodyBytes: bytesPorCuerpoDePrueba,
	}, nil)
	peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/productos/disponibilidad", strings.NewReader(`{"producto_ids":[]}`))
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if respuesta.Code != http.StatusNotFound {
		prueba.Fatalf("estado=%d; se esperaba 404 sin token", respuesta.Code)
	}
}
