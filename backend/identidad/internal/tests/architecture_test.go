package tests

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIdentityDoesNotDependOnOtherServices(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, i := range f.Imports {
			dependency, _ := strconv.Unquote(i.Path.Value)
			if strings.HasPrefix(dependency, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependency, "github.com/danirc2024/Taller_integracion_III/backend/identidad/") {
				t.Errorf("%s importa otro servicio: %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
