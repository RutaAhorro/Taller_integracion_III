package ingesta

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// ObtenerCadena busca la cadena de supermercado por ID o por nombre (ej. "Jumbo", "Santa Isabel").
// Si se busca por nombre y no existe, la crea automáticamente para soportar nuevas fuentes de scraping.
func (r *Repository) obtenerCadena(ctx context.Context, cadenaID int, nombreCadena string) (*CadenaSupermercado, error) {
	var cadena CadenaSupermercado
	query := r.db.WithContext(ctx)
	nombreLimpio := strings.TrimSpace(nombreCadena)

	if cadenaID > 0 {
		query = query.Where("id = ?", cadenaID)
		if err := query.First(&cadena).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return &cadena, nil
	}

	if nombreLimpio == "" {
		return nil, errors.New("debe proporcionar cadena_id o nombre de supermercado")
	}

	err := query.Where("nombre ILIKE ?", nombreLimpio).First(&cadena).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nueva := CadenaSupermercado{
				Nombre:     nombreLimpio,
				EstaActiva: true,
				CreadoEl:   time.Now(),
			}
			if createErr := r.db.WithContext(ctx).Create(&nueva).Error; createErr != nil {
				return nil, createErr
			}
			return &nueva, nil
		}
		return nil, err
	}
	return &cadena, nil
}

// ObtenerSucursal busca una sucursal específica por ID, código o retorna la primera sucursal activa de la cadena.
// Si la cadena no posee sucursales registradas, crea una sucursal por defecto para asegurar la integridad referencial.
func (r *Repository) obtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigoSucursal *string) (*SucursalSupermercado, error) {
	var sucursal SucursalSupermercado
	query := r.db.WithContext(ctx)

	if sucursalID != nil && *sucursalID > 0 {
		if cadenaID > 0 {
			query = query.Where("cadena_id = ?", cadenaID)
		}
		query = query.Where("id = ?", *sucursalID)
		if err := query.First(&sucursal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return &sucursal, nil
	}

	if codigoSucursal != nil && strings.TrimSpace(*codigoSucursal) != "" {
		if cadenaID > 0 {
			query = query.Where("cadena_id = ?", cadenaID)
		}
		query = query.Where("codigo_sucursal = ?", strings.TrimSpace(*codigoSucursal))
		if err := query.First(&sucursal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return &sucursal, nil
	}

	if cadenaID > 0 {
		err := query.Where("cadena_id = ? AND esta_activa = true", cadenaID).Order("id ASC").First(&sucursal).Error
		if err == nil {
			return &sucursal, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		// Auto-crear sucursal por defecto para la cadena
		var cadena CadenaSupermercado
		nombreCadena := "Supermercado"
		if errCad := r.db.WithContext(ctx).Where("id = ?", cadenaID).First(&cadena).Error; errCad == nil {
			nombreCadena = cadena.Nombre
		}

		cod := fmt.Sprintf("%s-CENTRAL", strings.ToUpper(strings.ReplaceAll(nombreCadena, " ", "-")))
		comuna := "Temuco"
		ciudad := "Temuco"
		nuevaSucursal := SucursalSupermercado{
			CadenaID:       cadenaID,
			CodigoSucursal: &cod,
			Nombre:         fmt.Sprintf("%s Central", nombreCadena),
			Direccion:      "Casa Matriz / Online",
			Comuna:         &comuna,
			Ciudad:         &ciudad,
			Lat:            -38.7359,
			Lon:            -72.5904,
			EstaActiva:     true,
		}
		if createErr := r.db.WithContext(ctx).Create(&nuevaSucursal).Error; createErr != nil {
			return nil, createErr
		}
		return &nuevaSucursal, nil
	}

	return nil, errors.New("no se especificaron criterios válidos para identificar la sucursal")
}

// Los locks transaccionales serializan sólo la resolución con autocreación,
// incluso entre réplicas de Catálogo; las búsquedas por ID siguen siendo lecturas.
func (r *Repository) ObtenerCadena(ctx context.Context, cadenaID int, nombre string) (*CadenaSupermercado, error) {
	if cadenaID > 0 {
		return r.obtenerCadena(ctx, cadenaID, nombre)
	}
	var result *CadenaSupermercado
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(2691, hashtext(?))", strings.ToLower(strings.TrimSpace(nombre))).Error; err != nil {
			return err
		}
		var err error
		result, err = (&Repository{db: tx}).obtenerCadena(ctx, cadenaID, nombre)
		return err
	})
	return result, err
}

func (r *Repository) ObtenerSucursal(ctx context.Context, cadenaID int, sucursalID *int, codigo *string) (*SucursalSupermercado, error) {
	if cadenaID <= 0 || (sucursalID != nil && *sucursalID > 0) || (codigo != nil && strings.TrimSpace(*codigo) != "") {
		return r.obtenerSucursal(ctx, cadenaID, sucursalID, codigo)
	}
	var result *SucursalSupermercado
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(2692, ?)", cadenaID).Error; err != nil {
			return err
		}
		var err error
		result, err = (&Repository{db: tx}).obtenerSucursal(ctx, cadenaID, sucursalID, codigo)
		return err
	})
	return result, err
}
