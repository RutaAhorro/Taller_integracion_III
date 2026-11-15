package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func respuesta(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

// Una respuesta de error de Control no debe provocar un panic (review de #101).
func TestLeerEstadoTrabajoRechazaRespuestasInvalidas(t *testing.T) {
	for name, resp := range map[string]*http.Response{
		"401 sin token":      respuesta(401, `{"estado":401,"mensaje":"Token de servicio inválido o ausente."}`),
		"404 trabajo":        respuesta(404, `{"error":"not found"}`),
		"500 sin cuerpo":     respuesta(500, ``),
		"200 JSON inválido":  respuesta(200, `<html>`),
		"200 sin estado":     respuesta(200, `{"cadena_nombre":"Jumbo"}`),
		"200 sin cadena":     respuesta(200, `{"estado":"en_progreso"}`),
		"200 tipos erróneos": respuesta(200, `{"estado":3,"cadena_nombre":null}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := leerEstadoTrabajo(resp); err == nil {
				t.Fatal("se aceptó una respuesta inválida")
			}
		})
	}
}

func TestLeerEstadoTrabajoValido(t *testing.T) {
	trabajo, err := leerEstadoTrabajo(respuesta(200, `{"estado":"fallido","cadena_nombre":"Jumbo","elementos_extraidos":42,"registro_errores":"timeout"}`))
	if err != nil {
		t.Fatal(err)
	}
	if trabajo.Estado != "fallido" || trabajo.Cadena != "Jumbo" || trabajo.ElementosExtraidos != 42 || trabajo.RegistroErrores != "timeout" {
		t.Fatalf("%+v", trabajo)
	}
}

func TestLeerIDTrabajo(t *testing.T) {
	if id, ok := leerIDTrabajo(respuesta(201, `{"id":"abc"}`)); !ok || id != "abc" {
		t.Fatal(id, ok)
	}
	for _, body := range []string{`{}`, `{"id":7}`, `{"id":""}`, `no-json`} {
		if _, ok := leerIDTrabajo(respuesta(201, body)); ok {
			t.Fatalf("id inválido aceptado: %s", body)
		}
	}
}
