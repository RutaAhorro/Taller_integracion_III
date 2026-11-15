// Package main inicia el bot de Discord.
package main

import (
	"log"

	"github.com/equipo/bot_discord/internal/bot"
)

// main delega la ejecución al paquete del bot.
func main() {
	if errorObtenido := bot.Run(); errorObtenido != nil {
		log.Fatal(errorObtenido)
	}
}
