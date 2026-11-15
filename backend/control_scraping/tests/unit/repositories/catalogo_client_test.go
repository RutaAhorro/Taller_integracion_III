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

func TestCatalogoClientContract(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	id := uuid.New()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		calls.Add(1)
		if peticion.Method != "POST" || peticion.Header.Get("Authorization") != "Bearer "+token || peticion.Header.Get("X-Request-ID") != "sup269-test" {
			prueba.Error("contrato o credenciales internas incorrectos")
		}
		body, _ := io.ReadAll(peticion.Body)
		if strings.Contains(string(body), id.String()) || peticion.Header.Get("Cookie") != "" {
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
	defer srv.Close()
	repo, err := repositories.NewCatalogoClient(srv.URL, token, time.Second)
	if err != nil {
		prueba.Fatal(err)
	}
	contexto := repositories.WithRequestID(context.Background(), "sup269-test")
	cadena, err := repo.ObtenerCadena(contexto, 1, "")
	if err != nil || cadena.ID != 1 {
		prueba.Fatalf("cadena: %v", err)
	}
	sucursal, err := repo.ObtenerSucursal(contexto, 1, nil, nil)
	if err != nil || sucursal.ID != 2 {
		prueba.Fatalf("sucursal: %v", err)
	}
	lote, err := repo.IngestarLote(contexto, &id, 2, []domain.ProductoScrapeadoDTO{{SKU: "SKU", Producto: "Producto", PrecioNormal: 1000}})
	if err != nil || lote.TrabajoID == nil || *lote.TrabajoID != id.String() || calls.Load() != 3 {
		prueba.Fatalf("lote: %+v %v", lote, err)
	}
}

func TestCatalogoClientObtenerCadenaPropagaEstado(prueba *testing.T) {
	token := strings.Repeat("t", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path != "/internal/v1/cadenas/resolver" {
			prueba.Errorf("ruta inesperada: %s", peticion.URL.Path)
			respuestaHTTP.WriteHeader(404)
			return
		}
		io.WriteString(respuestaHTTP, `{"id":1,"nombre":"Jumbo","esta_activa":false}`)
	}))
	defer srv.Close()

	repo, err := repositories.NewCatalogoClient(srv.URL, token, time.Second)
	if err != nil {
		prueba.Fatal(err)
	}
	cadena, err := repo.ObtenerCadena(context.Background(), 1, "")
	if err != nil {
		prueba.Fatal(err)
	}
	if cadena == nil || cadena.EstaActiva {
		prueba.Fatalf("el flag HTTP no se propagó: %+v", cadena)
	}
}

func TestCatalogoClientFailuresAndNoRetry(prueba *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"SQL no expuesto", 500, `secret postgres table`}, {"token inválido", 401, `secret`},
		{"redirect no seguido", 307, `secret`}, {"JSON inválido", 200, `invalid`},
		{"respuesta demasiado grande", 200, strings.Repeat("x", (1<<20)+1)},
		{"resultado inconsistente", 200, `{"sucursal_id":1,"total_recibidos":1,"insertados":1,"precios_registrados":0}`},
		{"lote no encontrado", 404, `{}`},
	} {
		prueba.Run(tc.name, func(prueba *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
				calls.Add(1)
				respuestaHTTP.Header().Set("Location", "/retry")
				respuestaHTTP.WriteHeader(tc.status)
				io.WriteString(respuestaHTTP, tc.body)
			}))
			defer srv.Close()
			repo, err := repositories.NewCatalogoClient(srv.URL, strings.Repeat("t", 32), time.Second)
			if err != nil {
				prueba.Fatal(err)
			}
			_, err = repo.IngestarLote(context.Background(), nil, 1, []domain.ProductoScrapeadoDTO{{SKU: "SKU"}})
			if !errors.Is(err, services.ErrCatalogoNoDisponible) || calls.Load() != 1 {
				prueba.Fatalf("se reintentó o filtró error: %v llamadas=%d", err, calls.Load())
			}
		})
	}
}

func TestCatalogoClientTimeoutAndCancellation(prueba *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(respuestaHTTP http.ResponseWriter, peticion *http.Request) {
		io.Copy(io.Discard, peticion.Body)
		<-peticion.Context().Done()
	}))
	defer srv.Close()
	repo, err := repositories.NewCatalogoClient(srv.URL, strings.Repeat("t", 32), 25*time.Millisecond)
	if err != nil {
		prueba.Fatal(err)
	}
	if _, err = repo.ObtenerCadena(context.Background(), 1, ""); !errors.Is(err, services.ErrCatalogoTimeout) {
		prueba.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = repo.ObtenerCadena(ctx, 1, ""); !errors.Is(err, context.Canceled) {
		prueba.Fatalf("cancelación: %v", err)
	}
}

func TestCatalogoClientInvalidConfiguration(prueba *testing.T) {
	for _, origin := range []string{"", "ftp://catalogo", "http://user:secret@catalogo", "http://catalogo/path", "http://catalogo?token=secret", "http://catalogo#fragment"} {
		if _, err := repositories.NewCatalogoClient(origin, strings.Repeat("t", 32), time.Second); err == nil {
			prueba.Errorf("aceptó %q", origin)
		}
	}
	if _, err := repositories.NewCatalogoClient("http://catalogo", "short", time.Second); err == nil {
		prueba.Error("aceptó secreto débil")
	}
	if _, err := repositories.NewCatalogoClient("http://catalogo", strings.Repeat("t", 32), 30*time.Second); err == nil {
		prueba.Error("aceptó timeout mayor al límite")
	}
}
