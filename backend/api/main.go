package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/docs"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/database"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping"
	"github.com/danirc2024/Taller_integracion_III/backend/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

var ScrapingDB *gorm.DB
var RDB *redis.Client

func initDB(ctx context.Context) error {
	cfg, err := database.Load()
	if err != nil {
		return err
	}
	ScrapingDB, err = database.Open(ctx, cfg.ScrapingURL)
	return err
}

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	var opt *redis.Options
	var err error

	if redisURL != "" {
		opt, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("Advertencia: No se pudo parsear REDIS_URL (%v), usando configuración por defecto", err)
			opt = &redis.Options{Addr: "localhost:6379"}
		}
	} else {
		redisHost := os.Getenv("REDIS_HOST")
		if redisHost == "" {
			redisHost = "localhost"
		}
		redisPort := os.Getenv("REDIS_PORT")
		if redisPort == "" {
			redisPort = "6379"
		}
		opt = &redis.Options{
			Addr: redisHost + ":" + redisPort,
		}
	}

	// Sin esta opción go-redis usa sus timeouts de socket aunque venza el
	// contexto del probe. Los comandos deben respetar el presupuesto HTTP.
	opt.ContextTimeoutEnabled = true
	RDB = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Printf("Aviso: No se pudo conectar a Redis (%v). La cola SQL conservará las ejecuciones hasta recuperar Redis.", err)
	} else {
		log.Println("Conexión a Redis establecida exitosamente.")
	}
}

func setupRouter() (*gin.Engine, error) {
	r, _, err := setupApplication()
	return r, err
}

func setupApplication() (*gin.Engine, *scraping.Module, error) {
	r := gin.New()

	// Logger por defecto y Middleware Global de Errores
	r.Use(gin.Logger())
	r.Use(middleware.ErrorHandler())

	// Habilitar detección de métodos HTTP no permitidos
	r.HandleMethodNotAllowed = true

	// Manejadores estandarizados para 404 (NoRoute) y 405 (NoMethod)
	r.NoRoute(middleware.NotFoundHandler())
	r.NoMethod(middleware.MethodNotAllowedHandler())

	// Configuración de CORS dinámica
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/", RootHandler)
	r.GET("/_api/live", LivenessHandler)
	r.GET("/health", HealthHandler)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Grupo de rutas de la API v1
	v1 := r.Group("/api/v1")
	v1.GET("/health", HealthHandler)
	coordinacion, err := scraping.New(ScrapingDB, RDB)
	if err != nil {
		return nil, nil, err
	}
	coordinacion.RegisterRoutes(v1)

	return r, coordinacion, nil
}

// RootHandler godoc
// @Summary      Muestra mensaje de bienvenida
// @Description  Endpoint raíz para comprobar el estado de la API
// @Tags         root
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       / [get]
func RootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Bienvenido a la API Gateway de Supermercados.",
		"docs":    "Visita /swagger/index.html para ver el autocompletado Swagger UI.",
	})
}

// HealthHandler godoc
// @Summary      Comprobación de salud del servicio (Healthcheck)
// @Description  Readiness: requiere la conexión PostgreSQL de coordinación; Redis es diagnóstico y puede degradarse sin retirar la API.
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /health [get]
func HealthHandler(c *gin.Context) {
	// Un presupuesto común mantiene el probe por debajo del timeout de K8s (3s).
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	checkDB := func(db *gorm.DB) string {
		if db == nil {
			return "not_configured"
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(ctx) != nil {
			return "error"
		}
		return "ok"
	}
	scrapingStatus := checkDB(ScrapingDB)
	dbStatus := "ok"
	if scrapingStatus != "ok" {
		dbStatus = "error"
		if scrapingStatus == "not_configured" {
			dbStatus = "not_configured"
		}
	}

	redisStatus := "ok"
	if RDB != nil {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := RDB.Ping(ctx).Err(); err != nil {
			redisStatus = "error"
		}
	} else {
		redisStatus = "not_configured"
	}

	status, state := http.StatusOK, "ok"
	if dbStatus != "ok" {
		status, state = http.StatusServiceUnavailable, "error"
	}
	c.JSON(status, gin.H{
		"status":    state,
		"service":   "api-gateway",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": gin.H{
			"database":          dbStatus,
			"scraping_database": scrapingStatus,
			"redis":             redisStatus,
		},
	})
}

// LivenessHandler godoc
// @Summary      Comprueba que el proceso responde
// @Description  Liveness independiente de PostgreSQL y Redis.
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /_api/live [get]
func LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api-gateway"})
}

// @title           API de Coordinación de Scraping
// @version         1.0
// @description     Contrato del servicio de coordinación. Identidad y Catálogo se atienden por Gateway y no forman parte de esta especificación.
// @host            localhost:8080
// @BasePath        /
func main() {
	// Intentar cargar .env, pero si no existe (ej. Producción/Docker), continuar usando variables de entorno inyectadas
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	// Permitir que Swagger se adapte dinámicamente a cualquier dominio o IP
	docs.SwaggerInfo.Host = ""

	connectionCtx, connectionCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := initDB(connectionCtx); err != nil {
		connectionCancel()
		log.Fatal(err)
	}
	connectionCancel()
	defer database.Close(ScrapingDB)
	initRedis()

	r, coordinacion, err := setupApplication()
	if err != nil {
		log.Fatalf("Configuración de scraping inválida: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	storageCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := coordinacion.VerifyStorage(storageCtx); err != nil {
		cancel()
		log.Fatal(err)
	}
	cancel()
	coordinacion.Start(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor en el puerto %s...", port)
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			log.Printf("Error al iniciar el servidor: %v", err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}
}
