// Package config lee y valida una sola vez el entorno de Catálogo, incluidos
// sus plazos y límites. Si un valor falta o es inválido, Cargar devuelve un error y
// el servicio no inicia: nunca se inventa un valor para seguir funcionando.
// Ningún otro paquete consulta variables de entorno.
package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ===== Valores por defecto de plazos y límites =====

// Valores por defecto de las variables CATALOGO_* de ajuste. La guía de
// barrido (sección 3) exige que plazos y límites salgan de configuración; se
// exportan para que las pruebas compongan el router con los mismos valores.
const (
	PlazoCabecerasPredeterminado   = 5 * time.Second  // CATALOGO_READ_HEADER_TIMEOUT
	PlazoLecturaPredeterminado     = 30 * time.Second // CATALOGO_READ_TIMEOUT
	PlazoEscrituraPredeterminado   = 30 * time.Second // CATALOGO_WRITE_TIMEOUT
	PlazoInactividadPredeterminado = 60 * time.Second // CATALOGO_IDLE_TIMEOUT
	PlazoApagadoPredeterminado     = 15 * time.Second // CATALOGO_SHUTDOWN_TIMEOUT
	PlazoInicioPredeterminado      = 5 * time.Second  // CATALOGO_STARTUP_TIMEOUT
	PlazoReadinessPredeterminado   = 2 * time.Second  // CATALOGO_READINESS_TIMEOUT

	TamanoPaginaPredeterminado       = 20   // CATALOGO_DEFAULT_PAGE_SIZE
	TamanoPaginaMaximoPredeterminado = 100  // CATALOGO_MAX_PAGE_SIZE
	LargoBusquedaPredeterminado      = 100  // CATALOGO_MAX_SEARCH_LENGTH, en bytes
	ProductosPorLotePredeterminados  = 1000 // CATALOGO_MAX_BATCH_PRODUCTOS
	// RadioMaximoKmPredeterminado acota la consulta geoespacial de SUP-317.
	RadioMaximoKmPredeterminado = 50 // CATALOGO_MAX_RADIO_KM
	// BytesCuerpoPredeterminados (8 MiB, CATALOGO_MAX_BODY_BYTES) cubre un lote
	// completo con holgura y corta antes los cuerpos que agotarían la memoria.
	BytesCuerpoPredeterminados = 8 << 20
)

// Límites de validación del entorno.
const (
	puertoPredeterminado    = "8080"
	puertoMinimo            = 1
	puertoMaximo            = 65535
	largoMinimoSecreto      = 32 // bytes mínimos de JWT_SECRET y de los tokens internos
	caracteresNoPermitidos  = " \t\r\n"
	caracteresOrigenVetados = "*?#\\ \t\r\n"
)

// Valores por defecto del pool y de la consulta de disponibilidad.
const (
	conexionesAbiertasPredeterminadas  = 10
	conexionesInactivasPredeterminadas = 5
	vidaConexionPredeterminada         = 30 * time.Minute
	productosConsultaPredeterminados   = 1000
	antiguedadPredeterminada           = 48 * time.Hour
)

// ===== Configuración del servicio =====

// Configuracion es la instantánea validada del entorno que el arranque inyecta
// en cada componente.
type Configuracion struct {
	DireccionEscucha string
	URLBaseDatos     string
	ClaveJWT         string
	// TokenIngesta habilita la escritura interna; vacío deja Catálogo en lectura.
	TokenIngesta string
	// TokenConsulta habilita la consulta de disponibilidad (sólo lectura).
	TokenConsulta      string
	OrigenesPermitidos []string

	MaximoConexionesAbiertas  int
	MaximoConexionesInactivas int
	VidaMaximaConexion        time.Duration

	// Plazos del servidor HTTP y del ciclo de vida del proceso.
	PlazoCabeceras   time.Duration
	PlazoLectura     time.Duration
	PlazoEscritura   time.Duration
	PlazoInactividad time.Duration
	PlazoApagado     time.Duration
	PlazoInicio      time.Duration // verificación de tablas y permisos al arrancar
	PlazoReadiness   time.Duration // ping a PostgreSQL de /_catalogo/ready

	// Límites de las consultas públicas y de la ingesta.
	TamanoPagina           int // limit cuando el cliente no lo envía
	TamanoPaginaMaximo     int // tope de limit en listados públicos y de administración
	LargoMaximoBusqueda    int // bytes conservados de cada texto de búsqueda
	MaximoProductosPorLote int // productos aceptados por lote de ingesta
	MaximoBytesCuerpo      int // bytes aceptados en el cuerpo JSON interno

	// Límites de la consulta interna de disponibilidad que usan Listas e IA.
	MaximoProductosConsulta        int
	AntiguedadMaximaDisponibilidad time.Duration
	// RadioMaximoKm acota la consulta de precios comparables por sucursal.
	RadioMaximoKm int
}

// Cargar lee y valida el entorno del proceso. Los mensajes de error nombran la
// variable, nunca reproducen su valor.
func Cargar() (Configuracion, error) {
	configuracion := Configuracion{
		URLBaseDatos:  os.Getenv("DB_URL"),
		ClaveJWT:      os.Getenv("JWT_SECRET"),
		TokenIngesta:  os.Getenv("CATALOGO_INTERNAL_TOKEN"),
		TokenConsulta: os.Getenv("CATALOGO_CONSULTA_TOKEN"),
	}
	if configuracion.URLBaseDatos == "" {
		return Configuracion{}, errors.New("DB_URL es obligatorio")
	}
	if len(configuracion.ClaveJWT) < largoMinimoSecreto || strings.TrimSpace(configuracion.ClaveJWT) != configuracion.ClaveJWT {
		return Configuracion{}, errors.New("JWT_SECRET requiere al menos " + strconv.Itoa(largoMinimoSecreto) + " bytes y no admite espacios externos")
	}
	if !tokenValido(configuracion.TokenIngesta, configuracion.ClaveJWT) {
		return Configuracion{}, errors.New("CATALOGO_INTERNAL_TOKEN debe tener al menos " + strconv.Itoa(largoMinimoSecreto) + " caracteres sin espacios y ser distinto de JWT_SECRET")
	}
	if !tokenValido(configuracion.TokenConsulta, configuracion.ClaveJWT, configuracion.TokenIngesta) {
		return Configuracion{}, errors.New("CATALOGO_CONSULTA_TOKEN debe tener al menos " + strconv.Itoa(largoMinimoSecreto) + " caracteres sin espacios y ser distinto de JWT_SECRET y CATALOGO_INTERNAL_TOKEN")
	}

	// La variable propia del servicio tiene prioridad; la genérica es respaldo.
	origenesCrudos := strings.TrimSpace(os.Getenv("CATALOGO_CORS_ALLOWED_ORIGINS"))
	if origenesCrudos == "" {
		origenesCrudos = os.Getenv("CORS_ALLOWED_ORIGINS")
	}
	origenes, errorOrigenes := leerOrigenes(origenesCrudos)
	if errorOrigenes != nil {
		return Configuracion{}, errorOrigenes
	}
	configuracion.OrigenesPermitidos = origenes

	direccion, errorPuerto := leerPuerto(os.Getenv("PORT"))
	if errorPuerto != nil {
		return Configuracion{}, errorPuerto
	}
	configuracion.DireccionEscucha = direccion

	for _, variable := range []struct {
		nombre       string
		destino      *int
		predetermina int
	}{
		{"DB_MAX_OPEN_CONNS", &configuracion.MaximoConexionesAbiertas, conexionesAbiertasPredeterminadas},
		{"DB_MAX_IDLE_CONNS", &configuracion.MaximoConexionesInactivas, conexionesInactivasPredeterminadas},
		{"CATALOGO_MAX_PRODUCTOS_CONSULTA", &configuracion.MaximoProductosConsulta, productosConsultaPredeterminados},
		{"CATALOGO_DEFAULT_PAGE_SIZE", &configuracion.TamanoPagina, TamanoPaginaPredeterminado},
		{"CATALOGO_MAX_PAGE_SIZE", &configuracion.TamanoPaginaMaximo, TamanoPaginaMaximoPredeterminado},
		{"CATALOGO_MAX_SEARCH_LENGTH", &configuracion.LargoMaximoBusqueda, LargoBusquedaPredeterminado},
		{"CATALOGO_MAX_BATCH_PRODUCTOS", &configuracion.MaximoProductosPorLote, ProductosPorLotePredeterminados},
		{"CATALOGO_MAX_BODY_BYTES", &configuracion.MaximoBytesCuerpo, BytesCuerpoPredeterminados},
		{"CATALOGO_MAX_RADIO_KM", &configuracion.RadioMaximoKm, RadioMaximoKmPredeterminado},
	} {
		valor, errorEntero := leerEnteroPositivo(variable.nombre, variable.predetermina)
		if errorEntero != nil {
			return Configuracion{}, errorEntero
		}
		*variable.destino = valor
	}
	if configuracion.MaximoConexionesInactivas > configuracion.MaximoConexionesAbiertas {
		return Configuracion{}, errors.New("DB_MAX_IDLE_CONNS no puede superar DB_MAX_OPEN_CONNS")
	}
	if configuracion.TamanoPagina > configuracion.TamanoPaginaMaximo {
		return Configuracion{}, errors.New("CATALOGO_DEFAULT_PAGE_SIZE no puede superar CATALOGO_MAX_PAGE_SIZE")
	}

	for _, variable := range []struct {
		nombre       string
		destino      *time.Duration
		predetermina time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", &configuracion.VidaMaximaConexion, vidaConexionPredeterminada},
		{"CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA", &configuracion.AntiguedadMaximaDisponibilidad, antiguedadPredeterminada},
		{"CATALOGO_READ_HEADER_TIMEOUT", &configuracion.PlazoCabeceras, PlazoCabecerasPredeterminado},
		{"CATALOGO_READ_TIMEOUT", &configuracion.PlazoLectura, PlazoLecturaPredeterminado},
		{"CATALOGO_WRITE_TIMEOUT", &configuracion.PlazoEscritura, PlazoEscrituraPredeterminado},
		{"CATALOGO_IDLE_TIMEOUT", &configuracion.PlazoInactividad, PlazoInactividadPredeterminado},
		{"CATALOGO_SHUTDOWN_TIMEOUT", &configuracion.PlazoApagado, PlazoApagadoPredeterminado},
		{"CATALOGO_STARTUP_TIMEOUT", &configuracion.PlazoInicio, PlazoInicioPredeterminado},
		{"CATALOGO_READINESS_TIMEOUT", &configuracion.PlazoReadiness, PlazoReadinessPredeterminado},
	} {
		valor, errorDuracion := leerDuracionPositiva(variable.nombre, variable.predetermina)
		if errorDuracion != nil {
			return Configuracion{}, errorDuracion
		}
		*variable.destino = valor
	}
	return configuracion, nil
}

// ===== Lectores de variables =====

// tokenValido acepta un token vacío (función deshabilitada) o uno largo, sin
// espacios y distinto de los demás secretos, para que la filtración de uno no
// abra las otras rutas.
func tokenValido(token string, otrosSecretos ...string) bool {
	if token == "" {
		return true
	}
	if len(token) < largoMinimoSecreto || strings.ContainsAny(token, caracteresNoPermitidos) {
		return false
	}
	for _, otro := range otrosSecretos {
		if token == otro {
			return false
		}
	}
	return true
}

// leerOrigenes valida la lista CORS: sólo orígenes HTTP(S) exactos. Sin
// orígenes, el router rechaza toda petición con cabecera Origin.
func leerOrigenes(texto string) ([]string, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return nil, nil
	}
	var origenes []string
	for _, entrada := range strings.Split(texto, ",") {
		origen := strings.TrimSpace(entrada)
		origenParseado, errorParseo := url.Parse(origen)
		if errorParseo != nil || (origenParseado.Scheme != "http" && origenParseado.Scheme != "https") || origenParseado.Hostname() == "" ||
			origenParseado.User != nil || origenParseado.Path != "" || origenParseado.RawQuery != "" || origenParseado.ForceQuery ||
			origenParseado.Fragment != "" || strings.ContainsAny(origen, caracteresOrigenVetados) {
			return nil, errors.New("CATALOGO_CORS_ALLOWED_ORIGINS (o CORS_ALLOWED_ORIGINS) debe contener orígenes HTTP(S) exactos, sin rutas ni comodines")
		}
		origenes = append(origenes, origen)
	}
	return origenes, nil
}

// leerPuerto convierte PORT en la dirección de escucha.
func leerPuerto(puerto string) (string, error) {
	if puerto == "" {
		puerto = puertoPredeterminado
	}
	numeroPuerto, errorConversion := strconv.Atoi(puerto)
	if errorConversion != nil || numeroPuerto < puertoMinimo || numeroPuerto > puertoMaximo {
		return "", errors.New("PORT debe ser un número entre " + strconv.Itoa(puertoMinimo) + " y " + strconv.Itoa(puertoMaximo))
	}
	return ":" + strconv.Itoa(numeroPuerto), nil
}

// leerEnteroPositivo devuelve el valor de la variable o su valor por defecto.
func leerEnteroPositivo(nombre string, predeterminado int) (int, error) {
	texto := os.Getenv(nombre)
	if texto == "" {
		return predeterminado, nil
	}
	valor, errorConversion := strconv.Atoi(texto)
	if errorConversion != nil || valor < 1 {
		return 0, errors.New(nombre + " debe ser un entero positivo")
	}
	return valor, nil
}

// leerDuracionPositiva devuelve la duración de la variable o su valor por defecto.
func leerDuracionPositiva(nombre string, predeterminada time.Duration) (time.Duration, error) {
	texto := os.Getenv(nombre)
	if texto == "" {
		return predeterminada, nil
	}
	duracion, errorConversion := time.ParseDuration(texto)
	if errorConversion != nil || duracion <= 0 {
		return 0, errors.New(nombre + " debe ser una duración positiva, por ejemplo 5s")
	}
	return duracion, nil
}
