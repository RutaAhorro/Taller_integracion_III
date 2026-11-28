/**
 * Utilidades de formateo para la interfaz de usuario.
 * NO deben acoplarse a datos de prueba (mocks) ni a la lógica de negocio profunda.
 */

/**
 * Formatea un número como moneda chilena (CLP).
 * @param price Valor numérico del precio.
 * @returns Cadena formateada, ej. "$15.000".
 */
export function formatPrice(precio: number | undefined | null): string {
  if (precio === undefined || precio === null) return '$0';
  return new Intl.NumberFormat('es-CL', {
    style: 'currency',
    currency: 'CLP',
    maximumFractionDigits: 0
  }).format(precio);
}

// Valida que el precio sea un número finito positivo; descarta 0, negativos y valores ausentes.
export function tienePrecioVigente(precio: number | null | undefined): precio is number {
  return typeof precio === 'number' && Number.isFinite(precio) && precio > 0;
}

/**
 * Calcula el porcentaje de descuento entre un precio normal y un precio oferta.
 * @param precioOriginal Precio normal/original.
 * @param precio Precio actual/oferta.
 * @returns Entero que representa el porcentaje de descuento (0 si no hay).
 */
export function calculateDiscountPct(precioOriginal?: number, precio?: number): number {
  if (!precioOriginal || !precio || precioOriginal <= precio) return 0;
  return Math.round(((precioOriginal - precio) / precioOriginal) * 100);
}

/**
 * Calcula el ahorro neto en moneda.
 * @param precioOriginal Precio normal/original.
 * @param precio Precio actual/oferta.
 * @returns Entero del ahorro neto (0 si no hay).
 */
export function calculateSavings(precioOriginal?: number, precio?: number): number {
  if (!precioOriginal || !precio || precioOriginal <= precio) return 0;
  return precioOriginal - precio;
}

// Convierte identificadores de categoría del catálogo en etiquetas legibles.
export function formatearCategoria(categoria: string | undefined | null): string {
  if (categoria === undefined || categoria === null) return '';
  const normalizada = categoria.replace(/[-_]+/g, ' ').replace(/\s+/g, ' ').trim();
  if (!normalizada) return '';

  const clave = normalizada.toLowerCase();
  const excepciones: Record<string, string> = {
    'dulces mermeladas y manjar': 'Dulces, mermeladas y manjar',
    'frescos y lacteos': 'Frescos y lácteos',
  };
  if (excepciones[clave]) return excepciones[clave];

  const palabrasConTilde: Record<string, string> = {
    lacteos: 'lácteos',
    panaderia: 'panadería',
    pasteleria: 'pastelería',
    carniceria: 'carnicería',
    pescaderia: 'pescadería',
    charcuteria: 'charcutería',
    fiambreria: 'fiambrería',
    perfumeria: 'perfumería',
    ferreteria: 'ferretería',
    tecnologia: 'tecnología',
    jugueteria: 'juguetería',
    libreria: 'librería',
    bano: 'baño',
    bebes: 'bebés',
    cafe: 'café',
    te: 'té',
    azucar: 'azúcar',
    limon: 'limón',
    jamon: 'jamón',
    atun: 'atún',
    maiz: 'maíz',
    mani: 'maní',
    organico: 'orgánico',
    organicos: 'orgánicos',
  };
  const etiqueta = normalizada
    .toLowerCase()
    .split(' ')
    .map((palabra) => palabrasConTilde[palabra] || palabra)
    .join(' ');
  return etiqueta.charAt(0).toUpperCase() + etiqueta.slice(1);
}

// Oculta valores de unidad que representan metadatos inválidos.
export function formatearUnidad(unidad: string | undefined | null): string | null {
  if (unidad === undefined || unidad === null) return null;
  const unidadVisible = unidad.trim();
  if (!unidadVisible) return null;
  if (['format', 'formato', 'n/a', 'na', 'null', 'undefined', '-'].includes(unidadVisible.toLowerCase())) return null;
  return unidadVisible;
}
