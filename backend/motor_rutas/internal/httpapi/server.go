// Package httpapi expone la API pública de listas (detrás del Gateway) y las
// sondas internas de salud.
//
// Recorrido de una petición, de afuera hacia adentro:
//
//	observar   → asigna X-Request-ID y escribe el log http_request
//	cors       → solo deja pasar orígenes de la lista blanca
//	recuperar  → convierte un panic en un 500 sin filtrar detalles
//	rutas      → elige el handler según ruta y método (404/405 propios)
//	conSesion  → valida la cookie jwt (solo rutas de /api/v1/listas)
//	conID      → valida el {lista_id} de la URL
//	handler    → listar, crear, obtener, actualizar o eliminar
//
// Este archivo contiene el armado, el despacho y los middlewares; los
// handlers de negocio están en listas.go y los errores en errores.go.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
)

// =============================================================================
// Opciones y armado
// =============================================================================

// Opciones reúne las dependencias del servidor.
type Opciones struct {
	Repo         listas.Repositorio         // acceso a datos (PostgreSQL o memoria en pruebas)
	JWTSecret    string                     // secreto para verificar la cookie jwt
	OrigenesCORS []string                   // orígenes exactos permitidos
	MaxListas    int                        // tope de listas por usuario
	Ping         func(context.Context) bool // salud de la base; nil en pruebas (ready responde ok)
	Log          *slog.Logger               // logger estructurado; nil usa el por defecto
}

// prefijoListas es la ruta base de la API pública.
const prefijoListas = "/api/v1/listas"

// requestIDValido limita el X-Request-ID recibido a caracteres seguros para logs.
var requestIDValido = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

// Nuevo arma el handler completo con sus middlewares en el orden descrito arriba.
func Nuevo(opciones Opciones) http.Handler {
	if opciones.Log == nil {
		opciones.Log = slog.Default()
	}
	conRutas := rutas(opciones)
	conRecuperacion := recuperar(opciones.Log, conRutas)
	conCORS := cors(opciones.OrigenesCORS, conRecuperacion)
	return observar(opciones.Log, conCORS)
}

// =============================================================================
// Despacho de rutas
// =============================================================================

// metodosPorRuta asocia cada método HTTP (GET, POST...) con su handler.
type metodosPorRuta map[string]http.HandlerFunc

// rutas despacha "a mano" (sin ServeMux con métodos) para que los 404 y 405
// también usen el envoltorio común de errores.
func rutas(opciones Opciones) http.Handler {
	manejador := &handlerListas{opciones: opciones}

	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		ruta := peticion.URL.Path

		switch {
		// --- Sondas internas (sin sesión) ------------------------------------
		case ruta == "/_rutas/live":
			despachar(respuesta, peticion, ruta, metodosPorRuta{
				http.MethodGet: manejador.live,
			})
		case ruta == "/_rutas/ready" || ruta == "/health":
			despachar(respuesta, peticion, ruta, metodosPorRuta{
				http.MethodGet: manejador.ready,
			})

		// --- /api/v1/listas --------------------------------------------------
		case ruta == prefijoListas || ruta == prefijoListas+"/":
			despachar(respuesta, peticion, prefijoListas, metodosPorRuta{
				http.MethodGet:  manejador.conSesion(manejador.listar),
				http.MethodPost: manejador.conSesion(manejador.crear),
			})

		// --- /api/v1/listas/{lista_id} (un solo segmento después) -------------
		case esRutaDeUnaLista(ruta):
			idLista := ruta[len(prefijoListas)+1:]
			despachar(respuesta, peticion, prefijoListas+"/{lista_id}", metodosPorRuta{
				http.MethodGet:    manejador.conSesion(manejador.conID(idLista, manejador.obtener)),
				http.MethodPut:    manejador.conSesion(manejador.conID(idLista, manejador.actualizar)),
				http.MethodDelete: manejador.conSesion(manejador.conID(idLista, manejador.eliminar)),
			})

		default:
			escribirError(respuesta, http.StatusNotFound, mensajeRutaNoExiste, "")
		}
	})
}

// esRutaDeUnaLista reconoce "/api/v1/listas/<algo>" sin más barras después.
func esRutaDeUnaLista(ruta string) bool {
	if !strings.HasPrefix(ruta, prefijoListas+"/") {
		return false
	}
	resto := ruta[len(prefijoListas)+1:]
	return !strings.Contains(resto, "/")
}

// despachar ejecuta el handler del método pedido o responde 405 con la
// cabecera Allow listando los métodos aceptados.
func despachar(respuesta http.ResponseWriter, peticion *http.Request, patron string, metodos metodosPorRuta) {
	marcarRuta(peticion, patron)

	if manejar, metodoPermitido := metodos[peticion.Method]; metodoPermitido {
		manejar(respuesta, peticion)
		return
	}

	permitidos := make([]string, 0, len(metodos))
	for metodo := range metodos {
		permitidos = append(permitidos, metodo)
	}
	slices.Sort(permitidos) // orden estable: "GET, POST"
	respuesta.Header().Set("Allow", strings.Join(permitidos, ", "))
	escribirError(respuesta, http.StatusMethodNotAllowed, mensajeMetodoNoPermitido, "")
}

// =============================================================================
// Middleware: observación (X-Request-ID y log)
// =============================================================================

// claveRuta identifica, dentro del contexto de la petición, el patrón de ruta
// que se escribirá en el log.
type claveRuta struct{}

// marcarRuta guarda el patrón ("/api/v1/listas/{lista_id}") y no la ruta
// concreta, para que el log no registre ids de listas.
func marcarRuta(peticion *http.Request, patron string) {
	if patronActual, hayPatron := peticion.Context().Value(claveRuta{}).(*string); hayPatron {
		*patronActual = patron
	}
}

// registro envuelve la respuesta para recordar el código de estado enviado.
type registro struct {
	http.ResponseWriter
	estado int
}

func (registro *registro) WriteHeader(estado int) {
	if registro.estado == 0 {
		registro.estado = estado
	}
	registro.ResponseWriter.WriteHeader(estado)
}

func (registro *registro) Write(contenido []byte) (int, error) {
	if registro.estado == 0 {
		registro.estado = http.StatusOK // Write sin WriteHeader implica 200
	}
	return registro.ResponseWriter.Write(contenido)
}

// observar es el middleware más externo: toda respuesta (incluso un 403 de
// CORS) lleva X-Request-ID y genera exactamente una línea de log.
func observar(log *slog.Logger, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		// Reutiliza el X-Request-ID del Gateway si es válido; si no, genera uno.
		requestID := peticion.Header.Get("X-Request-ID")
		if !requestIDValido.MatchString(requestID) {
			requestID = generarRequestID()
		}
		respuesta.Header().Set("X-Request-ID", requestID)

		// "unmatched" queda en el log si ninguna ruta reconoce la petición.
		patron := "unmatched"
		peticion = peticion.WithContext(context.WithValue(peticion.Context(), claveRuta{}, &patron))

		inicio := time.Now()
		respuestaRegistrada := &registro{ResponseWriter: respuesta}
		siguiente.ServeHTTP(respuestaRegistrada, peticion)
		if respuestaRegistrada.estado == 0 {
			respuestaRegistrada.estado = http.StatusOK
		}

		log.Info("http_request",
			"request_id", requestID,
			"method", peticion.Method,
			"path", patron,
			"status", respuestaRegistrada.estado,
			"duration_ms", time.Since(inicio).Milliseconds())
	})
}

// generarRequestID devuelve 32 caracteres hexadecimales aleatorios.
func generarRequestID() string {
	var bytesAleatorios [16]byte
	_, _ = rand.Read(bytesAleatorios[:])
	return hex.EncodeToString(bytesAleatorios[:])
}

// =============================================================================
// Middleware: CORS
// =============================================================================

// cors permite llamadas con cookies solo desde los orígenes configurados.
// Las peticiones sin cabecera Origin (curl, Gateway, sondas) pasan sin cambios.
// OPTIONS es el "preflight" del navegador y se responde aquí mismo.
func cors(origenesPermitidos []string, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		respuesta.Header().Add("Vary", "Origin") // los proxies no deben mezclar orígenes en caché

		origen := peticion.Header.Get("Origin")
		traeOrigen := len(peticion.Header.Values("Origin")) > 0

		if traeOrigen && !slices.Contains(origenesPermitidos, origen) {
			escribirError(respuesta, http.StatusForbidden, mensajeOrigenNoPermitido, "")
			return
		}
		if traeOrigen {
			cabeceras := respuesta.Header()
			cabeceras.Set("Access-Control-Allow-Origin", origen)
			cabeceras.Set("Access-Control-Allow-Credentials", "true")
			cabeceras.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			cabeceras.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if peticion.Method == http.MethodOptions {
			respuesta.WriteHeader(http.StatusNoContent)
			return
		}
		siguiente.ServeHTTP(respuesta, peticion)
	})
}

// =============================================================================
// Middleware: recuperación de panics
// =============================================================================

// recuperar convierte un panic en un 500 genérico. El detalle solo va al log,
// nunca al cliente.
func recuperar(log *slog.Logger, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		defer func() {
			valorPanic := recover()
			if valorPanic == nil {
				return
			}
			// ErrAbortHandler es la forma estándar de cortar una respuesta: se respeta.
			if valorPanic == http.ErrAbortHandler {
				panic(valorPanic)
			}
			log.Error("error_interno", "ruta", peticion.URL.Path, "detalle", valorPanic)
			escribirError(respuesta, http.StatusInternalServerError, mensajeErrorInterno, "")
		}()
		siguiente.ServeHTTP(respuesta, peticion)
	})
}
