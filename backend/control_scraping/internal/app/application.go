// Package app compone el servicio HTTP de coordinación y administra su ciclo de vida.
package app

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

// openScrapingDatabase carga y abre la conexión PostgreSQL propia de Control.
func openScrapingDatabase(ctx context.Context) (*gorm.DB, error) {
	configuracion, err := database.Load()
	if err != nil {
		return nil, err
	}
	return database.Open(ctx, configuracion.ScrapingURL)
}

// NewRedisClientFromEnvironment valida REDIS_URL y crea el cliente de Redis.
func NewRedisClientFromEnvironment() (*redis.Client, error) {
	direccionRedis := os.Getenv("REDIS_URL")
	if direccionRedis == "" {
		return nil, errors.New("REDIS_URL es obligatorio")
	}
	opciones, err := redis.ParseURL(direccionRedis)
	if err != nil {
		return nil, errors.New("REDIS_URL debe ser una URL Redis válida")
	}
	opciones.MaxRetries = -1
	opciones.DialTimeout = time.Second
	opciones.ReadTimeout = time.Second
	opciones.WriteTimeout = time.Second

	// Sin esta opción go-redis usa sus timeouts de socket aunque venza el
	// contexto del probe. Los comandos deben respetar el presupuesto HTTP.
	opciones.ContextTimeoutEnabled = true
	clienteRedis := redis.NewClient(opciones)

	contexto, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()

	if err := clienteRedis.Ping(contexto).Err(); err != nil {
		log.Print("Aviso: Redis no está disponible. La cola SQL conservará las ejecuciones hasta recuperar Redis.")
	} else {
		log.Println("Conexión a Redis establecida exitosamente.")
	}
	return clienteRedis, nil
}

// NewApplication compone el router y los loops de coordinación con sus dependencias.
func NewApplication(baseDatos *gorm.DB, clienteRedis *redis.Client) (*gin.Engine, *control.Module, error) {
	router := gin.New()

	// Logger por defecto y Middleware Global de Errores
	router.Use(middleware.Observe(slog.Default()))
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.BoundedRequest())

	// Habilitar detección de métodos HTTP no permitidos
	router.HandleMethodNotAllowed = true

	// Manejadores estandarizados para 404 (NoRoute) y 405 (NoMethod)
	router.NoRoute(middleware.NotFoundHandler())
	router.NoMethod(middleware.MethodNotAllowedHandler())

	// No se acepta IP/identidad de headers ni un origen ajeno a la allowlist.
	_ = router.SetTrustedProxies(nil)
	origenesPermitidos := strings.Split(os.Getenv("SCRAPING_CORS_ALLOWED_ORIGINS"), ",")
	for indice, origen := range origenesPermitidos {
		origenesPermitidos[indice] = strings.TrimSpace(origen)
		if origenesPermitidos[indice] == "" {
			continue
		}
		origenURL, err := url.Parse(origenesPermitidos[indice])
		if err != nil || origenURL.Hostname() == "" || (origenURL.Scheme != "http" && origenURL.Scheme != "https") || origenURL.User != nil || origenURL.Path != "" || origenURL.RawQuery != "" || origenURL.ForceQuery || origenURL.Fragment != "" {
			return nil, nil, errors.New("SCRAPING_CORS_ALLOWED_ORIGINS debe contener orígenes HTTP(S) exactos")
		}
	}
	router.Use(middleware.CORS(origenesPermitidos))

	router.GET("/", RootHandler)
	router.GET("/_api/live", LivenessHandler) // Alias de compatibilidad de la imagen SUP-272.
	router.GET("/_scraping/live", LivenessHandler)
	router.GET("/_scraping/ready", HealthHandler(baseDatos, clienteRedis, "control-scraping"))
	router.GET("/health", HealthHandler(baseDatos, clienteRedis, "api-gateway"))

	// Swagger describe todo el contrato de coordinación. Sólo se publica si se
	// habilita explícitamente (Compose local y CI); en Kubernetes queda apagado.
	switch os.Getenv("SCRAPING_SWAGGER_ENABLED") {
	case "true":
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	case "", "false":
	default:
		return nil, nil, errors.New("SCRAPING_SWAGGER_ENABLED debe ser true o false")
	}

	// Grupo de rutas de la API v1
	v1 := router.Group("/api/v1")
	v1.GET("/health", HealthHandler(baseDatos, clienteRedis, "api-gateway"))
	coordinacion, err := control.New(baseDatos, clienteRedis)
	if err != nil {
		return nil, nil, err
	}
	coordinacion.RegisterRoutes(v1)

	return router, coordinacion, nil
}

// RootHandler godoc
// @Summary      Muestra mensaje de bienvenida
// @Description  Endpoint raíz para comprobar el estado de la API
// @Tags         root
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       / [get]
func RootHandler(contextoHTTP *gin.Context) {
	contextoHTTP.JSON(http.StatusOK, gin.H{
		"message": "Bienvenido a la API Gateway de Supermercados.",
		"docs":    "Visita /swagger/index.html para ver el autocompletado Swagger UI.",
	})
}

// HealthHandler genera una comprobación de readiness para las dependencias recibidas.
func HealthHandler(baseDatos *gorm.DB, clienteRedis *redis.Client, servicio string) gin.HandlerFunc {
	return func(contextoHTTP *gin.Context) {
		writeHealth(contextoHTTP, baseDatos, clienteRedis, servicio)
	}
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
// writeHealth consulta SQL para readiness y Redis sólo como diagnóstico.
func writeHealth(contextoHTTP *gin.Context, baseDatos *gorm.DB, clienteRedis *redis.Client, servicio string) {
	// Un presupuesto común mantiene el probe por debajo del timeout de K8s (3s).
	contexto, cancelar := context.WithTimeout(contextoHTTP.Request.Context(), 2*time.Second)
	defer cancelar()
	verificarBaseDatos := func(conexion *gorm.DB) string {
		if conexion == nil {
			return "not_configured"
		}
		contextoBaseDatos, cancelarBaseDatos := context.WithTimeout(contexto, time.Second)
		defer cancelarBaseDatos()
		conexionSQL, err := conexion.DB()
		if err != nil || conexionSQL.PingContext(contextoBaseDatos) != nil {
			return "error"
		}
		return "ok"
	}
	estadoScraping := verificarBaseDatos(baseDatos)
	estadoBaseDatos := "ok"
	if estadoScraping != "ok" {
		estadoBaseDatos = "error"
		if estadoScraping == "not_configured" {
			estadoBaseDatos = "not_configured"
		}
	}

	estadoRedis := "ok"
	if clienteRedis != nil {
		contextoRedis, cancelarRedis := context.WithTimeout(contexto, 500*time.Millisecond)
		defer cancelarRedis()
		if err := clienteRedis.Ping(contextoRedis).Err(); err != nil {
			estadoRedis = "error"
		}
	} else {
		estadoRedis = "not_configured"
	}

	estadoHTTP, estadoGeneral := http.StatusOK, "ok"
	if estadoBaseDatos != "ok" {
		estadoHTTP, estadoGeneral = http.StatusServiceUnavailable, "error"
	}
	contextoHTTP.JSON(estadoHTTP, gin.H{
		"status":    estadoGeneral,
		"service":   servicio,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": gin.H{
			"database":          estadoBaseDatos,
			"scraping_database": estadoScraping,
			"redis":             estadoRedis,
		},
	})
}

// LivenessHandler no consulta dependencias. Probe privado.
func LivenessHandler(contextoHTTP *gin.Context) {
	contextoHTTP.JSON(http.StatusOK, gin.H{"status": "ok", "service": "control-scraping"})
}

// @title           Control de Scraping
// @version         1.0
// @description     Contrato del servicio de coordinación. Identidad y Catálogo se atienden por Gateway y no forman parte de esta especificación.
// @host            localhost:8080
// @BasePath        /
// Run carga configuración, abre dependencias y mantiene el servidor activo.
func Run() error {
	return run()
}

// run administra el inicio, apagado y liberación de recursos del servicio.
func run() error {
	// Intentar cargar .env, pero si no existe (ej. Producción/Docker), continuar usando variables de entorno inyectadas
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	// Permitir que Swagger se adapte dinámicamente a cualquier dominio o IP
	docs.SwaggerInfo.Host = ""

	contextoConexion, cancelarConexion := context.WithTimeout(context.Background(), 5*time.Second)
	baseDatos, err := openScrapingDatabase(contextoConexion)
	if err != nil {
		cancelarConexion()
		return err
	}
	cancelarConexion()
	defer database.Close(baseDatos)
	clienteRedis, err := NewRedisClientFromEnvironment()
	if err != nil {
		return err
	}
	defer clienteRedis.Close()

	router, coordinacion, err := NewApplication(baseDatos, clienteRedis)
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

	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	numeroPuerto, err := strconv.Atoi(puerto)
	if err != nil || numeroPuerto < 1 || numeroPuerto > 65535 {
		return errors.New("PORT debe estar entre 1 y 65535")
	}
	log.Printf("Iniciando servidor en el puerto %s...", puerto)
	servidorHTTP := &http.Server{Addr: ":" + puerto, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 * 1024}
	resultadoServidor := make(chan error, 1)
	go func() { resultadoServidor <- servidorHTTP.ListenAndServe() }()
	select {
	case err := <-resultadoServidor:
		if err != http.ErrServerClosed {
			return errors.New("no se pudo iniciar el listener HTTP")
		}
	case <-ctx.Done():
		contextoApagado, cancelarApagado := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelarApagado()
		if err := servidorHTTP.Shutdown(contextoApagado); err != nil {
			_ = servidorHTTP.Close()
		}
		runtimeCancel()
		if err := coordinacion.Wait(contextoApagado); err != nil {
			slog.Error("scraping_shutdown_timeout")
		}
	}
	return nil
}
