// Pruebas de la entrega durable de lotes: límites, idempotencia, confirmación
// del contador y reprogramación ante fallos de Catálogo.
package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/tests/dobles"
	"github.com/google/uuid"
)

// ===== Preparación =====

// escenarioEntregas reúne los dobles de una prueba de ingesta.
type escenarioEntregas struct {
	trabajos *dobles.Trabajos
	catalogo *dobles.Catalogo
	entregas *dobles.Entregas
	servicio *services.ServicioEntregas
}

// nuevoEscenario crea la cadena 1 (Jumbo) con la sucursal 10.
func nuevoEscenario() escenarioEntregas {
	trabajos, catalogo := dobles.NuevosTrabajos(), dobles.NuevoCatalogo()
	catalogo.AgregarCadena(1, "Jumbo", true)
	catalogo.AgregarSucursal(10, 1)
	entregas := dobles.NuevasEntregas(trabajos)
	return escenarioEntregas{trabajos, catalogo, entregas, services.NuevoServicioEntregas(entregas, catalogo)}
}

// lote crea un lote de n productos con una clave nueva.
func lote(cantidad int) domain.IngestaLoteDTO {
	productos := make([]domain.ProductoScrapeadoDTO, cantidad)
	for indice := range productos {
		productos[indice] = domain.ProductoScrapeadoDTO{SKU: uuid.NewString(), Producto: "Producto", PrecioNormal: 1000}
	}
	return domain.IngestaLoteDTO{LoteID: uuid.NewString(), Productos: productos}
}

// ===== Validación y éxito =====

// TestIngestarValidaLimites rechaza lotes vacíos, excesivos o sin UUID válido.
func TestIngestarValidaLimites(prueba *testing.T) {
	escenario := nuevoEscenario()
	casos := map[string]struct {
		entrada  domain.IngestaLoteDTO
		esperado error
	}{
		"vacío":            {domain.IngestaLoteDTO{LoteID: uuid.NewString()}, domain.ErrLoteVacio},
		"excede el máximo": {lote(domain.MaximoProductosPorLote + 1), domain.ErrLoteExcedeMaximo},
		"clave inválida":   {domain.IngestaLoteDTO{LoteID: "x", Productos: lote(1).Productos}, domain.ErrUUIDInvalido},
	}
	for nombre, caso := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), nil, caso.entrada); !errors.Is(errorObtenido, caso.esperado) {
				prueba.Fatal(errorObtenido)
			}
		})
	}
	trabajoInvalido := "no-uuid"
	if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &trabajoInvalido, lote(1)); !errors.Is(errorObtenido, domain.ErrUUIDInvalido) {
		prueba.Fatal(errorObtenido)
	}
}

// TestIngestarConTrabajoConfirmaUnaVez suma el contador y reutiliza el resultado al reenviar.
func TestIngestarConTrabajoConfirmaUnaVez(prueba *testing.T) {
	escenario := nuevoEscenario()
	trabajoID := escenario.trabajos.Agregar(1, "Jumbo")
	texto := trabajoID.String()
	entrada := lote(3)
	resultado, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, entrada)
	if errorObtenido != nil || resultado.Insertados != 3 || resultado.SucursalID != 10 {
		prueba.Fatalf("resultado=%+v error=%v", resultado, errorObtenido)
	}
	// Un reenvío idéntico devuelve el resultado guardado sin volver a Catálogo.
	reenviado, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, entrada)
	if errorObtenido != nil || reenviado.Insertados != 3 {
		prueba.Fatalf("reenvío=%+v error=%v", reenviado, errorObtenido)
	}
	if escenario.catalogo.Ingestas != 1 || escenario.trabajos.Guardados[trabajoID].ElementosExtraidos != 3 {
		prueba.Fatalf("ingestas=%d contador=%d", escenario.catalogo.Ingestas, escenario.trabajos.Guardados[trabajoID].ElementosExtraidos)
	}
	otroContenido := lote(1)
	otroContenido.LoteID = entrada.LoteID
	if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, otroContenido); !errors.Is(errorObtenido, domain.ErrIdempotencia) {
		prueba.Fatalf("misma clave con otro contenido: %v", errorObtenido)
	}
}

// TestIngestarDirectoResuelveCadenaPorNombre usa el supermercado del lote para la sucursal.
func TestIngestarDirectoResuelveCadenaPorNombre(prueba *testing.T) {
	escenario := nuevoEscenario()
	entrada := lote(1)
	nombre := "jumbo"
	entrada.Supermercado = &nombre
	resultado, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), nil, entrada)
	if errorObtenido != nil || resultado.SucursalID != 10 || resultado.TrabajoID != nil {
		prueba.Fatalf("resultado=%+v error=%v", resultado, errorObtenido)
	}
}

// ===== Fallos =====

// TestIngestarFalloCatalogoReprograma no suma el contador y deja la entrega pendiente.
func TestIngestarFalloCatalogoReprograma(prueba *testing.T) {
	for _, errorCatalogo := range []error{domain.ErrCatalogoNoDisponible, domain.ErrCatalogoTimeout} {
		escenario := nuevoEscenario()
		trabajoID := escenario.trabajos.Agregar(1, "Jumbo")
		texto := trabajoID.String()
		escenario.catalogo.ErrorIngesta = errorCatalogo
		_, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, lote(1))
		reprogramaciones := escenario.entregas.Reprogramaciones
		if !errors.Is(errorObtenido, errorCatalogo) || escenario.trabajos.Guardados[trabajoID].ElementosExtraidos != 0 ||
			len(reprogramaciones) != 1 || reprogramaciones[0] != (dobles.Reprogramacion{Codigo: domain.CodigoCatalogoNoDisponible}) {
			prueba.Fatalf("error=%v reprogramaciones=%+v", errorObtenido, reprogramaciones)
		}
	}
}

// TestIngestarRechazoEsPermanente marca la entrega fallida ante un rechazo de datos.
func TestIngestarRechazoEsPermanente(prueba *testing.T) {
	escenario := nuevoEscenario()
	escenario.catalogo.ErrorIngesta = domain.ErrCatalogoRechazo
	nombre := "Jumbo"
	entrada := lote(1)
	entrada.Supermercado = &nombre
	_, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), nil, entrada)
	if !errors.Is(errorObtenido, domain.ErrCatalogoRechazo) || escenario.entregas.Reprogramaciones[0] != (dobles.Reprogramacion{Codigo: domain.CodigoLoteRechazado, Permanente: true}) {
		prueba.Fatalf("error=%v reprogramaciones=%+v", errorObtenido, escenario.entregas.Reprogramaciones)
	}
	if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), nil, entrada); !errors.Is(errorObtenido, domain.ErrEntregaFallida) {
		prueba.Fatalf("reenvío de entrega fallida: %v", errorObtenido)
	}
}

// TestIngestarSinSucursalEsPermanente no reintenta un lote sin sucursal resoluble.
func TestIngestarSinSucursalEsPermanente(prueba *testing.T) {
	escenario := nuevoEscenario()
	sucursalInexistente := 999
	entrada := lote(1)
	entrada.SucursalID = &sucursalInexistente
	if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), nil, entrada); !errors.Is(errorObtenido, domain.ErrSucursalNoEncontrada) || !escenario.entregas.Reprogramaciones[0].Permanente {
		prueba.Fatalf("error=%v", errorObtenido)
	}
}

// TestIngestarContadorPendienteEsExplicito informa que Catálogo confirmó aunque el contador falló.
func TestIngestarContadorPendienteEsExplicito(prueba *testing.T) {
	escenario := nuevoEscenario()
	texto := escenario.trabajos.Agregar(1, "Jumbo").String()
	escenario.entregas.ErrorConfirmar = errors.New("SQL caído")
	_, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, lote(1))
	if !errors.Is(errorObtenido, domain.ErrContadorPendiente) || escenario.catalogo.Ingestas != 1 ||
		escenario.entregas.Reprogramaciones[0] != (dobles.Reprogramacion{Codigo: domain.CodigoContadorPendiente}) {
		prueba.Fatalf("error=%v reprogramaciones=%+v", errorObtenido, escenario.entregas.Reprogramaciones)
	}
}

// TestRecuperarEntregasReintenta entrega en segundo plano un lote que quedó pendiente.
func TestRecuperarEntregasReintenta(prueba *testing.T) {
	escenario := nuevoEscenario()
	texto := escenario.trabajos.Agregar(1, "Jumbo").String()
	escenario.catalogo.ErrorIngesta = domain.ErrCatalogoNoDisponible
	entrada := lote(2)
	if _, errorObtenido := escenario.servicio.IngestarProductos(context.Background(), &texto, entrada); errorObtenido == nil {
		prueba.Fatal("se esperaba un fallo inicial")
	}
	escenario.catalogo.ErrorIngesta = nil
	escenario.servicio.RecuperarEntregas(context.Background())
	if estado := escenario.entregas.Guardadas[uuid.MustParse(entrada.LoteID)].Estado; estado != domain.EntregaConfirmada {
		prueba.Fatalf("estado=%s", estado)
	}
}
