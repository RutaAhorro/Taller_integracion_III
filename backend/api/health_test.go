package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Un driver con Ping controlado ejercita sql.DB y el handler real sin red.
type healthConnector struct {
	failed atomic.Bool
	pings  atomic.Int32
}

func TestHealthRedisTimeoutDoesNotRemoveReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldIdentity, oldScraping, oldRedis := IdentityDB, ScrapingDB, RDB
	t.Cleanup(func() { IdentityDB, ScrapingDB, RDB = oldIdentity, oldScraping, oldRedis })
	IdentityDB = probeDatabase(t, &healthConnector{})
	ScrapingDB = probeDatabase(t, &healthConnector{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); io.Copy(io.Discard, conn) }()
		}
	}()
	// Acepta TCP pero no responde al handshake ni al PING de Redis.
	t.Setenv("REDIS_URL", "redis://"+listener.Addr().String()+"?max_retries=-1")
	initRedis()
	defer RDB.Close()
	if !RDB.Options().ContextTimeoutEnabled {
		t.Fatal("Redis ignora los deadlines del contexto")
	}
	router := gin.New()
	router.GET("/health", HealthHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	response := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/health", nil).WithContext(ctx))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Redis lento bloqueó el probe durante %v", elapsed)
	}
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || body.Checks["redis"] != "error" || body.Checks["database"] != "ok" {
		t.Fatalf("Redis retiró readiness con PostgreSQL sano: %d %s", response.Code, response.Body)
	}
}

func (d *healthConnector) Connect(context.Context) (driver.Conn, error) { return healthConn{d}, nil }
func (d *healthConnector) Driver() driver.Driver                        { return healthDriver{} }

type healthDriver struct{}

func (healthDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type healthConn struct{ d *healthConnector }

func (healthConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected SQL") }
func (healthConn) Close() error                        { return nil }
func (healthConn) Begin() (driver.Tx, error)           { return nil, errors.New("unexpected transaction") }
func (c healthConn) Ping(ctx context.Context) error {
	c.d.pings.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.d.failed.Load() {
		return errors.New("private PostgreSQL diagnostic")
	}
	return nil
}

func probeDatabase(t *testing.T, connector *healthConnector) *gorm.DB {
	t.Helper()
	sqlDB := sql.OpenDB(connector)
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHealthRequiresBothDatabasePools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldIdentity, oldScraping, oldRedis := IdentityDB, ScrapingDB, RDB
	t.Cleanup(func() { IdentityDB, ScrapingDB, RDB = oldIdentity, oldScraping, oldRedis })
	identity, scraping := &healthConnector{}, &healthConnector{}
	identityDB, scrapingDB := probeDatabase(t, identity), probeDatabase(t, scraping)
	RDB = nil // Redis no es requisito de readiness: la cola SQL sigue disponible.
	router := gin.New()
	router.GET("/health", HealthHandler)
	router.GET("/_api/live", LivenessHandler)
	for _, tc := range []struct {
		name                             string
		identityFailed, scrapingFailed   bool
		identityMissing, scrapingMissing bool
		status                           int
	}{
		{"healthy", false, false, false, false, 200},
		{"identity unavailable", true, false, false, false, 503},
		{"scraping unavailable", false, true, false, false, 503},
		{"both unavailable", true, true, false, false, 503},
		{"identity not configured", false, false, true, false, 503},
		{"both not configured", false, false, true, true, 503},
		{"recovered", false, false, false, false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			IdentityDB, ScrapingDB = identityDB, scrapingDB
			if tc.identityMissing {
				IdentityDB = nil
			}
			if tc.scrapingMissing {
				ScrapingDB = nil
			}
			identity.failed.Store(tc.identityFailed)
			scraping.failed.Store(tc.scrapingFailed)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/health", nil))
			if response.Code != tc.status || strings.Contains(response.Body.String(), "private PostgreSQL") {
				t.Fatalf("readiness incorrecto: %d %s", response.Code, response.Body)
			}
			var body struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if (body.Status == "ok") != (tc.status == 200) {
				t.Fatal("estado JSON contradice HTTP")
			}
			before := identity.pings.Load() + scraping.pings.Load()
			response = httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/_api/live", nil))
			if response.Code != 200 || before != identity.pings.Load()+scraping.pings.Load() {
				t.Fatal("liveness depende de PostgreSQL")
			}
		})
	}
}
