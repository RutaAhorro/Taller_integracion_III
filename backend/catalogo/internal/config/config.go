// Package config lee y valida la configuración de Catálogo una sola vez al
// arrancar. Si un valor falta o es inválido, Load devuelve un error y el
// servicio no inicia: nunca se inventa un valor para seguir funcionando.
package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne todo lo que el servicio necesita del entorno.
type Config struct {
	ListenAddr         string
	DatabaseURL        string
	JWTSecret          string
	InternalToken      string
	CORSAllowedOrigins []string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration

	// Plazos del servidor HTTP y del ciclo de vida del proceso.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	StartupTimeout    time.Duration // verificación de tablas y permisos al arrancar
	ReadinessTimeout  time.Duration // ping a PostgreSQL de /_catalogo/ready

	// Límites de las consultas públicas y de la ingesta.
	DefaultPageSize   int // tamaño de página cuando el cliente no envía limit
	MaxPageSize       int // tope de limit en listados públicos y de administración
	MaxSearchLength   int // caracteres conservados del texto de búsqueda
	MaxBatchProductos int // productos aceptados por lote de ingesta
}

// duracionConfigurable describe una variable de entorno de tipo duración.
type duracionConfigurable struct {
	nombre       string
	destino      *time.Duration
	predetermina time.Duration
}

// enteroConfigurable describe una variable de entorno de tipo entero positivo.
type enteroConfigurable struct {
	nombre       string
	destino      *int
	predetermina int
}

// Load lee la configuración desde las variables de entorno del proceso.
func Load() (Config, error) {
	configuracion := Config{
		DatabaseURL:   os.Getenv("DB_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		InternalToken: os.Getenv("CATALOGO_INTERNAL_TOKEN"),
	}
	if configuracion.DatabaseURL == "" {
		return Config{}, errors.New("DB_URL es obligatorio")
	}
	if configuracion.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET es obligatorio para verificar permisos administrativos")
	}
	tokenInterno := configuracion.InternalToken
	if tokenInterno != "" && (len(tokenInterno) < 32 || strings.ContainsAny(tokenInterno, " \t\r\n") || tokenInterno == configuracion.JWTSecret) {
		return Config{}, errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos 32 caracteres sin espacios y ser distinto de JWT_SECRET")
	}

	// --- CORS: primero la variable propia del servicio, luego la genérica ---
	origenesCrudos := strings.TrimSpace(os.Getenv("CATALOGO_CORS_ALLOWED_ORIGINS"))
	if origenesCrudos == "" {
		origenesCrudos = strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	}
	if origenesCrudos != "" {
		for _, entrada := range strings.Split(origenesCrudos, ",") {
			origen := strings.TrimSpace(entrada)
			origenParseado, errorParseo := url.Parse(origen)
			if errorParseo != nil || (origenParseado.Scheme != "http" && origenParseado.Scheme != "https") || origenParseado.Hostname() == "" ||
				origenParseado.User != nil || origenParseado.Path != "" || origenParseado.RawQuery != "" || origenParseado.ForceQuery ||
				origenParseado.Fragment != "" || strings.ContainsAny(origen, "*?#\\ \t\r\n") {
				return Config{}, errors.New("CATALOGO_CORS_ALLOWED_ORIGINS (o CORS_ALLOWED_ORIGINS) debe contener orígenes HTTP(S) exactos, sin rutas ni comodines")
			}
			configuracion.CORSAllowedOrigins = append(configuracion.CORSAllowedOrigins, origen)
		}
	}

	// --- Puerto de escucha ---
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}
	numeroPuerto, errorPuerto := strconv.Atoi(puerto)
	if errorPuerto != nil || numeroPuerto < 1 || numeroPuerto > 65535 {
		return Config{}, errors.New("PORT debe ser un número entre 1 y 65535")
	}
	configuracion.ListenAddr = ":" + strconv.Itoa(numeroPuerto)

	// --- Enteros positivos ---
	for _, variable := range []enteroConfigurable{
		{"DB_MAX_OPEN_CONNS", &configuracion.MaxOpenConns, 10},
		{"DB_MAX_IDLE_CONNS", &configuracion.MaxIdleConns, 5},
		{"CATALOGO_DEFAULT_PAGE_SIZE", &configuracion.DefaultPageSize, 20},
		{"CATALOGO_MAX_PAGE_SIZE", &configuracion.MaxPageSize, 100},
		{"CATALOGO_MAX_SEARCH_LENGTH", &configuracion.MaxSearchLength, 100},
		{"CATALOGO_MAX_BATCH_PRODUCTOS", &configuracion.MaxBatchProductos, 1000},
	} {
		*variable.destino = variable.predetermina
		if valorCrudo := os.Getenv(variable.nombre); valorCrudo != "" {
			valor, errorConversion := strconv.Atoi(valorCrudo)
			if errorConversion != nil || valor < 1 {
				return Config{}, errors.New(variable.nombre + " debe ser un entero positivo")
			}
			*variable.destino = valor
		}
	}
	if configuracion.MaxIdleConns > configuracion.MaxOpenConns {
		return Config{}, errors.New("DB_MAX_IDLE_CONNS no puede superar DB_MAX_OPEN_CONNS")
	}
	if configuracion.DefaultPageSize > configuracion.MaxPageSize {
		return Config{}, errors.New("CATALOGO_DEFAULT_PAGE_SIZE no puede superar CATALOGO_MAX_PAGE_SIZE")
	}

	// --- Duraciones positivas ---
	for _, variable := range []duracionConfigurable{
		{"DB_CONN_MAX_LIFETIME", &configuracion.ConnMaxLifetime, 30 * time.Minute},
		{"CATALOGO_READ_HEADER_TIMEOUT", &configuracion.ReadHeaderTimeout, 5 * time.Second},
		{"CATALOGO_READ_TIMEOUT", &configuracion.ReadTimeout, 30 * time.Second},
		{"CATALOGO_WRITE_TIMEOUT", &configuracion.WriteTimeout, 30 * time.Second},
		{"CATALOGO_IDLE_TIMEOUT", &configuracion.IdleTimeout, 60 * time.Second},
		{"CATALOGO_SHUTDOWN_TIMEOUT", &configuracion.ShutdownTimeout, 15 * time.Second},
		{"CATALOGO_STARTUP_TIMEOUT", &configuracion.StartupTimeout, 5 * time.Second},
		{"CATALOGO_READINESS_TIMEOUT", &configuracion.ReadinessTimeout, 2 * time.Second},
	} {
		*variable.destino = variable.predetermina
		if valorCrudo := os.Getenv(variable.nombre); valorCrudo != "" {
			duracion, errorConversion := time.ParseDuration(valorCrudo)
			if errorConversion != nil || duracion <= 0 {
				return Config{}, errors.New(variable.nombre + " debe ser una duración positiva, por ejemplo 5s")
			}
			*variable.destino = duracion
		}
	}
	return configuracion, nil
}
