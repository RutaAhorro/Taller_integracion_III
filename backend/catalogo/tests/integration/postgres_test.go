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

// enrutadorPostgres compone el router real sobre la base ya abierta por la
// prueba, para que la prueba y el router compartan una sola conexión.
func enrutadorPostgres(prueba *testing.T, baseDatos *gorm.DB) http.Handler {
	prueba.Helper()
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
		Equivalencias: services.NuevoServicioEquivalencias(repositories.NuevoRepositorioEquivalencias(baseDatos),
			services.LimitesEquivalencia{MaximoProductos: 10, AntiguedadMaxima: time.Hour}, time.Now),
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
	enrutador := enrutadorPostgres(prueba, baseDatos)
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
	enrutador := enrutadorPostgres(prueba, baseDatos)
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

// TestPostgresGuardaPrecioMinimo comprueba contra PostgreSQL real que las
// capturas con precio_normal < PrecioMinimoValido se ignoran y que los
// productos sin captura válida no se publican en el catálogo ni en detalle.
func TestPostgresGuardaPrecioMinimo(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba, baseDatos)
	sufijo := strings.ToUpper(uuid.NewString()[:8])
	termino := "Guarda" + sufijo

	cadena := sembrarCadena(prueba, baseDatos, enrutador, "Cadena Guarda "+sufijo)
	sembrarProducto(prueba, enrutador, cadena, termino+" Valido", "Despensa", 1290, true)
	sembrarProducto(prueba, enrutador, cadena, termino+" Basura", "Despensa", 2, true)
	sembrarProducto(prueba, enrutador, cadena, termino+" Umbral", "Despensa", domain.PrecioMinimoValido, true)

	// Producto sembrado directamente sin ninguna fila en scraper.capturas_precios.
	sinCapturaID := uuid.NewString()
	errorInsert := baseDatos.Exec(
		`INSERT INTO scraper.productos_crudos (id, sucursal_id, sku, titulo_crudo, categoria_cruda, en_stock)
		 VALUES (?::uuid, ?, ?, ?, 'Despensa', true)`,
		sinCapturaID, cadena.SucursalID, "SINCAP-"+sufijo, termino+" SinCaptura",
	).Error
	if errorInsert != nil {
		prueba.Fatalf("insert producto sin captura: %v", errorInsert)
	}

	var pagina domain.PaginaProductosDTO
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos?q="+termino, "", nil, nil, &pagina); estado != http.StatusOK {
		prueba.Fatalf("listado=%d", estado)
	}
	if pagina.TotalRegistros != 2 || len(pagina.Data) != 2 {
		prueba.Fatalf("sólo deben aparecer el producto válido y el del umbral (>= %d), llegaron %+v", domain.PrecioMinimoValido, pagina.Data)
	}
	for _, producto := range pagina.Data {
		if producto.PrecioNormal < domain.PrecioMinimoValido {
			prueba.Fatalf("se publicó un precio bajo el mínimo: %+v", producto)
		}
	}

	// Un producto sin captura válida tampoco debe publicarse en el detalle.
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos/"+sinCapturaID, "", nil, nil, nil); estado != http.StatusNotFound {
		prueba.Fatalf("detalle de producto sin captura=%d; se esperaba 404", estado)
	}
}

// TestPostgresRelevanciaBusqueda comprueba contra PostgreSQL real el orden por
// relevancia y precio del repositorio, el escape de comodines ILIKE (% y _) y
// la compatibilidad con filtros por supermercado, precio máximo y paginación.
func TestPostgresRelevanciaBusqueda(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba, baseDatos)
	repositorio := repositories.NuevoRepositorioProductos(baseDatos)
	contexto := prueba.Context()
	sufijo := strings.ToUpper(uuid.NewString()[:8])

	prefijoCadena := "Cadena Rel " + sufijo
	alfa := sembrarCadena(prueba, baseDatos, enrutador, prefijoCadena+" Alfa")
	beta := sembrarCadena(prueba, baseDatos, enrutador, prefijoCadena+" Beta")

	ingestarLote := func(sucursalID int, productos []map[string]any) {
		prueba.Helper()
		lote := map[string]any{
			"lote_id":     uuid.NewString(),
			"sucursal_id": sucursalID,
			"productos":   productos,
		}
		if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, nil); estado != http.StatusOK {
			prueba.Fatalf("ingesta de prueba=%d", estado)
		}
	}

	// Los productos menos relevantes tienen precios más bajos a propósito para
	// verificar que la relevancia prevalece sobre el precio cuando q está presente.
	ingestarLote(alfa.SucursalID, []map[string]any{
		{"sku": "REL-1-" + sufijo, "producto": "Yogurt Loncoleche " + sufijo, "marca": "Soprole", "categoria": "Postres", "precio_normal": 400, "en_stock": true},
		{"sku": "REL-2-" + sufijo, "producto": "Postre Cremoso " + sufijo, "marca": "Loncoleche", "categoria": "Postres", "precio_normal": 450, "en_stock": true},
		{"sku": "REL-3-" + sufijo, "producto": "Flan de leche " + sufijo, "marca": "Soprole", "categoria": "Postres", "precio_normal": 500, "en_stock": true},
		{"sku": "REL-4-" + sufijo, "producto": "Leche Entera Colun 1L " + sufijo, "marca": "Colun", "categoria": "Lacteos", "precio_normal": 1200, "en_stock": true},
		{"sku": "REL-5-" + sufijo, "producto": "Jugo 100%_especial " + sufijo, "marca": "Watt", "categoria": "Jugos", "precio_normal": 800, "en_stock": true},
		{"sku": "REL-6-" + sufijo, "producto": "Jugo 100XXespecial " + sufijo, "marca": "Watt", "categoria": "Jugos", "precio_normal": 700, "en_stock": true},
	})
	ingestarLote(beta.SucursalID, []map[string]any{
		{"sku": "REL-7-" + sufijo, "producto": "Leche Descremada 1L " + sufijo, "marca": "Soprole", "categoria": "Lacteos", "precio_normal": 900, "en_stock": true},
	})

	nombresDeProductos := func(productos []domain.ProductoDTO) []string {
		nombres := make([]string, len(productos))
		for indice, producto := range productos {
			nombres[indice] = producto.Nombre
		}
		return nombres
	}

	esperadoCompleto := []string{
		"Leche Descremada 1L " + sufijo,
		"Leche Entera Colun 1L " + sufijo,
		"Flan de leche " + sufijo,
		"Postre Cremoso " + sufijo,
		"Yogurt Loncoleche " + sufijo,
	}

	prueba.Run("leche prioriza prefijo, palabra completa, marca/categoría y resto", func(subprueba *testing.T) {
		productos, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "leche",
			Supermercado: prefijoCadena,
			Page:         1,
			Limit:        20,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("Listar generó error: %v", errorObtenido)
		}
		if total != 5 {
			subprueba.Fatalf("se esperaban 5 resultados, se obtuvo total=%d", total)
		}
		obtenidos := nombresDeProductos(productos)
		for indice, esperado := range esperadoCompleto {
			if obtenidos[indice] != esperado {
				subprueba.Fatalf("orden inesperado:\nobtenido: %v\nesperado: %v", obtenidos, esperadoCompleto)
			}
		}
	})

	prueba.Run("filtro por supermercado mantiene relevancia", func(subprueba *testing.T) {
		productos, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "leche",
			Supermercado: alfa.Nombre,
			Page:         1,
			Limit:        20,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("Listar con supermercado generó error: %v", errorObtenido)
		}
		if total != 4 || len(productos) != 4 {
			subprueba.Fatalf("se esperaban 4 resultados de Alfa, llegó total=%d len=%d", total, len(productos))
		}
		if productos[0].Nombre != "Leche Entera Colun 1L "+sufijo || productos[1].Nombre != "Flan de leche "+sufijo {
			subprueba.Fatalf("orden inesperado en Alfa: %v", nombresDeProductos(productos))
		}
	})

	prueba.Run("filtro por precio máximo mantiene relevancia", func(subprueba *testing.T) {
		precioMaximo := 1000.0
		productos, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "leche",
			Supermercado: prefijoCadena,
			PrecioMax:    &precioMaximo,
			Page:         1,
			Limit:        20,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("Listar con precio_max generó error: %v", errorObtenido)
		}
		if total != 4 || len(productos) != 4 {
			subprueba.Fatalf("se esperaban 4 resultados <= 1000, llegó total=%d len=%d", total, len(productos))
		}
		if productos[0].Nombre != "Leche Descremada 1L "+sufijo || productos[1].Nombre != "Flan de leche "+sufijo {
			subprueba.Fatalf("orden inesperado con precio_max: %v", nombresDeProductos(productos))
		}
	})

	prueba.Run("paginación corta sobre el conjunto ya ordenado por relevancia", func(subprueba *testing.T) {
		pagina1, total1, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "leche",
			Supermercado: prefijoCadena,
			Page:         1,
			Limit:        2,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("página 1 generó error: %v", errorObtenido)
		}
		pagina2, total2, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "leche",
			Supermercado: prefijoCadena,
			Page:         2,
			Limit:        2,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("página 2 generó error: %v", errorObtenido)
		}
		if total1 != 5 || total2 != 5 || len(pagina1) != 2 || len(pagina2) != 2 {
			subprueba.Fatalf("paginación inesperada: total1=%d len1=%d total2=%d len2=%d", total1, len(pagina1), total2, len(pagina2))
		}
		if pagina1[0].Nombre != esperadoCompleto[0] || pagina1[1].Nombre != esperadoCompleto[1] ||
			pagina2[0].Nombre != esperadoCompleto[2] || pagina2[1].Nombre != esperadoCompleto[3] {
			subprueba.Fatalf("páginas inconsistentes: p1=%v p2=%v", nombresDeProductos(pagina1), nombresDeProductos(pagina2))
		}
	})

	prueba.Run("sin q conserva el orden por precio", func(subprueba *testing.T) {
		productos, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Supermercado: prefijoCadena,
			Page:         1,
			Limit:        20,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("Listar sin q generó error: %v", errorObtenido)
		}
		if total != 7 || len(productos) != 7 {
			subprueba.Fatalf("se esperaban 7 productos, llegó total=%d len=%d", total, len(productos))
		}
		for indice := 1; indice < len(productos); indice++ {
			if productos[indice-1].Precio > productos[indice].Precio {
				subprueba.Fatalf("sin q no ordenó por precio ascendente: %+v", productos)
			}
		}
	})

	prueba.Run("escapa comodines % y _ en q", func(subprueba *testing.T) {
		productos, total, errorObtenido := repositorio.Listar(contexto, domain.FiltroProductosDTO{
			Query:        "100%_especial",
			Supermercado: prefijoCadena,
			Page:         1,
			Limit:        20,
		})
		if errorObtenido != nil {
			subprueba.Fatalf("Listar con comodines generó error: %v", errorObtenido)
		}
		if total != 1 || len(productos) != 1 || productos[0].Nombre != "Jugo 100%_especial "+sufijo {
			subprueba.Fatalf("los comodines %% y _ no se escaparon correctamente: total=%d productos=%v", total, nombresDeProductos(productos))
		}
	})
}
