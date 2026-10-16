// Capa de acceso a endpoints de administración (SUP-242 / SUP-243 / SUP-279).
// Hoy el backend aún no expone estas rutas (están To do), por lo que los hooks
// usan mocks y estas funciones quedan listas para cuando se creen.
//
// Convención de ramas: backend/feat/[tarea]-[nombre], frontend/feat/[tarea]-[nombre].
// Al crear el backend, solo hay que descomentar/ajustar ADMIN_ENDPOINTS y el
// mapeo de abajo; la UI ya habla contra estas funciones.

import type { AdminRole, AdminSource, AdminUser } from './admin-mock-data';

const API_BASE = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');
export const ADMIN_HAS_BACKEND = API_BASE.length > 0;

export const ADMIN_ENDPOINTS = {
  // SUP-242 — GET/PUT /api/v1/admin/usuarios (RequireRole admin, tras el Gateway).
  // Body PUT: { esta_activo?: boolean; rol?: 'registrado' | 'colaborador' | 'admin' }.
  usuarios: '/api/v1/admin/usuarios',
  // SUP-243 — GET/PUT de fuentes bajo /api/v1/admin/productos ya enrutado.
  // Ej.: /api/v1/admin/productos/fuentes — persiste scraper.cadenas_supermercado.
  fuentes: '/api/v1/admin/productos/fuentes',
  // SUP-279 — un endpoint de métricas por servicio, sin agregación en Gateway.
  // Identidad bajo /api/v1/admin/usuarios, Catálogo bajo /api/v1/admin/productos,
  // Control bajo /api/v1/scraper. Nombres exactos a confirmar al implementar.
  metricasIdentidad: '/api/v1/admin/usuarios/metricas',
  metricasCatalogo: '/api/v1/admin/productos/metricas',
  metricasControl: '/api/v1/scraper/metricas',
} as const;

// --- Mapeo rol UI <-> backend (SUP-242) ---
// UI: Administrador | Colaborador | Premium | Usuario
// Backend api.usuarios: rol 'admin' | 'colaborador' | 'registrado' (+ plan premium
// como campo aparte). 'Premium' y 'Usuario' viajan como 'registrado' hasta que
// el backend defina el flag de plan.
export function adminRoleToBackend(role: AdminRole): string {
  if (role === 'Administrador') return 'admin';
  if (role === 'Colaborador') return 'colaborador';
  return 'registrado';
}

export function adminRoleFromBackend(rol: string, plan?: string | null): AdminRole {
  if (rol === 'admin') return 'Administrador';
  if (rol === 'colaborador') return 'Colaborador';
  if (plan === 'premium' || plan === 'plus' || plan === 'Premium') return 'Premium';
  return 'Usuario';
}

// --- Métricas por servicio (SUP-279) ---
export type IdentidadMetrics = { totalUsuarios: number; usuariosActivos: number };
export type CatalogoMetrics = { cadenas: number; sucursales: number; productos: number; capturas: number };
export type ControlMetrics = {
  trabajosPorEstado: Record<string, number>;
  tasaExito: number;
  tasaError: number;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!response.ok) throw new Error(`La API respondió con ${response.status} en ${path}`);
  return response.json() as Promise<T>;
}

// Usuarios (SUP-242). GET lista, PUT { esta_activo, rol } por id.
export function fetchAdminUsers(signal?: AbortSignal): Promise<AdminUser[]> {
  return request<AdminUser[]>(ADMIN_ENDPOINTS.usuarios, { signal });
}

export function updateAdminUser(
  id: string,
  patch: { esta_activo?: boolean; rol?: string },
): Promise<AdminUser> {
  return request<AdminUser>(`${ADMIN_ENDPOINTS.usuarios}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  });
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
