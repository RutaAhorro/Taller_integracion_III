// Pruebas del cliente HTTP interno de Catálogo contra un servidor local: rutas,
// cabeceras, traducción de errores, ausencia de reintentos y plazos.
package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/google/uuid"
)

// tokenPrueba es un token interno válido sólo para estas pruebas.
var tokenPrueba = strings.Repeat("t", 32)

// productosPrueba es un lote mínimo de un producto.
var productosPrueba = []domain.ProductoScrapeadoDTO{{SKU: "SKU", Producto: "Producto", PrecioNormal: 1000}}

// ===== Contrato HTTP =====

// TestClienteCatalogoContrato verifica rutas, método, cabeceras y respuestas válidas.
func TestClienteCatalogoContrato(prueba *testing.T) {
	trabajoID := uuid.New()
	var llamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		llamadas.Add(1)
		if peticion.Method != http.MethodPost || peticion.Header.Get("Authorization") != "Bearer "+tokenPrueba || peticion.Header.Get("X-Request-ID") != "peticion-prueba" {
			prueba.Error("contrato o credenciales internas incorrectos")
		}
		cuerpo, _ := io.ReadAll(peticion.Body)
		if strings.Contains(string(cuerpo), trabajoID.String()) || peticion.Header.Get("Cookie") != "" {
			prueba.Error("se envió el trabajo o una cookie de usuario a Catálogo")
		}
		switch peticion.URL.Path {
		case "/internal/v1/cadenas/resolver":
			_, _ = io.WriteString(respuesta, `{"id":1,"nombre":"Jumbo","esta_activa":false}`)
		case "/internal/v1/sucursales/resolver":
			_, _ = io.WriteString(respuesta, `{"id":2,"cadena_id":1}`)
		case "/internal/v1/ingestas":
			_, _ = io.WriteString(respuesta, `{"sucursal_id":2,"total_recibidos":1,"insertados":1,"actualizados":0,"precios_registrados":1}`)
		default:
			prueba.Error(peticion.URL.Path)
			respuesta.WriteHeader(http.StatusNotFound)
		}
	}))
	defer servidor.Close()
	cliente := repositories.NuevoClienteCatalogo(servidor.URL, tokenPrueba, time.Second)
	contexto := domain.ConIDPeticion(context.Background(), "peticion-prueba")
	cadena, errorObtenido := cliente.ObtenerCadena(contexto, 1, "")
	if errorObtenido != nil || cadena.ID != 1 || cadena.EstaActiva {
		prueba.Fatalf("cadena=%+v error=%v", cadena, errorObtenido)
	}
	sucursal, errorObtenido := cliente.ObtenerSucursal(contexto, 1, nil, nil)
	if errorObtenido != nil || sucursal.ID != 2 {
		prueba.Fatalf("sucursal=%+v error=%v", sucursal, errorObtenido)
	}
	lote, errorObtenido := cliente.IngestarLote(contexto, &trabajoID, 2, productosPrueba)
	if errorObtenido != nil || lote.TrabajoID == nil || *lote.TrabajoID != trabajoID.String() || llamadas.Load() != 3 {
		prueba.Fatalf("lote=%+v error=%v", lote, errorObtenido)
	}
}

// TestClienteCatalogoAusenteEsNil distingue un 404 de un fallo.
func TestClienteCatalogoAusenteEsNil(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
		respuesta.WriteHeader(http.StatusNotFound)
	}))
	defer servidor.Close()
	cadena, errorObtenido := repositories.NuevoClienteCatalogo(servidor.URL, tokenPrueba, time.Second).ObtenerCadena(context.Background(), 9, "")
	if cadena != nil || errorObtenido != nil {
		prueba.Fatalf("cadena=%+v error=%v", cadena, errorObtenido)
	}
}

// TestClienteCatalogoEnviaClaveDelLote envía lote_id sin trabajo_id y no reintenta un 409.
func TestClienteCatalogoEnviaClaveDelLote(prueba *testing.T) {
	clave := uuid.New().String()
	var llamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		llamadas.Add(1)
		var carga map[string]any
		if errorObtenido := json.NewDecoder(peticion.Body).Decode(&carga); errorObtenido != nil {
			prueba.Error(errorObtenido)
		}
		if carga["lote_id"] != clave || carga["trabajo_id"] != nil || peticion.Header.Get("X-Request-ID") != clave {
			prueba.Errorf("contrato interno=%v cabecera=%q", carga, peticion.Header.Get("X-Request-ID"))
		}
		respuesta.WriteHeader(http.StatusConflict)
	}))
	defer servidor.Close()
	trabajoID := uuid.New()
	cliente := repositories.NuevoClienteCatalogo(servidor.URL, tokenPrueba, time.Second)
	_, errorObtenido := cliente.IngestarLote(domain.ConLoteID(context.Background(), clave), &trabajoID, 1, productosPrueba)
	if !errors.Is(errorObtenido, domain.ErrIdempotencia) || llamadas.Load() != 1 {
		prueba.Fatal(errorObtenido, llamadas.Load())
	}
}

// ===== Fallos y plazos =====

// TestClienteCatalogoFallosSinReintento comprueba que los errores no filtren datos ni reintenten.
func TestClienteCatalogoFallosSinReintento(prueba *testing.T) {
	casos := []struct {
		nombre   string
		estado   int
		cuerpo   string
		esperado error
	}{
		{"SQL no expuesto", http.StatusInternalServerError, `secreto tabla postgres`, domain.ErrCatalogoNoDisponible},
		{"token inválido", http.StatusUnauthorized, `secreto`, domain.ErrCatalogoNoDisponible},
		{"redirección no seguida", http.StatusTemporaryRedirect, `secreto`, domain.ErrCatalogoNoDisponible},
		{"no disponible", http.StatusServiceUnavailable, ``, domain.ErrCatalogoNoDisponible},
		{"JSON inválido", http.StatusOK, `invalido`, domain.ErrCatalogoNoDisponible},
		{"respuesta demasiado grande", http.StatusOK, strings.Repeat("x", (1<<20)+1), domain.ErrCatalogoNoDisponible},
		{"resultado inconsistente", http.StatusOK, `{"sucursal_id":1,"total_recibidos":1,"insertados":1,"precios_registrados":0}`, domain.ErrCatalogoNoDisponible},
		{"lote no encontrado", http.StatusNotFound, `{}`, domain.ErrCatalogoNoDisponible},
		{"lote rechazado", http.StatusBadRequest, `{}`, domain.ErrCatalogoRechazo},
		{"lote demasiado grande", http.StatusRequestEntityTooLarge, `{}`, domain.ErrCatalogoRechazo},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			var llamadas atomic.Int32
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, _ *http.Request) {
				llamadas.Add(1)
				respuesta.Header().Set("Location", "/reintentar")
				respuesta.WriteHeader(caso.estado)
				_, _ = io.WriteString(respuesta, caso.cuerpo)
			}))
			defer servidor.Close()
			cliente := repositories.NuevoClienteCatalogo(servidor.URL, tokenPrueba, time.Second)
			_, errorObtenido := cliente.IngestarLote(context.Background(), nil, 1, productosPrueba)
			if !errors.Is(errorObtenido, caso.esperado) || llamadas.Load() != 1 {
				prueba.Fatalf("error=%v llamadas=%d", errorObtenido, llamadas.Load())
			}
		})
	}
}

// TestClienteCatalogoPlazoYCancelacion distingue el timeout de la cancelación.
func TestClienteCatalogoPlazoYCancelacion(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, peticion *http.Request) {
		_, _ = io.Copy(io.Discard, peticion.Body)
		<-peticion.Context().Done()
	}))
	defer servidor.Close()
	cliente := repositories.NuevoClienteCatalogo(servidor.URL, tokenPrueba, 25*time.Millisecond)
	if _, errorObtenido := cliente.ObtenerCadena(context.Background(), 1, ""); !errors.Is(errorObtenido, domain.ErrCatalogoTimeout) {
		prueba.Fatalf("timeout: %v", errorObtenido)
	}
	contexto, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, errorObtenido := cliente.ObtenerCadena(contexto, 1, ""); !errors.Is(errorObtenido, context.Canceled) {
		prueba.Fatalf("cancelación: %v", errorObtenido)
	}
}
