/**
 * Utilidades de formateo para la interfaz de usuario.
 * NO deben acoplarse a datos de prueba (mocks) ni a la lógica de negocio profunda.
 */

/**
 * Formatea un número como moneda chilena (CLP).
 * @param price Valor numérico del precio.
 * @returns Cadena formateada, ej. "$15.000".
 */
export function formatPrice(price: number | undefined | null): string {
  if (price === undefined || price === null) return '$0';
  return new Intl.NumberFormat('es-CL', {
    style: 'currency',
    currency: 'CLP',
    maximumFractionDigits: 0
  }).format(price);
}

/**
 * Calcula el porcentaje de descuento entre un precio normal y un precio oferta.
 * @param originalPrice Precio normal/original.
 * @param price Precio actual/oferta.
 * @returns Entero que representa el porcentaje de descuento (0 si no hay).
 */
export function calculateDiscountPct(originalPrice?: number, price?: number): number {
  if (!originalPrice || !price || originalPrice <= price) return 0;
  return Math.round(((originalPrice - price) / originalPrice) * 100);
}

/**
 * Calcula el ahorro neto en moneda.
 * @param originalPrice Precio normal/original.
 * @param price Precio actual/oferta.
 * @returns Entero del ahorro neto (0 si no hay).
 */
export function calculateSavings(originalPrice?: number, price?: number): number {
  if (!originalPrice || !price || originalPrice <= price) return 0;
  return originalPrice - price;
}
