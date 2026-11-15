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
	for _, modulo := range []string{"control-scraping"} {
		prueba.Run(modulo, func(prueba *testing.T) {
			raiz := filepath.Join("..", "..", "internal")
			propio := api + "internal/"
			errorObtenido := filepath.WalkDir(raiz, func(ruta string, entrada fs.DirEntry, errorObtenido error) error {
				if errorObtenido != nil {
					return errorObtenido
				}
				if entrada.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
					return nil
				}
				archivo, errorObtenido := parser.ParseFile(token.NewFileSet(), ruta, nil, parser.ImportsOnly)
				if errorObtenido != nil {
					return errorObtenido
				}
				capa := filepath.Base(filepath.Dir(ruta))
				for _, especificacion := range archivo.Imports {
					dependencia, errorObtenido := strconv.Unquote(especificacion.Path.Value)
					if errorObtenido != nil {
						return errorObtenido
					}
					if strings.HasPrefix(dependencia, "github.com/danirc2024/Taller_integracion_III/backend/") && !strings.HasPrefix(dependencia, api) {
						prueba.Errorf("%s importa otro servicio: %s", ruta, dependencia)
					}
					if strings.HasPrefix(dependencia, api) {
						permitido := strings.HasPrefix(dependencia, propio) || dependencia == api+"internal/middleware" || dependencia == api+"docs"
						if !permitido {
							prueba.Errorf("%s depende de código de otro dominio: %s", ruta, dependencia)
						}
					}
					if capa == "services" && strings.HasPrefix(dependencia, propio) && dependencia != propio+"domain" && dependencia != propio+"security" {
						prueba.Errorf("%s: el servicio depende de un adaptador: %s", ruta, dependencia)
					}
					if capa == "domain" && dependencia != "context" && dependencia != "time" && dependencia != "github.com/google/uuid" {
						prueba.Errorf("%s: el contrato de dominio depende de infraestructura: %s", ruta, dependencia)
					}
					if capa == "services" && (strings.HasPrefix(dependencia, "gorm.io/") || strings.HasPrefix(dependencia, "github.com/gin-gonic/") || strings.HasPrefix(dependencia, "github.com/redis/")) {
						prueba.Errorf("%s: el servicio importa directamente infraestructura: %s", ruta, dependencia)
					}
				}
				return nil
			})
			if errorObtenido != nil {
				prueba.Fatal(errorObtenido)
			}
		})
	}
}
