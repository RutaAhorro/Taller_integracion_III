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

// Parámetros de la escritura masiva. PostgreSQL admite 65535 parámetros por
// sentencia; con 11 columnas por producto, 5000 filas usan 55000 y dejan margen.
const (
	tamanoBloqueIngesta = 5000
	columnasProducto    = 11
	columnasCaptura     = 9
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
		nueva := domain.CadenaSupermercado{Nombre: nombreLimpio, EstaActiva: true, CreadoEl: time.Now().UTC()}
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
		// capturado_el es timestamp sin zona y el servicio compara en UTC;
		// guardar la hora local del proceso sesgaría la vigencia del precio.
		ahora := time.Now().UTC()

		productos, capturas := normalizarProductos(lote.Productos, resultado)
		// Sin ítems válidos no hay INSERT masivo vacío.
		if len(capturas) > 0 {
			idPorSKU, insertados, errorProductos := guardarProductos(transaccion, lote.SucursalID, productos, ahora)
			if errorProductos != nil {
				return errorProductos
			}
			if errorCapturas := guardarCapturas(transaccion, idPorSKU, capturas, ahora); errorCapturas != nil {
				return errorCapturas
			}
			// Cada aparición válida cuenta: la primera de un SKU nuevo es inserción y
			// las demás (también las repetidas dentro del lote) son actualizaciones.
			resultado.Insertados = insertados
			resultado.Actualizados = len(capturas) - insertados
			resultado.PreciosRegistrados = len(capturas)
		}
			// Cada aparición válida cuenta: la primera de un SKU nuevo es inserción y
			// las demás (también las repetidas dentro del lote) son actualizaciones.
			resultado.Insertados = insertados
			resultado.Actualizados = len(capturas) - insertados
			resultado.PreciosRegistrados = len(capturas)
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

// ===== Normalización y deduplicación en memoria =====

// productoNormalizado es un producto válido, ya recortado y con los alias
// resueltos, listo para el upsert. Un SKU repetido en el lote se fusiona en una
// sola instancia con el resultado que dejaría aplicar las apariciones en orden.
type productoNormalizado struct {
	sku, titulo                                                  string
	eanGTIN, marca, categoria, formatoCrudo, urlProducto, imagen *string
	enStock                                                      bool
}

// capturaPendiente es la captura de precio de UNA aparición de un producto:
// cada aparición en el lote registra su captura aunque el SKU se repita.
type capturaPendiente struct {
	sku                                          string
	precioNormal                                 float64
	precioOferta, precioTarjeta, precioPorUnidad *float64
	metricaUnidad, mecanicaPromocion             *string
	enStock                                      bool
}

// fusionarOpcional replica en memoria el COALESCE(NULLIF(...)) del upsert: un
// valor nulo o vacío conserva el anterior.
func fusionarOpcional(actual, nuevo *string) *string {
	if nuevo != nil && *nuevo != "" {
		return nuevo
	}
	return actual
}

// normalizarProductos valida y normaliza el lote en memoria. Los ítems sin SKU
// o título se informan en Errores con su índice original sin abortar el lote.
// Devuelve los productos únicos (en orden de primera aparición) y las capturas
// (una por aparición válida).
func normalizarProductos(productos []domain.ProductoScrapeadoDTO, resultado *domain.ResultadoIngestaDTO) ([]productoNormalizado, []capturaPendiente) {
	unicos := make([]productoNormalizado, 0, len(productos))
	posicionPorSKU := make(map[string]int, len(productos))
	capturas := make([]capturaPendiente, 0, len(productos))
	for indice, producto := range productos {
		sku, titulo := strings.TrimSpace(producto.SKU), strings.TrimSpace(producto.Producto)
		if sku == "" || titulo == "" {
			resultado.Errores = append(resultado.Errores, fmt.Sprintf("item %d: SKU o título vacío", indice))
			continue
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
		aparicion := productoNormalizado{
			sku: sku, titulo: titulo, eanGTIN: producto.EANGTIN, marca: producto.Marca,
			categoria: producto.Categoria, formatoCrudo: producto.FormatoCrudo,
			urlProducto: producto.URLProducto, imagen: imagen, enStock: enStock,
		}
		if posicion, repetido := posicionPorSKU[sku]; repetido {
			// El último gana en título y stock; los opcionales vacíos conservan lo previo.
			previo := &unicos[posicion]
			previo.titulo, previo.enStock = aparicion.titulo, aparicion.enStock
			previo.eanGTIN = fusionarOpcional(previo.eanGTIN, aparicion.eanGTIN)
			previo.marca = fusionarOpcional(previo.marca, aparicion.marca)
			previo.categoria = fusionarOpcional(previo.categoria, aparicion.categoria)
			previo.formatoCrudo = fusionarOpcional(previo.formatoCrudo, aparicion.formatoCrudo)
			previo.urlProducto = fusionarOpcional(previo.urlProducto, aparicion.urlProducto)
			previo.imagen = fusionarOpcional(previo.imagen, aparicion.imagen)
		} else {
			posicionPorSKU[sku] = len(unicos)
			unicos = append(unicos, aparicion)
		}
		// precio_oferta prevalece; el alias heredado precio sólo cuenta como oferta
		// si es menor que el normal.
		oferta := producto.PrecioOferta
		if oferta == nil && producto.Precio != nil && *producto.Precio < producto.PrecioNormal {
			oferta = producto.Precio
		}
		capturas = append(capturas, capturaPendiente{
			sku: sku, precioNormal: producto.PrecioNormal, precioOferta: oferta,
			precioTarjeta: producto.PrecioTarjeta, precioPorUnidad: producto.PrecioPorUnidad,
			metricaUnidad: producto.MetricaUnidad, mecanicaPromocion: producto.MecanicaPromocion, enStock: enStock,
		})
	}
	return unicos, capturas
}

// ===== Escritura masiva =====

// marcadoresFilas arma "(?, ?), (?, ?)..." para un INSERT multi-fila.
func marcadoresFilas(filas, columnas int) string {
	fila := "(" + strings.TrimSuffix(strings.Repeat("?, ", columnas), ", ") + ")"
	return strings.TrimSuffix(strings.Repeat(fila+", ", filas), ", ")
}

// guardarProductos hace upsert masivo por bloques y devuelve el ID de cada SKU y
// cuántos productos se insertaron (xmax = 0 distingue insertado de actualizado).
// Los SKU ya llegan únicos: PostgreSQL rechaza afectar una fila dos veces en un
// mismo INSERT ... ON CONFLICT. uq_sucursal_sku serializa upserts concurrentes;
// NULL o vacío conserva los campos opcionales del producto existente.
func guardarProductos(transaccion *gorm.DB, sucursalID int, productos []productoNormalizado, ahora time.Time) (map[string]string, int, error) {
	idPorSKU := make(map[string]string, len(productos))
	insertados := 0
	for inicio := 0; inicio < len(productos); inicio += tamanoBloqueIngesta {
		bloque := productos[inicio:min(inicio+tamanoBloqueIngesta, len(productos))]
		argumentos := make([]any, 0, len(bloque)*columnasProducto)
		for _, producto := range bloque {
			argumentos = append(argumentos, sucursalID, producto.sku, producto.eanGTIN, producto.titulo,
				producto.marca, producto.categoria, producto.formatoCrudo, producto.urlProducto,
				producto.imagen, producto.enStock, ahora)
		}
		var filas []struct {
			ID        string `gorm:"column:id"`
			SKU       string `gorm:"column:sku"`
			Insertado bool   `gorm:"column:insertado"`
		}
		consulta := transaccion.Raw(`INSERT INTO scraper.productos_crudos AS pc
    (sucursal_id, sku, ean_gtin, titulo_crudo, marca_cruda, categoria_cruda,
     formato_crudo, url_producto, url_imagen, en_stock, ultima_extraccion_el)
    VALUES `+marcadoresFilas(len(bloque), columnasProducto)+`
    ON CONFLICT (sucursal_id, sku) DO UPDATE SET
     titulo_crudo = EXCLUDED.titulo_crudo, en_stock = EXCLUDED.en_stock,
     ultima_extraccion_el = EXCLUDED.ultima_extraccion_el,
     ean_gtin = COALESCE(NULLIF(EXCLUDED.ean_gtin, ''), pc.ean_gtin),
     marca_cruda = COALESCE(NULLIF(EXCLUDED.marca_cruda, ''), pc.marca_cruda),
     categoria_cruda = COALESCE(NULLIF(EXCLUDED.categoria_cruda, ''), pc.categoria_cruda),
     formato_crudo = COALESCE(NULLIF(EXCLUDED.formato_crudo, ''), pc.formato_crudo),
     url_producto = COALESCE(NULLIF(EXCLUDED.url_producto, ''), pc.url_producto),
     url_imagen = COALESCE(NULLIF(EXCLUDED.url_imagen, ''), pc.url_imagen)
    RETURNING id, sku, (xmax = 0) AS insertado`, argumentos...).Scan(&filas)
		if consulta.Error != nil {
			return nil, 0, consulta.Error
		}
		for _, fila := range filas {
			idPorSKU[fila.SKU] = fila.ID
			if fila.Insertado {
				insertados++
			}
		}
	}
	return idPorSKU, insertados, nil
}

// guardarCapturas inserta por bloques una captura por cada aparición válida.
func guardarCapturas(transaccion *gorm.DB, idPorSKU map[string]string, capturas []capturaPendiente, ahora time.Time) error {
	for inicio := 0; inicio < len(capturas); inicio += tamanoBloqueIngesta {
		bloque := capturas[inicio:min(inicio+tamanoBloqueIngesta, len(capturas))]
		argumentos := make([]any, 0, len(bloque)*columnasCaptura)
		for _, captura := range bloque {
			argumentos = append(argumentos, idPorSKU[captura.sku], captura.precioNormal, captura.precioOferta,
				captura.precioTarjeta, captura.precioPorUnidad, captura.metricaUnidad,
				captura.mecanicaPromocion, captura.enStock, ahora)
		}
		if errorCaptura := transaccion.Exec(`INSERT INTO scraper.capturas_precios
    (producto_crudo_id, precio_normal, precio_oferta, precio_tarjeta,
     precio_por_unidad, metrica_unidad, mecanica_promocion, esta_disponible, capturado_el)
    VALUES `+marcadoresFilas(len(bloque), columnasCaptura), argumentos...).Error; errorCaptura != nil {
			return errorCaptura
		}
	}
	return nil
}
