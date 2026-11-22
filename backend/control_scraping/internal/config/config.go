// Package config lee y valida una sola vez el entorno de Control de scraping.
// Ningún otro paquete consulta variables de entorno: reciben la configuración
// inyectada al arrancar. Un valor inválido impide el arranque con un mensaje
// que nunca reproduce secretos ni DSN.
package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// ===== Valores por defecto y límites =====

const (
	// rolBaseDatos es el único rol de runtime aceptado: sin fallback al administrador.
	rolBaseDatos = "app_scraping"
	// puertoPredeterminado coincide con Compose, K8s y el Dockerfile.
	puertoPredeterminado = 8080
	puertoMaximo         = 65535
	// LongitudMinimaToken aplica a los dos secretos M2M del servicio.
	LongitudMinimaToken = 32
	// plazoCatalogoPredeterminado y plazoCatalogoMaximo acotan
	// CATALOGO_INGEST_TIMEOUT; la lease de una entrega cubre tres llamadas al máximo.
	plazoCatalogoPredeterminado = 10 * time.Second
	PlazoCatalogoMaximo         = 20 * time.Second
	// caracteresEspacio no se admiten en tokens ni en el origen de Catálogo.
	caracteresEspacio = " \t\r\n"
)

// ===== Configuración del servicio =====

// Configuracion es la instantánea validada del entorno.
type Configuracion struct {
	DireccionEscucha   string
	URLBaseDatos       string
	URLRedis           string
	URLCatalogo        string
	TokenCatalogo      string
	TokenServicio      string
	PlazoCatalogo      time.Duration
	OrigenesPermitidos []string
	SwaggerHabilitado  bool
}

// Cargar lee y valida todas las variables del servicio.
func Cargar() (Configuracion, error) {
	configuracion := Configuracion{
		URLBaseDatos:  os.Getenv("SCRAPING_DB_URL"),
		URLRedis:      os.Getenv("REDIS_URL"),
		URLCatalogo:   os.Getenv("CATALOGO_INTERNAL_URL"),
		TokenCatalogo: os.Getenv("CATALOGO_INTERNAL_TOKEN"),
		TokenServicio: os.Getenv("SCRAPING_SERVICE_TOKEN"),
		PlazoCatalogo: plazoCatalogoPredeterminado,
	}
	if errorObtenido := validarBaseDatos(configuracion.URLBaseDatos); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if configuracion.URLRedis == "" {
		return Configuracion{}, errors.New("REDIS_URL es obligatorio")
	}
	if _, errorObtenido := redis.ParseURL(configuracion.URLRedis); errorObtenido != nil {
		return Configuracion{}, errors.New("REDIS_URL debe ser una URL Redis válida")
	}
	if errorObtenido := validarTokens(configuracion.TokenServicio, configuracion.TokenCatalogo); errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	if !esOrigenCatalogoValido(configuracion.URLCatalogo) {
		return Configuracion{}, errors.New("CATALOGO_INTERNAL_URL debe ser un origen HTTP(S), sin credenciales, rutas ni parámetros")
	}
	configuracion.URLCatalogo = strings.TrimSuffix(configuracion.URLCatalogo, "/")
	if texto := os.Getenv("CATALOGO_INGEST_TIMEOUT"); texto != "" {
		plazo, errorObtenido := time.ParseDuration(texto)
		if errorObtenido != nil {
			return Configuracion{}, errors.New("CATALOGO_INGEST_TIMEOUT debe ser una duración válida")
		}
		configuracion.PlazoCatalogo = plazo
	}
	if configuracion.PlazoCatalogo <= 0 || configuracion.PlazoCatalogo > PlazoCatalogoMaximo {
		return Configuracion{}, errors.New("CATALOGO_INGEST_TIMEOUT debe ser positivo y no superar " + PlazoCatalogoMaximo.String())
	}
	origenes, errorObtenido := leerOrigenes()
	if errorObtenido != nil {
		return Configuracion{}, errorObtenido
	}
	configuracion.OrigenesPermitidos = origenes
	switch os.Getenv("SCRAPING_SWAGGER_ENABLED") {
	case "true":
		configuracion.SwaggerHabilitado = true
	case "", "false":
	default:
		return Configuracion{}, errors.New("SCRAPING_SWAGGER_ENABLED debe ser true o false")
	}
	puerto := puertoPredeterminado
	if texto := os.Getenv("PORT"); texto != "" {
		puerto, errorObtenido = strconv.Atoi(texto)
		if errorObtenido != nil || puerto < 1 || puerto > puertoMaximo {
			return Configuracion{}, errors.New("PORT debe estar entre 1 y " + strconv.Itoa(puertoMaximo))
		}
	}
	configuracion.DireccionEscucha = ":" + strconv.Itoa(puerto)
	return configuracion, nil
}

// ===== Validaciones =====

// validarBaseDatos exige el rol de runtime, una base y una contraseña explícitos.
func validarBaseDatos(cadenaConexion string) error {
	if cadenaConexion == "" {
		return errors.New("SCRAPING_DB_URL es obligatorio")
	}
	analizada, errorObtenido := pgx.ParseConfig(cadenaConexion)
	if errorObtenido != nil || analizada.User != rolBaseDatos || analizada.Database == "" || analizada.Password == "" {
		// No devolver el DSN ni el error del parser: pueden contener secretos.
		return errors.New("SCRAPING_DB_URL debe usar el rol " + rolBaseDatos + ", una base y una contraseña explícitos")
	}
	return nil
}

// validarTokens exige secretos largos, sin espacios y distintos entre sí: un
// worker con el token de servicio no debe poder hablar directo con Catálogo.
func validarTokens(tokenServicio, tokenCatalogo string) error {
	if len(tokenServicio) < LongitudMinimaToken || strings.ContainsAny(tokenServicio, caracteresEspacio) {
		return errors.New("SCRAPING_SERVICE_TOKEN debe tener al menos " + strconv.Itoa(LongitudMinimaToken) + " caracteres y no contener espacios")
	}
	if tokenServicio == tokenCatalogo {
		return errors.New("SCRAPING_SERVICE_TOKEN debe ser distinto de CATALOGO_INTERNAL_TOKEN")
	}
	if len(tokenCatalogo) < LongitudMinimaToken || strings.ContainsAny(tokenCatalogo, caracteresEspacio) {
		return errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos " + strconv.Itoa(LongitudMinimaToken) + " caracteres y no contener espacios")
	}
	return nil
}

// esOrigenCatalogoValido acepta sólo un origen HTTP(S) sin credenciales, ruta
// ni parámetros, para que el token interno no viaje a otro destino.
func esOrigenCatalogoValido(origen string) bool {
	direccion, errorObtenido := url.Parse(origen)
	return errorObtenido == nil && (direccion.Scheme == "http" || direccion.Scheme == "https") && direccion.Hostname() != "" &&
		direccion.User == nil && (direccion.Path == "" || direccion.Path == "/") && direccion.RawQuery == "" && !direccion.ForceQuery &&
		direccion.Fragment == "" && !strings.ContainsAny(origen, "?#\\"+caracteresEspacio)
}

// leerOrigenes lee SCRAPING_CORS_ALLOWED_ORIGINS y, si no está definida, el
// fallback CORS_ALLOWED_ORIGINS de GUIA_CONFIGURACION. Las entradas vacías se
// ignoran; cualquier otra debe ser un origen HTTP(S) exacto.
func leerOrigenes() ([]string, error) {
	texto, definida := os.LookupEnv("SCRAPING_CORS_ALLOWED_ORIGINS")
	if !definida {
		texto = os.Getenv("CORS_ALLOWED_ORIGINS")
	}
	var origenes []string
	for _, entrada := range strings.Split(texto, ",") {
		origen := strings.TrimSpace(entrada)
		if origen == "" {
			continue
		}
		direccion, errorObtenido := url.Parse(origen)
		if errorObtenido != nil || direccion.Hostname() == "" || (direccion.Scheme != "http" && direccion.Scheme != "https") || direccion.User != nil ||
			direccion.Path != "" || direccion.RawQuery != "" || direccion.ForceQuery || direccion.Fragment != "" {
			return nil, errors.New("SCRAPING_CORS_ALLOWED_ORIGINS debe contener orígenes HTTP(S) exactos")
		}
		origenes = append(origenes, origen)
	}
	return origenes, nil
}
