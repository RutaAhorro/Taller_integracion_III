// Pruebas de la ingesta masiva de lotes contra un PostgreSQL real y aislado
// (CATALOGO_TEST_DB_URL, rol app_catalogo): SKU repetidos dentro del lote,
// ítems inválidos, lotes mayores que el bloque de escritura, reingesta,
// idempotencia y reversión. Se omiten sin la variable; nunca apuntarlas a una
// base compartida.
package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== Preparación =====

// productosMayoresQueElBloque supera dos bloques de escritura (5000 filas cada
// uno en repositories.tamanoBloqueIngesta) para ejercitar el troceo.
const productosMayoresQueElBloque = 11000

// Coordenadas arbitrarias de la sucursal desechable de cada prueba.
const (
	latitudPrueba  = -38.7
	longitudPrueba = -72.6
)

// entornoLote agrupa lo necesario para ingestar y verificar contra la base.
type entornoLote struct {
	baseDatos   *gorm.DB
	repositorio *repositories.RepositorioIngesta
	sucursalID  int
	sufijo      string
}

// prepararEntornoLote abre la base, crea una cadena y una sucursal únicas y
// devuelve el entorno. Se omite si no hay base de pruebas configurada.
func prepararEntornoLote(prueba *testing.T) entornoLote {
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
	repositorio := repositories.NuevoRepositorioIngesta(baseDatos)
	sufijo := uuid.NewString()[:8]
	cadena, errorCadena := repositorio.ObtenerCadena(context.Background(), 0, "Cadena Lote "+sufijo)
	if errorCadena != nil {
		prueba.Fatal(errorCadena)
	}
	// La sucursal se crea explícitamente: la ingesta no autocrea sucursales
	// (SUP-376), así que la prueba no depende de ese comportamiento.
	codigo := "LOTE-" + sufijo
	sucursal := domain.SucursalSupermercado{
		CadenaID: cadena.ID, CodigoSucursal: &codigo, Nombre: "Sucursal Lote " + sufijo,
		Direccion: "Dirección de prueba", Lat: latitudPrueba, Lon: longitudPrueba, EstaActiva: true,
	}
	if errorSucursal := baseDatos.Create(&sucursal).Error; errorSucursal != nil {
		prueba.Fatalf("sucursal no creada: %v", errorSucursal)
	}
	return entornoLote{baseDatos: baseDatos, repositorio: repositorio, sucursalID: sucursal.ID, sufijo: sufijo}
}

// ingestar envía un lote sin clave de idempotencia salvo que se indique.
func (entorno entornoLote) ingestar(prueba *testing.T, loteID string, productos []domain.ProductoScrapeadoDTO) *domain.ResultadoIngestaDTO {
	prueba.Helper()
	resultado, errorIngesta := entorno.repositorio.IngestarLote(context.Background(), domain.LoteIngestaDTO{
		LoteID: loteID, SucursalID: entorno.sucursalID, Productos: productos,
	})
	if errorIngesta != nil {
		prueba.Fatalf("ingesta: %v", errorIngesta)
	}
	return resultado
}

// contar devuelve cuántos productos y capturas tiene la sucursal de prueba.
func (entorno entornoLote) contar(prueba *testing.T) (productos, capturas int64) {
	prueba.Helper()
	entorno.baseDatos.Raw(`SELECT count(*) FROM scraper.productos_crudos WHERE sucursal_id = ?`, entorno.sucursalID).Scan(&productos)
	entorno.baseDatos.Raw(`SELECT count(*) FROM scraper.capturas_precios captura
		JOIN scraper.productos_crudos producto ON producto.id = captura.producto_crudo_id WHERE producto.sucursal_id = ?`, entorno.sucursalID).Scan(&capturas)
	return productos, capturas
}

// filaProducto es el estado persistido de un producto para las aserciones.
type filaProducto struct {
	TituloCrudo string
	MarcaCruda  *string
	EANGTIN     *string `gorm:"column:ean_gtin"`
	EnStock     bool
}

// producto lee el producto por SKU.
func (entorno entornoLote) producto(prueba *testing.T, sku string) filaProducto {
	prueba.Helper()
	var fila filaProducto
	consulta := entorno.baseDatos.Raw(`SELECT titulo_crudo, marca_cruda, ean_gtin, en_stock
		FROM scraper.productos_crudos WHERE sucursal_id = ? AND sku = ?`, entorno.sucursalID, sku).Scan(&fila)
	if consulta.Error != nil || consulta.RowsAffected != 1 {
		prueba.Fatalf("producto %s no encontrado: %v", sku, consulta.Error)
	}
	return fila
}

// texto devuelve un puntero a la cadena para construir DTO opcionales.
func texto(valor string) *string { return &valor }

// ===== Casos =====

// Un SKU repetido en el lote se escribe una vez, registra una captura por
// aparición y cuenta como el procesamiento secuencial anterior.
func TestPostgresLoteConSKUDuplicado(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	sinStock := false
	skuRepetido, skuUnico := "DUP-"+entorno.sufijo, "UNI-"+entorno.sufijo
	lote := []domain.ProductoScrapeadoDTO{
		{SKU: skuRepetido, Producto: "Primero", Marca: texto("MarcaOriginal"), PrecioNormal: 100},
		{SKU: skuUnico, Producto: "Unico", PrecioNormal: 50},
		// El último gana en título y stock; marca vacía conserva la anterior; EAN nuevo se suma.
		{SKU: " " + skuRepetido + " ", Producto: " Segundo ", Marca: texto(""), EANGTIN: texto("789"), PrecioNormal: 90, EnStock: &sinStock},
	}
	resultado := entorno.ingestar(prueba, "", lote)
	if resultado.TotalRecibidos != 3 || resultado.Insertados != 2 || resultado.Actualizados != 1 || resultado.PreciosRegistrados != 3 {
		prueba.Fatalf("conteos inesperados: %+v", resultado)
	}
	fila := entorno.producto(prueba, skuRepetido)
	if fila.TituloCrudo != "Segundo" || fila.EnStock || fila.MarcaCruda == nil || *fila.MarcaCruda != "MarcaOriginal" || fila.EANGTIN == nil || *fila.EANGTIN != "789" {
		prueba.Fatalf("fusión de duplicado incorrecta: %+v", fila)
	}
	if productos, capturas := entorno.contar(prueba); productos != 2 || capturas != 3 {
		prueba.Fatalf("productos=%d capturas=%d; se esperaba 2 y 3", productos, capturas)
	}

	// Con el SKU ya existente, todas sus apariciones son actualizaciones.
	resultado = entorno.ingestar(prueba, "", lote)
	if resultado.Insertados != 0 || resultado.Actualizados != 3 || resultado.PreciosRegistrados != 3 {
		prueba.Fatalf("conteos de reingesta con duplicado inesperados: %+v", resultado)
	}
	if productos, capturas := entorno.contar(prueba); productos != 2 || capturas != 6 {
		prueba.Fatalf("productos=%d capturas=%d; se esperaba 2 y 6", productos, capturas)
	}
}

// Los ítems inválidos se informan con su índice original sin abortar el lote.
func TestPostgresLoteConItemsInvalidosMezclados(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	lote := []domain.ProductoScrapeadoDTO{
		{SKU: "", Producto: "Sin sku", PrecioNormal: 1},
		{SKU: "OK1-" + entorno.sufijo, Producto: "Bueno", PrecioNormal: 10, Precio: ptrFlotante(8)},
		{SKU: "OK2-" + entorno.sufijo, Producto: "  ", PrecioNormal: 1},
		{SKU: "OK3-" + entorno.sufijo, Producto: "Otro", PrecioNormal: 20},
	}
	resultado := entorno.ingestar(prueba, "", lote)
	esperados := []string{"item 0: SKU o título vacío", "item 2: SKU o título vacío"}
	if resultado.TotalRecibidos != 4 || resultado.Insertados != 2 || resultado.Actualizados != 0 || resultado.PreciosRegistrados != 2 ||
		strings.Join(resultado.Errores, "|") != strings.Join(esperados, "|") {
		prueba.Fatalf("resultado inesperado: %+v", resultado)
	}
	if resultado.Mensaje != "Lote procesado: 2 insertados, 0 actualizados, 2 precios guardados" {
		prueba.Fatalf("mensaje inesperado: %q", resultado.Mensaje)
	}
	var oferta *float64
	entorno.baseDatos.Raw(`SELECT captura.precio_oferta FROM scraper.capturas_precios captura
		JOIN scraper.productos_crudos producto ON producto.id = captura.producto_crudo_id
		WHERE producto.sucursal_id = ? AND producto.sku = ?`, entorno.sucursalID, "OK1-"+entorno.sufijo).Scan(&oferta)
	if oferta == nil || *oferta != 8 {
		prueba.Fatalf("el alias precio debía ser oferta (8): %v", oferta)
	}
}

// Sin ítems válidos no se escribe ningún producto ni captura.
func TestPostgresLoteVacioOTodoInvalido(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	vacio := entorno.ingestar(prueba, "", nil)
	if vacio.TotalRecibidos != 0 || vacio.PreciosRegistrados != 0 || len(vacio.Errores) != 0 {
		prueba.Fatalf("lote vacío inesperado: %+v", vacio)
	}
	invalido := entorno.ingestar(prueba, uuid.NewString(), []domain.ProductoScrapeadoDTO{{SKU: " ", Producto: "x"}})
	if invalido.TotalRecibidos != 1 || invalido.Insertados != 0 || len(invalido.Errores) != 1 {
		prueba.Fatalf("lote inválido inesperado: %+v", invalido)
	}
	if productos, capturas := entorno.contar(prueba); productos != 0 || capturas != 0 {
		prueba.Fatalf("no debía escribirse nada: %d %d", productos, capturas)
	}
}

// Un lote mayor que tamanoBloqueIngesta se trocea sin perder conteos.
func TestPostgresLoteMayorQueElBloque(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	lote := make([]domain.ProductoScrapeadoDTO, 0, productosMayoresQueElBloque+1)
	for indice := range productosMayoresQueElBloque {
		lote = append(lote, domain.ProductoScrapeadoDTO{SKU: fmt.Sprintf("M%d-%s", indice, entorno.sufijo), Producto: "Masivo", PrecioNormal: float64(indice + 1)})
	}
	// Un duplicado del primero al final cruza fronteras de bloque tras deduplicar.
	lote = append(lote, domain.ProductoScrapeadoDTO{SKU: "M0-" + entorno.sufijo, Producto: "Masivo repetido", PrecioNormal: 1})
	resultado := entorno.ingestar(prueba, "", lote)
	if resultado.Insertados != productosMayoresQueElBloque || resultado.Actualizados != 1 || resultado.PreciosRegistrados != productosMayoresQueElBloque+1 {
		prueba.Fatalf("conteos inesperados: %+v", resultado)
	}
	if productos, capturas := entorno.contar(prueba); productos != productosMayoresQueElBloque || capturas != productosMayoresQueElBloque+1 {
		prueba.Fatalf("productos=%d capturas=%d", productos, capturas)
	}
	// Reingesta: todo actualizado.
	resultado = entorno.ingestar(prueba, "", lote[:productosMayoresQueElBloque])
	if resultado.Insertados != 0 || resultado.Actualizados != productosMayoresQueElBloque {
		prueba.Fatalf("reingesta inesperada: %+v", resultado)
	}
}

// Reingestar actualiza el producto y conserva los opcionales que llegan vacíos.
func TestPostgresReingestaActualizaYConservaOpcionales(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	sku := "RE-" + entorno.sufijo
	entorno.ingestar(prueba, "", []domain.ProductoScrapeadoDTO{{SKU: sku, Producto: "Viejo", Marca: texto("Marca"), PrecioNormal: 10}})
	resultado := entorno.ingestar(prueba, "", []domain.ProductoScrapeadoDTO{
		{SKU: sku, Producto: "Nuevo", PrecioNormal: 12},
		{SKU: "NU-" + entorno.sufijo, Producto: "Otro", PrecioNormal: 5},
	})
	if resultado.Insertados != 1 || resultado.Actualizados != 1 || resultado.PreciosRegistrados != 2 {
		prueba.Fatalf("conteos inesperados: %+v", resultado)
	}
	fila := entorno.producto(prueba, sku)
	if fila.TituloCrudo != "Nuevo" || fila.MarcaCruda == nil || *fila.MarcaCruda != "Marca" || !fila.EnStock {
		prueba.Fatalf("estado tras reingesta incorrecto: %+v", fila)
	}
	if _, capturas := entorno.contar(prueba); capturas != 3 {
		prueba.Fatalf("capturas=%d; se esperaban 3", capturas)
	}
}

// Repetir lote_id devuelve el recibo sin duplicar capturas; otro contenido falla.
func TestPostgresLoteIdempotentePorLoteID(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	loteID := uuid.NewString()
	productos := []domain.ProductoScrapeadoDTO{
		{SKU: "ID1-" + entorno.sufijo, Producto: "Uno", PrecioNormal: 1},
		{SKU: "ID1-" + entorno.sufijo, Producto: "Uno bis", PrecioNormal: 2},
	}
	primero := entorno.ingestar(prueba, loteID, productos)
	repetido := entorno.ingestar(prueba, loteID, productos)
	if repetido.Insertados != primero.Insertados || repetido.Actualizados != primero.Actualizados ||
		repetido.PreciosRegistrados != primero.PreciosRegistrados || repetido.Mensaje != primero.Mensaje {
		prueba.Fatalf("el recibo no coincide: %+v vs %+v", primero, repetido)
	}
	if _, capturas := entorno.contar(prueba); capturas != 2 {
		prueba.Fatalf("el reintento duplicó capturas: %d", capturas)
	}
	_, errorConflicto := entorno.repositorio.IngestarLote(context.Background(), domain.LoteIngestaDTO{LoteID: loteID, SucursalID: entorno.sucursalID, Productos: productos[:1]})
	if !errors.Is(errorConflicto, domain.ErrIdempotencia) {
		prueba.Fatalf("se esperaba ErrIdempotencia, llegó %v", errorConflicto)
	}
}

// Un error SQL en el INSERT masivo revierte productos, capturas y recibo.
func TestPostgresErrorSQLRevierteElLoteCompleto(prueba *testing.T) {
	entorno := prepararEntornoLote(prueba)
	loteID := uuid.NewString()
	// El SKU de 101 caracteres excede VARCHAR(100) y hace fallar el INSERT masivo.
	_, errorIngesta := entorno.repositorio.IngestarLote(context.Background(), domain.LoteIngestaDTO{
		LoteID: loteID, SucursalID: entorno.sucursalID, Productos: []domain.ProductoScrapeadoDTO{
			{SKU: "BUENO-" + entorno.sufijo, Producto: "Bueno", PrecioNormal: 1},
			{SKU: strings.Repeat("x", 101), Producto: "Malo", PrecioNormal: 1},
		},
	})
	if errorIngesta == nil {
		prueba.Fatal("se esperaba un error SQL")
	}
	if productos, capturas := entorno.contar(prueba); productos != 0 || capturas != 0 {
		prueba.Fatalf("el lote no se revirtió: %d %d", productos, capturas)
	}
	var recibos int64
	entorno.baseDatos.Raw(`SELECT count(*) FROM scraper.lotes_catalogo WHERE id = ?`, loteID).Scan(&recibos)
	if recibos != 0 {
		prueba.Fatalf("no debía quedar recibo: %d", recibos)
	}
}

// ptrFlotante devuelve un puntero al valor para los DTO con precios opcionales.
func ptrFlotante(valor float64) *float64 { return &valor }
