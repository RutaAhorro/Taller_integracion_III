// Precios comparables: lectura geoespacial de sucursales con PostGIS. El
// filtro por radio corre en SQL para que el índice GiST acote la consulta.

package repositories

import (
	"context"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
	"gorm.io/gorm"
)

// ===== Expresiones PostGIS =====

// puntoSucursal reusa la columna geography funcional que indexa la migración
// 13_ sin agregar columnas a la tabla compartida con Control. Se califica con
// public. porque app_catalogo opera con search_path = pg_catalog.
const puntoSucursal = `public.ST_SetSRID(public.ST_MakePoint(ss.lon, ss.lat), 4326)::public.geography`

// puntoOrigen recibe latitud y longitud en ese orden desde el servicio.
const puntoOrigen = `public.ST_SetSRID(public.ST_MakePoint(?, ?), 4326)::public.geography`

// ===== Repositorio =====

// RepositorioPreciosComparables implementa domain.RepositorioPreciosComparables.
type RepositorioPreciosComparables struct {
	baseDatos *gorm.DB
}

// NuevoRepositorioPreciosComparables crea el repositorio sobre la conexión
// compartida del servicio.
func NuevoRepositorioPreciosComparables(baseDatos *gorm.DB) *RepositorioPreciosComparables {
	return &RepositorioPreciosComparables{baseDatos: baseDatos}
}

// ===== Consultas =====

// ConsultarCercanas devuelve como máximo maximo sucursales físicas y activas
// dentro del radio, ordenadas por distancia real al origen. La condición
// ST_DWithin coincide con el predicado del índice parcial de la migración 13_.
func (repositorio *RepositorioPreciosComparables) ConsultarCercanas(contexto context.Context, latitud, longitud, radioKm float64, maximo int) ([]domain.SucursalCercanaRegistro, error) {
	registros := []domain.SucursalCercanaRegistro{}

	consulta := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.sucursales_supermercado ss").
		Joins("JOIN scraper.cadenas_supermercado cs ON cs.id = ss.cadena_id").
		Where(`ss.esta_activa = true
			AND ss.lat IS NOT NULL AND ss.lon IS NOT NULL
			AND COALESCE(ss.direccion, '') <> 'Casa Matriz / Online'
			AND ss.nombre NOT ILIKE '%online%'
			AND public.ST_DWithin(`+puntoSucursal+`, `+puntoOrigen+`, ?)`,
			longitud, latitud, radioKm*1000).
		Select(`
			ss.id,
			COALESCE(cs.nombre, '') AS cadena,
			ss.nombre,
			ss.lat,
			ss.lon,
			public.ST_Distance(`+puntoSucursal+`, `+puntoOrigen+`) / 1000.0 AS distancia_km`,
			longitud, latitud).
		Order("distancia_km ASC, ss.id ASC").
		Limit(maximo)

	if errorConsulta := consulta.Scan(&registros).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	return registros, nil
}

// ConsultarSucursales lee el inventario completo para informar por qué una
// sucursal no es candidata. Son pocas filas y no participa en el cálculo de
// precios.
func (repositorio *RepositorioPreciosComparables) ConsultarSucursales(contexto context.Context) ([]domain.SucursalRegistro, error) {
	registros := []domain.SucursalRegistro{}

	consulta := repositorio.baseDatos.WithContext(contexto).
		Table("scraper.sucursales_supermercado ss").
		Joins("JOIN scraper.cadenas_supermercado cs ON cs.id = ss.cadena_id").
		Select(`
			ss.id,
			ss.nombre,
			COALESCE(cs.nombre, '') AS cadena,
			COALESCE(ss.direccion, '') AS direccion,
			ss.esta_activa AS activa,
			ss.lat,
			ss.lon`).
		Order("ss.id ASC")

	if errorConsulta := consulta.Scan(&registros).Error; errorConsulta != nil {
		return nil, errorConsulta
	}
	return registros, nil
}
