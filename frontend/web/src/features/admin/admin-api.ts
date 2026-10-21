// Capa de acceso a endpoints de administración (SUP-242 / SUP-243 / SUP-279).
// Usa VITE_API_URL como base (ya incluye /api/v1); sin esa variable los hooks
// caen a mocks. SUP-242 (usuarios) ya tiene backend en Identidad; SUP-243
// (fuentes) y SUP-279 (métricas) dependen de sus propias tareas.
//
// Convención de ramas: backend/feat/[tarea]-[nombre], frontend/feat/[tarea]-[nombre].

import type { AdminPlan, AdminRole, AdminSource, AdminUser, AdminUserRole } from './admin-mock-data';

const API_BASE = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');
export const ADMIN_HAS_BACKEND = API_BASE.length > 0;

// Rutas relativas a VITE_API_URL, que ya incluye el prefijo `/api/v1` igual
// que AuthContext y products-api. Concatenar otro `/api/v1` producía
// `/api/v1/api/v1/...` y el Gateway devolvía 404.
// SUP-242: `/admin/usuarios*` ya está enrutado hacia Identidad. SUP-243
// (`/admin/productos*`) y Control (`/scraper*`) conservan sus rutas.
export const ADMIN_ENDPOINTS = {
  // SUP-242 — GET/PUT /api/v1/admin/usuarios (RequireRole admin, tras el Gateway).
  // GET paginado: ?page=&limit=&q=&rol=&esta_activo=.
  // Body PUT: { esta_activo?: boolean; rol?: 'registrado' | 'colaborador' | 'admin';
  //             plan?: 'basico' | 'premium' }.
  // El plan comercial viaja en su propio campo, nunca dentro de `rol` (H-06).
  usuarios: '/admin/usuarios',
  // SUP-243 — GET/PUT de fuentes bajo /api/v1/admin/productos ya enrutado.
  // Ej.: /api/v1/admin/productos/fuentes — persiste scraper.cadenas_supermercado.
  fuentes: '/admin/productos/fuentes',
  // SUP-279 — un endpoint de métricas por servicio, sin agregación en Gateway.
  // Nombres exactos a confirmar al implementar.
  metricasIdentidad: '/admin/usuarios/metricas',
  metricasCatalogo: '/admin/productos/metricas',
  metricasControl: '/scraper/metricas',
} as const;

// --- Mapeo rol UI <-> backend (SUP-242) ---
// UI RBAC: Administrador | Colaborador | Usuario.
// Backend api.usuarios: 'admin' | 'colaborador' | 'registrado'.
// El plan (Básico/Premium) es ortogonal al rol y viaja en su propio campo.
export function adminRoleToBackend(role: AdminRole): string {
  if (role === 'Administrador') return 'admin';
  if (role === 'Colaborador') return 'colaborador';
  return 'registrado';
}

export function adminRoleFromBackend(rol: string): AdminUserRole {
  switch (rol.trim().toLowerCase()) {
    case 'admin':
    case 'administrador':
      return 'Administrador';
    case 'colaborador':
      return 'Colaborador';
    case 'registrado':
    case 'usuario':
      return 'Usuario';
    default:
      return 'Desconocido';
  }
}

// --- Mapeo plan UI <-> backend ---
// Display: Básico | Premium. Un valor desconocido no se inventa: la UI muestra "—".
export function adminPlanFromBackend(plan?: string | null): AdminPlan | null {
  if (!plan) return null;
  switch (plan.trim().toLowerCase()) {
    case 'basico':
    case 'básico':
      return 'Básico';
    case 'premium':
    case 'plus':
      return 'Premium';
    default:
      return null;
  }
}

export function adminPlanToBackend(plan: AdminPlan): string {
  return plan === 'Premium' ? 'premium' : 'basico';
}

// --- Mapeo usuario backend <-> modelo de UI (SUP-242) ---
// Identidad devuelve el DTO de api.usuarios en español; la UI usa AdminUser.
type UsuarioBackend = {
  id: string;
  correo: string;
  nombre_completo: string;
  rol: string;
  plan?: string | null;
  esta_activo: boolean;
  ultimo_acceso?: string | null;
  creado_el?: string | null;
  actualizado_el?: string | null;
};

type PaginacionBackend = {
  total_registros: number;
  pagina_actual: number;
  total_paginas: number;
  limite: number;
};

type PaginaUsuariosBackend = {
  data: UsuarioBackend[];
  paginacion: PaginacionBackend;
};

export type AdminUsersPage = {
  users: AdminUser[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
};

// Sólo los usuarios reales tienen UUID; los mocks usan ids `usr-00X`.
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isPersistableId(id: string): boolean {
  return UUID_RE.test(id);
}

function initialsFromName(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0]?.[0] ?? '';
  const last = parts.length > 1 ? parts[parts.length - 1]?.[0] ?? '' : '';
  return (first + last).toUpperCase() || '?';
}

// `ultimo_acceso` se actualiza al iniciar sesión. Si no existe todavía para una
// cuenta antigua se muestra "Sin registros"; no se usa `actualizado_el` como
// sustituto porque no representa un acceso.
function formatLastAccess(value?: string | null): string {
  if (!value) return 'Sin registros';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Sin registros';
  return date.toLocaleString('es-CL', { dateStyle: 'short', timeStyle: 'short' });
}

export function adminUserFromBackend(usuario: UsuarioBackend): AdminUser {
  return {
    id: usuario.id,
    name: usuario.nombre_completo,
    email: usuario.correo,
    role: adminRoleFromBackend(usuario.rol),
    plan: adminPlanFromBackend(usuario.plan),
    status: usuario.esta_activo ? 'Activo' : 'Inactivo',
    lastAccess: formatLastAccess(usuario.ultimo_acceso),
    initials: initialsFromName(usuario.nombre_completo),
  };
}

// --- Métricas por servicio (SUP-279) ---
export type IdentidadMetrics = { totalUsuarios: number; usuariosActivos: number };
export type CatalogoMetrics = { cadenas: number; sucursales: number; productos: number; capturas: number };
export type ControlMetrics = {
  trabajosPorEstado: Record<string, number>;
  tasaExito: number;
  tasaError: number;
};

// Error de autorización del backend (auditoría H-03).
// Permite a los hooks distinguir 401/403 (fail-closed, sin mocks) del resto
// de fallos de red (fallback visual a mocks). Los hooks de useAdminData lo
// usan para cerrar la sesión y volver a /login (SUP-256).
export class AdminAuthError extends Error {
  readonly status: 401 | 403;
  constructor(status: 401 | 403, path: string) {
    super(`Sin autorización en ${path} (${status})`);
    this.name = 'AdminAuthError';
    this.status = status;
  }
}

export function isAdminAuthError(error: unknown): error is AdminAuthError {
  return error instanceof AdminAuthError;
}

// Error no autoritativo del backend con el mensaje de negocio (por ejemplo,
// el 409 de "último administrador") en vez de un texto técnico genérico.
export class AdminApiError extends Error {
  readonly status: number;
  readonly detail?: string;
  constructor(status: number, message: string, detail?: string) {
    super(message);
    this.name = 'AdminApiError';
    this.status = status;
    this.detail = detail;
  }
}

export function isAdminApiError(error: unknown): error is AdminApiError {
  return error instanceof AdminApiError;
}

type ErrorBackend = { mensaje?: string; detalle?: string };

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // Auditoría: no enviar Content-Type en GET sin cuerpo (evita preflights
  // OPTIONS innecesarios) y combinar headers en vez de sobrescribirlos.
  const hasBody = init?.body !== undefined && init?.body !== null;
  const headers = {
    ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
    ...init?.headers,
  };
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    ...init,
    headers,
  });
  if (response.status === 401 || response.status === 403) {
    throw new AdminAuthError(response.status, path);
  }
  if (!response.ok) {
    let message = `La API respondió con ${response.status} en ${path}`;
    let detail: string | undefined;
    try {
      const body = await response.json() as ErrorBackend;
      if (body.mensaje) message = body.mensaje;
      detail = body.detalle;
    } catch {
      // Respuesta sin JSON: conservar el mensaje con el estado.
    }
    throw new AdminApiError(response.status, message, detail);
  }
  return response.json() as Promise<T>;
}

// Usuarios (SUP-242). GET paginado con búsqueda/rol del servidor.
// PUT acepta esta_activo, rol y plan por separado.
export type AdminUsersQuery = {
  page?: number;
  limit?: number;
  q?: string;
  rol?: string;
};

export async function fetchAdminUsers(query: AdminUsersQuery = {}, signal?: AbortSignal): Promise<AdminUsersPage> {
  const params = new URLSearchParams();
  if (query.page) params.set('page', String(query.page));
  if (query.limit) params.set('limit', String(query.limit));
  if (query.q) params.set('q', query.q);
  if (query.rol) params.set('rol', query.rol);
  const suffix = params.toString() ? `?${params.toString()}` : '';
  const pagina = await request<PaginaUsuariosBackend>(`${ADMIN_ENDPOINTS.usuarios}${suffix}`, { signal });
  return {
    users: pagina.data.map(adminUserFromBackend),
    total: pagina.paginacion.total_registros,
    page: pagina.paginacion.pagina_actual,
    limit: pagina.paginacion.limite,
    totalPages: pagina.paginacion.total_paginas,
  };
}

export async function updateAdminUser(
  id: string,
  patch: { esta_activo?: boolean; rol?: string; plan?: string },
): Promise<AdminUser> {
  const usuario = await request<UsuarioBackend>(`${ADMIN_ENDPOINTS.usuarios}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  });
  return adminUserFromBackend(usuario);
}

// Fuentes (SUP-243). El backend devuelve el contrato de Catálogo en español;
// se adapta aquí al modelo de la UI para no acoplar los componentes al DTO.
type FuenteBackend = {
  id: number;
  nombre: string;
  habilitada: boolean;
  sucursales: number;
  productos: number;
  ultima_extraccion?: string | null;
};

function formatLastSync(value?: string | null): string {
  if (!value) return 'Sin registros';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Sin registros';
  return date.toLocaleString('es-CL', { dateStyle: 'short', timeStyle: 'short' });
}

export function adminSourceFromBackend(fuente: FuenteBackend): AdminSource {
  return {
    id: String(fuente.id),
    name: fuente.nombre,
    type: 'Supermercado',
    status: fuente.habilitada ? 'Operativa' : 'Inactiva',
    lastSync: formatLastSync(fuente.ultima_extraccion),
    products: fuente.productos,
    branches: fuente.sucursales,
    habilitada: fuente.habilitada,
  };
}

// Fuentes (SUP-243). GET lista, PUT { habilitada } por id.
export async function fetchAdminSources(signal?: AbortSignal): Promise<AdminSource[]> {
  const fuentes = await request<FuenteBackend[]>(ADMIN_ENDPOINTS.fuentes, { signal });
  return fuentes.map(adminSourceFromBackend);
}

export async function updateAdminSource(id: string, patch: { habilitada: boolean }): Promise<AdminSource> {
  const fuente = await request<FuenteBackend>(`${ADMIN_ENDPOINTS.fuentes}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  });
  return adminSourceFromBackend(fuente);
}

// Métricas (SUP-279). Cada servicio se pide por separado; el hook las combina
// con Promise.allSettled para tolerar fallo parcial (SUP-280).
export function fetchIdentidadMetrics(signal?: AbortSignal): Promise<IdentidadMetrics> {
  return request<IdentidadMetrics>(ADMIN_ENDPOINTS.metricasIdentidad, { signal });
}

export function fetchCatalogoMetrics(signal?: AbortSignal): Promise<CatalogoMetrics> {
  return request<CatalogoMetrics>(ADMIN_ENDPOINTS.metricasCatalogo, { signal });
}

export function fetchControlMetrics(signal?: AbortSignal): Promise<ControlMetrics> {
  return request<ControlMetrics>(ADMIN_ENDPOINTS.metricasControl, { signal });
}
