// Package main arranca el Bot de Discord: carga la configuración, conecta con
// Discord y mantiene el servidor de webhooks de GitHub hasta recibir una señal
// de apagado.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/config"
	"github.com/equipo/bot_discord/internal/discord"
	"github.com/equipo/bot_discord/internal/scraping"
	"github.com/joho/godotenv"
)

// ===== Constantes =====

// timeoutCierreServidor es el tiempo que se da a las peticiones en curso para
// terminar antes de cerrar el servidor de webhooks.
const timeoutCierreServidor = 10 * time.Second

// ===== Inicio del proceso =====

// main ejecuta el bot y termina con error si no pudo arrancar o cayó.
func main() {
	if errorObtenido := ejecutar(); errorObtenido != nil {
		log.Fatal(errorObtenido)
	}
}

// ejecutar arma las piezas del bot y espera hasta el apagado. Vive aparte de
// main para que los defer (cierre de Discord, cancelación de señales) corran
// antes de que log.Fatal termine el proceso.
func ejecutar() error {
	// El .env es solo una comodidad de desarrollo local; en contenedor la
	// configuración llega por el entorno.
	if errorObtenido := godotenv.Load(); errorObtenido != nil {
		log.Printf("Aviso: no se encontró .env; se usará la configuración del entorno: %v", errorObtenido)
	}
	configuracion, errorObtenido := config.Cargar()
	if errorObtenido != nil {
		return errorObtenido
	}
	control, errorObtenido := scraping.NuevoCliente(configuracion.URLBaseScraper, configuracion.TokenServicioScraping, configuracion.TimeoutHTTP)
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo crear el cliente de Control: %w", errorObtenido)
	}
	sesionDiscord, errorObtenido := discordgo.New("Bot " + configuracion.TokenDiscord)
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo crear la sesión de Discord: %w", errorObtenido)
	}

	contextoSenales, cancelarSenales := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelarSenales()

	manejador, errorObtenido := discord.NuevoManejador(contextoSenales, configuracion, sesionDiscord, control)
	if errorObtenido != nil {
		return errorObtenido
	}
	manejador.Registrar()
	if errorObtenido := sesionDiscord.Open(); errorObtenido != nil {
		return fmt.Errorf("no se pudo abrir la conexión de Discord: %w", errorObtenido)
	}
	defer func() {
		if errorObtenido := sesionDiscord.Close(); errorObtenido != nil {
			log.Printf("No se pudo cerrar la sesión de Discord: %v", errorObtenido)
		}
	}()
	log.Println("Bot conectado a Discord!")

	return servirWebhooks(contextoSenales, manejador.ServidorWebhook())
}

// servirWebhooks mantiene el servidor activo hasta que falle o se cancele el
// contexto; en ese caso lo cierra de forma ordenada.
func servirWebhooks(contexto context.Context, servidor *http.Server) error {
	resultadoServidor := make(chan error, 1)
	go func() {
		log.Printf("Servidor Webhook escuchando en %s", servidor.Addr)
		resultadoServidor <- servidor.ListenAndServe()
	}()

	select {
	case errorObtenido := <-resultadoServidor:
		if !errors.Is(errorObtenido, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks: %w", errorObtenido)
		}
	case <-contexto.Done():
		contextoCierre, cancelarCierre := context.WithTimeout(context.Background(), timeoutCierreServidor)
		defer cancelarCierre()
		if errorObtenido := servidor.Shutdown(contextoCierre); errorObtenido != nil {
			return fmt.Errorf("no se pudo cerrar el servidor de webhooks: %w", errorObtenido)
		}
		if errorObtenido := <-resultadoServidor; !errors.Is(errorObtenido, http.ErrServerClosed) {
			return fmt.Errorf("falló el servidor de webhooks durante el cierre: %w", errorObtenido)
		}
	}
	return nil
}
