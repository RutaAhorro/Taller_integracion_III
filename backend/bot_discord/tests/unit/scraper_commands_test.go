package bot_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/equipo/bot_discord/internal/bot"
)

func respuestaHTTP(estadoHTTP int, cuerpo string) *http.Response {
	return &http.Response{StatusCode: estadoHTTP, Body: io.NopCloser(strings.NewReader(cuerpo))}
}

// Una respuesta de error de Control no debe provocar un panic (review de #101).
func TestLeerEstadoTrabajoRechazaRespuestasInvalidas(prueba *testing.T) {
	for nombre, respuesta := range map[string]*http.Response{
		"401 sin token":      respuestaHTTP(401, `{"estado":401,"mensaje":"Token de servicio inválido o ausente."}`),
		"404 trabajo":        respuestaHTTP(404, `{"error":"not found"}`),
		"500 sin cuerpo":     respuestaHTTP(500, ``),
		"200 JSON inválido":  respuestaHTTP(200, `<html>`),
		"200 sin estado":     respuestaHTTP(200, `{"cadena_nombre":"Jumbo"}`),
		"200 sin cadena":     respuestaHTTP(200, `{"estado":"en_progreso"}`),
		"200 tipos erróneos": respuestaHTTP(200, `{"estado":3,"cadena_nombre":null}`),
	} {
		prueba.Run(nombre, func(prueba *testing.T) {
			if _, errorObtenido := bot.LeerEstadoTrabajo(respuesta); errorObtenido == nil {
				prueba.Fatal("se aceptó una respuesta inválida")
			}
		})
	}
}

func TestLeerEstadoTrabajoValido(prueba *testing.T) {
	trabajo, errorObtenido := bot.LeerEstadoTrabajo(respuestaHTTP(200, `{"estado":"fallido","cadena_nombre":"Jumbo","elementos_extraidos":42,"registro_errores":"timeout"}`))
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if trabajo.Estado != "fallido" || trabajo.Cadena != "Jumbo" || trabajo.ElementosExtraidos != 42 || trabajo.RegistroErrores != "timeout" {
		prueba.Fatalf("%+v", trabajo)
	}
}

func TestLeerIDTrabajo(prueba *testing.T) {
	if identificador, esValido := bot.LeerIDTrabajo(respuestaHTTP(201, `{"id":"abc"}`)); !esValido || identificador != "abc" {
		prueba.Fatal(identificador, esValido)
	}
	for _, cuerpo := range []string{`{}`, `{"id":7}`, `{"id":""}`, `no-json`} {
		if _, esValido := bot.LeerIDTrabajo(respuestaHTTP(201, cuerpo)); esValido {
			prueba.Fatalf("id inválido aceptado: %s", cuerpo)
		}
	}
}
