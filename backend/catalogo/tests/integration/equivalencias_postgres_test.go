// Prueba de extremo a extremo de la equivalencia por EAN contra un PostgreSQL
// real y aislado. Se omite si no se define CATALOGO_TEST_DB_URL; nunca debe
// apuntarse a una base compartida.
package integration_test

import (
	"net/http"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
)

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

	nombreLeche := "Leche Eq" + sufijo
	nombreConflicto := "Conflicto Eq" + sufijo
	nombreSinEAN := "SinEAN Eq" + sufijo
	loteOrigen := map[string]any{"sucursal_id": sucursalOrigen.ID, "productos": []map[string]any{
		{"sku": "LECHE-" + sufijo, "ean_gtin": "0036000291452", "producto": nombreLeche, "formato_crudo": "1 L", "precio_normal": 1290},
		{"sku": "CONFLICTO-" + sufijo, "ean_gtin": "7802900000004", "producto": nombreConflicto, "formato_crudo": "1 L", "precio_normal": 1500},
		{"sku": "SINEAN-" + sufijo, "producto": nombreSinEAN, "formato_crudo": "500 g", "precio_normal": 800},
	}}
	loteDestino := map[string]any{"sucursal_id": sucursalDestino.ID, "productos": []map[string]any{
		{"sku": "LECHE-" + sufijo, "ean_gtin": "036000291452", "producto": nombreLeche, "formato_crudo": "1 L", "precio_normal": 1190},
		{"sku": "CONFLICTO-" + sufijo, "ean_gtin": "7802900000004", "producto": nombreConflicto, "formato_crudo": "1.5 L", "precio_normal": 1900},
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
	if leche.Estado != domain.EstadoEquivalenciaComparable || leche.EAN == nil || *leche.EAN != "00036000291452" || len(leche.Ofertas) != 2 {
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
