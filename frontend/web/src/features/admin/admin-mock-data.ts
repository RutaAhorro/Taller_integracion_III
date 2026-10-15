export type AdminSection = 'overview' | 'users' | 'sources';

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  role: 'Administrador' | 'Editor' | 'Usuario';
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
};

export const adminUsers: AdminUser[] = [
  { id: 'usr-001', name: 'Camila Torres', email: 'camila.torres@uct.cl', role: 'Administrador', status: 'Activo', lastAccess: 'Hace 12 min', initials: 'CT' },
  { id: 'usr-002', name: 'Matías Rojas', email: 'matias.rojas@uct.cl', role: 'Editor', status: 'Activo', lastAccess: 'Hace 1 h', initials: 'MR' },
  { id: 'usr-003', name: 'Sofía Pérez', email: 'sofia.perez@uct.cl', role: 'Usuario', status: 'Activo', lastAccess: 'Hace 3 h', initials: 'SP' },
  { id: 'usr-004', name: 'Diego Morales', email: 'diego.morales@uct.cl', role: 'Usuario', status: 'Inactivo', lastAccess: '12 oct 2026', initials: 'DM' },
  { id: 'usr-005', name: 'Valentina Silva', email: 'valentina.silva@uct.cl', role: 'Editor', status: 'Activo', lastAccess: '14 oct 2026', initials: 'VS' },
];

export const adminSources: AdminSource[] = [
  { id: 'src-001', name: 'Jumbo', type: 'Supermercado', status: 'Operativa', lastSync: 'Hoy, 00:42', products: 12840, branches: 8 },
  { id: 'src-002', name: 'Líder', type: 'Supermercado', status: 'Operativa', lastSync: 'Hoy, 00:35', products: 11620, branches: 12 },
  { id: 'src-003', name: 'Santa Isabel', type: 'Supermercado', status: 'Advertencia', lastSync: 'Ayer, 23:18', products: 9340, branches: 6 },
  { id: 'src-004', name: 'API de precios regional', type: 'API externa', status: 'Operativa', lastSync: 'Hoy, 00:50', products: 5620, branches: 0 },
  { id: 'src-005', name: 'Carga inicial agosto', type: 'Carga manual', status: 'Inactiva', lastSync: '15 ago 2026', products: 2100, branches: 0 },
];
