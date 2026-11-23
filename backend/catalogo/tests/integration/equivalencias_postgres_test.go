// Prueba de extremo a extremo de la equivalencia por EAN contra un PostgreSQL
// real y aislado. Se omite si no se define CATALOGO_TEST_DB_URL; nunca debe
// apuntarse a una base compartida.
package integration_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
)

// eanUnicoPorSufijo deriva un GTIN-13 válido, su UPC-A de 12 dígitos (que
// canoniza al mismo valor) y el GTIN-14 canónico. Cada ejecución usa códigos
// distintos para que la prueba se pueda repetir sobre la misma base sin que
// se mezclen grupos de corridas anteriores.
func eanUnicoPorSufijo(sufijo, prefijo string) (string, string, string) {
	var digitos strings.Builder
	for _, caracter := range sufijo {
		valor, _ := strconv.ParseUint(string(caracter), 16, 8)
		digitos.WriteString(strconv.FormatUint(valor%10, 10))
	}
	base12 := "0" + prefijo + digitos.String()
	suma := 0
	for posicion, caracter := range base12 {
		digito := int(caracter - '0')
		if posicion%2 == 0 {
			suma += digito
		} else {
			suma += 3 * digito
		}
	}
	ean13 := base12 + strconv.Itoa((10-suma%10)%10)
	return ean13, ean13[1:], "0" + ean13
}

// TestPostgresEquivalenciaPorEAN ingiere el mismo EAN en dos sucursales y
// comprueba los cuatro resultados: comparable con ceros iniciales, sin EAN,
// EAN inválido y conflicto de presentación.
func TestPostgresEquivalenciaPorEAN(prueba *testing.T) {
	enrutador := enrutadorPostgres(prueba)
	sufijo := uuid.NewString()[:8]

	// Cada cadena recibe su sucursal central para no depender de datos semilla.
	var cadenaOrigen, cadenaDestino domain.CadenaSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil,
		map[string]any{"nombre": "Cadena Origen " + sufijo}, &cadenaOrigen); estado != http.StatusOK {
		prueba.Fatalf("resolver cadena origen=%d", estado)
	}
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil,
		map[string]any{"nombre": "Cadena Destino " + sufijo}, &cadenaDestino); estado != http.StatusOK {
		prueba.Fatalf("resolver cadena destino=%d", estado)
	}
	var sucursalOrigen, sucursalDestino domain.SucursalSupermercado
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/sucursales/resolver", tokenIngestaPostgres, nil,
		map[string]any{"cadena_id": cadenaOrigen.ID}, &sucursalOrigen); estado != http.StatusOK {
		prueba.Fatalf("resolver sucursal origen=%d", estado)
	}
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/sucursales/resolver", tokenIngestaPostgres, nil,
		map[string]any{"cadena_id": cadenaDestino.ID}, &sucursalDestino); estado != http.StatusOK {
		prueba.Fatalf("resolver sucursal destino=%d", estado)
	}

	eanLeche13, eanLeche12, canonicoLeche := eanUnicoPorSufijo(sufijo, "780")
	eanConflicto13, _, _ := eanUnicoPorSufijo(sufijo, "179")

	nombreLeche := "Leche Eq" + sufijo
	nombreConflicto := "Conflicto Eq" + sufijo
	nombreSinEAN := "SinEAN Eq" + sufijo
	loteOrigen := map[string]any{"sucursal_id": sucursalOrigen.ID, "productos": []map[string]any{
		{"sku": "LECHE-" + sufijo, "ean_gtin": eanLeche13, "producto": nombreLeche, "formato_crudo": "1 L", "precio_normal": 1290},
		{"sku": "CONFLICTO-" + sufijo, "ean_gtin": eanConflicto13, "producto": nombreConflicto, "formato_crudo": "1 L", "precio_normal": 1500},
		{"sku": "SINEAN-" + sufijo, "producto": nombreSinEAN, "formato_crudo": "500 g", "precio_normal": 800},
	}}
	loteDestino := map[string]any{"sucursal_id": sucursalDestino.ID, "productos": []map[string]any{
		{"sku": "LECHE-" + sufijo, "ean_gtin": eanLeche12, "producto": nombreLeche, "formato_crudo": "1 L", "precio_normal": 1190},
		{"sku": "CONFLICTO-" + sufijo, "ean_gtin": eanConflicto13, "producto": nombreConflicto, "formato_crudo": "1.5 L", "precio_normal": 1900},
		{"sku": "SINEAN-" + sufijo, "producto": nombreSinEAN, "formato_crudo": "500 g", "precio_normal": 850},
	}}
	for nombreLote, lote := range map[string]map[string]any{"origen": loteOrigen, "destino": loteDestino} {
		var resultado domain.ResultadoIngestaDTO
		if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, &resultado); estado != http.StatusOK {
			prueba.Fatalf("ingesta %s=%d %+v", nombreLote, estado, resultado)
		}
	}

	// El catálogo público entrega los IDs reales que luego se consultan.
	var pagina domain.PaginaProductosDTO
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos?q=Eq"+sufijo+"&limit=50", "", nil, nil, &pagina); estado != http.StatusOK {
		prueba.Fatalf("catálogo público=%d", estado)
	}
	productosPorNombre := make(map[string]domain.ProductoDTO, len(pagina.Data))
	for _, producto := range pagina.Data {
		productosPorNombre[producto.Nombre] = producto
	}
	productoLeche, existeLeche := productosPorNombre[nombreLeche]
	productoConflicto, existeConflicto := productosPorNombre[nombreConflicto]
	productoSinEAN, existeSinEAN := productosPorNombre[nombreSinEAN]
	if !existeLeche || !existeConflicto || !existeSinEAN {
		prueba.Fatalf("no se encontraron los productos de la prueba: %v", productosPorNombre)
	}

	var respuesta domain.RespuestaEquivalenciaDTO
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/productos/equivalencias", tokenConsultaPostgres, nil,
		map[string]any{"producto_ids": []string{productoLeche.ID, productoConflicto.ID, productoSinEAN.ID}}, &respuesta); estado != http.StatusOK {
		prueba.Fatalf("equivalencias=%d", estado)
	}
	itemsPorID := make(map[string]domain.EquivalenciaProductoDTO, len(respuesta.Productos))
	for _, item := range respuesta.Productos {
		itemsPorID[item.ProductoID] = item
	}

	leche := itemsPorID[productoLeche.ID]
	if leche.Estado != domain.EstadoEquivalenciaComparable || leche.EAN == nil || *leche.EAN != canonicoLeche || len(leche.Ofertas) != 2 {
		prueba.Fatalf("leche comparable incorrecta: %+v", leche)
	}
	precios := make(map[float64]bool, len(leche.Ofertas))
	for _, oferta := range leche.Ofertas {
		if oferta.Precio == nil {
			prueba.Fatalf("oferta sin precio vigente: %+v", oferta)
		}
		precios[*oferta.Precio] = true
	}
	if !precios[1290] || !precios[1190] {
		prueba.Fatalf("precios por sucursal incorrectos: %+v", leche.Ofertas)
	}

	conflicto := itemsPorID[productoConflicto.ID]
	if conflicto.Estado != domain.EstadoEquivalenciaConflictoPresentacion || len(conflicto.Ofertas) != 1 {
		prueba.Fatalf("conflicto de presentación incorrecto: %+v", conflicto)
	}

	sinEAN := itemsPorID[productoSinEAN.ID]
	if sinEAN.Estado != domain.EstadoEquivalenciaSinEAN || sinEAN.EAN != nil || len(sinEAN.Ofertas) != 1 {
		prueba.Fatalf("producto sin EAN incorrecto: %+v", sinEAN)
	}

	if respuesta.Resumen.Solicitados != 3 || respuesta.Resumen.Comparables != 1 ||
		respuesta.Resumen.ConflictosPresentacion != 1 || respuesta.Resumen.SinEAN != 1 {
		prueba.Fatalf("resumen inesperado: %+v", respuesta.Resumen)
	}
}

// TestPostgresEquivalenciaCategoriaComunPorCadena comprueba una categoría
// común a las seis cadenas sembradas: un producto por cadena con el mismo EAN
// debe formar un solo grupo comparable con seis ofertas vigentes. Es una
// prueba funcional con datos creados por la propia prueba; la cobertura real
// de EAN se mide por separado sobre los datos persistidos por el scraper.
func TestPostgresEquivalenciaCategoriaComunPorCadena(prueba *testing.T) {
	enrutador := enrutadorPostgres(prueba)
	sufijo := uuid.NewString()[:8]
	categoria := "Lacteos Eq" + sufijo
	nombreProducto := "Leche Categoria Comun " + sufijo
	ean13, _, canonico := eanUnicoPorSufijo(sufijo, "906")
	cadenas := []string{"Jumbo", "Lider", "Unimarc", "Santa Isabel", "A Cuenta", "Cugat"}
	preciosEsperados := make(map[string]float64, len(cadenas))

	for indice, nombreCadena := range cadenas {
		var cadena domain.CadenaSupermercado
		if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/cadenas/resolver", tokenIngestaPostgres, nil,
			map[string]any{"nombre": nombreCadena}, &cadena); estado != http.StatusOK || cadena.ID == 0 {
			prueba.Fatalf("resolver cadena %s=%d %+v", nombreCadena, estado, cadena)
		}
		var sucursal domain.SucursalSupermercado
		if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/sucursales/resolver", tokenIngestaPostgres, nil,
			map[string]any{"cadena_id": cadena.ID}, &sucursal); estado != http.StatusOK || sucursal.ID == 0 {
			prueba.Fatalf("resolver sucursal de %s=%d %+v", nombreCadena, estado, sucursal)
		}
		precio := float64(1000 + indice*10)
		preciosEsperados[cadena.Nombre] = precio
		lote := map[string]any{"sucursal_id": sucursal.ID, "productos": []map[string]any{{
			"sku": "CATCOM-" + sufijo, "ean_gtin": ean13, "producto": nombreProducto,
			"categoria": categoria, "formato_crudo": "1 L", "precio_normal": precio,
		}}}
		var resultado domain.ResultadoIngestaDTO
		if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/ingestas", tokenIngestaPostgres, nil, lote, &resultado); estado != http.StatusOK || resultado.Insertados != 1 {
			prueba.Fatalf("ingesta %s=%d %+v", nombreCadena, estado, resultado)
		}
	}

	var pagina domain.PaginaProductosDTO
	if estado := pedir(prueba, enrutador, http.MethodGet, "/api/v1/productos?q=Categoria+Comun+"+sufijo+"&limit=50", "", nil, nil, &pagina); estado != http.StatusOK {
		prueba.Fatalf("catálogo público=%d", estado)
	}
	if pagina.TotalRegistros != int64(len(cadenas)) || len(pagina.Data) != len(cadenas) {
		prueba.Fatalf("se esperaban %d productos, se obtuvieron %d: %+v", len(cadenas), pagina.TotalRegistros, pagina)
	}
	ids := make([]string, 0, len(pagina.Data))
	for _, producto := range pagina.Data {
		ids = append(ids, producto.ID)
	}

	var respuesta domain.RespuestaEquivalenciaDTO
	// El contrato devuelve un grupo por cada ID solicitado, aunque todos
	// pertenezcan al mismo EAN.
	if estado := pedir(prueba, enrutador, http.MethodPost, "/internal/v1/productos/equivalencias", tokenConsultaPostgres, nil,
		map[string]any{"producto_ids": ids}, &respuesta); estado != http.StatusOK || len(respuesta.Productos) != len(cadenas) {
		prueba.Fatalf("equivalencias=%d %+v", estado, respuesta)
	}
	for _, item := range respuesta.Productos {
		if item.Estado != domain.EstadoEquivalenciaComparable || item.EAN == nil || *item.EAN != canonico || len(item.Ofertas) != len(cadenas) {
			prueba.Fatalf("grupo de categoría común incorrecto: %+v", item)
		}
		preciosVistos := make(map[string]float64, len(cadenas))
		for _, oferta := range item.Ofertas {
			if oferta.EstadoPrecio != domain.EstadoPrecioVigente || oferta.Precio == nil {
				prueba.Fatalf("oferta sin precio vigente: %+v", oferta)
			}
			preciosVistos[oferta.Cadena] = *oferta.Precio
		}
		for _, nombreCadena := range cadenas {
			if precio, encontrado := preciosVistos[nombreCadena]; !encontrado || precio != preciosEsperados[nombreCadena] {
				prueba.Fatalf("cadena %s: precio=%v encontrado=%t esperado=%v", nombreCadena, precio, encontrado, preciosEsperados[nombreCadena])
			}
		}
	}
}
