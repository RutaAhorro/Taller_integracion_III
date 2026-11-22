// spool.go mantiene el outbox SQLite por trabajo para no perder productos si
// Control de Scraping falla a mitad de crawl.
package cola

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

	"github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/config"
	dominio "github.com/danirc2024/Taller_integracion_III/backend/scraper_go/internal/dominio"
)

// ===== Estructura del Spool =====

// Spool guarda en SQLite los productos extraídos, los lotes sellados y el
// progreso del crawl de un trabajo.
type Spool struct {
	trabajoID   string
	directorio  string
	minimoLibre int64
	baseDatos   *sql.DB
}

// ===== Constructor e Inicialización =====

// Open abre o crea el spool del trabajo. Exige que workID sea un UUID para
// impedir path traversal sobre stateDir.
func Open(trabajoID, directorioEstado string, minimoLibre int64) (*Spool, error) {
	analizada, errorObtenido := uuid.Parse(trabajoID)
	if errorObtenido != nil {
		return nil, fmt.Errorf("trabajo_id inválido: %w", errorObtenido)
	}
	if errorObtenido := os.MkdirAll(directorioEstado, 0o700); errorObtenido != nil {
		return nil, errorObtenido
	}
	if errorObtenido := EnsureCapacity(directorioEstado, minimoLibre, 0); errorObtenido != nil {
		return nil, errorObtenido
	}

	directorio := filepath.Join(directorioEstado, analizada.String())
	if errorObtenido := os.MkdirAll(directorio, 0o700); errorObtenido != nil {
		return nil, errorObtenido
	}

	rutaBaseDatos := filepath.Join(directorio, "outbox.sqlite")
	// journal_mode=DELETE evita dejar archivos -wal/-shm en trabajos cortos y
	// synchronous=FULL fuerza que cada lote sellado quede en disco antes de
	// intentar la entrega o confirmar la lease.
	conexionCadena := rutaBaseDatos + "?_pragma=journal_mode(DELETE)&_pragma=synchronous(FULL)&_pragma=busy_timeout(30000)"
	baseDatos, errorObtenido := sql.Open("sqlite", conexionCadena)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	baseDatos.SetMaxOpenConns(1)

	spool := &Spool{
		trabajoID:   analizada.String(),
		directorio:  directorio,
		minimoLibre: minimoLibre,
		baseDatos:   baseDatos,
	}
	if errorObtenido := spool.inicializar(); errorObtenido != nil {
		_ = baseDatos.Close()
		return nil, errorObtenido
	}
	return spool, nil
}

// inicializar crea las tablas base en SQLite para almacenar ítems y lotes.
func (spool *Spool) inicializar() error {
	if _, errorObtenido := spool.baseDatos.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY, fingerprint TEXT UNIQUE NOT NULL,
			market TEXT, product TEXT NOT NULL, batch_id TEXT);
		CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY, payload TEXT NOT NULL, delivered INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE IF NOT EXISTS progress (key TEXT PRIMARY KEY, value INTEGER NOT NULL);
	`); errorObtenido != nil {
		return errorObtenido
	}
	return os.Chmod(filepath.Join(spool.directorio, "outbox.sqlite"), 0o600)
}

// ===== Ciclo de Vida =====

// Directory devuelve el directorio donde vive el spool.
func (spool *Spool) Directory() string {
	return spool.directorio
}

// Close cierra la base SQLite del spool.
func (spool *Spool) Close() error {
	return spool.baseDatos.Close()
}

// ===== Productos =====

// Add guarda un producto antes de enviarlo. Si el mismo producto ya está en
// el spool, la huella UNIQUE evita duplicarlo.
func (spool *Spool) Add(producto dominio.Product, mercado *string) error {
	datos, errorObtenido := json.Marshal(producto)
	if errorObtenido != nil {
		return errorObtenido
	}
	if errorObtenido := EnsureCapacity(spool.directorio, spool.minimoLibre, int64(len(datos))*4); errorObtenido != nil {
		return errorObtenido
	}

	huella := huellaProducto(mercado, datos)

	var argumentoMercado any
	if mercado != nil {
		argumentoMercado = *mercado
	}
	_, errorObtenido = spool.baseDatos.Exec(
		"INSERT OR IGNORE INTO items(fingerprint, market, product) VALUES (?, ?, ?)",
		huella, argumentoMercado, string(datos),
	)
	return errorObtenido
}

// ===== Huellas de Producto =====

// huellaProducto genera un hash SHA-256 para garantizar idempotencia en productos.
func huellaProducto(mercado *string, datos []byte) string {
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
	errorObtenido := spool.baseDatos.QueryRow("SELECT count(*) FROM items").Scan(&cantidad)
	return cantidad, errorObtenido
}

// ===== Sellado de Lotes =====

// Seal agrupa los items pendientes por supermercado. Nunca mezcla mercados en
// un mismo lote y conserva el payload exacto que se enviará.
func (spool *Spool) Seal(tamanoLote int) error {
	if tamanoLote < config.MinBatchSize || tamanoLote > config.MaxBatchSize {
		return fmt.Errorf("el tamaño de lote debe estar entre %d y %d", config.MinBatchSize, config.MaxBatchSize)
	}
	for {
		mercado, encontrado, errorObtenido := spool.primerMercadoSinLote()
		if errorObtenido != nil {
			return errorObtenido
		}
		if !encontrado {
			return nil
		}
		filas, errorObtenido := spool.filasSinLote(mercado, tamanoLote)
		if errorObtenido != nil {
			return errorObtenido
		}
		if len(filas) == 0 {
			return nil
		}
		if errorObtenido := spool.persistirLote(mercado, filas); errorObtenido != nil {
			return errorObtenido
		}
	}
}

// ===== Consultas de Lotes =====

// filaElemento representa una fila de producto leída directamente desde SQLite local.
type filaElemento struct {
	identificador int64
	producto      string
}

// primerMercadoSinLote identifica el primer supermercado con productos pendientes de entrega.
func (spool *Spool) primerMercadoSinLote() (sql.NullString, bool, error) {
	var mercado sql.NullString
	errorObtenido := spool.baseDatos.QueryRow(
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

// filasSinLote extrae las filas aún no entregadas de un supermercado dado.
func (spool *Spool) filasSinLote(mercado sql.NullString, tamanoLote int) ([]filaElemento, error) {
	var (
		filas         *sql.Rows
		errorObtenido error
	)
	if mercado.Valid {
		filas, errorObtenido = spool.baseDatos.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market = ? ORDER BY id LIMIT ?",
			mercado.String, tamanoLote,
		)
	} else {
		filas, errorObtenido = spool.baseDatos.Query(
			"SELECT id, product FROM items WHERE batch_id IS NULL AND market IS NULL ORDER BY id LIMIT ?",
			tamanoLote,
		)
	}
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	defer filas.Close()

	resultado := make([]filaElemento, 0, tamanoLote)
	for filas.Next() {
		var fila filaElemento
		if errorObtenido := filas.Scan(&fila.identificador, &fila.producto); errorObtenido != nil {
			return nil, errorObtenido
		}
		resultado = append(resultado, fila)
	}
	return resultado, filas.Err()
}

// ===== Persistencia de Lotes =====

// cargaLote agrupa el conjunto de productos que se despachan a la API de control.
type cargaLote struct {
	Products    []json.RawMessage `json:"products"`
	Supermarket *string           `json:"supermarket"`
}

// persistirLote actualiza el estado de entrega de las filas procesadas en SQLite.
func (spool *Spool) persistirLote(mercado sql.NullString, filas []filaElemento) error {
	var extra int64
	productos := make([]json.RawMessage, 0, len(filas))
	for _, fila := range filas {
		extra += int64(len(fila.producto)) * 2
		productos = append(productos, json.RawMessage(fila.producto))
	}
	if errorObtenido := EnsureCapacity(spool.directorio, spool.minimoLibre, extra); errorObtenido != nil {
		return errorObtenido
	}

	var supermercado *string
	if mercado.Valid {
		valor := mercado.String
		supermercado = &valor
	}
	carga, errorObtenido := json.Marshal(cargaLote{Products: productos, Supermarket: supermercado})
	if errorObtenido != nil {
		return errorObtenido
	}

	loteID := uuid.NewString()
	transaccion, errorObtenido := spool.baseDatos.Begin()
	if errorObtenido != nil {
		return errorObtenido
	}
	if _, errorObtenido := transaccion.Exec("INSERT INTO batches(id, payload) VALUES (?, ?)", loteID, string(carga)); errorObtenido != nil {
		_ = transaccion.Rollback()
		return errorObtenido
	}
	for _, fila := range filas {
		if _, errorObtenido := transaccion.Exec("UPDATE items SET batch_id = ? WHERE id = ?", loteID, fila.identificador); errorObtenido != nil {
			_ = transaccion.Rollback()
			return errorObtenido
		}
	}
	return transaccion.Commit()
}

// ===== Reenvío de Lotes =====

// Replay envía los lotes pendientes en orden. Se consulta un lote por vez
// para no cargar en memoria todo el catálogo si la API estuvo caída.
func (spool *Spool) Replay(contexto context.Context, cliente *Client) error {
	for {
		if errorObtenido := contexto.Err(); errorObtenido != nil {
			return errorObtenido
		}
		var (
			loteID string
			carga  []byte
		)
		errorObtenido := spool.baseDatos.QueryRow(
			"SELECT id, payload FROM batches WHERE delivered = 0 ORDER BY rowid LIMIT 1",
		).Scan(&loteID, &carga)
		if errors.Is(errorObtenido, sql.ErrNoRows) {
			return nil
		}
		if errorObtenido != nil {
			return errorObtenido
		}

		var lote cargaLote
		if errorObtenido := json.Unmarshal(carga, &lote); errorObtenido != nil {
			return fmt.Errorf("lote %s inválido: %w", loteID, errorObtenido)
		}
		productos := make([]dominio.Product, 0, len(lote.Products))
		for _, crudo := range lote.Products {
			var producto dominio.Product
			if errorObtenido := json.Unmarshal(crudo, &producto); errorObtenido != nil {
				return fmt.Errorf("producto del lote %s inválido: %w", loteID, errorObtenido)
			}
			productos = append(productos, producto)
		}
		if _, errorObtenido := cliente.IngestBatch(contexto, productos, lote.Supermarket, spool.trabajoID, loteID); errorObtenido != nil {
			return errorObtenido
		}
		if _, errorObtenido := spool.baseDatos.Exec("UPDATE batches SET delivered = 1 WHERE id = ?", loteID); errorObtenido != nil {
			return errorObtenido
		}
	}
}

// ===== Progreso =====

// Progress lee una clave de progreso; devuelve 0 si todavía no existe.
func (spool *Spool) Progress(clave string) (int, error) {
	var valor int
	errorObtenido := spool.baseDatos.QueryRow("SELECT value FROM progress WHERE key = ?", clave).Scan(&valor)
	if errors.Is(errorObtenido, sql.ErrNoRows) {
		return 0, nil
	}
	return valor, errorObtenido
}

// SetProgress guarda o reemplaza una clave de progreso del trabajo.
func (spool *Spool) SetProgress(clave string, valor int) error {
	_, errorObtenido := spool.baseDatos.Exec(
		"INSERT INTO progress(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		clave, valor,
	)
	return errorObtenido
}
