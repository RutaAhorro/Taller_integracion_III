package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"gorm.io/gorm"
)

type gormProductoRepository struct {
	db *gorm.DB
}

// NewProductoRepository inicializa el repositorio de productos con GORM
func NewProductoRepository(db *gorm.DB) domain.ProductoRepository {
	return &gormProductoRepository{db: db}
}

// Listar ejecuta la consulta dinámica sobre scraper.productos_crudos con joins a sucursales,
// cadenas y capturas de precio, aplicando filtros, ordenamiento y paginación.
func (r *gormProductoRepository) Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	// 1. Consulta base uniendo productos, sucursales, cadenas y la captura de precio más reciente
	baseQuery := r.db.WithContext(ctx).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins("LEFT JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id ORDER BY capturado_el DESC LIMIT 1) cp ON true")

	// 2. Encadenamiento dinámico de cláusulas .Where()
	if filtro.Query != "" {
		qPattern := "%" + filtro.Query + "%"
		baseQuery = baseQuery.Where("(pc.titulo_crudo ILIKE ? OR pc.marca_cruda ILIKE ? OR pc.categoria_cruda ILIKE ?)", qPattern, qPattern, qPattern)
	}

	if filtro.Categoria != "" {
		terms := strings.FieldsFunc(filtro.Categoria, func(r rune) bool {
			return r == ' ' || r == ',' || r == '/' || r == ';'
		})
		conditions := make([]string, 0, len(terms))
		args := make([]interface{}, 0, len(terms))
		for _, term := range terms {
			if strings.EqualFold(term, "y") || strings.EqualFold(term, "e") {
				continue
			}
			term = strings.ToLower(term)
			switch {
			case strings.HasSuffix(term, "es") && len(term) > 4:
				term = strings.TrimSuffix(term, "es")
			case strings.HasSuffix(term, "s") && len(term) > 3:
				term = strings.TrimSuffix(term, "s")
			}
			conditions = append(conditions, "pc.titulo_crudo ILIKE ?")
			args = append(args, "%"+term+"%")
		}
		if len(conditions) > 0 {
			baseQuery = baseQuery.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}
	}

	if filtro.Supermercado != "" {
		baseQuery = baseQuery.Where("cs.nombre ILIKE ?", "%"+filtro.Supermercado+"%")
	}

	if filtro.Marca != "" {
		baseQuery = baseQuery.Where("pc.marca_cruda ILIKE ?", "%"+filtro.Marca+"%")
	}

	if filtro.PrecioMin != nil {
		baseQuery = baseQuery.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) >= ?", *filtro.PrecioMin)
	}

	if filtro.PrecioMax != nil {
		baseQuery = baseQuery.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) <= ?", *filtro.PrecioMax)
	}

	if filtro.EnOferta != nil {
		if *filtro.EnOferta {
			baseQuery = baseQuery.Where("cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal")
		} else {
			baseQuery = baseQuery.Where("cp.precio_oferta IS NULL OR cp.precio_oferta >= cp.precio_normal")
		}
	}

	// 3. Conteo total de registros sin paginar
	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 4. Configurar cláusula de ordenamiento seguro
	direction := "ASC"
	if strings.EqualFold(filtro.Order, "desc") {
		direction = "DESC"
	}

	var orderClause string
	switch strings.ToLower(filtro.SortBy) {
	case "nombre":
		orderClause = fmt.Sprintf("pc.titulo_crudo %s, pc.id ASC", direction)
	case "marca":
		orderClause = fmt.Sprintf("pc.marca_cruda %s, pc.id ASC", direction)
	default: // "precio" por defecto
		orderClause = fmt.Sprintf("COALESCE(cp.precio_oferta, cp.precio_normal, 0) %s, pc.id ASC", direction)
	}

	// 5. Paginación (.Offset y .Limit)
	offset := (filtro.Page - 1) * filtro.Limit
	if offset < 0 {
		offset = 0
	}

	selectCols := `
		pc.id::text AS id,
		pc.titulo_crudo AS nombre,
		COALESCE(pc.marca_cruda, '') AS marca,
		COALESCE(pc.categoria_cruda, '') AS categoria,
		cs.nombre AS supermercado,
		COALESCE(cp.precio_oferta, cp.precio_normal, 0) AS precio,
		COALESCE(cp.precio_normal, 0) AS precio_normal,
		cp.precio_oferta,
		(cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal) AS en_oferta,
		COALESCE(pc.url_imagen, '') AS url_imagen,
		COALESCE(pc.formato_crudo, '') AS unidad,
		pc.en_stock
	`

	var items []domain.ProductoDTO
	err := baseQuery.
		Select(selectCols).
		Order(orderClause).
		Limit(filtro.Limit).
		Offset(offset).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	if items == nil {
		items = []domain.ProductoDTO{}
	}

	return items, total, nil
}

// ListarParaAdmin ejecuta la consulta dinámica para el dashboard administrativo,
// agregando filtros por SKU y disponibilidad de stock, y proyectando ProductoAdminDTO con datos técnicos.
func (r *gormProductoRepository) ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	// 1. Consulta base uniendo productos, sucursales, cadenas y la captura de precio más reciente
	baseQuery := r.db.WithContext(ctx).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins("LEFT JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id ORDER BY capturado_el DESC LIMIT 1) cp ON true")

	// 2. Encadenamiento dinámico de cláusulas .Where()
	if filtro.Query != "" {
		qPattern := "%" + filtro.Query + "%"
		baseQuery = baseQuery.Where("(pc.titulo_crudo ILIKE ? OR pc.marca_cruda ILIKE ? OR pc.categoria_cruda ILIKE ? OR pc.sku ILIKE ?)", qPattern, qPattern, qPattern, qPattern)
	}

	if filtro.SKU != "" {
		baseQuery = baseQuery.Where("pc.sku ILIKE ?", "%"+filtro.SKU+"%")
	}

	if filtro.EnStock != nil {
		baseQuery = baseQuery.Where("pc.en_stock = ?", *filtro.EnStock)
	}

	if filtro.Categoria != "" {
		baseQuery = baseQuery.Where("pc.categoria_cruda ILIKE ?", "%"+filtro.Categoria+"%")
	}

	if filtro.Supermercado != "" {
		baseQuery = baseQuery.Where("cs.nombre ILIKE ?", "%"+filtro.Supermercado+"%")
	}

	if filtro.Marca != "" {
		baseQuery = baseQuery.Where("pc.marca_cruda ILIKE ?", "%"+filtro.Marca+"%")
	}

	if filtro.PrecioMin != nil {
		baseQuery = baseQuery.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) >= ?", *filtro.PrecioMin)
	}

	if filtro.PrecioMax != nil {
		baseQuery = baseQuery.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) <= ?", *filtro.PrecioMax)
	}

	if filtro.EnOferta != nil {
		if *filtro.EnOferta {
			baseQuery = baseQuery.Where("cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal")
		} else {
			baseQuery = baseQuery.Where("cp.precio_oferta IS NULL OR cp.precio_oferta >= cp.precio_normal")
		}
	}

	// 3. Conteo total de registros sin paginar
	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 4. Configurar cláusula de ordenamiento seguro
	direction := "ASC"
	if strings.EqualFold(filtro.Order, "desc") {
		direction = "DESC"
	}

	var orderClause string
	switch strings.ToLower(filtro.SortBy) {
	case "nombre":
		orderClause = fmt.Sprintf("pc.titulo_crudo %s, pc.id ASC", direction)
	case "marca":
		orderClause = fmt.Sprintf("pc.marca_cruda %s, pc.id ASC", direction)
	case "sku":
		orderClause = fmt.Sprintf("pc.sku %s, pc.id ASC", direction)
	case "fecha", "ultima_extraccion":
		orderClause = fmt.Sprintf("pc.ultima_extraccion_el %s, pc.id ASC", direction)
	default: // "precio" por defecto
		orderClause = fmt.Sprintf("COALESCE(cp.precio_oferta, cp.precio_normal, 0) %s, pc.id ASC", direction)
	}

	// 5. Paginación (.Offset y .Limit)
	offset := (filtro.Page - 1) * filtro.Limit
	if offset < 0 {
		offset = 0
	}

	selectCols := `
		pc.id::text AS id,
		COALESCE(pc.sku, '') AS sku,
		pc.titulo_crudo AS nombre,
		COALESCE(pc.marca_cruda, '') AS marca,
		COALESCE(pc.categoria_cruda, '') AS categoria,
		cs.nombre AS supermercado,
		ROUND(COALESCE(cp.precio_normal, 0))::int AS precio_normal,
		ROUND(cp.precio_oferta)::int AS precio_oferta,
		(cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal) AS en_oferta,
		COALESCE(pc.url_imagen, '') AS url_imagen,
		COALESCE(pc.formato_crudo, '') AS unidad,
		pc.en_stock,
		pc.ultima_extraccion_el,
		pc.sucursal_id
	`

	var items []domain.ProductoAdminDTO
	err := baseQuery.
		Select(selectCols).
		Order(orderClause).
		Limit(filtro.Limit).
		Offset(offset).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	if items == nil {
		items = []domain.ProductoAdminDTO{}
	}

	return items, total, nil
}

// ObtenerPorID busca el producto por su UUID en scraper.productos_crudos e incluye su historial de precios
func (r *gormProductoRepository) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	selectCols := `
		pc.id::text AS id,
		pc.titulo_crudo AS nombre,
		COALESCE(pc.marca_cruda, '') AS marca,
		COALESCE(pc.categoria_cruda, '') AS categoria,
		cs.nombre AS supermercado,
		COALESCE(cp.precio_oferta, cp.precio_normal, 0) AS precio,
		COALESCE(cp.precio_normal, 0) AS precio_normal,
		cp.precio_oferta,
		(cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal) AS en_oferta,
		COALESCE(pc.url_imagen, '') AS url_imagen,
		COALESCE(pc.formato_crudo, '') AS unidad,
		pc.en_stock
	`

	var baseProd domain.ProductoDTO
	res := r.db.WithContext(ctx).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins("LEFT JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id ORDER BY capturado_el DESC LIMIT 1) cp ON true").
		Where("pc.id::text = ?", id).
		Select(selectCols).
		Limit(1).
		Scan(&baseProd)

	if res.Error != nil {
		return nil, res.Error
	}

	if res.RowsAffected == 0 || baseProd.ID == "" {
		return nil, nil
	}

	var historial []domain.HistorialPrecioDTO
	err := r.db.WithContext(ctx).
		Table("scraper.capturas_precios").
		Where("producto_crudo_id::text = ?", id).
		Order("capturado_el DESC").
		Select("precio_normal, precio_oferta, capturado_el").
		Scan(&historial).Error

	if err != nil {
		return nil, err
	}

	if historial == nil {
		historial = []domain.HistorialPrecioDTO{}
	}

	return &domain.ProductoDetalleDTO{
		ProductoDTO: baseProd,
		Historial:   historial,
	}, nil
}
