// Centraliza las rutas usadas por navegación, enlaces y controles de acceso.
import { normalizeRole, ROLE_ADMIN } from '@/features/autenticacion/roles';
export const ROUTES = {
  HOME: '/',
  CATALOGO: '/dashboard',
  ADMIN: '/admin',
  ADMIN_METRICAS: '/admin/metricas',
  PRODUCTO_DETALLE: (id: string) => `/producto/${id}`,
  LOGIN: '/login',
  ONBOARDING: '/onboarding',
  PERFIL: '/profile',
  LISTAS: '/listas',
  CHAT: '/chat',
  COLABORADOR: '/colaborador',
  RUTA: '/route',
  HISTORIAL: '/history',
  PLANES: '/planes',
  TERMINOS: '/terminos',
} as const;

// Usar únicamente con el rol recibido tras una autenticación exitosa.
export function rutaTrasAutenticacion(rol?: string | null) {
  return normalizeRole(rol) === ROLE_ADMIN ? ROUTES.ADMIN : ROUTES.CATALOGO;
}
