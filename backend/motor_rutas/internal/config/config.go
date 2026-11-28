// Package config lee y valida la configuración desde variables de entorno.
//
// Regla del paquete: si algo está mal configurado, el servicio NO arranca
// (es preferible caerse al inicio que servir sin aislamiento). Ningún mensaje
// de error incluye el DSN, contraseñas ni el secreto JWT.
//
// Variables que se leen:
//
//	PORT                  puerto HTTP (por defecto 8001)
//	RUTAS_DB_URL          DSN de PostgreSQL con el rol app_rutas (obligatoria)
//	JWT_SECRET            secreto HS256 compartido con Identidad (obligatoria, ≥ 32 bytes)
//	CORS_ALLOWED_ORIGINS  orígenes permitidos, separados por coma
//	RUTAS_DB_POOL_MAX     conexiones máximas del pool, 1–20 (por defecto 5)
//	RUTAS_MAX_LISTAS      listas máximas por usuario, 1–1000 (por defecto 100)
//	CATALOGO_INTERNAL_URL origen interno de Catálogo, sin ruta (obligatoria)
//	CATALOGO_CONSULTA_TOKEN token de la consulta de disponibilidad (obligatorio, ≥ 32 bytes)
//	RUTAS_CATALOGO_TIMEOUT plazo por consulta a Catálogo (por defecto 5s, máximo 30s)
//	RUTAS_RADIO_KM        radio de búsqueda de sucursales en km, (0, 50] (por defecto 10)
//	RUTAS_MAX_SUCURSALES  máximo de sucursales candidatas, 1–10 (por defecto 10)
//	RUTAS_MAX_PARADAS     máximo de paradas en una ruta, 1–3 (por defecto 3)
//	RUTAS_MAX_ARTICULOS   máximo de artículos pendientes por cálculo, 1–100 (por defecto 30)
//	RUTAS_FACTOR_DISTANCIA factor de corrección sobre Haversine, (0, 5] (por defecto 1.3)
//	RUTAS_UMBRAL_AHORRO   ahorro mínimo en CLP para recomendar varias tiendas (por defecto 500)
//	RUTAS_ORS_URL         origen ORS opcional, solo HTTP(S), sin rutas ni credenciales
//	RUTAS_ORS_API_KEY     clave ORS solo para backend, nunca registrar
//
// Los topes de artículos de listas no son variables: son constantes de internal/listas
// (ver MaxArticulosPorLista y MaxEnvasesPorArticulo).
package config

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// Límites de las variables
// =============================================================================

const (
	largoMinimoSecreto    = 32               // bytes mínimos de JWT_SECRET y CATALOGO_CONSULTA_TOKEN
	plazoMaximoCatalogo   = 30 * time.Second // RUTAS_CATALOGO_TIMEOUT más largo aceptado
	maximoTopes           = 1000             // RUTAS_MAX_LISTAS más alto aceptado
	radioMaximoKm         = 50.0             // tope alineado con CATALOGO_MAX_RADIO_KM por defecto
	maximoSucursalesTope  = 10               // tope de Catálogo precios-comparables
	maximoParadasTope     = 3                // límite práctico del algoritmo de fuerza bruta
	maximoArticulosRuta   = 100              // alineado con MaxArticulosPorLista
	factorDistanciaMaximo = 5.0              // factor máximo razonable sobre línea recta
	umbralAhorroMaximo    = 1000000          // cota defensiva en CLP
)

// =============================================================================
// Modelo
// =============================================================================

// Config es la configuración ya validada del servicio.
type Config struct {
	ORSURL       string   // RUTAS_ORS_URL: origen ORS opcional
	ORSAPIKey    string   // RUTAS_ORS_API_KEY: secreto, nunca registrar
	Puerto       string   // puerto HTTP, como texto ("8001")
	DBURL        string   // DSN completo de PostgreSQL (contiene la contraseña: no registrar)
	JWTSecret    string   // secreto para verificar la firma de la cookie jwt
	OrigenesCORS []string // orígenes exactos que pueden llamar con cookies
	DBPoolMax    int32    // conexiones máximas abiertas a la base
	MaxListas    int      // tope de listas por usuario

	CatalogoURL   string        // origen interno de Catálogo ("http://catalogo:8080")
	CatalogoToken string        // Bearer de la consulta de disponibilidad (no registrar)
	CatalogoPlazo time.Duration // timeout de cada consulta a Catálogo

	RadioKm         float64 // RUTAS_RADIO_KM: radio de búsqueda en km
	MaxSucursales   int     // RUTAS_MAX_SUCURSALES: sucursales candidatas pedidas a Catálogo
	MaxParadas      int     // RUTAS_MAX_PARADAS: paradas máximas en la ruta
	MaxArticulos    int     // RUTAS_MAX_ARTICULOS: artículos pendientes máximos por cálculo
	FactorDistancia float64 // RUTAS_FACTOR_DISTANCIA: corrección sobre Haversine
	UmbralAhorro    int     // RUTAS_UMBRAL_AHORRO: ahorro mínimo en CLP (RN-08)
}

// =============================================================================
// Carga
// =============================================================================

// Cargar lee y valida todas las variables.
//
// buscarVariable recibe el nombre de una variable y devuelve (valor, existe).
// En producción se pasa os.LookupEnv; en las pruebas, un mapa falso.
func Cargar(buscarVariable func(string) (string, bool)) (Config, error) {
	// leerVariable devuelve el valor de la variable o, si no existe, el valor por defecto.
	leerVariable := func(clave, porDefecto string) string {
		if valor, existe := buscarVariable(clave); existe {
			return valor
		}
		return porDefecto
	}

	// --- PORT ----------------------------------------------------------------
	configuracion := Config{Puerto: leerVariable("PORT", "8001")}
	if !puertoValido(configuracion.Puerto) {
		return Config{}, errors.New("PORT debe estar entre 1 y 65535")
	}

	// --- RUTAS_DB_URL --------------------------------------------------------
	configuracion.DBURL = leerVariable("RUTAS_DB_URL", "")
	if !dsnValido(configuracion.DBURL) {
		return Config{}, errors.New("RUTAS_DB_URL requiere rol app_rutas, base y contraseña explícitos")
	}

	// --- JWT_SECRET ----------------------------------------------------------
	// Espacios al inicio o al final suelen ser un error de copiado del Secret.
	configuracion.JWTSecret = leerVariable("JWT_SECRET", "")
	if !secretoValido(configuracion.JWTSecret) {
		return Config{}, errors.New("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")
	}

	// --- CORS_ALLOWED_ORIGINS ------------------------------------------------
	for _, parte := range strings.Split(leerVariable("CORS_ALLOWED_ORIGINS", ""), ",") {
		origen := strings.TrimSpace(parte)
		if origen == "" {
			continue // tolera comas sobrantes: "a, b,"
		}
		if !origenExacto(origen) {
			return Config{}, errors.New("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
		}
		configuracion.OrigenesCORS = append(configuracion.OrigenesCORS, origen)
	}

	// --- RUTAS_DB_POOL_MAX ---------------------------------------------------
	maximoConexiones, errorObtenido := entero(leerVariable("RUTAS_DB_POOL_MAX", "5"), 1, 20)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_DB_POOL_MAX debe estar entre 1 y 20")
	}
	configuracion.DBPoolMax = int32(maximoConexiones)

	// --- RUTAS_MAX_LISTAS ----------------------------------------------------
	// Tope defensivo por usuario: el servicio no tiene un límite de peticiones propio.
	configuracion.MaxListas, errorObtenido = entero(leerVariable("RUTAS_MAX_LISTAS", "100"), 1, maximoTopes)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_MAX_LISTAS debe estar entre 1 y 1000")
	}

	// --- Catálogo (SUP-241) --------------------------------------------------
	// Obligatorio: sin Catálogo, Motor no puede verificar productos y agregar
	// artículos fallaría en cada petición. Mejor no arrancar.
	configuracion.CatalogoURL = strings.TrimSuffix(leerVariable("CATALOGO_INTERNAL_URL", ""), "/")
	if !origenExacto(configuracion.CatalogoURL) {
		return Config{}, errors.New("CATALOGO_INTERNAL_URL requiere un origen HTTP(S) exacto, sin ruta ni credenciales")
	}
	configuracion.CatalogoToken = leerVariable("CATALOGO_CONSULTA_TOKEN", "")
	if !secretoValido(configuracion.CatalogoToken) {
		return Config{}, errors.New("CATALOGO_CONSULTA_TOKEN requiere al menos 32 bytes y no admite espacios externos")
	}
	if configuracion.CatalogoToken == configuracion.JWTSecret {
		return Config{}, errors.New("CATALOGO_CONSULTA_TOKEN debe ser distinto de JWT_SECRET")
	}
	configuracion.CatalogoPlazo, errorObtenido = time.ParseDuration(leerVariable("RUTAS_CATALOGO_TIMEOUT", "5s"))
	if errorObtenido != nil || configuracion.CatalogoPlazo <= 0 || configuracion.CatalogoPlazo > plazoMaximoCatalogo {
		return Config{}, errors.New("RUTAS_CATALOGO_TIMEOUT debe ser una duración positiva de hasta 30s")
	}

	// --- Optimización de rutas (MVP) -----------------------------------------
	configuracion.ORSURL = leerVariable("RUTAS_ORS_URL", "")
	if configuracion.ORSURL != "" && !origenExacto(configuracion.ORSURL) {
		return Config{}, errors.New("RUTAS_ORS_URL requiere un origen HTTP(S) exacto")
	}
	configuracion.ORSAPIKey = leerVariable("RUTAS_ORS_API_KEY", "")
	if strings.ContainsAny(configuracion.ORSAPIKey, "\r\n") || strings.TrimSpace(configuracion.ORSAPIKey) != configuracion.ORSAPIKey {
		return Config{}, errors.New("RUTAS_ORS_API_KEY tiene un formato inválido")
	}
	configuracion.RadioKm, errorObtenido = decimalPositivo(leerVariable("RUTAS_RADIO_KM", "10"), radioMaximoKm)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_RADIO_KM debe ser un número positivo de hasta 50")
	}
	configuracion.MaxSucursales, errorObtenido = entero(leerVariable("RUTAS_MAX_SUCURSALES", "10"), 1, maximoSucursalesTope)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_MAX_SUCURSALES debe estar entre 1 y 10")
	}
	configuracion.MaxParadas, errorObtenido = entero(leerVariable("RUTAS_MAX_PARADAS", "3"), 1, maximoParadasTope)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_MAX_PARADAS debe estar entre 1 y 3")
	}
	configuracion.MaxArticulos, errorObtenido = entero(leerVariable("RUTAS_MAX_ARTICULOS", "30"), 1, maximoArticulosRuta)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_MAX_ARTICULOS debe estar entre 1 y 100")
	}
	configuracion.FactorDistancia, errorObtenido = decimalPositivo(leerVariable("RUTAS_FACTOR_DISTANCIA", "1.3"), factorDistanciaMaximo)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_FACTOR_DISTANCIA debe ser un número positivo de hasta 5")
	}
	configuracion.UmbralAhorro, errorObtenido = entero(leerVariable("RUTAS_UMBRAL_AHORRO", "500"), 0, umbralAhorroMaximo)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_UMBRAL_AHORRO debe estar entre 0 y 1000000")
	}

	return configuracion, nil
}

// =============================================================================
// Validaciones auxiliares
// =============================================================================

// decimalPositivo convierte texto a número real estricto (sin espacios) y exige
// que sea finito, mayor que 0 y menor o igual a maximo.
func decimalPositivo(crudo string, maximo float64) (float64, error) {
	if crudo == "" || strings.TrimSpace(crudo) != crudo {
		return 0, errors.New("formato decimal inválido")
	}
	numero, errorObtenido := strconv.ParseFloat(crudo, 64)
	if errorObtenido != nil || math.IsNaN(numero) || math.IsInf(numero, 0) || numero <= 0 || numero > maximo {
		return 0, errors.New("fuera de rango")
	}
	return numero, nil
}

// entero convierte texto a número exigiendo solo dígitos (rechaza "+5", "-1",
// " 5") y que el resultado esté entre minimo y maximo, ambos incluidos.
func entero(crudo string, minimo, maximo int) (int, error) {
	for _, caracter := range crudo {
		if caracter < '0' || caracter > '9' {
			return 0, errors.New("no es un entero")
		}
	}
	numero, errorObtenido := strconv.Atoi(crudo)
	if errorObtenido != nil || numero < minimo || numero > maximo {
		return 0, errors.New("fuera de rango")
	}
	return numero, nil
}

// secretoValido exige al menos largoMinimoSecreto bytes y ningún espacio en
// los extremos, que suele ser un error al copiar el Secret.
func secretoValido(secreto string) bool {
	return len(secreto) >= largoMinimoSecreto && strings.TrimSpace(secreto) == secreto
}

// puertoValido acepta un número de puerto TCP entre 1 y 65535.
func puertoValido(texto string) bool {
	numero, errorObtenido := strconv.Atoi(texto)
	return errorObtenido == nil && numero >= 1 && numero <= 65535
}

// dsnValido exige un DSN postgres:// con el rol app_rutas, contraseña, host y
// base explícitos. Así el servicio nunca arranca con un rol con más permisos.
func dsnValido(dsn string) bool {
	direccion, errorObtenido := url.Parse(dsn)
	if errorObtenido != nil {
		return false
	}
	esquemaPostgres := direccion.Scheme == "postgres" || direccion.Scheme == "postgresql"
	if !esquemaPostgres || direccion.User == nil {
		return false
	}
	if puerto := direccion.Port(); puerto != "" && !puertoValido(puerto) {
		return false
	}
	clave, tieneClave := direccion.User.Password()
	rolCorrecto := direccion.User.Username() == "app_rutas"
	tieneHost := direccion.Hostname() != ""
	tieneBase := strings.Trim(direccion.Path, "/") != ""
	return rolCorrecto && tieneClave && clave != "" && tieneHost && tieneBase
}

// origenExacto acepta solo "esquema://host[:puerto]": sin rutas, comodines,
// consultas ni credenciales. Un origen mal escrito abriría CORS de más.
func origenExacto(origen string) bool {
	if strings.ContainsAny(origen, "*?#\\ \t\r\n") {
		return false
	}
	direccion, errorObtenido := url.Parse(origen)
	if errorObtenido != nil {
		return false
	}
	esquemaWeb := direccion.Scheme == "http" || direccion.Scheme == "https"
	sinExtras := direccion.User == nil && direccion.Path == "" && direccion.RawQuery == "" &&
		direccion.Fragment == "" && direccion.Opaque == ""
	return esquemaWeb && direccion.Hostname() != "" && sinExtras
}
