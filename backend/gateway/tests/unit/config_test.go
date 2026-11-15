package unit_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

func TestLoadDefaults(prueba *testing.T) {
	configuracion, err := config.Load(func(key string) string {
		if key == "BACKEND_URL" {
			return "http://127.0.0.1:8080"
		}
		return ""
	})
	if err != nil {
		prueba.Fatal(err)
	}
	if configuracion.ListenAddr != ":8082" || configuracion.RequestTimeout != 30*time.Second || configuracion.ReadinessTimeout != 2*time.Second || configuracion.KeepAliveTimeout != 30*time.Second || len(configuracion.TrustedProxies) != 0 {
		prueba.Fatalf("unexpected defaults: %+v", configuracion)
	}
	if len(configuracion.CatalogPrefixes) != 2 || len(configuracion.IdentityPrefixes) != 2 || len(configuracion.RoutesPrefixes) != 1 || len(configuracion.ScrapingPrefixes) != 2 || len(configuracion.IAPrefixes) != 1 {
		prueba.Fatalf("unexpected default route prefixes: %+v", configuracion)
	}
}

func TestLoadRejectsInvalidConfiguration(prueba *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"BACKEND_URL", ""},
		{"BACKEND_URL", "ftp://backend:8080"},
		{"BACKEND_URL", "http://user:password@backend:8080"},
		{"BACKEND_URL", "http://backend:8080/api"},
		{"BACKEND_URL", "http://backend:8080?token=private"},
		{"BACKEND_URL", "http://backend:8080#fragment"},
		{"BACKEND_URL", "http://backend:70000"},
		{"IDENTIDAD_URL", "ftp://identidad:8080"},
		{"IDENTIDAD_URL", "http://user:password@identidad:8080"},
		{"IDENTIDAD_URL", "http://identidad:8080/api/v1/auth"},
		{"IDENTIDAD_URL", "http://identidad:70000"},
		{"SCRAPING_URL", "ftp://control:8080"},
		{"SCRAPING_URL", "http://user:password@control:8080"},
		{"SCRAPING_URL", "http://control:8080/api"},
		{"SCRAPING_URL", "http://control:8080?token=private"},
		{"SCRAPING_URL", "http://control:8080#fragment"},
		{"SCRAPING_URL", "http://control:70000"},
		{"CATALOGO_URL", "ftp://catalogo:8080"},
		{"CATALOGO_URL", "http://user:password@catalogo:8080"},
		{"CATALOGO_URL", "http://catalogo:8080/api"},
		{"CATALOGO_URL", "http://catalogo:8080?token=private"},
		{"CATALOGO_URL", "http://catalogo:8080#fragment"},
		{"CATALOGO_URL", "http://catalogo:70000"},
		{"RUTAS_URL", "ftp://motor-rutas:8001"},
		{"RUTAS_URL", "http://user:password@motor-rutas:8001"},
		{"RUTAS_URL", "http://motor-rutas:8001/api/v1/listas"},
		{"RUTAS_URL", "http://motor-rutas:8001?token=private"},
		{"IA_URL", "ftp://ia-conversacional:8002"},
		{"IA_URL", "http://user:password@ia-conversacional:8002"},
		{"IA_URL", "http://ia-conversacional:8002/api"},
		{"IA_URL", "http://ia-conversacional:8002?token=private"},
		{"IA_URL", "http://ia-conversacional:70000"},
		{"GATEWAY_LISTEN_ADDR", "8082"},
		{"GATEWAY_LISTEN_ADDR", ":0"},
		{"GATEWAY_REQUEST_TIMEOUT", "-1s"},
		{"GATEWAY_DIAL_TIMEOUT", "0s"},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", "soon"},
		{"GATEWAY_READINESS_TIMEOUT", "0s"},
		{"GATEWAY_KEEPALIVE_TIMEOUT", "-5s"},
		{"GATEWAY_TRUSTED_PROXIES", "localhost"},
	} {
		prueba.Run(tc.key+"="+tc.value, func(subprueba *testing.T) {
			_, err := config.Load(func(key string) string {
				if key == tc.key {
					return tc.value
				}
				if key == "BACKEND_URL" {
					return "http://backend:8080"
				}
				return ""
			})
			if err == nil {
				subprueba.Fatal("expected configuration error")
			}
			if strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "private") {
				subprueba.Fatalf("error exposes supplied credentials: %v", err)
			}
		})
	}
}

func TestFinalTopologyDoesNotRequireLegacyBackend(prueba *testing.T) {
	env := map[string]string{
		"SCRAPING_URL":  "http://control-scraping:8080",
		"IDENTIDAD_URL": "http://identidad:8080",
		"CATALOGO_URL":  "http://catalogo:8080",
		"RUTAS_URL":     "http://motor-rutas:8001",
		"IA_URL":        "http://ia-conversacional:8002",
	}
	configuracion, err := config.Load(func(key string) string { return env[key] })
	if err != nil || configuracion.Backend != nil || configuracion.Scraping == nil || configuracion.Identidad == nil || configuracion.Catalogo == nil || configuracion.Rutas == nil || configuracion.IA == nil {
		prueba.Fatalf("topología final inválida: %v %v", configuracion, err)
	}
}

func TestLoadOverrides(prueba *testing.T) {
	env := map[string]string{
		"BACKEND_URL": "https://[::1]:8443/", "GATEWAY_LISTEN_ADDR": "127.0.0.1:9000",
		"CATALOGO_URL":            "http://catalogo:8080",
		"GATEWAY_REQUEST_TIMEOUT": "2s", "GATEWAY_TRUSTED_PROXIES": "127.0.0.1,10.0.0.0/24,::1",
		"GATEWAY_KEEPALIVE_TIMEOUT": "15s",
		"GATEWAY_CATALOGO_PREFIXES": "/api/v1/custom-productos",
	}
	configuracion, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		prueba.Fatal(err)
	}
	if configuracion.ListenAddr != env["GATEWAY_LISTEN_ADDR"] || configuracion.Catalogo.String() != env["CATALOGO_URL"] || configuracion.RequestTimeout != 2*time.Second || configuracion.KeepAliveTimeout != 15*time.Second || len(configuracion.TrustedProxies) != 3 {
		prueba.Fatalf("overrides not applied: %+v", configuracion)
	}
	if len(configuracion.CatalogPrefixes) != 1 || configuracion.CatalogPrefixes[0] != "/api/v1/custom-productos" {
		prueba.Fatalf("custom catalog prefix not applied: %+v", configuracion.CatalogPrefixes)
	}
}

// TestRoutePrefixValidation recorre los formatos de prefijo aceptados y
// rechazados. El "/" final se recorta; el resto de formatos inválidos impide
// arrancar y el error nombra la variable.
func TestRoutePrefixValidation(prueba *testing.T) {
	for _, caso := range []struct {
		nombre, valor string
		esperado      []string
		debeFallar    bool
	}{
		{nombre: "valido", valor: "/api/v1/productos,/api/v1/admin/productos", esperado: []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{nombre: "vacio_usa_defaults", valor: "", esperado: []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{nombre: "solo_espacios_usa_defaults", valor: "   ", esperado: []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{nombre: "barra_final_se_recorta", valor: "/api/v1/productos/", esperado: []string{"/api/v1/productos"}},
		{nombre: "varias_barras_finales_se_recortan", valor: "/api/v1/productos//", esperado: []string{"/api/v1/productos"}},
		{nombre: "espacios_alrededor_se_recortan", valor: " /api/v1/productos , /api/v1/admin/productos ", esperado: []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{nombre: "url_completa", valor: "http://catalogo:8080/api/v1/productos", debeFallar: true},
		{nombre: "sin_barra_inicial", valor: "api/v1/productos", debeFallar: true},
		{nombre: "relativa_al_esquema", valor: "//catalogo/api/v1/productos", debeFallar: true},
		{nombre: "query", valor: "/api/v1/productos?en_stock=true", debeFallar: true},
		{nombre: "fragmento", valor: "/api/v1/productos#lista", debeFallar: true},
		{nombre: "espacio_interno", valor: "/api/v1/mis productos", debeFallar: true},
		{nombre: "tabulador_interno", valor: "/api/v1/mis\tproductos", debeFallar: true},
		{nombre: "codificado", valor: "/api/v1/productos%2Fadmin", debeFallar: true},
		{nombre: "segmento_punto", valor: "/api/v1/../admin", debeFallar: true},
		{nombre: "segmento_vacio", valor: "/api//productos", debeFallar: true},
		{nombre: "solo_raiz", valor: "/", debeFallar: true},
		{nombre: "elemento_vacio", valor: "/api/v1/productos,,/api/v1/admin/productos", debeFallar: true},
		{nombre: "coma_final", valor: "/api/v1/productos,", debeFallar: true},
		{nombre: "duplicado", valor: "/api/v1/productos,/api/v1/productos", debeFallar: true},
		{nombre: "duplicado_tras_recortar", valor: "/api/v1/productos,/api/v1/productos/", debeFallar: true},
		{nombre: "duplicado_con_otro_servicio", valor: "/api/v1/auth", debeFallar: true},
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			configuracion, errorCarga := config.Load(func(clave string) string {
				switch clave {
				case "BACKEND_URL":
					return "http://backend:8080"
				case "GATEWAY_CATALOGO_PREFIXES":
					return caso.valor
				}
				return ""
			})
			if caso.debeFallar {
				if errorCarga == nil {
					subprueba.Fatalf("se esperaba un error para %q; prefijos: %v", caso.valor, configuracion.CatalogPrefixes)
				}
				if !strings.Contains(errorCarga.Error(), "GATEWAY_CATALOGO_PREFIXES") {
					subprueba.Fatalf("el error no nombra la variable: %v", errorCarga)
				}
				return
			}
			if errorCarga != nil {
				subprueba.Fatalf("error inesperado: %v", errorCarga)
			}
			if !reflect.DeepEqual(configuracion.CatalogPrefixes, caso.esperado) {
				subprueba.Fatalf("prefijos = %v, se esperaba %v", configuracion.CatalogPrefixes, caso.esperado)
			}
		})
	}
}

// TestEveryPrefixVariableIsValidated comprueba que las cuatro variables pasan
// por la misma validación.
func TestEveryPrefixVariableIsValidated(prueba *testing.T) {
	for _, variable := range []string{"GATEWAY_CATALOGO_PREFIXES", "GATEWAY_IDENTIDAD_PREFIXES", "GATEWAY_RUTAS_PREFIXES", "GATEWAY_SCRAPING_PREFIXES", "GATEWAY_IA_PREFIXES"} {
		prueba.Run(variable, func(subprueba *testing.T) {
			_, errorCarga := config.Load(func(clave string) string {
				switch clave {
				case "BACKEND_URL":
					return "http://backend:8080"
				case variable:
					return "https://externo.example/api?token=1"
				}
				return ""
			})
			if errorCarga == nil || !strings.Contains(errorCarga.Error(), variable) {
				subprueba.Fatalf("se esperaba un error que nombre %s, se obtuvo %v", variable, errorCarga)
			}
		})
	}
}
