package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/api/docs"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/identity"
	"github.com/danirc2024/Taller_integracion_III/backend/api/internal/scraping"
	"github.com/danirc2024/Taller_integracion_III/backend/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var RDB *redis.Client

func initDB() {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("ERROR DE SEGURIDAD: No se encontró DB_URL en las variables de entorno. Abortando inicio.")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error conectando a la base de datos: %v", err)
	}

	log.Println("Conexión a la base de datos establecida exitosamente.")
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

	RDB = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Printf("Aviso: No se pudo conectar a Redis (%v). Rate limiting operará en modo fail-open.", err)
	} else {
		log.Println("Conexión a Redis establecida exitosamente.")
	}
}

func setupRouter() *gin.Engine {
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
	r.GET("/health", HealthHandler)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Grupo de rutas de la API v1
	v1 := r.Group("/api/v1")
	v1.GET("/health", HealthHandler)
	identidad := identity.New(DB, RDB)
	coordinacion := scraping.New(DB, RDB)
	identidad.RegisterRoutes(v1)
	coordinacion.RegisterRoutes(v1)

	return r
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
// @Description  Verifica la disponibilidad de la API Gateway y sus dependencias (PostgreSQL, Redis).
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func HealthHandler(c *gin.Context) {
	dbStatus := "ok"
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "error"
		}
	} else {
		dbStatus = "not_configured"
	}

	redisStatus := "ok"
	if RDB != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		if err := RDB.Ping(ctx).Err(); err != nil {
			redisStatus = "error"
		}
	} else {
		redisStatus = "not_configured"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "api-gateway",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": gin.H{
			"database": dbStatus,
			"redis":    redisStatus,
		},
	})
}

// @title           API Gateway de Supermercados
// @version         1.0
// @description     API Gateway construida en Go con Gin
// @host            localhost:8080
// @BasePath        /
func main() {
	// Intentar cargar .env, pero si no existe (ej. Producción/Docker), continuar usando variables de entorno inyectadas
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	// Permitir que Swagger se adapte dinámicamente a cualquier dominio o IP
	docs.SwaggerInfo.Host = ""

	initDB()
	initRedis()

	r := setupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor en el puerto %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
