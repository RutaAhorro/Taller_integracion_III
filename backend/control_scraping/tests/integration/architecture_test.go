// Package integration_test verifica límites arquitectónicos del módulo.
package integration_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Go impide importar los internal anidados desde fuera de su módulo. Esta
// prueba impide además que el propio módulo vuelva a depender del monolito
// global o que su negocio importe sus adaptadores de persistencia/HTTP.
func TestModuleDependencies(prueba *testing.T) {
	const api = "github.com/danirc2024/Taller_integracion_III/backend/control_scraping/"
	for _, module := range []string{"control-scraping"} {
		prueba.Run(module, func(prueba *testing.T) {
			root := filepath.Join("..", "..", "internal")
			own := api + "internal/"
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
					dependency, err := strconv.Unquote(spec.Path.Value)
					if err != nil {
						return err
					}
					if strings.HasPrefix(dependency, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependency, api) {
						prueba.Errorf("%s importa otro servicio: %s", path, dependency)
					}
					if strings.HasPrefix(dependency, api) {
						allowed := strings.HasPrefix(dependency, own) || dependency == api+"internal/middleware" || dependency == api+"docs"
						if !allowed {
							prueba.Errorf("%s depende de código de otro dominio: %s", path, dependency)
						}
					}
					if layer == "services" && strings.HasPrefix(dependency, own) && dependency != own+"domain" && dependency != own+"security" {
						prueba.Errorf("%s: el servicio depende de un adaptador: %s", path, dependency)
					}
					if layer == "domain" && dependency != "context" && dependency != "time" && dependency != "github.com/google/uuid" {
						prueba.Errorf("%s: el contrato de dominio depende de infraestructura: %s", path, dependency)
					}
					if layer == "services" && (strings.HasPrefix(dependency, "gorm.io/") || strings.HasPrefix(dependency, "github.com/gin-gonic/") || strings.HasPrefix(dependency, "github.com/redis/")) {
						prueba.Errorf("%s: el servicio importa directamente infraestructura: %s", path, dependency)
					}
				}
				return nil
			})
			if err != nil {
				prueba.Fatal(err)
			}
		})
	}
}
