// Pruebas de internal/config: variables obligatorias, valores por defecto,
// tokens internos, CORS y rechazo de valores inválidos al arrancar.
package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
)

// ===== Entorno de prueba =====

// claveJWTDePrueba cumple el mínimo de 32 bytes de JWT_SECRET.
var claveJWTDePrueba = strings.Repeat("j", 32)

// variablesCatalogo son todas las variables que lee config.Cargar.
var variablesCatalogo = []string{
	"DB_URL", "JWT_SECRET", "PORT", "CATALOGO_INTERNAL_TOKEN", "CATALOGO_CONSULTA_TOKEN",
	"CORS_ALLOWED_ORIGINS", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME",
	"CATALOGO_MAX_PRODUCTOS_CONSULTA", "CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA", "CATALOGO_CORS_ALLOWED_ORIGINS",
	"CATALOGO_DEFAULT_PAGE_SIZE", "CATALOGO_MAX_PAGE_SIZE", "CATALOGO_MAX_SEARCH_LENGTH", "CATALOGO_MAX_BATCH_PRODUCTOS",
	"CATALOGO_MAX_BODY_BYTES", "CATALOGO_READ_HEADER_TIMEOUT", "CATALOGO_READ_TIMEOUT", "CATALOGO_WRITE_TIMEOUT",
	"CATALOGO_IDLE_TIMEOUT", "CATALOGO_SHUTDOWN_TIMEOUT", "CATALOGO_STARTUP_TIMEOUT", "CATALOGO_READINESS_TIMEOUT",
}

// prepararEntornoMinimo deja definidas sólo las variables obligatorias.
func prepararEntornoMinimo(prueba *testing.T) {
	prueba.Helper()
	for _, nombre := range variablesCatalogo {
		prueba.Setenv(nombre, "")
	}
	prueba.Setenv("DB_URL", "postgres://test-only")
	prueba.Setenv("JWT_SECRET", claveJWTDePrueba)
}

// ===== Obligatorias y valores por defecto =====

func TestCargarExigeBaseDatosYClaveJWT(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("DB_URL", "")
	if _, errorCarga := config.Cargar(); errorCarga == nil {
		prueba.Fatal("se aceptó la configuración sin PostgreSQL")
	}
	prueba.Setenv("DB_URL", "postgres://test-only")
	for _, clave := range []string{"", "corta", " " + claveJWTDePrueba} {
		prueba.Setenv("JWT_SECRET", clave)
		if _, errorCarga := config.Cargar(); errorCarga == nil {
			prueba.Errorf("se aceptó JWT_SECRET inválido %q", clave)
		}
	}
}

func TestCargarAplicaValoresPorDefecto(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.DireccionEscucha != ":8080" || configuracion.MaximoConexionesAbiertas != 10 ||
		configuracion.MaximoConexionesInactivas != 5 || configuracion.VidaMaximaConexion != 30*time.Minute ||
		configuracion.MaximoProductosConsulta != 1000 || configuracion.AntiguedadMaximaDisponibilidad != 48*time.Hour ||
		len(configuracion.OrigenesPermitidos) != 0 || configuracion.TokenIngesta != "" || configuracion.TokenConsulta != "" ||
		configuracion.PlazoCabeceras != 5*time.Second || configuracion.PlazoLectura != 30*time.Second ||
		configuracion.PlazoEscritura != 30*time.Second || configuracion.PlazoInactividad != 60*time.Second ||
		configuracion.PlazoApagado != 15*time.Second || configuracion.PlazoInicio != 5*time.Second ||
		configuracion.PlazoReadiness != 2*time.Second || configuracion.TamanoPagina != 20 || configuracion.TamanoPaginaMaximo != 100 ||
		configuracion.LargoMaximoBusqueda != 100 || configuracion.MaximoProductosPorLote != 1000 ||
		configuracion.MaximoBytesCuerpo != 8<<20 || configuracion.RadioMaximoKm != 50 ||
		configuracion.VigenciaCacheSupermercados != 2*time.Minute {
		prueba.Fatalf("configuración predeterminada incorrecta: %+v", configuracion)
	}
}

func TestCargarAplicaValoresConfigurados(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("PORT", "9090")
	prueba.Setenv("CATALOGO_MAX_PRODUCTOS_CONSULTA", "75")
	prueba.Setenv("CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA", "36h")
	prueba.Setenv("CATALOGO_MAX_RADIO_KM", "25")
	prueba.Setenv("CATALOGO_CACHE_SUPERMERCADOS", "30s")
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.DireccionEscucha != ":9090" || configuracion.MaximoProductosConsulta != 75 ||
		configuracion.AntiguedadMaximaDisponibilidad != 36*time.Hour || configuracion.RadioMaximoKm != 25 ||
		configuracion.VigenciaCacheSupermercados != 30*time.Second {
		prueba.Fatalf("no se aplicaron los valores configurados: %+v", configuracion)
	}
}

func TestCargarRechazaValoresInvalidos(prueba *testing.T) {
	for _, caso := range []struct{ variable, valor string }{
		{"PORT", "0"}, {"PORT", "70000"}, {"PORT", "no-es-puerto"},
		{"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_IDLE_CONNS", "11"},
		{"DB_CONN_MAX_LIFETIME", "-1m"},
		{"CATALOGO_MAX_PRODUCTOS_CONSULTA", "0"},
		{"CATALOGO_MAX_RADIO_KM", "0"},
		{"CATALOGO_MAX_RADIO_KM", "-1"},
		{"CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA", "0s"},
		{"CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA", "48"},
	} {
		prueba.Run(caso.variable+"="+caso.valor, func(subprueba *testing.T) {
			prepararEntornoMinimo(subprueba)
			subprueba.Setenv(caso.variable, caso.valor)
			_, errorCarga := config.Cargar()
			if errorCarga == nil {
				subprueba.Fatal("se aceptó un valor inválido")
			}
			if !strings.Contains(errorCarga.Error(), caso.variable) {
				subprueba.Fatalf("el error no nombra la variable: %v", errorCarga)
			}
		})
	}
}

// ===== CORS =====

func TestCargarValidaOrigenesCORS(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	for _, origen := range []string{"*", "null", "https://*.example.com", "https://example.com/path", "https://user@example.com", "https://example.com?", "https://example.com#", "https://example.com,", "example.com", "ftp://example.com"} {
		prueba.Run(origen, func(subprueba *testing.T) {
			subprueba.Setenv("CORS_ALLOWED_ORIGINS", origen)
			if _, errorCarga := config.Cargar(); errorCarga == nil {
				subprueba.Fatal("se aceptó un origen inválido")
			}
		})
	}
	prueba.Setenv("CORS_ALLOWED_ORIGINS", " https://frontend.example, http://localhost:5173 ")
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil || len(configuracion.OrigenesPermitidos) != 2 || configuracion.OrigenesPermitidos[1] != "http://localhost:5173" {
		prueba.Fatalf("lista explícita inválida: %v", errorCarga)
	}
}

func TestCargarPriorizaLaVariableCORSDeCatalogo(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "https://generica.example")
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil || len(configuracion.OrigenesPermitidos) != 1 || configuracion.OrigenesPermitidos[0] != "https://generica.example" {
		prueba.Fatalf("sin la variable propia debe usarse CORS_ALLOWED_ORIGINS: %v %v", configuracion.OrigenesPermitidos, errorCarga)
	}
	prueba.Setenv("CATALOGO_CORS_ALLOWED_ORIGINS", "https://catalogo.example")
	configuracion, errorCarga = config.Cargar()
	if errorCarga != nil || len(configuracion.OrigenesPermitidos) != 1 || configuracion.OrigenesPermitidos[0] != "https://catalogo.example" {
		prueba.Fatalf("CATALOGO_CORS_ALLOWED_ORIGINS debe tener prioridad: %v %v", configuracion.OrigenesPermitidos, errorCarga)
	}
	prueba.Setenv("CATALOGO_CORS_ALLOWED_ORIGINS", "*")
	if _, errorCarga = config.Cargar(); errorCarga == nil || !strings.Contains(errorCarga.Error(), "CATALOGO_CORS_ALLOWED_ORIGINS (o CORS_ALLOWED_ORIGINS)") {
		prueba.Fatalf("el error debe nombrar ambas variables: %v", errorCarga)
	}
}

// ===== Plazos y límites =====

func TestCargarAplicaPlazosYLimitesConfigurados(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("CATALOGO_READINESS_TIMEOUT", "750ms")
	prueba.Setenv("CATALOGO_SHUTDOWN_TIMEOUT", "3s")
	prueba.Setenv("CATALOGO_DEFAULT_PAGE_SIZE", "10")
	prueba.Setenv("CATALOGO_MAX_PAGE_SIZE", "50")
	prueba.Setenv("CATALOGO_MAX_BODY_BYTES", "4096")
	configuracion, errorCarga := config.Cargar()
	if errorCarga != nil || configuracion.PlazoReadiness != 750*time.Millisecond || configuracion.PlazoApagado != 3*time.Second ||
		configuracion.TamanoPagina != 10 || configuracion.TamanoPaginaMaximo != 50 || configuracion.MaximoBytesCuerpo != 4096 {
		prueba.Fatalf("valores configurados ignorados: %+v %v", configuracion, errorCarga)
	}
}

func TestCargarRechazaPlazosYLimitesInvalidos(prueba *testing.T) {
	for _, caso := range []struct{ nombre, valor string }{
		{"CATALOGO_READ_TIMEOUT", "0s"},
		{"CATALOGO_STARTUP_TIMEOUT", "rapido"},
		{"CATALOGO_MAX_BATCH_PRODUCTOS", "0"},
		{"CATALOGO_MAX_SEARCH_LENGTH", "-1"},
		{"CATALOGO_DEFAULT_PAGE_SIZE", "500"}, // supera CATALOGO_MAX_PAGE_SIZE por defecto
	} {
		prueba.Run(caso.nombre, func(subprueba *testing.T) {
			prepararEntornoMinimo(subprueba)
			subprueba.Setenv(caso.nombre, caso.valor)
			if _, errorCarga := config.Cargar(); errorCarga == nil || !strings.Contains(errorCarga.Error(), caso.nombre) {
				subprueba.Fatalf("se esperaba un error que nombre %s: %v", caso.nombre, errorCarga)
			}
		})
	}
}

// ===== Tokens internos =====

func TestCargarValidaTokenIngesta(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	for _, token := range []string{"corto", claveJWTDePrueba, strings.Repeat("i", 32) + "\n"} {
		prueba.Setenv("CATALOGO_INTERNAL_TOKEN", token)
		if _, errorCarga := config.Cargar(); errorCarga == nil {
			prueba.Errorf("se aceptó token inválido %q", token)
		}
	}
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("i", 34))
	if configuracion, errorCarga := config.Cargar(); errorCarga != nil || configuracion.TokenIngesta != strings.Repeat("i", 34) {
		prueba.Fatal("token de ingesta válido rechazado", errorCarga)
	}
}

func TestCargarValidaTokenConsulta(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("CATALOGO_INTERNAL_TOKEN", strings.Repeat("i", 32))
	for _, token := range []string{"corto", claveJWTDePrueba, strings.Repeat("i", 32), strings.Repeat("c", 31) + "\n"} {
		prueba.Setenv("CATALOGO_CONSULTA_TOKEN", token)
		if _, errorCarga := config.Cargar(); errorCarga == nil {
			prueba.Errorf("se aceptó token de consulta inválido %q", token)
		}
	}
	prueba.Setenv("CATALOGO_CONSULTA_TOKEN", strings.Repeat("c", 32))
	if configuracion, errorCarga := config.Cargar(); errorCarga != nil || configuracion.TokenConsulta != strings.Repeat("c", 32) {
		prueba.Fatal("token de consulta válido rechazado", errorCarga)
	}
}
