package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/docs"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/control"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/database"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/middleware"
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

func initRedis() error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return errors.New("REDIS_URL es obligatorio")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return errors.New("REDIS_URL debe ser una URL Redis válida")
	}
	opt.MaxRetries = -1
	opt.DialTimeout = time.Second
	opt.ReadTimeout = time.Second
	opt.WriteTimeout = time.Second

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
	return nil
}

func setupRouter() (*gin.Engine, error) {
	r, _, err := setupApplication()
	return r, err
}

func setupApplication() (*gin.Engine, *control.Module, error) {
	r := gin.New()

	// Logger por defecto y Middleware Global de Errores
	r.Use(middleware.Observe(slog.Default()))
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.BoundedRequest())

	// Habilitar detección de métodos HTTP no permitidos
	r.HandleMethodNotAllowed = true

	// Manejadores estandarizados para 404 (NoRoute) y 405 (NoMethod)
	r.NoRoute(middleware.NotFoundHandler())
	r.NoMethod(middleware.MethodNotAllowedHandler())

	// No se acepta IP/identidad de headers ni un origen ajeno a la allowlist.
	_ = r.SetTrustedProxies(nil)
	allowed := strings.Split(os.Getenv("SCRAPING_CORS_ALLOWED_ORIGINS"), ",")
	for i, origin := range allowed {
		allowed[i] = strings.TrimSpace(origin)
		if allowed[i] == "" {
			continue
		}
		u, err := url.Parse(allowed[i])
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return nil, nil, errors.New("SCRAPING_CORS_ALLOWED_ORIGINS debe contener orígenes HTTP(S) exactos")
		}
	}
	r.Use(middleware.CORS(allowed))

	r.GET("/", RootHandler)
	r.GET("/_api/live", LivenessHandler) // Alias de compatibilidad de la imagen SUP-272.
	r.GET("/_scraping/live", LivenessHandler)
	r.GET("/_scraping/ready", ReadinessHandler)
	r.GET("/health", HealthHandler)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Grupo de rutas de la API v1
	v1 := r.Group("/api/v1")
	v1.GET("/health", HealthHandler)
	coordinacion, err := control.New(ScrapingDB, RDB)
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
// @Router       /api/v1/health [get]
func HealthHandler(c *gin.Context) { writeHealth(c, "api-gateway") }

// ReadinessHandler comprueba SQL; Redis es sólo diagnóstico. Probe privado.
func ReadinessHandler(c *gin.Context) { writeHealth(c, "control-scraping") }

func writeHealth(c *gin.Context, service string) {
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
		"service":   service,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": gin.H{
			"database":          dbStatus,
			"scraping_database": scrapingStatus,
			"redis":             redisStatus,
		},
	})
}

// LivenessHandler no consulta dependencias. Probe privado.
func LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "control-scraping"})
}

// @title           Control de Scraping
// @version         1.0
// @description     Contrato del servicio de coordinación. Identidad y Catálogo se atienden por Gateway y no forman parte de esta especificación.
// @host            localhost:8080
// @BasePath        /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("scraping_startup_failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Intentar cargar .env, pero si no existe (ej. Producción/Docker), continuar usando variables de entorno inyectadas
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	// Permitir que Swagger se adapte dinámicamente a cualquier dominio o IP
	docs.SwaggerInfo.Host = ""

	connectionCtx, connectionCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := initDB(connectionCtx); err != nil {
		connectionCancel()
		return err
	}
	connectionCancel()
	defer database.Close(ScrapingDB)
	if err := initRedis(); err != nil {
		return err
	}
	defer RDB.Close()

	r, coordinacion, err := setupApplication()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	storageCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := coordinacion.VerifyStorage(storageCtx); err != nil {
		cancel()
		return err
	}
	cancel()
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	defer func() {
		runtimeCancel()
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = coordinacion.Wait(waitCtx)
	}()
	coordinacion.Start(runtimeCtx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("PORT debe estar entre 1 y 65535")
	}
	log.Printf("Iniciando servidor en el puerto %s...", port)
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 * 1024}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			return errors.New("no se pudo iniciar el listener HTTP")
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
		runtimeCancel()
		if err := coordinacion.Wait(shutdown); err != nil {
			slog.Error("scraping_shutdown_timeout")
		}
	}
	return nil
}
