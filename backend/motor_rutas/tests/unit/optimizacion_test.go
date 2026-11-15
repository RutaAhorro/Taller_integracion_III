// Pruebas de caja negra del cálculo de rutas (SUP-313 y SUP-328). Los casos
// A, B y C son los de la sección 5 del contrato del motor de rutas: usan las
// coordenadas de sucursales de infrastructure/db/init.sql y precios
// inventados para el ejemplo, creados aquí mismo y no en la aplicación.
package unit_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/optimizacion"
)

// =============================================================================
// Datos de los casos del contrato
// =============================================================================

// origenContrato es el punto de partida común de los casos.
var origenContrato = optimizacion.Coordenada{Lat: -38.739, Lon: -72.598}

// Identificadores de las sucursales del contrato.
const (
	sucursalJumbo   = 1
	sucursalLider   = 2
	sucursalUnimarc = 3
)

// sucursalesContrato son las tres sucursales de init.sql usadas en los casos.
var sucursalesContrato = []optimizacion.Sucursal{
	{ID: sucursalJumbo, Nombre: "Jumbo Los Pablos", Cadena: "Jumbo", Ubicacion: optimizacion.Coordenada{Lat: -38.741, Lon: -72.6}},
	{ID: sucursalLider, Nombre: "Lider Prieto Norte", Cadena: "Lider", Ubicacion: optimizacion.Coordenada{Lat: -38.735, Lon: -72.59}},
	{ID: sucursalUnimarc, Nombre: "Unimarc Av. Alemania", Cadena: "Unimarc", Ubicacion: optimizacion.Coordenada{Lat: -38.745, Lon: -72.61}},
}

// parametrosContrato son los parámetros comunes de la sección 5.
func parametrosContrato(umbralAhorro int) optimizacion.Parametros {
	return optimizacion.Parametros{
		MaxParadas:         3,
		FactorDistancia:    1.3,
		RendimientoKmL:     12,
		PrecioCombustibleL: 1300,
		UmbralAhorro:       umbralAhorro,
	}
}

// umbralContrato es el valor inicial de RUTAS_UMBRAL_AHORRO del contrato.
const umbralContrato = 500

// preciosPorSucursal es el precio de un producto en cada sucursal que lo vende.
type preciosPorSucursal map[int]int

// armarEntrada crea la entrada del cálculo a partir de tablas de precios. El
// producto que se compra en cada sucursal se nombra "<producto>-<sucursal>"
// para comprobar que se informa producto_id_oferta.
func armarEntrada(articulos []optimizacion.Articulo, precios map[string]preciosPorSucursal, parametros optimizacion.Parametros) optimizacion.Entrada {
	entrada := optimizacion.Entrada{
		Origen:     origenContrato,
		Articulos:  articulos,
		Sucursales: slices.Clone(sucursalesContrato),
		Parametros: parametros,
	}
	for _, articulo := range articulos {
		for sucursalID, precio := range precios[articulo.ProductoID] {
			entrada.Ofertas = append(entrada.Ofertas, optimizacion.Oferta{
				ProductoID:       articulo.ProductoID,
				ProductoIDOferta: articulo.ProductoID + "-" + nombreSucursal(sucursalID),
				SucursalID:       sucursalID,
				Precio:           precio,
				EnStock:          true,
			})
		}
	}
	return entrada
}

// nombreSucursal da un sufijo legible para producto_id_oferta.
func nombreSucursal(sucursalID int) string {
	return map[int]string{sucursalJumbo: "jumbo", sucursalLider: "lider", sucursalUnimarc: "unimarc"}[sucursalID]
}

// listaCasoB es la lista de los casos B y C: leche ×2, arroz, detergente y quinoa.
var listaCasoB = []optimizacion.Articulo{
	{ProductoID: "leche", Cantidad: 2},
	{ProductoID: "arroz", Cantidad: 1},
	{ProductoID: "detergente", Cantidad: 1},
	{ProductoID: "quinoa", Cantidad: 1},
}

// preciosCasoB arma los precios del caso B con la leche de Jumbo indicada.
// El caso B usa $690 y el caso C, $990. La quinoa no está en ninguna tienda.
func preciosCasoB(lecheJumbo int) map[string]preciosPorSucursal {
	return map[string]preciosPorSucursal{
		"leche":      {sucursalJumbo: lecheJumbo, sucursalLider: 1190},
		"arroz":      {sucursalJumbo: 1690, sucursalLider: 1590, sucursalUnimarc: 1490},
		"detergente": {sucursalJumbo: 9990, sucursalLider: 6990, sucursalUnimarc: 7490},
	}
}

// =============================================================================
// Ayudantes de verificación
// =============================================================================

// calcularSinError ejecuta el cálculo y falla la prueba si devuelve error.
func calcularSinError(prueba *testing.T, entrada optimizacion.Entrada) optimizacion.Resultado {
	prueba.Helper()
	resultado, errorCalculo := optimizacion.Calcular(entrada)
	if errorCalculo != nil {
		prueba.Fatalf("Calcular devolvió error: %v", errorCalculo)
	}
	return resultado
}

// idsParadas devuelve los sucursal_id de la ruta en orden de visita.
func idsParadas(ruta optimizacion.Ruta) []int {
	ids := []int{}
	for _, parada := range ruta.Paradas {
		ids = append(ids, parada.SucursalID)
	}
	return ids
}

// verificarRuta compara paradas, costos y distancia con lo esperado.
func verificarRuta(prueba *testing.T, ruta optimizacion.Ruta, paradas []int, costoProductos, costoTraslado, total int, distanciaKm float64) {
	prueba.Helper()
	if !slices.Equal(idsParadas(ruta), paradas) {
		prueba.Fatalf("paradas = %v, se esperaba %v", idsParadas(ruta), paradas)
	}
	if ruta.CostoProductos != costoProductos || ruta.CostoTraslado != costoTraslado || ruta.Total != total || ruta.DistanciaKm != distanciaKm {
		prueba.Fatalf("ruta = productos %d, traslado %d, total %d, %.2f km; se esperaba %d, %d, %d, %.2f km",
			ruta.CostoProductos, ruta.CostoTraslado, ruta.Total, ruta.DistanciaKm, costoProductos, costoTraslado, total, distanciaKm)
	}
}

// =============================================================================
// Casos del contrato (sección 5)
// =============================================================================

// TestCasoAUnaTiendaAlcanza: comprar todo en Jumbo es lo más barato.
func TestCasoAUnaTiendaAlcanza(prueba *testing.T) {
	articulos := []optimizacion.Articulo{
		{ProductoID: "leche", Cantidad: 2},
		{ProductoID: "arroz", Cantidad: 1},
		{ProductoID: "aceite", Cantidad: 1},
	}
	precios := map[string]preciosPorSucursal{
		"leche":  {sucursalJumbo: 1150, sucursalLider: 1190, sucursalUnimarc: 1250},
		"arroz":  {sucursalJumbo: 1690, sucursalLider: 1590, sucursalUnimarc: 1650},
		"aceite": {sucursalJumbo: 2890, sucursalLider: 2990, sucursalUnimarc: 3190},
	}
	resultado := calcularSinError(prueba, armarEntrada(articulos, precios, parametrosContrato(umbralContrato)))

	verificarRuta(prueba, resultado.Ruta, []int{sucursalJumbo}, 6880, 79, 6959, 0.73)
	if resultado.BaseUnaTienda == nil || resultado.BaseUnaTienda.SucursalID != sucursalJumbo || resultado.BaseUnaTienda.Total != 6959 {
		prueba.Fatalf("base = %+v, se esperaba Jumbo con $6.959", resultado.BaseUnaTienda)
	}
	if resultado.Ahorro == nil || *resultado.Ahorro != 0 {
		prueba.Fatalf("ahorro = %v, se esperaba 0", resultado.Ahorro)
	}
	if len(resultado.NoCubiertos) != 0 || resultado.CandidatasEvaluadas != 3 {
		prueba.Fatalf("no_cubiertos = %v, candidatas = %d", resultado.NoCubiertos, resultado.CandidatasEvaluadas)
	}
}

// TestCasoBConvieneCombinarDosTiendas: Jumbo → Lider ahorra $922, más que el
// umbral; la tercera parada en Unimarc no conviene y la quinoa no se encuentra.
func TestCasoBConvieneCombinarDosTiendas(prueba *testing.T) {
	resultado := calcularSinError(prueba, armarEntrada(listaCasoB, preciosCasoB(690), parametrosContrato(umbralContrato)))

	verificarRuta(prueba, resultado.Ruta, []int{sucursalJumbo, sucursalLider}, 9960, 310, 10270, 2.86)
	if resultado.BaseUnaTienda == nil || resultado.BaseUnaTienda.SucursalID != sucursalLider || resultado.BaseUnaTienda.Total != 11192 {
		prueba.Fatalf("base = %+v, se esperaba Lider con $11.192", resultado.BaseUnaTienda)
	}
	if resultado.Ahorro == nil || *resultado.Ahorro != 922 {
		prueba.Fatalf("ahorro = %v, se esperaba 922", resultado.Ahorro)
	}
	esperadoNoCubierto := []optimizacion.NoCubierto{{ProductoID: "quinoa", Motivo: optimizacion.MotivoSinOfertaEnRadio}}
	if !slices.Equal(resultado.NoCubiertos, esperadoNoCubierto) {
		prueba.Fatalf("no_cubiertos = %v", resultado.NoCubiertos)
	}

	// Qué se compra en cada parada y con qué producto de la sucursal.
	jumbo, lider := resultado.Ruta.Paradas[0], resultado.Ruta.Paradas[1]
	esperadoJumbo := []optimizacion.ArticuloAsignado{{ProductoID: "leche", ProductoIDOferta: "leche-jumbo", Cantidad: 2, PrecioUnitario: 690, Subtotal: 1380}}
	if jumbo.Orden != 1 || jumbo.Subtotal != 1380 || !slices.Equal(jumbo.Articulos, esperadoJumbo) {
		prueba.Fatalf("parada Jumbo = %+v", jumbo)
	}
	if lider.Orden != 2 || lider.Subtotal != 8580 || len(lider.Articulos) != 2 ||
		lider.Articulos[0].ProductoIDOferta != "arroz-lider" || lider.Articulos[1].ProductoIDOferta != "detergente-lider" {
		prueba.Fatalf("parada Lider = %+v", lider)
	}
}

// TestCasoBTramosEnJSON comprueba la forma de los tramos del contrato: el
// origen se escribe "origen" y cada sucursal, su sucursal_id.
func TestCasoBTramosEnJSON(prueba *testing.T) {
	resultado := calcularSinError(prueba, armarEntrada(listaCasoB, preciosCasoB(690), parametrosContrato(umbralContrato)))
	cuerpo, errorJSON := json.Marshal(resultado.Ruta.Tramos)
	if errorJSON != nil {
		prueba.Fatal(errorJSON)
	}
	esperado := `[{"desde":"origen","hasta":1,"distancia_km":0.37},{"desde":1,"hasta":2,"distancia_km":1.42},{"desde":2,"hasta":"origen","distancia_km":1.07}]`
	if string(cuerpo) != esperado {
		prueba.Fatalf("tramos = %s\nse esperaba %s", cuerpo, esperado)
	}
}

// TestCasoCElAhorroNoSuperaElUmbral: Jumbo → Lider ahorra solo $322, que no
// supera los $500 del umbral, así que se recomienda comprar todo en Lider.
func TestCasoCElAhorroNoSuperaElUmbral(prueba *testing.T) {
	resultado := calcularSinError(prueba, armarEntrada(listaCasoB, preciosCasoB(990), parametrosContrato(umbralContrato)))

	verificarRuta(prueba, resultado.Ruta, []int{sucursalLider}, 10960, 232, 11192, 2.14)
	if resultado.BaseUnaTienda == nil || resultado.BaseUnaTienda.SucursalID != sucursalLider {
		prueba.Fatalf("base = %+v, se esperaba Lider", resultado.BaseUnaTienda)
	}
	if resultado.Ahorro == nil || *resultado.Ahorro != 0 {
		prueba.Fatalf("ahorro = %v, se esperaba 0", resultado.Ahorro)
	}
	if len(resultado.NoCubiertos) != 1 || resultado.NoCubiertos[0].ProductoID != "quinoa" {
		prueba.Fatalf("no_cubiertos = %v", resultado.NoCubiertos)
	}
}

// TestUmbralCeroRecomiendaCualquierAhorro: con el mismo caso C y
// RUTAS_UMBRAL_AHORRO=0, la combinación que ahorra $322 sí se recomienda.
func TestUmbralCeroRecomiendaCualquierAhorro(prueba *testing.T) {
	resultado := calcularSinError(prueba, armarEntrada(listaCasoB, preciosCasoB(990), parametrosContrato(0)))

	verificarRuta(prueba, resultado.Ruta, []int{sucursalJumbo, sucursalLider}, 10560, 310, 10870, 2.86)
	if resultado.Ahorro == nil || *resultado.Ahorro != 322 {
		prueba.Fatalf("ahorro = %v, se esperaba 322", resultado.Ahorro)
	}
}

// =============================================================================
// Orden de visita, cobertura y empates
// =============================================================================

// preciosExclusivos deja cada producto en una sola tienda, de modo que la
// única forma de comprar todo es visitar las tres.
var preciosExclusivos = map[string]preciosPorSucursal{
	"leche":      {sucursalJumbo: 990},
	"arroz":      {sucursalUnimarc: 1490},
	"detergente": {sucursalLider: 6990},
}

// listaExclusiva son los tres productos de preciosExclusivos.
var listaExclusiva = []optimizacion.Articulo{
	{ProductoID: "leche", Cantidad: 1},
	{ProductoID: "arroz", Cantidad: 1},
	{ProductoID: "detergente", Cantidad: 1},
}

// TestOrdenDeVisitaConTresTiendas: entre un recorrido y su inverso (misma
// distancia) se visita primero la tienda más cercana al origen. Sin tienda que
// alcance sola, no hay base ni ahorro.
func TestOrdenDeVisitaConTresTiendas(prueba *testing.T) {
	resultado := calcularSinError(prueba, armarEntrada(listaExclusiva, preciosExclusivos, parametrosContrato(umbralContrato)))

	if obtenidas := idsParadas(resultado.Ruta); !slices.Equal(obtenidas, []int{sucursalLider, sucursalJumbo, sucursalUnimarc}) {
		prueba.Fatalf("orden = %v, se esperaba Lider → Jumbo → Unimarc", obtenidas)
	}
	if resultado.Ruta.DistanciaKm != 5.37 || len(resultado.Ruta.Tramos) != 4 {
		prueba.Fatalf("distancia = %.2f, tramos = %d", resultado.Ruta.DistanciaKm, len(resultado.Ruta.Tramos))
	}
	if resultado.BaseUnaTienda != nil || resultado.Ahorro != nil {
		prueba.Fatalf("sin tienda suficiente, base y ahorro deben ser null: %+v %v", resultado.BaseUnaTienda, resultado.Ahorro)
	}
	cuerpo, _ := json.Marshal(resultado)
	if !strings.Contains(string(cuerpo), `"base_una_tienda":null`) || !strings.Contains(string(cuerpo), `"ahorro":null`) {
		prueba.Fatalf("JSON sin null explícito: %s", cuerpo)
	}
}

// TestNingunConjuntoCubreLaCompra: si hacen falta tres tiendas y el máximo es
// dos, no se inventa una ruta parcial.
func TestNingunConjuntoCubreLaCompra(prueba *testing.T) {
	parametros := parametrosContrato(umbralContrato)
	parametros.MaxParadas = 2
	_, errorCalculo := optimizacion.Calcular(armarEntrada(listaExclusiva, preciosExclusivos, parametros))

	var sinCobertura *optimizacion.ErrorSinCobertura
	if !errors.As(errorCalculo, &sinCobertura) || len(sinCobertura.NoCubiertos) != 0 {
		prueba.Fatalf("error = %v, se esperaba ErrorSinCobertura sin productos faltantes", errorCalculo)
	}
}

// TestNingunArticuloTieneOferta: el error lista los productos sin cobertura.
// Una oferta sin stock cuenta como inexistente.
func TestNingunArticuloTieneOferta(prueba *testing.T) {
	entrada := armarEntrada([]optimizacion.Articulo{{ProductoID: "quinoa", Cantidad: 1}, {ProductoID: "chia", Cantidad: 2}}, nil, parametrosContrato(umbralContrato))
	entrada.Ofertas = []optimizacion.Oferta{{ProductoID: "chia", ProductoIDOferta: "chia-jumbo", SucursalID: sucursalJumbo, Precio: 2500, EnStock: false}}
	_, errorCalculo := optimizacion.Calcular(entrada)

	var sinCobertura *optimizacion.ErrorSinCobertura
	if !errors.As(errorCalculo, &sinCobertura) || !slices.Equal(sinCobertura.NoCubiertos, []string{"quinoa", "chia"}) {
		prueba.Fatalf("error = %v, se esperaba ErrorSinCobertura con quinoa y chia", errorCalculo)
	}
}

// TestEmpateDeterminista: dos sucursales iguales en ubicación y precio; gana
// la de menor sucursal_id, sin importar el orden en que llegan los datos.
func TestEmpateDeterminista(prueba *testing.T) {
	ubicacion := optimizacion.Coordenada{Lat: -38.740, Lon: -72.599}
	gemelas := []optimizacion.Sucursal{
		{ID: 8, Nombre: "Tienda B", Cadena: "Cadena", Ubicacion: ubicacion},
		{ID: 7, Nombre: "Tienda A", Cadena: "Cadena", Ubicacion: ubicacion},
	}
	ofertas := []optimizacion.Oferta{
		{ProductoID: "pan", ProductoIDOferta: "pan-b", SucursalID: 8, Precio: 1000, EnStock: true},
		{ProductoID: "pan", ProductoIDOferta: "pan-a", SucursalID: 7, Precio: 1000, EnStock: true},
	}
	for _, invertir := range []bool{false, true} {
		entrada := optimizacion.Entrada{
			Origen:     origenContrato,
			Articulos:  []optimizacion.Articulo{{ProductoID: "pan", Cantidad: 1}},
			Sucursales: slices.Clone(gemelas),
			Ofertas:    slices.Clone(ofertas),
			Parametros: parametrosContrato(umbralContrato),
		}
		if invertir {
			slices.Reverse(entrada.Sucursales)
			slices.Reverse(entrada.Ofertas)
		}
		resultado := calcularSinError(prueba, entrada)
		if obtenidas := idsParadas(resultado.Ruta); !slices.Equal(obtenidas, []int{7}) {
			prueba.Fatalf("invertir=%v: paradas = %v, se esperaba [7]", invertir, obtenidas)
		}
	}
}

// TestOfertaMasBarataEntreEquivalentes: con dos productos equivalentes en la
// misma sucursal se compra el más barato y se informa cuál fue.
func TestOfertaMasBarataEntreEquivalentes(prueba *testing.T) {
	entrada := armarEntrada([]optimizacion.Articulo{{ProductoID: "leche", Cantidad: 3}}, nil, parametrosContrato(umbralContrato))
	entrada.Ofertas = []optimizacion.Oferta{
		{ProductoID: "leche", ProductoIDOferta: "leche-marca-a", SucursalID: sucursalJumbo, Precio: 1100, EnStock: true},
		{ProductoID: "leche", ProductoIDOferta: "leche-marca-b", SucursalID: sucursalJumbo, Precio: 950, EnStock: true},
	}
	resultado := calcularSinError(prueba, entrada)
	articulo := resultado.Ruta.Paradas[0].Articulos[0]
	if articulo.ProductoIDOferta != "leche-marca-b" || articulo.Subtotal != 2850 {
		prueba.Fatalf("artículo = %+v, se esperaba leche-marca-b por $2.850", articulo)
	}
}

// =============================================================================
// Asignación por separado (SUP-313)
// =============================================================================

// TestAsignarConjuntosDescartaIncompletosYTiendasSobrantes: en el caso B hay
// 7 conjuntos posibles; {Unimarc} no cubre la leche y {Jumbo, Lider, Unimarc}
// sí, porque en cada tienda se compra algo. Quedan 6 alternativas.
func TestAsignarConjuntosDescartaIncompletosYTiendasSobrantes(prueba *testing.T) {
	asignacion, errorAsignacion := optimizacion.AsignarConjuntos(armarEntrada(listaCasoB, preciosCasoB(690), parametrosContrato(umbralContrato)))
	if errorAsignacion != nil {
		prueba.Fatal(errorAsignacion)
	}
	var conjuntos [][]int
	for _, alternativa := range asignacion.Alternativas {
		ids := []int{}
		for _, sucursal := range alternativa.Sucursales {
			ids = append(ids, sucursal.ID)
		}
		conjuntos = append(conjuntos, ids)
	}
	esperados := [][]int{{1}, {2}, {1, 2}, {1, 3}, {2, 3}, {1, 2, 3}}
	if !slices.EqualFunc(conjuntos, esperados, slices.Equal[[]int]) {
		prueba.Fatalf("conjuntos = %v, se esperaba %v", conjuntos, esperados)
	}
}

// TestAsignarConjuntosSumaArticulosRepetidos: el mismo producto dos veces en
// la lista se compra una vez con la cantidad total.
func TestAsignarConjuntosSumaArticulosRepetidos(prueba *testing.T) {
	articulos := []optimizacion.Articulo{{ProductoID: "arroz", Cantidad: 1}, {ProductoID: "arroz", Cantidad: 2}}
	precios := map[string]preciosPorSucursal{"arroz": {sucursalLider: 1590}}
	asignacion, errorAsignacion := optimizacion.AsignarConjuntos(armarEntrada(articulos, precios, parametrosContrato(umbralContrato)))
	if errorAsignacion != nil {
		prueba.Fatal(errorAsignacion)
	}
	if len(asignacion.Alternativas) != 1 || asignacion.Alternativas[0].CostoProductos != 4770 {
		prueba.Fatalf("alternativas = %+v, se esperaba una de $4.770", asignacion.Alternativas)
	}
}

// =============================================================================
// Entradas inválidas
// =============================================================================

// TestEntradasInvalidas: el cálculo falla rápido en lugar de inventar valores.
func TestEntradasInvalidas(prueba *testing.T) {
	valida := func() optimizacion.Entrada {
		return armarEntrada(listaExclusiva, preciosExclusivos, parametrosContrato(umbralContrato))
	}
	casos := []struct {
		nombre   string
		ajustar  func(entrada *optimizacion.Entrada)
		esperado error
	}{
		{"sin artículos", func(entrada *optimizacion.Entrada) { entrada.Articulos = nil }, optimizacion.ErrSinArticulos},
		{"sin sucursales", func(entrada *optimizacion.Entrada) { entrada.Sucursales = nil }, optimizacion.ErrSinSucursales},
		{"cantidad cero", func(entrada *optimizacion.Entrada) { entrada.Articulos[0].Cantidad = 0 }, optimizacion.ErrParametros},
		{"rendimiento cero", func(entrada *optimizacion.Entrada) { entrada.Parametros.RendimientoKmL = 0 }, optimizacion.ErrParametros},
		{"factor negativo", func(entrada *optimizacion.Entrada) { entrada.Parametros.FactorDistancia = -1 }, optimizacion.ErrParametros},
		{"combustible cero", func(entrada *optimizacion.Entrada) { entrada.Parametros.PrecioCombustibleL = 0 }, optimizacion.ErrParametros},
		{"sin paradas", func(entrada *optimizacion.Entrada) { entrada.Parametros.MaxParadas = 0 }, optimizacion.ErrParametros},
		{"umbral negativo", func(entrada *optimizacion.Entrada) { entrada.Parametros.UmbralAhorro = -1 }, optimizacion.ErrParametros},
		{"origen fuera de rango", func(entrada *optimizacion.Entrada) { entrada.Origen.Lat = 91 }, optimizacion.ErrParametros},
		{"sucursal repetida", func(entrada *optimizacion.Entrada) {
			entrada.Sucursales = append(entrada.Sucursales, entrada.Sucursales[0])
		}, optimizacion.ErrParametros},
		{"precio cero", func(entrada *optimizacion.Entrada) { entrada.Ofertas[0].Precio = 0 }, optimizacion.ErrParametros},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			entrada := valida()
			caso.ajustar(&entrada)
			if _, errorCalculo := optimizacion.Calcular(entrada); !errors.Is(errorCalculo, caso.esperado) {
				subprueba.Fatalf("error = %v, se esperaba %v", errorCalculo, caso.esperado)
			}
		})
	}
}
