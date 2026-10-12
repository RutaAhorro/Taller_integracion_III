package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func getApiURL() string {
	url := os.Getenv("SCRAPER_API_BASE_URL")
	if url == "" {
		url = "http://api:8080/api/v1/scraper/trabajos" // Fallback para k8s
	}
	return url
}

// Control de scraping exige el token de servicio en todas sus rutas.
var scraperHTTP = &http.Client{Timeout: 15 * time.Second}

func scraperRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := os.Getenv("SCRAPING_SERVICE_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return scraperHTTP.Do(req)
}

func scraperPost(url string, body io.Reader) (*http.Response, error) {
	return scraperRequest(http.MethodPost, url, body)
}

func scraperGet(url string) (*http.Response, error) {
	return scraperRequest(http.MethodGet, url, nil)
}

func sendScraperMenu(s *discordgo.Session, channelID string) {
	msg := &discordgo.MessageSend{
		Content: "🤖 **Menú de Scraping**\nSelecciona el supermercado que deseas scrapear:",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Jumbo",
						Style:    discordgo.SuccessButton,
						CustomID: "scrape_1_jumbo_rsc",
						Emoji: &discordgo.ComponentEmoji{Name: "🐘"},
					},
					discordgo.Button{
						Label:    "Lider",
						Style:    discordgo.PrimaryButton,
						CustomID: "scrape_2_lider_rsc",
						Emoji: &discordgo.ComponentEmoji{Name: "🛒"},
					},
					discordgo.Button{
						Label:    "Santa Isabel",
						Style:    discordgo.DangerButton,
						CustomID: "scrape_4_santa_isabel_rsc",
						Emoji: &discordgo.ComponentEmoji{Name: "🛍️"},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "A Cuenta",
						Style:    discordgo.SuccessButton,
						CustomID: "scrape_5_acuenta_rsc",
						Emoji: &discordgo.ComponentEmoji{Name: "📉"},
					},
					discordgo.Button{
						Label:    "Cugat",
						Style:    discordgo.SecondaryButton,
						CustomID: "scrape_6_cugat_rsc",
						Emoji: &discordgo.ComponentEmoji{Name: "🏪"},
					},
					discordgo.Button{
						Label:    "Todos a la vez",
						Style:    discordgo.PrimaryButton,
						CustomID: "scrape_0_todos",
						Emoji: &discordgo.ComponentEmoji{Name: "🚀"},
					},
				},
			},
		},
	}
	_, err := s.ChannelMessageSendComplex(channelID, msg)
	if err != nil {
		log.Printf("Error enviando menú: %v", err)
	}
}

func handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	customID := i.MessageComponentData().CustomID

	if strings.HasPrefix(customID, "refresh_") {
		handleRefresh(s, i)
		return
	}

	if strings.HasPrefix(customID, "scrape_") {
		handleScrapeStart(s, i)
		return
	}
}

func handleScrapeStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Responder inmediatamente para evitar el "This interaction failed"
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	customID := i.MessageComponentData().CustomID
	
	// Formato: scrape_<cadenaID>_<spider>
	parts := strings.SplitN(strings.TrimPrefix(customID, "scrape_"), "_", 2)
	if len(parts) != 2 {
		return
	}
	cadenaIDStr := parts[0]
	spider := parts[1]
	
	var cadenaID int
	fmt.Sscanf(cadenaIDStr, "%d", &cadenaID)

	nombre := spider // Default
	switch cadenaID {
	case 1: nombre = "Jumbo"
	case 2: nombre = "Lider"
	case 4: nombre = "Santa Isabel"
	case 5: nombre = "A Cuenta"
	case 6: nombre = "Cugat"
	case 0: nombre = "Todos los Supermercados"
	}

	userID := i.Member.User.ID


	// Si eligió "Todos", disparamos cada scraper individualmente
	if cadenaID == 0 {
		cadenas := []struct{ id int; spider string; nombre string }{
			{1, "jumbo_rsc", "Jumbo"},
			{2, "lider_rsc", "Lider"},
			{4, "santa_isabel_rsc", "Santa Isabel"},
			{5, "acuenta_rsc", "A Cuenta"},
			{6, "cugat_rsc", "Cugat"},
		}

		var trabajosExitosos []struct {
			id     string
			nombre string
		}

		for _, c := range cadenas {
			createPayload := map[string]interface{}{
				"cadena_id": c.id,
				"disparado_por_usuario_id": userID,
			}
			createBytes, _ := json.Marshal(createPayload)
			resp, err := scraperPost(getApiURL(), bytes.NewBuffer(createBytes))
			if err != nil { continue }
			
			if resp.StatusCode == http.StatusCreated {
				var createResp map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&createResp)
				trabajoID := createResp["id"].(string)
				resp.Body.Close()

				startPayload := map[string]interface{}{ "spider": c.spider }
				startBytes, _ := json.Marshal(startPayload)
				respStart, errStart := scraperPost(fmt.Sprintf("%s/%s/ejecutar", getApiURL(), trabajoID), bytes.NewBuffer(startBytes))
				if errStart == nil && respStart.StatusCode == http.StatusAccepted {
					trabajosExitosos = append(trabajosExitosos, struct{ id string; nombre string }{id: trabajoID, nombre: c.nombre})
					go monitorJobAndNotify(s, i.ChannelID, trabajoID, c.nombre, userID)
				}
				if respStart != nil { respStart.Body.Close() }
			} else {
				resp.Body.Close()
			}
		}

		msgContent := fmt.Sprintf("🚀 **¡Scraping Masivo Iniciado!**\nSe han encolado `%d` trabajos de scraping en Redis. Puedes revisar el estado individual con los botones de abajo:\n", len(trabajosExitosos))
		var buttons []discordgo.MessageComponent

		for _, t := range trabajosExitosos {
			msgContent += fmt.Sprintf("• **%s**: `%s`\n", t.nombre, t.id)
			buttons = append(buttons, discordgo.Button{
				Label:    "Refresh " + t.nombre,
				Style:    discordgo.SecondaryButton,
				CustomID: "refresh_" + t.id,
				Emoji: &discordgo.ComponentEmoji{
					Name: "🔄",
				},
			})
		}

		// Discord permite hasta 5 componentes (botones) por ActionRow. Como tenemos 5, calza perfecto en 1 fila.
		// Si hubieran más, habría que dividir el array de botones en múltiples ActionRows.
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: msgContent,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		})
		return
	}

	// 1. Crear el Trabajo Normal (Individual)
	createPayload := map[string]interface{}{
		"cadena_id": cadenaID,
		"disparado_por_usuario_id": userID,
	}
	createBytes, _ := json.Marshal(createPayload)

	resp, err := scraperPost(getApiURL(), bytes.NewBuffer(createBytes))
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Error de red al crear trabajo para %s: %v", nombre, err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Error al crear trabajo para %s. Status: %d\nBody: %s", nombre, resp.StatusCode, string(bodyBytes)),
		})
		return
	}

	var createResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResp)
	trabajoID := createResp["id"].(string)

	// 2. Iniciar el Scraper
	startPayload := map[string]interface{}{
		"spider": spider,
	}
	startBytes, _ := json.Marshal(startPayload)

	respStart, err := scraperPost(fmt.Sprintf("%s/%s/ejecutar", getApiURL(), trabajoID), bytes.NewBuffer(startBytes))
	if err != nil {
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Error de red al iniciar spider %s: %v", spider, err),
		})
		return
	}
	defer respStart.Body.Close()

	if respStart.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(respStart.Body)
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("❌ Error al ejecutar spider %s. Status: %d\nBody: %s", spider, respStart.StatusCode, string(bodyBytes)),
		})
		return
	}

	// Éxito con botón de refresh
	s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("🚀 **¡Scraping de %s Iniciado!**\nID del trabajo: `%s`\nLa araña `%s` está recolectando productos.", nombre, trabajoID, spider),
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Actualizar Estado",
						Style:    discordgo.SecondaryButton,
						CustomID: "refresh_" + trabajoID,
						Emoji: &discordgo.ComponentEmoji{
							Name: "🔄",
						},
					},
				},
			},
		},
	})

	// Lanzar Goroutine para notificar cuando termine
	go monitorJobAndNotify(s, i.ChannelID, trabajoID, nombre, userID)
}

func monitorJobAndNotify(s *discordgo.Session, channelID, trabajoID, nombre, userID string) {
	for {
		time.Sleep(5 * time.Second)

		resp, err := scraperGet(fmt.Sprintf("%s/%s", getApiURL(), trabajoID))
		if err != nil {
			continue // reintentar
		}
		
		var estadoResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&estadoResp)
		resp.Body.Close()

		estado, ok := estadoResp["estado"].(string)
		if !ok {
			continue
		}

		if estado == "completado" || estado == "fallido" {
			extraidos := 0
			if val, ok := estadoResp["elementos_extraidos"].(float64); ok {
				extraidos = int(val)
			}
			
			emoji := "✅"
			resultado := "completado con éxito"
			if estado == "fallido" {
				emoji = "❌"
				resultado = "fallado brutalmente"
			}

			mensajeNotificacion := fmt.Sprintf("%s <@%s>, ¡Te aviso que tu scraping de **%s** ha %s!\n", emoji, userID, nombre, resultado)
			if estado == "completado" {
				mensajeNotificacion += fmt.Sprintf("📊 Se extrajeron **%d** productos y ya están en la base de datos.", extraidos)
			} else if errMsg, ok := estadoResp["registro_errores"].(string); ok && errMsg != "" {
				mensajeNotificacion += fmt.Sprintf("🛑 Motivo: `%s`", errMsg)
			}

			s.ChannelMessageSend(channelID, mensajeNotificacion)
			break
		}
	}
}

func handleRefresh(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	trabajoID := strings.TrimPrefix(customID, "refresh_")

	// Obtener estado
	resp, err := scraperGet(fmt.Sprintf("%s/%s", getApiURL(), trabajoID))
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Error de red al consultar el estado",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	defer resp.Body.Close()

	var estadoResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&estadoResp)

	estado := estadoResp["estado"].(string)
	extraidos := 0
	if val, ok := estadoResp["elementos_extraidos"].(float64); ok {
		extraidos = int(val)
	}
	cadena := estadoResp["cadena_nombre"].(string)

	emojiEstado := "⏳"
	if estado == "completado" {
		emojiEstado = "✅"
	} else if estado == "fallido" {
		emojiEstado = "❌"
	}

	// Generar barrita de progreso visual (simulada basada en un estimado, digamos 5000)
	// Si no sabemos el total, mostramos los extraidos en texto
	progresoVisual := ""
	if estado == "en_progreso" {
		progresoVisual = fmt.Sprintf("\n📊 Progreso: **%d productos extraídos**", extraidos)
	}

	nuevoMensaje := fmt.Sprintf("%s **Estado del Scraping: %s**\nID: `%s`\nSupermercado: **%s**\nEstado: `%s`%s\nÚltima actualización: <t:%d:R>", 
		emojiEstado, cadena, trabajoID, cadena, estado, progresoVisual, time.Now().Unix())

	// Mantener el botón si sigue en progreso
	var components []discordgo.MessageComponent
	if estado == "en_progreso" || estado == "encolado" {
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Actualizar Estado",
						Style:    discordgo.SecondaryButton,
						CustomID: customID,
						Emoji: &discordgo.ComponentEmoji{
							Name: "🔄",
						},
					},
				},
			},
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: nuevoMensaje,
			Components: components,
		},
	})
}
