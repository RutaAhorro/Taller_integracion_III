package input_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/input"
)

func TestCollectCombinaArgumentosYArchivo(prueba *testing.T) {
	file := filepath.Join(prueba.TempDir(), "urls.txt")
	body := "# comentario\nhttps://www.jumbo.cl/uno\n\n  https://www.jumbo.cl/dos  \n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		prueba.Fatal(err)
	}

	got, err := input.Collect([]string{"https://www.jumbo.cl/inicial"}, file)
	if err != nil {
		prueba.Fatalf("Collect() error = %v", err)
	}
	want := []string{
		"https://www.jumbo.cl/inicial",
		"https://www.jumbo.cl/uno",
		"https://www.jumbo.cl/dos",
	}
	if !reflect.DeepEqual(got, want) {
		prueba.Errorf("Collect() = %#v, want %#v", got, want)
	}
}

func TestCollectSinArchivoConservaLosArgumentos(prueba *testing.T) {
	got, err := input.Collect([]string{"https://www.jumbo.cl/uno"}, "")
	if err != nil {
		prueba.Fatalf("Collect() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"https://www.jumbo.cl/uno"}) {
		prueba.Errorf("Collect() = %#v", got)
	}
}

func TestCollectRechazaArchivoInexistente(prueba *testing.T) {
	if _, err := input.Collect(nil, filepath.Join(prueba.TempDir(), "no-existe.txt")); err == nil {
		prueba.Fatal("Collect() debió fallar con un archivo inexistente")
	}
}
