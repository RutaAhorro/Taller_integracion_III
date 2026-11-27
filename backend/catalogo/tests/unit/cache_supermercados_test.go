// Pruebas de la caché de supermercados disponibles del listado público: se
// reutiliza entre páginas y órdenes de un mismo filtro, se separa por filtro,
// no guarda errores, no expone su contenido a modificaciones y se desactiva
// con vigencia 0.
package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// ===== Auxiliares =====

// vigenciaCacheDePrueba es mayor que cualquier prueba: nada vence durante ella.
const vigenciaCacheDePrueba = time.Hour

// servicioConCache crea el servicio con la caché de supermercados encendida.
func servicioConCache(repositorio *mockProductoRepository, vigencia time.Duration) *services.ServicioProductos {
	limites := limitesProductosDePrueba
	limites.VigenciaCacheSupermercados = vigencia
	return services.NuevoServicioProductos(repositorio, limites)
}

// pedirCatalogo consulta el listado y falla la prueba ante un error.
func pedirCatalogo(prueba *testing.T, servicio *services.ServicioProductos, filtro domain.FiltroProductosDTO) *domain.PaginaProductosDTO {
	prueba.Helper()
	pagina, errorObtenido := servicio.ObtenerCatalogo(context.Background(), filtro)
	if errorObtenido != nil {
		prueba.Fatalf("error inesperado: %v", errorObtenido)
	}
	return pagina
}

// ===== Pruebas =====

// TestCacheSupermercados_ReutilizaEntrePaginasYOrden comprueba que cambiar de
// página, de orden o de cadena elegida no repite la consulta de cadenas.
func TestCacheSupermercados_ReutilizaEntrePaginasYOrden(prueba *testing.T) {
	enStock := true
	repositorio := &mockProductoRepository{supermercados: []domain.SupermercadoDTO{{ID: 1, Nombre: "Jumbo"}}}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)

	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{EnStock: &enStock, Page: 1})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{EnStock: &enStock, Page: 4, Limit: 50})
	pagina := pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{EnStock: &enStock, SortBy: "nombre", Order: "desc", Supermercado: "Jumbo"})

	if repositorio.llamadasSupermercados != 1 {
		prueba.Fatalf("se esperaba 1 consulta de supermercados, hubo %d", repositorio.llamadasSupermercados)
	}
	if len(pagina.Supermercados) != 1 || pagina.Supermercados[0].Nombre != "Jumbo" {
		prueba.Fatalf("supermercados inesperados desde la caché: %#v", pagina.Supermercados)
	}
}

// TestCacheSupermercados_SeparaFiltrosDistintos comprueba que una búsqueda,
// stock u oferta distintos no reciben las cadenas de otro filtro.
func TestCacheSupermercados_SeparaFiltrosDistintos(prueba *testing.T) {
	enStock, enOferta := true, true
	repositorio := &mockProductoRepository{}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)

	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{EnStock: &enStock})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{EnStock: &enStock, EnOferta: &enOferta})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{Query: "leche"})

	if repositorio.llamadasSupermercados != 4 {
		prueba.Fatalf("se esperaban 4 consultas (una por filtro), hubo %d", repositorio.llamadasSupermercados)
	}
}

// TestCacheSupermercados_NoGuardaErrores comprueba que un fallo se propaga y
// que la petición siguiente vuelve a consultar.
func TestCacheSupermercados_NoGuardaErrores(prueba *testing.T) {
	errorAgregacion := errors.New("falló la agregación de supermercados")
	repositorio := &mockProductoRepository{errorSupermercados: errorAgregacion}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)

	if _, errorObtenido := servicio.ObtenerCatalogo(context.Background(), domain.FiltroProductosDTO{}); !errors.Is(errorObtenido, errorAgregacion) {
		prueba.Fatalf("se esperaba el error de la agregación, se obtuvo %v", errorObtenido)
	}
	repositorio.errorSupermercados = nil
	repositorio.supermercados = []domain.SupermercadoDTO{{ID: 2, Nombre: "Lider"}}
	pagina := pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})

	if repositorio.llamadasSupermercados != 2 || len(pagina.Supermercados) != 1 {
		prueba.Fatalf("el error quedó en caché: %d consultas, %#v", repositorio.llamadasSupermercados, pagina.Supermercados)
	}
}

// TestCacheSupermercados_DevuelveCopias comprueba que modificar una respuesta
// no altera lo guardado para la petición siguiente.
func TestCacheSupermercados_DevuelveCopias(prueba *testing.T) {
	repositorio := &mockProductoRepository{supermercados: []domain.SupermercadoDTO{{ID: 1, Nombre: "Jumbo"}}}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)

	primera := pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	primera.Supermercados[0].Nombre = "modificado"
	segunda := pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})

	if segunda.Supermercados[0].Nombre != "Jumbo" {
		prueba.Fatalf("la caché expuso su contenido: %#v", segunda.Supermercados)
	}
}

// TestCacheSupermercados_VigenciaCeroConsultaSiempre comprueba que con
// vigencia 0 cada petición consulta y que nil sigue siendo lista vacía.
func TestCacheSupermercados_VigenciaCeroConsultaSiempre(prueba *testing.T) {
	repositorio := &mockProductoRepository{}
	servicio := servicioConCache(repositorio, 0)

	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	pagina := pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})

	if repositorio.llamadasSupermercados != 2 {
		prueba.Fatalf("sin caché se esperaban 2 consultas, hubo %d", repositorio.llamadasSupermercados)
	}
	if pagina.Supermercados == nil {
		prueba.Fatal("sin caché los supermercados deben ser [] y no null")
	}
}
