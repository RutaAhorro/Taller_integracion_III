package repositories

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
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/google/uuid"
)

func TestCatalogoClientContract(t *testing.T) {
	token := strings.Repeat("t", 32)
	id := uuid.New()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-Request-ID") != "sup269-test" {
			t.Error("contrato o credenciales internas incorrectos")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), id.String()) || r.Header.Get("Cookie") != "" {
			t.Error("se envió un trabajo o cookie de usuario a Catálogo")
		}
		switch r.URL.Path {
		case "/internal/v1/cadenas/resolver":
			io.WriteString(w, `{"id":1,"nombre":"Jumbo"}`)
		case "/internal/v1/sucursales/resolver":
			io.WriteString(w, `{"id":2,"cadena_id":1}`)
		case "/internal/v1/ingestas":
			io.WriteString(w, `{"sucursal_id":2,"total_recibidos":1,"insertados":1,"actualizados":0,"precios_registrados":1}`)
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	repo, err := NewCatalogoClient(srv.URL, token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithRequestID(context.Background(), "sup269-test")
	chain, err := repo.ObtenerCadena(ctx, 1, "")
	if err != nil || chain.ID != 1 {
		t.Fatalf("cadena: %v", err)
	}
	branch, err := repo.ObtenerSucursal(ctx, 1, nil, nil)
	if err != nil || branch.ID != 2 {
		t.Fatalf("sucursal: %v", err)
	}
	batch, err := repo.IngestarLote(ctx, &id, 2, []domain.ProductoScrapeadoDTO{{SKU: "SKU", Producto: "Producto", PrecioNormal: 1000}})
	if err != nil || batch.TrabajoID == nil || *batch.TrabajoID != id.String() || calls.Load() != 3 {
		t.Fatalf("lote: %+v %v", batch, err)
	}
}

func TestCatalogoClientFailuresAndNoRetry(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/retry")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			repo, err := NewCatalogoClient(srv.URL, strings.Repeat("t", 32), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.IngestarLote(context.Background(), nil, 1, []domain.ProductoScrapeadoDTO{{SKU: "SKU"}})
			if !errors.Is(err, services.ErrCatalogoNoDisponible) || calls.Load() != 1 {
				t.Fatalf("se reintentó o filtró error: %v llamadas=%d", err, calls.Load())
			}
		})
	}
}

func TestCatalogoClientTimeoutAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer srv.Close()
	repo, err := NewCatalogoClient(srv.URL, strings.Repeat("t", 32), 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ObtenerCadena(context.Background(), 1, ""); !errors.Is(err, services.ErrCatalogoTimeout) {
		t.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = repo.ObtenerCadena(ctx, 1, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelación: %v", err)
	}
}

func TestCatalogoClientInvalidConfiguration(t *testing.T) {
	for _, origin := range []string{"", "ftp://catalogo", "http://user:secret@catalogo", "http://catalogo/path", "http://catalogo?token=secret", "http://catalogo#fragment"} {
		if _, err := NewCatalogoClient(origin, strings.Repeat("t", 32), time.Second); err == nil {
			t.Errorf("aceptó %q", origin)
		}
	}
	if _, err := NewCatalogoClient("http://catalogo", "short", time.Second); err == nil {
		t.Error("aceptó secreto débil")
	}
	if _, err := NewCatalogoClient("http://catalogo", strings.Repeat("t", 32), 30*time.Second); err == nil {
		t.Error("aceptó timeout mayor al límite")
	}
}
