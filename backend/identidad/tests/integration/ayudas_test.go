// Arma el router público para las pruebas de integración que no necesitan
// PostgreSQL ni Redis reales.
package integration_test

import (
	"context"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/api"
	"github.com/danirc2024/Taller_integracion_III/backend/identidad/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dsnSinConexion apunta a un puerto cerrado: toda consulta falla de inmediato,
// igual que con la base caída, sin tocar ninguna base compartida.
const dsnSinConexion = "host=127.0.0.1 port=1 user=app_identity dbname=sin_conexion sslmode=disable connect_timeout=1"

// enrutadorDePrueba construye el router con el módulo real sobre una base
// inalcanzable y sin Redis. Completa los ajustes predeterminados si faltan.
func enrutadorDePrueba(prueba *testing.T, configuracion config.Configuracion, sondaBase func(context.Context) error) *gin.Engine {
	prueba.Helper()
	if configuracion.Ajustes == (config.Ajustes{}) {
		configuracion.Ajustes = config.AjustesPredeterminados()
	}
	base, errorApertura := gorm.Open(postgres.Open(dsnSinConexion), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if errorApertura != nil {
		prueba.Fatal(errorApertura)
	}
	modulo := api.NuevoModulo(base, nil, configuracion)
	return api.NuevoEnrutador(configuracion, modulo, nil, sondaBase, nil)
}
