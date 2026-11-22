// Package api implementa el Gateway: elige el dominio dueño de cada ruta y
// reenvía la petición sin tocar sesión, permisos ni datos.
//
// pasarela.go arma el enrutador y registra cada petición; proxy.go reenvía a
// los upstreams; reenvio.go sanea X-Request-ID y X-Forwarded-*; salud.go
// responde las sondas propias del Gateway.
package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Constantes =====

const (
	// estadoClienteCerro sólo aparece en el log: el cliente cortó antes de que
	// hubiera respuesta (convención de nginx).
	estadoClienteCerro = 499

	cabeceraIDPeticion  = "X-Request-ID"
	tipoContenidoJSON   = "application/json; charset=utf-8"
	mensajeRutaNoExiste = "La ruta o recurso solicitado no existe."
)

// rutasSaludScraping son las rutas fuera de los prefijos que también
// pertenecen a Control de scraping.
var rutasSaludScraping = map[string]bool{
	"/": true, "/health": true, "/health/": true, "/api/v1/health": true, "/api/v1/health/": true,
}

// ===== Pasarela =====

// Pasarela es el http.Handler del Gateway. Guarda los transportes para poder
// cerrar sus conexiones al apagar.
type Pasarela struct {
	manejador  http.Handler
	lecturas   *http.Transport
	escrituras *http.Transport
}

// destino asocia los prefijos de un dominio con su proxy.
type destino struct {
	prefijos []string
	proxy    *httputil.ReverseProxy
}

// Nueva arma la Pasarela. Un dominio sin origen configurado no se registra y
// sus prefijos responden 404; no hay respaldo hacia otro dominio.
func Nueva(configuracion config.Configuracion, registro *slog.Logger) *Pasarela {
	if registro == nil {
		registro = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	lecturas, escrituras := nuevosTransportes(configuracion)
	clienteDiagnostico := &http.Client{
		Transport:     lecturas,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	transporte := transportePorMetodo{lecturas: lecturas, escrituras: escrituras}
	nuevo := func(dominio string) *httputil.ReverseProxy {
		return nuevoProxy(dominio, configuracion, transporte, registro)
	}

	// El orden importa sólo para legibilidad: config rechaza prefijos repetidos.
	var destinos []destino
	for _, candidato := range []struct {
		origenConfigurado bool
		dominio           string
		prefijos          []string
	}{
		{configuracion.Identidad != nil, dominioIdentidad, configuracion.PrefijosIdentidad},
		{configuracion.Catalogo != nil, dominioCatalogo, configuracion.PrefijosCatalogo},
		{configuracion.Rutas != nil, dominioRutas, configuracion.PrefijosRutas},
		{configuracion.IA != nil, dominioIA, configuracion.PrefijosIA},
	} {
		if candidato.origenConfigurado {
			destinos = append(destinos, destino{prefijos: candidato.prefijos, proxy: nuevo(candidato.dominio)})
		}
	}
	proxyScraping := nuevo(dominioScraping)

	manejador := http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		idPeticion := normalizarIDPeticion(peticion.Header.Get(cabeceraIDPeticion))
		peticion = peticion.Clone(peticion.Context())
		peticion.Header.Set(cabeceraIDPeticion, idPeticion)
		respuesta.Header().Set(cabeceraIDPeticion, idPeticion)
		inicio := time.Now()
		registrada := &respuestaRegistrada{ResponseWriter: respuesta}
		defer registrarPeticion(registro, peticion, registrada, idPeticion, inicio)

		if responderSonda(registrada, peticion, configuracion, clienteDiagnostico, idPeticion) {
			return
		}
		contexto, cancelar := context.WithTimeout(peticion.Context(), configuracion.LimitePeticion)
		defer cancelar()
		peticion = peticion.WithContext(contexto)

		for _, candidato := range destinos {
			if tienePrefijo(peticion.URL.Path, candidato.prefijos) {
				candidato.proxy.ServeHTTP(registrada, peticion)
				return
			}
		}
		if tienePrefijo(peticion.URL.Path, configuracion.PrefijosScraping) || rutasSaludScraping[peticion.URL.Path] {
			proxyScraping.ServeHTTP(registrada, peticion)
			return
		}
		responderRutaNoExiste(registrada, peticion)
	})
	return &Pasarela{manejador: manejador, lecturas: lecturas, escrituras: escrituras}
}

// ServeHTTP delega en el enrutador armado por Nueva.
func (pasarela *Pasarela) ServeHTTP(respuesta http.ResponseWriter, peticion *http.Request) {
	pasarela.manejador.ServeHTTP(respuesta, peticion)
}

// CerrarConexionesInactivas libera las conexiones reutilizables al apagar.
func (pasarela *Pasarela) CerrarConexionesInactivas() {
	pasarela.lecturas.CloseIdleConnections()
	pasarela.escrituras.CloseIdleConnections()
}

// ===== Enrutamiento =====

// tienePrefijo exige coincidencia exacta o por segmento: /api/v1/auth cubre
// /api/v1/auth/login pero no /api/v1/authentication.
func tienePrefijo(ruta string, prefijos []string) bool {
	for _, prefijo := range prefijos {
		if ruta == prefijo || strings.HasPrefix(ruta, prefijo+"/") {
			return true
		}
	}
	return false
}

// responderRutaNoExiste devuelve el 404 JSON propio del Gateway.
func responderRutaNoExiste(respuesta http.ResponseWriter, peticion *http.Request) {
	respuesta.Header().Set("Content-Type", tipoContenidoJSON)
	respuesta.WriteHeader(http.StatusNotFound)
	if peticion.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(respuesta).Encode(map[string]any{"estado": http.StatusNotFound, "mensaje": mensajeRutaNoExiste})
}

// ===== Registro de peticiones =====

// registrarPeticion escribe una línea JSON por petición, con su estado final.
func registrarPeticion(registro *slog.Logger, peticion *http.Request, respuesta *respuestaRegistrada, idPeticion string, inicio time.Time) {
	estado := respuesta.estado
	if estado == 0 {
		estado = http.StatusOK
		if peticion.Context().Err() != nil {
			estado = estadoClienteCerro
		}
	}
	registro.Info("request", "request_id", idPeticion, "method", peticion.Method,
		"path", peticion.URL.EscapedPath(), "status", estado,
		"duration_ms", time.Since(inicio).Milliseconds(), "response_bytes", respuesta.bytes)
}

// respuestaRegistrada anota estado y bytes para el log. Unwrap conserva el
// soporte de ResponseController para flush y cambios de protocolo.
type respuestaRegistrada struct {
	http.ResponseWriter
	estado int
	bytes  int
}

// Unwrap expone la respuesta original a http.ResponseController.
func (respuesta *respuestaRegistrada) Unwrap() http.ResponseWriter { return respuesta.ResponseWriter }

// WriteHeader guarda el primer estado final (los 1xx informativos no cuentan,
// salvo 101 que sí cierra la respuesta HTTP).
func (respuesta *respuestaRegistrada) WriteHeader(estado int) {
	if respuesta.estado == 0 && (estado >= http.StatusOK || estado == http.StatusSwitchingProtocols) {
		respuesta.estado = estado
	}
	respuesta.ResponseWriter.WriteHeader(estado)
}

// Write cuenta los bytes enviados y fija 200 si nadie escribió cabeceras.
func (respuesta *respuestaRegistrada) Write(contenido []byte) (int, error) {
	if respuesta.estado == 0 {
		respuesta.WriteHeader(http.StatusOK)
	}
	escritos, errorEscritura := respuesta.ResponseWriter.Write(contenido)
	respuesta.bytes += escritos
	return escritos, errorEscritura
}
