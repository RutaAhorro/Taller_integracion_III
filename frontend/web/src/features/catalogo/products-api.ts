// Consulta Catálogo y adapta sus productos sin añadir cadenas, precios o ubicaciones ficticias.
import type { UiProduct as Producto, UiSupermarket } from '@/features/catalogo/tipos';

const URL_API = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');

type ProductoApi = {
  id: string;
  nombre: string;
  marca: string;
  categoria: string;
  supermercado: string;
  precio: number;
  precio_normal: number;
  precio_oferta?: number;
  en_oferta: boolean;
  url_imagen: string;
  unidad: string;
  en_stock: boolean;
};

export type ApiPriceHistory = {
  precio_normal: number;
  precio_oferta?: number;
  capturado_el: string;
};

export type ApiProductDetail = ProductoApi & {
  historial: ApiPriceHistory[];
};

type PaginaProductos = {
  data: ProductoApi[];
  supermercados: unknown;
  total_registros: number;
  pagina_actual?: number;
  total_paginas?: number;
};

/** Forma de cada cadena en las facetas completas de Catálogo (SUP-371). */
type SupermercadoApi = {
  id: number;
  nombre: string;
};

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

/** Comprueba el contrato de facetas antes de convertirlo al modelo de interfaz. */
function esSupermercadoApi(valor: unknown): valor is SupermercadoApi {
  return (
    typeof valor === 'object' &&
    valor !== null &&
    'id' in valor &&
    typeof valor.id === 'number' &&
    Number.isSafeInteger(valor.id) &&
    valor.id > 0 &&
    'nombre' in valor &&
    typeof valor.nombre === 'string' &&
    valor.nombre.trim().length > 0
  );
}

/** Conserva las alternativas del servidor, incluso si no aparecen en la página de productos. */
function adaptarSupermercados(valor: unknown): UiSupermarket[] {
  if (!Array.isArray(valor) || !valor.every(esSupermercadoApi)) {
    throw new Error('Respuesta de supermercados inválida');
  }
  return valor.map((supermercado) => ({
    id: slugifyMarket(supermercado.nombre),
    name: supermercado.nombre.trim(),
  }));
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
  };
}

/** Propaga los errores de las lecturas del catálogo. */
async function solicitar<Respuesta>(ruta: string, signal?: AbortSignal): Promise<Respuesta> {
  const respuesta = await fetch(`${URL_API}${ruta}`, { signal });
  if (!respuesta.ok) throw new Error(`La API respondió con ${respuesta.status}`);
  return respuesta.json() as Promise<Respuesta>;
}

// Separa las categorías de BD de los filtros destacados, aunque compartan nombre.
export const PREFIJO_CATEGORIA = 'categoria:';

/** Traduce la selección del lateral sin modificar el contrato de categoría anterior. */
export function parametrosCategoria(categoria: string | null): URLSearchParams {
  const parametros = new URLSearchParams();
  if (categoria?.startsWith(PREFIJO_CATEGORIA)) {
    parametros.set('categoria_exacta', categoria.slice(PREFIJO_CATEGORIA.length));
  } else if (categoria === 'ofertas' || categoria === 'descuentos') {
    parametros.set('en_oferta', 'true');
  } else if (categoria === 'marcas') {
    parametros.set('solo_marcas_blancas', 'true');
  } else if (categoria) {
    parametros.set('categoria', categoria);
  }
  return parametros;
}

/** Consulta categorías completas, no una muestra de productos; rechaza contratos inválidos. */
export async function obtenerCategorias(signal?: AbortSignal): Promise<string[]> {
  const datos = await solicitar<{ categorias: unknown }>('/productos/categorias', signal);
  if (
    !datos ||
    !Array.isArray(datos.categorias) ||
    !datos.categorias.every((categoria: unknown) => typeof categoria === 'string' && categoria.trim())
  ) {
    throw new Error('Respuesta de categorías inválida');
  }
  return datos.categorias;
}

/** Recupera una página de productos con los totales informados por el servidor. */
export async function getProducts(parametros = '', signal?: AbortSignal) {
  const pagina = await solicitar<PaginaProductos>(`/productos${parametros}`, signal);
  return {
    products: pagina.data.map(toUiProduct),
    markets: adaptarSupermercados(pagina.supermercados),
    total: pagina.total_registros,
    page: pagina.pagina_actual ?? 1,
    totalPages: pagina.total_paginas ?? 1,
  };
}

/** Recupera el producto seleccionado y su historial real de precios. */
export async function getProductDetail(id: string) {
  const producto = await solicitar<ApiProductDetail>(`/productos/${encodeURIComponent(id)}`);
  return { product: toUiProduct(producto), history: producto.historial };
}
