package spool

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
)

// Los valores esperados fueron generados con el mismo json.dumps que usa
// scraper_core/outbox.py (resultados/gen_fingerprints.py). Si cambian, el
// rollback a Python dejaría de reutilizar la restricción UNIQUE.
func TestCanonicalProductJSONMatchesPython(t *testing.T) {
	full := domain.Product{
		Producto:          "Leche Entera Colún 1 L",
		Precio:            domain.FloatPtr(1290),
		PrecioNormal:      domain.FloatPtr(1590),
		PrecioOferta:      domain.FloatPtr(1290),
		EanGtin:           domain.StringPtr("7800000000001"),
		SKU:               domain.StringPtr("SKU-1"),
		Marca:             domain.StringPtr("Colún"),
		FormatoCrudo:      domain.StringPtr("1 L"),
		MecanicaPromocion: domain.StringPtr("2x1"),
		EnStock:           domain.BoolPtr(true),
		URLProducto:       domain.StringPtr("https://www.jumbo.cl/leche-colun/p"),
		Imagen:            domain.StringPtr("https://img.test/leche.jpg"),
		Categoria:         "lacteos",
	}
	tests := []struct {
		name    string
		product domain.Product
		market  *string
		want    string
		hash    string
	}{
		{
			name:    "completo con acentos y floats enteros",
			product: full,
			market:  domain.StringPtr("Jumbo"),
			want: `{"categoria":"lacteos","ean_gtin":"7800000000001","en_stock":true,` +
				`"formato_crudo":"1 L","imagen":"https://img.test/leche.jpg","marca":"Colún",` +
				`"mecanica_promocion":"2x1","precio":1290.0,"precio_normal":1590.0,` +
				`"precio_oferta":1290.0,"producto":"Leche Entera Colún 1 L","sku":"SKU-1",` +
				`"url_producto":"https://www.jumbo.cl/leche-colun/p"}`,
			hash: "13a22d59149e06e9cdca5545c47b89ee5e7428b2996fd17bdc60da5c8c7cac57",
		},
		{
			name:    "todos los opcionales nulos",
			product: domain.Product{Producto: "Pan", Categoria: "panaderia"},
			market:  domain.StringPtr("Lider"),
			want: `{"categoria":"panaderia","ean_gtin":null,"en_stock":null,"formato_crudo":null,` +
				`"imagen":null,"marca":null,"mecanica_promocion":null,"precio":null,` +
				`"precio_normal":null,"precio_oferta":null,"producto":"Pan","sku":null,` +
				`"url_producto":null}`,
			hash: "44a5ce4dc7a0438a90c142ab157fbeef9b0587ff00518eea5cb62d9ee83c4882",
		},
		{
			name: "comillas y sin supermercado",
			product: domain.Product{
				Producto:     `Aceite "Girasol" 900 ml`,
				Precio:       domain.FloatPtr(2499.5),
				PrecioNormal: domain.FloatPtr(2499.5),
				Marca:        domain.StringPtr("Chef"),
				FormatoCrudo: domain.StringPtr("900 ml"),
				EnStock:      domain.BoolPtr(false),
				URLProducto:  domain.StringPtr("https://www.santaisabel.cl/aceite/p"),
				Categoria:    "aceites",
			},
			market: nil,
			want: `{"categoria":"aceites","ean_gtin":null,"en_stock":false,"formato_crudo":"900 ml",` +
				`"imagen":null,"marca":"Chef","mecanica_promocion":null,"precio":2499.5,` +
				`"precio_normal":2499.5,"precio_oferta":null,"producto":"Aceite \"Girasol\" 900 ml",` +
				`"sku":null,"url_producto":"https://www.santaisabel.cl/aceite/p"}`,
			hash: "cc58b2b46403d8399caf95892df6dd72745e03a30edb7da571ec9c411f36329b",
		},
		{
			name: "notación científica de Python",
			product: domain.Product{
				Producto:     "Queso Gauda 250 g",
				Precio:       domain.FloatPtr(0.5),
				PrecioNormal: domain.FloatPtr(1e16),
				PrecioOferta: domain.FloatPtr(0.00001),
				Categoria:    "lacteos",
			},
			market: domain.StringPtr("A Cuenta"),
			want: `{"categoria":"lacteos","ean_gtin":null,"en_stock":null,"formato_crudo":null,` +
				`"imagen":null,"marca":null,"mecanica_promocion":null,"precio":0.5,` +
				`"precio_normal":1e+16,"precio_oferta":1e-05,"producto":"Queso Gauda 250 g",` +
				`"sku":null,"url_producto":null}`,
			hash: "ca2d572215950e482767f2c8a3668b44439a8518c313a2b257e002984fee3a51",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := canonicalProductJSON(test.product)
			if err != nil {
				t.Fatalf("canonicalProductJSON() error = %v", err)
			}
			if string(data) != test.want {
				t.Errorf("producto canónico =\n%s\nwant\n%s", data, test.want)
			}
			hash, err := pythonFingerprint(test.market, data)
			if err != nil {
				t.Fatalf("pythonFingerprint() error = %v", err)
			}
			if hash != test.hash {
				t.Errorf("fingerprint = %s, want %s", hash, test.hash)
			}
		})
	}
}

// Un rollback a Python deja filas escritas por outbox.py. Go debe reconocerlas
// como el mismo producto y no insertar una segunda captura.
func TestAddDeduplicatesRowsWrittenByPython(t *testing.T) {
	store, err := Open(uuid.NewString(), t.TempDir(), 1<<20)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	product := domain.Product{
		Producto:  "Leche Entera Colún 1 L",
		Precio:    domain.FloatPtr(1290),
		Marca:     domain.StringPtr("Colún"),
		Categoria: "lacteos",
	}
	market := "Jumbo"
	data, err := canonicalProductJSON(product)
	if err != nil {
		t.Fatalf("canonicalProductJSON() error = %v", err)
	}
	fingerprint, err := pythonFingerprint(&market, data)
	if err != nil {
		t.Fatalf("pythonFingerprint() error = %v", err)
	}

	// Fila tal como la escribió outbox.py antes del corte a Go.
	if _, err := store.db.Exec(
		"INSERT INTO items(fingerprint, market, product) VALUES (?, ?, ?)",
		fingerprint, market, string(data),
	); err != nil {
		t.Fatalf("insertar fila Python: %v", err)
	}
	if err := store.Add(product, &market); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	var (
		count       int
		stored      string
		storedPrint sql.NullString
	)
	if err := store.db.QueryRow("SELECT count(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("contar items: %v", err)
	}
	if count != 1 {
		t.Fatalf("items = %d, want 1 (la huella de Python debe aplicar UNIQUE)", count)
	}
	if err := store.db.QueryRow("SELECT product, fingerprint FROM items").Scan(&stored, &storedPrint); err != nil {
		t.Fatalf("leer item: %v", err)
	}
	if stored != string(data) {
		t.Errorf("producto almacenado = %s", stored)
	}
	if !storedPrint.Valid || storedPrint.String != fingerprint {
		t.Errorf("fingerprint almacenado = %v, want %s", storedPrint, fingerprint)
	}
}
