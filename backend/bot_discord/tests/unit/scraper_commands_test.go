package bot_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/equipo/bot_discord/internal/bot"
)

func respuestaHTTP(status int, cuerpo string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(cuerpo))}
}

// Una respuesta de error de Control no debe provocar un panic (review de #101).
func TestLeerEstadoTrabajoRechazaRespuestasInvalidas(t *testing.T) {
	for name, resp := range map[string]*http.Response{
		"401 sin token":      respuestaHTTP(401, `{"estado":401,"mensaje":"Token de servicio inválido o ausente."}`),
		"404 trabajo":        respuestaHTTP(404, `{"error":"not found"}`),
		"500 sin cuerpo":     respuestaHTTP(500, ``),
		"200 JSON inválido":  respuestaHTTP(200, `<html>`),
		"200 sin estado":     respuestaHTTP(200, `{"cadena_nombre":"Jumbo"}`),
		"200 sin cadena":     respuestaHTTP(200, `{"estado":"en_progreso"}`),
		"200 tipos erróneos": respuestaHTTP(200, `{"estado":3,"cadena_nombre":null}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := bot.LeerEstadoTrabajo(resp); err == nil {
				t.Fatal("se aceptó una respuesta inválida")
			}
		})
	}
}

func TestLeerEstadoTrabajoValido(t *testing.T) {
	trabajo, err := bot.LeerEstadoTrabajo(respuestaHTTP(200, `{"estado":"fallido","cadena_nombre":"Jumbo","elementos_extraidos":42,"registro_errores":"timeout"}`))
	if err != nil {
		t.Fatal(err)
	}
	if trabajo.Estado != "fallido" || trabajo.Cadena != "Jumbo" || trabajo.ElementosExtraidos != 42 || trabajo.RegistroErrores != "timeout" {
		t.Fatalf("%+v", trabajo)
	}
}

func TestLeerIDTrabajo(t *testing.T) {
	if id, ok := bot.LeerIDTrabajo(respuestaHTTP(201, `{"id":"abc"}`)); !ok || id != "abc" {
		t.Fatal(id, ok)
	}
	for _, body := range []string{`{}`, `{"id":7}`, `{"id":""}`, `no-json`} {
		if _, ok := bot.LeerIDTrabajo(respuestaHTTP(201, body)); ok {
			t.Fatalf("id inválido aceptado: %s", body)
		}
	}
}
