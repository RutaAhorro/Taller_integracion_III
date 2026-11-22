// Roles RBAC del JWT de Identidad (SUP-256, RN-11).
// Backend: api.usuarios.rol 'admin' | 'colaborador' | 'registrado'.
// El frontend solo compara en minúsculas; nunca inventa roles.
export const ROLE_ADMIN = 'admin';
export const ROLE_COLABORADOR = 'colaborador';
export const ROLE_REGISTRADO = 'registrado';

export function normalizeRole(rol?: string | null): string {
  return (rol ?? '').trim().toLowerCase();
}

export function hasAllowedRole(rol: string | null | undefined, rolesPermitidos: string[]): boolean {
  const rolActual = normalizeRole(rol);
  return rolesPermitidos.some((rolPermitido) => normalizeRole(rolPermitido) === rolActual);
}
