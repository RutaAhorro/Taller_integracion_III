// Prueba de extremo a extremo contra un PostgreSQL real y aislado, con el rol
// de runtime app_catalogo: permisos al arrancar, ingesta idempotente, precios
// del catálogo público, disponibilidad y activación de fuentes. Se omite si no
// se define CATALOGO_TEST_DB_URL. Nunca apuntarla a una base compartida.
package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Preparación =====

// tokensPostgres son los tokens internos de la prueba, distintos entre sí.
var (
	tokenIngestaPostgres  = strings.Repeat("i", 32)
	tokenConsultaPostgres = strings.Repeat("c", 32)
)

// baseDatosPostgres abre la base de la variable con el rol de runtime y la
// cierra al terminar la prueba; se omite la prueba si la variable no existe.
func baseDatosPostgres(prueba *testing.T) *gorm.DB {
	prueba.Helper()
	urlBaseDatos := os.Getenv("CATALOGO_TEST_DB_URL")
	if urlBaseDatos == "" {
		prueba.Skip("CATALOGO_TEST_DB_URL no definida: se omite la prueba con PostgreSQL aislado")
	}
	baseDatos, errorApertura := repositories.AbrirBaseDatos(urlBaseDatos)
	if errorApertura != nil {
		prueba.Fatal(errorApertura)
	}
	conexiones, errorConexiones := baseDatos.DB()
	if errorConexiones != nil {
		prueba.Fatal(errorConexiones)
	}
	prueba.Cleanup(func() { _ = conexiones.Close() })
	return baseDatos
}

// enrutadorPostgres compone el router real sobre la base de la variable.
func enrutadorPostgres(prueba *testing.T) http.Handler {
	prueba.Helper()
	baseDatos := baseDatosPostgres(prueba)
	conexiones, errorConexiones := baseDatos.DB()
	if errorConexiones != nil {
		prueba.Fatal(errorConexiones)
	}
	if errorPermisos := repositories.VerificarAlmacenamiento(prueba.Context(), baseDatos, true); errorPermisos != nil {
		prueba.Fatalf("el rol de runtime no tiene los permisos de Catálogo: %v", errorPermisos)
	}
	return nuevoEnrutadorDePrueba(api.Dependencias{
		Productos: services.NuevoServicioProductos(repositories.NuevoRepositorioProductos(baseDatos), limitesProductosDePrueba),
		Fuentes:   services.NuevoServicioFuentes(repositories.NuevoRepositorioFuentes(baseDatos)),
		Disponibilidad: services.NuevoServicioDisponibilidad(repositories.NuevoRepositorioDisponibilidad(baseDatos),
			services.LimitesDisponibilidad{MaximoProductos: 10, AntiguedadMaxima: time.Hour}, time.Now),
		Ingesta:       repositories.NuevoRepositorioIngesta(baseDatos),
		TokenIngesta:  tokenIngestaPostgres,
		TokenConsulta: tokenConsultaPostgres,
		SondaBase:     conexiones.PingContext,
	})
}

// pedir envía una petición JSON y decodifica la respuesta en destino.
func pedir(prueba *testing.T, enrutador http.Handler, metodo, ruta, token string, cookie *http.Cookie, cuerpo any, destino any) int {
	prueba.Helper()
	var lector *bytes.Reader
	if cuerpo != nil {
		contenido, _ := json.Marshal(cuerpo)
		lector = bytes.NewReader(contenido)
	} else {
		lector = bytes.NewReader(nil)
	}
	peticion := httptest.NewRequest(metodo, ruta, lector)
	peticion.Header.Set("Content-Type", "application/json")
	if token != "" {
		peticion.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != nil {
		peticion.AddCookie(cookie)
	}
	respuesta := httptest.NewRecorder()
	enrutador.ServeHTTP(respuesta, peticion)
	if destino != nil && respuesta.Code < http.StatusBadRequest {
		if errorJSON := json.Unmarshal(respuesta.Body.Bytes(), destino); errorJSON != nil {
			prueba.Fatalf("%s %s: JSON inválido %v: %s", metodo, ruta, errorJSON, respuesta.Body)
		}
	}
	return respuesta.Code
}

// ===== Flujo completo =====

func TestPostgresIngestaPreciosYDisponibilidad(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba)
	sufijo := uuid.NewString()[:8]
	listo := pedir(prueba, enrutador, http.MethodGet, "/_catalogo/ready", "", nil, nil, nil)
	if listo != http.StatusOK {
		prueba.Fatalf("readiness=%d", listo)
	}

	// Cadena nueva por nombre: se crea y al repetir con otra capitalización se reutiliza.
	var cadena, cadenaRepetida domain.CadenaSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil, map[string]any{"nombre": "Cadena Prueba " + sufijo}, &cadena); estado != http.StatusOK || cadena.ID == 0 {
		prueba.Fatalf("resolver cadena=%d %+v", estado, cadena)
	}
	pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil, map[string]any{"nombre": "CADENA PRUEBA " + sufijo}, &cadenaRepetida)
	if cadenaRepetida.ID != cadena.ID {
		prueba.Fatalf("la cadena se duplicó: %d y %d", cadena.ID, cadenaRepetida.ID)
	}

	// SUP-376: Catálogo no crea sucursales. Se registra una sucursal real (como
	// lo haría quien administra la cadena) y recién entonces se resuelve por cadena.
	sucursalSembrada := sembrarSucursal(prueba, baseDatos, cadena.ID, "SUC-"+sufijo, true)
	var sucursal domain.SucursalSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/sucursales/resolver", tokenIngestaPostgres, nil, map[string]any{"cadena_id": cadena.ID}, &sucursal); estado != http.StatusOK || sucursal.ID != sucursalSembrada {
		prueba.Fatalf("resolver sucursal=%d %+v; se esperaba la sembrada %d", estado, sucursal, sucursalSembrada)
	}

	// Lote idempotente: oferta explícita, alias heredado `precio` y un ítem inválido.
	loteID := uuid.NewString()
	sinStock := false
	lote := map[string]any{"lote_id": loteID, "sucursal_id": sucursal.ID, "productos": []map[string]any{
		{"sku": "A-" + sufijo, "producto": "Leche Prueba " + sufijo, "marca": "Marca" + sufijo, "precio_normal": 1490, "precio_oferta": 990},
		{"sku": "B-" + sufijo, "producto": "Arroz Prueba " + sufijo, "precio_normal": 2000, "precio": 1800, "en_stock": sinStock},
		{"sku": " ", "producto": "Sin SKU", "precio_normal": 10},
	}}
	var resultado, repetido domain.ResultadoIngestaDTO
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, &resultado); estado != http.StatusOK {
		prueba.Fatalf("ingesta=%d", estado)
	}
	if resultado.TotalRecibidos != 3 || resultado.Insertados != 2 || resultado.PreciosRegistrados != 2 || len(resultado.Errores) != 1 {
		prueba.Fatalf("resultado de ingesta inesperado: %+v", resultado)
	}
	pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, &repetido)
	if repetido.PreciosRegistrados != 2 || repetido.Mensaje != resultado.Mensaje {
		prueba.Fatalf("el reintento no devolvió el recibo: %+v", repetido)
	}
	lote["productos"] = lote["productos"].([]map[string]any)[:1]
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, nil); estado != http.StatusConflict {
		prueba.Fatalf("misma clave con otro contenido=%d; se esperaba 409", estado)
	}

	// Catálogo público: el reintento no duplicó capturas y los precios son los efectivos.
	var pagina domain.PaginaProductosDTO
	pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos?q=Prueba+"+sufijo+"&sort_by=precio&order=asc", "", nil, nil, &pagina)
	if pagina.TotalRegistros != 2 || len(pagina.Data) != 2 {
		prueba.Fatalf("catálogo público inesperado: %+v", pagina)
	}
	precios := map[string]domain.ProductoDTO{}
	for _, producto := range pagina.Data {
		precios[producto.Nombre] = producto
	}
	leche, arroz := precios["Leche Prueba "+sufijo], precios["Arroz Prueba "+sufijo]
	if leche.Precio != 990 || !leche.EnOferta || !leche.EnStock || arroz.Precio != 1800 || !arroz.EnOferta || arroz.EnStock {
		prueba.Fatalf("precios o stock incorrectos: leche=%+v arroz=%+v", leche, arroz)
	}
	var detalle domain.ProductoDetalleDTO
	pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos/"+leche.ID, "", nil, nil, &detalle)
	if len(detalle.Historial) != 1 {
		prueba.Fatalf("el reintento duplicó capturas: %+v", detalle.Historial)
	}

	// Disponibilidad: vigente con y sin stock, y no encontrado.
	inexistente := uuid.NewString()
	var disponibilidad domain.RespuestaDisponibilidadDTO
	pedir(prueba, enrutador, http.MethodPost, "/internal/v1/productos/disponibilidad", tokenConsultaPostgres, nil,
		map[string]any{"producto_ids": []string{leche.ID, arroz.ID, inexistente}}, &disponibilidad)
	estados := map[string]string{}
	for _, item := range disponibilidad.Productos {
		estados[item.ProductoID] = item.EstadoProducto + "/" + item.EstadoPrecio
	}
	if estados[leche.ID] != "disponible/vigente" || estados[arroz.ID] != "sin_stock/vigente" || estados[inexistente] != "no_encontrado/sin_captura" {
		prueba.Fatalf("disponibilidad inesperada: %v", estados)
	}

	// Fuentes: el admin desactiva la cadena; Catálogo sólo actualiza esta_activa.
	var fuente domain.FuenteDTO
	ruta := "/api/v1/admin/productos/fuentes/" + strconv.Itoa(cadena.ID)
	if estado := pedir(prueba, enrutador, http.MethodPut, ruta, "", cookieAdministrador(prueba), map[string]any{"habilitada": false}, &fuente); estado != http.StatusOK || fuente.Habilitada || fuente.TotalProductos != 2 {
		prueba.Fatalf("actualizar fuente=%d %+v", estado, fuente)
	}
}

// ===== Sucursales: Catálogo no las crea (SUP-376) =====

// sembrarSucursal inserta una sucursal con datos de prueba y devuelve su ID. El
// rol app_catalogo conserva INSERT sobre la tabla, pero el código de Catálogo ya
// no lo usa: aquí actúa como quien registra la sucursal real.
func sembrarSucursal(prueba *testing.T, baseDatos *gorm.DB, cadenaID int, codigo string, activa bool) int {
	prueba.Helper()
	var sucursalID int
	errorInsercion := baseDatos.Raw(`INSERT INTO scraper.sucursales_supermercado
		(cadena_id, codigo_sucursal, nombre, direccion, lat, lon, esta_activa)
		VALUES (?, ?, ?, 'Dirección de prueba 123', -38.7, -72.6, ?) RETURNING id`,
		cadenaID, codigo, "Sucursal "+codigo, activa).Scan(&sucursalID).Error
	if errorInsercion != nil {
		prueba.Fatalf("no se pudo sembrar la sucursal: %v", errorInsercion)
	}
	return sucursalID
}

// contarSucursales devuelve cuántas filas tiene la cadena, activas o no.
func contarSucursales(prueba *testing.T, baseDatos *gorm.DB, cadenaID int) int64 {
	prueba.Helper()
	var total int64
	if errorConteo := baseDatos.Table("scraper.sucursales_supermercado").Where("cadena_id = ?", cadenaID).Count(&total).Error; errorConteo != nil {
		prueba.Fatal(errorConteo)
	}
	return total
}

// TestPostgresSucursalNoSeInventa cubre la regla de SUP-376: una cadena sin
// sucursal activa no recibe ninguna fila nueva y la ruta responde 404.
func TestPostgresSucursalNoSeInventa(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba)
	repositorio := repositories.NuevoRepositorioIngesta(baseDatos)
	sufijo := uuid.NewString()[:8]

	var cadena domain.CadenaSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil, map[string]any{"nombre": "Cadena Sin Sucursal " + sufijo}, &cadena); estado != http.StatusOK {
		prueba.Fatalf("resolver cadena=%d", estado)
	}

	prueba.Run("sin sucursal activa devuelve nil y no inserta", func(subprueba *testing.T) {
		sucursal, errorObtenido := repositorio.ObtenerSucursal(subprueba.Context(), cadena.ID, nil, nil)
		if errorObtenido != nil || sucursal != nil {
			subprueba.Fatalf("se esperaba nil sin error, llegó %+v, %v", sucursal, errorObtenido)
		}
		if total := contarSucursales(subprueba, baseDatos, cadena.ID); total != 0 {
			subprueba.Fatalf("Catálogo insertó %d sucursales para una cadena sin sucursal", total)
		}
	})

	prueba.Run("HTTP responde 404 branch_not_found y no inserta", func(subprueba *testing.T) {
		peticion := httptest.NewRequest(http.MethodPost, "/internal/v1/sucursales/resolver", strings.NewReader(`{"cadena_id":`+strconv.Itoa(cadena.ID)+`}`))
		peticion.Header.Set("Content-Type", "application/json")
		peticion.Header.Set("Authorization", "Bearer "+tokenIngestaPostgres)
		respuesta := httptest.NewRecorder()
		enrutador.ServeHTTP(respuesta, peticion)
		if respuesta.Code != http.StatusNotFound || !strings.Contains(respuesta.Body.String(), `"branch_not_found"`) {
			subprueba.Fatalf("respuesta=%d %s", respuesta.Code, respuesta.Body)
		}
		if total := contarSucursales(subprueba, baseDatos, cadena.ID); total != 0 {
			subprueba.Fatalf("la ruta insertó %d sucursales", total)
		}
	})

	prueba.Run("una sucursal inactiva no se devuelve", func(subprueba *testing.T) {
		sembrarSucursal(subprueba, baseDatos, cadena.ID, "INACTIVA-"+sufijo, false)
		sucursal, errorObtenido := repositorio.ObtenerSucursal(subprueba.Context(), cadena.ID, nil, nil)
		if errorObtenido != nil || sucursal != nil {
			subprueba.Fatalf("la sucursal inactiva no debe resolverse: %+v, %v", sucursal, errorObtenido)
		}
		if total := contarSucursales(subprueba, baseDatos, cadena.ID); total != 1 {
			subprueba.Fatalf("debía quedar sólo la sucursal sembrada, hay %d", total)
		}
	})

	prueba.Run("con sucursal activa devuelve la primera por id", func(subprueba *testing.T) {
		primera := sembrarSucursal(subprueba, baseDatos, cadena.ID, "ACTIVA-1-"+sufijo, true)
		sembrarSucursal(subprueba, baseDatos, cadena.ID, "ACTIVA-2-"+sufijo, true)
		sucursal, errorObtenido := repositorio.ObtenerSucursal(subprueba.Context(), cadena.ID, nil, nil)
		if errorObtenido != nil || sucursal == nil || sucursal.ID != primera {
			subprueba.Fatalf("se esperaba la sucursal %d, llegó %+v, %v", primera, sucursal, errorObtenido)
		}
		if total := contarSucursales(subprueba, baseDatos, cadena.ID); total != 3 {
			subprueba.Fatalf("la resolución modificó la tabla: %d filas", total)
		}
	})
}
