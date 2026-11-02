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
