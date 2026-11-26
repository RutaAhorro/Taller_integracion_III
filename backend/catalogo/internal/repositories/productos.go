// Package repositories implementa las consultas SQL del catálogo sobre las
// tablas scraper.* que pertenecen a Catálogo, y la verificación de permisos al
// arrancar. Nunca lee tablas de Identidad ni de Control.
package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ===== Repositorio =====

// RepositorioProductos lee productos crudos con su captura más reciente.
type RepositorioProductos struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioProductos crea el repositorio sobre la conexión compartida.
func NuevoRepositorioProductos(baseDatos *gorm.DB) *RepositorioProductos {
	return &RepositorioProductos{baseDatos: baseDatos}
}

// ===== Marcas propias =====

// marcasPropiasPorMarca lista los patrones de marcas propias de las cadenas
// chilenas que se buscan en marca_cruda.
var marcasPropiasPorMarca = []string{
	"%lider%",
	"%líder%",
	"%great value%",
	"%a cuenta%",
	"%acuenta%",
	"%ekono%",
	"%seleccion%",
	"%selección%",
	"%jumbo%",
	"%cuisine & co%",
	"%cuisine&co%",
	"%santa isabel%",
	"%maxima%",
	"%máxima%",
	"%tottus%",
	"%precio uno%",
	"%unimarc%",
	"%merkat%",
	"%nuestra cocina%",
	"%cugat%",
}

// marcasPropiasPorTitulo es un subconjunto más estricto para productos sin marca:
// excluye términos que también son palabras comunes en los títulos (por ejemplo "jumbo",
// "seleccion" o "maxima").
var marcasPropiasPorTitulo = []string{
	"%great value%",
	"%cuisine & co%",
	"%cuisine&co%",
	"%a cuenta%",
	"%acuenta%",
	"%santa isabel%",
	"%precio uno%",
	"%merkat%",
	"%nuestra cocina%",
	"%lider%",
	"%líder%",
	"%tottus%",
	"%unimarc%",
	"%ekono%",
	"%cugat%",
}

// ===== Consultas =====

// opcionesConsulta ajusta consultaBase sin duplicar la lógica de filtros.
type opcionesConsulta struct {
	// omitirSupermercado deja fuera sólo el filtro de cadena: la agregación de
	// supermercados disponibles debe conservar las alternativas al elegir una.
	omitirSupermercado bool
	// sinCapturas evita el JOIN LATERAL de la última captura cuando ningún
	// filtro ni columna lo necesita (la agregación sólo lee la cadena).
	sinCapturas bool
}

// filtroUsaPrecios indica si algún filtro lee la última captura (cp.*).
// Todo filtro nuevo de consultaBase que lea cp.* debe sumarse aquí: si no, la
// agregación de supermercados (sinCapturas) fallaría por falta del JOIN.
func filtroUsaPrecios(filtro domain.FiltroProductosDTO) bool {
	return filtro.PrecioMin != nil || filtro.PrecioMax != nil || filtro.EnOferta != nil
}

// consultaBase centraliza los JOINs y los filtros comunes de Listar,
// ListarParaAdmin y ListarSupermercados. Es el ÚNICO lugar donde se traducen
// los filtros a SQL: cualquier filtro nuevo (p. ej. marcas blancas) se agrega
// aquí y lo heredan el listado, el total y los supermercados disponibles.
func (repositorio *RepositorioProductos) consultaBase(contexto context.Context, filtro domain.FiltroProductosDTO, opciones opcionesConsulta) *gorm.DB {
	consulta := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.productos_crudos pc").
		Joins("JOIN scraper.sucursales_supermercado ss ON pc.sucursal_id = ss.id").
		Joins("JOIN scraper.cadenas_supermercado cs ON ss.cadena_id = cs.id")
	if !opciones.sinCapturas || filtroUsaPrecios(filtro) {
		// JOIN LATERAL descarta productos sin ninguna captura válida (>= PrecioMinimoValido)
		// y toma la captura válida más reciente ignorando precios basura.
		consulta = consulta.Joins(
			"JOIN LATERAL (SELECT precio_normal, precio_oferta, esta_disponible FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id AND precio_normal >= ? ORDER BY capturado_el DESC LIMIT 1) cp ON true",
			domain.PrecioMinimoValido,
		)
	} else {
		// Cuando sólo se agregan supermercados se evita el LATERAL, pero se exige
		// al menos una captura válida para coincidir con el listado publicado.
		consulta = consulta.Where(
			"EXISTS (SELECT 1 FROM scraper.capturas_precios WHERE producto_crudo_id = pc.id AND precio_normal >= ?)",
			domain.PrecioMinimoValido,
		)
	}

	if filtro.Query != "" {
		terminoBusqueda := "%" + filtro.Query + "%"
		consulta = consulta.Where("(pc.titulo_crudo ILIKE ? OR pc.marca_cruda ILIKE ? OR pc.categoria_cruda ILIKE ? OR pc.sku ILIKE ?)", terminoBusqueda, terminoBusqueda, terminoBusqueda, terminoBusqueda)
	}
	if filtro.CategoriaExacta != "" {
		consulta = consulta.Where("BTRIM(pc.categoria_cruda) = ?", filtro.CategoriaExacta)
	}
	if filtro.SKU != "" {
		consulta = consulta.Where("pc.sku ILIKE ?", "%"+filtro.SKU+"%")
	}
	if filtro.EnStock != nil {
		consulta = consulta.Where("pc.en_stock = ?", *filtro.EnStock)
	}
	if filtro.Supermercado != "" && !opciones.omitirSupermercado {
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
	if filtro.SoloMarcasBlancas != nil && *filtro.SoloMarcasBlancas {
		consulta = consulta.Where(`(
			pc.marca_cruda ILIKE ANY(?)
			OR (
				COALESCE(pc.marca_cruda, '') = '' AND pc.titulo_crudo ILIKE ANY(?)
			)
		)`, pq.Array(marcasPropiasPorMarca), pq.Array(marcasPropiasPorTitulo))
	}
	return consulta
}

// ListarCategorias consulta todo el catálogo sin paginar ni cargar productos o
// capturas: devuelve los valores existentes, sin vacíos ni espacios exteriores.
func (repositorio *RepositorioProductos) ListarCategorias(contexto context.Context) ([]string, error) {
	categorias := []string{}
	errorConsulta := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.productos_crudos").
		Distinct("BTRIM(categoria_cruda) AS categoria").
		Where("BTRIM(categoria_cruda) <> ''").
		Order("categoria ASC").
		Pluck("BTRIM(categoria_cruda) AS categoria", &categorias).Error
	return categorias, errorConsulta
}

// Reglas de la categoría pública: separadores, conectores ignorados y largo
// mínimo para quitar el plural sin dejar raíces demasiado cortas.
const (
	separadoresCategoria = " ,/;"
	largoMinimoPluralEs  = 4
	largoMinimoPluralS   = 3
)

var conectoresCategoria = map[string]bool{"y": true, "e": true}

// filtroCategoriaPublico busca cada término de la categoría (singularizado) en el título.
func filtroCategoriaPublico(consulta *gorm.DB, categoria string) *gorm.DB {
	terminos := strings.FieldsFunc(categoria, func(caracter rune) bool {
		return strings.ContainsRune(separadoresCategoria, caracter)
	})
	condiciones := make([]string, 0, len(terminos))
	argumentos := make([]any, 0, len(terminos))
	for _, termino := range terminos {
		termino = strings.ToLower(termino)
		if conectoresCategoria[termino] {
			continue
		}
		switch {
		case strings.HasSuffix(termino, "es") && len(termino) > largoMinimoPluralEs:
			termino = strings.TrimSuffix(termino, "es")
		case strings.HasSuffix(termino, "s") && len(termino) > largoMinimoPluralS:
			termino = strings.TrimSuffix(termino, "s")
		}
		condiciones = append(condiciones, "pc.titulo_crudo ILIKE ?")
		argumentos = append(argumentos, "%"+termino+"%")
	}
	if len(condiciones) == 0 {
		return consulta
	}
	return consulta.Where("("+strings.Join(condiciones, " OR ")+")", argumentos...)
}

// consultaPublica es la consulta base más la categoría pública. La comparten el
// listado, su total y los supermercados disponibles para que coincidan siempre.
func (repositorio *RepositorioProductos) consultaPublica(contexto context.Context, filtro domain.FiltroProductosDTO, opciones opcionesConsulta) *gorm.DB {
	consulta := repositorio.consultaBase(contexto, filtro, opciones)
	if filtro.Categoria != "" {
		consulta = filtroCategoriaPublico(consulta, filtro.Categoria)
	}
	return consulta
}

// ordenSeguro construye la cláusula ORDER BY sólo con columnas permitidas.
func ordenSeguro(filtro domain.FiltroProductosDTO) string {
	direccion := "ASC"
	if strings.EqualFold(filtro.Order, "desc") {
		direccion = "DESC"
	}
	switch strings.ToLower(filtro.SortBy) {
	case "nombre":
		return fmt.Sprintf("pc.titulo_crudo %s, pc.id ASC", direccion)
	case "marca":
		return fmt.Sprintf("pc.marca_cruda %s, pc.id ASC", direccion)
	case "sku":
		return fmt.Sprintf("pc.sku %s, pc.id ASC", direccion)
	case "fecha", "ultima_extraccion":
		return fmt.Sprintf("pc.ultima_extraccion_el %s, pc.id ASC", direccion)
	default: // "precio" por defecto: precios faltantes (NULL) al final
		return fmt.Sprintf("COALESCE(cp.precio_oferta, cp.precio_normal) %s NULLS LAST, pc.id ASC", direccion)
	}
}

// paginar aplica Limit/Offset; el servicio garantiza Page >= 1.
func paginar(consulta *gorm.DB, filtro domain.FiltroProductosDTO) *gorm.DB {
	return consulta.Limit(filtro.Limit).Offset((filtro.Page - 1) * filtro.Limit)
}

// Listar ejecuta la consulta pública del catálogo. Los productos con stock se muestran primero.
func (repositorio *RepositorioProductos) Listar(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoDTO, int64, error) {
	consulta := repositorio.consultaPublica(contexto, filtro, opcionesConsulta{})

	var total int64
	if errorConteo := consulta.Session(&gorm.Session{}).Count(&total).Error; errorConteo != nil {
		return nil, 0, errorConteo
	}

	columnas := `
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
	errorConsulta := paginar(consulta.Select(columnas).Order("pc.en_stock DESC").Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if errorConsulta != nil {
		return nil, 0, errorConsulta
	}
	return items, total, nil
}

// ListarSupermercados devuelve las cadenas con al menos un producto que cumple
// TODOS los filtros activos salvo el de supermercado, sobre el conjunto
// completo (sin LIMIT/OFFSET): no depende de la página ni del tamaño. Respeta
// en_stock como el listado: sin el parámetro cuentan también los agotados. El
// orden (nombre, id) es estable y la lista nunca es nil.
func (repositorio *RepositorioProductos) ListarSupermercados(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.SupermercadoDTO, error) {
	supermercados := []domain.SupermercadoDTO{}
	errorConsulta := repositorio.consultaPublica(contexto, filtro, opcionesConsulta{omitirSupermercado: true, sinCapturas: true}).
		Select("DISTINCT cs.id AS id, cs.nombre AS nombre").
		Order("cs.nombre ASC, cs.id ASC").
		Scan(&supermercados).Error
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	return supermercados, nil
}

// ListarParaAdmin ejecuta la consulta del dashboard administrativo y proyecta ProductoAdminDTO.
func (repositorio *RepositorioProductos) ListarParaAdmin(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.ProductoAdminDTO, int64, error) {
	consulta := repositorio.consultaBase(contexto, filtro, opcionesConsulta{})
	if filtro.Categoria != "" {
		consulta = consulta.Where("pc.categoria_cruda ILIKE ?", "%"+filtro.Categoria+"%")
	}

	var total int64
	if errorConteo := consulta.Session(&gorm.Session{}).Count(&total).Error; errorConteo != nil {
		return nil, 0, errorConteo
	}

	columnas := `
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
	errorConsulta := paginar(consulta.Select(columnas).Order(ordenSeguro(filtro)), filtro).
		Scan(&items).Error
	if errorConsulta != nil {
		return nil, 0, errorConsulta
	}
	return items, total, nil
}

// ObtenerPorID busca el producto por su UUID en scraper.productos_crudos e incluye su historial de precios
func (repositorio *RepositorioProductos) ObtenerPorID(contexto context.Context, productoID string) (*domain.ProductoDetalleDTO, error) {
	identificador, errorParseo := uuid.Parse(productoID)
	if errorParseo != nil {
		return nil, nil // un ID que no es UUID no puede existir: 404
	}
	idCanonico := identificador.String()

	columnas := `
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

	var producto domain.ProductoDTO
	resultado := repositorio.consultaBase(contexto, domain.FiltroProductosDTO{}, opcionesConsulta{}).
		Where("pc.id = ?::uuid", idCanonico).
		Select(columnas).
		Limit(1).
		Scan(&producto)
	if resultado.Error != nil {
		return nil, resultado.Error
	}
	if resultado.RowsAffected == 0 || producto.ID == "" {
		return nil, nil
	}

	historial := []domain.HistorialPrecioDTO{}
	errorHistorial := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.capturas_precios").
		Where("producto_crudo_id = ?::uuid", idCanonico).
		Where("precio_normal >= ?", domain.PrecioMinimoValido).
		Order("capturado_el DESC").
		Select("precio_normal, precio_oferta, capturado_el").
		Scan(&historial).Error
	if errorHistorial != nil {
		return nil, errorHistorial
	}

	return &domain.ProductoDetalleDTO{
		ProductoDTO: producto,
		Historial:   historial,
	}, nil
}
