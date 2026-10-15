import { useState, useEffect, useMemo, useRef } from 'react';
import { useLayoutContext } from '@/layouts/MainLayout';
import { getProducts } from '@/lib/products-api';
import { calculateDiscountPct } from '@/lib/formatters';
import type { UiProduct as Product } from '@/types';

const PAGE_SIZE = 24;

// Filtros virtuales de "Destacados": no existen como categoría en la API,
// se resuelven en el frontend con datos reales del catálogo.
const FEATURED_IDS = new Set(['ofertas', 'descuentos', 'marcas']);

// Marcas propias de las cadenas chilenas (para el filtro "Marcas blancas").
const STORE_BRAND_TERMS = [
  'lider',
  'acuenta',
  'ekono',
  'jumbo',
  'santa isabel',
  'tottus',
  'unimarc',
  'cugat',
  'great value',
  'mi marca',
];

function normalizeCategoryTerm(term: string) {
  const normalized = term.trim().toLowerCase()
  if (normalized.length > 4 && normalized.endsWith('es')) return normalized.slice(0, -2)
  if (normalized.length > 3 && normalized.endsWith('s')) return normalized.slice(0, -1)
  return normalized
}

function getCategoryTerms(category: string) {
  return category
    .split(/[\s,/;]+/)
    .filter((term) => term !== 'y' && term !== 'e')
    .map(normalizeCategoryTerm)
    .filter(Boolean)
}

function productMatchesCategoryTerm(product: Product, term: string) {
  return product.name.toLowerCase().includes(term)
}

function isStoreBrand(product: Product) {
  const brand = product.brand.toLowerCase();
  return STORE_BRAND_TERMS.some((term) => brand.includes(term));
}

export function useCatalog(query: string, category: string | null, activeMarket: string | null) {
  const { markets } = useLayoutContext();
  const [isLoading, setIsLoading] = useState(true)
  const [products, setProducts] = useState<Product[]>([])
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)

  const [isFetchingMore, setIsFetchingMore] = useState(false)
  const requestId = useRef(0)

  // Nombre real de la cadena (desde el catálogo scrapeado) para el
  // parámetro `supermercado` de la API. Sin lista hardcodeada.
  const marketName = activeMarket
    ? markets.find((item) => item.id === activeMarket)?.name ?? null
    : null;

  const buildParams = (pageNum: number) => {
    const search = query.trim()
    const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE), en_stock: 'true' })
    if (search.length >= 3) params.set('q', search)
    // Ofertas se delegan al backend (filtro nativo indexado `en_oferta`).
    // El orden (por $ o por %) se aplica localmente sobre cada lote.
    // "Marcas blancas" sigue siendo virtual: no tiene equivalente en la API.
    if (category === 'ofertas' || category === 'descuentos') {
      params.set('en_oferta', 'true')
    } else if (category && !FEATURED_IDS.has(category)) {
      params.set('categoria', category)
    }
    if (marketName) params.set('supermercado', marketName)
    return params
  };

  // Carga una página del catálogo. Con replace=true reinicia el acumulado
  // (cambio de filtros); si no, agrega (scroll infinito).
  useEffect(() => {
    const current = ++requestId.current
    const replace = true
    setIsLoading(true)
    setIsFetchingMore(false)
    setError(null)

    getProducts(`?${buildParams(1).toString()}`)
      .then((result) => {
        if (requestId.current !== current) return
        setProducts(result.products)
        setPage(result.page)
        setTotalPages(result.totalPages)
      })
      .catch(() => {
        if (requestId.current !== current) return
        if (replace) setProducts([])
        setError('No se pudo cargar el catálogo desde la base de datos.')
      })
      .finally(() => {
        if (requestId.current !== current) return
        setIsLoading(false)
      })
  }, [query, category, marketName])

  // Filter local logic (red de seguridad sobre lo que devuelve la API,
  // más los filtros virtuales de Destacados).
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const baseProducts = products.filter((p) => {
      const isAvailable = p.inStock === true
      const matchesMarket = activeMarket === null || p.supermarketId === activeMarket
      const matchesQuery = !q || p.name.toLowerCase().includes(q) || p.brand.toLowerCase().includes(q)
      return isAvailable && matchesMarket && matchesQuery
    })

    if (!category) return baseProducts

    // Destacados con datos reales del catálogo.
    if (category === 'ofertas' || category === 'descuentos') {
      const onSale = baseProducts.filter(
        (product) => calculateDiscountPct(product.originalPrice, product.price) > 0,
      );
      // "Mejores ofertas": ordenadas por ahorro absoluto ($).
      // "Mayor descuento": ordenadas por porcentaje de descuento.
      return onSale.sort((a, b) =>
        category === 'descuentos'
          ? calculateDiscountPct(b.originalPrice, b.price) -
            calculateDiscountPct(a.originalPrice, a.price)
          : b.originalPrice - b.price - (a.originalPrice - a.price),
      );
    }
    if (category === 'marcas') {
      return baseProducts.filter(isStoreBrand);
    }

    const categoryTerms = getCategoryTerms(category)
    return baseProducts.filter((product) =>
      categoryTerms.some((term) => productMatchesCategoryTerm(product, term)),
    )
  }, [products, query, category, activeMarket])

  const supermarketCount = useMemo(() => {
    const ids = new Set(filtered.map((p) => p.supermarketId))
    return ids.size
  }, [filtered])

  const fetchMore = () => {
    if (isFetchingMore || isLoading || page >= totalPages) return;
    const current = ++requestId.current
    const nextPage = page + 1
    setIsFetchingMore(true);

    getProducts(`?${buildParams(nextPage).toString()}`)
      .then((result) => {
        if (requestId.current !== current) return
        setProducts((prev) => {
          const seen = new Set(prev.map((p) => p.id))
          return [...prev, ...result.products.filter((p) => !seen.has(p.id))]
        })
        setPage(result.page)
        setTotalPages(result.totalPages)
      })
      .catch(() => {
        if (requestId.current !== current) return
        setError('No se pudieron cargar más productos.')
      })
      .finally(() => {
        // Liberación incondicional: si esta petición quedó obsoleta por un
        // cambio de filtros, el flag igual debe soltarse para no trabar
        // el scroll infinito de la nueva búsqueda.
        setIsFetchingMore(false);
      })
  }

  return {
    isLoading,
    error,
    filtered,
    displayedProducts: filtered,
    supermarketCount,
    isFetchingMore,
    fetchMore
  }
}
