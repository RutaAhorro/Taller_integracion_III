// Pruebas del outbox de ejecuciones y de los ciclos de recuperación.
package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/tests/dobles"
	"github.com/google/uuid"
)

// ===== Outbox =====

// TestEncolarGuardaHuellaYPublicaDespues acepta la ejecución en SQL y la publica en la recuperación.
func TestEncolarGuardaHuellaYPublicaDespues(prueba *testing.T) {
	ejecuciones := &dobles.Ejecuciones{}
	servicio := services.NuevoServicioEjecuciones(ejecuciones, ejecuciones)
	trabajoID := uuid.New()
	if errorObtenido := servicio.Encolar(context.Background(), trabajoID.String(), []byte(`{"spider":"jumbo_rsc"}`)); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if len(ejecuciones.Registradas) != 1 || ejecuciones.Registradas[0].TrabajoID != trabajoID || len(ejecuciones.Registradas[0].PayloadHash) != 64 || len(ejecuciones.Publicadas) != 0 {
		prueba.Fatalf("registradas=%+v publicadas=%+v", ejecuciones.Registradas, ejecuciones.Publicadas)
	}
	servicio.RecuperarEjecuciones(context.Background())
	if len(ejecuciones.Publicadas) != 1 {
		prueba.Fatalf("publicadas=%+v", ejecuciones.Publicadas)
	}
	if errorObtenido := servicio.Encolar(context.Background(), "invalido", nil); !errors.Is(errorObtenido, domain.ErrUUIDInvalido) {
		prueba.Fatal(errorObtenido)
	}
}

// TestRecuperarEjecucionesToleraRedisCaido deja la ejecución en el outbox si Redis falla.
func TestRecuperarEjecucionesToleraRedisCaido(prueba *testing.T) {
	ejecuciones := &dobles.Ejecuciones{ErrorPublicar: errors.New("Redis caído")}
	servicio := services.NuevoServicioEjecuciones(ejecuciones, ejecuciones)
	if errorObtenido := servicio.Encolar(context.Background(), uuid.NewString(), []byte(`{}`)); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	servicio.RecuperarEjecuciones(context.Background())
	if len(ejecuciones.Registradas) != 1 || len(ejecuciones.Publicadas) != 0 {
		prueba.Fatal("la ejecución debe seguir en el outbox")
	}
}

// ===== Ciclos de recuperación =====

// nuevaRecuperacion compone los ciclos con dobles y registra el cierre del publicador.
func nuevaRecuperacion(cerrado *bool) *services.Recuperacion {
	trabajos := dobles.NuevosTrabajos()
	ejecuciones := &dobles.Ejecuciones{}
	return services.NuevaRecuperacion(
		services.NuevoServicioEntregas(dobles.NuevasEntregas(trabajos), dobles.NuevoCatalogo()),
		services.NuevoServicioEjecuciones(ejecuciones, ejecuciones),
		func() error { *cerrado = true; return nil },
	)
}

// TestRecuperacionTerminaAlCancelar detiene ambos ciclos y cierra el publicador.
func TestRecuperacionTerminaAlCancelar(prueba *testing.T) {
	cerrado := false
	recuperacion := nuevaRecuperacion(&cerrado)
	contexto, cancelar := context.WithCancel(prueba.Context())
	recuperacion.Iniciar(contexto)
	cancelar()
	contextoEspera, cancelarEspera := context.WithTimeout(prueba.Context(), time.Second)
	defer cancelarEspera()
	if errorObtenido := recuperacion.Esperar(contextoEspera); errorObtenido != nil || !cerrado {
		prueba.Fatalf("los ciclos no terminaron: %v cerrado=%v", errorObtenido, cerrado)
	}
}

// TestRecuperacionEsperaTienePlazo no bloquea el apagado si los ciclos siguen activos.
func TestRecuperacionEsperaTienePlazo(prueba *testing.T) {
	cerrado := false
	recuperacion := nuevaRecuperacion(&cerrado)
	contexto, cancelar := context.WithCancel(prueba.Context())
	recuperacion.Iniciar(contexto)
	contextoEspera, cancelarEspera := context.WithTimeout(prueba.Context(), 10*time.Millisecond)
	defer cancelarEspera()
	if errorObtenido := recuperacion.Esperar(contextoEspera); !errors.Is(errorObtenido, context.DeadlineExceeded) {
		cancelar()
		prueba.Fatal(errorObtenido)
	}
	cancelar()
	contextoFinal, cancelarFinal := context.WithTimeout(prueba.Context(), time.Second)
	defer cancelarFinal()
	if errorObtenido := recuperacion.Esperar(contextoFinal); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
}
