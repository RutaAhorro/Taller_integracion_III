package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type gormProductoRepository struct {
	db *gorm.DB
}

// NewProductoRepository inicializa el repositorio de productos con GORM
func NewProductoRepository(db *gorm.DB) domain.ProductoRepository {
	return &gormProductoRepository{db: db}
}

// consultaBase centraliza los JOINs y los filtros comunes de Listar y ListarParaAdmin.
func (r *gormProductoRepository) consultaBase(ctx context.Context, filtro domain.FiltroProductosDTO) *gorm.DB {
	q := r.db.WithContext(ctx).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins("LEFT JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id ORDER BY capturado_el DESC LIMIT 1) cp ON true")

	if filtro.Query != "" {
		p := "%" + filtro.Query + "%"
		q = q.Where("(pc.titulo_crudo ILIKE ? OR pc.marca_cruda ILIKE ? OR pc.categoria_cruda ILIKE ? OR pc.sku ILIKE ?)", p, p, p, p)
	}
	if filtro.SKU != "" {
		q = q.Where("pc.sku ILIKE ?", "%"+filtro.SKU+"%")
	}
	if filtro.EnStock != nil {
		q = q.Where("pc.en_stock = ?", *filtro.EnStock)
	}
	if filtro.Supermercado != "" {
		q = q.Where("cs.nombre ILIKE ?", "%"+filtro.Supermercado+"%")
	}
	if filtro.Marca != "" {
		q = q.Where("pc.marca_cruda ILIKE ?", "%"+filtro.Marca+"%")
	}
	if filtro.PrecioMin != nil {
		q = q.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) >= ?", *filtro.PrecioMin)
	}
	if filtro.PrecioMax != nil {
		q = q.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) <= ?", *filtro.PrecioMax)
	}
	if filtro.EnOferta != nil {
		if *filtro.EnOferta {
			q = q.Where("cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal")
		} else {
			q = q.Where("(cp.precio_oferta IS NULL OR cp.precio_oferta >= cp.precio_normal)")
		}
	}
	return q
}

// filtroCategoriaPublico busca cada término de la categoría (singularizado) en el título.
func filtroCategoriaPublico(q *gorm.DB, categoria string) *gorm.DB {
	terms := strings.FieldsFunc(categoria, func(r rune) bool {
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
	if len(conditions) == 0 {
		return q
	}
	return q.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

// ordenSeguro construye la cláusula ORDER BY sólo con columnas permitidas.
func ordenSeguro(filtro domain.FiltroProductosDTO) string {
	dir := "ASC"
	if strings.EqualFold(filtro.Order, "desc") {
		dir = "DESC"
	}
	col := "COALESCE(cp.precio_oferta, cp.precio_normal, 0)" // "precio" por defecto
	switch strings.ToLower(filtro.SortBy) {
	case "nombre":
		col = "pc.titulo_crudo"
	case "marca":
		col = "pc.marca_cruda"
	case "sku":
		col = "pc.sku"
	case "fecha", "ultima_extraccion":
		col = "pc.ultima_extraccion_el"
	}
	return fmt.Sprintf("%s %s, pc.id ASC", col, dir)
}

// paginar aplica Limit/Offset a partir de Page y Limit del filtro.
func paginar(q *gorm.DB, filtro domain.FiltroProductosDTO) *gorm.DB {
	offset := (filtro.Page - 1) * filtro.Limit
	if offset < 0 {
		offset = 0
	}
	return q.Limit(filtro.Limit).Offset(offset)
}

// Listar ejecuta la consulta pública del catálogo. Los productos con stock se muestran primero.
func (r *gormProductoRepository) Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	q := r.consultaBase(ctx, filtro)
	if filtro.Categoria != "" {
		q = filtroCategoriaPublico(q, filtro.Categoria)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
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

	items := []domain.ProductoDTO{}
	err := paginar(q.Select(selectCols).Order("pc.en_stock DESC").Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListarParaAdmin ejecuta la consulta del dashboard administrativo y proyecta ProductoAdminDTO.
func (r *gormProductoRepository) ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	q := r.consultaBase(ctx, filtro)
	if filtro.Categoria != "" {
		q = q.Where("pc.categoria_cruda ILIKE ?", "%"+filtro.Categoria+"%")
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
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

	items := []domain.ProductoAdminDTO{}
	err := paginar(q.Select(selectCols).Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ObtenerPorID busca el producto por su UUID en scraper.productos_crudos e incluye su historial de precios
func (r *gormProductoRepository) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	// Un ID que no es UUID no puede existir: se responde "no encontrado" sin consultar la BD.
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
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

	var baseProd domain.ProductoDTO
	res := r.consultaBase(ctx, domain.FiltroProductosDTO{}).
		Where("pc.id = ?::uuid", id).
		Select(selectCols).
		Limit(1).
		Scan(&baseProd)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 || baseProd.ID == "" {
		return nil, nil
	}

	historial := []domain.HistorialPrecioDTO{}
	err := r.db.WithContext(ctx).
		Table("scraper.capturas_precios").
		Where("producto_crudo_id = ?::uuid", id).
		Order("capturado_el DESC").
		Select("precio_normal, precio_oferta, capturado_el").
		Scan(&historial).Error
	if err != nil {
		return nil, err
	}

	return &domain.ProductoDetalleDTO{
		ProductoDTO: baseProd,
		Historial:   historial,
	}, nil
}
