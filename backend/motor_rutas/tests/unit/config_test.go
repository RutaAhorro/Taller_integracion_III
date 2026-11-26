package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/config"
)

var valido = map[string]string{
	"RUTAS_DB_URL":            "postgres://app_rutas:fixture-private@db:5432/supermercados?sslmode=disable",
	"JWT_SECRET":              strings.Repeat("j", 32),
	"CORS_ALLOWED_ORIGINS":    "https://frontend.example, http://localhost:5173",
	"CATALOGO_INTERNAL_URL":   "http://catalogo:8080/",
	"CATALOGO_CONSULTA_TOKEN": strings.Repeat("t", 32) + "-fixture-private",
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
	// La barra final del origen se quita para no duplicarla al armar la URL.
	if configuracion.CatalogoURL != "http://catalogo:8080" || configuracion.CatalogoPlazo != 5*time.Second {
		prueba.Fatalf("catálogo por defecto: %q %v", configuracion.CatalogoURL, configuracion.CatalogoPlazo)
	}
	if configuracion.RadioKm != 10 || configuracion.MaxSucursales != 10 || configuracion.MaxParadas != 3 ||
		configuracion.MaxArticulos != 30 || configuracion.FactorDistancia != 1.3 || configuracion.UmbralAhorro != 500 {
		prueba.Fatalf("parámetros de rutas por defecto: %+v", configuracion)
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
		{"CATALOGO_INTERNAL_URL", ""},
		{"CATALOGO_INTERNAL_URL", "ftp://catalogo"},
		{"CATALOGO_INTERNAL_URL", "http://catalogo:8080/internal"},
		{"CATALOGO_INTERNAL_URL", "http://usuario:fixture-private@catalogo"},
		{"CATALOGO_CONSULTA_TOKEN", ""},
		{"CATALOGO_CONSULTA_TOKEN", "fixture-private"},
		{"CATALOGO_CONSULTA_TOKEN", " " + strings.Repeat("t", 32) + "-fixture-private"},
		{"CATALOGO_CONSULTA_TOKEN", valido["JWT_SECRET"]},
		{"RUTAS_CATALOGO_TIMEOUT", "0s"},
		{"RUTAS_CATALOGO_TIMEOUT", "31s"},
		{"RUTAS_CATALOGO_TIMEOUT", "dos"},
		{"RUTAS_RADIO_KM", "0"},
		{"RUTAS_RADIO_KM", "51"},
		{"RUTAS_RADIO_KM", "NaN"},
		{"RUTAS_MAX_SUCURSALES", "0"},
		{"RUTAS_MAX_SUCURSALES", "11"},
		{"RUTAS_MAX_PARADAS", "0"},
		{"RUTAS_MAX_PARADAS", "4"},
		{"RUTAS_MAX_ARTICULOS", "0"},
		{"RUTAS_MAX_ARTICULOS", "101"},
		{"RUTAS_FACTOR_DISTANCIA", "0"},
		{"RUTAS_FACTOR_DISTANCIA", "5.5"},
		{"RUTAS_UMBRAL_AHORRO", "-1"},
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
