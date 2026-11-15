package control_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/control"
)

func TestCanceledRecoveryLoopsStop(t *testing.T) {
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 32))
	modulo, err := control.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	contexto, cancelar := context.WithCancel(t.Context())
	modulo.Start(contexto)
	cancelar()
	contextoEspera, detenerEspera := context.WithTimeout(t.Context(), time.Second)
	defer detenerEspera()
	if err := modulo.Wait(contextoEspera); err != nil {
		t.Fatalf("los loops no terminaron: %v", err)
	}
}

func TestRecoveryShutdownHasDeadline(t *testing.T) {
	t.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	t.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 32))
	modulo, err := control.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	contexto, cancelar := context.WithCancel(t.Context())
	modulo.Start(contexto)
	contextoEspera, detenerEspera := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer detenerEspera()
	if err := modulo.Wait(contextoEspera); !errors.Is(err, context.DeadlineExceeded) {
		cancelar()
		t.Fatal(err)
	}
	cancelar()
	contextoFinal, cancelarFinal := context.WithTimeout(t.Context(), time.Second)
	defer cancelarFinal()
	if err := modulo.Wait(contextoFinal); err != nil {
		t.Fatal(err)
	}
}
