package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectURLs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "urls.txt")
	body := "# comentario\nhttps://www.jumbo.cl/uno\n\nhttps://www.jumbo.cl/dos\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := collectURLs([]string{"https://www.jumbo.cl/inicial"}, file)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"https://www.jumbo.cl/inicial",
		"https://www.jumbo.cl/uno",
		"https://www.jumbo.cl/dos",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectURLs() = %#v, want %#v", got, want)
	}
}

func TestTemporaryCategoryDir(t *testing.T) {
	dir, err := temporaryCategoryDir("santa_isabel_rsc", []string{"https://www.santaisabel.cl/despensa"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if filepath.Base(dir) == "" {
		t.Fatal("directorio temporal vacío")
	}
	if got := categoryFileName("santa_isabel_rsc"); got != "santa_isabel_categories.txt" {
		t.Errorf("categoryFileName() = %q", got)
	}
	body, err := os.ReadFile(filepath.Join(dir, "santa_isabel_categories.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "https://www.santaisabel.cl/despensa\n" {
		t.Errorf("contenido = %q", body)
	}
}
