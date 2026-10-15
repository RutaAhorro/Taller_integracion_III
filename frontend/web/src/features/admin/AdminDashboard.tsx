import { useMemo, useState } from 'react';
import { Activity, ArrowUpRight, Database, MoreHorizontal, Search, Server, Users, Wifi } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { Input } from '@/components/ui/input';
import { AdminLayout } from './AdminLayout';
import { adminSources, adminUsers, type AdminSection } from './admin-mock-data';

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

function StatCard({ icon: Icon, label, value, detail, accent }: { icon: typeof Users; label: string; value: string; detail: string; accent: string }) {
  return (
    <article className="rounded-2xl border border-[#dfe5dc] bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <span className={`flex size-11 items-center justify-center rounded-xl ${accent}`}><Icon className="size-5" /></span>
        <ArrowUpRight className="size-4 text-muted-foreground" />
      </div>
      <p className="mt-5 text-sm font-medium text-muted-foreground">{label}</p>
      <p className="mt-1 text-3xl font-bold tracking-tight">{value}</p>
      <p className="mt-2 text-xs text-muted-foreground">{detail}</p>
    </article>
  );
}

function UsersTable() {
  const [query, setQuery] = useState('');
  const [role, setRole] = useState('Todos');
  const filteredUsers = useMemo(() => adminUsers.filter((user) => {
    const matchesQuery = `${user.name} ${user.email}`.toLowerCase().includes(query.toLowerCase());
    return matchesQuery && (role === 'Todos' || user.role === role);
  }), [query, role]);

  return (
    <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between">
        <div><h2 className="text-lg font-bold">Usuarios recientes</h2><p className="text-sm text-muted-foreground">Administra las cuentas y sus permisos.</p></div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar usuario..." className="pl-9 shadow-none" /></div>
          <select value={role} onChange={(event) => setRole(event.target.value)} className="h-10 rounded-md border-2 border-border bg-background px-3 text-sm font-medium shadow-none outline-none focus:ring-2 focus:ring-ring">
            <option>Todos</option><option>Administrador</option><option>Editor</option><option>Usuario</option>
          </select>
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[680px] text-left text-sm">
          <thead className="bg-[#fafbf9] text-xs uppercase tracking-wider text-muted-foreground"><tr><th className="px-5 py-3 font-bold">Usuario</th><th className="px-5 py-3 font-bold">Rol</th><th className="px-5 py-3 font-bold">Estado</th><th className="px-5 py-3 font-bold">Último acceso</th><th className="px-5 py-3" /></tr></thead>
          <tbody className="divide-y divide-[#edf0eb]">{filteredUsers.map((user) => <tr key={user.id} className="hover:bg-[#fcfdfb]">
            <td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-full bg-[#ebe5db] text-xs font-bold text-[#4a5a3f]">{user.initials}</span><div><p className="font-bold">{user.name}</p><p className="text-xs text-muted-foreground">{user.email}</p></div></div></td>
            <td className="px-5 py-4 text-muted-foreground">{user.role}</td><td className="px-5 py-4"><StatusBadge status={user.status} /></td><td className="px-5 py-4 text-muted-foreground">{user.lastAccess}</td><td className="px-5 py-4 text-right"><button type="button" aria-label={`Más acciones para ${user.name}`} className="rounded-lg p-2 text-muted-foreground hover:bg-secondary hover:text-foreground"><MoreHorizontal className="size-5" /></button></td>
          </tr>)}</tbody>
        </table>
      </div>
      {filteredUsers.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos usuarios con esos filtros.</p>}
    </section>
  );
}

function SourcesTable() {
  const [query, setQuery] = useState('');
  const filteredSources = adminSources.filter((source) => source.name.toLowerCase().includes(query.toLowerCase()));
  return <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
    <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between"><div><h2 className="text-lg font-bold">Fuentes conectadas</h2><p className="text-sm text-muted-foreground">Supervisa el estado de las fuentes de precios.</p></div><div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar fuente..." className="pl-9 shadow-none" /></div></div>
    <div className="overflow-x-auto"><table className="w-full min-w-[760px] text-left text-sm"><thead className="bg-[#fafbf9] text-xs uppercase tracking-wider text-muted-foreground"><tr><th className="px-5 py-3 font-bold">Fuente</th><th className="px-5 py-3 font-bold">Estado</th><th className="px-5 py-3 font-bold">Última sincronización</th><th className="px-5 py-3 font-bold">Productos</th><th className="px-5 py-3 font-bold">Sucursales</th><th className="px-5 py-3" /></tr></thead><tbody className="divide-y divide-[#edf0eb]">{filteredSources.map((source) => <tr key={source.id} className="hover:bg-[#fcfdfb]"><td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-lg bg-[#ebe5db] text-sm font-bold text-[#4a5a3f]">{source.name.charAt(0)}</span><div><p className="font-bold">{source.name}</p><p className="text-xs text-muted-foreground">{source.type}</p></div></div></td><td className="px-5 py-4"><StatusBadge status={source.status} /></td><td className="px-5 py-4 text-muted-foreground">{source.lastSync}</td><td className="px-5 py-4 font-mono text-xs">{source.products.toLocaleString('es-CL')}</td><td className="px-5 py-4 text-muted-foreground">{source.branches || '—'}</td><td className="px-5 py-4 text-right"><button type="button" aria-label={`Más acciones para ${source.name}`} className="rounded-lg p-2 text-muted-foreground hover:bg-secondary hover:text-foreground"><MoreHorizontal className="size-5" /></button></td></tr>)}</tbody></table></div>
    {filteredSources.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos fuentes con esa búsqueda.</p>}
  </section>;
}

function Overview() {
  return <><div className="mb-7"><p className="mb-2 text-sm font-bold text-primary">Miércoles, 15 de octubre de 2026</p><h2 className="font-serif text-3xl font-bold tracking-tight md:text-4xl">Hola, administradora 👋</h2><p className="mt-2 max-w-2xl text-muted-foreground">Este es el estado general de tu plataforma. Revisa usuarios y fuentes desde un mismo lugar.</p></div><div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><StatCard icon={Users} label="Usuarios registrados" value="1.284" detail="+12,4% este mes" accent="bg-[#e3f1e4] text-[#376344]" /><StatCard icon={Activity} label="Usuarios activos" value="1.126" detail="87,7% del total" accent="bg-[#e8f0f3] text-[#416978]" /><StatCard icon={Database} label="Fuentes activas" value="8" detail="2 sincronizando ahora" accent="bg-[#fff0d8] text-[#9a641c]" /><StatCard icon={Server} label="Productos indexados" value="41.520" detail="Última actualización: hoy" accent="bg-[#ebe3ea] text-[#765b72]" /></div><div className="mt-6 grid gap-6 xl:grid-cols-[1.3fr_0.7fr]"><UsersTable /><div className="rounded-2xl border border-[#dfe5dc] bg-[#263524] p-6 text-white shadow-sm"><div className="flex items-start justify-between"><div><p className="text-sm font-bold text-white/60">Salud de las fuentes</p><h3 className="mt-2 text-3xl font-bold">96,8%</h3></div><Wifi className="size-5 text-[#d4a373]" /></div><div className="mt-7 h-2 overflow-hidden rounded-full bg-white/10"><div className="h-full w-[96.8%] rounded-full bg-[#d4a373]" /></div><div className="mt-5 space-y-3 text-sm"><div className="flex justify-between"><span className="text-white/65">Operativas</span><span className="font-bold">6 fuentes</span></div><div className="flex justify-between"><span className="text-white/65">Con advertencias</span><span className="font-bold text-[#f2c77f]">2 fuentes</span></div></div></div></div></>;
}

export default function AdminDashboard() {
  const [params] = useSearchParams();
  const section = (params.get('section') as AdminSection) || 'overview';
  const content = section === 'users' ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Usuarios</h2><p className="mt-2 text-muted-foreground">Gestiona las cuentas y permisos de la plataforma.</p></div><UsersTable /></> : section === 'sources' ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Fuentes</h2><p className="mt-2 text-muted-foreground">Visualiza las conexiones que alimentan el catálogo de precios.</p></div><SourcesTable /></> : <Overview />;
  return <AdminLayout>{content}</AdminLayout>;
}
