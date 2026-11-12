package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/services"
)

// variablesDeLimites son las variables CATALOGO_* de plazos y límites (SUP-349).
var variablesDeLimites = []string{
	"CATALOGO_READ_HEADER_TIMEOUT", "CATALOGO_READ_TIMEOUT", "CATALOGO_WRITE_TIMEOUT",
	"CATALOGO_IDLE_TIMEOUT", "CATALOGO_SHUTDOWN_TIMEOUT", "CATALOGO_STARTUP_TIMEOUT",
	"CATALOGO_READINESS_TIMEOUT", "CATALOGO_DEFAULT_PAGE_SIZE", "CATALOGO_MAX_PAGE_SIZE",
	"CATALOGO_MAX_SEARCH_LENGTH", "CATALOGO_MAX_BATCH_PRODUCTOS",
	"CATALOGO_CORS_ALLOWED_ORIGINS", "CORS_ALLOWED_ORIGINS",
}

// prepararEntornoMinimo deja solo las variables obligatorias definidas.
func prepararEntornoMinimo(prueba *testing.T) {
	prueba.Helper()
	for _, nombre := range variablesDeLimites {
		prueba.Setenv(nombre, "")
	}
	prueba.Setenv("DB_URL", "postgres://test-only")
	prueba.Setenv("JWT_SECRET", "test-only")
}

func TestLimitesPredeterminadosConservanLosValoresAnteriores(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	configuracion, errorCarga := config.Load()
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	plazosEsperados := map[string][2]time.Duration{
		"ReadHeaderTimeout": {configuracion.ReadHeaderTimeout, 5 * time.Second},
		"ReadTimeout":       {configuracion.ReadTimeout, 30 * time.Second},
		"WriteTimeout":      {configuracion.WriteTimeout, 30 * time.Second},
		"IdleTimeout":       {configuracion.IdleTimeout, 60 * time.Second},
		"ShutdownTimeout":   {configuracion.ShutdownTimeout, 15 * time.Second},
		"StartupTimeout":    {configuracion.StartupTimeout, 5 * time.Second},
		"ReadinessTimeout":  {configuracion.ReadinessTimeout, 2 * time.Second},
	}
	for nombre, par := range plazosEsperados {
		if par[0] != par[1] {
			prueba.Errorf("%s = %s, se esperaba %s", nombre, par[0], par[1])
		}
	}
	if configuracion.DefaultPageSize != 20 || configuracion.MaxPageSize != 100 ||
		configuracion.MaxSearchLength != 100 || configuracion.MaxBatchProductos != 1000 {
		prueba.Fatalf("límites por defecto incorrectos: %+v", configuracion)
	}
}

func TestLimitesSePuedenConfigurar(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("CATALOGO_READINESS_TIMEOUT", "750ms")
	prueba.Setenv("CATALOGO_SHUTDOWN_TIMEOUT", "1m")
	prueba.Setenv("CATALOGO_DEFAULT_PAGE_SIZE", "12")
	prueba.Setenv("CATALOGO_MAX_PAGE_SIZE", "48")
	prueba.Setenv("CATALOGO_MAX_SEARCH_LENGTH", "64")
	prueba.Setenv("CATALOGO_MAX_BATCH_PRODUCTOS", "250")
	configuracion, errorCarga := config.Load()
	if errorCarga != nil {
		prueba.Fatal(errorCarga)
	}
	if configuracion.ReadinessTimeout != 750*time.Millisecond || configuracion.ShutdownTimeout != time.Minute ||
		configuracion.DefaultPageSize != 12 || configuracion.MaxPageSize != 48 ||
		configuracion.MaxSearchLength != 64 || configuracion.MaxBatchProductos != 250 {
		prueba.Fatalf("no se aplicaron los valores configurados: %+v", configuracion)
	}
}

func TestLimitesInvalidosImpidenArrancar(prueba *testing.T) {
	for _, caso := range []struct{ variable, valor string }{
		{"CATALOGO_READ_HEADER_TIMEOUT", "0s"},
		{"CATALOGO_READ_TIMEOUT", "-1s"},
		{"CATALOGO_WRITE_TIMEOUT", "pronto"},
		{"CATALOGO_IDLE_TIMEOUT", "60"},
		{"CATALOGO_SHUTDOWN_TIMEOUT", "0"},
		{"CATALOGO_STARTUP_TIMEOUT", "-5s"},
		{"CATALOGO_READINESS_TIMEOUT", "0ms"},
		{"CATALOGO_DEFAULT_PAGE_SIZE", "0"},
		{"CATALOGO_MAX_PAGE_SIZE", "-1"},
		{"CATALOGO_MAX_SEARCH_LENGTH", "cien"},
		{"CATALOGO_MAX_BATCH_PRODUCTOS", "0"},
		{"CATALOGO_DEFAULT_PAGE_SIZE", "101"}, // supera CATALOGO_MAX_PAGE_SIZE por defecto
	} {
		prueba.Run(caso.variable+"="+caso.valor, func(subprueba *testing.T) {
			prepararEntornoMinimo(subprueba)
			subprueba.Setenv(caso.variable, caso.valor)
			_, errorCarga := config.Load()
			if errorCarga == nil {
				subprueba.Fatal("se aceptó un valor inválido")
			}
			if !strings.Contains(errorCarga.Error(), caso.variable) {
				subprueba.Fatalf("el error no nombra la variable: %v", errorCarga)
			}
		})
	}
}

func TestCORSPrefiereLaVariableDelServicio(prueba *testing.T) {
	prepararEntornoMinimo(prueba)
	prueba.Setenv("CORS_ALLOWED_ORIGINS", "https://generica.example")
	configuracion, errorCarga := config.Load()
	if errorCarga != nil || len(configuracion.CORSAllowedOrigins) != 1 || configuracion.CORSAllowedOrigins[0] != "https://generica.example" {
		prueba.Fatalf("sin CATALOGO_CORS_ALLOWED_ORIGINS debe usarse CORS_ALLOWED_ORIGINS: %v %v", configuracion.CORSAllowedOrigins, errorCarga)
	}

	prueba.Setenv("CATALOGO_CORS_ALLOWED_ORIGINS", "https://catalogo.example, http://localhost:5173")
	configuracion, errorCarga = config.Load()
	if errorCarga != nil || len(configuracion.CORSAllowedOrigins) != 2 || configuracion.CORSAllowedOrigins[0] != "https://catalogo.example" {
		prueba.Fatalf("CATALOGO_CORS_ALLOWED_ORIGINS debe tener prioridad: %v %v", configuracion.CORSAllowedOrigins, errorCarga)
	}

	prueba.Setenv("CATALOGO_CORS_ALLOWED_ORIGINS", "https://*.example")
	if _, errorCarga = config.Load(); errorCarga == nil {
		prueba.Fatal("se aceptó un comodín en CATALOGO_CORS_ALLOWED_ORIGINS")
	}
}

func TestProductoServiceRespetaLimitesConfigurados(prueba *testing.T) {
	repositorioDoble := &mockProductoRepository{}
	servicio := services.NewProductoService(repositorioDoble, services.LimitesConsulta{
		TamanoPaginaPredeterminado: 7, TamanoPaginaMaximo: 15, LargoMaximoBusqueda: 5,
	})
	contexto := prueba.Context()

	if _, errorObtenido := servicio.ObtenerCatalogo(contexto, domainFiltro("", 0)); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if repositorioDoble.lastFilter.Limit != 7 {
		prueba.Errorf("sin limit se esperaba 7, se obtuvo %d", repositorioDoble.lastFilter.Limit)
	}

	if _, errorObtenido := servicio.ObtenerCatalogoAdmin(contexto, domainFiltro("", 500)); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if repositorioDoble.lastFilter.Limit != 15 {
		prueba.Errorf("limit 500 debía acotarse a 15, se obtuvo %d", repositorioDoble.lastFilter.Limit)
	}

	if _, errorObtenido := servicio.ObtenerCatalogo(contexto, domainFiltro("lechesdescremadas", 0)); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if repositorioDoble.lastFilter.Query != "leche" {
		prueba.Errorf("la búsqueda debía cortarse a 5 bytes, se obtuvo %q", repositorioDoble.lastFilter.Query)
	}
}

// domainFiltro arma un filtro con búsqueda y limit.
func domainFiltro(busqueda string, limite int) domain.FiltroProductosDTO {
	return domain.FiltroProductosDTO{Query: busqueda, Limit: limite}
}
