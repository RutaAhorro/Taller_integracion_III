// Package spool implementa el outbox durable en SQLite. El esquema y la
// semántica son compatibles con scraper_core/outbox.py para permitir el
// rollback al worker de Python sin perder el trabajo pendiente.
package spool

import (
	"context"
	"database/sql"
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

type Spool struct {
	workID    string
	directory string
	minFree   int64
	db        *sql.DB
}

// Open abre o crea el spool de un trabajo. La comprobación de capacidad se
// hace antes de SQLite para no llenar el volumen con un archivo nuevo.
func Open(workID, stateDir string, minFree int64) (*Spool, error) {
	parsed, err := uuid.Parse(workID)
	if err != nil {
		return nil, fmt.Errorf("trabajo_id inválido: %w", err)
	}
	workID = parsed.String()
	directory := filepath.Join(stateDir, workID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, err
	}
	if err := storage.EnsureCapacity(directory, minFree, 0); err != nil {
		return nil, err
	}

	path := filepath.Join(directory, "outbox.sqlite")
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	result := &Spool{workID: workID, directory: directory, minFree: minFree, db: db}
	if err := result.initialize(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return result, nil
}

func (s *Spool) initialize() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY, fingerprint TEXT UNIQUE NOT NULL,
			market TEXT, product TEXT NOT NULL, batch_id TEXT);
		CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY, payload TEXT NOT NULL, delivered INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE IF NOT EXISTS progress (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
	`); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(s.directory, "outbox.sqlite"), 0o600)
}

func (s *Spool) WorkID() string {
	return s.workID
}

func (s *Spool) Directory() string {
	return s.directory
}

func (s *Spool) Close() error {
	return s.db.Close()
}

// Add guarda un producto antes de enviarlo. Si el mismo producto ya está en
// el spool, la huella UNIQUE evita duplicarlo.
func (s *Spool) Add(product domain.Product, market *string) error {
	data, err := canonicalProductJSON(product)
	if err != nil {
		return err
	}
	if err := storage.EnsureCapacity(s.directory, s.minFree, int64(len(data))*4); err != nil {
		return err
	}

	fingerprint, err := pythonFingerprint(market, data)
	if err != nil {
		return err
	}

	var marketArg any
	if market != nil {
		marketArg = *market
	}
	_, err = s.db.Exec(
		"INSERT OR IGNORE INTO items(fingerprint, market, product) VALUES (?, ?, ?)",
		fingerprint, marketArg, string(data),
	)
	return err
}

func (s *Spool) Unbatched() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM items WHERE batch_id IS NULL").Scan(&count)
	return count, err
}

// Count devuelve todos los productos persistidos, entregados o no. El worker
// lo usa para no declarar completado un trabajo que terminó con cero
// elementos, incluso si el crawl no devolvió un error explícito.
func (s *Spool) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM items").Scan(&count)
	return count, err
}

// Seal agrupa los items pendientes por supermercado. Nunca mezcla mercados en
// un mismo lote y conserva el payload exacto que se enviará.
func (s *Spool) Seal(batchSize int) error {
	if batchSize < 1 || batchSize > 1000 {
		return fmt.Errorf("el tamaño de lote debe estar entre 1 y 1000")
	}
	for {
		market, found, err := s.firstUnbatchedMarket()
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		rows, err := s.unbatchedRows(market, batchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := s.persistBatch(market, rows); err != nil {
			return err
		}
	}
}

type itemRow struct {
	id      int64
	product string
}

func (s *Spool) firstUnbatchedMarket() (sql.NullString, bool, error) {
	var market sql.NullString
	err := s.db.QueryRow(
		"SELECT market FROM items WHERE batch_id IS NULL ORDER BY id LIMIT 1",
	).Scan(&market)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.NullString{}, false, nil
	}
	if err != nil {
		return sql.NullString{}, false, err
	}
	return market, true, nil
}

func (s *Spool) unbatchedRows(market sql.NullString, batchSize int) ([]itemRow, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if market.Valid {
		rows, err = s.db.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market = ? ORDER BY id LIMIT ?",
			market.String, batchSize,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market IS NULL ORDER BY id LIMIT ?",
			batchSize,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]itemRow, 0, batchSize)
	for rows.Next() {
		var row itemRow
		if err := rows.Scan(&row.id, &row.product); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

type batchPayload struct {
	Products    []json.RawMessage `json:"products"`
	Supermarket *string           `json:"supermarket"`
}

func (s *Spool) persistBatch(market sql.NullString, rows []itemRow) error {
	var extra int64
	products := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		extra += int64(len(row.product)) * 2
		products = append(products, json.RawMessage(row.product))
	}
	if err := storage.EnsureCapacity(s.directory, s.minFree, extra); err != nil {
		return err
	}

	var supermarket *string
	if market.Valid {
		value := market.String
		supermarket = &value
	}
	payload, err := json.Marshal(batchPayload{Products: products, Supermarket: supermarket})
	if err != nil {
		return err
	}

	batchID := uuid.NewString()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO batches(id, payload) VALUES (?, ?)", batchID, string(payload)); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, row := range rows {
		if _, err := tx.Exec("UPDATE items SET batch_id = ? WHERE id = ?", batchID, row.id); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// Replay envía los lotes pendientes en orden. Se consulta un lote por vez
// para no cargar en memoria todo el catálogo si la API estuvo caída.
func (s *Spool) Replay(ctx context.Context, client *controlscraping.Client) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var (
			batchID string
			payload []byte
		)
		err := s.db.QueryRow(
			"SELECT id, payload FROM batches WHERE delivered = 0 ORDER BY rowid LIMIT 1",
		).Scan(&batchID, &payload)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		var batch batchPayload
		if err := json.Unmarshal(payload, &batch); err != nil {
			return fmt.Errorf("lote %s inválido: %w", batchID, err)
		}
		products := make([]domain.Product, 0, len(batch.Products))
		for _, raw := range batch.Products {
			var product domain.Product
			if err := json.Unmarshal(raw, &product); err != nil {
				return fmt.Errorf("producto del lote %s inválido: %w", batchID, err)
			}
			products = append(products, product)
		}
		if _, err := client.IngestBatch(ctx, products, batch.Supermarket, s.workID, batchID); err != nil {
			return err
		}
		if _, err := s.db.Exec("UPDATE batches SET delivered = 1 WHERE id = ?", batchID); err != nil {
			return err
		}
	}
}

func (s *Spool) Progress(key string) (int, error) {
	var value int
	err := s.db.QueryRow("SELECT value FROM progress WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return value, err
}

func (s *Spool) SetProgress(key string, value int) error {
	_, err := s.db.Exec(
		"INSERT INTO progress(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}
