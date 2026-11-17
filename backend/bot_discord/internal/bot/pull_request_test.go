package bot

import (
	"strings"
	"testing"
)

func TestFormatearEtiquetasPullRequest(prueba *testing.T) {
	datosPR := map[string]interface{}{
		"labels": []interface{}{
			map[string]interface{}{"name": "bug"},
			map[string]interface{}{"name": "prioridad-alta"},
		},
	}

	mensajeEtiquetas := formatearEtiquetasPullRequest(datosPR)
	if !strings.Contains(mensajeEtiquetas, "**Etiquetas**: `bug`, `prioridad-alta`") {
		prueba.Fatalf("no se mostraron las etiquetas del PR: %q", mensajeEtiquetas)
	}
}
