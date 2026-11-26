package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// repositorioDisponibilidadDoble permite probar el servicio sin PostgreSQL y
// registra los IDs que recibe para verificar la deduplicación.
type repositorioDisponibilidadDoble struct {
	registros     []domain.ProductoDisponibilidadRegistro
	errorConsulta error
	idsRecibidos  []string
}

func (repositorio *repositorioDisponibilidadDoble) ConsultarPorIDs(_ context.Context, productoIDs []string) ([]domain.ProductoDisponibilidadRegistro, error) {
	repositorio.idsRecibidos = append([]string(nil), productoIDs...)
	if repositorio.errorConsulta != nil {
		return nil, repositorio.errorConsulta
	}
	return repositorio.registros, nil
}

// relojFijo devuelve siempre la misma hora para que la antigüedad de las
// capturas no dependa del momento en que corre la prueba.
func relojFijo(momento time.Time) func() time.Time {
	return func() time.Time { return momento }
}

func punteroBooleano(valor bool) *bool      { return &valor }
func punteroDecimal(valor float64) *float64 { return &valor }
func punteroFecha(valor time.Time) *time.Time {
	return &valor
}

// TestDisponibilidadServiceValidaLote comprueba que el servicio rechaza lotes
// vacíos o excesivos y que normaliza los UUID sin repetirlos.
func TestDisponibilidadServiceValidaLote(prueba *testing.T) {
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	identificador := "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e"
	otroIdentificador := "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	for _, caso := range []struct {
		nombre        string
		ids           []string
		errorEsperado error
	}{
		{"lote vacío", nil, services.ErrLoteDisponibilidadVacio},
		{"lote excesivo", []string{identificador, identificador, identificador}, services.ErrLoteDisponibilidadExcesivo},
		{"identificador inválido", []string{"no-es-un-uuid"}, services.ErrIDProductoDisponibilidad},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &repositorioDisponibilidadDoble{}
			limites := services.LimitesDisponibilidad{MaximoProductos: 2, AntiguedadMaxima: 48 * time.Hour}
			servicio := services.NuevoServicioDisponibilidad(repositorio, limites, relojFijo(momento))

			_, errorObtenido := servicio.Consultar(context.Background(), caso.ids)
			if !errors.Is(errorObtenido, caso.errorEsperado) {
				subprueba.Fatalf("error = %v; esperado %v", errorObtenido, caso.errorEsperado)
			}
			if len(repositorio.idsRecibidos) != 0 {
				subprueba.Fatalf("se consultó la persistencia con un lote inválido: %v", repositorio.idsRecibidos)
			}
		})
	}

	repositorio := &repositorioDisponibilidadDoble{}
	servicio := services.NuevoServicioDisponibilidad(repositorio,
		services.LimitesDisponibilidad{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))
	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{identificador, otroIdentificador, identificador})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(repositorio.idsRecibidos) != 2 || repositorio.idsRecibidos[0] != identificador || repositorio.idsRecibidos[1] != otroIdentificador {
		prueba.Fatalf("los IDs no se deduplicaron conservando el orden: %v", repositorio.idsRecibidos)
	}
	if len(respuesta.Productos) != 2 {
		prueba.Fatalf("se esperaban 2 productos, se obtuvieron %d", len(respuesta.Productos))
	}
}

// TestDisponibilidadServiceAplicaVigencia verifica los cuatro estados, que el
// stock no confirmado sea desconocido y que una captura vencida, futura o
// inexistente nunca se publique como vigente.
func TestDisponibilidadServiceAplicaVigencia(prueba *testing.T) {
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-1 * time.Hour)
	viejo := momento.Add(-10 * 24 * time.Hour)
	futuro := momento.Add(2 * time.Hour)

	disponibleID := "00000000-0000-4000-8000-000000000001"
	sinStockID := "00000000-0000-4000-8000-000000000002"
	stockDesconocidoID := "00000000-0000-4000-8000-000000000003"
	vencidoID := "00000000-0000-4000-8000-000000000004"
	sinCapturaID := "00000000-0000-4000-8000-000000000005"
	inexistenteID := "00000000-0000-4000-8000-000000000006"
	futuroID := "00000000-0000-4000-8000-000000000007"

	repositorio := &repositorioDisponibilidadDoble{registros: []domain.ProductoDisponibilidadRegistro{
		{
			ProductoID: disponibleID, Nombre: "Leche", Marca: "Colún", Categoria: "Lácteos",
			Supermercado: "Lider", SucursalID: 1, Sucursal: "Lider Ñuñoa", Unidad: "1 L",
			URLImagen: "https://imagen", EnStock: punteroBooleano(true),
			PrecioNormal: punteroDecimal(1290), PrecioOferta: punteroDecimal(990),
			CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: sinStockID, Nombre: "Arroz", Marca: "Tucapel", Categoria: "Despensa",
			Supermercado: "Jumbo", SucursalID: 2, Sucursal: "Jumbo Centro", Unidad: "1 kg",
			EnStock: punteroBooleano(false), PrecioNormal: punteroDecimal(1990),
			CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: stockDesconocidoID, Nombre: "Fideos", Marca: "Carozzi", Categoria: "Despensa",
			Supermercado: "Tottus", SucursalID: 3, Sucursal: "Tottus Maipú", Unidad: "400 g",
			EnStock: nil, PrecioNormal: punteroDecimal(1490),
			CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: vencidoID, Nombre: "Aceite", Marca: "Chef", Categoria: "Despensa",
			Supermercado: "Tottus", SucursalID: 3, Sucursal: "Tottus Maipú", Unidad: "1 L",
			EnStock: punteroBooleano(true), PrecioNormal: punteroDecimal(2490),
			CapturadoEl: punteroFecha(viejo),
		},
		{
			ProductoID: sinCapturaID, Nombre: "Harina", Marca: "Selecta", Categoria: "Despensa",
			Supermercado: "Santa Isabel", SucursalID: 4, Sucursal: "Santa Isabel Ñuñoa", Unidad: "1 kg",
			EnStock: punteroBooleano(true),
		},
		{
			ProductoID: futuroID, Nombre: "Té", Marca: "Supremo", Categoria: "Despensa",
			Supermercado: "Lider", SucursalID: 1, Sucursal: "Lider Ñuñoa", Unidad: "20 bolsas",
			EnStock: punteroBooleano(true), PrecioNormal: punteroDecimal(1000),
			CapturadoEl: punteroFecha(futuro),
		},
	}}
	servicio := services.NuevoServicioDisponibilidad(repositorio,
		services.LimitesDisponibilidad{MaximoProductos: 1000, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{
		disponibleID, sinStockID, stockDesconocidoID, vencidoID, sinCapturaID, inexistenteID, futuroID, disponibleID,
	})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 7 {
		prueba.Fatalf("se esperaban 7 productos únicos, se obtuvieron %d", len(respuesta.Productos))
	}

	productos := make(map[string]domain.DisponibilidadItemDTO, len(respuesta.Productos))
	for _, item := range respuesta.Productos {
		productos[item.ProductoID] = item
	}

	disponible := productos[disponibleID]
	if disponible.EstadoProducto != domain.EstadoProductoDisponible || disponible.EnStock == nil || !*disponible.EnStock ||
		disponible.EstadoPrecio != domain.EstadoPrecioVigente || disponible.Producto == nil {
		prueba.Fatalf("producto disponible incorrecto: %+v", disponible)
	}
	if disponible.Producto.Precio == nil || *disponible.Producto.Precio != 990 ||
		disponible.Producto.PrecioNormal == nil || *disponible.Producto.PrecioNormal != 1290 {
		prueba.Fatalf("precios efectivos incorrectos: %+v", disponible.Producto)
	}

	sinStock := productos[sinStockID]
	if sinStock.EstadoProducto != domain.EstadoProductoSinStock || sinStock.EnStock == nil || *sinStock.EnStock ||
		sinStock.EstadoPrecio != domain.EstadoPrecioVigente {
		prueba.Fatalf("producto sin stock incorrecto: %+v", sinStock)
	}

	stockDesconocido := productos[stockDesconocidoID]
	if stockDesconocido.EstadoProducto != domain.EstadoProductoDesconocido || stockDesconocido.EnStock != nil ||
		stockDesconocido.EstadoPrecio != domain.EstadoPrecioVigente || stockDesconocido.Producto == nil {
		prueba.Fatalf("producto con stock no confirmado incorrecto: %+v", stockDesconocido)
	}
	if stockDesconocido.Producto.Precio == nil || *stockDesconocido.Producto.Precio != 1490 {
		prueba.Fatalf("el precio vigente debe conservarse: %+v", stockDesconocido.Producto)
	}

	vencido := productos[vencidoID]
	if vencido.EstadoProducto != domain.EstadoProductoDesconocido || vencido.EnStock != nil ||
		vencido.EstadoPrecio != domain.EstadoPrecioVencido || vencido.Producto == nil {
		prueba.Fatalf("producto vencido incorrecto: %+v", vencido)
	}
	if vencido.Producto.Precio != nil || vencido.Producto.PrecioNormal != nil || vencido.Producto.PrecioOferta != nil {
		prueba.Fatalf("un producto vencido no debe publicar precios: %+v", vencido.Producto)
	}
	if vencido.CapturadoEl == nil || !vencido.CapturadoEl.Equal(viejo) {
		prueba.Fatalf("la fecha de captura vencida debe informarse: %+v", vencido.CapturadoEl)
	}

	sinCaptura := productos[sinCapturaID]
	if sinCaptura.EstadoProducto != domain.EstadoProductoDesconocido || sinCaptura.EnStock != nil ||
		sinCaptura.EstadoPrecio != domain.EstadoPrecioSinCaptura || sinCaptura.CapturadoEl != nil {
		prueba.Fatalf("producto sin captura incorrecto: %+v", sinCaptura)
	}

	inexistente := productos[inexistenteID]
	if inexistente.EstadoProducto != domain.EstadoProductoNoEncontrado || inexistente.Producto != nil ||
		inexistente.EnStock != nil || inexistente.EstadoPrecio != domain.EstadoPrecioSinCaptura {
		prueba.Fatalf("producto inexistente incorrecto: %+v", inexistente)
	}

	capturaFutura := productos[futuroID]
	if capturaFutura.EstadoProducto != domain.EstadoProductoDesconocido || capturaFutura.EnStock != nil ||
		capturaFutura.EstadoPrecio != domain.EstadoPrecioVencido || capturaFutura.Producto == nil {
		prueba.Fatalf("producto con captura futura incorrecto: %+v", capturaFutura)
	}
	if capturaFutura.Producto.Precio != nil || capturaFutura.Producto.PrecioNormal != nil {
		prueba.Fatalf("una captura futura no debe publicar precios: %+v", capturaFutura.Producto)
	}
}

// TestDisponibilidadServiceIgnoraPrecioBajoMinimo comprueba que las capturas
// con precio_normal menor a PrecioMinimoValido (o nulo) se traten como sin
// captura y nunca publiquen precios basura, mientras que el umbral exacto sí se
// publica como vigente.
func TestDisponibilidadServiceIgnoraPrecioBajoMinimo(prueba *testing.T) {
	momento := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-1 * time.Hour)

	precioUnoID := "00000000-0000-4000-8000-000000000011"
	precioNoventaYNueveID := "00000000-0000-4000-8000-000000000012"
	precioNuloID := "00000000-0000-4000-8000-000000000013"
	precioUmbralID := "00000000-0000-4000-8000-000000000014"

	repositorio := &repositorioDisponibilidadDoble{registros: []domain.ProductoDisponibilidadRegistro{
		{
			ProductoID: precioUnoID, Nombre: "Arroz basura", Supermercado: "Lider", SucursalID: 1,
			EnStock: punteroBooleano(true), PrecioNormal: punteroDecimal(1), CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: precioNoventaYNueveID, Nombre: "Leche borde inferior", Supermercado: "Jumbo", SucursalID: 2,
			EnStock: punteroBooleano(true), PrecioNormal: punteroDecimal(99), CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: precioNuloID, Nombre: "Aceite sin precio", Supermercado: "Tottus", SucursalID: 3,
			EnStock: punteroBooleano(true), PrecioNormal: nil, CapturadoEl: punteroFecha(fresco),
		},
		{
			ProductoID: precioUmbralID, Nombre: "Sal umbral exacto", Supermercado: "Lider", SucursalID: 1,
			EnStock: punteroBooleano(true), PrecioNormal: punteroDecimal(domain.PrecioMinimoValido), CapturadoEl: punteroFecha(fresco),
		},
	}}
	servicio := services.NuevoServicioDisponibilidad(repositorio,
		services.LimitesDisponibilidad{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{
		precioUnoID, precioNoventaYNueveID, precioNuloID, precioUmbralID,
	})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	productos := make(map[string]domain.DisponibilidadItemDTO, len(respuesta.Productos))
	for _, item := range respuesta.Productos {
		productos[item.ProductoID] = item
	}

	for _, identificadorInvalido := range []string{precioUnoID, precioNoventaYNueveID, precioNuloID} {
		item := productos[identificadorInvalido]
		if item.EstadoPrecio != domain.EstadoPrecioSinCaptura || item.EstadoProducto != domain.EstadoProductoDesconocido {
			prueba.Errorf("%s no debe publicarse como vigente: %+v", identificadorInvalido, item)
		}
		if item.Producto == nil || item.Producto.Precio != nil || item.Producto.PrecioNormal != nil || item.Producto.PrecioOferta != nil {
			prueba.Errorf("%s no debe exponer precios: %+v", identificadorInvalido, item.Producto)
		}
		if item.EnStock != nil || item.CapturadoEl != nil {
			prueba.Errorf("%s no debe exponer stock ni fecha de captura inválida: %+v", identificadorInvalido, item)
		}
	}

	valido := productos[precioUmbralID]
	if valido.EstadoPrecio != domain.EstadoPrecioVigente || valido.EstadoProducto != domain.EstadoProductoDisponible ||
		valido.Producto == nil || valido.Producto.Precio == nil || *valido.Producto.Precio != domain.PrecioMinimoValido {
		prueba.Fatalf("el precio exacto en el umbral (%d) debe publicarse: %+v", domain.PrecioMinimoValido, valido)
	}
}

// TestDisponibilidadServicePropagaError verifica que un fallo de persistencia
// no se transforme en una respuesta válida.
func TestDisponibilidadServicePropagaError(prueba *testing.T) {
	errorPersistencia := errors.New("fallo de persistencia")
	repositorio := &repositorioDisponibilidadDoble{errorConsulta: errorPersistencia}
	servicio := services.NuevoServicioDisponibilidad(repositorio,
		services.LimitesDisponibilidad{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, time.Now)

	_, errorObtenido := servicio.Consultar(context.Background(),
		[]string{"00000000-0000-4000-8000-000000000001"})
	if !errors.Is(errorObtenido, errorPersistencia) {
		prueba.Fatalf("error = %v; esperado %v", errorObtenido, errorPersistencia)
	}
}

// Garantiza que el doble de pruebas cumpla la interfaz del dominio.
var _ domain.RepositorioDisponibilidad = (*repositorioDisponibilidadDoble)(nil)
