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

export const MOCK_ROUTES: Record<string, 'Sprint 2' | 'Sprint 3'> = {
  [ROUTES.CHAT]: 'Sprint 2',
  [ROUTES.COLABORADOR]: 'Sprint 2',
  [ROUTES.RUTA]: 'Sprint 3',
  [ROUTES.HISTORIAL]: 'Sprint 3',
};
