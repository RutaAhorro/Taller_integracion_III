// Package unit_test prueba la carga de configuración del Gateway sin red.
package unit_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Ayudantes =====

// origenControl es el SCRAPING_URL mínimo para que la carga sea válida.
const origenControl = "http://control-scraping:8080"

// entornoCon devuelve una función de entorno con SCRAPING_URL y los valores dados.
func entornoCon(valores map[string]string) func(string) string {
	return func(clave string) string {
		if valor, existe := valores[clave]; existe {
			return valor
		}
		if clave == "SCRAPING_URL" {
			return origenControl
		}
		return ""
	}
}

// ===== Valores por defecto =====

// TestCargarValoresPorDefecto verifica puerto, límites y prefijos por defecto.
func TestCargarValoresPorDefecto(prueba *testing.T) {
	configuracion, errorCarga := config.Cargar(entornoCon(nil))
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.DireccionEscucha != ":8082" || configuracion.LimitePeticion != 30*time.Second ||
		configuracion.LimiteDiagnostico != 2*time.Second || configuracion.LimiteKeepAlive != 30*time.Second ||
		len(configuracion.ProxiesFiables) != 0 {
		prueba.Fatalf("valores por defecto inesperados: %+v", configuracion)
	}
	if len(configuracion.PrefijosCatalogo) != 2 || len(configuracion.PrefijosIdentidad) != 2 || len(configuracion.PrefijosRutas) != 1 ||
		len(configuracion.PrefijosScraping) != 2 || len(configuracion.PrefijosIA) != 1 {
		prueba.Fatalf("prefijos por defecto inesperados: %+v", configuracion)
	}
	if configuracion.Catalogo != nil || configuracion.Identidad != nil || configuracion.Rutas != nil || configuracion.IA != nil {
		prueba.Fatalf("los dominios opcionales deben quedar sin origen: %+v", configuracion)
	}
}

// TestTopologiaCompleta carga los cinco orígenes de Compose/K8s.
func TestTopologiaCompleta(prueba *testing.T) {
	configuracion, errorCarga := config.Cargar(entornoCon(map[string]string{
		"IDENTIDAD_URL": "http://identidad:8080",
		"CATALOGO_URL":  "http://catalogo:8080",
		"RUTAS_URL":     "http://motor-rutas:8001",
		"IA_URL":        "http://ia-conversacional:8002",
	}))
	if errorCarga != nil || configuracion.Scraping == nil || configuracion.Identidad == nil ||
		configuracion.Catalogo == nil || configuracion.Rutas == nil || configuracion.IA == nil {
		prueba.Fatalf("topología inválida: %+v %v", configuracion, errorCarga)
	}
}

// TestCargarAplicaValores verifica que cada variable reemplaza su valor por defecto.
func TestCargarAplicaValores(prueba *testing.T) {
	entorno := map[string]string{
		"SCRAPING_URL": "https://[::1]:8443/", "GATEWAY_LISTEN_ADDR": "127.0.0.1:9000",
		"CATALOGO_URL":              "http://catalogo:8080",
		"GATEWAY_REQUEST_TIMEOUT":   "2s",
		"GATEWAY_TRUSTED_PROXIES":   "127.0.0.1,10.0.0.0/24,::1",
		"GATEWAY_KEEPALIVE_TIMEOUT": "15s",
		"GATEWAY_CATALOGO_PREFIXES": "/api/v1/productos-propios",
	}
	configuracion, errorCarga := config.Cargar(entornoCon(entorno))
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.DireccionEscucha != entorno["GATEWAY_LISTEN_ADDR"] || configuracion.Catalogo.String() != entorno["CATALOGO_URL"] ||
		configuracion.LimitePeticion != 2*time.Second || configuracion.LimiteKeepAlive != 15*time.Second ||
		len(configuracion.ProxiesFiables) != 3 {
		prueba.Fatalf("no se aplicaron los valores: %+v", configuracion)
	}
	if !reflect.DeepEqual(configuracion.PrefijosCatalogo, []string{"/api/v1/productos-propios"}) {
		prueba.Fatalf("no se aplicó el prefijo propio: %v", configuracion.PrefijosCatalogo)
	}
}

// ===== Rechazos =====

// TestCargarRechazaConfiguracionInvalida recorre valores que deben impedir
// arrancar, incluido SCRAPING_URL vacío, y que el error no repita secretos.
func TestCargarRechazaConfiguracionInvalida(prueba *testing.T) {
	casos := []struct{ clave, valor string }{
		{"SCRAPING_URL", ""},
		{"GATEWAY_LISTEN_ADDR", "8082"},
		{"GATEWAY_LISTEN_ADDR", ":0"},
		{"GATEWAY_REQUEST_TIMEOUT", "-1s"},
		{"GATEWAY_DIAL_TIMEOUT", "0s"},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", "pronto"},
		{"GATEWAY_READINESS_TIMEOUT", "0s"},
		{"GATEWAY_KEEPALIVE_TIMEOUT", "-5s"},
		{"GATEWAY_TRUSTED_PROXIES", "localhost"},
	}
	for _, variable := range []string{"SCRAPING_URL", "CATALOGO_URL", "IDENTIDAD_URL", "RUTAS_URL", "IA_URL"} {
		for _, valor := range []string{
			"ftp://servicio:8080",
			"http://usuario:password@servicio:8080",
			"http://servicio:8080/api",
			"http://servicio:8080?token=private",
			"http://servicio:8080#fragmento",
			"http://servicio:70000",
		} {
			casos = append(casos, struct{ clave, valor string }{variable, valor})
		}
	}
	for _, caso := range casos {
		prueba.Run(caso.clave+"="+caso.valor, func(subprueba *testing.T) {
			_, errorCarga := config.Cargar(entornoCon(map[string]string{caso.clave: caso.valor}))
			if errorCarga == nil {
				subprueba.Fatal("se esperaba un error de configuración")
			}
			if strings.Contains(errorCarga.Error(), "password") || strings.Contains(errorCarga.Error(), "private") {
				subprueba.Fatalf("el error expone credenciales: %v", errorCarga)
			}
		})
	}
}

// TestBackendURLYaNoSeLee verifica que BACKEND_URL no suple a SCRAPING_URL.
func TestBackendURLYaNoSeLee(prueba *testing.T) {
	_, errorCarga := config.Cargar(func(clave string) string {
		if clave == "BACKEND_URL" {
			return "http://backend:8080"
		}
		return ""
	})
	if errorCarga == nil || !strings.Contains(errorCarga.Error(), "SCRAPING_URL") {
		prueba.Fatalf("sin SCRAPING_URL debe fallar nombrándola: %v", errorCarga)
	}
}

// ===== Prefijos =====

// TestValidacionDePrefijos recorre formatos aceptados y rechazados. El "/"
// final se recorta; el resto de formatos inválidos impide arrancar y el error
// nombra la variable.
func TestValidacionDePrefijos(prueba *testing.T) {
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
			configuracion, errorCarga := config.Cargar(entornoCon(map[string]string{"GATEWAY_CATALOGO_PREFIXES": caso.valor}))
			if caso.debeFallar {
				if errorCarga == nil {
					subprueba.Fatalf("se esperaba un error para %q; prefijos: %v", caso.valor, configuracion.PrefijosCatalogo)
				}
				if !strings.Contains(errorCarga.Error(), "GATEWAY_CATALOGO_PREFIXES") {
					subprueba.Fatalf("el error no nombra la variable: %v", errorCarga)
				}
				return
			}
			if errorCarga != nil {
				subprueba.Fatalf("error inesperado: %v", errorCarga)
			}
			if !reflect.DeepEqual(configuracion.PrefijosCatalogo, caso.esperado) {
				subprueba.Fatalf("prefijos = %v, se esperaba %v", configuracion.PrefijosCatalogo, caso.esperado)
			}
		})
	}
}

// TestCadaVariableDePrefijosSeValida comprueba que las cinco variables de
// prefijos pasan por la misma validación.
func TestCadaVariableDePrefijosSeValida(prueba *testing.T) {
	for _, variable := range []string{"GATEWAY_CATALOGO_PREFIXES", "GATEWAY_IDENTIDAD_PREFIXES", "GATEWAY_RUTAS_PREFIXES", "GATEWAY_SCRAPING_PREFIXES", "GATEWAY_IA_PREFIXES"} {
		prueba.Run(variable, func(subprueba *testing.T) {
			_, errorCarga := config.Cargar(entornoCon(map[string]string{variable: "https://externo.example/api?token=1"}))
			if errorCarga == nil || !strings.Contains(errorCarga.Error(), variable) {
				subprueba.Fatalf("se esperaba un error que nombre %s, se obtuvo %v", variable, errorCarga)
			}
		})
	}
}
