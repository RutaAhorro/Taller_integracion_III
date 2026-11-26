// Valores y constructores compartidos por las pruebas unitarias. El código de
// producción recibe siempre su configuración por parámetro; aquí se fijan los
// valores de prueba en un único lugar en vez de depender del entorno.
package unit_test

import (
	"context"
	"errors"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/services"
	"google.golang.org/api/idtoken"
)

// ===== Valores de prueba =====

const (
	// claveJWTDePrueba firma y valida las sesiones emitidas dentro de las pruebas.
	claveJWTDePrueba = "sup272-test-only-key-at-least-thirty-two-bytes"
	// clienteGoogleDePrueba es la audiencia que esperan los validadores falsos.
	clienteGoogleDePrueba = "fixture-google-client"
)

// plazoGoogleDePrueba es el plazo de producción para validar un id_token.
var plazoGoogleDePrueba = config.AjustesPredeterminados().PlazoGoogle

// ===== Constructores =====

// nuevoServicioAuth arma el servicio sin login Google ni admins configurados,
// que es lo que necesitan las pruebas de registro, login local y perfil.
func nuevoServicioAuth(repositorio domain.RepositorioUsuarios) *services.ServicioAuth {
	return services.NuevoServicioAuth(repositorio, idtoken.Validate, "", plazoGoogleDePrueba, nil)
}

// usuariosQueFallan simula una base caída al buscar la cuenta.
type usuariosQueFallan struct{ *mockUsuarioRepository }

// BuscarPorCorreo siempre falla.
func (usuariosQueFallan) BuscarPorCorreo(context.Context, string) (*domain.Usuario, error) {
	return nil, errors.New("conexión rechazada")
}
