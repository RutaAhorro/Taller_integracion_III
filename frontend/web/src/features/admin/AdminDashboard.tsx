import { useMemo, useState } from 'react';
import {
  Activity,
  ArrowUpRight,
  Database,
  MoreHorizontal,
  Power,
  RefreshCw,
  Search,
  Server,
  TriangleAlert,
  Users,
  Wifi,
} from 'lucide-react';
import { Link, useLocation, useSearchParams } from 'react-router-dom';
import { Input } from '@/components/ui/input';
import { ROUTES } from '@/core/routes';
import { AdminLayout } from './AdminLayout';
import { ADMIN_HAS_BACKEND } from './admin-api';
import { ADMIN_PLANS, ADMIN_ROLES, type AdminSection } from './admin-mock-data';
import { useAdminMetrics, useAdminSources, useAdminUsers, type AdminMetricsState } from './useAdminData';

const statusStyles = {
  Activo: 'bg-[#e3f1e4] text-[#376344]',
  Inactivo: 'bg-[#eef0ed] text-[#667061]',
  Operativa: 'bg-[#e3f1e4] text-[#376344]',
  Advertencia: 'bg-[#fff0d8] text-[#9a641c]',
  Inactiva: 'bg-[#eef0ed] text-[#667061]',
};

function StatusBadge({ status }: { status: keyof typeof statusStyles }) {
  return <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-bold ${statusStyles[status]}`}>{status}</span>;
}

function StatCard({ icon: Icon, label, value, detail, accent, error }: {
  icon: typeof Users;
  label: string;
  value: string;
  detail: string;
  accent: string;
  error?: string | null;
}) {
  return (
    <article className="rounded-2xl border border-[#dfe5dc] bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <span className={`flex size-11 items-center justify-center rounded-xl ${accent}`}><Icon className="size-5" /></span>
        <ArrowUpRight className="size-4 text-muted-foreground" />
      </div>
      <p className="mt-5 text-sm font-medium text-muted-foreground">{label}</p>
      {error ? (
        <>
          <p className="mt-1 text-2xl font-bold tracking-tight text-muted-foreground">Sin datos</p>
          <p className="mt-2 text-xs text-destructive" title={error}>Servicio no disponible</p>
        </>
      ) : (
        <>
          <p className="mt-1 text-3xl font-bold tracking-tight">{value}</p>
          <p className="mt-2 text-xs text-muted-foreground">{detail}</p>
        </>
      )}
    </article>
  );
}

function TableSkeleton({ rows = 4, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-3 p-5" role="status" aria-label="Cargando datos">
      {Array.from({ length: rows }).map((_, row) => (
        <div key={row} className="flex gap-3">
          {Array.from({ length: cols }).map((_, col) => (
            <div key={col} className="h-9 flex-1 animate-pulse rounded-lg bg-[#edf0eb]" />
          ))}
        </div>
      ))}
    </div>
  );
}

function ErrorBanner({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col gap-2 border-b border-[#edf0eb] bg-[#fdf3f1] p-4 text-sm sm:flex-row sm:items-center sm:justify-between" role="alert">
      <p className="flex items-center gap-2 text-destructive">
        <TriangleAlert className="size-4 shrink-0" aria-hidden="true" />
        <span>{message}</span>
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="inline-flex shrink-0 items-center gap-2 rounded-full border border-destructive/30 px-3 py-1.5 text-xs font-bold text-destructive hover:bg-destructive/10"
      >
        <RefreshCw className="size-3.5" aria-hidden="true" />
        Reintentar
      </button>
    </div>
  );
}

// SUP-255: listado de usuarios con activar/desactivar, asignación de rol RBAC
// y plan comercial en columnas separadas (auditoría H-06: el plan nunca viaja
// en `rol`). En la vista general se usa sin acciones (solo lectura); en la
// sección Usuarios se usa con showActions para gestionar (SUP-242).
function UsersTable({ showActions = false }: { showActions?: boolean }) {
  const [query, setQuery] = useState('');
  const [role, setRole] = useState('Todos');
  const { users, isLoading, error, pendingId, refresh, toggleUserStatus, changeUserRole, changeUserPlan } = useAdminUsers();
  const filteredUsers = useMemo(() => users.filter((user) => {
    const matchesQuery = `${user.name} ${user.email}`.toLowerCase().includes(query.toLowerCase());
    return matchesQuery && (role === 'Todos' || user.role === role);
  }), [query, role, users]);

  return (
    <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between">
        <div><h2 className="text-lg font-bold">Usuarios recientes</h2><p className="text-sm text-muted-foreground">Administra las cuentas y sus permisos.</p></div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar usuario..." className="pl-9 shadow-none" /></div>
          <select value={role} onChange={(event) => setRole(event.target.value)} aria-label="Filtrar por rol" className="h-10 rounded-md border-2 border-border bg-background px-3 text-sm font-medium shadow-none outline-none focus:ring-2 focus:ring-ring">
            <option>Todos</option>{ADMIN_ROLES.map((r) => <option key={r}>{r}</option>)}
          </select>
        </div>
      </div>
      {error && <ErrorBanner message={error} onRetry={refresh} />}
      {isLoading ? <TableSkeleton rows={4} cols={showActions ? 6 : 5} /> : (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[840px] text-left text-sm">
              <thead className="bg-[#fafbf9] text-xs uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="px-5 py-3 font-bold">Usuario</th>
                  <th className="px-5 py-3 font-bold">Rol</th>
                  <th className="px-5 py-3 font-bold">Plan</th>
                  <th className="px-5 py-3 font-bold">Estado</th>
                  <th className="px-5 py-3 font-bold">Último acceso</th>
                  {showActions && <th className="px-5 py-3 font-bold">Acción</th>}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#edf0eb]">
                {filteredUsers.map((user) => {
                  const isActive = user.status === 'Activo';
                  const busy = pendingId === user.id;
                  return (
                    <tr key={user.id} className="hover:bg-[#fcfdfb]">
                      <td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-full bg-[#ebe5db] text-xs font-bold text-[#4a5a3f]">{user.initials}</span><div><p className="font-bold">{user.name}</p><p className="text-xs text-muted-foreground">{user.email}</p></div></div></td>
                      <td className="px-5 py-4 text-muted-foreground">
                        {showActions ? (
                          <select
                            value={user.role}
                            disabled={busy}
                            onChange={(event) => void changeUserRole(user.id, event.target.value as (typeof ADMIN_ROLES)[number])}
                            aria-label={`Asignar rol a ${user.name}`}
                            title="Asignar rol RBAC (SUP-242: PUT rol: admin | colaborador | registrado)"
                            className="h-9 rounded-md border-2 border-border bg-background px-2 text-xs font-bold outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
                          >
                            {ADMIN_ROLES.map((r) => <option key={r}>{r}</option>)}
                          </select>
                        ) : user.role}
                      </td>
                      <td className="px-5 py-4 text-muted-foreground">
                        {!ADMIN_HAS_BACKEND && showActions ? (
                          <select
                            value={user.plan ?? ''}
                            disabled={busy}
                            onChange={(event) => changeUserPlan(user.id, event.target.value as (typeof ADMIN_PLANS)[number])}
                            aria-label={`Asignar plan a ${user.name}`}
                            title="Asignar plan comercial (campo aparte, nunca en `rol`)"
                            className="h-9 rounded-md border-2 border-border bg-background px-2 text-xs font-bold outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
                          >
                            {ADMIN_PLANS.map((p) => <option key={p}>{p}</option>)}
                          </select>
                        ) : (
                          <span title={ADMIN_HAS_BACKEND ? 'api.usuarios todavía no expone el plan comercial' : undefined}>
                            {ADMIN_HAS_BACKEND ? '—' : user.plan ?? '—'}
                          </span>
                        )}
                      </td>
                      <td className="px-5 py-4"><StatusBadge status={user.status} /></td>
                      <td className="px-5 py-4 text-muted-foreground">{user.lastAccess}</td>
                      {showActions && (
                        <td className="px-5 py-4">
                          <button
                            type="button"
                            onClick={() => void toggleUserStatus(user.id)}
                            disabled={busy}
                            aria-label={`${isActive ? 'Desactivar' : 'Activar'} a ${user.name}`}
                            title={isActive ? 'Desactivar usuario (SUP-242: esta_activo=false)' : 'Activar usuario (SUP-242: esta_activo=true)'}
                            className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold transition-colors disabled:opacity-50 ${isActive ? 'border-destructive/30 text-destructive hover:bg-destructive/10' : 'border-primary/30 text-primary hover:bg-primary/10'}`}
                          >
                            <Power className="size-3.5" aria-hidden="true" />{busy ? 'Guardando…' : isActive ? 'Desactivar' : 'Activar'}
                          </button>
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {filteredUsers.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos usuarios con esos filtros.</p>}
        </>
      )}
    </section>
  );
}

// SUP-255: listado de fuentes con habilitar/deshabilitar (SUP-243).
function SourcesTable() {
  const [query, setQuery] = useState('');
  const { sources, isLoading, error, pendingId, refresh, toggleSource } = useAdminSources();
  const filteredSources = useMemo(
    () => sources.filter((source) => source.name.toLowerCase().includes(query.toLowerCase())),
    [sources, query],
  );

  return (
    <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between"><div><h2 className="text-lg font-bold">Fuentes conectadas</h2><p className="text-sm text-muted-foreground">Supervisa el estado de las fuentes de precios.</p></div><div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar fuente..." className="pl-9 shadow-none" /></div></div>
      {error && <ErrorBanner message={error} onRetry={refresh} />}
      {isLoading ? <TableSkeleton rows={4} cols={5} /> : (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[820px] text-left text-sm">
              <thead className="bg-[#fafbf9] text-xs uppercase tracking-wider text-muted-foreground"><tr><th className="px-5 py-3 font-bold">Fuente</th><th className="px-5 py-3 font-bold">Estado</th><th className="px-5 py-3 font-bold">Última sincronización</th><th className="px-5 py-3 font-bold">Productos</th><th className="px-5 py-3 font-bold">Sucursales</th><th className="px-5 py-3 font-bold">Habilitada</th><th className="px-5 py-3" /></tr></thead>
              <tbody className="divide-y divide-[#edf0eb]">
                {filteredSources.map((source) => {
                  const busy = pendingId === source.id;
                  return (
                    <tr key={source.id} className="hover:bg-[#fcfdfb]">
                      <td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-lg bg-[#ebe5db] text-sm font-bold text-[#4a5a3f]">{source.name.charAt(0)}</span><div><p className="font-bold">{source.name}</p><p className="text-xs text-muted-foreground">{source.type}</p></div></div></td>
                      <td className="px-5 py-4"><StatusBadge status={source.status} /></td>
                      <td className="px-5 py-4 text-muted-foreground">{source.lastSync}</td>
                      <td className="px-5 py-4 font-mono text-xs">{source.products.toLocaleString('es-CL')}</td>
                      <td className="px-5 py-4 text-muted-foreground">{source.branches || '—'}</td>
                      <td className="px-5 py-4">
                        <button
                          type="button"
                          onClick={() => void toggleSource(source.id)}
                          disabled={busy}
                          aria-label={`${source.habilitada ? 'Deshabilitar' : 'Habilitar'} fuente ${source.name}`}
                          title={source.habilitada ? 'Deshabilitar fuente (SUP-243)' : 'Habilitar fuente (SUP-243)'}
                          className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold transition-colors disabled:opacity-50 ${source.habilitada ? 'border-destructive/30 text-destructive hover:bg-destructive/10' : 'border-primary/30 text-primary hover:bg-primary/10'}`}
                        >
                          <Power className="size-3.5" aria-hidden="true" />{busy ? 'Guardando…' : source.habilitada ? 'Deshabilitar' : 'Habilitar'}
                        </button>
                      </td>
                      <td className="px-5 py-4 text-right"><button type="button" aria-label={`Más acciones para ${source.name}`} className="rounded-lg p-2 text-muted-foreground hover:bg-secondary hover:text-foreground"><MoreHorizontal className="size-5" /></button></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {filteredSources.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos fuentes con esa búsqueda.</p>}
        </>
      )}
    </section>
  );
}

const fmt = (n: number) => n.toLocaleString('es-CL');

// SUP-280: tarjetas de conteo combinando los tres servicios en paralelo.
// Si un servicio falla, su tarjeta muestra "Sin datos" sin romper la vista.
function MetricsCards({ metrics }: { metrics: AdminMetricsState }) {
  const { identidad, catalogo, control } = metrics;
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        icon={Users}
        label="Usuarios registrados"
        value={identidad.data ? fmt(identidad.data.totalUsuarios) : '—'}
        detail={identidad.data ? 'Identidad (SUP-279)' : 'Identidad sin datos'}
        accent="bg-[#e3f1e4] text-[#376344]"
        error={identidad.error}
      />
      <StatCard
        icon={Activity}
        label="Usuarios activos"
        value={identidad.data ? fmt(identidad.data.usuariosActivos) : '—'}
        detail={identidad.data ? `${((identidad.data.usuariosActivos / Math.max(identidad.data.totalUsuarios, 1)) * 100).toFixed(1).replace('.', ',')}% del total` : 'Identidad sin datos'}
        accent="bg-[#e8f0f3] text-[#416978]"
        error={identidad.error}
      />
      <StatCard
        icon={Database}
        label="Fuentes activas"
        value={catalogo.data ? String(catalogo.data.cadenas) : '—'}
        detail={catalogo.data ? `${catalogo.data.sucursales} sucursales · Catálogo` : 'Catálogo sin datos'}
        accent="bg-[#fff0d8] text-[#9a641c]"
        error={catalogo.error}
      />
      <StatCard
        icon={Server}
        label="Productos indexados"
        value={catalogo.data ? fmt(catalogo.data.productos) : '—'}
        detail={catalogo.data ? `${fmt(catalogo.data.capturas)} capturas` : 'Catálogo sin datos'}
        accent="bg-[#ebe3ea] text-[#765b72]"
        error={catalogo.error}
      />
      {(identidad.error || catalogo.error || control.error) && (
        <p className="text-xs text-muted-foreground sm:col-span-2 xl:col-span-4">
          Fallo parcial tolerado: {control.error ? 'Control' : identidad.error ? 'Identidad' : 'Catálogo'} no disponible.
          <button type="button" onClick={metrics.refresh} className="ml-2 font-bold text-primary hover:underline">Reintentar</button>
        </p>
      )}
    </div>
  );
}

// SUP-280: barras y tabla sin librería pesada, con tokens --chart-*.
function ControlPanel({ metrics }: { metrics: AdminMetricsState }) {
  const { control } = metrics;
  const entries = Object.entries(control.data?.trabajosPorEstado ?? {});
  const max = Math.max(1, ...entries.map(([, v]) => v));
  const chartBars = ['bg-[var(--chart-1)]', 'bg-[var(--chart-2)]', 'bg-[var(--chart-3)]', 'bg-[var(--chart-4)]', 'bg-[var(--chart-5)]'];

  return (
    <div className="rounded-2xl border border-[#dfe5dc] bg-[#263524] p-6 text-white shadow-sm">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-bold text-white/60">Trabajos de scraping (Control)</p>
          {control.data ? (
            <h3 className="mt-2 text-3xl font-bold">
              {control.data.tasaExito.toFixed(1).replace('.', ',')}% <span className="text-base font-medium text-white/60">éxito</span>
            </h3>
          ) : (
            <h3 className="mt-2 text-2xl font-bold text-white/70">Sin datos</h3>
          )}
        </div>
        <Wifi className="size-5 text-[#d4a373]" />
      </div>
      {control.error && (
        <p className="mt-4 rounded-xl bg-white/10 p-3 text-xs text-[#f2c77f]" role="alert">
          Control no disponible: {control.error}
        </p>
      )}
      <div className="mt-6 space-y-3">
        {entries.length === 0 && !control.error && <p className="text-sm text-white/60">Sin trabajos registrados.</p>}
        {entries.map(([estado, total], i) => (
          <div key={estado}>
            <div className="mb-1 flex justify-between text-xs">
              <span className="capitalize text-white/70">{estado.replace('_', ' ')}</span>
              <span className="font-bold">{fmt(total)}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-white/10">
              <div className={`h-full rounded-full ${chartBars[i % chartBars.length]}`} style={{ width: `${(total / max) * 100}%` }} />
            </div>
          </div>
        ))}
      </div>
      {control.data && (
        <div className="mt-5 space-y-3 border-t border-white/10 pt-5 text-sm">
          <div className="flex justify-between"><span className="text-white/65">Tasa de éxito</span><span className="font-bold">{control.data.tasaExito.toFixed(1).replace('.', ',')}%</span></div>
          <div className="flex justify-between"><span className="text-white/65">Tasa de error</span><span className="font-bold text-[#f2c77f]">{control.data.tasaError.toFixed(1).replace('.', ',')}%</span></div>
        </div>
      )}
    </div>
  );
}

function Overview() {
  // Auditoría H-04: las métricas solo se piden al montar esta vista,
  // no en la raíz del dashboard (evita 3 llamadas en /admin?section=users).
  const metrics = useAdminMetrics();
  return (
    <>
      <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="mb-2 text-sm font-bold text-primary">Miércoles, 15 de octubre de 2026</p>
          <h2 className="font-serif text-3xl font-bold tracking-tight md:text-4xl">Hola, administradora 👋</h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">Este es el estado general de tu plataforma. Revisa usuarios y fuentes desde un mismo lugar.</p>
        </div>
        <button
          type="button"
          onClick={metrics.refresh}
          disabled={metrics.isLoading}
          className="inline-flex items-center gap-2 rounded-full border border-border bg-white px-4 py-2 text-xs font-bold text-muted-foreground hover:bg-secondary disabled:opacity-50"
        >
          <RefreshCw className={`size-3.5 ${metrics.isLoading ? 'animate-spin' : ''}`} aria-hidden="true" />
          {metrics.isLoading ? 'Actualizando…' : 'Refrescar métricas'}
        </button>
      </div>
      {metrics.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => <div key={i} className="h-36 animate-pulse rounded-2xl border border-[#dfe5dc] bg-white" />)}
        </div>
      ) : <MetricsCards metrics={metrics} />}
      <div className="mt-6 grid gap-6 xl:grid-cols-[1.3fr_0.7fr]">
        {/* Vista general: solo lectura, sin columna de acción (va en sección Usuarios). */}
        <UsersTable />
        <ControlPanel metrics={metrics} />
      </div>
      <p className="mt-6 text-sm text-muted-foreground">
        ¿Necesitas el detalle por servicio? <Link to={`${ROUTES.ADMIN}?section=metrics`} className="font-bold text-primary hover:underline">Ver métricas</Link>
      </p>
    </>
  );
}

function MetricsSection() {
  // Auditoría H-04: idem Overview; el hook vive en la sección visible.
  const metrics = useAdminMetrics();
  return (
    <>
      <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="font-serif text-3xl font-bold">Métricas</h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">
            Tres endpoints en paralelo (SUP-279), uno por servicio. El Gateway no agrega: el frontend combina.
            {!ADMIN_HAS_BACKEND && ' Hoy se muestran datos de ejemplo hasta crear los endpoints.'}
          </p>
        </div>
        <button
          type="button"
          onClick={metrics.refresh}
          disabled={metrics.isLoading}
          className="inline-flex items-center gap-2 rounded-full bg-primary px-4 py-2 text-xs font-bold text-primary-foreground hover:brightness-110 disabled:opacity-50"
        >
          <RefreshCw className={`size-3.5 ${metrics.isLoading ? 'animate-spin' : ''}`} aria-hidden="true" />
          {metrics.isLoading ? 'Actualizando…' : 'Refrescar ahora'}
        </button>
      </div>
      {metrics.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => <div key={i} className="h-36 animate-pulse rounded-2xl border border-[#dfe5dc] bg-white" />)}
        </div>
      ) : <MetricsCards metrics={metrics} />}
      <div className="mt-6 grid gap-6 xl:grid-cols-2">
        <ControlPanel metrics={metrics} />
        <div className="rounded-2xl border border-[#dfe5dc] bg-white p-6 shadow-sm">
          <p className="text-sm font-bold text-muted-foreground">Contrato de métricas (SUP-279)</p>
          <h3 className="mt-1 text-xl font-bold">Endpoints por servicio</h3>
          <ul className="mt-4 space-y-3 font-mono text-xs">
            <li className="rounded-lg bg-[#fafbf9] p-3">GET /api/v1/admin/usuarios/metricas <span className="text-muted-foreground">→ Identidad</span></li>
            <li className="rounded-lg bg-[#fafbf9] p-3">GET /api/v1/admin/productos/metricas <span className="text-muted-foreground">→ Catálogo</span></li>
            <li className="rounded-lg bg-[#fafbf9] p-3">GET /api/v1/scraper/metricas <span className="text-muted-foreground">→ Control</span></li>
          </ul>
          <p className="mt-4 text-xs text-muted-foreground">JWT con rol admin (SUP-256). Conteos con COUNT indexado &lt; 500ms. Sin agregación en Gateway.</p>
        </div>
      </div>
    </>
  );
}

function resolveSection(pathname: string, search: string): AdminSection {
  if (pathname.endsWith('/metricas')) return 'metrics';
  const raw = new URLSearchParams(search).get('section');
  if (raw === 'metricas' || raw === 'metrics') return 'metrics';
  if (raw === 'users' || raw === 'sources') return raw;
  return 'overview';
}

export default function AdminDashboard() {
  const [params] = useSearchParams();
  const { search, pathname } = useLocation();
  const section = resolveSection(pathname, search || `?${params.toString()}`);

  // SUP-256: el RBAC vive en el router (ProtectedRoute roles=['admin']).
  // Este componente solo monta con rol admin; sin backend muestra mocks.
  const content = section === 'users'
    ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Usuarios</h2><p className="mt-2 text-muted-foreground">Gestiona las cuentas y permisos de la plataforma.</p></div><UsersTable showActions /></>
    : section === 'sources'
      ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Fuentes</h2><p className="mt-2 text-muted-foreground">Visualiza las conexiones que alimentan el catálogo de precios.</p></div><SourcesTable /></>
      : section === 'metrics'
        ? <MetricsSection />
        : <Overview />;

  return (
    <AdminLayout>
      {!ADMIN_HAS_BACKEND && (
        <p className="mb-4 rounded-xl border border-[#dfe5dc] bg-[#fafbf9] px-4 py-2.5 text-xs text-muted-foreground" role="note">
          Datos de ejemplo: el backend de administración (SUP-242/243/279) aún no está conectado.
        </p>
      )}
      {content}
    </AdminLayout>
  );
}
