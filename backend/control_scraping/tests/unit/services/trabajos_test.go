// Pruebas del ciclo de vida de trabajos con dobles en memoria.
package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/tests/dobles"
	"github.com/google/uuid"
)

// ===== Inicio =====

// TestIniciarTrabajoGuardaInstantanea crea el trabajo en progreso con el nombre de la cadena.
func TestIniciarTrabajoGuardaInstantanea(prueba *testing.T) {
	trabajos, catalogo := dobles.NuevosTrabajos(), dobles.NuevoCatalogo()
	catalogo.AgregarCadena(1, "Jumbo", true)
	usuario := "no-es-uuid"
	trabajo, errorObtenido := services.NuevoServicioTrabajos(trabajos, catalogo).IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{CadenaID: 1, DisparadoPorUsuarioID: &usuario})
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	guardado := trabajos.Guardados[uuid.MustParse(trabajo.ID)]
	if trabajo.Estado != domain.EstadoEnProgreso || trabajo.CadenaNombre != "Jumbo" || guardado.CadenaNombre != "Jumbo" || guardado.DisparadoPorUsuarioID != nil {
		prueba.Fatalf("trabajo=%+v guardado=%+v", trabajo, guardado)
	}
}

// TestIniciarTrabajoValidaFuente rechaza cadenas inexistentes o deshabilitadas.
func TestIniciarTrabajoValidaFuente(prueba *testing.T) {
	catalogo := dobles.NuevoCatalogo()
	catalogo.AgregarCadena(1, "Jumbo", false)
	servicio := services.NuevoServicioTrabajos(dobles.NuevosTrabajos(), catalogo)
	if _, errorObtenido := servicio.IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{CadenaID: 1}); !errors.Is(errorObtenido, domain.ErrFuenteDeshabilitada) {
		prueba.Fatalf("deshabilitada: %v", errorObtenido)
	}
	if _, errorObtenido := servicio.IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{CadenaID: 99}); !errors.Is(errorObtenido, domain.ErrCadenaNoEncontrada) {
		prueba.Fatalf("inexistente: %v", errorObtenido)
	}
	catalogo.Error = domain.ErrCatalogoTimeout
	if _, errorObtenido := servicio.IniciarTrabajo(context.Background(), domain.IniciarTrabajoDTO{CadenaID: 1}); !errors.Is(errorObtenido, domain.ErrCatalogoTimeout) {
		prueba.Fatalf("catálogo caído: %v", errorObtenido)
	}
}

// ===== Cierre =====

// TestFinalizarTrabajo cubre estado inválido, cierre, repetición idempotente y conflicto.
func TestFinalizarTrabajo(prueba *testing.T) {
	trabajos := dobles.NuevosTrabajos()
	identificador := trabajos.Agregar(1, "Jumbo").String()
	servicio := services.NuevoServicioTrabajos(trabajos, dobles.NuevoCatalogo())
	contexto := context.Background()
	if _, errorObtenido := servicio.FinalizarTrabajo(contexto, identificador, domain.FinalizarTrabajoDTO{Estado: "pausado"}); !errors.Is(errorObtenido, domain.ErrEstadoTrabajoInvalido) {
		prueba.Fatalf("estado inválido: %v", errorObtenido)
	}
	elementos, bloqueadas := 150, 2
	cerrado, errorObtenido := servicio.FinalizarTrabajo(contexto, identificador, domain.FinalizarTrabajoDTO{Estado: " Completado ", ElementosExtraidos: &elementos, RespuestasBloqueadas: &bloqueadas})
	if errorObtenido != nil || cerrado.Estado != domain.EstadoCompletado || cerrado.ElementosExtraidos != elementos || cerrado.RespuestasBloqueadas != bloqueadas || cerrado.DuracionSegundos == nil || *cerrado.DuracionSegundos <= 0 {
		prueba.Fatalf("cierre=%+v error=%v", cerrado, errorObtenido)
	}
	repetido, errorObtenido := servicio.FinalizarTrabajo(contexto, identificador, domain.FinalizarTrabajoDTO{Estado: domain.EstadoCompletado})
	if errorObtenido != nil || repetido.ElementosExtraidos != elementos {
		prueba.Fatalf("el mismo cierre debe ser idempotente: %+v %v", repetido, errorObtenido)
	}
	if _, errorObtenido := servicio.FinalizarTrabajo(contexto, identificador, domain.FinalizarTrabajoDTO{Estado: domain.EstadoFallido}); !errors.Is(errorObtenido, domain.ErrTrabajoYaFinalizado) {
		prueba.Fatalf("otro cierre debe ser conflicto: %v", errorObtenido)
	}
	if _, errorObtenido := servicio.FinalizarTrabajo(contexto, "invalido", domain.FinalizarTrabajoDTO{Estado: domain.EstadoFallido}); !errors.Is(errorObtenido, domain.ErrUUIDInvalido) {
		prueba.Fatalf("UUID inválido: %v", errorObtenido)
	}
}

// ===== Consulta =====

// TestObtenerTrabajoResuelveNombreHeredado consulta Catálogo sólo si falta la instantánea.
func TestObtenerTrabajoResuelveNombreHeredado(prueba *testing.T) {
	trabajos, catalogo := dobles.NuevosTrabajos(), dobles.NuevoCatalogo()
	catalogo.AgregarCadena(3, "Lider", true)
	heredado := trabajos.Agregar(3, "").String()
	servicio := services.NuevoServicioTrabajos(trabajos, catalogo)
	trabajo, errorObtenido := servicio.ObtenerTrabajo(context.Background(), heredado)
	if errorObtenido != nil || trabajo.CadenaNombre != "Lider" {
		prueba.Fatalf("trabajo=%+v error=%v", trabajo, errorObtenido)
	}
	// Con Catálogo caído el trabajo se responde igual, sin nombre.
	catalogo.Error = domain.ErrCatalogoNoDisponible
	trabajo, errorObtenido = servicio.ObtenerTrabajo(context.Background(), heredado)
	if errorObtenido != nil || trabajo.CadenaNombre != "" {
		prueba.Fatalf("trabajo=%+v error=%v", trabajo, errorObtenido)
	}
	if _, errorObtenido := servicio.ObtenerTrabajo(context.Background(), uuid.NewString()); !errors.Is(errorObtenido, domain.ErrTrabajoNoEncontrado) {
		prueba.Fatalf("inexistente: %v", errorObtenido)
	}
}

// TestObtenerCadenaTraduceAusencia convierte el nil de Catálogo en error de dominio.
func TestObtenerCadenaTraduceAusencia(prueba *testing.T) {
	catalogo := dobles.NuevoCatalogo()
	catalogo.AgregarCadena(1, "Jumbo", true)
	servicio := services.NuevoServicioTrabajos(dobles.NuevosTrabajos(), catalogo)
	if cadena, errorObtenido := servicio.ObtenerCadena(context.Background(), 1, ""); errorObtenido != nil || !cadena.EstaActiva {
		prueba.Fatalf("cadena=%+v error=%v", cadena, errorObtenido)
	}
	if _, errorObtenido := servicio.ObtenerCadena(context.Background(), 99, ""); !errors.Is(errorObtenido, domain.ErrCadenaNoEncontrada) {
		prueba.Fatalf("inexistente: %v", errorObtenido)
	}
}
