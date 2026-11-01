import { useState, useEffect, useMemo, useRef } from 'react';
import { useLayoutContext } from '@/layouts/MainLayout';
import { getProducts } from '@/lib/products-api';
import { calculateDiscountPct } from '@/lib/formatters';
import type { UiProduct as Product } from '@/types';

const PAGE_SIZE = 24;

// Marcas propias de las cadenas chilenas (para el filtro "Marcas blancas").
const STORE_BRAND_TERMS = [
  'lider',
  'a cuenta',
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

const isOfferFilter = (category: string | null) => category === 'ofertas' || category === 'descuentos';

const isStoreBrand = (product: Product) =>
  STORE_BRAND_TERMS.some((term) => product.brand.toLowerCase().includes(term));

const savings = (p: Product) => p.originalPrice - p.price;

export function useCatalog(query: string, category: string | null, activeMarket: string | null) {
  const { markets } = useLayoutContext();
  const [products, setProducts] = useState<Product[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [isLoading, setIsLoading] = useState(true);
  const [isFetchingMore, setIsFetchingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Descarta respuestas de peticiones que quedaron obsoletas al cambiar filtros.
  const requestId = useRef(0);

  const marketName = markets.find((m) => m.id === activeMarket)?.name ?? null;

  // Los filtros reales los resuelve la API. "Marcas blancas" no tiene
  // equivalente en el backend y se aplica abajo, en memoria.
  const loadPage = (pageNum: number) => {
    const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE), en_stock: 'true' });
    const search = query.trim();
    if (search.length >= 3) params.set('q', search);
    if (isOfferFilter(category)) params.set('en_oferta', 'true');
    else if (category && category !== 'marcas') params.set('categoria', category);
    if (marketName) params.set('supermercado', marketName);
    return getProducts(`?${params}`);
  };

  // Primera página cada vez que cambian los filtros.
  useEffect(() => {
    const current = ++requestId.current;
    setIsLoading(true);
    setError(null);

    loadPage(1)
      .then((result) => {
        if (requestId.current !== current) return;
        setProducts(result.products);
        setPage(result.page);
        setTotalPages(result.totalPages);
      })
      .catch(() => {
        if (requestId.current !== current) return;
        setProducts([]);
        setError('No se pudo cargar el catálogo desde la base de datos.');
      })
      .finally(() => {
        if (requestId.current === current) setIsLoading(false);
      });

    return () => {
      requestId.current++;
    };
  }, [query, category, marketName]);

  // Scroll infinito: agrega la página siguiente sin duplicar productos.
  const fetchMore = () => {
    if (isFetchingMore || isLoading || page >= totalPages) return;
    const current = requestId.current;
    setIsFetchingMore(true);

    loadPage(page + 1)
      .then((result) => {
        if (requestId.current !== current) return;
        setProducts((prev) => {
          const seen = new Set(prev.map((p) => p.id));
          return [...prev, ...result.products.filter((p) => !seen.has(p.id))];
        });
        setPage(result.page);
        setTotalPages(result.totalPages);
      })
      .catch(() => {
        if (requestId.current === current) setError('No se pudieron cargar más productos.');
      })
      .finally(() => setIsFetchingMore(false));
  };

  const filtered = useMemo(() => {
    // Búsquedas de 1-2 letras no viajan a la API; se filtran aquí.
    const q = query.trim().toLowerCase();
    const result = q
      ? products.filter((p) => p.name.toLowerCase().includes(q) || p.brand.toLowerCase().includes(q))
      : products;

    if (category === 'marcas') return result.filter(isStoreBrand);
    // "Mejores ofertas" ordena por ahorro en $; "Mayor descuento", por %.
    if (category === 'ofertas') return [...result].sort((a, b) => savings(b) - savings(a));
    if (category === 'descuentos') {
      return [...result].sort(
        (a, b) => calculateDiscountPct(b.originalPrice, b.price) - calculateDiscountPct(a.originalPrice, a.price),
      );
    }
    return result;
  }, [products, query, category]);

  const supermarketCount = useMemo(() => new Set(filtered.map((p) => p.supermarketId)).size, [filtered]);

  // "Marcas blancas" filtra en memoria: si el lote trae pocas coincidencias,
  // se piden más páginas para que la grilla no quede vacía sin scroll.
  useEffect(() => {
    if (category === 'marcas' && filtered.length < PAGE_SIZE / 2) fetchMore();
  }, [category, filtered.length, page, totalPages, isLoading, isFetchingMore]);

  return { isLoading, error, filtered, supermarketCount, isFetchingMore, fetchMore };
}
