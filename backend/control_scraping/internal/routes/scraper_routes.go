package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
)

// ===== Registro de rutas de scraping =====

// RegistrarRutasScraper configura los endpoints de ingesta de datos y ciclo de vida de arañas
// bajo el prefijo /api/v1/scraper. auth se aplica a todo el grupo (token de servicio).
func RegistrarRutasScraper(grupoRutas *gin.RouterGroup, manejadorScraper *handlers.ScraperHandler, autenticacion ...gin.HandlerFunc) {
	rutasScraper := grupoRutas.Group("/scraper", autenticacion...)
	{
		// Auditoría y ciclo de vida de arañas
		rutasScraper.POST("/trabajos", manejadorScraper.IniciarTrabajo)
		rutasScraper.GET("/trabajos/:id", manejadorScraper.ObtenerTrabajo)
		rutasScraper.PUT("/trabajos/:id/finalizar", manejadorScraper.FinalizarTrabajo)
		rutasScraper.POST("/trabajos/:id/ejecutar", manejadorScraper.EjecutarTrabajo)

		// Ingesta de productos (con sesión de trabajo o directa)
		rutasScraper.POST("/trabajos/:id/productos", manejadorScraper.IngestarProductosConTrabajo)
		rutasScraper.POST("/productos", manejadorScraper.IngestarProductosDirecto)
	}
}
