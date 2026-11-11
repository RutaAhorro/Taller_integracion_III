// Package listas define el modelo de las listas de compra (SUP-240) y su
// persistencia en PostgreSQL (postgres.go).
//
// Los nombres JSON van en snake_case, igual que en Catálogo.
package listas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// =============================================================================
// Reglas del dominio
// =============================================================================

// LargoMaximoNombre coincide con la columna rutas.listas_compras.nombre VARCHAR(150).
const LargoMaximoNombre = 150

// Estados lista los estados válidos de una lista.
var Estados = map[string]bool{"activa": true, "completada": true, "archivada": true}

// ErrNombre se devuelve cuando el nombre queda vacío o es demasiado largo.
// No repite el valor recibido.
var ErrNombre = errors.New(fmt.Sprintf("debe tener entre 1 y %d caracteres", LargoMaximoNombre))

// =============================================================================
// Modelos (forma del JSON de respuesta)
// =============================================================================

// Lista es el resumen de una lista de compra.
type Lista struct {
	ID             string    `json:"id"`              // UUID
	Nombre         string    `json:"nombre"`          // ya normalizado
	Estado         string    `json:"estado"`          // activa, completada o archivada
	TotalArticulos int       `json:"total_articulos"` // cantidad de artículos en la lista
	CreadaEl       time.Time `json:"creada_el"`       // siempre en UTC
	ActualizadaEl  time.Time `json:"actualizada_el"`  // siempre en UTC
}

// Articulo es un producto agregado a una lista.
type Articulo struct {
	ID                    int64  `json:"id"`
	ProductoNormalizadoID string `json:"producto_normalizado_id"`
	Cantidad              int    `json:"cantidad"`
	EstaComprado          bool   `json:"esta_comprado"`
}

// ListaDetalle es la lista con sus artículos (respuesta de GET /listas/{id}).
type ListaDetalle struct {
	Lista
	Articulos []Articulo `json:"articulos"` // nunca null: lista vacía es []
}

// =============================================================================
// Contrato de persistencia
// =============================================================================

// Repositorio es lo que el servidor HTTP necesita de la base de datos.
//
// Todas las operaciones filtran por el dueño (usuarioID). Cuando la lista no
// existe para ese usuario se devuelve nil (o false), nunca un error: así una
// lista ajena es indistinguible de una inexistente.
type Repositorio interface {
	Contar(contexto context.Context, usuarioID string) (int, error)
	Listar(contexto context.Context, usuarioID string, limite int) ([]Lista, error)
	Crear(contexto context.Context, usuarioID, nombre string) (Lista, error)
	Obtener(contexto context.Context, usuarioID, listaID string) (*ListaDetalle, error)
	Actualizar(contexto context.Context, usuarioID, listaID string, nombre, estado *string) (*Lista, error)
	Eliminar(contexto context.Context, usuarioID, listaID string) (bool, error)
}

// =============================================================================
// Normalización
// =============================================================================

// NormalizarNombre limpia el nombre que escribe el usuario:
//   - elimina caracteres de control y de formato (por ejemplo, el espacio de ancho cero);
//   - convierte tabulaciones y saltos de línea en espacios;
//   - colapsa espacios repetidos y quita los de los extremos;
//   - conserva tildes y ñ.
//
// El largo se mide en caracteres, no en bytes ("ñ" cuenta como 1).
func NormalizarNombre(valor string) (string, error) {
	var constructor strings.Builder
	for _, caracter := range valor {
		switch {
		case unicode.IsSpace(caracter):
			constructor.WriteRune(' ')
		case unicode.Is(unicode.Cc, caracter) || unicode.Is(unicode.Cf, caracter):
			// se descarta: control (Cc) o formato invisible (Cf)
		default:
			constructor.WriteRune(caracter)
		}
	}

	nombreLimpio := strings.Join(strings.Fields(constructor.String()), " ")
	largo := utf8.RuneCountInString(nombreLimpio)
	if largo < 1 || largo > LargoMaximoNombre {
		return "", ErrNombre
	}
	return nombreLimpio, nil
}
