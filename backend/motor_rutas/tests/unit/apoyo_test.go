package unit_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/httpapi"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// =============================================================================
// Datos fijos de las pruebas
// =============================================================================

const (
	secreto  = "sup240-solo-pruebas-clave-de-al-menos-32-bytes"
	origen   = "https://frontend.example"
	usuarioA = "11111111-1111-4111-8111-111111111111"
	usuarioB = "22222222-2222-4222-8222-222222222222"
	api      = "/api/v1/listas"
)

// =============================================================================
// Tokens JWT de prueba
// =============================================================================

// token firma claims como los emite Identidad. Un valor nil en cambios elimina el claim.
func token(prueba *testing.T, metodo jwt.SigningMethod, clave any, cambios map[string]any) string {
	prueba.Helper()
	ahora := time.Now().Unix()
	claimsToken := jwt.MapClaims{"user_id": usuarioA, "rol": "usuario", "provider": "local", "iat": ahora, "nbf": ahora, "exp": ahora + 3600}
	for clave, valor := range cambios {
		if valor == nil {
			delete(claimsToken, clave)
		} else {
			claimsToken[clave] = valor
		}
	}
	firmado, errorObtenido := jwt.NewWithClaims(metodo, claimsToken).SignedString(clave)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return firmado
}

func tokenDe(prueba *testing.T, usuario string) string {
	return token(prueba, jwt.SigningMethodHS256, []byte(secreto), map[string]any{"user_id": usuario})
}

// =============================================================================
// Repositorio en memoria (reemplaza a PostgreSQL en las pruebas HTTP)
// =============================================================================

// repositorioMemoria implementa listas.Repositorio sólo para las pruebas HTTP.
type repositorioMemoria struct {
	mu          sync.Mutex
	filas       map[string]*fila
	siguienteID int64 // imita el SERIAL de rutas.articulos_lista_compras
}

type fila struct {
	usuario   string
	lista     listas.Lista
	articulos []listas.Articulo
}

func nuevaMemoria() *repositorioMemoria { return &repositorioMemoria{filas: map[string]*fila{}} }

func (repositorio *repositorioMemoria) Contar(_ context.Context, usuarioID string) (int, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	cantidad := 0
	for _, filaLista := range repositorio.filas {
		if filaLista.usuario == usuarioID {
			cantidad++
		}
	}
	return cantidad, nil
}

func (repositorio *repositorioMemoria) Listar(_ context.Context, usuarioID string, limite int) ([]listas.Lista, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	out := []listas.Lista{}
	for _, filaLista := range repositorio.filas {
		if filaLista.usuario == usuarioID {
			out = append(out, filaLista.lista)
		}
	}
	sort.Slice(out, func(izquierda, derecha int) bool {
		return out[izquierda].ActualizadaEl.After(out[derecha].ActualizadaEl)
	})
	if len(out) > limite {
		out = out[:limite]
	}
	return out, nil
}

func (repositorio *repositorioMemoria) Crear(_ context.Context, usuarioID, nombre string) (listas.Lista, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	ahora := time.Now().UTC()
	lista := listas.Lista{ID: uuid.NewString(), Nombre: nombre, Estado: "activa", CreadaEl: ahora, ActualizadaEl: ahora}
	repositorio.filas[lista.ID] = &fila{usuario: usuarioID, lista: lista}
	return lista, nil
}

func (repositorio *repositorioMemoria) Obtener(_ context.Context, usuarioID, listaID string) (*listas.ListaDetalle, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista, existe := repositorio.filas[listaID]
	if !existe || filaLista.usuario != usuarioID {
		return nil, nil
	}
	detalle := &listas.ListaDetalle{Lista: filaLista.lista, Articulos: []listas.ArticuloDetalle{}}
	for _, articulo := range filaLista.articulos {
		detalle.Articulos = append(detalle.Articulos, listas.ArticuloDetalle{Articulo: articulo})
	}
	detalle.TotalArticulos = len(filaLista.articulos)
	return detalle, nil
}

// filaDelUsuario devuelve la lista si es del usuario; nil si no existe o es ajena.
// Se llama con el mutex tomado.
func (repositorio *repositorioMemoria) filaDelUsuario(usuarioID, listaID string) *fila {
	filaLista, existe := repositorio.filas[listaID]
	if !existe || filaLista.usuario != usuarioID {
		return nil
	}
	return filaLista
}

func (repositorio *repositorioMemoria) ListaEsDelUsuario(_ context.Context, usuarioID, listaID string) (bool, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	return repositorio.filaDelUsuario(usuarioID, listaID) != nil, nil
}

func (repositorio *repositorioMemoria) AgregarArticulo(_ context.Context, usuarioID, listaID, productoID string, cantidad int, limites listas.Limites) (*listas.Articulo, bool, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista := repositorio.filaDelUsuario(usuarioID, listaID)
	if filaLista == nil {
		return nil, false, nil
	}
	for indice := range filaLista.articulos {
		articulo := &filaLista.articulos[indice]
		if articulo.ProductoID != nil && *articulo.ProductoID == productoID {
			if articulo.Cantidad+cantidad > limites.MaxCantidad {
				return nil, false, listas.ErrMaxCantidad
			}
			articulo.Cantidad += cantidad
			copia := *articulo
			return &copia, false, nil
		}
	}
	if len(filaLista.articulos) >= limites.MaxArticulos {
		return nil, false, listas.ErrMaxArticulos
	}
	repositorio.siguienteID++
	articulo := listas.Articulo{ID: repositorio.siguienteID, ProductoID: &productoID, Cantidad: cantidad}
	filaLista.articulos = append(filaLista.articulos, articulo)
	return &articulo, true, nil
}

func (repositorio *repositorioMemoria) ActualizarArticulo(_ context.Context, usuarioID, listaID string, articuloID int64, cantidad *int, estaComprado *bool) (*listas.Articulo, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista := repositorio.filaDelUsuario(usuarioID, listaID)
	if filaLista == nil {
		return nil, nil
	}
	for indice := range filaLista.articulos {
		articulo := &filaLista.articulos[indice]
		if articulo.ID != articuloID {
			continue
		}
		if cantidad != nil {
			articulo.Cantidad = *cantidad
		}
		if estaComprado != nil {
			articulo.EstaComprado = *estaComprado
		}
		copia := *articulo
		return &copia, nil
	}
	return nil, nil
}

func (repositorio *repositorioMemoria) EliminarArticulo(_ context.Context, usuarioID, listaID string, articuloID int64) (bool, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista := repositorio.filaDelUsuario(usuarioID, listaID)
	if filaLista == nil {
		return false, nil
	}
	for indice, articulo := range filaLista.articulos {
		if articulo.ID == articuloID {
			filaLista.articulos = append(filaLista.articulos[:indice], filaLista.articulos[indice+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// =============================================================================
// Catálogo falso (reemplaza la consulta HTTP de disponibilidad)
// =============================================================================

// catalogoFalso responde con los estados cargados en la prueba. Un producto
// que no está cargado se informa "no_encontrado", igual que el cliente real.
// caido simula que Catálogo no responde.
type catalogoFalso struct {
	mu        sync.Mutex
	estados   map[string]listas.Disponibilidad
	caido     bool
	consultas int
}

func (catalogo *catalogoFalso) Consultar(_ context.Context, productoIDs []string) (map[string]listas.Disponibilidad, error) {
	catalogo.mu.Lock()
	defer catalogo.mu.Unlock()
	catalogo.consultas++
	if catalogo.caido {
		return nil, listas.ErrCatalogoNoDisponible
	}
	resultado := map[string]listas.Disponibilidad{}
	for _, productoID := range productoIDs {
		disponibilidad, existe := catalogo.estados[productoID]
		if !existe {
			disponibilidad = listas.Disponibilidad{Estado: listas.EstadoNoEncontrado}
		}
		resultado[productoID] = disponibilidad
	}
	return resultado, nil
}

// producto carga un producto en el catálogo falso con el estado indicado.
func (catalogo *catalogoFalso) producto(productoID, estado string) {
	catalogo.mu.Lock()
	defer catalogo.mu.Unlock()
	catalogo.estados[productoID] = listas.Disponibilidad{Estado: estado,
		Producto: &listas.ProductoCatalogo{Nombre: "Leche entera 1 L", Supermercado: "Lider"}}
}

func (repositorio *repositorioMemoria) Actualizar(_ context.Context, usuarioID, listaID string, nombre, estado *string) (*listas.Lista, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista, existe := repositorio.filas[listaID]
	if !existe || filaLista.usuario != usuarioID {
		return nil, nil
	}
	if nombre != nil {
		filaLista.lista.Nombre = *nombre
	}
	if estado != nil {
		filaLista.lista.Estado = *estado
	}
	filaLista.lista.ActualizadaEl = time.Now().UTC()
	lista := filaLista.lista
	return &lista, nil
}

func (repositorio *repositorioMemoria) Eliminar(_ context.Context, usuarioID, listaID string) (bool, error) {
	repositorio.mu.Lock()
	defer repositorio.mu.Unlock()
	filaLista, existe := repositorio.filas[listaID]
	if !existe || filaLista.usuario != usuarioID {
		return false, nil
	}
	delete(repositorio.filas, listaID)
	return true, nil
}

// =============================================================================
// Cliente HTTP de prueba
// =============================================================================

// cliente llama al handler sin abrir puertos, enviando la cookie jwt indicada.
type cliente struct {
	prueba    *testing.T
	manejador http.Handler
	jwt       string
	repo      *repositorioMemoria
	catalogo  *catalogoFalso
	extra     http.Header
}

// Topes chicos para poder probar los límites con pocas peticiones.
const (
	maxArticulosPrueba = 3
	maxCantidadPrueba  = 10
)

func nuevoCliente(prueba *testing.T) *cliente {
	repo := nuevaMemoria()
	catalogo := &catalogoFalso{estados: map[string]listas.Disponibilidad{}}
	manejador := httpapi.Nuevo(httpapi.Opciones{Repo: repo, Catalogo: catalogo, JWTSecret: secreto,
		OrigenesCORS: []string{origen}, MaxListas: 100,
		Limites: listas.Limites{MaxArticulos: maxArticulosPrueba, MaxCantidad: maxCantidadPrueba},
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil))})
	return &cliente{prueba: prueba, manejador: manejador, repo: repo, catalogo: catalogo, jwt: tokenDe(prueba, usuarioA)}
}

func (clienteHTTP *cliente) pedir(metodo, ruta, cuerpo string, cabeceras map[string]string) *httptest.ResponseRecorder {
	var peticion *http.Request
	if cuerpo == "" {
		peticion = httptest.NewRequest(metodo, ruta, nil)
	} else {
		peticion = httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
		peticion.Header.Set("Content-Type", "application/json")
	}
	if clienteHTTP.jwt != "" {
		peticion.AddCookie(&http.Cookie{Name: "jwt", Value: clienteHTTP.jwt})
	}
	for clave, valor := range cabeceras {
		peticion.Header.Set(clave, valor)
	}
	respuesta := httptest.NewRecorder()
	clienteHTTP.manejador.ServeHTTP(respuesta, peticion)
	return respuesta
}
