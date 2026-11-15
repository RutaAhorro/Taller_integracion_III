// Package main inicia el bot de Discord.
package main

import (
	"log"

	"github.com/equipo/bot_discord/internal/bot"
)

// main delega la ejecución al paquete del bot.
func main() {
	if err := bot.Run(); err != nil {
		log.Fatal(err)
	}
}
