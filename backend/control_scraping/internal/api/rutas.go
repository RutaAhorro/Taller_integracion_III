// Package api es la capa HTTP de Control de scraping: router, middlewares,
// sondas, autenticación de servicio y handlers. Traduce HTTP a casos de uso de
// services y no contiene reglas de negocio.
//
// @title           Control de Scraping
// @version         1.0
// @description     Contrato del servicio de coordinación. Identidad y Catálogo se atienden por Gateway y no forman parte de esta especificación.
// @BasePath        /
// @securityDefinitions.apikey TokenServicio
// @in              header
// @name            Authorization
// @description     Bearer SCRAPING_SERVICE_TOKEN (Scraper Go y bot de Discord).
package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/docs"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/config"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/repositories"
	"github.com/danirc2024/Taller_integracion_III/backend/control_scraping/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// ===== Límites de petición y sondas =====

const (
	// plazoPeticion supera la lease de una entrega síncrona con tres llamadas a Catálogo.
	plazoPeticion = 65 * time.Second
	// MaximoBytesCuerpo es el mismo máximo que acepta Catálogo por lote.
	MaximoBytesCuerpo = 8 << 20
	// El presupuesto común de readiness queda bajo el timeout del probe de K8s (3 s).
	plazoReadiness     = 2 * time.Second
	plazoPingBaseDatos = time.Second
	plazoPingRedis     = 500 * time.Millisecond
	nombreServicio     = "control-scraping"
	// nombreServicioSalud es el valor histórico de /health y /api/v1/health.
	nombreServicioSalud = "api-gateway"
	estadoOK            = "ok"
	estadoError         = "error"
	estadoNoConfigurado = "not_configured"
	// claveIDPeticion guarda en gin.Context el X-Request-ID saneado.
	claveIDPeticion   = "request_id"
	bytesIDGenerado   = 16
	cabecerasCORS     = "Content-Type, Authorization, X-Request-ID, X-CSRF-Token, Idempotency-Key"
	metodosCORS       = "GET, POST, PUT, OPTIONS"
	desafioAutenticar = `Bearer realm="control-scraping"`
)

// patronIDPeticion acepta sólo identificadores de petición seguros para logs.
var patronIDPeticion = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

// ===== Composición del módulo =====

// Modulo agrupa los servicios que exponen los handlers de scraping.
type Modulo struct {
	trabajos    *services.ServicioTrabajos
	entregas    *services.ServicioEntregas
	ejecuciones *services.ServicioEjecuciones
}

// NuevoModulo recibe los servicios ya compuestos; las pruebas inyectan dobles.
func NuevoModulo(trabajos *services.ServicioTrabajos, entregas *services.ServicioEntregas, ejecuciones *services.ServicioEjecuciones) *Modulo {
	return &Modulo{trabajos: trabajos, entregas: entregas, ejecuciones: ejecuciones}
}

// Componer arma repositorios, servicios, router y recuperación sobre las
// conexiones abiertas. Lo usan main y las pruebas de integración.
func Componer(configuracion config.Configuracion, baseDatos *gorm.DB, clienteRedis *redis.Client) (*gin.Engine, *services.Recuperacion) {
	catalogo := repositories.NuevoClienteCatalogo(configuracion.URLCatalogo, configuracion.TokenCatalogo, configuracion.PlazoCatalogo)
	publicador := repositories.NuevoPublicadorRedis(clienteRedis, baseDatos)
	trabajos := services.NuevoServicioTrabajos(repositories.NuevoRepositorioTrabajos(baseDatos), catalogo)
	entregas := services.NuevoServicioEntregas(repositories.NuevoRepositorioEntregas(baseDatos), catalogo)
	ejecuciones := services.NuevoServicioEjecuciones(repositories.NuevoRepositorioEjecuciones(baseDatos), publicador)
	enrutador := NuevoEnrutador(configuracion, NuevoModulo(trabajos, entregas, ejecuciones), baseDatos, clienteRedis)
	return enrutador, services.NuevaRecuperacion(entregas, ejecuciones, publicador.Cerrar)
}

// ===== Router =====

// NuevoEnrutador monta middlewares, sondas, Swagger y las rutas protegidas.
func NuevoEnrutador(configuracion config.Configuracion, modulo *Modulo, baseDatos *gorm.DB, clienteRedis *redis.Client) *gin.Engine {
	enrutador := gin.New()
	// No se acepta IP ni identidad desde headers de proxy.
	_ = enrutador.SetTrustedProxies(nil)
	enrutador.Use(observar(slog.Default()), recuperarPanicos(), acotarPeticion(), cors(configuracion.OrigenesPermitidos))
	enrutador.HandleMethodNotAllowed = true
	enrutador.NoRoute(rutaNoEncontrada)
	enrutador.NoMethod(metodoNoPermitido)

	salud := sonda(baseDatos, clienteRedis, nombreServicioSalud)
	enrutador.GET("/", raiz)
	// /_api/live lo usa tests/smoke_data_ownership.py en el CI de Gateway.
	enrutador.GET("/_api/live", vivo)
	enrutador.GET("/_scraping/live", vivo)
	enrutador.GET("/_scraping/ready", sonda(baseDatos, clienteRedis, nombreServicio))
	enrutador.GET("/health", salud)
	// Swagger se publica sólo si se habilita (Compose y CI); en K8s queda apagado.
	if configuracion.SwaggerHabilitado {
		// Host vacío: Swagger UI usa el dominio desde el que se abre.
		docs.SwaggerInfo.Host = ""
		enrutador.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	grupoAPI := enrutador.Group("/api/v1")
	grupoAPI.GET("/health", salud)
	scraper := grupoAPI.Group("/scraper", exigirTokenServicio(configuracion.TokenServicio))
	scraper.POST("/trabajos", modulo.IniciarTrabajo)
	scraper.GET("/trabajos/:id", modulo.ObtenerTrabajo)
	scraper.PUT("/trabajos/:id/finalizar", modulo.FinalizarTrabajo)
	scraper.POST("/trabajos/:id/ejecutar", modulo.EjecutarTrabajo)
	scraper.POST("/trabajos/:id/productos", modulo.IngestarProductosConTrabajo)
	scraper.POST("/productos", modulo.IngestarProductosDirecto)
	return enrutador
}

// ===== Middlewares =====

// observar asigna un X-Request-ID seguro y registra el resultado sin datos sensibles.
func observar(registrador *slog.Logger) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		identificador := contexto.GetHeader(cabeceraIDPeticion)
		if !patronIDPeticion.MatchString(identificador) {
			var aleatorio [bytesIDGenerado]byte
			_, _ = rand.Read(aleatorio[:])
			identificador = hex.EncodeToString(aleatorio[:])
		}
		contexto.Header(cabeceraIDPeticion, identificador)
		contexto.Set(claveIDPeticion, identificador)
		inicio := time.Now()
		contexto.Next()
		ruta := contexto.FullPath()
		if ruta == "" {
			ruta = "unmatched"
		}
		registrador.Info("http_request", "request_id", identificador, "method", contexto.Request.Method, "path", ruta, "status", contexto.Writer.Status(), "duration_ms", time.Since(inicio).Milliseconds())
	}
}

// acotarPeticion limita duración y tamaño del cuerpo de cada petición.
func acotarPeticion() gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contextoAcotado, cancelar := context.WithTimeout(contexto.Request.Context(), plazoPeticion)
		defer cancelar()
		contexto.Request = contexto.Request.WithContext(contextoAcotado)
		contexto.Request.Body = http.MaxBytesReader(contexto.Writer, contexto.Request.Body, MaximoBytesCuerpo)
		contexto.Next()
	}
}

// cors rechaza con 403 los orígenes fuera de la allowlist antes de los
// handlers y resuelve los preflight con 204.
func cors(origenes []string) gin.HandlerFunc {
	permitidos := map[string]bool{}
	for _, origen := range origenes {
		permitidos[origen] = true
	}
	return func(contexto *gin.Context) {
		contexto.Writer.Header().Add("Vary", "Origin")
		origen := contexto.GetHeader("Origin")
		if origen != "" {
			if !permitidos[origen] {
				responderError(contexto, http.StatusForbidden, "Origen no permitido.", "")
				return
			}
			contexto.Header("Access-Control-Allow-Origin", origen)
			contexto.Header("Access-Control-Allow-Credentials", "true")
			contexto.Header("Access-Control-Allow-Headers", cabecerasCORS)
			contexto.Header("Access-Control-Allow-Methods", metodosCORS)
		}
		if contexto.Request.Method == http.MethodOptions {
			contexto.AbortWithStatus(http.StatusNoContent)
			return
		}
		contexto.Next()
	}
}

// exigirTokenServicio protege /api/v1/scraper: Gateway es público, así que
// sólo Scraper Go y el bot con el Bearer correcto pasan. Se comparan hashes de
// largo fijo para no filtrar largo ni contenido por tiempo.
func exigirTokenServicio(token string) gin.HandlerFunc {
	esperado := sha256.Sum256([]byte("Bearer " + token))
	return func(contexto *gin.Context) {
		recibido := sha256.Sum256([]byte(contexto.GetHeader("Authorization")))
		if subtle.ConstantTimeCompare(recibido[:], esperado[:]) != 1 {
			contexto.Header("WWW-Authenticate", desafioAutenticar)
			responderError(contexto, http.StatusUnauthorized, "Token de servicio inválido o ausente.", "")
			return
		}
		contexto.Next()
	}
}

// ===== Sondas =====

// raiz godoc
// @Summary      Muestra mensaje de bienvenida
// @Description  Endpoint raíz para comprobar que el servicio responde
// @Tags         root
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       / [get]
func raiz(contexto *gin.Context) {
	contexto.JSON(http.StatusOK, gin.H{
		"message": "Bienvenido a Control de scraping.",
		"docs":    "Visita /swagger/index.html para ver el autocompletado Swagger UI.",
	})
}

// vivo es la sonda de liveness: no consulta dependencias.
func vivo(contexto *gin.Context) {
	contexto.JSON(http.StatusOK, gin.H{"status": estadoOK, "service": nombreServicio})
}

// sonda godoc
// @Summary      Comprobación de salud del servicio (Healthcheck)
// @Description  Readiness: requiere la conexión PostgreSQL de coordinación; Redis es diagnóstico y puede degradarse sin retirar la API.
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /health [get]
// @Router       /api/v1/health [get]
//
// sonda genera la readiness: SQL decide el estado y Redis sólo se informa.
func sonda(baseDatos *gorm.DB, clienteRedis *redis.Client, servicio string) gin.HandlerFunc {
	return func(contexto *gin.Context) {
		contextoSonda, cancelar := context.WithTimeout(contexto.Request.Context(), plazoReadiness)
		defer cancelar()
		estadoBaseDatos := verificarBaseDatos(contextoSonda, baseDatos)
		estadoRedis := estadoNoConfigurado
		if clienteRedis != nil {
			contextoRedis, cancelarRedis := context.WithTimeout(contextoSonda, plazoPingRedis)
			defer cancelarRedis()
			estadoRedis = estadoOK
			if clienteRedis.Ping(contextoRedis).Err() != nil {
				estadoRedis = estadoError
			}
		}
		estadoHTTP, estadoGeneral := http.StatusOK, estadoOK
		if estadoBaseDatos != estadoOK {
			estadoHTTP, estadoGeneral = http.StatusServiceUnavailable, estadoError
		}
		contexto.JSON(estadoHTTP, gin.H{
			"status":    estadoGeneral,
			"service":   servicio,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			// database y scraping_database coinciden; Gateway y los smoke leen ambas.
			"checks": gin.H{"database": estadoBaseDatos, "scraping_database": estadoBaseDatos, "redis": estadoRedis},
		})
	}
}

// verificarBaseDatos hace ping a PostgreSQL sin exponer el error del driver.
func verificarBaseDatos(contexto context.Context, baseDatos *gorm.DB) string {
	if baseDatos == nil {
		return estadoNoConfigurado
	}
	contextoPing, cancelar := context.WithTimeout(contexto, plazoPingBaseDatos)
	defer cancelar()
	conexionSQL, errorObtenido := baseDatos.DB()
	if errorObtenido != nil || conexionSQL.PingContext(contextoPing) != nil {
		return estadoError
	}
	return estadoOK
}
