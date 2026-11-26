// Pruebas unitarias HTTP de POST /api/v1/rutas/optimizar (MVP).
package unit_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/httpapi"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// =============================================================================
// Dobles de prueba para precios comparables
// =============================================================================

const rutaOptimizar = "/api/v1/rutas/optimizar"

// preciosFalso simula POST /internal/v1/productos/precios-comparables de Catálogo.
type preciosFalso struct {
	mu             sync.Mutex
	respuesta      listas.RespuestaPreciosComparables
	caido          bool
	ultimaConsulta *listas.ConsultaPreciosComparables
	totalConsultas int
}

func (falso *preciosFalso) ConsultarPreciosComparables(_ context.Context, consulta listas.ConsultaPreciosComparables) (listas.RespuestaPreciosComparables, error) {
	falso.mu.Lock()
	defer falso.mu.Unlock()
	falso.totalConsultas++
	copia := consulta
	falso.ultimaConsulta = &copia
	if falso.caido {
		return listas.RespuestaPreciosComparables{}, listas.ErrCatalogoNoDisponible
	}
	return falso.respuesta, nil
}

// nuevoClienteRutas arma un servidor HTTP completo con catálogo de precios falso.
func nuevoClienteRutas(prueba *testing.T, precios *preciosFalso, opcionesOpt httpapi.OpcionesOptimizacion) *cliente {
	prueba.Helper()
	repo := nuevaMemoria()
	catalogo := &catalogoFalso{estados: map[string]listas.Disponibilidad{}}
	manejador := httpapi.Nuevo(httpapi.Opciones{
		Repo:         repo,
		Catalogo:     catalogo,
		Precios:      precios,
		Optimizacion: opcionesOpt,
		JWTSecret:    secreto,
		OrigenesCORS: []string{origen},
		MaxListas:    100,
		Limites:      listas.Limites{MaxArticulos: 10, MaxCantidad: 10},
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return &cliente{prueba: prueba, manejador: manejador, repo: repo, catalogo: catalogo, jwt: tokenDe(prueba, usuarioA)}
}

// ptrFloat y ptrBool ayudan a construir ofertas comparables con punteros.
func ptrFloat(valor float64) *float64 { return &valor }
func ptrBool(valor bool) *bool        { return &valor }

// crearListaConArticulos siembra una lista del usuario indicado directamente en memoria.
func crearListaConArticulos(prueba *testing.T, repo *repositorioMemoria, usuarioID string, articulos []listas.ArticuloLote) string {
	prueba.Helper()
	lista, errorCrear := repo.Crear(prueba.Context(), usuarioID, "Compras semana")
	if errorCrear != nil {
		prueba.Fatal(errorCrear)
	}
	for _, articulo := range articulos {
		if _, _, errorAgregar := repo.AgregarArticulo(prueba.Context(), usuarioID, lista.ID, articulo.ProductoID, articulo.Cantidad, listas.LimitesContrato); errorAgregar != nil {
			prueba.Fatal(errorAgregar)
		}
	}
	return lista.ID
}

// sucursalesTemucoPrueba devuelve 3 sucursales en Temuco (incluye Unimarc sin precios).
func sucursalesTemucoPrueba() []listas.SucursalCandidata {
	return []listas.SucursalCandidata{
		{SucursalID: 5, Nombre: "SuperBodega aCuenta Temuco Matta", Cadena: "A Cuenta", Lat: -38.7338383, Lon: -72.5806422, DistanciaKm: 0.88},
		{SucursalID: 6, Nombre: "Cugat Temuco Barros Arana", Cadena: "Cugat", Lat: -38.7385969, Lon: -72.5813372, DistanciaKm: 0.84},
		{SucursalID: 3, Nombre: "Unimarc Av. Alemania", Cadena: "Unimarc", Lat: -38.745, Lon: -72.610, DistanciaKm: 1.98},
	}
}

// =============================================================================
// Casos de éxito (200 OK)
// =============================================================================

func TestOptimizarRutaSinOrigenUsaCentroTemucoYCombinaCadenas(prueba *testing.T) {
	precios := &preciosFalso{
		respuesta: listas.RespuestaPreciosComparables{
			Sucursales: sucursalesTemucoPrueba(),
			Productos: []listas.ProductoPreciosComparable{
				{
					ProductoID: productoLeche,
					Estado:     "comparable",
					Ofertas: []listas.OfertaComparable{
						{ProductoIDOferta: productoLeche, SucursalID: 5, Cadena: "A Cuenta", Nombre: "Leche 1 L", Precio: ptrFloat(700), EnStock: ptrBool(true)},
						{ProductoIDOferta: productoLeche, SucursalID: 6, Cadena: "Cugat", Nombre: "Leche 1 L", Precio: ptrFloat(1200), EnStock: ptrBool(true)},
						// Oferta sin precio (debe ignorarse sin error) y oferta sin stock en Unimarc.
						{ProductoIDOferta: productoLeche, SucursalID: 3, Cadena: "Unimarc", Nombre: "Leche 1 L", Precio: nil, EnStock: ptrBool(true)},
					},
				},
				{
					ProductoID: productoPan,
					Estado:     "comparable",
					Ofertas: []listas.OfertaComparable{
						{ProductoIDOferta: productoPan, SucursalID: 5, Cadena: "A Cuenta", Nombre: "Detergente 3 L", Precio: ptrFloat(9900), EnStock: ptrBool(true)},
						{ProductoIDOferta: productoPan, SucursalID: 6, Cadena: "Cugat", Nombre: "Detergente 3 L", Precio: ptrFloat(6900), EnStock: ptrBool(true)},
						{ProductoIDOferta: productoPan, SucursalID: 3, Cadena: "Unimarc", Nombre: "Detergente 3 L", Precio: ptrFloat(5000), EnStock: ptrBool(false)},
					},
				},
			},
			SinOfertas: []string{productoArroz},
		},
	}

	clienteHTTP := nuevoClienteRutas(prueba, precios, httpapi.OpcionesOptimizacion{})
	listaID := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioA, []listas.ArticuloLote{
		{ProductoID: productoLeche, Cantidad: 2},
		{ProductoID: productoPan, Cantidad: 1},
		{ProductoID: productoArroz, Cantidad: 1},
	})

	respuesta := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaID+`"}`, nil)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado = %d, cuerpo = %s", respuesta.Code, respuesta.Body.String())
	}

	var cuerpo struct {
		Estimado bool `json:"estimado"`
		Origen   struct {
			Lat        float64 `json:"lat"`
			Lon        float64 `json:"lon"`
			PorDefecto bool    `json:"por_defecto"`
		} `json:"origen"`
		Parametros struct {
			UmbralAhorro int     `json:"umbral_ahorro"`
			MaxParadas   int     `json:"max_paradas"`
			RadioKm      float64 `json:"radio_km"`
		} `json:"parametros"`
		Ruta struct {
			Paradas []struct {
				Orden      int    `json:"orden"`
				SucursalID int    `json:"sucursal_id"`
				Cadena     string `json:"cadena"`
				Subtotal   int    `json:"subtotal"`
			} `json:"paradas"`
			CostoProductos int `json:"costo_productos"`
			CostoTraslado  int `json:"costo_traslado"`
			Total          int `json:"total"`
		} `json:"ruta"`
		BaseUnaTienda *struct {
			SucursalID int    `json:"sucursal_id"`
			Cadena     string `json:"cadena"`
			Total      int    `json:"total"`
		} `json:"base_una_tienda"`
		Ahorro      *int `json:"ahorro"`
		NoCubiertos []struct {
			ProductoID string `json:"producto_id"`
			Motivo     string `json:"motivo"`
		} `json:"no_cubiertos"`
		CandidatasEvaluadas int `json:"candidatas_evaluadas"`
	}
	if errorJSON := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorJSON != nil {
		prueba.Fatal(errorJSON)
	}

	if !cuerpo.Estimado || !cuerpo.Origen.PorDefecto || cuerpo.Origen.Lat != -38.7359 || cuerpo.Origen.Lon != -72.5904 {
		prueba.Fatalf("origen = %+v, se esperaba centro de Temuco por defecto", cuerpo.Origen)
	}
	if precios.ultimaConsulta == nil || precios.ultimaConsulta.Lat != -38.7359 || precios.ultimaConsulta.Lon != -72.5904 {
		prueba.Fatalf("consulta a Catálogo = %+v", precios.ultimaConsulta)
	}
	if cuerpo.Ruta.CostoTraslado != 0 || cuerpo.Ruta.Total != 8300 || cuerpo.Ruta.CostoProductos != 8300 {
		prueba.Fatalf("costos = %+v, se esperaba total=8300 sin traslado", cuerpo.Ruta)
	}
	if len(cuerpo.Ruta.Paradas) != 2 {
		prueba.Fatalf("paradas = %+v, se esperaban 2 paradas (A Cuenta y Cugat)", cuerpo.Ruta.Paradas)
	}
	if cuerpo.BaseUnaTienda == nil || cuerpo.BaseUnaTienda.SucursalID != 6 || cuerpo.BaseUnaTienda.Total != 9300 {
		prueba.Fatalf("base = %+v, se esperaba Cugat con 9300", cuerpo.BaseUnaTienda)
	}
	if cuerpo.Ahorro == nil || *cuerpo.Ahorro != 1000 {
		prueba.Fatalf("ahorro = %v, se esperaba 1000", cuerpo.Ahorro)
	}
	if len(cuerpo.NoCubiertos) != 1 || cuerpo.NoCubiertos[0].ProductoID != productoArroz {
		prueba.Fatalf("no_cubiertos = %+v, se esperaba arroz", cuerpo.NoCubiertos)
	}
	if cuerpo.CandidatasEvaluadas != 3 {
		prueba.Fatalf("candidatas = %d, se esperaba 3", cuerpo.CandidatasEvaluadas)
	}
}

func TestOptimizarRutaConOrigenExplicitoYUmbralGanaUnaTienda(prueba *testing.T) {
	// Combinar ahorra solo $100 (<= umbral $500) → ruta recomendada tiene 1 parada y ahorro = 0.
	precios := &preciosFalso{
		respuesta: listas.RespuestaPreciosComparables{
			Sucursales: sucursalesTemucoPrueba(),
			Productos: []listas.ProductoPreciosComparable{
				{
					ProductoID: productoLeche,
					Ofertas: []listas.OfertaComparable{
						{ProductoIDOferta: productoLeche, SucursalID: 5, Cadena: "A Cuenta", Nombre: "Leche", Precio: ptrFloat(1000), EnStock: ptrBool(true)},
						{ProductoIDOferta: productoLeche, SucursalID: 6, Cadena: "Cugat", Nombre: "Leche", Precio: ptrFloat(1100), EnStock: ptrBool(true)},
					},
				},
				{
					ProductoID: productoPan,
					Ofertas: []listas.OfertaComparable{
						{ProductoIDOferta: productoPan, SucursalID: 5, Cadena: "A Cuenta", Nombre: "Pan", Precio: ptrFloat(1600), EnStock: ptrBool(true)},
						{ProductoIDOferta: productoPan, SucursalID: 6, Cadena: "Cugat", Nombre: "Pan", Precio: ptrFloat(1500), EnStock: ptrBool(true)},
					},
				},
			},
		},
	}

	clienteHTTP := nuevoClienteRutas(prueba, precios, httpapi.OpcionesOptimizacion{})
	listaID := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioA, []listas.ArticuloLote{
		{ProductoID: productoLeche, Cantidad: 1},
		{ProductoID: productoPan, Cantidad: 1},
	})

	cuerpoPedido := `{"lista_id":"` + listaID + `","origen":{"lat":-38.740,"lon":-72.600}}`
	respuesta := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, cuerpoPedido, nil)
	if respuesta.Code != http.StatusOK {
		prueba.Fatalf("estado = %d, cuerpo = %s", respuesta.Code, respuesta.Body.String())
	}

	var cuerpo struct {
		Origen struct {
			Lat        float64 `json:"lat"`
			Lon        float64 `json:"lon"`
			PorDefecto bool    `json:"por_defecto"`
		} `json:"origen"`
		Ruta struct {
			Paradas []struct {
				SucursalID int `json:"sucursal_id"`
			} `json:"paradas"`
			Total int `json:"total"`
		} `json:"ruta"`
		Ahorro *int `json:"ahorro"`
	}
	if errorJSON := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); errorJSON != nil {
		prueba.Fatal(errorJSON)
	}
	if cuerpo.Origen.PorDefecto || cuerpo.Origen.Lat != -38.740 || cuerpo.Origen.Lon != -72.600 {
		prueba.Fatalf("origen = %+v", cuerpo.Origen)
	}
	if len(cuerpo.Ruta.Paradas) != 1 || cuerpo.Ruta.Total != 2600 {
		prueba.Fatalf("se esperaba 1 parada por umbral de ahorro, obtenido %+v", cuerpo.Ruta)
	}
	if cuerpo.Ahorro == nil || *cuerpo.Ahorro != 0 {
		prueba.Fatalf("ahorro = %v, se esperaba 0", cuerpo.Ahorro)
	}
}

// =============================================================================
// Casos de error (401, 400, 404, 422, 503)
// =============================================================================

func TestOptimizarRutaErroresHTTP(prueba *testing.T) {
	precios := &preciosFalso{respuesta: listas.RespuestaPreciosComparables{Sucursales: sucursalesTemucoPrueba()}}
	clienteHTTP := nuevoClienteRutas(prueba, precios, httpapi.OpcionesOptimizacion{
		RadioKm:         10,
		MaxSucursales:   10,
		MaxParadas:      3,
		MaxArticulos:    2,
		FactorDistancia: 1.3,
		UmbralAhorro:    500,
	})

	listaVacia, _ := clienteHTTP.repo.Crear(prueba.Context(), usuarioA, "Vacía")
	listaAjena := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioB, []listas.ArticuloLote{{ProductoID: productoLeche, Cantidad: 1}})
	listaConUno := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioA, []listas.ArticuloLote{{ProductoID: productoLeche, Cantidad: 1}})
	listaGrande := crearListaConArticulos(prueba, clienteHTTP.repo, usuarioA, []listas.ArticuloLote{
		{ProductoID: productoLeche, Cantidad: 1},
		{ProductoID: productoPan, Cantidad: 1},
		{ProductoID: productoArroz, Cantidad: 1},
	})

	// 401 sin cookie jwt
	clienteSinSesion := nuevoClienteRutas(prueba, precios, httpapi.OpcionesOptimizacion{})
	clienteSinSesion.jwt = ""
	if codigo := clienteSinSesion.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaConUno+`"}`, nil).Code; codigo != http.StatusUnauthorized {
		prueba.Fatalf("sin cookie: estado = %d, se esperaba 401", codigo)
	}

	// 400 cuerpo inválido, campos extra, lista_id inválido y origen fuera de rango
	casos400 := []struct {
		nombre string
		cuerpo string
	}{
		{"JSON roto", `{`},
		{"campo extra", `{"lista_id":"` + listaConUno + `","extra":1}`},
		{"lista_id inválido", `{"lista_id":"no-es-uuid"}`},
		{"origen sin lon", `{"lista_id":"` + listaConUno + `","origen":{"lat":-38.7}}`},
		{"lat fuera de rango", `{"lista_id":"` + listaConUno + `","origen":{"lat":95,"lon":-72.5}}`},
		{"lon fuera de rango", `{"lista_id":"` + listaConUno + `","origen":{"lat":-38.7,"lon":200}}`},
	}
	for _, caso := range casos400 {
		if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, caso.cuerpo, nil).Code; codigo != http.StatusBadRequest {
			prueba.Errorf("%s: estado = %d, se esperaba 400", caso.nombre, codigo)
		}
	}

	// 404 lista de otro usuario (no debe consultar Catálogo)
	consultasAntes := precios.totalConsultas
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaAjena+`"}`, nil).Code; codigo != http.StatusNotFound {
		prueba.Fatalf("lista ajena: estado = %d, se esperaba 404", codigo)
	}
	if precios.totalConsultas != consultasAntes {
		prueba.Fatalf("lista ajena no debió consultar Catálogo")
	}

	// 422 lista vacía
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaVacia.ID+`"}`, nil).Code; codigo != http.StatusUnprocessableEntity {
		prueba.Fatalf("lista vacía: estado = %d, se esperaba 422", codigo)
	}

	// 422 supera MaxArticulos
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaGrande+`"}`, nil).Code; codigo != http.StatusUnprocessableEntity {
		prueba.Fatalf("lista grande: estado = %d, se esperaba 422", codigo)
	}

	// 422 sin ofertas válidas
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaConUno+`"}`, nil).Code; codigo != http.StatusUnprocessableEntity {
		prueba.Fatalf("sin ofertas: estado = %d, se esperaba 422", codigo)
	}

	// 422 sin sucursales cercanas
	precios.respuesta.Sucursales = nil
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaConUno+`"}`, nil).Code; codigo != http.StatusUnprocessableEntity {
		prueba.Fatalf("sin sucursales: estado = %d, se esperaba 422", codigo)
	}

	// 503 Catálogo caído
	precios.caido = true
	if codigo := clienteHTTP.pedir(http.MethodPost, rutaOptimizar, `{"lista_id":"`+listaConUno+`"}`, nil).Code; codigo != http.StatusServiceUnavailable {
		prueba.Fatalf("catálogo caído: estado = %d, se esperaba 503", codigo)
	}
}
