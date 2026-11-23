// Package config lee y valida la configuración del Gateway desde el entorno.
//
// El Gateway sólo enruta: aquí no hay credenciales, base de datos ni Redis.
// Toda la validación ocurre al arrancar para que un valor mal escrito impida
// iniciar en vez de dejar rutas inalcanzables con el Gateway marcado como sano.
package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ===== Valores por defecto =====

const (
	// direccionEscuchaPorDefecto es el puerto del binario fuera de Compose/K8s,
	// que fijan GATEWAY_LISTEN_ADDR=:8080.
	direccionEscuchaPorDefecto = ":8082"
	puertoMinimo               = 1
	puertoMaximo               = 65535

	limitePeticionPorDefecto           = 30 * time.Second
	limiteConexionPorDefecto           = 5 * time.Second
	limiteCabecerasRespuestaPorDefecto = 10 * time.Second
	limiteLecturaCabecerasPorDefecto   = 5 * time.Second
	limiteInactividadPorDefecto        = 60 * time.Second
	limiteApagadoPorDefecto            = 10 * time.Second
	limiteDiagnosticoPorDefecto        = 2 * time.Second
	limiteKeepAlivePorDefecto          = 30 * time.Second
)

// ===== Variables de entorno =====

const (
	variableDireccionEscucha = "GATEWAY_LISTEN_ADDR"
	variableScraping         = "SCRAPING_URL"
	variableCatalogo         = "CATALOGO_URL"
	variableIdentidad        = "IDENTIDAD_URL"
	variableRutas            = "RUTAS_URL"
	variableIA               = "IA_URL"
	variableProxiesFiables   = "GATEWAY_TRUSTED_PROXIES"
)

// ===== Tipos =====

// Configuracion reúne los orígenes de cada dominio, los límites de tiempo y
// los prefijos de ruta. Un origen nil deja sus prefijos sin enrutar (404).
type Configuracion struct {
	DireccionEscucha string

	// Scraping es obligatorio: además de sus prefijos, Control es dueño de
	// /, /health y /api/v1/health.
	Scraping  *url.URL
	Catalogo  *url.URL
	Identidad *url.URL
	Rutas     *url.URL
	IA        *url.URL

	LimitePeticion           time.Duration
	LimiteConexion           time.Duration
	LimiteCabecerasRespuesta time.Duration
	LimiteLecturaCabeceras   time.Duration
	LimiteInactividad        time.Duration
	LimiteApagado            time.Duration
	LimiteDiagnostico        time.Duration
	LimiteKeepAlive          time.Duration

	ProxiesFiables []netip.Prefix

	PrefijosIdentidad []string
	PrefijosCatalogo  []string
	PrefijosRutas     []string
	PrefijosScraping  []string
	PrefijosIA        []string
}

// ===== Carga =====

// Cargar lee la configuración con la función de entorno recibida (os.Getenv
// en producción, un mapa en las pruebas) y devuelve el primer error hallado.
func Cargar(leerEntorno func(string) string) (Configuracion, error) {
	configuracion := Configuracion{DireccionEscucha: direccionEscuchaPorDefecto}
	if valor := leerEntorno(variableDireccionEscucha); valor != "" {
		configuracion.DireccionEscucha = valor
	}
	_, puerto, errorDireccion := net.SplitHostPort(configuracion.DireccionEscucha)
	if errorDireccion != nil {
		return configuracion, fmt.Errorf("%s debe ser una dirección host:puerto", variableDireccionEscucha)
	}
	if !puertoValido(puerto) {
		return configuracion, fmt.Errorf("%s debe tener un puerto entre %d y %d", variableDireccionEscucha, puertoMinimo, puertoMaximo)
	}

	if errorOrigenes := cargarOrigenes(leerEntorno, &configuracion); errorOrigenes != nil {
		return configuracion, errorOrigenes
	}
	if errorLimites := cargarLimites(leerEntorno, &configuracion); errorLimites != nil {
		return configuracion, errorLimites
	}
	if errorPrefijos := cargarPrefijos(leerEntorno, &configuracion); errorPrefijos != nil {
		return configuracion, errorPrefijos
	}
	proxies, errorProxies := interpretarProxiesFiables(leerEntorno(variableProxiesFiables))
	if errorProxies != nil {
		return configuracion, errorProxies
	}
	configuracion.ProxiesFiables = proxies
	return configuracion, nil
}

// cargarOrigenes lee el origen de cada dominio. SCRAPING_URL es obligatorio;
// los demás son opcionales y, vacíos, dejan sus prefijos respondiendo 404.
func cargarOrigenes(leerEntorno func(string) string, configuracion *Configuracion) error {
	valorScraping := leerEntorno(variableScraping)
	if valorScraping == "" {
		return fmt.Errorf("%s es obligatoria: Control de scraping es dueño de /health y /api/v1/scraper", variableScraping)
	}
	scraping, errorScraping := interpretarOrigen(variableScraping, valorScraping)
	if errorScraping != nil {
		return errorScraping
	}
	configuracion.Scraping = scraping

	for _, opcional := range []struct {
		variable string
		destino  **url.URL
	}{
		{variableCatalogo, &configuracion.Catalogo},
		{variableIdentidad, &configuracion.Identidad},
		{variableRutas, &configuracion.Rutas},
		{variableIA, &configuracion.IA},
	} {
		valor := leerEntorno(opcional.variable)
		if valor == "" {
			continue
		}
		origen, errorOrigen := interpretarOrigen(opcional.variable, valor)
		if errorOrigen != nil {
			return errorOrigen
		}
		*opcional.destino = origen
	}
	return nil
}

// cargarLimites lee las duraciones; vacías usan el valor por defecto.
func cargarLimites(leerEntorno func(string) string, configuracion *Configuracion) error {
	for _, limite := range []struct {
		variable   string
		destino    *time.Duration
		porDefecto time.Duration
	}{
		{"GATEWAY_REQUEST_TIMEOUT", &configuracion.LimitePeticion, limitePeticionPorDefecto},
		{"GATEWAY_DIAL_TIMEOUT", &configuracion.LimiteConexion, limiteConexionPorDefecto},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", &configuracion.LimiteCabecerasRespuesta, limiteCabecerasRespuestaPorDefecto},
		{"GATEWAY_READ_HEADER_TIMEOUT", &configuracion.LimiteLecturaCabeceras, limiteLecturaCabecerasPorDefecto},
		{"GATEWAY_IDLE_TIMEOUT", &configuracion.LimiteInactividad, limiteInactividadPorDefecto},
		{"GATEWAY_SHUTDOWN_TIMEOUT", &configuracion.LimiteApagado, limiteApagadoPorDefecto},
		{"GATEWAY_READINESS_TIMEOUT", &configuracion.LimiteDiagnostico, limiteDiagnosticoPorDefecto},
		{"GATEWAY_KEEPALIVE_TIMEOUT", &configuracion.LimiteKeepAlive, limiteKeepAlivePorDefecto},
	} {
		*limite.destino = limite.porDefecto
		valor := leerEntorno(limite.variable)
		if valor == "" {
			continue
		}
		duracion, errorDuracion := time.ParseDuration(valor)
		if errorDuracion != nil || duracion <= 0 {
			return fmt.Errorf("%s debe ser una duración positiva, por ejemplo 5s", limite.variable)
		}
		*limite.destino = duracion
	}
	return nil
}

// cargarPrefijos lee los prefijos de cada dominio y rechaza que dos dominios
// reclamen el mismo, porque sólo uno de ellos recibiría el tráfico.
func cargarPrefijos(leerEntorno func(string) string, configuracion *Configuracion) error {
	duenoPorPrefijo := map[string]string{}
	for _, dominio := range []struct {
		variable   string
		destino    *[]string
		porDefecto []string
	}{
		{"GATEWAY_IDENTIDAD_PREFIXES", &configuracion.PrefijosIdentidad, []string{"/api/v1/auth", "/api/v1/admin/usuarios", "/api/v1/usuarios/me/direcciones"}},
		{"GATEWAY_CATALOGO_PREFIXES", &configuracion.PrefijosCatalogo, []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{"GATEWAY_RUTAS_PREFIXES", &configuracion.PrefijosRutas, []string{"/api/v1/listas"}},
		{"GATEWAY_SCRAPING_PREFIXES", &configuracion.PrefijosScraping, []string{"/api/v1/scraper", "/swagger"}},
		{"GATEWAY_IA_PREFIXES", &configuracion.PrefijosIA, []string{"/api/v1/chat"}},
	} {
		prefijos, errorPrefijos := interpretarListaPrefijos(dominio.variable, leerEntorno(dominio.variable), dominio.porDefecto)
		if errorPrefijos != nil {
			return errorPrefijos
		}
		for _, prefijo := range prefijos {
			if dueno, repetido := duenoPorPrefijo[prefijo]; repetido {
				return fmt.Errorf("%s: el prefijo %q ya lo asigna %s", dominio.variable, prefijo, dueno)
			}
			duenoPorPrefijo[prefijo] = dominio.variable
		}
		*dominio.destino = prefijos
	}
	return nil
}

// ===== Validación de valores =====

// puertoValido indica si el texto es un puerto TCP utilizable.
func puertoValido(texto string) bool {
	numero, errorNumero := strconv.Atoi(texto)
	return errorNumero == nil && numero >= puertoMinimo && numero <= puertoMaximo
}

// interpretarOrigen acepta sólo un origen http(s) sin credenciales, ruta,
// query ni fragmento. El error nunca repite el valor, que podría traer secretos.
func interpretarOrigen(variable, valor string) (*url.URL, error) {
	origen, errorURL := url.Parse(valor)
	if errorURL != nil || origen == nil || origen.Hostname() == "" ||
		(origen.Scheme != "http" && origen.Scheme != "https") ||
		origen.User != nil || origen.Opaque != "" ||
		(origen.Path != "" && origen.Path != "/") ||
		origen.RawQuery != "" || origen.ForceQuery || origen.Fragment != "" ||
		strings.ContainsAny(origen.Host, " \t\r\n") {
		return nil, fmt.Errorf("%s debe ser un origen http(s) sin credenciales, ruta, query ni fragmento", variable)
	}
	if puerto := origen.Port(); puerto != "" && !puertoValido(puerto) {
		return nil, fmt.Errorf("el puerto de %s debe estar entre %d y %d", variable, puertoMinimo, puertoMaximo)
	}
	return origen, nil
}

// interpretarListaPrefijos lee prefijos separados por comas; vacío usa los
// valores por defecto. Cada entrada debe ser una ruta absoluta sin esquema,
// host, query, fragmento, codificación %, segmentos punto ni espacios. La
// barra final se recorta para que "/api/v1/x/" siga cubriendo sus rutas hijas.
func interpretarListaPrefijos(variable, valor string, porDefecto []string) ([]string, error) {
	if strings.TrimSpace(valor) == "" {
		return append([]string(nil), porDefecto...), nil
	}
	entradas := strings.Split(valor, ",")
	prefijos := make([]string, 0, len(entradas))
	vistos := make(map[string]bool, len(entradas))
	for _, entrada := range entradas {
		prefijo := strings.TrimSpace(entrada)
		invalido := func(motivo string) error {
			return fmt.Errorf("%s: prefijo inválido %q: %s", variable, prefijo, motivo)
		}
		switch {
		case prefijo == "":
			return nil, fmt.Errorf("%s: entrada vacía en %q", variable, valor)
		case strings.Contains(prefijo, "://"):
			return nil, invalido("debe ser una ruta, no una URL")
		case !strings.HasPrefix(prefijo, "/") || strings.HasPrefix(prefijo, "//"):
			return nil, invalido("debe empezar con una sola /")
		case strings.ContainsAny(prefijo, "?#"):
			return nil, invalido("no debe tener query ni fragmento")
		case strings.Contains(prefijo, "%"):
			return nil, invalido("no debe estar codificado con %")
		case strings.IndexFunc(prefijo, unicode.IsSpace) >= 0:
			return nil, invalido("no debe tener espacios")
		}
		prefijo = strings.TrimRight(prefijo, "/")
		if prefijo == "" {
			return nil, invalido("la raíz / capturaría todas las rutas")
		}
		for _, segmento := range strings.Split(prefijo[1:], "/") {
			if segmento == "" || segmento == "." || segmento == ".." {
				return nil, invalido("no debe tener segmentos vacíos ni punto")
			}
		}
		if vistos[prefijo] {
			return nil, invalido("repetido")
		}
		vistos[prefijo] = true
		prefijos = append(prefijos, prefijo)
	}
	return prefijos, nil
}

// interpretarProxiesFiables lee IPs o CIDRs separados por comas. Sólo los
// pares dentro de estas redes pueden fijar X-Forwarded-For/Proto.
func interpretarProxiesFiables(valor string) ([]netip.Prefix, error) {
	if valor == "" {
		return nil, nil
	}
	var redes []netip.Prefix
	for _, entrada := range strings.Split(valor, ",") {
		entrada = strings.TrimSpace(entrada)
		red, errorRed := netip.ParsePrefix(entrada)
		if errorRed != nil {
			direccion, errorDireccion := netip.ParseAddr(entrada)
			if errorDireccion != nil {
				return nil, fmt.Errorf("%s debe contener direcciones IP o CIDR", variableProxiesFiables)
			}
			red = netip.PrefixFrom(direccion, direccion.BitLen())
		}
		redes = append(redes, red.Masked())
	}
	return redes, nil
}
