package repositories_test

import (
	"context"
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
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
)

// ===== Contratos HTTP con Catálogo =====

// TestCatalogoClientContract verifica rutas, headers y respuestas válidas del cliente.
func TestCatalogoClientContract(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	identificador := uuid.New()
	var llamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		llamadas.Add(1)
		if peticion.Method != "POST" || peticion.Header.Get("Authorization") != "Bearer "+token || peticion.Header.Get("X-Request-ID") != "sup269-test" {
			prueba.Error("contrato o credenciales internas incorrectos")
		}
		cuerpo, _ := io.ReadAll(peticion.Body)
		if strings.Contains(string(cuerpo), identificador.String()) || peticion.Header.Get("Cookie") != "" {
			prueba.Error("se envió un trabajo o cookie de usuario a Catálogo")
		}
		switch peticion.URL.Path {
		case "/internal/v1/cadenas/resolver":
			io.WriteString(respuestaHTTP, `{"id":1,"nombre":"Jumbo"}`)
		case "/internal/v1/sucursales/resolver":
			io.WriteString(respuestaHTTP, `{"id":2,"cadena_id":1}`)
		case "/internal/v1/ingestas":
			io.WriteString(respuestaHTTP, `{"sucursal_id":2,"total_recibidos":1,"insertados":1,"actualizados":0,"precios_registrados":1}`)
		default:
			prueba.Error(peticion.URL.Path)
			respuestaHTTP.WriteHeader(404)
		}
	}))
	defer servidor.Close()
	repositorio, errorObtenido := repositories.NewCatalogoClient(servidor.URL, token, time.Second)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	contexto := repositories.WithRequestID(context.Background(), "sup269-test")
	cadena, errorObtenido := repositorio.ObtenerCadena(contexto, 1, "")
	if errorObtenido != nil || cadena.ID != 1 {
		prueba.Fatalf("cadena: %v", errorObtenido)
	}
	sucursal, errorObtenido := repositorio.ObtenerSucursal(contexto, 1, nil, nil)
	if errorObtenido != nil || sucursal.ID != 2 {
		prueba.Fatalf("sucursal: %v", errorObtenido)
	}
	lote, errorObtenido := repositorio.IngestarLote(contexto, &identificador, 2, []domain.ProductoScrapeadoDTO{{SKU: "SKU", Producto: "Producto", PrecioNormal: 1000}})
	if errorObtenido != nil || lote.TrabajoID == nil || *lote.TrabajoID != identificador.String() || llamadas.Load() != 3 {
		prueba.Fatalf("lote: %+v %v", lote, errorObtenido)
	}
}

// TestCatalogoClientObtenerCadenaPropagaEstado conserva el estado de activación recibido.
func TestCatalogoClientObtenerCadenaPropagaEstado(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path != "/internal/v1/cadenas/resolver" {
			prueba.Errorf("ruta inesperada: %s", peticion.URL.Path)
			respuestaHTTP.WriteHeader(404)
			return
		}
		io.WriteString(respuestaHTTP, `{"id":1,"nombre":"Jumbo","esta_activa":false}`)
	}))
	defer servidor.Close()

	repositorio, errorObtenido := repositories.NewCatalogoClient(servidor.URL, token, time.Second)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	cadena, errorObtenido := repositorio.ObtenerCadena(context.Background(), 1, "")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if cadena == nil || cadena.EstaActiva {
		prueba.Fatalf("el flag HTTP no se propagó: %+v", cadena)
	}
}

// ===== Fallos, timeouts y validación de configuración =====

// TestCatalogoClientFailuresAndNoRetry comprueba que los errores no filtren datos ni reintenten.
func TestCatalogoClientFailuresAndNoRetry(prueba *testing.T) {
	for _, casoPrueba := range []struct {
		nombre       string
		codigoEstado int
		cuerpo       string
	}{
		{"SQL no expuesto", 500, `secret postgres table`}, {"token inválido", 401, `secret`},
		{"redirect no seguido", 307, `secret`}, {"JSON inválido", 200, `invalid`},
		{"respuesta demasiado grande", 200, strings.Repeat("x", (1<<20)+1)},
		{"resultado inconsistente", 200, `{"sucursal_id":1,"total_recibidos":1,"insertados":1,"precios_registrados":0}`},
		{"lote no encontrado", 404, `{}`},
	} {
		prueba.Run(casoPrueba.nombre, func(prueba *testing.T) {
			var llamadas atomic.Int32
			servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
				llamadas.Add(1)
				respuestaHTTP.Header().Set("Location", "/retry")
				respuestaHTTP.WriteHeader(casoPrueba.codigoEstado)
				io.WriteString(respuestaHTTP, casoPrueba.cuerpo)
			}))
			defer servidor.Close()
			repositorio, errorObtenido := repositories.NewCatalogoClient(servidor.URL, strings.Repeat("t", 32), time.Second)
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
			_, errorObtenido = repositorio.IngestarLote(context.Background(), nil, 1, []domain.ProductoScrapeadoDTO{{SKU: "SKU"}})
			if !errors.Is(errorObtenido, services.ErrCatalogoNoDisponible) || llamadas.Load() != 1 {
				prueba.Fatalf("se reintentó o filtró error: %v llamadas=%d", errorObtenido, llamadas.Load())
			}
		})
	}
}

// TestCatalogoClientTimeoutAndCancellation distingue timeout y cancelación del contexto.
func TestCatalogoClientTimeoutAndCancellation(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		io.Copy(io.Discard, peticion.Body)
		<-peticion.Context().Done()
	}))
	defer servidor.Close()
	repositorio, errorObtenido := repositories.NewCatalogoClient(servidor.URL, strings.Repeat("t", 32), 25*time.Millisecond)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, errorObtenido = repositorio.ObtenerCadena(context.Background(), 1, ""); !errors.Is(errorObtenido, services.ErrCatalogoTimeout) {
		prueba.Fatalf("timeout: %v", errorObtenido)
	}
	contexto, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, errorObtenido = repositorio.ObtenerCadena(contexto, 1, ""); !errors.Is(errorObtenido, context.Canceled) {
		prueba.Fatalf("cancelación: %v", errorObtenido)
	}
}

// TestCatalogoClientInvalidConfiguration rechaza orígenes, tokens y timeouts inválidos.
func TestCatalogoClientInvalidConfiguration(prueba *testing.T) {
	for _, origen := range []string{"", "ftp://catalogo", "http://user:secret@catalogo", "http://catalogo/path", "http://catalogo?token=secret", "http://catalogo#fragment"} {
		if _, errorObtenido := repositories.NewCatalogoClient(origen, strings.Repeat("t", 32), time.Second); errorObtenido == nil {
			prueba.Errorf("aceptó %q", origen)
		}
	}
	if _, errorObtenido := repositories.NewCatalogoClient("http://catalogo", "short", time.Second); errorObtenido == nil {
		prueba.Error("aceptó secreto débil")
	}
	if _, errorObtenido := repositories.NewCatalogoClient("http://catalogo", strings.Repeat("t", 32), 30*time.Second); errorObtenido == nil {
		prueba.Error("aceptó timeout mayor al límite")
	}
}
