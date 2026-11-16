// Package main inicia el servidor del servicio de coordinación de scraping.
package main

import (
	"log/slog"
	"os"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/app"
)

// ===== Inicio del proceso =====

// main inicia la aplicación y termina con error si el servicio no puede arrancar.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if errorObtenido := app.Run(); errorObtenido != nil {
		slog.Error("scraping_startup_failed", "error", errorObtenido)
		os.Exit(1)
	}
}
