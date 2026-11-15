// Package control compone trabajos y entrega los lotes al servicio Catálogo.
package control

import (
	"context"
	"errors"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/handlers"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/middleware"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/routes"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"os"
	"sync"
	"time"
)

type Module struct {
	baseDatos     *gorm.DB
	manejador     *handlers.ScraperHandler
	autenticacion gin.HandlerFunc
	recuperar     func(context.Context)
	cola          *services.QueueService
	cerrar        func() error
	grupoEspera   sync.WaitGroup
}

func New(baseDatos *gorm.DB, clienteRedis *redis.Client) (*Module, error) {
	token := os.Getenv("CATALOGO_INTERNAL_TOKEN")
	tokenServicio := os.Getenv("SCRAPING_SERVICE_TOKEN")
	if errorObtenido := middleware.ValidarTokenServicio(tokenServicio, token); errorObtenido != nil {
		return nil, errorObtenido
	}
	tiempoEspera := 10 * time.Second
	if crudo := os.Getenv("CATALOGO_INGEST_TIMEOUT"); crudo != "" {
		var errorObtenido error
		tiempoEspera, errorObtenido = time.ParseDuration(crudo)
		if errorObtenido != nil {
			return nil, errors.New("CATALOGO_INGEST_TIMEOUT debe ser una duración válida")
		}
	}
	trabajos := repositories.NewTrabajoRepository(baseDatos)
	ingesta, errorObtenido := repositories.NewCatalogoClient(os.Getenv("CATALOGO_INTERNAL_URL"), token, tiempoEspera)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	servicio := services.NewScraperService(trabajos, ingesta, repositories.NewEntregaRepository(baseDatos))
	publicador := repositories.NewJobPublisher(clienteRedis, baseDatos)
	cola := services.NewQueueService(repositories.NewEjecucionRepository(baseDatos), publicador)
	manejador := handlers.NewScraperHandler(servicio, clienteRedis)
	manejador.SetQueue(cola)
	return &Module{baseDatos: baseDatos, manejador: manejador, autenticacion: middleware.RequireServiceToken(tokenServicio), cola: cola, cerrar: publicador.Close, recuperar: servicio.(interface{ Recuperar(context.Context) }).Recuperar}, nil
}

func (modulo *Module) VerifyStorage(contexto context.Context) error {
	filas, errorObtenido := modulo.baseDatos.WithContext(contexto).Raw("SELECT cadena_nombre FROM scraper.trabajos_scraper LIMIT 0").Rows()
	if errorObtenido != nil {
		return errors.New("aplicar 06_propiedad_permisos.sql y permisos de coordinación antes de iniciar la API")
	}
	filas.Close()
	for _, tabla := range []string{"scraper.trabajos_scraper", "scraper.entregas_ingesta", "scraper.entregas_trabajos"} {
		filas, errorObtenido := modulo.baseDatos.WithContext(contexto).Raw("SELECT * FROM " + tabla + " LIMIT 0").Rows()
		if errorObtenido != nil {
			return errors.New("aplicar 05_recuperacion_scraping.sql y permisos de coordinación antes de iniciar la API")
		}
		filas.Close()
		var permitido bool
		if errorObtenido := modulo.baseDatos.WithContext(contexto).Raw(`SELECT
            has_table_privilege(current_user, ?, 'SELECT') AND
            has_table_privilege(current_user, ?, 'INSERT') AND
            has_table_privilege(current_user, ?, 'UPDATE')`, tabla, tabla, tabla).Scan(&permitido).Error; errorObtenido != nil || !permitido {
			return errors.New("faltan permisos runtime de coordinación; revisar 06_propiedad_permisos.sql")
		}
	}
	var puedeEliminar bool
	if errorObtenido := modulo.baseDatos.WithContext(contexto).Raw("SELECT has_table_privilege(current_user, 'scraper.entregas_trabajos', 'DELETE')").Scan(&puedeEliminar).Error; errorObtenido != nil || !puedeEliminar {
		return errors.New("conceder DELETE en scraper.entregas_trabajos al rol de coordinación antes de iniciar la API")
	}
	return nil
}

// Sólo main inicia los loops; construir el router no crea goroutines.
func (modulo *Module) Start(contexto context.Context) {
	modulo.grupoEspera.Add(2)
	go func() {
		defer modulo.grupoEspera.Done()
		temporizador := time.NewTicker(time.Second)
		defer temporizador.Stop()
		for {
			select {
			case <-contexto.Done():
				return
			case <-temporizador.C:
				intento, cancelar := context.WithTimeout(contexto, 65*time.Second)
				modulo.recuperar(intento)
				cancelar()
			}
		}
	}()
	go func() {
		defer modulo.grupoEspera.Done()
		temporizador := time.NewTicker(5 * time.Second)
		defer temporizador.Stop()
		defer modulo.cerrar()
		for {
			select {
			case <-contexto.Done():
				return
			case <-temporizador.C:
				intento, cancelar := context.WithTimeout(contexto, 10*time.Second)
				modulo.cola.Recuperar(intento)
				cancelar()
			}
		}
	}()
}

// Wait espera que terminen los dos loops y cierre el cliente del publisher.
func (modulo *Module) Wait(contexto context.Context) error {
	terminado := make(chan struct{})
	go func() { modulo.grupoEspera.Wait(); close(terminado) }()
	select {
	case <-terminado:
		return nil
	case <-contexto.Done():
		return contexto.Err()
	}
}

func (modulo *Module) RegisterRoutes(grupo *gin.RouterGroup) {
	routes.RegistrarRutasScraper(grupo, modulo.manejador, modulo.autenticacion)
}
