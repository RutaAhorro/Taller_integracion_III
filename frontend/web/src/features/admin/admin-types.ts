// Contratos de presentación del panel; separan permisos, planes y datos reales del servidor.
export type AdminSection = 'overview' | 'users' | 'sources' | 'metrics';

// Roles RBAC del panel (SUP-255: Editor -> Colaborador).
// El plan comercial va aparte (auditoría H-06): no se mezcla con el rol.
// Mapeo backend implementado en admin-api.ts (SUP-242, api.usuarios):
// admin <-> 'admin', colaborador <-> 'colaborador', usuario <-> 'registrado'.
// El plan premium viaja como campo aparte (p. ej. plan='premium'), no como rol.
export const ADMIN_ROLES = ['Administrador', 'Colaborador', 'Usuario'] as const;
export type AdminRole = (typeof ADMIN_ROLES)[number];
// Un rol no reconocido se muestra, pero no se puede convertir en otro por accidente.
export type AdminUserRole = AdminRole | 'Desconocido';

export const ADMIN_PLANS = ['Básico', 'Premium'] as const;
export type AdminPlan = (typeof ADMIN_PLANS)[number];

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  role: AdminUserRole;
  // null cuando el backend no informa un plan reconocido (se muestra "—").
  plan: AdminPlan | null;
  status: 'Activo' | 'Inactivo';
  lastAccess: string;
  initials: string;
};

export type AdminSource = {
  id: string;
  name: string;
  type: 'Supermercado' | 'API externa' | 'Carga manual';
  status: 'Operativa' | 'Advertencia' | 'Inactiva';
  lastSync: string;
  products: number;
  branches: number;
  // Indicador administrable de habilitación de la fuente (SUP-243).
  // Operativa/Advertencia => habilitada; Inactiva => deshabilitada.
  habilitada: boolean;
};
