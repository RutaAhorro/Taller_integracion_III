// Pruebas de la caché de supermercados disponibles del listado público: se
// reutiliza entre páginas y órdenes de un mismo filtro, se separa por filtro,
// no guarda errores, no expone su contenido a modificaciones, vence con el
// plazo, libera espacio al llenarse, soporta concurrencia y se desactiva con
// vigencia 0.
package unit_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
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

// relojManual permite adelantar el tiempo de la caché sin esperar.
type relojManual struct{ ahora time.Time }

// Ahora devuelve el instante actual del reloj de prueba.
func (reloj *relojManual) Ahora() time.Time { return reloj.ahora }

// TestCacheSupermercados_VenceTrasLaVigencia comprueba que una entrada se
// reutiliza antes del plazo y se vuelve a consultar al cumplirse.
func TestCacheSupermercados_VenceTrasLaVigencia(prueba *testing.T) {
	const vigencia = 2 * time.Minute
	reloj := &relojManual{ahora: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	repositorio := &mockProductoRepository{}
	limites := limitesProductosDePrueba
	limites.VigenciaCacheSupermercados = vigencia
	limites.Reloj = reloj.Ahora
	servicio := services.NuevoServicioProductos(repositorio, limites)

	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	reloj.ahora = reloj.ahora.Add(vigencia - time.Second)
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	if repositorio.llamadasSupermercados != 1 {
		prueba.Fatalf("antes del plazo se esperaba 1 consulta, hubo %d", repositorio.llamadasSupermercados)
	}
	reloj.ahora = reloj.ahora.Add(time.Second)
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{})
	if repositorio.llamadasSupermercados != 2 {
		prueba.Fatalf("al cumplirse el plazo se esperaba otra consulta, hubo %d", repositorio.llamadasSupermercados)
	}
}

// entradasParaLlenarCache supera el máximo interno de 512 entradas de la caché.
const entradasParaLlenarCache = 513

// TestCacheSupermercados_LiberaEspacioAlLlenarse comprueba que al superar el
// máximo de entradas vigentes la caché se vacía y vuelve a consultar.
func TestCacheSupermercados_LiberaEspacioAlLlenarse(prueba *testing.T) {
	repositorio := &mockProductoRepository{}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)
	for indice := 0; indice < entradasParaLlenarCache; indice++ {
		pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{Query: fmt.Sprintf("busqueda%d", indice)})
	}
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{Query: "busqueda0"})
	if repositorio.llamadasSupermercados != entradasParaLlenarCache+1 {
		prueba.Fatalf("la primera entrada debía descartarse al llenarse: %d consultas", repositorio.llamadasSupermercados)
	}
}

// TestCacheSupermercados_FiltroNoSerializableNoSeGuarda comprueba que un
// precio NaN no comparte clave con otros filtros: consulta siempre.
func TestCacheSupermercados_FiltroNoSerializableNoSeGuarda(prueba *testing.T) {
	repositorio := &mockProductoRepository{}
	servicio := servicioConCache(repositorio, vigenciaCacheDePrueba)
	precioInvalido := math.NaN()
	precioInfinito := math.Inf(1)

	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{PrecioMin: &precioInvalido})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{PrecioMin: &precioInfinito})
	pedirCatalogo(prueba, servicio, domain.FiltroProductosDTO{PrecioMin: &precioInvalido})
	if repositorio.llamadasSupermercados != 3 {
		prueba.Fatalf("un filtro no serializable no debe usar la caché: %d consultas", repositorio.llamadasSupermercados)
	}
}

// repositorioConcurrente es un doble seguro para goroutines que sólo cuenta
// las consultas de supermercados.
type repositorioConcurrente struct {
	mockProductoRepository
	llamadas atomic.Int64
}

// Listar responde una página vacía sin compartir estado entre goroutines.
func (repositorio *repositorioConcurrente) Listar(context.Context, domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	return nil, 0, nil
}

// ListarSupermercados cuenta la consulta de forma atómica.
func (repositorio *repositorioConcurrente) ListarSupermercados(context.Context, domain.FiltroProductosDTO) ([]domain.SupermercadoDTO, error) {
	repositorio.llamadas.Add(1)
	return []domain.SupermercadoDTO{{ID: 1, Nombre: "Jumbo"}}, nil
}

// peticionesConcurrentes es la cantidad de goroutines que comparten la caché.
const peticionesConcurrentes = 50

// TestCacheSupermercados_PeticionesConcurrentes comprueba con -race que muchas
// peticiones simultáneas comparten la caché sin carreras y sin errores.
func TestCacheSupermercados_PeticionesConcurrentes(prueba *testing.T) {
	repositorio := &repositorioConcurrente{}
	limites := limitesProductosDePrueba
	limites.VigenciaCacheSupermercados = vigenciaCacheDePrueba
	servicio := services.NuevoServicioProductos(repositorio, limites)

	var grupo sync.WaitGroup
	errores := make(chan error, peticionesConcurrentes)
	for indice := 0; indice < peticionesConcurrentes; indice++ {
		grupo.Add(1)
		go func(indice int) {
			defer grupo.Done()
			filtro := domain.FiltroProductosDTO{Query: fmt.Sprintf("busqueda%d", indice%3)}
			pagina, errorObtenido := servicio.ObtenerCatalogo(context.Background(), filtro)
			if errorObtenido == nil && len(pagina.Supermercados) != 1 {
				errorObtenido = fmt.Errorf("supermercados inesperados: %#v", pagina.Supermercados)
			}
			errores <- errorObtenido
		}(indice)
	}
	grupo.Wait()
	close(errores)
	for errorObtenido := range errores {
		if errorObtenido != nil {
			prueba.Fatal(errorObtenido)
		}
	}
	if llamadas := repositorio.llamadas.Load(); llamadas < 3 || llamadas > peticionesConcurrentes {
		prueba.Fatalf("consultas fuera de rango: %d", llamadas)
	}
}
