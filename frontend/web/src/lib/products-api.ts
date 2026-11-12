// Consulta Catálogo y adapta sus productos sin añadir cadenas, precios o ubicaciones ficticias.
import type { UiProduct as Product, UiSupermarket } from '@/types';

const API_URL = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '')

type ApiProduct = {
  id: string
  nombre: string
  marca: string
  categoria: string
  supermercado: string
  precio: number
  precio_normal: number
  precio_oferta?: number
  en_oferta: boolean
  url_imagen: string
  unidad: string
  en_stock: boolean
}

export type ApiPriceHistory = {
  precio_normal: number
  precio_oferta?: number
  capturado_el: string
}

export type ApiProductDetail = ApiProduct & {
  historial: ApiPriceHistory[]
}

type ProductPage = {
  data: ApiProduct[]
  total_registros: number
  pagina_actual?: number
  total_paginas?: number
}

/** Normaliza el nombre de una cadena para usarlo como identificador de interfaz. */
export function slugifyMarket(name: string | null | undefined) {
  return (name ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export const FALLBACK_MARKET_NAME = 'Sin supermercado';

/** Obtiene solo las cadenas presentes en la muestra recibida; no representa todo el catálogo. */
export function getMarketsFromProducts(products: Product[]): UiSupermarket[] {
  const markets = new Map<string, UiSupermarket>();
  for (const product of products) {
    const name = product.supermarketName?.trim();
    if (!name || name === FALLBACK_MARKET_NAME || !product.supermarketId) continue;
    markets.set(product.supermarketId, { id: product.supermarketId, name });
  }
  return Array.from(markets.values());
}

/** Conserva los datos recibidos y transforma los nombres del contrato para los componentes. */
export function toUiProduct(product: ApiProduct): Product {
  const marketName = product.supermercado || FALLBACK_MARKET_NAME;
  return {
    id: product.id,
    name: product.nombre,
    brand: product.marca || 'Sin marca',
    category: product.categoria || undefined,
    image: product.url_imagen,
    unit: product.unidad || 'Unidad',
    price: product.precio,
    originalPrice: product.precio_normal || product.precio,
    supermarketId: slugifyMarket(marketName),
    supermarketName: marketName,
    inStock: product.en_stock,
  }
}

/** Propaga los errores de las lecturas del catálogo. */
async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`)
  if (!response.ok) throw new Error(`La API respondió con ${response.status}`)
  return response.json() as Promise<T>
}

/** Recupera una página de productos con los totales informados por el servidor. */
export async function getProducts(params = '') {
  const page = await request<ProductPage>(`/productos${params}`)
  return {
    products: page.data.map(toUiProduct),
    total: page.total_registros,
    page: page.pagina_actual ?? 1,
    totalPages: page.total_paginas ?? 1,
  }
}

/** Recupera el producto seleccionado y su historial real de precios. */
export async function getProductDetail(id: string) {
  const product = await request<ApiProductDetail>(`/productos/${encodeURIComponent(id)}`)
  return { product: toUiProduct(product), history: product.historial }
}
