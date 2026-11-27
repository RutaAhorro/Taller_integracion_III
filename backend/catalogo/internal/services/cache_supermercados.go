// Caché en memoria de los supermercados disponibles del listado público. Su
// consulta recorre todo el catálogo filtrado para saber qué cadenas tienen al
// menos un producto, y el resultado casi no cambia entre páginas ni entre
// usuarios: reutilizarlo por un plazo corto evita repetir ese recorrido en
// cada petición. Cada réplica tiene su propia caché; no se usa Redis.

package services

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/catalogo/internal/domain"
)

// ===== Límites =====

// maximoEntradasCacheSupermercados acota la memoria: cada búsqueda distinta es
// una entrada. Al llenarse se descartan las vencidas y, si no alcanza, todas.
const maximoEntradasCacheSupermercados = 512

// ===== Tipos =====

// consultaSupermercados es la lectura real que la caché evita repetir.
type consultaSupermercados func(contexto context.Context, filtro domain.FiltroProductosDTO) ([]domain.SupermercadoDTO, error)

// entradaSupermercados guarda una respuesta y cuándo deja de ser válida.
type entradaSupermercados struct {
	supermercados []domain.SupermercadoDTO
	venceEl       time.Time
}

// cacheSupermercados asocia la clave de un filtro con sus cadenas disponibles.
// Es segura para peticiones concurrentes.
type cacheSupermercados struct {
	vigencia time.Duration
	reloj    func() time.Time
	mutex    sync.Mutex
	entradas map[string]entradaSupermercados
}

// nuevaCacheSupermercados crea la caché; una vigencia de 0 la desactiva.
func nuevaCacheSupermercados(vigencia time.Duration, reloj func() time.Time) *cacheSupermercados {
	return &cacheSupermercados{
		vigencia: vigencia,
		reloj:    reloj,
		entradas: make(map[string]entradaSupermercados),
	}
}

// ===== Lectura =====

// obtener devuelve las cadenas del filtro desde la caché o consultando. Los
// errores no se guardan: la siguiente petición vuelve a intentar. Siempre
// devuelve una copia y nunca nil, para que el JSON sea [] y ningún llamador
// modifique lo guardado.
func (cache *cacheSupermercados) obtener(contexto context.Context, filtro domain.FiltroProductosDTO, consultar consultaSupermercados) ([]domain.SupermercadoDTO, error) {
	if cache.vigencia <= 0 {
		return consultarSinNulos(contexto, filtro, consultar)
	}

	clave := claveSupermercados(filtro)
	ahora := cache.reloj()
	cache.mutex.Lock()
	entrada, existe := cache.entradas[clave]
	cache.mutex.Unlock()
	if existe && ahora.Before(entrada.venceEl) {
		return copiarSupermercados(entrada.supermercados), nil
	}

	// La consulta corre sin el candado: dos peticiones simultáneas pueden
	// consultar las dos, pero ninguna espera a la otra.
	supermercados, errorConsulta := consultarSinNulos(contexto, filtro, consultar)
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	cache.guardar(clave, supermercados, ahora)
	return copiarSupermercados(supermercados), nil
}

// guardar registra la respuesta y libera espacio si se alcanzó el máximo.
func (cache *cacheSupermercados) guardar(clave string, supermercados []domain.SupermercadoDTO, ahora time.Time) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if len(cache.entradas) >= maximoEntradasCacheSupermercados {
		for claveGuardada, entrada := range cache.entradas {
			if !ahora.Before(entrada.venceEl) {
				delete(cache.entradas, claveGuardada)
			}
		}
		if len(cache.entradas) >= maximoEntradasCacheSupermercados {
			cache.entradas = make(map[string]entradaSupermercados)
		}
	}
	cache.entradas[clave] = entradaSupermercados{
		supermercados: copiarSupermercados(supermercados),
		venceEl:       ahora.Add(cache.vigencia),
	}
}

// ===== Auxiliares =====

// claveSupermercados identifica el filtro sin los campos que no cambian las
// cadenas disponibles: página, tamaño y orden, y la cadena elegida, que la
// agregación omite para seguir ofreciendo las alternativas.
func claveSupermercados(filtro domain.FiltroProductosDTO) string {
	filtro.Page, filtro.Limit = 0, 0
	filtro.SortBy, filtro.Order = "", ""
	filtro.Supermercado = ""
	clave, errorSerializacion := json.Marshal(filtro)
	if errorSerializacion != nil {
		return "" // FiltroProductosDTO sólo tiene textos, números y booleanos
	}
	return string(clave)
}

// consultarSinNulos llama al repositorio y convierte nil en una lista vacía.
func consultarSinNulos(contexto context.Context, filtro domain.FiltroProductosDTO, consultar consultaSupermercados) ([]domain.SupermercadoDTO, error) {
	supermercados, errorConsulta := consultar(contexto, filtro)
	if errorConsulta != nil {
		return nil, errorConsulta
	}
	if supermercados == nil {
		return []domain.SupermercadoDTO{}, nil
	}
	return supermercados, nil
}

// copiarSupermercados devuelve una lista nueva con los mismos valores.
func copiarSupermercados(supermercados []domain.SupermercadoDTO) []domain.SupermercadoDTO {
	return append(make([]domain.SupermercadoDTO, 0, len(supermercados)), supermercados...)
}
