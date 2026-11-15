package control_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/control"
)

func TestCanceledRecoveryLoopsStop(prueba *testing.T) {
	prueba.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("prueba", 32))
	prueba.Setenv("SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 32))
	modulo, err := control.New(nil, nil)
	if err != nil {
		prueba.Fatal(err)
	}
	contexto, cancelar := context.WithCancel(prueba.Context())
	modulo.Start(contexto)
	cancelar()
	contextoEspera, detenerEspera := context.WithTimeout(prueba.Context(), time.Second)
	defer detenerEspera()
	if err := modulo.Wait(contextoEspera); err != nil {
		prueba.Fatalf("los loops no terminaron: %v", err)
	}
}

func TestRecoveryShutdownHasDeadline(prueba *testing.T) {
	prueba.Setenv("CATALOGO_INTERNAL_URL", "http://catalogo:8080")
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("prueba", 32))
	prueba.Setenv("SCRAPING_SERVICE_TOKEN", strings.Repeat("s", 32))
	modulo, err := control.New(nil, nil)
	if err != nil {
		prueba.Fatal(err)
	}
	contexto, cancelar := context.WithCancel(prueba.Context())
	modulo.Start(contexto)
	contextoEspera, detenerEspera := context.WithTimeout(prueba.Context(), 10*time.Millisecond)
	defer detenerEspera()
	if err := modulo.Wait(contextoEspera); !errors.Is(err, context.DeadlineExceeded) {
		cancelar()
		prueba.Fatal(err)
	}
	cancelar()
	contextoFinal, cancelarFinal := context.WithTimeout(prueba.Context(), time.Second)
	defer cancelarFinal()
	if err := modulo.Wait(contextoFinal); err != nil {
		prueba.Fatal(err)
	}
}
