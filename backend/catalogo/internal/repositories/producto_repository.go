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
func (repositorio *gormProductoRepository) consultaBase(ctx context.Context, filtro domain.FiltroProductosDTO) *gorm.DB {
	consulta := repositorio.db.WithContext(ctx).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id").
		Joins("LEFT JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id ORDER BY capturado_el DESC LIMIT 1) cp ON true")

	if filtro.Query != "" {
		terminoBusqueda := "%" + filtro.Query + "%"
		consulta = consulta.Where("(pc.titulo_crudo ILIKE ? OR pc.marca_cruda ILIKE ? OR pc.categoria_cruda ILIKE ? OR pc.sku ILIKE ?)", terminoBusqueda, terminoBusqueda, terminoBusqueda, terminoBusqueda)
	}
	if filtro.SKU != "" {
		consulta = consulta.Where("pc.sku ILIKE ?", "%"+filtro.SKU+"%")
	}
	if filtro.EnStock != nil {
		consulta = consulta.Where("pc.en_stock = ?", *filtro.EnStock)
	}
	if filtro.Supermercado != "" {
		consulta = consulta.Where("cs.nombre ILIKE ?", "%"+filtro.Supermercado+"%")
	}
	if filtro.Marca != "" {
		consulta = consulta.Where("pc.marca_cruda ILIKE ?", "%"+filtro.Marca+"%")
	}
	if filtro.PrecioMin != nil {
		consulta = consulta.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) >= ?", *filtro.PrecioMin)
	}
	if filtro.PrecioMax != nil {
		consulta = consulta.Where("COALESCE(cp.precio_oferta, cp.precio_normal, 0) <= ?", *filtro.PrecioMax)
	}
	if filtro.EnOferta != nil {
		if *filtro.EnOferta {
			consulta = consulta.Where("cp.precio_oferta IS NOT NULL AND cp.precio_oferta < cp.precio_normal")
		} else {
			consulta = consulta.Where("(cp.precio_oferta IS NULL OR cp.precio_oferta >= cp.precio_normal)")
		}
	}
	return consulta
}

// filtroCategoriaPublico busca cada término de la categoría (singularizado) en el título.
func filtroCategoriaPublico(consulta *gorm.DB, categoria string) *gorm.DB {
	terms := strings.FieldsFunc(categoria, func(caracter rune) bool {
		return caracter == ' ' || caracter == ',' || caracter == '/' || caracter == ';'
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
		return consulta
	}
	return consulta.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

// ordenSeguro construye la cláusula ORDER BY sólo con columnas permitidas.
func ordenSeguro(filtro domain.FiltroProductosDTO) string {
	dir := "ASC"
	if strings.EqualFold(filtro.Order, "desc") {
		dir = "DESC"
	}
	switch strings.ToLower(filtro.SortBy) {
	case "nombre":
		return fmt.Sprintf("pc.titulo_crudo %s, pc.id ASC", dir)
	case "marca":
		return fmt.Sprintf("pc.marca_cruda %s, pc.id ASC", dir)
	case "sku":
		return fmt.Sprintf("pc.sku %s, pc.id ASC", dir)
	case "fecha", "ultima_extraccion":
		return fmt.Sprintf("pc.ultima_extraccion_el %s, pc.id ASC", dir)
	default: // "precio" por defecto: precios faltantes (NULL) al final
		return fmt.Sprintf("COALESCE(cp.precio_oferta, cp.precio_normal) %s NULLS LAST, pc.id ASC", dir)
	}
}

// paginar aplica Limit/Offset a partir de Page y Limit del filtro.
func paginar(consulta *gorm.DB, filtro domain.FiltroProductosDTO) *gorm.DB {
	offset := (filtro.Page - 1) * filtro.Limit
	if offset < 0 {
		offset = 0
	}
	return consulta.Limit(filtro.Limit).Offset(offset)
}

// Listar ejecuta la consulta pública del catálogo. Los productos con stock se muestran primero.
func (repositorio *gormProductoRepository) Listar(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	consulta := repositorio.consultaBase(ctx, filtro)
	if filtro.Categoria != "" {
		consulta = filtroCategoriaPublico(consulta, filtro.Categoria)
	}

	var total int64
	if err := consulta.Session(&gorm.Session{}).Count(&total).Error; err != nil {
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
	err := paginar(consulta.Select(selectCols).Order("pc.en_stock DESC").Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListarParaAdmin ejecuta la consulta del dashboard administrativo y proyecta ProductoAdminDTO.
func (repositorio *gormProductoRepository) ListarParaAdmin(ctx context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	consulta := repositorio.consultaBase(ctx, filtro)
	if filtro.Categoria != "" {
		consulta = consulta.Where("pc.categoria_cruda ILIKE ?", "%"+filtro.Categoria+"%")
	}

	var total int64
	if err := consulta.Session(&gorm.Session{}).Count(&total).Error; err != nil {
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
	err := paginar(consulta.Select(selectCols).Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ObtenerPorID busca el producto por su UUID en scraper.productos_crudos e incluye su historial de precios
func (repositorio *gormProductoRepository) ObtenerPorID(ctx context.Context, id string) (*domain.ProductoDetalleDTO, error) {
	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, nil
	}
	canonicalID := parsedUUID.String()

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
	res := repositorio.consultaBase(ctx, domain.FiltroProductosDTO{}).
		Where("pc.id = ?::uuid", canonicalID).
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
	err = repositorio.db.WithContext(ctx).
		Table("scraper.capturas_precios").
		Where("producto_crudo_id = ?::uuid", canonicalID).
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
