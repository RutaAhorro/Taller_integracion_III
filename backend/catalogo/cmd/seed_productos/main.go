package main

import (
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type productoSemilla struct {
	SucursalID   int
	SKU          string
	Titulo       string
	Marca        string
	Categoria    string
	Formato      string
	URLImagen    string
	PrecioNormal float64
	PrecioOferta *float64
}

func main() {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("DB_URL es obligatorio para ejecutar semillas de desarrollo")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("No se pudo conectar a PostgreSQL para ejecutar las semillas")
	}

	log.Println("Limpiando semillas previas de productos...")
	db.Exec("DELETE FROM scraper.capturas_precios WHERE producto_crudo_id IN (SELECT id FROM scraper.productos_crudos WHERE sku LIKE 'SEED-PROD-%')")
	db.Exec("DELETE FROM scraper.productos_crudos WHERE sku LIKE 'SEED-PROD-%'")

	oferta1 := 990.00
	oferta3 := 1890.00
	oferta4 := 1390.00
	oferta6 := 4190.00
	oferta7 := 1290.00
	oferta9 := 8990.00
	oferta10 := 790.00

	semillas := []productoSemilla{
		// 1. Lider - Lácteos - Oferta
		{
			SucursalID:   2,
			SKU:          "SEED-PROD-001",
			Titulo:       "Leche Entera Colun 1L",
			Marca:        "Colun",
			Categoria:    "Lácteos",
			Formato:      "1 L",
			URLImagen:    "https://images.unsplash.com/photo-1550583724-b2692b85b150",
			PrecioNormal: 1290.00,
			PrecioOferta: &oferta1,
		},
		// 2. Lider - Lácteos - Precio normal
		{
			SucursalID:   2,
			SKU:          "SEED-PROD-002",
			Titulo:       "Yogur Batido Frutilla Soprole 120g",
			Marca:        "Soprole",
			Categoria:    "Lácteos",
			Formato:      "120 g",
			URLImagen:    "https://images.unsplash.com/photo-1488477181946-6428a0291777",
			PrecioNormal: 450.00,
			PrecioOferta: nil,
		},
		// 3. Lider - Panadería - Oferta
		{
			SucursalID:   2,
			SKU:          "SEED-PROD-003",
			Titulo:       "Pan de Molde Blanco Ideal 560g",
			Marca:        "Ideal",
			Categoria:    "Panadería",
			Formato:      "560 g",
			URLImagen:    "https://images.unsplash.com/photo-1509440159596-0249088772ff",
			PrecioNormal: 2490.00,
			PrecioOferta: &oferta3,
		},
		// 4. Unimarc - Despensa - Oferta
		{
			SucursalID:   3,
			SKU:          "SEED-PROD-004",
			Titulo:       "Arroz Grado 1 Selección Tucapel 1kg",
			Marca:        "Tucapel",
			Categoria:    "Despensa",
			Formato:      "1 kg",
			URLImagen:    "https://images.unsplash.com/photo-1586201375761-83865001e31c",
			PrecioNormal: 1890.00,
			PrecioOferta: &oferta4,
		},
		// 5. Unimarc - Despensa - Precio normal
		{
			SucursalID:   3,
			SKU:          "SEED-PROD-005",
			Titulo:       "Aceite Vegetal Maravilla Chef 900ml",
			Marca:        "Chef",
			Categoria:    "Despensa",
			Formato:      "900 ml",
			URLImagen:    "https://images.unsplash.com/photo-1474979266404-7eaacbcd87c5",
			PrecioNormal: 2790.00,
			PrecioOferta: nil,
		},
		// 6. Unimarc - Despensa - Oferta
		{
			SucursalID:   3,
			SKU:          "SEED-PROD-006",
			Titulo:       "Café Instantáneo Nescafé Tradición 170g",
			Marca:        "Nescafé",
			Categoria:    "Despensa",
			Formato:      "170 g",
			URLImagen:    "https://images.unsplash.com/photo-1514432324607-a09d9b4aefdd",
			PrecioNormal: 5490.00,
			PrecioOferta: &oferta6,
		},
		// 7. Santa Isabel - Bebidas - Oferta
		{
			SucursalID:   4,
			SKU:          "SEED-PROD-007",
			Titulo:       "Bebida Coca-Cola Sabor Original 1.5L",
			Marca:        "Coca-Cola",
			Categoria:    "Bebidas",
			Formato:      "1.5 L",
			URLImagen:    "https://images.unsplash.com/photo-1622483767028-3f66f32aef97",
			PrecioNormal: 1790.00,
			PrecioOferta: &oferta7,
		},
		// 8. Santa Isabel - Lácteos - Precio normal
		{
			SucursalID:   4,
			SKU:          "SEED-PROD-008",
			Titulo:       "Queso Gauda Laminado Colun 500g",
			Marca:        "Colun",
			Categoria:    "Lácteos",
			Formato:      "500 g",
			URLImagen:    "https://images.unsplash.com/photo-1486297678162-eb2a19b0a32d",
			PrecioNormal: 4990.00,
			PrecioOferta: nil,
		},
		// 9. Santa Isabel - Limpieza - Oferta
		{
			SucursalID:   4,
			SKU:          "SEED-PROD-009",
			Titulo:       "Detergente Líquido Omo Matic 3L",
			Marca:        "Omo",
			Categoria:    "Limpieza",
			Formato:      "3 L",
			URLImagen:    "https://images.unsplash.com/photo-1585421514738-01798e348b17",
			PrecioNormal: 12990.00,
			PrecioOferta: &oferta9,
		},
		// 10. Jumbo - Snacks - Oferta
		{
			SucursalID:   1,
			SKU:          "SEED-PROD-010",
			Titulo:       "Galletas Oreo Original 126g",
			Marca:        "Oreo",
			Categoria:    "Snacks",
			Formato:      "126 g",
			URLImagen:    "https://images.unsplash.com/photo-1558961363-fa8fdf82db35",
			PrecioNormal: 1090.00,
			PrecioOferta: &oferta10,
		},
	}

	log.Printf("Insertando %d productos de prueba variados...", len(semillas))

	for _, s := range semillas {
		prod := productoCrudoSemilla{
			ID:                 uuid.New(),
			SucursalID:         s.SucursalID,
			SKU:                s.SKU,
			TituloCrudo:        s.Titulo,
			MarcaCruda:         &s.Marca,
			CategoriaCruda:     &s.Categoria,
			FormatoCrudo:       &s.Formato,
			URLImagen:          &s.URLImagen,
			EnStock:            true,
			UltimaExtraccionEl: time.Now(),
		}

		if err := db.Create(&prod).Error; err != nil {
			log.Fatalf("Error insertando producto %s: %v", s.SKU, err)
		}

		// Captura actual
		capturaActual := capturaPrecioSemilla{
			ProductoCrudoID: prod.ID,
			PrecioNormal:    s.PrecioNormal,
			PrecioOferta:    s.PrecioOferta,
			EstaDisponible:  true,
			CapturadoEl:     time.Now(),
		}

		if err := db.Create(&capturaActual).Error; err != nil {
			log.Fatalf("Error insertando captura de precio para %s: %v", s.SKU, err)
		}

		// Para SEED-PROD-001 inyectamos capturas históricas anteriores para pruebas de gráficas
		if s.SKU == "SEED-PROD-001" {
			precioViejo1 := 1350.00
			capturaVieja1 := capturaPrecioSemilla{
				ProductoCrudoID: prod.ID,
				PrecioNormal:    precioViejo1,
				PrecioOferta:    nil,
				EstaDisponible:  true,
				CapturadoEl:     time.Now().Add(-7 * 24 * time.Hour),
			}
			_ = db.Create(&capturaVieja1)

			precioViejo2 := 1390.00
			capturaVieja2 := capturaPrecioSemilla{
				ProductoCrudoID: prod.ID,
				PrecioNormal:    precioViejo2,
				PrecioOferta:    nil,
				EstaDisponible:  true,
				CapturadoEl:     time.Now().Add(-14 * 24 * time.Hour),
			}
			_ = db.Create(&capturaVieja2)
		}
	}

	log.Println("10 productos de prueba inyectados exitosamente con historial de precios para pruebas.")
}
