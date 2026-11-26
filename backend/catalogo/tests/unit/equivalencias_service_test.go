package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// repositorioEquivalenciasDoble permite probar el servicio sin PostgreSQL y
// registra los IDs que recibe para verificar la deduplicación.
type repositorioEquivalenciasDoble struct {
	registros     []domain.ProductoEquivalenciaRegistro
	errorConsulta error
	idsRecibidos  []string
}

func (repositorio *repositorioEquivalenciasDoble) ConsultarPorIDs(_ context.Context, productoIDs []string) ([]domain.ProductoEquivalenciaRegistro, error) {
	repositorio.idsRecibidos = append([]string(nil), productoIDs...)
	if repositorio.errorConsulta != nil {
		return nil, repositorio.errorConsulta
	}
	return repositorio.registros, nil
}

// registroEquivalenciaBase arma una fila válida que cada prueba ajusta según el
// caso. Evita repetir los datos descriptivos que no son el objeto de la prueba.
func registroEquivalenciaBase(productoID string, sucursalID int, cadena string) domain.ProductoEquivalenciaRegistro {
	return domain.ProductoEquivalenciaRegistro{
		ProductoID: productoID,
		SucursalID: sucursalID,
		Cadena:     cadena,
		Sucursal:   cadena + " Centro",
		Nombre:     "Producto " + productoID,
		Formato:    "1 L",
	}
}

// TestEquivalenciasServiceAgrupaPorEAN cubre EAN igual, ceros iniciales entre
// UPC-A y EAN-13, EAN ausente, dígito verificador inválido, conflicto de
// presentación y producto inexistente.
func TestEquivalenciasServiceAgrupaPorEAN(prueba *testing.T) {
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-1 * time.Hour)
	viejo := momento.Add(-10 * 24 * time.Hour)

	lecheLiderID := "00000000-0000-4000-8000-000000000001"
	lecheJumboID := "00000000-0000-4000-8000-000000000002"
	lecheUnimarcID := "00000000-0000-4000-8000-000000000003"
	sinEANID := "00000000-0000-4000-8000-000000000004"
	eanInvalidoID := "00000000-0000-4000-8000-000000000005"
	conflictoLiderID := "00000000-0000-4000-8000-000000000006"
	conflictoJumboID := "00000000-0000-4000-8000-000000000007"
	inexistenteID := "00000000-0000-4000-8000-000000000008"

	lecheLider := registroEquivalenciaBase(lecheLiderID, 2, "Lider")
	lecheLider.CodigoEAN = referenciaCadena("0036000291452")
	lecheLider.PrecioNormal = punteroDecimal(1290)
	lecheLider.PrecioOferta = punteroDecimal(990)
	lecheLider.EnStock = punteroBooleano(true)
	lecheLider.CapturadoEl = punteroFecha(fresco)

	lecheJumbo := registroEquivalenciaBase(lecheJumboID, 1, "Jumbo")
	lecheJumbo.CodigoEAN = referenciaCadena("036000291452")
	lecheJumbo.PrecioNormal = punteroDecimal(1190)
	lecheJumbo.EnStock = punteroBooleano(false)
	lecheJumbo.CapturadoEl = punteroFecha(fresco)

	lecheUnimarc := registroEquivalenciaBase(lecheUnimarcID, 3, "Unimarc")
	lecheUnimarc.CodigoEAN = referenciaCadena("0036000291452")
	lecheUnimarc.Formato = "1  l"
	lecheUnimarc.PrecioNormal = punteroDecimal(1250)
	lecheUnimarc.EnStock = punteroBooleano(true)
	lecheUnimarc.CapturadoEl = punteroFecha(viejo)

	sinEAN := registroEquivalenciaBase(sinEANID, 4, "Santa Isabel")
	sinEAN.Formato = "500 g"
	sinEAN.PrecioNormal = punteroDecimal(1000)
	sinEAN.CapturadoEl = punteroFecha(fresco)

	eanInvalido := registroEquivalenciaBase(eanInvalidoID, 5, "A Cuenta")
	eanInvalido.CodigoEAN = referenciaCadena("7802900000005")
	eanInvalido.PrecioNormal = punteroDecimal(800)
	eanInvalido.CapturadoEl = punteroFecha(fresco)

	conflictoLider := registroEquivalenciaBase(conflictoLiderID, 2, "Lider")
	conflictoLider.CodigoEAN = referenciaCadena("7802900000004")
	conflictoLider.PrecioNormal = punteroDecimal(1500)
	conflictoLider.CapturadoEl = punteroFecha(fresco)

	conflictoJumbo := registroEquivalenciaBase(conflictoJumboID, 1, "Jumbo")
	conflictoJumbo.CodigoEAN = referenciaCadena("7802900000004")
	conflictoJumbo.Formato = "1.5 L"
	conflictoJumbo.PrecioNormal = punteroDecimal(1900)
	conflictoJumbo.CapturadoEl = punteroFecha(fresco)

	repositorio := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{
		lecheLider, lecheJumbo, lecheUnimarc, sinEAN, eanInvalido, conflictoLider, conflictoJumbo,
	}}
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: 1000, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{
		lecheLiderID, sinEANID, eanInvalidoID, conflictoLiderID, inexistenteID, lecheLiderID,
	})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 5 {
		prueba.Fatalf("se esperaban 5 productos únicos, se obtuvieron %d", len(respuesta.Productos))
	}

	productos := make(map[string]domain.EquivalenciaProductoDTO, len(respuesta.Productos))
	for _, item := range respuesta.Productos {
		productos[item.ProductoID] = item
	}

	leche := productos[lecheLiderID]
	if leche.Estado != domain.EstadoEquivalenciaComparable || leche.EAN == nil || *leche.EAN != "00036000291452" {
		prueba.Fatalf("equivalencia de leche incorrecta: %+v", leche)
	}
	if len(leche.Ofertas) != 3 {
		prueba.Fatalf("se esperaban 3 ofertas para leche, se obtuvieron %d", len(leche.Ofertas))
	}
	for indice, sucursalEsperada := range []int{1, 2, 3} {
		if leche.Ofertas[indice].SucursalID != sucursalEsperada {
			prueba.Fatalf("orden de ofertas incorrecto: %+v", leche.Ofertas)
		}
	}
	ofertaLider := leche.Ofertas[1]
	if ofertaLider.ProductoIDOferta != lecheLiderID || ofertaLider.Precio == nil || *ofertaLider.Precio != 990 ||
		ofertaLider.PrecioNormal == nil || *ofertaLider.PrecioNormal != 1290 ||
		ofertaLider.EnStock == nil || !*ofertaLider.EnStock || ofertaLider.EstadoPrecio != domain.EstadoPrecioVigente ||
		ofertaLider.CapturadoEl == nil || !ofertaLider.CapturadoEl.Equal(fresco) {
		prueba.Fatalf("oferta de Lider incorrecta: %+v", ofertaLider)
	}
	ofertaJumbo := leche.Ofertas[0]
	if ofertaJumbo.Precio == nil || *ofertaJumbo.Precio != 1190 || ofertaJumbo.EnStock == nil || *ofertaJumbo.EnStock {
		prueba.Fatalf("oferta de Jumbo incorrecta: %+v", ofertaJumbo)
	}
	ofertaVencida := leche.Ofertas[2]
	if ofertaVencida.EstadoPrecio != domain.EstadoPrecioVencido || ofertaVencida.Precio != nil ||
		ofertaVencida.PrecioNormal != nil || ofertaVencida.EnStock != nil {
		prueba.Fatalf("una captura vencida no debe publicar precio ni stock: %+v", ofertaVencida)
	}

	sinCodigo := productos[sinEANID]
	if sinCodigo.Estado != domain.EstadoEquivalenciaSinEAN || sinCodigo.EAN != nil ||
		sinCodigo.Presentacion != "500 g" || len(sinCodigo.Ofertas) != 1 ||
		sinCodigo.Ofertas[0].ProductoIDOferta != sinEANID {
		prueba.Fatalf("producto sin EAN incorrecto: %+v", sinCodigo)
	}

	invalido := productos[eanInvalidoID]
	if invalido.Estado != domain.EstadoEquivalenciaEANInvalido || invalido.EAN != nil || len(invalido.Ofertas) != 1 {
		prueba.Fatalf("producto con EAN inválido incorrecto: %+v", invalido)
	}

	conflicto := productos[conflictoLiderID]
	if conflicto.Estado != domain.EstadoEquivalenciaConflictoPresentacion || conflicto.EAN == nil ||
		*conflicto.EAN != "07802900000004" || len(conflicto.Ofertas) != 1 ||
		conflicto.Ofertas[0].ProductoIDOferta != conflictoLiderID {
		prueba.Fatalf("conflicto de presentación incorrecto: %+v", conflicto)
	}

	inexistente := productos[inexistenteID]
	if inexistente.Estado != domain.EstadoEquivalenciaNoEncontrado || inexistente.EAN != nil ||
		inexistente.Ofertas == nil || len(inexistente.Ofertas) != 0 {
		prueba.Fatalf("producto inexistente incorrecto: %+v", inexistente)
	}

	resumenEsperado := domain.ResumenEquivalenciaDTO{
		Solicitados: 5, Comparables: 1, SinEAN: 1, EANInvalidos: 1, ConflictosPresentacion: 1, NoEncontrados: 1,
	}
	if respuesta.Resumen != resumenEsperado {
		prueba.Fatalf("resumen = %+v; esperado %+v", respuesta.Resumen, resumenEsperado)
	}
}

// TestEquivalenciasServiceValidaLote comprueba que el servicio rechace lotes
// vacíos o excesivos y que normalice los UUID sin repetirlos.
func TestEquivalenciasServiceValidaLote(prueba *testing.T) {
	momento := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	identificador := "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e"
	otroIdentificador := "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	for _, caso := range []struct {
		nombre        string
		ids           []string
		errorEsperado error
	}{
		{"lote vacío", nil, services.ErrLoteEquivalenciaVacio},
		{"lote excesivo", []string{identificador, identificador, identificador}, services.ErrLoteEquivalenciaExcesivo},
		{"identificador inválido", []string{"no-es-un-uuid"}, services.ErrIDProductoEquivalencia},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			repositorio := &repositorioEquivalenciasDoble{}
			servicio := services.NuevoServicioEquivalencias(repositorio,
				services.LimitesEquivalencia{MaximoProductos: 2, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

			_, errorObtenido := servicio.Consultar(context.Background(), caso.ids)
			if !errors.Is(errorObtenido, caso.errorEsperado) {
				subprueba.Fatalf("error = %v; esperado %v", errorObtenido, caso.errorEsperado)
			}
			if len(repositorio.idsRecibidos) != 0 {
				subprueba.Fatalf("se consultó la persistencia con un lote inválido: %v", repositorio.idsRecibidos)
			}
		})
	}

	repositorio := &repositorioEquivalenciasDoble{}
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))
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

// TestEquivalenciasServiceIgnoraPrecioBajoMinimo comprueba que una oferta con
// precio_normal < PrecioMinimoValido no publique precio ni stock, mientras que
// el umbral exacto sí se publica como vigente.
func TestEquivalenciasServiceIgnoraPrecioBajoMinimo(prueba *testing.T) {
	momento := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-1 * time.Hour)

	productoBasuraID := "00000000-0000-4000-8000-000000000021"
	productoValidoID := "00000000-0000-4000-8000-000000000022"

	basura := registroEquivalenciaBase(productoBasuraID, 1, "Jumbo")
	basura.CodigoEAN = referenciaCadena("0036000291452")
	basura.PrecioNormal = punteroDecimal(2)
	basura.EnStock = punteroBooleano(true)
	basura.CapturadoEl = punteroFecha(fresco)

	valido := registroEquivalenciaBase(productoValidoID, 2, "Lider")
	valido.CodigoEAN = referenciaCadena("0036000291452")
	valido.PrecioNormal = punteroDecimal(domain.PrecioMinimoValido)
	valido.EnStock = punteroBooleano(true)
	valido.CapturadoEl = punteroFecha(fresco)

	repositorio := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{basura, valido}}
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{productoBasuraID})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 1 || len(respuesta.Productos[0].Ofertas) != 2 {
		prueba.Fatalf("se esperaban 2 ofertas en el grupo: %+v", respuesta.Productos)
	}

	ofertaBasura := respuesta.Productos[0].Ofertas[0]
	if ofertaBasura.EstadoPrecio != domain.EstadoPrecioSinCaptura || ofertaBasura.Precio != nil ||
		ofertaBasura.PrecioNormal != nil || ofertaBasura.EnStock != nil || ofertaBasura.CapturadoEl != nil {
		prueba.Fatalf("la oferta con precio < %d no debe publicar precio ni stock: %+v", domain.PrecioMinimoValido, ofertaBasura)
	}

	ofertaValida := respuesta.Productos[0].Ofertas[1]
	if ofertaValida.EstadoPrecio != domain.EstadoPrecioVigente || ofertaValida.Precio == nil ||
		*ofertaValida.Precio != domain.PrecioMinimoValido {
		prueba.Fatalf("la oferta en el umbral (%d) debe publicarse vigente: %+v", domain.PrecioMinimoValido, ofertaValida)
	}
}

// TestEquivalenciasServiceIgnoraFormatoGenericoUnidad comprueba que un marcador
// genérico de unidad de venta ("1 un", "1 unidad", "Format") no bloquee la
// equivalencia por EAN frente a un formato específico ("1 Kg"), mientras dos
// formatos específicos distintos siguen reportando conflicto.
func TestEquivalenciasServiceIgnoraFormatoGenericoUnidad(prueba *testing.T) {
	momento := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	fresco := momento.Add(-1 * time.Hour)

	arrozACuentaID := "00000000-0000-4000-8000-000000000031"
	arrozCugatID := "00000000-0000-4000-8000-000000000032"
	arrozSantaIsabelID := "00000000-0000-4000-8000-000000000033"

	arrozACuenta := registroEquivalenciaBase(arrozACuentaID, 5, "A Cuenta")
	arrozACuenta.CodigoEAN = referenciaCadena("7801420140139")
	arrozACuenta.Formato = "1 Kg"
	arrozACuenta.PrecioNormal = punteroDecimal(1690)
	arrozACuenta.EnStock = punteroBooleano(true)
	arrozACuenta.CapturadoEl = punteroFecha(fresco)

	arrozCugat := registroEquivalenciaBase(arrozCugatID, 6, "Cugat")
	arrozCugat.CodigoEAN = referenciaCadena("7801420140139")
	arrozCugat.Formato = ""
	arrozCugat.PrecioNormal = punteroDecimal(2390)
	arrozCugat.EnStock = punteroBooleano(true)
	arrozCugat.CapturadoEl = punteroFecha(fresco)

	arrozSantaIsabel := registroEquivalenciaBase(arrozSantaIsabelID, 4, "Santa Isabel")
	arrozSantaIsabel.CodigoEAN = referenciaCadena("7801420140139")
	arrozSantaIsabel.Formato = "1 un"
	arrozSantaIsabel.PrecioNormal = punteroDecimal(2570)
	arrozSantaIsabel.EnStock = punteroBooleano(true)
	arrozSantaIsabel.CapturadoEl = punteroFecha(fresco)

	repositorio := &repositorioEquivalenciasDoble{registros: []domain.ProductoEquivalenciaRegistro{
		arrozACuenta, arrozCugat, arrozSantaIsabel,
	}}
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, relojFijo(momento))

	respuesta, errorObtenido := servicio.Consultar(context.Background(), []string{arrozCugatID})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(respuesta.Productos) != 1 {
		prueba.Fatalf("se esperaba 1 producto, se obtuvieron %d", len(respuesta.Productos))
	}
	item := respuesta.Productos[0]
	if item.Estado != domain.EstadoEquivalenciaComparable {
		prueba.Fatalf("estado = %q; esperado %q", item.Estado, domain.EstadoEquivalenciaComparable)
	}
	if len(item.Ofertas) != 3 {
		prueba.Fatalf("se esperaban 3 ofertas comparables entre cadenas, se obtuvieron %d: %+v", len(item.Ofertas), item.Ofertas)
	}
}

// TestEquivalenciasServicePropagaError verifica que un fallo de persistencia no
// se transforme en una respuesta válida.
func TestEquivalenciasServicePropagaError(prueba *testing.T) {
	errorPersistencia := errors.New("fallo de persistencia")
	repositorio := &repositorioEquivalenciasDoble{errorConsulta: errorPersistencia}
	servicio := services.NuevoServicioEquivalencias(repositorio,
		services.LimitesEquivalencia{MaximoProductos: 10, AntiguedadMaxima: 48 * time.Hour}, time.Now)

	_, errorObtenido := servicio.Consultar(context.Background(),
		[]string{"00000000-0000-4000-8000-000000000001"})
	if !errors.Is(errorObtenido, errorPersistencia) {
		prueba.Fatalf("error = %v; esperado %v", errorObtenido, errorPersistencia)
	}
}

// referenciaCadena devuelve un puntero a una copia del texto recibido.
func referenciaCadena(valor string) *string {
	return &valor
}

// Garantiza que el doble de pruebas cumpla la interfaz del dominio.
var _ domain.RepositorioEquivalencias = (*repositorioEquivalenciasDoble)(nil)
