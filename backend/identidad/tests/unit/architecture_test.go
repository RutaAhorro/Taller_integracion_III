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
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, entrada fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entrada.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		archivo, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, indice := range archivo.Imports {
			dependency, _ := strconv.Unquote(indice.Path.Value)
			if strings.HasPrefix(dependency, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependency, "github.com/danirc2024/Taller_integracion_III/backend/identidad/") {
				prueba.Errorf("%s importa otro servicio: %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		prueba.Fatal(err)
	}
}
