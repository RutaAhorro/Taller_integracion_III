// Cliente de administración: adapta usuarios, fuentes y métricas y conserva los errores reales.
// VITE_API_URL incluye /api/v1; sin configuración, los hooks informan la indisponibilidad.

import type { AdminPlan, AdminRole, AdminSource, AdminUser, AdminUserRole } from './admin-types';

const URL_BASE_API = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');
export const ADMIN_HAS_BACKEND = URL_BASE_API.length > 0;

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
  // Identidad ya expone /admin/usuarios/metricas (SUP-242); Catálogo y Control
  // quedan pendientes de sus tickets.
  metricasIdentidad: '/admin/usuarios/metricas',
  metricasCatalogo: '/admin/productos/metricas',
  metricasControl: '/scraper/metricas',
} as const;

// --- Mapeo rol UI <-> backend (SUP-242) ---
// UI RBAC: Administrador | Colaborador | Usuario.
// Backend api.usuarios: 'admin' | 'colaborador' | 'registrado'.
// El plan (Básico/Premium) es ortogonal al rol y viaja en su propio campo.
export function adminRoleToBackend(rol: AdminRole): string {
  if (rol === 'Administrador') return 'admin';
  if (rol === 'Colaborador') return 'colaborador';
  return 'registrado';
}

export function adminRoleFromBackend(rol: string): AdminUserRole {
  switch (rol.trim().toLowerCase()) {
    case 'admin':
      return 'Administrador';
    case 'colaborador':
      return 'Colaborador';
    case 'registrado':
      return 'Usuario';
    default:
      return 'Desconocido';
  }
}

// --- Mapeo plan UI <-> backend ---
// Presentación: Básico o Premium. Un valor desconocido se muestra como "—".
export function adminPlanFromBackend(plan?: string | null): AdminPlan | null {
  if (!plan) return null;
  switch (plan.trim().toLowerCase()) {
    case 'basico':
      return 'Básico';
    case 'premium':
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

// Identidad utiliza UUID para los identificadores de usuario.
const EXPRESION_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isPersistableId(id: string): boolean {
  return EXPRESION_UUID.test(id);
}

function inicialesDesdeNombre(nombre: string): string {
  const partes = nombre.trim().split(/\s+/).filter(Boolean);
  if (partes.length === 0) return '?';
  const primera = partes[0]?.[0] ?? '';
  const ultima = partes.length > 1 ? partes[partes.length - 1]?.[0] ?? '' : '';
  return (primera + ultima).toUpperCase() || '?';
}

// `ultimo_acceso` se actualiza al iniciar sesión. Si no existe todavía para una
// cuenta antigua se muestra "Sin registros"; no se usa `actualizado_el` como
// sustituto porque no representa un acceso.
function formatearUltimoAcceso(valor?: string | null): string {
  if (!valor) return 'Sin registros';
  const fecha = new Date(valor);
  if (Number.isNaN(fecha.getTime())) return 'Sin registros';
  return fecha.toLocaleString('es-CL', { dateStyle: 'short', timeStyle: 'short' });
}

export function adminUserFromBackend(usuario: UsuarioBackend): AdminUser {
  return {
    id: usuario.id,
    name: usuario.nombre_completo,
    email: usuario.correo,
    role: adminRoleFromBackend(usuario.rol),
    plan: adminPlanFromBackend(usuario.plan),
    status: usuario.esta_activo ? 'Activo' : 'Inactivo',
    lastAccess: formatearUltimoAcceso(usuario.ultimo_acceso),
    initials: inicialesDesdeNombre(usuario.nombre_completo),
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
// Permite a los hooks distinguir 401/403 del resto
// de fallos de red. Los hooks de useAdminData lo
// usan para cerrar la sesión y volver a /login (SUP-256).
export class AdminAuthError extends Error {
  readonly status: 401 | 403;
  constructor(estado: 401 | 403, ruta: string) {
    super(`Sin autorización en ${ruta} (${estado})`);
    this.name = 'AdminAuthError';
    this.status = estado;
  }
}

export function isAdminAuthError(errorObtenido: unknown): errorObtenido is AdminAuthError {
  return errorObtenido instanceof AdminAuthError;
}

// Error no autoritativo del backend con el mensaje de negocio (por ejemplo,
// el 409 de "último administrador") en vez de un texto técnico genérico.
export class AdminApiError extends Error {
  readonly status: number;
  readonly detail?: string;
  constructor(estado: number, mensaje: string, detalle?: string) {
    super(mensaje);
    this.name = 'AdminApiError';
    this.status = estado;
    this.detail = detalle;
  }
}

export function isAdminApiError(errorObtenido: unknown): errorObtenido is AdminApiError {
  return errorObtenido instanceof AdminApiError;
}

type ErrorBackend = { mensaje?: string; error?: string; detalle?: string };

/** Envía la cookie de sesión y distingue errores de permisos de los errores de negocio o conexión. */
async function solicitar<Respuesta>(ruta: string, opciones?: RequestInit): Promise<Respuesta> {
  // Auditoría: no enviar Content-Type en GET sin cuerpo (evita preflights
  // OPTIONS innecesarios) y combinar headers en vez de sobrescribirlos.
  const tieneCuerpo = opciones?.body !== undefined && opciones?.body !== null;
  const cabeceras = {
    ...(tieneCuerpo ? { 'Content-Type': 'application/json' } : {}),
    ...opciones?.headers,
  };
  const respuesta = await fetch(`${URL_BASE_API}${ruta}`, {
    credentials: 'include',
    ...opciones,
    headers: cabeceras,
  });
  if (respuesta.status === 401 || respuesta.status === 403) {
    throw new AdminAuthError(respuesta.status, ruta);
  }
  if (!respuesta.ok) {
    let mensaje = `La API respondió con ${respuesta.status} en ${ruta}`;
    let detalle: string | undefined;
    try {
      const cuerpo = await respuesta.json() as ErrorBackend;
      mensaje = cuerpo.mensaje || cuerpo.error || mensaje;
      detalle = cuerpo.detalle;
    } catch {
      // Respuesta sin JSON: conservar el mensaje con el estado.
    }
    throw new AdminApiError(respuesta.status, mensaje, detalle);
  }
  return respuesta.json() as Promise<Respuesta>;
}

// Usuarios (SUP-242). GET paginado con búsqueda/rol del servidor.
// PUT acepta esta_activo, rol y plan por separado.
export type AdminUsersQuery = {
  page?: number;
  limit?: number;
  q?: string;
  rol?: string;
};

export async function fetchAdminUsers(consulta: AdminUsersQuery = {}, senal?: AbortSignal): Promise<AdminUsersPage> {
  const parametros = new URLSearchParams();
  if (consulta.page) parametros.set('page', String(consulta.page));
  if (consulta.limit) parametros.set('limit', String(consulta.limit));
  if (consulta.q) parametros.set('q', consulta.q);
  if (consulta.rol) parametros.set('rol', consulta.rol);
  const sufijo = parametros.toString() ? `?${parametros.toString()}` : '';
  const pagina = await solicitar<PaginaUsuariosBackend>(`${ADMIN_ENDPOINTS.usuarios}${sufijo}`, { signal: senal });
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
  cambios: { esta_activo?: boolean; rol?: string; plan?: string },
): Promise<AdminUser> {
  const usuario = await solicitar<UsuarioBackend>(`${ADMIN_ENDPOINTS.usuarios}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(cambios),
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

function formatearUltimaSincronizacion(valor?: string | null): string {
  if (!valor) return 'Sin registros';
  const fecha = new Date(valor);
  if (Number.isNaN(fecha.getTime())) return 'Sin registros';
  return fecha.toLocaleString('es-CL', { dateStyle: 'short', timeStyle: 'short' });
}

export function adminSourceFromBackend(fuente: FuenteBackend): AdminSource {
  return {
    id: String(fuente.id),
    name: fuente.nombre,
    type: 'Supermercado',
    status: !fuente.habilitada ? 'Inactiva' : fuente.productos > 0 ? 'Operativa' : 'Sin datos',
    lastSync: formatearUltimaSincronizacion(fuente.ultima_extraccion),
    products: fuente.productos,
    branches: fuente.sucursales,
    habilitada: fuente.habilitada,
  };
}

// Fuentes (SUP-243). GET lista, PUT { habilitada } por id.
export async function fetchAdminSources(senal?: AbortSignal): Promise<AdminSource[]> {
  const fuentes = await solicitar<FuenteBackend[]>(ADMIN_ENDPOINTS.fuentes, { signal: senal });
  return fuentes.map(adminSourceFromBackend);
}

export async function updateAdminSource(id: string, cambios: { habilitada: boolean }): Promise<AdminSource> {
  const fuente = await solicitar<FuenteBackend>(`${ADMIN_ENDPOINTS.fuentes}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(cambios),
  });
  return adminSourceFromBackend(fuente);
}

// Métricas (SUP-279/SUP-242). Identidad expone el conteo real de cuentas;
// Catálogo y Control siguen pendientes de sus tickets.
export function fetchIdentidadMetrics(senal?: AbortSignal): Promise<IdentidadMetrics> {
  return solicitar<{ total_usuarios: number; usuarios_activos: number }>(
    ADMIN_ENDPOINTS.metricasIdentidad,
    { signal: senal },
  ).then((datos) => ({ totalUsuarios: datos.total_usuarios, usuariosActivos: datos.usuarios_activos }));
}

export function fetchCatalogoMetrics(senal?: AbortSignal): Promise<CatalogoMetrics> {
  return solicitar<CatalogoMetrics>(ADMIN_ENDPOINTS.metricasCatalogo, { signal: senal });
}

export function fetchControlMetrics(senal?: AbortSignal): Promise<ControlMetrics> {
  return solicitar<ControlMetrics>(ADMIN_ENDPOINTS.metricasControl, { signal: senal });
}
