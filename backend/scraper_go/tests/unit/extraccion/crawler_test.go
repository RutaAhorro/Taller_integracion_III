// crawler_test.go verifica el cliente HTTP del crawler: manejo de 404,
// reintentos, detección de bloqueos antibot, robots.txt y límites de descarga.
package extraccion_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	extraccion "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/extraccion"
)

// ===== Auxiliares de Cliente HTTP =====

// clienteCrawlerDePrueba construye un cliente HTTP de prueba con backoff mínimo para tests rápidos.
func clienteCrawlerDePrueba(maximoReintentos int) *extraccion.Client {
	return extraccion.New(extraccion.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    2 * time.Second,
		MaxSize:    1 << 20,
		MaxRetries: maximoReintentos,
		BackoffMin: time.Millisecond,
		BackoffMax: 50 * time.Millisecond,
		Logger:     registradorSilencioso(),
	})
}

// ===== Pruebas de Descarga y Reintentos =====

// TestFetchDevuelve404ParaLaPaginacion verifica que HTTP 404 se devuelva como Response sin error.
func TestFetchDevuelve404ParaLaPaginacion(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		http.NotFound(respuesta, peticion)
	}))
	defer servidor.Close()

	respuesta, errorObtenido := clienteCrawlerDePrueba(0).Fetch(context.Background(), servidor.URL+"/missing")
	if errorObtenido != nil {
		prueba.Fatalf("Fetch() error = %v", errorObtenido)
	}
	if respuesta.Status != http.StatusNotFound {
		prueba.Errorf("status = %d", respuesta.Status)
	}
}

// TestFetchAceptaPaginasConScriptsDeCDN verifica que un 200 real con scripts públicos de Cloudflare/CDN no se marque bloqueado.
func TestFetchAceptaPaginasConScriptsDeCDN(prueba *testing.T) {
	cuerpo := `<html><head><script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js"></script>` +
		`<script src="https://static.cloudflareinsights.com/beacon.min.js"></script></head>` +
		`<body><h1>Leche Entera</h1>` +
		`<div class="wpforms-recaptcha-container"></div>` +
		`<script>var config={"readOnlyDisallowedFields":["captcha"]};</script></body></html>`
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		_, _ = respuesta.Write([]byte(cuerpo))
	}))
	defer servidor.Close()

	respuesta, errorObtenido := clienteCrawlerDePrueba(0).Fetch(context.Background(), servidor.URL+"/producto")
	if errorObtenido != nil {
		prueba.Fatalf("Fetch() error = %v", errorObtenido)
	}
	if string(respuesta.Body) != cuerpo {
		prueba.Errorf("cuerpo = %q", respuesta.Body)
	}
}

// TestFetchReintentaEstadosTransitorios verifica que el crawler reintente ante HTTP 429.
func TestFetchReintentaEstadosTransitorios(prueba *testing.T) {
	var llamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		if llamadas.Add(1) == 1 {
			respuesta.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer servidor.Close()

	respuesta, errorObtenido := clienteCrawlerDePrueba(1).Fetch(context.Background(), servidor.URL+"/retry")
	if errorObtenido != nil {
		prueba.Fatalf("Fetch() error = %v", errorObtenido)
	}
	if respuesta.Status != http.StatusOK || string(respuesta.Body) != "ok" {
		prueba.Errorf("respuesta = %d %q", respuesta.Status, respuesta.Body)
	}
	if llamadas.Load() != 2 {
		prueba.Errorf("llamadas = %d, esperado 2", llamadas.Load())
	}
}

// ===== Pruebas de Bloqueos Antibot =====

// TestFetchReportaBloqueoPersistente verifica que tras agotar reintentos se devuelva BlockedError con el conteo acumulado.
func TestFetchReportaBloqueoPersistente(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		respuesta.WriteHeader(http.StatusForbidden)
		_, _ = respuesta.Write([]byte("Just a moment"))
	}))
	defer servidor.Close()

	_, errorObtenido := clienteCrawlerDePrueba(1).Fetch(context.Background(), servidor.URL+"/blocked")
	var bloqueado *extraccion.BlockedError
	if !errors.As(errorObtenido, &bloqueado) {
		prueba.Fatalf("error = %T %v, esperado *BlockedError", errorObtenido, errorObtenido)
	}
	if bloqueado.BlockedResponses != 2 {
		prueba.Errorf("BlockedResponses = %d, esperado 2 (intento inicial + reintento)", bloqueado.BlockedResponses)
	}
}

// TestFetchClasificaEstadosDeBloqueo verifica la detección de bloqueos por código de estado HTTP.
func TestFetchClasificaEstadosDeBloqueo(prueba *testing.T) {
	for _, estado := range []int{
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		prueba.Run(http.StatusText(estado), func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				if peticion.URL.Path == "/robots.txt" {
					http.NotFound(respuesta, peticion)
					return
				}
				respuesta.WriteHeader(estado)
			}))
			defer servidor.Close()

			_, errorObtenido := clienteCrawlerDePrueba(0).Fetch(context.Background(), servidor.URL+"/blocked")
			var bloqueado *extraccion.BlockedError
			if !errors.As(errorObtenido, &bloqueado) {
				prueba.Fatalf("error = %T %v, esperado *BlockedError", errorObtenido, errorObtenido)
			}
			if bloqueado.Status != estado {
				prueba.Errorf("status = %d, esperado %d", bloqueado.Status, estado)
			}
			if bloqueado.BlockedResponses != 1 {
				prueba.Errorf("BlockedResponses = %d, esperado 1", bloqueado.BlockedResponses)
			}
		})
	}
}

// TestFetchDetectaMarcadoresDeChallenge verifica la detección de desafíos antibot en respuestas HTTP 200.
func TestFetchDetectaMarcadoresDeChallenge(prueba *testing.T) {
	casos := map[string]string{
		"cloudflare":      "cf-chl",
		"just a moment":   "Just a moment...",
		"access denied":   "<h1>Access Denied</h1>",
		"unusual traffic": "Our systems have detected unusual traffic",
		"captcha":         `<div id="captcha"></div>`,
		"captcha solve":   "please solve the captcha.",
		"perimeter x":     `<title>Robot or human?</title><div id="px-captcha"></div>`,
	}
	for nombre, cuerpo := range casos {
		prueba.Run(nombre, func(prueba *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				if peticion.URL.Path == "/robots.txt" {
					http.NotFound(respuesta, peticion)
					return
				}
				_, _ = respuesta.Write([]byte(cuerpo))
			}))
			defer servidor.Close()

			_, errorObtenido := clienteCrawlerDePrueba(0).Fetch(context.Background(), servidor.URL+"/challenge")
			var bloqueado *extraccion.BlockedError
			if !errors.As(errorObtenido, &bloqueado) {
				prueba.Fatalf("error = %T %v, esperado *BlockedError", errorObtenido, errorObtenido)
			}
			if bloqueado.Reason == "" {
				prueba.Error("BlockedError.Reason vacío")
			}
		})
	}
}

// ===== Pruebas de Timeouts, Robots y Tamaño =====

// TestFetchReintentaTimeout verifica que los timeouts de descarga se reintenten según MaxRetries.
func TestFetchReintentaTimeout(prueba *testing.T) {
	var llamadasPagina atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		llamadasPagina.Add(1)
		time.Sleep(100 * time.Millisecond)
		_, _ = respuesta.Write([]byte("late"))
	}))
	defer servidor.Close()

	cliente := extraccion.New(extraccion.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    20 * time.Millisecond,
		MaxSize:    1 << 20,
		MaxRetries: 1,
		BackoffMin: time.Millisecond,
		BackoffMax: 50 * time.Millisecond,
		Logger:     registradorSilencioso(),
	})
	if _, errorObtenido := cliente.Fetch(context.Background(), servidor.URL+"/slow"); errorObtenido == nil {
		prueba.Fatal("Fetch() debió fallar por timeout")
	}
	if llamadasPagina.Load() != 2 {
		prueba.Fatalf("llamadas = %d, esperado 2 (timeout + reintento)", llamadasPagina.Load())
	}
}

// TestFetchRespetaRobots verifica que una ruta prohibida por robots.txt devuelva ErrRobotsDisallowed.
func TestFetchRespetaRobots(prueba *testing.T) {
	var llamadasPagina atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			_, _ = respuesta.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		llamadasPagina.Add(1)
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer servidor.Close()

	_, errorObtenido := clienteCrawlerDePrueba(0).Fetch(context.Background(), servidor.URL+"/private")
	if !errors.Is(errorObtenido, extraccion.ErrRobotsDisallowed) {
		prueba.Fatalf("error = %v, esperado ErrRobotsDisallowed", errorObtenido)
	}
	if llamadasPagina.Load() != 0 {
		prueba.Errorf("se solicitó una URL prohibida %d veces", llamadasPagina.Load())
	}
}

// TestFetchRespetaRetryAfter verifica que el crawler espere la duración indicada en Retry-After.
func TestFetchRespetaRetryAfter(prueba *testing.T) {
	var llamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		if llamadas.Add(1) == 1 {
			respuesta.Header().Set("Retry-After", "1")
			respuesta.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = respuesta.Write([]byte("ok"))
	}))
	defer servidor.Close()

	inicio := time.Now()
	respuesta, errorObtenido := clienteCrawlerDePrueba(1).Fetch(context.Background(), servidor.URL+"/retry")
	if errorObtenido != nil {
		prueba.Fatalf("Fetch() error = %v", errorObtenido)
	}
	if respuesta.Status != http.StatusOK {
		prueba.Errorf("status = %d", respuesta.Status)
	}
	if transcurrido := time.Since(inicio); transcurrido < 900*time.Millisecond {
		prueba.Errorf("Fetch() esperó %v; debía respetar Retry-After de 1 s", transcurrido)
	}
}

// TestFetchRechazaRespuestasDemasiadoGrandes verifica que una descarga superior a MaxSize devuelva ErrResponseTooLarge.
func TestFetchRechazaRespuestasDemasiadoGrandes(prueba *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/robots.txt" {
			http.NotFound(respuesta, peticion)
			return
		}
		_, _ = respuesta.Write([]byte("0123456789"))
	}))
	defer servidor.Close()

	cliente := extraccion.New(extraccion.Options{
		UserAgent:  "TallerIntegracionIII/1.0",
		Delay:      time.Millisecond,
		Timeout:    time.Second,
		MaxSize:    4,
		MaxRetries: 0,
		Logger:     registradorSilencioso(),
	})
	if _, errorObtenido := cliente.Fetch(context.Background(), servidor.URL+"/grande"); !errors.Is(errorObtenido, extraccion.ErrResponseTooLarge) {
		prueba.Fatalf("error = %v, esperado ErrResponseTooLarge", errorObtenido)
	}
}
