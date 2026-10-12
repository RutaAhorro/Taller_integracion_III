package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
)

// RegistrarRutasScraper configura los endpoints de ingesta de datos y ciclo de vida de arañas
// bajo el prefijo /api/v1/scraper. auth se aplica a todo el grupo (token de servicio).
func RegistrarRutasScraper(rg *gin.RouterGroup, scraperHandler *handlers.ScraperHandler, auth ...gin.HandlerFunc) {
	scraper := rg.Group("/scraper", auth...)
	{
		// Auditoría y ciclo de vida de arañas
		scraper.POST("/trabajos", scraperHandler.IniciarTrabajo)
		scraper.GET("/trabajos/:id", scraperHandler.ObtenerTrabajo)
		scraper.PUT("/trabajos/:id/finalizar", scraperHandler.FinalizarTrabajo)
		scraper.POST("/trabajos/:id/ejecutar", scraperHandler.EjecutarTrabajo)

		// Ingesta de productos (con sesión de trabajo o directa)
		scraper.POST("/trabajos/:id/productos", scraperHandler.IngestarProductosConTrabajo)
		scraper.POST("/productos", scraperHandler.IngestarProductosDirecto)
	}
}
