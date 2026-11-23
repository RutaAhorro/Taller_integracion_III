// Pruebas del servicio sobre los supermercados disponibles (SUP-371): el
// listado público los incluye siempre como lista (nunca null), los calcula con
// el filtro normalizado completo y propaga los errores del repositorio.
package unit_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// TestProductoService_SupermercadosIncluidosYNoNulos comprueba que la página
// lleva las cadenas del repositorio y que sin coincidencias es una lista vacía.
func TestProductoService_SupermercadosIncluidosYNoNulos(prueba *testing.T) {
	casos := []struct {
		nombre       string
		repositorio  []domain.SupermercadoDTO
		esperadoLong int
	}{
		{"con cadenas", []domain.SupermercadoDTO{{ID: 1, Nombre: "Jumbo"}, {ID: 2, Nombre: "Lider"}}, 2},
		{"nil se convierte en lista vacía", nil, 0},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &mockProductoRepository{supermercados: caso.repositorio}
			servicio := services.NuevoServicioProductos(repositorio, limitesProductosDePrueba)
			pagina, errorObtenido := servicio.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{})
			if errorObtenido != nil {
				subprueba.Fatalf("error inesperado: %v", errorObtenido)
			}
			if pagina.Supermercados == nil || len(pagina.Supermercados) != caso.esperadoLong {
				subprueba.Fatalf("supermercados inesperados: %#v", pagina.Supermercados)
			}
		})
	}
}

// TestProductoService_SupermercadosRecibenFiltroNormalizado comprueba que la
// agregación recibe el mismo filtro que el listado (búsqueda saneada, stock,
// cadena y página), de modo que el repositorio decida qué omitir.
func TestProductoService_SupermercadosRecibenFiltroNormalizado(prueba *testing.T) {
	enStock := true
	repositorio := &mockProductoRepository{}
	servicio := services.NuevoServicioProductos(repositorio, limitesProductosDePrueba)
	_, errorObtenido := servicio.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{
		Query: " leche<> ", EnStock: &enStock, Supermercado: "Lider", Categoria: "Lacteos", Page: 3, Limit: 5,
	})
	if errorObtenido != nil {
		prueba.Fatalf("error inesperado: %v", errorObtenido)
	}
	recibido := repositorio.filtroSupermercados
	if recibido.Query != "leche" || recibido.EnStock == nil || !*recibido.EnStock ||
		recibido.Supermercado != "Lider" || recibido.Categoria != "Lacteos" {
		prueba.Fatalf("filtro de supermercados distinto del listado: %+v", recibido)
	}
	if !reflect.DeepEqual(recibido, repositorio.lastFilter) {
		prueba.Fatalf("filtros divergentes: %+v frente a %+v", recibido, repositorio.lastFilter)
	}
}

// TestProductoService_ErrorDeSupermercadosSePropaga comprueba que un fallo de
// la agregación no devuelve una página con filtro lateral incompleto. Listar
// responde bien, así que el error sólo puede venir de ListarSupermercados.
func TestProductoService_ErrorDeSupermercadosSePropaga(prueba *testing.T) {
	errorAgregacion := errors.New("falló la agregación de supermercados")
	repositorio := &mockProductoRepository{
		items:              []domain.ProductoDTO{{ID: "producto-1"}},
		total:              1,
		errorSupermercados: errorAgregacion,
	}
	servicio := services.NuevoServicioProductos(repositorio, limitesProductosDePrueba)
	pagina, errorObtenido := servicio.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{})
	if !errors.Is(errorObtenido, errorAgregacion) {
		prueba.Fatalf("se esperaba el error de la agregación, se obtuvo %v", errorObtenido)
	}
	if pagina != nil {
		prueba.Fatal("no debe devolverse una página con el filtro lateral incompleto")
	}
}
