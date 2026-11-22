// Deriva los permisos de navegación de la interfaz a partir del rol de la sesión (no del plan).
import { useAuth } from '@/features/autenticacion/useAuth';
import { normalizeRole, ROLE_ADMIN, ROLE_COLABORADOR } from '@/features/autenticacion/roles';

// Consulta el rol para los controles de acceso y el menú (SUP-256).
// canAccessColaborador cubre /colaborador: colaborador o admin.
export function useRole() {
  const { user, isGuest } = useAuth();
  // Invariante de autenticación (SUP-256): un invitado nunca computa rol;
  // evita estados mixtos isGuest + isAdmin con localStorage corrupto.
  const role = isGuest ? '' : normalizeRole(user?.rol);
  const isAdmin = role === ROLE_ADMIN;
  const isColaborador = role === ROLE_COLABORADOR;
  const canAccessColaborador = isAdmin || isColaborador;
  return { role, isGuest, isAdmin, isColaborador, canAccessColaborador };
}
