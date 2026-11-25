// Persistencia y atomicidad reales; sólo se ejecuta contra la BD desechable configurada por preparar.
package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
)

// ===== Catálogo controlado para SQL real =====

// catalogoCarrito no consulta redes externas; permite provocar rollback por indisponibilidad.
type catalogoCarrito struct {
	errorConsulta error
	noEncontrado  bool
}

// Consultar devuelve estados sin precio, sin falsificar capturas ni stock del catálogo desplegado.
func (catalogo catalogoCarrito) Consultar(_ context.Context, productoIDs []string) (map[string]listas.Disponibilidad, error) {
	if catalogo.errorConsulta != nil {
		return nil, catalogo.errorConsulta
	}
	estados := map[string]listas.Disponibilidad{}
	for _, productoID := range productoIDs {
		estado := listas.EstadoDesconocido
		if catalogo.noEncontrado {
			estado = listas.EstadoNoEncontrado
		}
		estados[productoID] = listas.Disponibilidad{Estado: estado}
	}
	return estados, nil
}

// ===== Recuperación y concurrencia =====

// TestCarritoPersistidoYAislado verifica recuperación, compare-and-swap y separación de cuentas.
func TestCarritoPersistidoYAislado(prueba *testing.T) {
	_, repositorio, _ := preparar(prueba)
	contexto := context.Background()
	usuario, otro := uuid.NewString(), uuid.NewString()
	articulos := []listas.ArticuloCarrito{{ProductoID: uuid.NewString(), Cantidad: 2}}
	inicial, errorObtenido := repositorio.ObtenerCarrito(contexto, usuario)
	if errorObtenido != nil || inicial.Version != 0 || len(inicial.Articulos) != 0 {
		prueba.Fatal(inicial, errorObtenido)
	}
	guardado, errorObtenido := repositorio.GuardarCarrito(contexto, usuario, 0, articulos)
	if errorObtenido != nil || guardado.Version != 1 {
		prueba.Fatal(guardado, errorObtenido)
	}
	recuperado, errorObtenido := repositorio.ObtenerCarrito(contexto, usuario)
	if errorObtenido != nil || recuperado.Articulos[0].Cantidad != 2 {
		prueba.Fatal(recuperado, errorObtenido)
	}
	ajeno, errorObtenido := repositorio.ObtenerCarrito(contexto, otro)
	if errorObtenido != nil || len(ajeno.Articulos) != 0 {
		prueba.Fatal("otra cuenta leyó el carrito", errorObtenido)
	}
	listaAjena, errorObtenido := repositorio.Crear(contexto, otro, "Destino ajeno")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repositorio.Eliminar(context.Background(), otro, listaAjena.ID) })
	if _, errorObtenido = repositorio.PrepararCarrito(contexto, usuario, guardado.Version, listas.EnvioCarrito{ListaID: listaAjena.ID, Clave: uuid.NewString()}); !errors.Is(errorObtenido, listas.ErrDestinoCarrito) {
		prueba.Fatal("se preparó un envío hacia una lista ajena", errorObtenido)
	}
	repetido, errorObtenido := repositorio.GuardarCarrito(contexto, usuario, 0, articulos)
	if errorObtenido != nil || repetido.Version != 1 {
		prueba.Fatal("reintento incrementó la versión", errorObtenido)
	}
	if _, errorObtenido = repositorio.GuardarCarrito(contexto, usuario, 0, []listas.ArticuloCarrito{}); !errors.Is(errorObtenido, listas.ErrVersionCarrito) {
		prueba.Fatal(errorObtenido)
	}
}

// TestConfirmacionCarritoAtomicaYRecuperable compite por la misma clave y comprueba que sólo suma una vez.
func TestConfirmacionCarritoAtomicaYRecuperable(prueba *testing.T) {
	_, repositorio, _ := preparar(prueba)
	contexto := context.Background()
	usuario, productoID := uuid.NewString(), uuid.NewString()
	lista, errorObtenido := repositorio.Crear(contexto, usuario, "Confirmación atómica")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repositorio.Eliminar(context.Background(), usuario, lista.ID) })
	carrito, errorObtenido := repositorio.GuardarCarrito(contexto, usuario, 0, []listas.ArticuloCarrito{{ProductoID: productoID, Cantidad: 2}})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	envio := listas.EnvioCarrito{ListaID: lista.ID, Clave: uuid.NewString()}
	carrito, errorObtenido = repositorio.PrepararCarrito(contexto, usuario, carrito.Version, envio)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, errorObtenido = repositorio.GuardarCarrito(contexto, usuario, carrito.Version, []listas.ArticuloCarrito{}); !errors.Is(errorObtenido, listas.ErrVersionCarrito) {
		prueba.Fatal("se editó el envío preparado", errorObtenido)
	}
	const intentos = 4
	errores := make(chan error, intentos)
	var grupo sync.WaitGroup
	for range intentos {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			resultado, errorConfirmacion := repositorio.ConfirmarCarrito(contexto, usuario, envio.Clave, listas.LimitesContrato, catalogoCarrito{})
			if errorConfirmacion == nil && (resultado.UltimoEnvio == nil || !resultado.UltimoEnvio.Aplicado || len(resultado.Articulos) != 0) {
				errorConfirmacion = errors.New("confirmación incompleta")
			}
			errores <- errorConfirmacion
		}()
	}
	grupo.Wait()
	close(errores)
	for errorConfirmacion := range errores {
		if errorConfirmacion != nil {
			prueba.Fatal(errorConfirmacion)
		}
	}
	detalle, errorObtenido := repositorio.Obtener(contexto, usuario, lista.ID)
	if errorObtenido != nil || len(detalle.Articulos) != 1 || detalle.Articulos[0].Cantidad != 2 {
		prueba.Fatal(detalle, errorObtenido)
	}
	recuperado, errorObtenido := repositorio.ObtenerCarrito(contexto, usuario)
	if errorObtenido != nil || recuperado.Envio != nil || recuperado.UltimoEnvio.Clave != envio.Clave {
		prueba.Fatal(recuperado, errorObtenido)
	}
	// Un recibo antiguo no puede vaciar una compra nueva.
	nuevo, errorObtenido := repositorio.GuardarCarrito(contexto, usuario, recuperado.Version, []listas.ArticuloCarrito{{ProductoID: productoID, Cantidad: 5}})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	reintento, errorObtenido := repositorio.ConfirmarCarrito(contexto, usuario, envio.Clave, listas.LimitesContrato, catalogoCarrito{errorConsulta: listas.ErrCatalogoNoDisponible})
	if errorObtenido != nil || reintento.Version != nuevo.Version || reintento.Articulos[0].Cantidad != 5 {
		prueba.Fatal("un reintento borró una compra nueva", reintento, errorObtenido)
	}
}

// TestCarritoConservaProductosTrasFallo comprueba rollback de Catálogo y liberación tras un rechazo definitivo.
func TestCarritoConservaProductosTrasFallo(prueba *testing.T) {
	_, repositorio, _ := preparar(prueba)
	contexto := context.Background()
	usuario, productoID := uuid.NewString(), uuid.NewString()
	lista, errorObtenido := repositorio.Crear(contexto, usuario, "Conservar borrador")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repositorio.Eliminar(context.Background(), usuario, lista.ID) })
	carrito, errorObtenido := repositorio.GuardarCarrito(contexto, usuario, 0, []listas.ArticuloCarrito{{ProductoID: productoID, Cantidad: 2}})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	clave := uuid.NewString()
	carrito, errorObtenido = repositorio.PrepararCarrito(contexto, usuario, carrito.Version, listas.EnvioCarrito{ListaID: lista.ID, Clave: clave})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, errorObtenido = repositorio.ConfirmarCarrito(contexto, usuario, clave, listas.LimitesContrato, catalogoCarrito{errorConsulta: listas.ErrCatalogoNoDisponible}); !errors.Is(errorObtenido, listas.ErrCatalogoNoDisponible) {
		prueba.Fatal(errorObtenido)
	}
	preservado, errorObtenido := repositorio.ObtenerCarrito(contexto, usuario)
	if errorObtenido != nil || preservado.Version != carrito.Version || preservado.Envio == nil || len(preservado.Articulos) != 1 {
		prueba.Fatal(preservado, errorObtenido)
	}
	rechazado, errorObtenido := repositorio.ConfirmarCarrito(contexto, usuario, clave, listas.LimitesContrato, catalogoCarrito{noEncontrado: true})
	if errorObtenido != nil || rechazado.Envio != nil || rechazado.UltimoEnvio.Aplicado || len(rechazado.Articulos) != 1 {
		prueba.Fatal(rechazado, errorObtenido)
	}
	detalle, errorObtenido := repositorio.Obtener(contexto, usuario, lista.ID)
	if errorObtenido != nil || len(detalle.Articulos) != 0 {
		prueba.Fatal("un rechazo dejó artículos parciales", errorObtenido)
	}
}
