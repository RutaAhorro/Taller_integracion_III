// Ingesta: resolución de cadenas y sucursales y confirmación transaccional de
// lotes de productos y precios, con recibo idempotente en scraper.lotes_catalogo.

package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"gorm.io/gorm"
)

// ===== Constantes =====

// Claves de pg_advisory_xact_lock. Sólo las usa Catálogo y serializan, entre
// sus réplicas, la autocreación de cadenas y cada clave de lote. Son parte del
// contrato operativo: no cambiarlas sin coordinar despliegue. (SUP-376: la
// clave 2692, que protegía la autocreación de sucursales, se retiró junto con
// ella; no reutilizarla para otro fin sin coordinar.)
const (
	bloqueoCadena = 2691
	bloqueoLote   = 2701
)

// ===== Repositorio =====

// RepositorioIngesta implementa domain.RepositorioIngesta.
type RepositorioIngesta struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioIngesta crea el repositorio sobre la conexión compartida.
func NuevoRepositorioIngesta(baseDatos *gorm.DB) *RepositorioIngesta {
	return &RepositorioIngesta{baseDatos: baseDatos}
}

// ===== Cadenas =====

// ObtenerCadena busca por ID (sólo lectura) o por nombre sin distinguir
// mayúsculas; si el nombre no existe, la crea bajo un lock para que dos
// réplicas no dupliquen la misma cadena.
func (repositorio *RepositorioIngesta) ObtenerCadena(contexto context.Context, cadenaID int, nombre string) (*domain.CadenaSupermercado, error) {
	if cadenaID > 0 {
		return buscarCadenaPorID(repositorio.baseDatos.WithContext(contexto), cadenaID)
	}
	nombreLimpio := strings.TrimSpace(nombre)
	var resultado *domain.CadenaSupermercado
	errorTransaccion := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		if errorBloqueo := transaccion.Exec("SELECT pg_advisory_xact_lock(?, hashtext(?))", bloqueoCadena, strings.ToLower(nombreLimpio)).Error; errorBloqueo != nil {
			return errorBloqueo
		}
		var cadena domain.CadenaSupermercado
		errorBusqueda := transaccion.Where("nombre ILIKE ?", nombreLimpio).First(&cadena).Error
		if errorBusqueda == nil {
			resultado = &cadena
			return nil
		}
		if !errors.Is(errorBusqueda, gorm.ErrRecordNotFound) {
			return errorBusqueda
		}
		nueva := domain.CadenaSupermercado{Nombre: nombreLimpio, EstaActiva: true, CreadoEl: time.Now()}
		if errorCreacion := transaccion.Create(&nueva).Error; errorCreacion != nil {
			return errorCreacion
		}
		resultado = &nueva
		return nil
	})
	return resultado, errorTransaccion
}

// buscarCadenaPorID devuelve nil sin error si la cadena no existe.
func buscarCadenaPorID(baseDatos *gorm.DB, cadenaID int) (*domain.CadenaSupermercado, error) {
	var cadena domain.CadenaSupermercado
	if errorBusqueda := baseDatos.Where("id = ?", cadenaID).First(&cadena).Error; errorBusqueda != nil {
		if errors.Is(errorBusqueda, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorBusqueda
	}
	return &cadena, nil
}

// ===== Sucursales =====

// ObtenerSucursal busca por ID o código (filtrando por cadena si llega). Sólo
// con la cadena, devuelve su primera sucursal activa (por id ascendente). Nunca
// crea sucursales (SUP-376): antes inventaba una "<Cadena> Central" con dirección
// y coordenadas de Temuco que quedaban como reales y el motor de rutas las
// usaría. Si no hay ninguna sucursal activa devuelve nil sin error, y la ruta
// interna responde 404 branch_not_found; la sucursal real debe registrarse antes.
// Es una lectura simple, por lo que ya no necesita transacción ni advisory lock.
func (repositorio *RepositorioIngesta) ObtenerSucursal(contexto context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*domain.SucursalSupermercado, error) {
	consulta := repositorio.baseDatos.WithContext(contexto)
	if cadenaID > 0 {
		consulta = consulta.Where("cadena_id = ?", cadenaID)
	}
	switch {
	case sucursalID != nil && *sucursalID > 0:
		return primeraSucursal(consulta.Where("id = ?", *sucursalID))
	case codigoSucursal != nil && strings.TrimSpace(*codigoSucursal) != "":
		return primeraSucursal(consulta.Where("codigo_sucursal = ?", strings.TrimSpace(*codigoSucursal)))
	case cadenaID <= 0:
		return nil, nil // sin ningún criterio no hay sucursal que resolver
	}
	return primeraSucursal(consulta.Where("esta_activa = true").Order("id ASC"))
}

// primeraSucursal ejecuta la consulta y devuelve nil sin error si no hay fila.
func primeraSucursal(consulta *gorm.DB) (*domain.SucursalSupermercado, error) {
	var sucursal domain.SucursalSupermercado
	if errorBusqueda := consulta.First(&sucursal).Error; errorBusqueda != nil {
		if errors.Is(errorBusqueda, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorBusqueda
	}
	return &sucursal, nil
}

// ===== Lotes =====

// IngestarLote confirma productos, precios y recibo en una sola transacción.
// Un error SQL revierte el lote completo; nunca se continúa con una
// transacción PostgreSQL ya abortada. Con lote_id, el mismo contenido devuelve
// el recibo guardado y otro contenido devuelve domain.ErrIdempotencia.
func (repositorio *RepositorioIngesta) IngestarLote(contexto context.Context, lote domain.LoteIngestaDTO) (*domain.ResultadoIngestaDTO, error) {
	resultado := &domain.ResultadoIngestaDTO{SucursalID: lote.SucursalID, TotalRecibidos: len(lote.Productos)}
	errorTransaccion := repositorio.baseDatos.WithContext(contexto).Transaction(func(transaccion *gorm.DB) error {
		contenido, errorSerializacion := json.Marshal(lote)
		if errorSerializacion != nil {
			return errorSerializacion
		}
		resumen := sha256.Sum256(contenido)
		huella := hex.EncodeToString(resumen[:])
		if lote.LoteID != "" {
			reciboPrevio, errorRecibo := leerRecibo(transaccion, lote.LoteID, huella, resultado)
			if errorRecibo != nil || reciboPrevio {
				return errorRecibo
			}
		}
		ahora := time.Now()
		for indice, producto := range lote.Productos {
			if errorProducto := guardarProducto(transaccion, lote.SucursalID, indice, producto, ahora, resultado); errorProducto != nil {
				return errorProducto
			}
		}
		resultado.Mensaje = fmt.Sprintf("Lote procesado: %d insertados, %d actualizados, %d precios guardados", resultado.Insertados, resultado.Actualizados, resultado.PreciosRegistrados)
		if lote.LoteID == "" {
			return nil
		}
		recibo, errorRecibo := json.Marshal(resultado)
		if errorRecibo != nil {
			return errorRecibo
		}
		return transaccion.Exec(`INSERT INTO scraper.lotes_catalogo (id, payload_hash, resultado) VALUES (?, ?, ?::jsonb)`, lote.LoteID, huella, string(recibo)).Error
	})
	if errorTransaccion != nil {
		return nil, errorTransaccion
	}
	return resultado, nil
}

// leerRecibo toma el lock de la clave y, si el lote ya se confirmó, copia su
// resultado. Devuelve true cuando el recibo existe y coincide la huella.
func leerRecibo(transaccion *gorm.DB, loteID, huella string, resultado *domain.ResultadoIngestaDTO) (bool, error) {
	if errorBloqueo := transaccion.Exec(`SELECT pg_advisory_xact_lock(?, hashtext(?))`, bloqueoLote, loteID).Error; errorBloqueo != nil {
		return false, errorBloqueo
	}
	var recibo struct {
		PayloadHash string
		Resultado   string
	}
	consulta := transaccion.Raw(`SELECT payload_hash, resultado::text FROM scraper.lotes_catalogo WHERE id = ?`, loteID).Scan(&recibo)
	if consulta.Error != nil || consulta.RowsAffected == 0 {
		return false, consulta.Error
	}
	if recibo.PayloadHash != huella {
		return false, domain.ErrIdempotencia
	}
	return true, json.Unmarshal([]byte(recibo.Resultado), resultado)
}

// guardarProducto hace upsert del producto y registra su captura de precio. Un
// producto sin SKU o título se informa en Errores sin abortar el lote.
func guardarProducto(transaccion *gorm.DB, sucursalID, indice int, producto domain.ProductoScrapeadoDTO, ahora time.Time, resultado *domain.ResultadoIngestaDTO) error {
	sku, titulo := strings.TrimSpace(producto.SKU), strings.TrimSpace(producto.Producto)
	if sku == "" || titulo == "" {
		resultado.Errores = append(resultado.Errores, fmt.Sprintf("item %d: SKU o título vacío", indice))
		return nil
	}
	// url_imagen prevalece sobre el alias heredado imagen.
	var imagen *string
	if producto.URLImagen != nil && strings.TrimSpace(*producto.URLImagen) != "" {
		imagen = producto.URLImagen
	} else if producto.Imagen != nil && strings.TrimSpace(*producto.Imagen) != "" {
		imagen = producto.Imagen
	}
	enStock := true
	if producto.EnStock != nil {
		enStock = *producto.EnStock
	}
	var fila struct {
		ID        string
		Insertado bool
	}
	// uq_sucursal_sku serializa upserts concurrentes. NULL o vacío conserva los
	// campos opcionales del producto existente.
	consulta := transaccion.Raw(`INSERT INTO scraper.productos_crudos AS pc
    (sucursal_id, sku, ean_gtin, titulo_crudo, marca_cruda, categoria_cruda,
     formato_crudo, url_producto, url_imagen, en_stock, ultima_extraccion_el)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT (sucursal_id, sku) DO UPDATE SET
     titulo_crudo = EXCLUDED.titulo_crudo, en_stock = EXCLUDED.en_stock,
     ultima_extraccion_el = EXCLUDED.ultima_extraccion_el,
     ean_gtin = COALESCE(NULLIF(EXCLUDED.ean_gtin, ''), pc.ean_gtin),
     marca_cruda = COALESCE(NULLIF(EXCLUDED.marca_cruda, ''), pc.marca_cruda),
     categoria_cruda = COALESCE(NULLIF(EXCLUDED.categoria_cruda, ''), pc.categoria_cruda),
     formato_crudo = COALESCE(NULLIF(EXCLUDED.formato_crudo, ''), pc.formato_crudo),
     url_producto = COALESCE(NULLIF(EXCLUDED.url_producto, ''), pc.url_producto),
     url_imagen = COALESCE(NULLIF(EXCLUDED.url_imagen, ''), pc.url_imagen)
    RETURNING id, (xmax = 0) AS insertado`, sucursalID, sku, producto.EANGTIN,
		titulo, producto.Marca, producto.Categoria, producto.FormatoCrudo, producto.URLProducto, imagen, enStock, ahora).Scan(&fila)
	if consulta.Error != nil {
		return consulta.Error
	}
	if fila.Insertado {
		resultado.Insertados++
	} else {
		resultado.Actualizados++
	}
	// precio_oferta prevalece; el alias heredado precio sólo cuenta como oferta
	// si es menor que el normal.
	oferta := producto.PrecioOferta
	if oferta == nil && producto.Precio != nil && *producto.Precio < producto.PrecioNormal {
		oferta = producto.Precio
	}
	if errorCaptura := transaccion.Exec(`INSERT INTO scraper.capturas_precios
    (producto_crudo_id, precio_normal, precio_oferta, precio_tarjeta,
     precio_por_unidad, metrica_unidad, mecanica_promocion, esta_disponible, capturado_el)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, fila.ID, producto.PrecioNormal, oferta,
		producto.PrecioTarjeta, producto.PrecioPorUnidad, producto.MetricaUnidad, producto.MecanicaPromocion, enStock, ahora).Error; errorCaptura != nil {
		return errorCaptura
	}
	resultado.PreciosRegistrados++
	return nil
}
