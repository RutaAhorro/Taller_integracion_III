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

func TestCatalogDependencies(t *testing.T) {
	const backend = "github.com/danirc2024/Taller_integracion_III/backend/"
	const own = backend + "catalogo/internal/"
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		layer := filepath.Base(filepath.Dir(path))
		for _, spec := range file.Imports {
			dep, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(dep, backend) && !strings.HasPrefix(dep, own) {
				t.Errorf("%s importa otro servicio: %s", path, dep)
			}
			if strings.Contains(dep, "redis") {
				t.Errorf("%s depende de la cola del coordinador", path)
			}
			if layer == "services" && strings.HasPrefix(dep, own) && dep != own+"domain" && dep != own+"utils" {
				t.Errorf("%s importa un adaptador desde negocio: %s", path, dep)
			}
			if layer == "domain" && dep != "context" && dep != "time" {
				t.Errorf("%s importa infraestructura desde el contrato: %s", path, dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
