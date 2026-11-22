// Consulta Catálogo y adapta sus productos sin añadir cadenas, precios o ubicaciones ficticias.
import type { UiProduct as Producto, UiSupermarket } from '@/features/catalogo/tipos';

const URL_API = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '')

type ProductoApi = {
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

export type ApiProductDetail = ProductoApi & {
  historial: ApiPriceHistory[]
}

type PaginaProductos = {
  data: ProductoApi[]
  total_registros: number
  pagina_actual?: number
  total_paginas?: number
}

/** Normaliza el nombre de una cadena para usarlo como identificador de interfaz. */
export function slugifyMarket(nombre: string | null | undefined) {
  return (nombre ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export const FALLBACK_MARKET_NAME = 'Sin supermercado';

/** Obtiene solo las cadenas presentes en la muestra recibida; no representa todo el catálogo. */
export function getMarketsFromProducts(productos: Producto[]): UiSupermarket[] {
  const supermercados = new Map<string, UiSupermarket>();
  for (const producto of productos) {
    const nombre = producto.supermarketName?.trim();
    if (!nombre || nombre === FALLBACK_MARKET_NAME || !producto.supermarketId) continue;
    supermercados.set(producto.supermarketId, { id: producto.supermarketId, name: nombre });
  }
  return Array.from(supermercados.values());
}

/** Conserva los datos recibidos y transforma los nombres del contrato para los componentes. */
export function toUiProduct(producto: ProductoApi): Producto {
  const nombreSupermercado = producto.supermercado || FALLBACK_MARKET_NAME;
  return {
    id: producto.id,
    name: producto.nombre,
    brand: producto.marca || 'Sin marca',
    category: producto.categoria || undefined,
    image: producto.url_imagen,
    unit: producto.unidad || 'Unidad',
    price: producto.precio,
    originalPrice: producto.precio_normal || producto.precio,
    supermarketId: slugifyMarket(nombreSupermercado),
    supermarketName: nombreSupermercado,
    inStock: producto.en_stock,
  }
}

/** Propaga los errores de las lecturas del catálogo. */
async function solicitar<Respuesta>(ruta: string): Promise<Respuesta> {
  const respuesta = await fetch(`${URL_API}${ruta}`)
  if (!respuesta.ok) throw new Error(`La API respondió con ${respuesta.status}`)
  return respuesta.json() as Promise<Respuesta>
}

/** Recupera una página de productos con los totales informados por el servidor. */
export async function getProducts(parametros = '') {
  const pagina = await solicitar<PaginaProductos>(`/productos${parametros}`)
  return {
    products: pagina.data.map(toUiProduct),
    total: pagina.total_registros,
    page: pagina.pagina_actual ?? 1,
    totalPages: pagina.total_paginas ?? 1,
  }
}

/** Recupera el producto seleccionado y su historial real de precios. */
export async function getProductDetail(id: string) {
  const producto = await solicitar<ApiProductDetail>(`/productos/${encodeURIComponent(id)}`)
  return { product: toUiProduct(producto), history: producto.historial }
}
