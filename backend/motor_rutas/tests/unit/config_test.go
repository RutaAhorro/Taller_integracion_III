package unit_test

import (
	"strings"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/config"
)

var valido = map[string]string{
	"RUTAS_DB_URL":         "postgres://app_rutas:fixture-private@db:5432/supermercados?sslmode=disable",
	"JWT_SECRET":           strings.Repeat("j", 32),
	"CORS_ALLOWED_ORIGINS": "https://frontend.example, http://localhost:5173",
}

func entorno(cambios map[string]string) func(string) (string, bool) {
	variables := map[string]string{}
	for clave, valor := range valido {
		variables[clave] = valor
	}
	for clave, valor := range cambios {
		variables[clave] = valor
	}
	return func(clave string) (string, bool) { valor, existe := variables[clave]; return valor, existe }
}

func TestConfigValida(prueba *testing.T) {
	configuracion, errorObtenido := config.Cargar(entorno(nil))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if strings.Join(configuracion.OrigenesCORS, "|") != "https://frontend.example|http://localhost:5173" ||
		configuracion.DBPoolMax != 5 || configuracion.MaxListas != 100 || configuracion.Puerto != "8001" {
		prueba.Fatalf("%+v", configuracion)
	}
}

func TestConfigRechazaSinExponerSecretos(prueba *testing.T) {
	casos := [][2]string{
		{"RUTAS_DB_URL", ""},
		{"RUTAS_DB_URL", "postgres://postgres:fixture-private@db/supermercados"},
		{"RUTAS_DB_URL", "postgres://app_catalogo:fixture-private@db/supermercados"},
		{"RUTAS_DB_URL", "postgres://app_rutas@db/supermercados"},
		{"RUTAS_DB_URL", "postgres://app_rutas:fixture-private@db"},
		{"RUTAS_DB_URL", "mysql://app_rutas:fixture-private@db/supermercados"},
		{"RUTAS_DB_URL", "postgres://app_rutas:fixture-private@db:99999999/supermercados"},
		{"JWT_SECRET", "corta"},
		{"JWT_SECRET", " " + strings.Repeat("j", 32)},
		{"CORS_ALLOWED_ORIGINS", "*"},
		{"CORS_ALLOWED_ORIGINS", "null"},
		{"CORS_ALLOWED_ORIGINS", "https://frontend.example/ruta"},
		{"CORS_ALLOWED_ORIGINS", "https://user:fixture-private@frontend.example"},
		{"RUTAS_DB_POOL_MAX", "0"},
		{"RUTAS_DB_POOL_MAX", "muchos"},
		{"RUTAS_DB_POOL_MAX", "+5"},
		{"RUTAS_MAX_LISTAS", "0"},
		{"PORT", "http"},
	}
	for _, caso := range casos {
		_, errorObtenido := config.Cargar(entorno(map[string]string{caso[0]: caso[1]}))
		if errorObtenido == nil {
			prueba.Errorf("%s=%q debió fallar", caso[0], caso[1])
			continue
		}
		if strings.Contains(errorObtenido.Error(), "fixture-private") {
			prueba.Errorf("%s: el error expone el secreto", caso[0])
		}
	}
}
