// Límite de arquitectura: Identidad no importa código de otros servicios.

package unit_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIdentityDoesNotDependOnOtherServices(prueba *testing.T) {
	errorOperacion := filepath.WalkDir(filepath.Join("..", ".."), func(ruta string, entrada fs.DirEntry, errorObtenido error) error {
		if errorObtenido != nil {
			return errorObtenido
		}
		if entrada.IsDir() || !strings.HasSuffix(ruta, ".go") {
			return nil
		}
		archivo, errorObtenido := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
		if errorObtenido != nil {
			return errorObtenido
		}
		for _, indice := range archivo.Imports {
			dependencia, _ := strconv.Unquote(indice.Path.Value)
			if strings.HasPrefix(dependencia, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependencia, "github.com/danirc2024/Taller_integracion_III/backend/identidad/") {
				prueba.Errorf("%s importa otro servicio: %s", ruta, dependencia)
			}
		}
		return nil
	})
	if errorOperacion != nil {
		prueba.Fatal(errorOperacion)
	}
}
