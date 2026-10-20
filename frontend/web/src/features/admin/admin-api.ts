// Capa de acceso a endpoints de administración (SUP-242 / SUP-243 / SUP-279).
// Usa VITE_API_URL como base (ya incluye /api/v1); sin esa variable los hooks
// caen a mocks. SUP-242 (usuarios) ya tiene backend en Identidad; SUP-243
// (fuentes) y SUP-279 (métricas) dependen de sus propias tareas.
//
// Convención de ramas: backend/feat/[tarea]-[nombre], frontend/feat/[tarea]-[nombre].

import type { AdminPlan, AdminRole, AdminSource, AdminUser } from './admin-mock-data';

const API_BASE = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');
export const ADMIN_HAS_BACKEND = API_BASE.length > 0;

// Rutas relativas a VITE_API_URL, que ya incluye el prefijo `/api/v1` igual
// que AuthContext y products-api. Concatenar otro `/api/v1` producía
// `/api/v1/api/v1/...` y el Gateway devolvía 404.
// SUP-242: `/admin/usuarios*` ya está enrutado hacia Identidad. SUP-243
// (`/admin/productos*`) y Control (`/scraper*`) conservan sus rutas.
export const ADMIN_ENDPOINTS = {
  // SUP-242 — GET/PUT /api/v1/admin/usuarios (RequireRole admin, tras el Gateway).
  // Body PUT: { esta_activo?: boolean; rol?: 'registrado' | 'colaborador' | 'admin' }.
  // El plan comercial NO viaja en `rol` (auditoría H-06); va en campo aparte.
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

export function adminRoleFromBackend(rol: string): AdminRole {
  if (rol === 'admin') return 'Administrador';
  if (rol === 'colaborador') return 'Colaborador';
  return 'Usuario';
}

// --- Mapeo plan UI <-> backend ---
// Display: Básico | Premium. Backend: campo de plan/suscripción aparte
// (p. ej. plan='premium'); nunca se envía como `rol`.
export function adminPlanToBackend(plan: AdminPlan): string {
  return plan === 'Premium' ? 'premium' : 'basico';
}

export function adminPlanFromBackend(plan?: string | null): AdminPlan {
  if (plan === 'premium' || plan === 'plus' || plan === 'Premium') return 'Premium';
  return 'Básico';
}

// --- Mapeo usuario backend <-> modelo de UI (SUP-242) ---
// Identidad devuelve el DTO de api.usuarios en español; la UI usa AdminUser.
type UsuarioBackend = {
  id: string;
  correo: string;
  nombre_completo: string;
  rol: string;
  esta_activo: boolean;
  plan?: string | null;
  creado_el?: string | null;
  actualizado_el?: string | null;
};

function initialsFromName(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0]?.[0] ?? '';
  const last = parts.length > 1 ? parts[parts.length - 1]?.[0] ?? '' : '';
  return (first + last).toUpperCase() || '?';
}

// Última actualización como aproximación: api.usuarios aún no expone último acceso.
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
    lastAccess: formatLastAccess(usuario.actualizado_el ?? usuario.creado_el),
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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // Auditoría: no enviar Content-Type en GET sin cuerpo (evita preflights
  // OPTIONS innecesarios) y combinar headers en vez de sobrescribirlos.
  const hasBody = init?.body !== undefined && init?.body !== null;
  const headers = {
    ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
    ...(init?.headers ?? {}),
  };
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    ...init,
    headers,
  });
  if (response.status === 401 || response.status === 403) {
    throw new AdminAuthError(response.status, path);
  }
  if (!response.ok) throw new Error(`La API respondió con ${response.status} en ${path}`);
  return response.json() as Promise<T>;
}

// Usuarios (SUP-242). GET lista, PUT { esta_activo, rol, plan } por id.
// `rol` solo acepta valores RBAC; el plan viaja en su propio campo.
export async function fetchAdminUsers(signal?: AbortSignal): Promise<AdminUser[]> {
  const usuarios = await request<UsuarioBackend[]>(ADMIN_ENDPOINTS.usuarios, { signal });
  return usuarios.map(adminUserFromBackend);
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

// Fuentes (SUP-243). GET lista, PUT { habilitada } por id.
export function fetchAdminSources(signal?: AbortSignal): Promise<AdminSource[]> {
  return request<AdminSource[]>(ADMIN_ENDPOINTS.fuentes, { signal });
}

export function updateAdminSource(id: string, patch: { habilitada: boolean }): Promise<AdminSource> {
  return request<AdminSource>(`${ADMIN_ENDPOINTS.fuentes}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  });
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
