// Package scraping_test verifica el cliente HTTP hacia Control con servidores
// httptest locales: no se contacta a ningún Control real ni se inicia scraping.
package scraping_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/equipo/bot_discord/internal/scraping"
)

const (
	tokenPrueba   = "token-de-servicio-de-prueba-0123456789"
	timeoutPrueba = 5 * time.Second
)

// servidorControl levanta un Control simulado que responde siempre con el
// estado y cuerpo indicados y devuelve un cliente apuntando a él.
func servidorControl(prueba *testing.T, estadoHTTP int, cuerpo string) *scraping.Cliente {
	prueba.Helper()
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		respuesta.WriteHeader(estadoHTTP)
		_, _ = io.WriteString(respuesta, cuerpo)
	}))
	prueba.Cleanup(servidor.Close)
	cliente, errorObtenido := scraping.NuevoCliente(servidor.URL+"/api/v1/scraper/trabajos", tokenPrueba, timeoutPrueba)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	return cliente
}

// TestNuevoClienteExigeTokenYURL comprueba que el constructor falle sin dependencias obligatorias.
func TestNuevoClienteExigeTokenYURL(prueba *testing.T) {
	if _, errorObtenido := scraping.NuevoCliente("http://control/trabajos", "  ", timeoutPrueba); errorObtenido == nil {
		prueba.Fatal("se aceptó un token de servicio vacío")
	}
	if _, errorObtenido := scraping.NuevoCliente("", tokenPrueba, timeoutPrueba); errorObtenido == nil {
		prueba.Fatal("se aceptó una URL vacía")
	}
}

// TestConsultarEstadoRechazaRespuestasInvalidas: una respuesta de error o mal formada de Control no debe provocar un panic (review de #101).
func TestConsultarEstadoRechazaRespuestasInvalidas(prueba *testing.T) {
	casos := []struct {
		nombre string
		estado int
		cuerpo string
	}{
		{"401 sin token", http.StatusUnauthorized, `{"estado":401,"mensaje":"Token de servicio inválido o ausente."}`},
		{"404 trabajo", http.StatusNotFound, `{"error":"not found"}`},
		{"500 sin cuerpo", http.StatusInternalServerError, ``},
		{"200 JSON inválido", http.StatusOK, `<html>`},
		{"200 sin estado", http.StatusOK, `{"cadena_nombre":"Jumbo"}`},
		{"200 sin cadena", http.StatusOK, `{"estado":"en_progreso"}`},
		{"200 tipos erróneos", http.StatusOK, `{"estado":3,"cadena_nombre":null}`},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			cliente := servidorControl(prueba, caso.estado, caso.cuerpo)
			if _, errorObtenido := cliente.ConsultarEstado(context.Background(), "trabajo-1"); errorObtenido == nil {
				prueba.Fatal("se aceptó una respuesta inválida")
			}
		})
	}
}

// TestConsultarEstadoValido verifica la lectura de todos los campos que muestra el bot.
func TestConsultarEstadoValido(prueba *testing.T) {
	cliente := servidorControl(prueba, http.StatusOK, `{"estado":"fallido","cadena_nombre":"Jumbo","elementos_extraidos":42,"registro_errores":"timeout"}`)
	trabajo, errorObtenido := cliente.ConsultarEstado(context.Background(), "trabajo-1")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if trabajo.Estado != scraping.EstadoFallido || trabajo.Cadena != "Jumbo" || trabajo.ElementosExtraidos != 42 || trabajo.RegistroErrores != "timeout" {
		prueba.Fatalf("%+v", trabajo)
	}
}

// TestConsultarEstadoDistingueFalloDeRed comprueba que un Control inalcanzable se marque como error de comunicación.
func TestConsultarEstadoDistingueFalloDeRed(prueba *testing.T) {
	servidor := httptest.NewServer(http.NotFoundHandler())
	direccion := servidor.URL
	servidor.Close()
	cliente, errorObtenido := scraping.NuevoCliente(direccion+"/api/v1/scraper/trabajos", tokenPrueba, timeoutPrueba)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, errorObtenido := cliente.ConsultarEstado(context.Background(), "trabajo-1"); errorObtenido == nil || !strings.Contains(errorObtenido.Error(), scraping.ErrComunicacion.Error()) {
		prueba.Fatalf("se esperaba un error de comunicación: %v", errorObtenido)
	}
}

// TestCrearTrabajo verifica el contrato de la creación: ruta, cabeceras, cuerpo y lectura del identificador.
func TestCrearTrabajo(prueba *testing.T) {
	var metodo, ruta, autorizacion, tipoContenido, cuerpo string
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		datos, _ := io.ReadAll(peticion.Body)
		metodo, ruta, cuerpo = peticion.Method, peticion.URL.Path, string(datos)
		autorizacion, tipoContenido = peticion.Header.Get("Authorization"), peticion.Header.Get("Content-Type")
		respuesta.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(respuesta, `{"id":"abc"}`)
	}))
	prueba.Cleanup(servidor.Close)
	cliente, errorObtenido := scraping.NuevoCliente(servidor.URL+"/api/v1/scraper/trabajos/", tokenPrueba, timeoutPrueba)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	trabajoID, errorObtenido := cliente.CrearTrabajo(context.Background(), 1, "usuario-1")
	if errorObtenido != nil || trabajoID != "abc" {
		prueba.Fatal(trabajoID, errorObtenido)
	}
	if metodo != http.MethodPost || ruta != "/api/v1/scraper/trabajos" ||
		autorizacion != "Bearer "+tokenPrueba || tipoContenido != "application/json" ||
		!strings.Contains(cuerpo, `"cadena_id":1`) || !strings.Contains(cuerpo, `"disparado_por_usuario_id":"usuario-1"`) {
		prueba.Fatalf("petición inesperada: %s %s auth=%q tipo=%q cuerpo=%s", metodo, ruta, autorizacion, tipoContenido, cuerpo)
	}
}

// TestCrearTrabajoRechazaRespuestasInvalidas cubre identificadores ausentes y estados distintos de 201.
func TestCrearTrabajoRechazaRespuestasInvalidas(prueba *testing.T) {
	casos := []struct {
		nombre string
		estado int
		cuerpo string
	}{
		{"sin id", http.StatusCreated, `{}`},
		{"id numérico", http.StatusCreated, `{"id":7}`},
		{"id vacío", http.StatusCreated, `{"id":""}`},
		{"no JSON", http.StatusCreated, `no-json`},
		{"estado 500", http.StatusInternalServerError, `{"id":"abc"}`},
	}
	for _, caso := range casos {
		prueba.Run(caso.nombre, func(prueba *testing.T) {
			cliente := servidorControl(prueba, caso.estado, caso.cuerpo)
			if _, errorObtenido := cliente.CrearTrabajo(context.Background(), 1, "usuario-1"); errorObtenido == nil {
				prueba.Fatal("se aceptó una respuesta inválida")
			}
		})
	}
}

// TestEjecutarTrabajo verifica que solo 202 cuente como aceptado y que se envíe el rastreador.
func TestEjecutarTrabajo(prueba *testing.T) {
	var ruta, cuerpo string
	servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		datos, _ := io.ReadAll(peticion.Body)
		ruta, cuerpo = peticion.URL.Path, string(datos)
		respuesta.WriteHeader(http.StatusAccepted)
	}))
	prueba.Cleanup(servidor.Close)
	cliente, errorObtenido := scraping.NuevoCliente(servidor.URL+"/api/v1/scraper/trabajos", tokenPrueba, timeoutPrueba)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if errorObtenido := cliente.EjecutarTrabajo(context.Background(), "abc", "jumbo_rsc"); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if ruta != "/api/v1/scraper/trabajos/abc/ejecutar" || cuerpo != `{"spider":"jumbo_rsc"}` {
		prueba.Fatalf("petición inesperada: %s %s", ruta, cuerpo)
	}
	if errorObtenido := servidorControl(prueba, http.StatusOK, ``).EjecutarTrabajo(context.Background(), "abc", "jumbo_rsc"); errorObtenido == nil {
		prueba.Fatal("se aceptó un estado distinto de 202")
	}
}

// TestNombreCadena verifica el nombre del catálogo y el alternativo para cadenas desconocidas.
func TestNombreCadena(prueba *testing.T) {
	if nombre := scraping.NombreCadena(4, "otro"); nombre != "Santa Isabel" {
		prueba.Fatalf("nombre inesperado: %q", nombre)
	}
	if nombre := scraping.NombreCadena(99, "otro"); nombre != "otro" {
		prueba.Fatalf("nombre alternativo inesperado: %q", nombre)
	}
}
