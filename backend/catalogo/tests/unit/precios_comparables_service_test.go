package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// repositorioPreciosDoble sustituye la lectura PostGIS y guarda los parámetros
// recibidos para comprobar que el radio y el tope llegan a la consulta acotada.
type repositorioPreciosDoble struct {
	candidatas       []domain.SucursalCercanaRegistro
	sucursales       []domain.SucursalRegistro
	latitudRecibida  float64
	longitudRecibida float64
	radioRecibido    float64
	maximoRecibido   int
	errorCandidatas  error
	errorSucursales  error
}

func (repositorio *repositorioPreciosDoble) ConsultarCercanas(_ context.Context, latitud, longitud, radioKm float64, maximo int) ([]domain.SucursalCercanaRegistro, error) {
	repositorio.latitudRecibida = latitud
	repositorio.longitudRecibida = longitud
	repositorio.radioRecibido = radioKm
	repositorio.maximoRecibido = maximo
	if repositorio.errorCandidatas != nil {
		return nil, repositorio.errorCandidatas
	}
	return repositorio.candidatas, nil
}

func (repositorio *repositorioPreciosDoble) ConsultarSucursales(_ context.Context) ([]domain.SucursalRegistro, error) {
	if repositorio.errorSucursales != nil {
		return nil, repositorio.errorSucursales
	}
	return repositorio.sucursales, nil
}

// solicitudPrecios arma una solicitud válida con el origen de la demo.
func solicitudPrecios(ids ...string) domain.ConsultaPreciosComparablesDTO {
	latitud, longitud, radio, maximo := -38.739, -72.598, 10.0, 5
	return domain.ConsultaPreciosComparablesDTO{
		ProductoIDs:      ids,
		Latitud:          &latitud,
		Longitud:         &longitud,
		RadioKm:          &radio,
		MaximoSucursales: &maximo,
	}
}

// nuevoServicioPrecios compone el servicio real de equivalencias sobre un doble
// para que las reglas de EAN de SUP-244 se apliquen de verdad en la prueba.
func nuevoServicioPrecios(repositorioPrecios *repositorioPreciosDoble, repositorioEquivalencias *repositorioEquivalenciasDoble, momento time.Time) *services.ServicioPreciosComparables {
	equivalencias := services.NuevoServicioEquivalencias(repositorioEquivalencias,
		services.LimitesEquivalencia{MaximoProductos: 100, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))
	return services.NuevoServicioPreciosComparables(repositorioPrecios, equivalencias,
		services.LimitesPreciosComparables{MaximoProductos: 100, RadioMaximoKm: 50})
}

// TestPreciosComparablesAplicaRadioYExclusiones cubre el filtro por sucursales
// candidatas, la exclusión explícita y el resumen de cobertura.
func TestPreciosComparablesAplicaRadioYExclusiones(prueba *testing.T) {
	momento := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	lecheLiderID := "00000000-0000-4000-8000-000000000001"
	lecheJumboID := "00000000-0000-4000-8000-000000000002"
	sinEANID := "00000000-0000-4000-8000-000000000004"
	inexistenteID := "00000000-0000-4000-8000-000000000008"

	lecheLider := registroEquivalenciaBase(lecheLiderID, 2, "Lider")
	lecheLider.CodigoEAN = referenciaCadena("0036000291452")
	lecheLider.PrecioNormal = punteroDecimal(1290)
	lecheLider.EnStock = punteroBooleano(true)
	lecheLider.CapturadoEl = punteroFecha(momento.Add(-time.Hour))

	lecheJumbo := registroEquivalenciaBase(lecheJumboID, 1, "Jumbo")
	lecheJumbo.CodigoEAN = referenciaCadena("036000291452")
	lecheJumbo.PrecioNormal = punteroDecimal(1190)
	lecheJumbo.EnStock = punteroBooleano(true)
	lecheJumbo.CapturadoEl = punteroFecha(momento.Add(-time.Hour))

	sinEAN := registroEquivalenciaBase(sinEANID, 4, "Santa Isabel")
	sinEAN.PrecioNormal = punteroDecimal(1000)
	sinEAN.EnStock = punteroBooleano(true)
	sinEAN.CapturadoEl = punteroFecha(momento.Add(-time.Hour))

	repositorioPrecios := &repositorioPreciosDoble{
		candidatas: []domain.SucursalCercanaRegistro{
			{ID: 1, Nombre: "Jumbo Los Pablos", Cadena: "Jumbo", Lat: -38.741, Lon: -72.6, DistanciaKm: 0.28},
			{ID: 2, Nombre: "Lider Prieto Norte", Cadena: "Lider", Lat: -38.735, Lon: -72.59, DistanciaKm: 0.82},
		},
		sucursales: []domain.SucursalRegistro{
			{ID: 1, Nombre: "Jumbo Los Pablos", Cadena: "Jumbo", Direccion: "Av. Los Pablos 01860", Activa: true, Lat: punteroDecimal(-38.741), Lon: punteroDecimal(-72.6)},
			{ID: 2, Nombre: "Lider Prieto Norte", Cadena: "Lider", Direccion: "Av. Caupolicán 0450", Activa: true, Lat: punteroDecimal(-38.735), Lon: punteroDecimal(-72.59)},
			{ID: 3, Nombre: "Unimarc Cerrada", Cadena: "Unimarc", Direccion: "Av. Alemania 0655", Activa: false, Lat: punteroDecimal(-38.745), Lon: punteroDecimal(-72.61)},
			{ID: 4, Nombre: "Santa Isabel San Martín", Cadena: "Santa Isabel", Direccion: "San Martín 0890", Activa: true, Lat: punteroDecimal(-38.73), Lon: punteroDecimal(-72.605)},
			{ID: 5, Nombre: "Sucursal sin coords", Cadena: "Cugat", Direccion: "Sin ubicación", Activa: true},
			{ID: 6, Nombre: "Tienda Online", Cadena: "A Cuenta", Direccion: "Casa Matriz / Online", Activa: true, Lat: punteroDecimal(-38.7359), Lon: punteroDecimal(-72.5904)},
		},
	}
	repositorioEquivalencias := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{
		lecheLider, lecheJumbo, sinEAN,
	}}
	servicio := nuevoServicioPrecios(repositorioPrecios, repositorioEquivalencias, momento)

	respuesta, errorObtenido := servicio.Consultar(context.Background(), solicitudPrecios(lecheLiderID, lecheJumboID, sinEANID, inexistenteID))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if repositorioPrecios.latitudRecibida != -38.739 || repositorioPrecios.longitudRecibida != -72.598 ||
		repositorioPrecios.radioRecibido != 10 || repositorioPrecios.maximoRecibido != 5 {
		prueba.Fatalf("parámetros del radio incorrectos: %+v", repositorioPrecios)
	}
	if len(respuesta.Sucursales) != 2 || respuesta.Sucursales[0].SucursalID != 1 {
		prueba.Fatalf("se esperaban 2 candidatas ordenadas, se obtuvieron %+v", respuesta.Sucursales)
	}
	if len(respuesta.Productos) != 2 {
		prueba.Fatalf("se esperaban 2 productos con oferta, se obtuvieron %d", len(respuesta.Productos))
	}
	for _, producto := range respuesta.Productos {
		if len(producto.Ofertas) != 2 {
			prueba.Errorf("%s debería tener ofertas en las 2 candidatas, tiene %d", producto.ProductoID, len(producto.Ofertas))
		}
	}
	if len(respuesta.SinOfertas) != 2 || respuesta.SinOfertas[0] != sinEANID || respuesta.SinOfertas[1] != inexistenteID {
		prueba.Fatalf("sin_ofertas incorrecto: %v", respuesta.SinOfertas)
	}
	motivos := make(map[int]string, len(respuesta.SucursalesExcluidas))
	for _, excluida := range respuesta.SucursalesExcluidas {
		motivos[excluida.SucursalID] = excluida.Motivo
	}
	if motivos[3] != domain.MotivoSucursalInactiva || motivos[5] != domain.MotivoSucursalSinCoordenadas || motivos[6] != domain.MotivoSucursalOnline {
		prueba.Fatalf("motivos de exclusión incorrectos: %+v", motivos)
	}
	resumen := respuesta.Resumen
	if resumen.Solicitados != 4 || resumen.ConOferta != 2 || resumen.SinOferta != 2 ||
		resumen.Comparables != 2 || resumen.SinEAN != 1 || resumen.NoEncontrados != 1 ||
		resumen.SucursalesCandidatas != 2 || resumen.SucursalesExcluidas != 3 {
		prueba.Fatalf("resumen incorrecto: %+v", resumen)
	}
}

// TestPreciosComparablesPrecioNoDisponiblePublicaOfertaSinPrecio comprueba que
// una captura vencida no se convierte en precio vigente ni se calla.
func TestPreciosComparablesPrecioNoDisponiblePublicaOfertaSinPrecio(prueba *testing.T) {
	momento := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	productoID := "00000000-0000-4000-8000-000000000001"

	registro := registroEquivalenciaBase(productoID, 1, "Jumbo")
	registro.CodigoEAN = referenciaCadena("0036000291452")
	registro.PrecioNormal = punteroDecimal(1290)
	registro.EnStock = punteroBooleano(true)
	registro.CapturadoEl = punteroFecha(momento.Add(-10 * 24 * time.Hour))

	repositorioPrecios := &repositorioPreciosDoble{
		candidatas: []domain.SucursalCercanaRegistro{{ID: 1, Cadena: "Jumbo", DistanciaKm: 0.3}},
		sucursales: []domain.SucursalRegistro{{ID: 1, Cadena: "Jumbo", Activa: true, Lat: punteroDecimal(-38.741), Lon: punteroDecimal(-72.6)}},
	}
	repositorioEquivalencias := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{registro}}
	servicio := nuevoServicioPrecios(repositorioPrecios, repositorioEquivalencias, momento)

	respuesta, errorObtenido := servicio.Consultar(context.Background(), solicitudPrecios(productoID))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 1 || len(respuesta.Productos[0].Ofertas) != 1 {
		prueba.Fatalf("se esperaba una oferta visible: %+v", respuesta.Productos)
	}
	oferta := respuesta.Productos[0].Ofertas[0]
	if oferta.Precio != nil || oferta.EstadoPrecio != domain.EstadoPrecioVencido {
		prueba.Fatalf("precio vencido publicado como vigente: %+v", oferta)
	}
	if respuesta.Resumen.OfertasSinPrecio != 1 || len(respuesta.SinOfertas) != 0 {
		prueba.Fatalf("cobertura incorrecta: %+v", respuesta)
	}
}

// TestPreciosComparablesSinEANNoEquiparaPorNombre verifica que dos productos
// sin EAN con el mismo nombre conserven sólo su identidad exacta.
func TestPreciosComparablesSinEANNoEquiparaPorNombre(prueba *testing.T) {
	momento := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	productoUno := "00000000-0000-4000-8000-000000000001"
	productoDos := "00000000-0000-4000-8000-000000000002"

	uno := registroEquivalenciaBase(productoUno, 1, "Jumbo")
	uno.Nombre = "Leche entera"
	uno.PrecioNormal = punteroDecimal(1290)
	uno.CapturadoEl = punteroFecha(momento.Add(-time.Hour))
	dos := registroEquivalenciaBase(productoDos, 2, "Lider")
	dos.Nombre = "Leche entera"
	dos.PrecioNormal = punteroDecimal(1390)
	dos.CapturadoEl = punteroFecha(momento.Add(-time.Hour))

	repositorioPrecios := &repositorioPreciosDoble{
		candidatas: []domain.SucursalCercanaRegistro{
			{ID: 1, Cadena: "Jumbo", DistanciaKm: 0.3},
			{ID: 2, Cadena: "Lider", DistanciaKm: 0.9},
		},
		sucursales: []domain.SucursalRegistro{
			{ID: 1, Cadena: "Jumbo", Activa: true, Lat: punteroDecimal(-38.741), Lon: punteroDecimal(-72.6)},
			{ID: 2, Cadena: "Lider", Activa: true, Lat: punteroDecimal(-38.735), Lon: punteroDecimal(-72.59)},
		},
	}
	repositorioEquivalencias := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{uno, dos}}
	servicio := nuevoServicioPrecios(repositorioPrecios, repositorioEquivalencias, momento)

	respuesta, errorObtenido := servicio.Consultar(context.Background(), solicitudPrecios(productoUno, productoDos))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 2 {
		prueba.Fatalf("se esperaban 2 productos, se obtuvieron %d", len(respuesta.Productos))
	}
	for _, producto := range respuesta.Productos {
		if producto.Estado != domain.EstadoEquivalenciaSinEAN || len(producto.Ofertas) != 1 ||
			producto.Ofertas[0].ProductoIDOferta != producto.ProductoID {
			prueba.Errorf("producto sin EAN equiparado por nombre: %+v", producto)
		}
	}
}

// TestPreciosComparablesValidaSolicitud evita consultas sin lote, con UUID
// inválido, coordenadas fuera de rango o parámetros sin límite.
func TestPreciosComparablesValidaSolicitud(prueba *testing.T) {
	identificador := "00000000-0000-4000-8000-000000000001"
	latitudMala := 91.0
	longitudMala := -181.0
	radioCero := 0.0
	radioExcesivo := 51.0
	maximoCero := 0
	maximoExcesivo := 11

	for _, caso := range []struct {
		nombre        string
		solicitud     domain.ConsultaPreciosComparablesDTO
		errorEsperado error
	}{
		{"lote vacío", solicitudPrecios(), services.ErrLotePreciosVacio},
		{"UUID inválido", solicitudPrecios("no-uuid"), services.ErrIDProductoPrecios},
		{"sin latitud", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.Latitud = nil
			return solicitud
		}(), services.ErrCoordenadasPrecios},
		{"latitud fuera de rango", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.Latitud = &latitudMala
			return solicitud
		}(), services.ErrCoordenadasPrecios},
		{"longitud fuera de rango", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.Longitud = &longitudMala
			return solicitud
		}(), services.ErrCoordenadasPrecios},
		{"radio cero", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.RadioKm = &radioCero
			return solicitud
		}(), services.ErrRadioPrecios},
		{"radio sobre el límite", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.RadioKm = &radioExcesivo
			return solicitud
		}(), services.ErrRadioPrecios},
		{"sin sucursales", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.MaximoSucursales = &maximoCero
			return solicitud
		}(), services.ErrMaximoSucursalesPrecios},
		{"demasiadas sucursales", func() domain.ConsultaPreciosComparablesDTO {
			solicitud := solicitudPrecios(identificador)
			solicitud.MaximoSucursales = &maximoExcesivo
			return solicitud
		}(), services.ErrMaximoSucursalesPrecios},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			servicio := nuevoServicioPrecios(&repositorioPreciosDoble{}, &repositorioEquivalenciasDoble{}, time.Now())
			_, errorObtenido := servicio.Consultar(context.Background(), caso.solicitud)
			if !errors.Is(errorObtenido, caso.errorEsperado) {
				subprueba.Fatalf("se esperaba %v, se obtuvo %v", caso.errorEsperado, errorObtenido)
			}
		})
	}
}
