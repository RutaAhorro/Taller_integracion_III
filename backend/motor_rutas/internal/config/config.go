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
package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// =============================================================================
// Modelo
// =============================================================================

// Config es la configuración ya validada del servicio.
type Config struct {
	Puerto       string   // puerto HTTP, como texto ("8001")
	DBURL        string   // DSN completo de PostgreSQL (contiene la contraseña: no registrar)
	JWTSecret    string   // secreto para verificar la firma de la cookie jwt
	OrigenesCORS []string // orígenes exactos que pueden llamar con cookies
	DBPoolMax    int32    // conexiones máximas abiertas a la base
	MaxListas    int      // tope de listas por usuario
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
	secretoCorto := len(configuracion.JWTSecret) < 32
	secretoConEspacios := strings.TrimSpace(configuracion.JWTSecret) != configuracion.JWTSecret
	if secretoCorto || secretoConEspacios {
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
	configuracion.MaxListas, errorObtenido = entero(leerVariable("RUTAS_MAX_LISTAS", "100"), 1, 1000)
	if errorObtenido != nil {
		return Config{}, errors.New("RUTAS_MAX_LISTAS debe estar entre 1 y 1000")
	}

	return configuracion, nil
}

// =============================================================================
// Validaciones auxiliares
// =============================================================================

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
