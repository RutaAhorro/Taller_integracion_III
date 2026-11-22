// proxy.go reenvía las peticiones al dominio dueño y traduce sus fallos de
// red a 502/504 en JSON. Nunca reintenta: una escritura repetida podría
// duplicar un efecto aunque lleve Idempotency-Key.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

// ===== Dominios =====

// Nombres de dominio: prefijo de los códigos de error (<dominio>_unavailable,
// <dominio>_timeout) y claves del diagnóstico.
const (
	dominioIdentidad = "identidad"
	dominioCatalogo  = "catalogo"
	dominioRutas     = "rutas"
	dominioIA        = "ia"
	dominioScraping  = "scraping"
)

// mensajesDominio son los textos para el usuario cuando el dominio falla.
type mensajesDominio struct{ noDisponible, sinRespuesta string }

var mensajesPorDominio = map[string]mensajesDominio{
	dominioIdentidad: {"Identidad no está disponible.", "Identidad no respondió a tiempo."},
	dominioCatalogo:  {"Catálogo no está disponible.", "Catálogo no respondió a tiempo."},
	dominioRutas:     {"El motor de rutas no está disponible.", "El motor de rutas no respondió a tiempo."},
	dominioIA:        {"El asistente de IA no está disponible.", "El asistente de IA no respondió a tiempo."},
	dominioScraping:  {"Control de scraping no está disponible.", "Control de scraping no respondió a tiempo."},
}

// cabecerasIdentidadFalsificables las fija sólo cada servicio a partir del
// JWT; si el cliente las envía, se eliminan antes de reenviar.
var cabecerasIdentidadFalsificables = []string{"X-User-ID", "X-Role", "X-User-Role"}

// ===== Transportes =====

// nuevosTransportes crea un transporte con keep-alive para lecturas y otro
// HTTP/1 sin keep-alive para escrituras: net/http reenvía solo una petición
// si una conexión reutilizada se cierra, y eso no debe pasar con escrituras.
func nuevosTransportes(configuracion config.Configuracion) (lecturas, escrituras *http.Transport) {
	lecturas = http.DefaultTransport.(*http.Transport).Clone()
	lecturas.Proxy = nil // El tráfico interno no debe salir por un proxy de egreso.
	lecturas.DialContext = (&net.Dialer{Timeout: configuracion.LimiteConexion, KeepAlive: configuracion.LimiteKeepAlive}).DialContext
	lecturas.TLSHandshakeTimeout = configuracion.LimiteConexion
	lecturas.ResponseHeaderTimeout = configuracion.LimiteCabecerasRespuesta
	lecturas.DisableCompression = true // Conserva la representación y cabeceras del upstream.
	escrituras = lecturas.Clone()
	escrituras.DisableKeepAlives = true
	escrituras.Protocols = new(http.Protocols)
	escrituras.Protocols.SetHTTP1(true)
	return lecturas, escrituras
}

// transportePorMetodo elige el transporte según el método sea seguro o no.
type transportePorMetodo struct{ lecturas, escrituras *http.Transport }

// RoundTrip envía GET/HEAD/OPTIONS/TRACE por lecturas y el resto por escrituras.
func (transporte transportePorMetodo) RoundTrip(peticion *http.Request) (*http.Response, error) {
	switch peticion.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return transporte.lecturas.RoundTrip(peticion)
	default:
		return transporte.escrituras.RoundTrip(peticion)
	}
}

// ===== Proxy inverso =====

// origenDe devuelve el origen configurado para un dominio.
func origenDe(dominio string, configuracion config.Configuracion) *url.URL {
	switch dominio {
	case dominioIdentidad:
		return configuracion.Identidad
	case dominioCatalogo:
		return configuracion.Catalogo
	case dominioRutas:
		return configuracion.Rutas
	case dominioIA:
		return configuracion.IA
	default:
		return configuracion.Scraping
	}
}

// nuevoProxy crea el proxy de un dominio. Conserva ruta y query tal como
// llegaron (codificación incluida), Host, cookies y Authorization.
func nuevoProxy(dominio string, configuracion config.Configuracion, transporte http.RoundTripper, registro *slog.Logger) *httputil.ReverseProxy {
	origen := origenDe(dominio, configuracion)
	mensajes := mensajesPorDominio[dominio]
	return &httputil.ReverseProxy{
		Transport: transporte,
		Rewrite: func(reescritura *httputil.ProxyRequest) {
			reescritura.SetURL(origen)
			// El destino es un origen: no se normalizan rutas ni queries.
			reescritura.Out.URL.Path = reescritura.In.URL.Path
			reescritura.Out.URL.RawPath = reescritura.In.URL.RawPath
			reescritura.Out.URL.RawQuery = reescritura.In.URL.RawQuery
			reescritura.Out.URL.ForceQuery = reescritura.In.URL.ForceQuery
			reescritura.Out.Host = reescritura.In.Host
			reescritura.Out.Header.Set(cabeceraIDPeticion, reescritura.In.Header.Get(cabeceraIDPeticion))
			for _, cabecera := range cabecerasIdentidadFalsificables {
				reescritura.Out.Header.Del(cabecera)
			}
			ipCliente, esquema := identidadReenvio(reescritura.In, configuracion.ProxiesFiables)
			reescritura.SetXForwarded()
			if ipCliente != "" {
				reescritura.Out.Header.Set("X-Forwarded-For", ipCliente)
			}
			reescritura.Out.Header.Set("X-Forwarded-Proto", esquema)
		},
		ModifyResponse: func(respuesta *http.Response) error {
			respuesta.Header.Set(cabeceraIDPeticion, respuesta.Request.Header.Get(cabeceraIDPeticion))
			return nil
		},
		ErrorLog: slog.NewLogLogger(registro.Handler(), slog.LevelError),
		ErrorHandler: func(respuesta http.ResponseWriter, peticion *http.Request, errorUpstream error) {
			responderFalloUpstream(respuesta, peticion, errorUpstream, dominio, mensajes, registro)
		},
	}
}

// responderFalloUpstream traduce un fallo de red a 502 o, si fue por tiempo,
// a 504. Si el cliente canceló no hay a quién responder y sólo se registra.
func responderFalloUpstream(respuesta http.ResponseWriter, peticion *http.Request, errorUpstream error, dominio string, mensajes mensajesDominio, registro *slog.Logger) {
	idPeticion := peticion.Header.Get(cabeceraIDPeticion)
	if errors.Is(peticion.Context().Err(), context.Canceled) {
		registro.Info("upstream_canceled", "request_id", idPeticion)
		return
	}
	estado, codigo, mensaje := http.StatusBadGateway, dominio+"_unavailable", mensajes.noDisponible
	var errorRed net.Error
	if errors.Is(errorUpstream, context.DeadlineExceeded) || (errors.As(errorUpstream, &errorRed) && errorRed.Timeout()) {
		estado, codigo, mensaje = http.StatusGatewayTimeout, dominio+"_timeout", mensajes.sinRespuesta
	}
	registro.Warn("upstream_error", "request_id", idPeticion, "upstream", dominio, "status", estado, "error", errorUpstream.Error())
	respuesta.Header().Set("Content-Type", tipoContenidoJSON)
	respuesta.Header().Set(cabeceraIDPeticion, idPeticion)
	respuesta.WriteHeader(estado)
	_ = json.NewEncoder(respuesta).Encode(map[string]string{"error": codigo, "mensaje": mensaje, "request_id": idPeticion})
}
