// proxy_test.go comprueba que el reenvío es transparente (métodos, cuerpo,
// cabeceras, estados, redirecciones, gzip) y cómo se tratan tiempos límite,
// cancelaciones y escrituras que no deben repetirse.
package integration_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ===== Transparencia =====

// TestPeticionYRespuestaTransparentes verifica que ruta codificada, query,
// Host, cookies, Authorization y Set-Cookie pasan sin cambios, y que las
// cabeceras de identidad falsificables y X-Forwarded-* del cliente no.
func TestPeticionYRespuestaTransparentes(prueba *testing.T) {
	for _, metodo := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"} {
		prueba.Run(metodo, func(subprueba *testing.T) {
			carga := "{\"productos\":[{\"producto\":\"Leche\"}]}"
			observada := make(chan *http.Request, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				cuerpo, _ := io.ReadAll(peticion.Body)
				peticion.Body = io.NopCloser(bytes.NewReader(cuerpo))
				observada <- peticion
				respuesta.Header().Set("Content-Type", "application/json")
				respuesta.Header().Add("Set-Cookie", "jwt=sesion-prueba; Path=/; HttpOnly; SameSite=Lax")
				respuesta.Header().Add("Set-Cookie", "preferencia=compacta; Path=/")
				respuesta.Header().Set("Access-Control-Allow-Origin", "https://frontend.example")
				respuesta.Header().Set("Access-Control-Allow-Credentials", "true")
				respuesta.WriteHeader(http.StatusCreated)
				_, _ = respuesta.Write([]byte("{\"mensaje\":\"Respuesta del upstream\"}"))
			}))
			defer upstream.Close()
			pasarela := pasarelaPara(subprueba, upstream.URL, nil, nil)
			peticion, _ := http.NewRequest(metodo, pasarela.URL+"/api/v1/scraper/a%2Fb?q=leche+entera&q=oferta&vacio=&codificado=%2F", strings.NewReader(carga))
			peticion.Host = "api.example"
			peticion.Header.Set("Content-Type", "application/json")
			peticion.Header.Set("Authorization", "Bearer token-prueba")
			peticion.Header.Set("Cookie", "jwt=sesion-prueba")
			peticion.Header.Set("Origin", "https://frontend.example")
			peticion.Header.Set("X-Request-ID", "sup-264.request_1")
			peticion.Header.Set("X-Forwarded-For", "203.0.113.200")
			peticion.Header.Set("X-Forwarded-Proto", "https")
			peticion.Header.Set("X-Role", "admin")
			peticion.Header.Set("X-User-ID", "usuario-falsificado")
			respuesta, errorPeticion := http.DefaultClient.Do(peticion)
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			cuerpo := cuerpoDe(subprueba, respuesta)
			if respuesta.StatusCode != http.StatusCreated || len(respuesta.Header.Values("Set-Cookie")) != 2 ||
				respuesta.Header.Get("X-Request-ID") != "sup-264.request_1" ||
				respuesta.Header.Get("Access-Control-Allow-Origin") != "https://frontend.example" ||
				respuesta.Header.Get("Access-Control-Allow-Credentials") != "true" {
				subprueba.Fatalf("cambió el contrato de respuesta: estado=%d cabeceras=%v", respuesta.StatusCode, respuesta.Header)
			}
			if metodo != "HEAD" && string(cuerpo) != "{\"mensaje\":\"Respuesta del upstream\"}" {
				subprueba.Fatalf("cambió el cuerpo de respuesta: %s", cuerpo)
			}
			recibida := <-observada
			cuerpoRecibido, _ := io.ReadAll(recibida.Body)
			if recibida.Method != metodo || recibida.Host != "api.example" || recibida.URL.EscapedPath() != "/api/v1/scraper/a%2Fb" ||
				recibida.URL.RawQuery != peticion.URL.RawQuery || string(cuerpoRecibido) != carga ||
				recibida.Header.Get("Authorization") != "Bearer token-prueba" || recibida.Header.Get("Cookie") != "jwt=sesion-prueba" ||
				recibida.Header.Get("Origin") != "https://frontend.example" || recibida.Header.Get("Content-Type") != "application/json" ||
				recibida.Header.Get("X-Request-ID") != "sup-264.request_1" {
				subprueba.Fatalf("cambió el contrato de petición: metodo=%s host=%s uri=%s cabeceras=%v cuerpo=%s", recibida.Method, recibida.Host, recibida.RequestURI, recibida.Header, cuerpoRecibido)
			}
			if recibida.Header.Get("X-Forwarded-For") != "127.0.0.1" || recibida.Header.Get("X-Forwarded-Proto") != "http" ||
				recibida.Header.Get("X-Role") != "" || recibida.Header.Get("X-User-ID") != "" {
				subprueba.Fatalf("se reenvió identidad falsificada: %v", recibida.Header)
			}
		})
	}
}

// TestEstadosYSaludDelUpstreamNoSeReemplazan verifica que el Gateway no
// reescribe estados ni cuerpos del upstream, tampoco en /api/v1/health.
func TestEstadosYSaludDelUpstreamNoSeReemplazan(prueba *testing.T) {
	for _, estado := range []int{200, 400, 401, 403, 404, 405, 409, 429, 500, 503} {
		prueba.Run(http.StatusText(estado), func(subprueba *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				respuesta.Header().Set("Retry-After", "15")
				respuesta.WriteHeader(estado)
				_, _ = respuesta.Write([]byte("respuesta-upstream"))
			}))
			defer upstream.Close()
			pasarela := pasarelaPara(subprueba, upstream.URL, nil, nil)
			respuesta, errorPeticion := http.Get(pasarela.URL + "/api/v1/health")
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			if cuerpo := string(cuerpoDe(subprueba, respuesta)); respuesta.StatusCode != estado || cuerpo != "respuesta-upstream" || respuesta.Header.Get("Retry-After") != "15" {
				subprueba.Fatalf("cambió la respuesta del upstream: estado=%d cuerpo=%q", respuesta.StatusCode, cuerpo)
			}
		})
	}
}

// TestRedireccionYGzipSeConservan verifica que el Gateway no sigue
// redirecciones ni descomprime la respuesta.
func TestRedireccionYGzipSeConservan(prueba *testing.T) {
	var comprimido bytes.Buffer
	escritorGzip := gzip.NewWriter(&comprimido)
	_, _ = escritorGzip.Write([]byte("contenido-upstream"))
	_ = escritorGzip.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.URL.Path == "/api/v1/scraper/redirigir" {
			respuesta.Header().Set("Location", "/api/v1/auth/me?desde=login")
			respuesta.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		respuesta.Header().Set("Content-Encoding", "gzip")
		_, _ = respuesta.Write(comprimido.Bytes())
	}))
	defer upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, nil, nil)
	transporte := http.DefaultTransport.(*http.Transport).Clone()
	transporte.DisableCompression = true
	defer transporte.CloseIdleConnections()
	cliente := &http.Client{Transport: transporte, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	respuesta, errorPeticion := cliente.Get(pasarela.URL + "/api/v1/scraper/redirigir")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	_ = cuerpoDe(prueba, respuesta)
	if respuesta.StatusCode != http.StatusTemporaryRedirect || respuesta.Header.Get("Location") != "/api/v1/auth/me?desde=login" {
		prueba.Fatal("cambió la redirección")
	}
	respuesta, errorPeticion = cliente.Get(pasarela.URL + "/api/v1/scraper/comprimido")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	if cuerpo := cuerpoDe(prueba, respuesta); respuesta.Header.Get("Content-Encoding") != "gzip" || !bytes.Equal(cuerpo, comprimido.Bytes()) {
		prueba.Fatal("cambió la compresión del upstream")
	}
}

// ===== Fallos y tiempos límite =====

// TestFalloUpstreamYVidaLocal verifica el 502 JSON con X-Request-ID y que
// /_gateway/live sigue respondiendo con el upstream caído.
func TestFalloUpstreamYVidaLocal(prueba *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, nil, nil)
	respuesta, errorPeticion := http.Get(pasarela.URL + "/api/v1/scraper/trabajos")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	cuerpo := cuerpoDe(prueba, respuesta)
	if respuesta.StatusCode != http.StatusBadGateway || !bytes.Contains(cuerpo, []byte("scraping_unavailable")) || respuesta.Header.Get("X-Request-ID") == "" {
		prueba.Fatalf("fallo de upstream incorrecto: %d %s", respuesta.StatusCode, cuerpo)
	}
	respuesta, errorPeticion = http.Get(pasarela.URL + "/_gateway/live")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	if cuerpo := cuerpoDe(prueba, respuesta); respuesta.StatusCode != http.StatusOK || !bytes.Contains(cuerpo, []byte("gateway")) {
		prueba.Fatalf("falló la sonda de vida local: %d %s", respuesta.StatusCode, cuerpo)
	}
}

// TestTiempoLimiteCancelaAlUpstream verifica el 504 y que el upstream ve la
// cancelación, tanto por cabeceras de respuesta como por la petición entera.
func TestTiempoLimiteCancelaAlUpstream(prueba *testing.T) {
	for _, entorno := range []map[string]string{
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT": "50ms"},
		{"GATEWAY_REQUEST_TIMEOUT": "50ms"},
	} {
		prueba.Run("limite", func(subprueba *testing.T) {
			cancelada := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				<-peticion.Context().Done()
				close(cancelada)
			}))
			defer upstream.Close()
			pasarela := pasarelaPara(subprueba, upstream.URL, entorno, nil)
			respuesta, errorPeticion := http.Get(pasarela.URL + "/api/v1/scraper/lento")
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			if cuerpo := cuerpoDe(subprueba, respuesta); respuesta.StatusCode != http.StatusGatewayTimeout || !bytes.Contains(cuerpo, []byte("scraping_timeout")) {
				subprueba.Fatalf("respuesta de tiempo límite incorrecta: %d %s", respuesta.StatusCode, cuerpo)
			}
			select {
			case <-cancelada:
			case <-time.After(2 * time.Second):
				subprueba.Fatal("el tiempo límite no llegó al upstream")
			}
		})
	}
}

// TestCancelacionDelClienteLlegaAlUpstream verifica que cortar la petición
// del cliente cancela la del upstream.
func TestCancelacionDelClienteLlegaAlUpstream(prueba *testing.T) {
	iniciada, cancelada := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		close(iniciada)
		<-peticion.Context().Done()
		close(cancelada)
	}))
	defer upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, nil, nil)
	contexto, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	peticion, _ := http.NewRequestWithContext(contexto, "GET", pasarela.URL+"/api/v1/scraper/esperando", nil)
	terminada := make(chan error, 1)
	go func() {
		respuesta, errorPeticion := http.DefaultClient.Do(peticion)
		if respuesta != nil {
			_ = respuesta.Body.Close()
		}
		terminada <- errorPeticion
	}()
	select {
	case <-iniciada:
	case <-time.After(2 * time.Second):
		prueba.Fatal("el upstream no recibió la petición")
	}
	cancelar()
	select {
	case errorPeticion := <-terminada:
		if errorPeticion == nil {
			prueba.Fatal("el cliente no observó la cancelación")
		}
	case <-time.After(2 * time.Second):
		prueba.Fatal("la petición del cliente no se detuvo")
	}
	select {
	case <-cancelada:
	case <-time.After(2 * time.Second):
		prueba.Fatal("la petición al upstream no se detuvo")
	}
}

// TestLimiteCortaRespuestaYaIniciada verifica que el límite de petición
// también corta una respuesta en streaming.
func TestLimiteCortaRespuestaYaIniciada(prueba *testing.T) {
	cancelada := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		_, _ = respuesta.Write([]byte("respuesta-parcial"))
		respuesta.(http.Flusher).Flush()
		<-peticion.Context().Done()
		close(cancelada)
	}))
	defer upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, map[string]string{"GATEWAY_REQUEST_TIMEOUT": "100ms"}, nil)
	respuesta, errorPeticion := http.Get(pasarela.URL + "/api/v1/scraper/flujo")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	defer respuesta.Body.Close()
	cuerpo, errorLectura := io.ReadAll(respuesta.Body)
	if respuesta.StatusCode != http.StatusOK || string(cuerpo) != "respuesta-parcial" || errorLectura == nil {
		prueba.Fatalf("no se cortó el flujo tras el límite: estado=%d cuerpo=%q error=%v", respuesta.StatusCode, cuerpo, errorLectura)
	}
	select {
	case <-cancelada:
	case <-time.After(2 * time.Second):
		prueba.Fatal("el límite del flujo no canceló al upstream")
	}
}

// TestEscriturasNoSeRepitenTrasFalloDeConexion simula una escritura aplicada
// cuya respuesta se pierde: debe llegar una sola vez y devolver 502.
func TestEscriturasNoSeRepitenTrasFalloDeConexion(prueba *testing.T) {
	var escrituras atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		if peticion.Method == "GET" {
			respuesta.WriteHeader(http.StatusOK)
			return
		}
		escrituras.Add(1)
		conexion, _, errorHijack := respuesta.(http.Hijacker).Hijack()
		if errorHijack == nil {
			_ = conexion.Close()
		}
	}))
	defer upstream.Close()
	pasarela := pasarelaPara(prueba, upstream.URL, nil, nil)
	respuesta, errorPeticion := http.Get(pasarela.URL + "/api/v1/scraper/calentar-conexion")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	_ = cuerpoDe(prueba, respuesta)
	peticion, _ := http.NewRequest("POST", pasarela.URL+"/api/v1/scraper/trabajos", nil)
	peticion.Header.Set("Idempotency-Key", "observacion-prueba")
	respuesta, errorPeticion = http.DefaultClient.Do(peticion)
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	_ = cuerpoDe(prueba, respuesta)
	if respuesta.StatusCode != http.StatusBadGateway || escrituras.Load() != 1 {
		prueba.Fatalf("se repitió la escritura u ocultó el fallo: estado=%d escrituras=%d", respuesta.StatusCode, escrituras.Load())
	}
}
