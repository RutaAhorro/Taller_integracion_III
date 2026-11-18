// Package spool implementa el outbox durable en SQLite para almacenar los
// productos scrapeados y garantizar entregas confiables hacia la API.
package spool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/controlscraping"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/domain"
	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/storage"
)

// ===== Estructura del Spool =====

// Spool es el outbox durable de un trabajo: productos, lotes y progreso de
// recuperación. Cada trabajo tiene su propio archivo SQLite.
type Spool struct {
	workID    string
	directory string
	minFree   int64
	db        *sql.DB
}

// ===== Apertura =====

// Open abre o crea el spool de un trabajo. La comprobación de capacidad se
// hace antes de SQLite para no llenar el volumen con un archivo nuevo.
func Open(trabajoID, directorioEstado string, minimoLibre int64) (*Spool, error) {
	analizada, errorObtenido := uuid.Parse(trabajoID)
	if errorObtenido != nil {
		return nil, fmt.Errorf("trabajo_id inválido: %w", errorObtenido)
	}
	trabajoID = analizada.String()
	directorio := filepath.Join(directorioEstado, trabajoID)
	if errorObtenido := os.MkdirAll(directorio, 0o700); errorObtenido != nil {
		return nil, errorObtenido
	}
	if errorObtenido := os.Chmod(directorio, 0o700); errorObtenido != nil {
		return nil, errorObtenido
	}
	if errorObtenido := storage.EnsureCapacity(directorio, minimoLibre, 0); errorObtenido != nil {
		return nil, errorObtenido
	}

	ruta := filepath.Join(directorio, "outbox.sqlite")
	cadenaConexion := "file:" + filepath.ToSlash(ruta) +
		"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"
	baseDatos, errorObtenido := sql.Open("sqlite", cadenaConexion)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	baseDatos.SetMaxOpenConns(1)
	if errorObtenido := baseDatos.Ping(); errorObtenido != nil {
		_ = baseDatos.Close()
		return nil, errorObtenido
	}

	resultado := &Spool{workID: trabajoID, directory: directorio, minFree: minimoLibre, db: baseDatos}
	if errorObtenido := resultado.initialize(); errorObtenido != nil {
		_ = baseDatos.Close()
		return nil, errorObtenido
	}
	return resultado, nil
}

// ===== Inicialización =====

// initialize prepara el esquema de tablas e índices del almacén SQLite local.
func (spool *Spool) initialize() error {
	if _, errorObtenido := spool.db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY, fingerprint TEXT UNIQUE NOT NULL,
			market TEXT, product TEXT NOT NULL, batch_id TEXT);
		CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY, payload TEXT NOT NULL, delivered INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE IF NOT EXISTS progress (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
	`); errorObtenido != nil {
		return errorObtenido
	}
	return os.Chmod(filepath.Join(spool.directory, "outbox.sqlite"), 0o600)
}

// ===== Ciclo de Vida =====

// Directory devuelve el directorio donde vive el spool.
func (spool *Spool) Directory() string {
	return spool.directory
}

// Close cierra la base SQLite del spool.
func (spool *Spool) Close() error {
	return spool.db.Close()
}

// ===== Productos =====

// Add guarda un producto antes de enviarlo. Si el mismo producto ya está en
// el spool, la huella UNIQUE evita duplicarlo.
func (spool *Spool) Add(producto domain.Product, mercado *string) error {
	datos, errorObtenido := json.Marshal(producto)
	if errorObtenido != nil {
		return errorObtenido
	}
	if errorObtenido := storage.EnsureCapacity(spool.directory, spool.minFree, int64(len(datos))*4); errorObtenido != nil {
		return errorObtenido
	}

	huella := productFingerprint(mercado, datos)

	var argumentoMercado any
	if mercado != nil {
		argumentoMercado = *mercado
	}
	_, errorObtenido = spool.db.Exec(
		"INSERT OR IGNORE INTO items(fingerprint, market, product) VALUES (?, ?, ?)",
		huella, argumentoMercado, string(datos),
	)
	return errorObtenido
}

// ===== Huellas de Producto =====

// productFingerprint genera un hash SHA-256 para garantizar idempotencia en productos.
func productFingerprint(mercado *string, datos []byte) string {
	calculadorHash := sha256.New()
	if mercado != nil {
		calculadorHash.Write([]byte(*mercado))
	}
	calculadorHash.Write([]byte{0})
	calculadorHash.Write(datos)
	return hex.EncodeToString(calculadorHash.Sum(nil))
}

// ===== Conteo =====

// Count devuelve todos los productos persistidos, entregados o no. El worker
// lo usa para no declarar completado un trabajo que terminó con cero
// elementos, incluso si el crawl no devolvió un error explícito.
func (spool *Spool) Count() (int, error) {
	var cantidad int
	errorObtenido := spool.db.QueryRow("SELECT count(*) FROM items").Scan(&cantidad)
	return cantidad, errorObtenido
}

// ===== Sellado de Lotes =====

// Seal agrupa los items pendientes por supermercado. Nunca mezcla mercados en
// un mismo lote y conserva el payload exacto que se enviará.
func (spool *Spool) Seal(tamanoLote int) error {
	if tamanoLote < 1 || tamanoLote > 1000 {
		return fmt.Errorf("el tamaño de lote debe estar entre 1 y 1000")
	}
	for {
		mercado, encontrado, errorObtenido := spool.firstUnbatchedMarket()
		if errorObtenido != nil {
			return errorObtenido
		}
		if !encontrado {
			return nil
		}
		filas, errorObtenido := spool.unbatchedRows(mercado, tamanoLote)
		if errorObtenido != nil {
			return errorObtenido
		}
		if len(filas) == 0 {
			return nil
		}
		if errorObtenido := spool.persistBatch(mercado, filas); errorObtenido != nil {
			return errorObtenido
		}
	}
}

// ===== Consultas de Lotes =====

// itemRow representa una fila de producto leída directamente desde SQLite local.
type itemRow struct {
	id      int64
	product string
}

// firstUnbatchedMarket identifica el primer supermercado con productos pendientes de entrega.
func (spool *Spool) firstUnbatchedMarket() (sql.NullString, bool, error) {
	var mercado sql.NullString
	errorObtenido := spool.db.QueryRow(
		"SELECT market FROM items WHERE batch_id IS NULL ORDER BY id LIMIT 1",
	).Scan(&mercado)
	if errors.Is(errorObtenido, sql.ErrNoRows) {
		return sql.NullString{}, false, nil
	}
	if errorObtenido != nil {
		return sql.NullString{}, false, errorObtenido
	}
	return mercado, true, nil
}

// unbatchedRows extrae las filas aún no entregadas de un supermercado dado.
func (spool *Spool) unbatchedRows(mercado sql.NullString, tamanoLote int) ([]itemRow, error) {
	var (
		filas         *sql.Rows
		errorObtenido error
	)
	if mercado.Valid {
		filas, errorObtenido = spool.db.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market = ? ORDER BY id LIMIT ?",
			mercado.String, tamanoLote,
		)
	} else {
		filas, errorObtenido = spool.db.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market IS NULL ORDER BY id LIMIT ?",
			tamanoLote,
		)
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer filas.Close()

	resultado := make([]itemRow, 0, tamanoLote)
	for filas.Next() {
		var fila itemRow
		if errorObtenido := filas.Scan(&fila.id, &fila.product); errorObtenido != nil {
			return nil, errorObtenido
		}
		resultado = append(resultado, fila)
	}
	return resultado, filas.Err()
}

// ===== Persistencia de Lotes =====

// batchPayload agrupa el conjunto de productos que se despachan a la API de control.
type batchPayload struct {
	Products    []json.RawMessage `json:"products"`
	Supermarket *string           `json:"supermarket"`
}

// persistBatch actualiza el estado de entrega de las filas procesadas en SQLite.
func (spool *Spool) persistBatch(mercado sql.NullString, filas []itemRow) error {
	var extra int64
	productos := make([]json.RawMessage, 0, len(filas))
	for _, fila := range filas {
		extra += int64(len(fila.product)) * 2
		productos = append(productos, json.RawMessage(fila.product))
	}
	if errorObtenido := storage.EnsureCapacity(spool.directory, spool.minFree, extra); errorObtenido != nil {
		return errorObtenido
	}

	var supermercado *string
	if mercado.Valid {
		valor := mercado.String
		supermercado = &valor
	}
	carga, errorObtenido := json.Marshal(batchPayload{Products: productos, Supermarket: supermercado})
	if errorObtenido != nil {
		return errorObtenido
	}

	loteID := uuid.NewString()
	transaccion, errorObtenido := spool.db.Begin()
	if errorObtenido != nil {
		return errorObtenido
	}
	if _, errorObtenido := transaccion.Exec("INSERT INTO batches(id, payload) VALUES (?, ?)", loteID, string(carga)); errorObtenido != nil {
		_ = transaccion.Rollback()
		return errorObtenido
	}
	for _, fila := range filas {
		if _, errorObtenido := transaccion.Exec("UPDATE items SET batch_id = ? WHERE id = ?", loteID, fila.id); errorObtenido != nil {
			_ = transaccion.Rollback()
			return errorObtenido
		}
	}
	return transaccion.Commit()
}

// ===== Reenvío de Lotes =====

// Replay envía los lotes pendientes en orden. Se consulta un lote por vez
// para no cargar en memoria todo el catálogo si la API estuvo caída.
func (spool *Spool) Replay(contexto context.Context, cliente *controlscraping.Client) error {
	for {
		if errorObtenido := contexto.Err(); errorObtenido != nil {
			return errorObtenido
		}
		var (
			loteID string
			carga  []byte
		)
		errorObtenido := spool.db.QueryRow(
			"SELECT id, payload FROM batches WHERE delivered = 0 ORDER BY rowid LIMIT 1",
		).Scan(&loteID, &carga)
		if errors.Is(errorObtenido, sql.ErrNoRows) {
			return nil
		}
		if errorObtenido != nil {
			return errorObtenido
		}

		var lote batchPayload
		if errorObtenido := json.Unmarshal(carga, &lote); errorObtenido != nil {
			return fmt.Errorf("lote %s inválido: %w", loteID, errorObtenido)
		}
		productos := make([]domain.Product, 0, len(lote.Products))
		for _, crudo := range lote.Products {
			var producto domain.Product
			if errorObtenido := json.Unmarshal(crudo, &producto); errorObtenido != nil {
				return fmt.Errorf("producto del lote %s inválido: %w", loteID, errorObtenido)
			}
			productos = append(productos, producto)
		}
		if _, errorObtenido := cliente.IngestBatch(contexto, productos, lote.Supermarket, spool.workID, loteID); errorObtenido != nil {
			return errorObtenido
		}
		if _, errorObtenido := spool.db.Exec("UPDATE batches SET delivered = 1 WHERE id = ?", loteID); errorObtenido != nil {
			return errorObtenido
		}
	}
}

// ===== Progreso =====

// Progress lee una clave de progreso; devuelve 0 si todavía no existe.
func (spool *Spool) Progress(clave string) (int, error) {
	var valor int
	errorObtenido := spool.db.QueryRow("SELECT value FROM progress WHERE key = ?", clave).Scan(&valor)
	if errors.Is(errorObtenido, sql.ErrNoRows) {
		return 0, nil
	}
	return valor, errorObtenido
}

// SetProgress guarda o reemplaza una clave de progreso del trabajo.
func (spool *Spool) SetProgress(clave string, valor int) error {
	_, errorObtenido := spool.db.Exec(
		"INSERT INTO progress(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		clave, valor,
	)
	return errorObtenido
}
