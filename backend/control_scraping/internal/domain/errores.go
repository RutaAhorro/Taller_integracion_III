// Package domain reúne los modelos, contratos de persistencia y errores de
// negocio de Control de scraping. No depende de HTTP, SQL ni Redis: services lo
// usa para expresar reglas y repositories para implementar los contratos.
package domain

import (
	"errors"
	"strconv"
)

// ===== Errores de negocio =====

// Los errores viven en domain para que repositories y services los compartan
// sin que la persistencia dependa de la capa de negocio. Los mensajes forman
// parte de las respuestas HTTP actuales y no se cambian.
var (
	ErrTrabajoNoEncontrado   = errors.New("trabajo de scraping no encontrado")
	ErrTrabajoYaFinalizado   = errors.New("el trabajo de scraping ya ha sido finalizado")
	ErrCadenaNoEncontrada    = errors.New("cadena de supermercado no encontrada")
	ErrSucursalNoEncontrada  = errors.New("sucursal de supermercado no encontrada")
	ErrLoteVacio             = errors.New("el lote de productos no puede estar vacío")
	ErrLoteExcedeMaximo      = errors.New("el lote excede el tamaño máximo permitido de " + strconv.Itoa(MaximoProductosPorLote) + " productos")
	ErrEstadoTrabajoInvalido = errors.New("el estado debe ser '" + EstadoCompletado + "' o '" + EstadoFallido + "'")
	ErrUUIDInvalido          = errors.New("el identificador proporcionado no es un UUID válido")
	ErrFuenteDeshabilitada   = errors.New("la fuente de scraping se encuentra deshabilitada")
	ErrCatalogoNoDisponible  = errors.New("Catálogo no está disponible")
	ErrCatalogoTimeout       = errors.New("Catálogo no respondió a tiempo")
	ErrContadorPendiente     = errors.New("Catálogo confirmó el lote y el contador está pendiente; conservar la clave del lote")
	ErrIdempotencia          = errors.New("la clave ya fue utilizada con otro contenido")
	ErrEntregaPendiente      = errors.New("el lote está registrado y pendiente de confirmación")
	ErrEntregaFallida        = errors.New("hay una entrega fallida o rechazada; requiere revisión")
	ErrCatalogoRechazo       = errors.New("Catálogo rechazó el lote")
)
