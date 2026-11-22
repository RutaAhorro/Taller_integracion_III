// Package discord reúne el comportamiento del bot frente a Discord: comandos
// de texto, menú e interacciones de scraping, notificaciones de GitHub y el
// servidor HTTP que recibe los webhooks. Se organiza por tema: comandos.go,
// scraping.go, github.go y webhook.go.
package discord

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/equipo/bot_discord/internal/config"
	"github.com/equipo/bot_discord/internal/scraping"
)

// ===== Manejador =====

// Manejador agrupa la sesión de Discord, el cliente de Control y el estado que
// comparten los comandos, las interacciones y los webhooks. Sustituye el
// estado global del proceso para que cada instancia (por ejemplo, en pruebas)
// tenga el suyo.
type Manejador struct {
	configuracion config.Configuracion
	sesion        *discordgo.Session
	control       *scraping.Cliente
	// contexto se cancela al apagar el proceso para detener los seguimientos
	// de trabajos que siguen corriendo en segundo plano.
	contexto context.Context

	// Agrupación de solicitudes de revisión: GitHub envía un evento por
	// revisor, y se prefiere un solo aviso por PR (ver notificarRevisionSolicitada).
	mutexRevisiones       sync.Mutex
	temporizadoresPR      map[int]*time.Timer
	revisoresPendientesPR map[int][]string
}

// NuevoManejador crea el manejador. Falla si falta la sesión o el cliente de
// Control porque el bot no puede operar sin ellos.
func NuevoManejador(contexto context.Context, configuracion config.Configuracion, sesion *discordgo.Session, control *scraping.Cliente) (*Manejador, error) {
	if sesion == nil {
		return nil, errors.New("la sesión de Discord es obligatoria")
	}
	if control == nil {
		return nil, errors.New("el cliente de Control es obligatorio")
	}
	return &Manejador{
		configuracion:         configuracion,
		sesion:                sesion,
		control:               control,
		contexto:              contexto,
		temporizadoresPR:      make(map[int]*time.Timer),
		revisoresPendientesPR: make(map[int][]string),
	}, nil
}

// Los manejadores de eventos reciben la sesión que exige discordgo pero usan
// la del Manejador, que es la misma en producción y la que reemplazan las pruebas.

// Registrar conecta los manejadores de mensajes e interacciones a la sesión y
// declara la intención de recibir mensajes de servidores.
func (manejador *Manejador) Registrar() {
	manejador.sesion.AddHandler(manejador.ManejarMensaje)
	manejador.sesion.AddHandler(manejador.ManejarInteraccion)
	manejador.sesion.Identify.Intents = discordgo.IntentsGuildMessages
}

// ===== Envío de mensajes =====

// enviarMensaje publica texto en un canal y registra el fallo, porque una
// notificación perdida no debe interrumpir al resto del procesamiento.
func (manejador *Manejador) enviarMensaje(canalID, mensaje string) {
	if _, errorObtenido := manejador.sesion.ChannelMessageSend(canalID, mensaje); errorObtenido != nil {
		log.Printf("No se pudo enviar el mensaje al canal %s: %v", canalID, errorObtenido)
	}
}
