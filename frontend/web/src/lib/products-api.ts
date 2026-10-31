import { supermarkets as fallbackMarkets } from './constants';
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

export function slugifyMarket(name: string | null | undefined) {
  return (name ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function supermarketId(name: string) {
  return slugifyMarket(name);
}

// Metadatos visuales (logo/color/coords) para una cadena scrapeada.
// Usa la lista fija solo como respaldo: si la cadena no está ahí,
// devuelve valores neutros en vez de romper el filtro.
export function marketMeta(name: string): Pick<UiSupermarket, 'logo' | 'color' | 'coords'> {
  const found = fallbackMarkets.find(
    (market) => slugifyMarket(market.name) === slugifyMarket(name),
  );
  return {
    logo: found?.logo ?? '/placeholder.svg',
    color: found?.color ?? 'oklch(0.5 0 0)',
    coords: found?.coords ?? [0, 0],
  };
}

export const FALLBACK_MARKET_NAME = 'Sin supermercado';

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
    supermarketId: supermarketId(marketName),
    supermarketName: marketName,
    inStock: product.en_stock,
  }
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`)
  if (!response.ok) throw new Error(`La API respondió con ${response.status}`)
  return response.json() as Promise<T>
}

export async function getProducts(params = '') {
  const page = await request<ProductPage>(`/productos${params}`)
  return {
    products: page.data.map(toUiProduct),
    total: page.total_registros,
    page: page.pagina_actual ?? 1,
    totalPages: page.total_paginas ?? 1,
  }
}

export async function getProductDetail(id: string) {
  const product = await request<ApiProductDetail>(`/productos/${encodeURIComponent(id)}`)
  return { product: toUiProduct(product), history: product.historial }
}