export type AdminSection = 'overview' | 'users' | 'sources' | 'metrics';

// Roles RBAC del panel (SUP-255: Editor -> Colaborador).
// El plan comercial va aparte (auditoría H-06): no se mezcla con el rol.
// Mapeo backend implementado en admin-api.ts (SUP-242, api.usuarios):
// admin <-> 'admin', colaborador <-> 'colaborador', usuario <-> 'registrado'.
// El plan premium viaja como campo aparte (p. ej. plan='premium'), no como rol.
export const ADMIN_ROLES = ['Administrador', 'Colaborador', 'Usuario'] as const;
export type AdminRole = (typeof ADMIN_ROLES)[number];

export const ADMIN_PLANS = ['Básico', 'Premium'] as const;
export type AdminPlan = (typeof ADMIN_PLANS)[number];

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  role: AdminRole;
  plan: AdminPlan;
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
  // Flag administrable (SUP-243: scraper.cadenas_supermercado).
  // Operativa/Advertencia => habilitada; Inactiva => deshabilitada.
  habilitada: boolean;
};

// Datos de ejemplo solo visuales (el panel aún no consume backend).
// Correos ficticios en dominio reservado RFC 2606 para no exponer PII en el bundle.
export const adminUsers: AdminUser[] = [
  { id: 'usr-001', name: 'Camila Torres', email: 'camila.torres@example.com', role: 'Administrador', plan: 'Premium', status: 'Activo', lastAccess: 'Hace 12 min', initials: 'CT' },
  { id: 'usr-002', name: 'Matías Rojas', email: 'matias.rojas@example.com', role: 'Colaborador', plan: 'Básico', status: 'Activo', lastAccess: 'Hace 1 h', initials: 'MR' },
  { id: 'usr-003', name: 'Sofía Pérez', email: 'sofia.perez@example.com', role: 'Usuario', plan: 'Premium', status: 'Activo', lastAccess: 'Hace 3 h', initials: 'SP' },
  { id: 'usr-004', name: 'Diego Morales', email: 'diego.morales@example.com', role: 'Usuario', plan: 'Básico', status: 'Inactivo', lastAccess: '12 oct 2026', initials: 'DM' },
  { id: 'usr-005', name: 'Valentina Silva', email: 'valentina.silva@example.com', role: 'Colaborador', plan: 'Básico', status: 'Activo', lastAccess: '14 oct 2026', initials: 'VS' },
];

export const adminSources: AdminSource[] = [
  { id: 'src-001', name: 'Jumbo', type: 'Supermercado', status: 'Operativa', lastSync: 'Hoy, 00:42', products: 12840, branches: 8, habilitada: true },
  { id: 'src-002', name: 'Líder', type: 'Supermercado', status: 'Operativa', lastSync: 'Hoy, 00:35', products: 11620, branches: 12, habilitada: true },
  { id: 'src-003', name: 'Santa Isabel', type: 'Supermercado', status: 'Advertencia', lastSync: 'Ayer, 23:18', products: 9340, branches: 6, habilitada: true },
  { id: 'src-004', name: 'API de precios regional', type: 'API externa', status: 'Operativa', lastSync: 'Hoy, 00:50', products: 5620, branches: 0, habilitada: true },
  { id: 'src-005', name: 'Carga inicial agosto', type: 'Carga manual', status: 'Inactiva', lastSync: '15 ago 2026', products: 2100, branches: 0, habilitada: false },
];

// Mock de métricas por servicio (SUP-280, a reemplazar por SUP-279 cuando exista backend).
// Cada bloque equivale a la respuesta de un endpoint distinto; el frontend los
// combina y tolera que uno falle (tarjeta en "sin datos").
export const adminMetricsMock = {
  identidad: { totalUsuarios: 1284, usuariosActivos: 1126 },
  catalogo: { cadenas: 8, sucursales: 26, productos: 41520, capturas: 96210 },
  control: {
    trabajosPorEstado: { completado: 42, fallido: 3, en_curso: 2, pendiente: 5 },
    tasaExito: 93.3,
    tasaError: 6.7,
  },
};
