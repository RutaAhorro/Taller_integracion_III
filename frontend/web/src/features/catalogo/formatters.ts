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
