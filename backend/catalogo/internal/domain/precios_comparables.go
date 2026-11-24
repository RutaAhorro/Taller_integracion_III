// Precios comparables por sucursal: DTO e interfaces de lectura que usa el
// motor de rutas para elegir paradas dentro de un radio.

package domain

import "context"

// ===== Motivos de exclusión =====

// Motivos por los que una sucursal no entra en la lista de candidatas. Se
// devuelven explícitamente para que Rutas sepa que la ausencia es real y no un
// silencio de Catálogo.
const (
	MotivoSucursalSinCoordenadas = "sin_coordenadas"
	MotivoSucursalInactiva       = "sucursal_inactiva"
	MotivoSucursalOnline         = "sucursal_online"
)

// ===== Contrato de la consulta =====

// ConsultaPreciosComparablesDTO es el cuerpo aceptado por la ruta interna.
// Los valores numéricos son punteros para distinguir "no enviado" de cero, que
// es una coordenada válida.
type ConsultaPreciosComparablesDTO struct {
	ProductoIDs      []string `json:"producto_ids"`
	Latitud          *float64 `json:"lat"`
	Longitud         *float64 `json:"lon"`
	RadioKm          *float64 `json:"radio_km"`
	MaximoSucursales *int     `json:"maximo_sucursales"`
}

// RespuestaPreciosComparablesDTO agrupa candidatas, ofertas por producto y las
// ausencias que Rutas debe conocer antes de calcular.
type RespuestaPreciosComparablesDTO struct {
	Sucursales          []SucursalCandidataDTO         `json:"sucursales"`
	Productos           []ProductoPreciosComparableDTO `json:"productos"`
	SinOfertas          []string                       `json:"sin_ofertas"`
	SucursalesExcluidas []SucursalExcluidaDTO          `json:"sucursales_excluidas"`
	Resumen             ResumenPreciosComparablesDTO   `json:"resumen"`
}

// SucursalCandidataDTO es una parada posible: sucursal física y activa con
// coordenadas reales, dentro del radio y ordenada por distancia al origen.
type SucursalCandidataDTO struct {
	SucursalID  int     `json:"sucursal_id"`
	Nombre      string  `json:"nombre"`
	Cadena      string  `json:"cadena"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	DistanciaKm float64 `json:"distancia_km"`
}

// ProductoPreciosComparableDTO conserva el estado de equivalencia de SUP-244 y
// decide qué ofertas sobreviven al filtro geográfico. No inventa equivalencias:
// un producto sin EAN conserva sólo su identidad exacta.
type ProductoPreciosComparableDTO struct {
	ProductoID   string                 `json:"producto_id"`
	Estado       string                 `json:"estado"`
	EAN          *string                `json:"ean"`
	Presentacion string                 `json:"presentacion"`
	Ofertas      []OfertaEquivalenteDTO `json:"ofertas"`
}

// SucursalExcluidaDTO informa de una sucursal que no puede ser parada y por qué.
type SucursalExcluidaDTO struct {
	SucursalID int    `json:"sucursal_id"`
	Nombre     string `json:"nombre"`
	Cadena     string `json:"cadena"`
	Motivo     string `json:"motivo"`
}

// ResumenPreciosComparablesDTO cuantifica la cobertura real de la consulta.
type ResumenPreciosComparablesDTO struct {
	Solicitados            int `json:"solicitados"`
	ConOferta              int `json:"con_oferta"`
	SinOferta              int `json:"sin_oferta"`
	Comparables            int `json:"comparables"`
	SinEAN                 int `json:"sin_ean"`
	EANInvalidos           int `json:"ean_invalidos"`
	ConflictosPresentacion int `json:"conflictos_presentacion"`
	NoEncontrados          int `json:"no_encontrados"`
	OfertasSinPrecio       int `json:"ofertas_sin_precio"`
	SucursalesCandidatas   int `json:"sucursales_candidatas"`
	SucursalesExcluidas    int `json:"sucursales_excluidas"`
}

// ===== Persistencia =====

// SucursalCercanaRegistro es una fila del filtro PostGIS por radio.
type SucursalCercanaRegistro struct {
	ID          int
	Nombre      string
	Cadena      string
	Lat         float64
	Lon         float64
	DistanciaKm float64
}

// SucursalRegistro describe una sucursal para clasificar su exclusión. Lat y
// Lon son punteros porque NULL significa que el dato no existe.
type SucursalRegistro struct {
	ID        int
	Nombre    string
	Cadena    string
	Direccion string
	Activa    bool
	Lat       *float64
	Lon       *float64
}

// RepositorioPreciosComparables define las dos lecturas que necesita el
// servicio: candidatas acotadas por índice geoespacial y el inventario de
// sucursales para Informar exclusiones.
type RepositorioPreciosComparables interface {
	ConsultarCercanas(contexto context.Context, latitud, longitud, radioKm float64, maximo int) ([]SucursalCercanaRegistro, error)
	ConsultarSucursales(contexto context.Context) ([]SucursalRegistro, error)
}
