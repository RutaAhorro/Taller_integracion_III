// Pruebas del campo `supermercados` del listado público (SUP-371): contrato
// JSON, forma del SQL (sin paginación, sin filtro de cadena, filtros comunes) y
// comportamiento real contra PostgreSQL aislado (CATALOGO_TEST_DB_URL).
package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Contrato JSON =====

// TestSupermercados_ContratoJSON comprueba los nombres de campo y que, sin
// coincidencias, el campo es `[]` y no `null`.
func TestSupermercados_ContratoJSON(prueba *testing.T) {
	casos := []struct {
		nombre   string
		cadenas  []domain.SupermercadoDTO
		esperado string
	}{
		{"con cadenas", []domain.SupermercadoDTO{{ID: 2, Nombre: "Jumbo"}}, `"supermercados":[{"id":2,"nombre":"Jumbo"}]`},
		{"sin coincidencias", nil, `"supermercados":[]`},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &mockProductoRepository{supermercados: caso.cadenas}
			enrutador := nuevoEnrutadorDePrueba(api.Dependencias{Productos: services.NuevoServicioProductos(repositorio, limitesProductosDePrueba)})
			for _, ruta := range []string{"/api/v1/productos", "/api/v1/productos/buscar?q=leche"} {
				respuesta := httptest.NewRecorder()
				enrutador.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, ruta, nil))
				if respuesta.Code != http.StatusOK || !strings.Contains(respuesta.Body.String(), caso.esperado) {
					subprueba.Fatalf("%s: %d %s", ruta, respuesta.Code, respuesta.Body.String())
				}
			}
		})
	}
}

// ===== SQL generado =====

// TestListarSupermercados_SQL comprueba la forma de la agregación: DISTINCT,
// orden estable, filtros comunes, sin LIMIT/OFFSET ni filtro de cadena, y sin
// la captura más reciente salvo que un filtro de precio la necesite.
func TestListarSupermercados_SQL(prueba *testing.T) {
	enStock := true
	precioMinimo := 100.0
	casos := []struct {
		nombre       string
		filtro       domain.FiltroProductosDTO
		debeContener []string
		noContener   []string
	}{
		{
			nombre: "filtros comunes sin cadena ni paginación",
			filtro: domain.FiltroProductosDTO{Query: "leche", Categoria: "Lacteos", EnStock: &enStock, Supermercado: "Lider", Page: 3, Limit: 10},
			debeContener: []string{"SELECT DISTINCT cs.id AS id, cs.nombre AS nombre", "pc.titulo_crudo ILIKE", "pc.en_stock =",
				"ORDER BY cs.nombre ASC, cs.id ASC"},
			noContener: []string{"cs.nombre ILIKE", "LIMIT", "OFFSET", "LATERAL"},
		},
		{
			nombre:       "la categoría exacta se comparte con el listado sin unir capturas",
			filtro:       domain.FiltroProductosDTO{CategoriaExacta: "Pan / masas", Supermercado: "Lider"},
			debeContener: []string{"BTRIM(pc.categoria_cruda) ="},
			noContener:   []string{"pc.titulo_crudo ILIKE", "cs.nombre ILIKE", "LATERAL"},
		},
		{
			nombre:       "un filtro de precio sí une la última captura",
			filtro:       domain.FiltroProductosDTO{PrecioMin: &precioMinimo},
			debeContener: []string{"LATERAL", "COALESCE(cp.precio_oferta, cp.precio_normal, 0) >="},
		},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio, consultas := crearRepositorioSeco(subprueba)
			consultaDominio, ok := repositorio.(domain.RepositorioProductos)
			if !ok {
				subprueba.Fatal("el repositorio no implementa la interfaz")
			}
			if _, errorObtenido := consultaDominio.ListarSupermercados(context.Background(), caso.filtro); errorObtenido != nil {
				subprueba.Fatal(errorObtenido)
			}
			consultaSQL := consultas.String()
			for _, esperado := range caso.debeContener {
				if !strings.Contains(consultaSQL, esperado) {
					subprueba.Errorf("falta %q en: %s", esperado, consultaSQL)
				}
			}
			for _, noEsperado := range caso.noContener {
				if strings.Contains(consultaSQL, noEsperado) {
					subprueba.Errorf("sobra %q en: %s", noEsperado, consultaSQL)
				}
			}
		})
	}
}

// TestPostgresCategorias verifica valores completos y selección exacta contra
// PostgreSQL aislado, incluyendo signos, duplicados y cadenas fuera de la página.
func TestPostgresCategorias(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba, baseDatos)
	sufijo := strings.ToUpper(uuid.NewString()[:8])
	categoria := "Lácteos / masas & 100%_" + sufijo
	alfa := sembrarCadena(prueba, baseDatos, enrutador, "Categoria Alfa "+sufijo)
	beta := sembrarCadena(prueba, baseDatos, enrutador, "Categoria Beta "+sufijo)
	sembrarProducto(prueba, enrutador, alfa, "Producto A "+sufijo, categoria, 1000, true)
	sembrarProducto(prueba, enrutador, beta, "Producto B "+sufijo, " "+categoria+" ", 2000, true)
	sembrarProducto(prueba, enrutador, beta, "Producto C "+sufijo, categoria+" especial", 3000, true)
	sembrarProducto(prueba, enrutador, beta, "Producto D "+sufijo, "", 4000, true)

	var respuesta struct {
		Categorias []string `json:"categorias"`
	}
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos/categorias", "", nil, nil, &respuesta); estado != http.StatusOK {
		prueba.Fatalf("categorías: HTTP %d", estado)
	}
	coincidencias := 0
	for _, valor := range respuesta.Categorias {
		if valor == "" || valor != strings.TrimSpace(valor) {
			prueba.Fatalf("categoría vacía o con espacios exteriores: %q", valor)
		}
		if valor == categoria {
			coincidencias++
		}
	}
	if coincidencias != 1 {
		prueba.Fatalf("categoría no deduplicada: %v", respuesta.Categorias)
	}

	// El título no contiene la categoría. Solo la igualdad con categoria_cruda
	// puede devolver estos dos productos, nunca el de la categoría más larga.
	parametros := url.Values{"categoria_exacta": {categoria}, "en_stock": {"true"}, "limit": {"1"}, "page": {"2"}}
	pagina := consultarListado(prueba, enrutador, parametros)
	if pagina.TotalRegistros != 2 || len(pagina.Data) != 1 ||
		!reflect.DeepEqual(nombresDe(pagina.Supermercados), []string{alfa.Nombre, beta.Nombre}) {
		prueba.Fatalf("categoría exacta/paginación/facetas: %+v", pagina)
	}
	parametros.Set("supermercado", alfa.Nombre)
	pagina = consultarListado(prueba, enrutador, parametros)
	if pagina.TotalRegistros != 1 || !reflect.DeepEqual(nombresDe(pagina.Supermercados), []string{alfa.Nombre, beta.Nombre}) {
		prueba.Fatalf("se perdieron alternativas: %+v", pagina)
	}
	parametros.Set("categoria_exacta", categoria+" inexistente")
	pagina = consultarListado(prueba, enrutador, parametros)
	if pagina.TotalRegistros != 0 || len(pagina.Supermercados) != 0 {
		prueba.Fatalf("se aceptó una coincidencia aproximada: %+v", pagina)
	}
}

// ===== PostgreSQL real =====

// cadenaDePrueba es una cadena sembrada con su sucursal.
type cadenaDePrueba struct {
	ID         int
	Nombre     string
	SucursalID int
}

// sembrarCadena crea (por la API interna) una cadena y le registra una
// sucursal real; desde SUP-376 Catálogo ya no la inventa al resolver la cadena.
func sembrarCadena(prueba *testing.T, baseDatos *gorm.DB, enrutador http.Handler, nombre string) cadenaDePrueba {
	prueba.Helper()
	var cadena domain.CadenaSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil, map[string]any{"nombre": nombre}, &cadena); estado != http.StatusOK {
		prueba.Fatalf("resolver cadena=%d", estado)
	}
	sucursalID := sembrarSucursal(prueba, baseDatos, cadena.ID, "SUC-"+uuid.NewString()[:8], true)
	return cadenaDePrueba{ID: cadena.ID, Nombre: nombre, SucursalID: sucursalID}
}

// sembrarProducto ingesta un producto con su stock y precio en la cadena.
func sembrarProducto(prueba *testing.T, enrutador http.Handler, cadena cadenaDePrueba, titulo, categoria string, precio int, enStock bool) {
	prueba.Helper()
	lote := map[string]any{"lote_id": uuid.NewString(), "sucursal_id": cadena.SucursalID, "productos": []map[string]any{
		{"sku": uuid.NewString()[:12], "producto": titulo, "categoria": categoria, "precio_normal": precio, "en_stock": enStock},
	}}
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, nil); estado != http.StatusOK {
		prueba.Fatalf("ingesta=%d", estado)
	}
}

// consultarListado pide el listado público y lo decodifica.
func consultarListado(prueba *testing.T, enrutador http.Handler, parametros url.Values) domain.PaginaProductosDTO {
	prueba.Helper()
	var pagina domain.PaginaProductosDTO
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos?"+parametros.Encode(), "", nil, nil, &pagina); estado != http.StatusOK {
		prueba.Fatalf("listado=%d", estado)
	}
	return pagina
}

// nombresDe devuelve los nombres de cadena en el orden recibido.
func nombresDe(supermercados []domain.SupermercadoDTO) []string {
	nombres := []string{}
	for _, supermercado := range supermercados {
		nombres = append(nombres, supermercado.Nombre)
	}
	return nombres
}

// TestPostgresSupermercadosDisponibles cubre los criterios de SUP-371 con datos
// reales: cadenas en páginas posteriores, independencia de la paginación,
// selección de cadena, stock, vacío y recálculo al limpiar la búsqueda.
func TestPostgresSupermercadosDisponibles(prueba *testing.T) {
	baseDatos := baseDatosPostgres(prueba)
	enrutador := enrutadorPostgres(prueba, baseDatos)
	sufijo := strings.ToUpper(uuid.NewString()[:8])
	termino := "Leche" + sufijo // único: aísla la prueba de otros datos de la base

	// Los nombres ordenan Alfa < Beta < Gamma < Delta-sin-stock (por nombre).
	alfa := sembrarCadena(prueba, baseDatos, enrutador, "A-Alfa "+sufijo)
	beta := sembrarCadena(prueba, baseDatos, enrutador, "B-Beta "+sufijo)
	gamma := sembrarCadena(prueba, baseDatos, enrutador, "C-Gamma "+sufijo)
	agotada := sembrarCadena(prueba, baseDatos, enrutador, "D-Agotada "+sufijo)

	// Precios crecientes: con orden por precio, cada cadena cae en una página distinta con limit=1.
	sembrarProducto(prueba, enrutador, alfa, termino+" entera", "Lacteos", 1000, true)
	sembrarProducto(prueba, enrutador, beta, termino+" light", "Lacteos", 2000, true)
	sembrarProducto(prueba, enrutador, gamma, termino+" deslactosada", "Lacteos", 3000, true)
	sembrarProducto(prueba, enrutador, agotada, termino+" polvo", "Lacteos", 4000, false)
	// Producto de otra categoría con el mismo término en Beta y una cadena distinta solo aquí.
	sembrarProducto(prueba, enrutador, beta, termino+" crema", "Cremas", 5000, true)

	todas := []string{alfa.Nombre, beta.Nombre, gamma.Nombre}
	conAgotada := append(append([]string{}, todas...), agotada.Nombre)

	prueba.Run("las cadenas de páginas posteriores aparecen y no dependen de la paginación", func(subprueba *testing.T) {
		for _, tamano := range []string{"1", "2", "50"} {
			for _, pagina := range []string{"1", "2", "3"} {
				respuesta := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "en_stock": {"true"}, "page": {pagina}, "limit": {tamano}})
				if !reflect.DeepEqual(nombresDe(respuesta.Supermercados), todas) {
					subprueba.Fatalf("page=%s limit=%s: %v", pagina, tamano, nombresDe(respuesta.Supermercados))
				}
			}
		}
		primera := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "en_stock": {"true"}, "limit": {"1"}})
		if len(primera.Data) != 1 || primera.TotalRegistros != 4 || primera.TotalPaginas != 4 {
			subprueba.Fatalf("total/páginas inesperados: %+v", primera.Paginacion)
		}
	})

	prueba.Run("el stock excluye la cadena sin productos disponibles, igual que el listado", func(subprueba *testing.T) {
		sinFiltro := consultarListado(subprueba, enrutador, url.Values{"q": {termino}})
		if !reflect.DeepEqual(nombresDe(sinFiltro.Supermercados), conAgotada) || sinFiltro.TotalRegistros != 5 {
			subprueba.Fatalf("sin en_stock: %v total=%d", nombresDe(sinFiltro.Supermercados), sinFiltro.TotalRegistros)
		}
		soloStock := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "en_stock": {"true"}})
		if !reflect.DeepEqual(nombresDe(soloStock.Supermercados), todas) {
			subprueba.Fatalf("con en_stock=true: %v", nombresDe(soloStock.Supermercados))
		}
	})

	prueba.Run("elegir una cadena filtra productos pero conserva las alternativas", func(subprueba *testing.T) {
		elegida := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "en_stock": {"true"}, "supermercado": {beta.Nombre}})
		if elegida.TotalRegistros != 2 {
			subprueba.Fatalf("la lista debe respetar la cadena: total=%d", elegida.TotalRegistros)
		}
		for _, producto := range elegida.Data {
			if producto.Supermercado != beta.Nombre {
				subprueba.Fatalf("producto de otra cadena: %+v", producto)
			}
		}
		if !reflect.DeepEqual(nombresDe(elegida.Supermercados), todas) {
			subprueba.Fatalf("se perdieron alternativas: %v", nombresDe(elegida.Supermercados))
		}
	})

	prueba.Run("los demás filtros sí restringen las cadenas", func(subprueba *testing.T) {
		cremas := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "categoria": {"Cremas"}, "en_stock": {"true"}})
		if !reflect.DeepEqual(nombresDe(cremas.Supermercados), []string{beta.Nombre}) {
			subprueba.Fatalf("categoría: %v", nombresDe(cremas.Supermercados))
		}
		caros := consultarListado(subprueba, enrutador, url.Values{"q": {termino}, "en_stock": {"true"}, "precio_min": {"2500"}})
		if !reflect.DeepEqual(nombresDe(caros.Supermercados), []string{beta.Nombre, gamma.Nombre}) {
			subprueba.Fatalf("precio_min: %v", nombresDe(caros.Supermercados))
		}
	})

	prueba.Run("sin coincidencias devuelve lista vacía", func(subprueba *testing.T) {
		var cuerpo map[string]json.RawMessage
		ruta := "/api/v1/productos?q=" + url.QueryEscape("Inexistente"+sufijo)
		if estado := pedir(subprueba, enrutador, http.MethodGet, ruta, "", nil, nil, &cuerpo); estado != http.StatusOK {
			subprueba.Fatalf("estado=%d", estado)
		}
		if string(cuerpo["supermercados"]) != "[]" {
			subprueba.Fatalf("se esperaba [] y llegó %s", cuerpo["supermercados"])
		}
	})

	prueba.Run("sin búsqueda se recalcula con los filtros restantes y el orden es estable", func(subprueba *testing.T) {
		sinBusqueda := consultarListado(subprueba, enrutador, url.Values{"categoria": {"Cremas"}, "supermercado": {beta.Nombre}, "limit": {"1"}})
		encontradas := map[string]bool{}
		for _, nombre := range nombresDe(sinBusqueda.Supermercados) {
			encontradas[nombre] = true
		}
		if !encontradas[beta.Nombre] {
			subprueba.Fatalf("faltan cadenas: %v", nombresDe(sinBusqueda.Supermercados))
		}
		primera := consultarListado(subprueba, enrutador, url.Values{"q": {termino}})
		segunda := consultarListado(subprueba, enrutador, url.Values{"q": {termino}})
		if !reflect.DeepEqual(primera, segunda) {
			subprueba.Fatalf("respuestas distintas entre llamadas idénticas: %+v / %+v", primera, segunda)
		}
	})
}
