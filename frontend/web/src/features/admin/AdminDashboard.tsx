// Panel administrativo con usuarios, fuentes y métricas reales o errores explícitos por servicio.
import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  Database,
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
import { ROUTES } from '@/app/routes';
import { useAuth } from '@/features/autenticacion/useAuth';
import { AdminLayout } from './AdminLayout';
import { ADMIN_HAS_BACKEND } from './admin-api';
import { ADMIN_PLANS, ADMIN_ROLES, type AdminSection } from './admin-types';
import { useAdminMetrics, useAdminSources, useAdminUsers, type AdminMetricsState } from './useAdminData';

const estilosEstado = {
  Activo: 'bg-[#e3f1e4] text-[#376344]',
  Inactivo: 'bg-[#eef0ed] text-[#667061]',
  Operativa: 'bg-[#e3f1e4] text-[#376344]',
  Advertencia: 'bg-[#fff0d8] text-[#9a641c]',
  Inactiva: 'bg-[#eef0ed] text-[#667061]',
};

function EtiquetaEstado({ estado }: { estado: keyof typeof estilosEstado }) {
  return <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-bold ${estilosEstado[estado]}`}>{estado}</span>;
}

function TarjetaEstadistica({ icono: Icono, etiqueta, valor, detalle, acento, error }: {
  icono: typeof Users;
  etiqueta: string;
  valor: string;
  detalle: string;
  acento: string;
  error?: string | null;
}) {
  return (
    <article className="rounded-2xl border border-[#dfe5dc] bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <span className={`flex size-11 items-center justify-center rounded-xl ${acento}`}><Icono className="size-5" /></span>
        <ArrowUpRight className="size-4 text-muted-foreground" />
      </div>
      <p className="mt-5 text-sm font-medium text-muted-foreground">{etiqueta}</p>
      {error ? (
        <>
          <p className="mt-1 text-2xl font-bold tracking-tight text-muted-foreground">Sin datos</p>
          <p className="mt-2 text-xs text-destructive" title={error}>Servicio no disponible</p>
        </>
      ) : (
        <>
          <p className="mt-1 text-3xl font-bold tracking-tight">{valor}</p>
          <p className="mt-2 text-xs text-muted-foreground">{detalle}</p>
        </>
      )}
    </article>
  );
}

function EsqueletoTabla({ filas = 4, columnas = 4 }: { filas?: number; columnas?: number }) {
  return (
    <div className="space-y-3 p-5" role="status" aria-label="Cargando datos">
      {Array.from({ length: filas }).map((_, fila) => (
        <div key={fila} className="flex gap-3">
          {Array.from({ length: columnas }).map((_, columna) => (
            <div key={columna} className="h-9 flex-1 animate-pulse rounded-lg bg-[#edf0eb]" />
          ))}
        </div>
      ))}
    </div>
  );
}

function AvisoError({ mensaje, alReintentar }: { mensaje: string; alReintentar: () => void }) {
  return (
    <div className="flex flex-col gap-2 border-b border-[#edf0eb] bg-[#fdf3f1] p-4 text-sm sm:flex-row sm:items-center sm:justify-between" role="alert">
      <p className="flex items-center gap-2 text-destructive">
        <TriangleAlert className="size-4 shrink-0" aria-hidden="true" />
        <span>{mensaje}</span>
      </p>
      <button
        type="button"
        onClick={alReintentar}
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
// sección Usuarios se usa con mostrarAcciones para gestionar (SUP-242).
function TablaUsuarios({ mostrarAcciones = false }: { mostrarAcciones?: boolean }) {
  const [consulta, setConsulta] = useState('');
  const [consultaDiferida, setConsultaDiferida] = useState('');
  const [rolSeleccionado, setRolSeleccionado] = useState('Todos');
  const [pagina, setPagina] = useState(1);
  const limite = 20;
  const { user: usuarioActual } = useAuth();
  const {
    users: usuarios,
    pagination: paginacion,
    isLoading: cargando,
    error,
    pendingId: idPendiente,
    refresh: recargar,
    toggleUserStatus: alternarEstadoUsuario,
    changeUserRole: cambiarRolUsuario,
    changeUserPlan: cambiarPlanUsuario,
  } = useAdminUsers({
    page: pagina,
    limit: limite,
    q: consultaDiferida,
    rol: rolSeleccionado as 'Todos' | (typeof ADMIN_ROLES)[number],
  });

  useEffect(() => {
    const temporizador = window.setTimeout(() => setConsultaDiferida(consulta), 300);
    return () => window.clearTimeout(temporizador);
  }, [consulta]);

  useEffect(() => {
    setPagina(1);
  }, [consultaDiferida, rolSeleccionado]);

  useEffect(() => {
    if (cargando) return;
    if (paginacion.totalPages === 0 && pagina !== 1) {
      setPagina(1);
    } else if (paginacion.totalPages > 0 && pagina > paginacion.totalPages) {
      setPagina(paginacion.totalPages);
    }
  }, [cargando, pagina, paginacion.totalPages]);

  const inicio = paginacion.total === 0 ? 0 : (paginacion.page - 1) * paginacion.limit + 1;
  const fin = Math.min(paginacion.page * paginacion.limit, paginacion.total);
  // El hook serializa las mutaciones (una a la vez). Bloquear toda la tabla
  // mientras hay una en vuelo evita que el operador crea que aplicó un cambio
  // que el hook descartaría en silencio (SUP-242).
  const hayMutacionPendiente = idPendiente !== null;

  return (
    <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between">
        <div><h2 className="text-lg font-bold">Usuarios recientes</h2><p className="text-sm text-muted-foreground">Administra las cuentas y sus permisos.</p></div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={consulta} onChange={(evento) => setConsulta(evento.target.value)} placeholder="Buscar usuario..." className="pl-9 shadow-none" /></div>
          <select value={rolSeleccionado} onChange={(evento) => setRolSeleccionado(evento.target.value)} aria-label="Filtrar por rol" className="h-10 rounded-md border-2 border-border bg-background px-3 text-sm font-medium shadow-none outline-none focus:ring-2 focus:ring-ring">
            <option>Todos</option>{ADMIN_ROLES.map((rol) => <option key={rol}>{rol}</option>)}
          </select>
        </div>
      </div>
      {error && <AvisoError mensaje={error} alReintentar={recargar} />}
      {cargando ? <EsqueletoTabla filas={4} columnas={mostrarAcciones ? 6 : 5} /> : (
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
                  {mostrarAcciones && <th className="px-5 py-3 font-bold">Acción</th>}
                </tr>
              </thead>
              <tbody className="divide-y divide-[#edf0eb]">
                {usuarios.map((usuario) => {
                  const estaActivo = usuario.status === 'Activo';
                  const ocupado = idPendiente === usuario.id;
                  const esPropio = usuarioActual?.id === usuario.id;
                  const rolEditable = (ADMIN_ROLES as readonly string[]).includes(usuario.role);
                  return (
                    <tr key={usuario.id} className="hover:bg-[#fcfdfb]">
                      <td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-full bg-[#ebe5db] text-xs font-bold text-[#4a5a3f]">{usuario.initials}</span><div><p className="font-bold">{usuario.name}</p><p className="text-xs text-muted-foreground">{usuario.email}</p></div></div></td>
                      <td className="px-5 py-4 text-muted-foreground">
                        {mostrarAcciones ? (
                          <select
                            value={rolEditable ? usuario.role : ''}
                            disabled={hayMutacionPendiente || !rolEditable || esPropio}
                            onChange={(evento) => void cambiarRolUsuario(usuario.id, evento.target.value as (typeof ADMIN_ROLES)[number])}
                            aria-label={`Asignar rol a ${usuario.name}`}
                            title={esPropio
                              ? 'No puedes cambiar tu propio rol; debe hacerlo otro administrador'
                              : rolEditable
                                ? 'Asignar rol RBAC (SUP-242: PUT rol: admin | colaborador | registrado)'
                                : 'El backend devolvió un rol no reconocido; no se puede reasignar desde el panel'}
                            className="h-9 rounded-md border-2 border-border bg-background px-2 text-xs font-bold outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
                          >
                            {!rolEditable && <option value="">Desconocido</option>}
                            {ADMIN_ROLES.map((rol) => <option key={rol}>{rol}</option>)}
                          </select>
                        ) : usuario.role}
                      </td>
                      <td className="px-5 py-4 text-muted-foreground">
                        {mostrarAcciones ? (
                          <select
                            value={usuario.plan ?? ''}
                            disabled={hayMutacionPendiente}
                            onChange={(evento) => void cambiarPlanUsuario(usuario.id, evento.target.value as (typeof ADMIN_PLANS)[number])}
                            aria-label={`Asignar plan a ${usuario.name}`}
                            title="Asignar plan comercial (campo aparte, nunca en `rol`)"
                            className="h-9 rounded-md border-2 border-border bg-background px-2 text-xs font-bold outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
                          >
                            {!usuario.plan && <option value="">—</option>}
                            {ADMIN_PLANS.map((plan) => <option key={plan}>{plan}</option>)}
                          </select>
                        ) : (
                          <span>{usuario.plan ?? '—'}</span>
                        )}
                      </td>
                      <td className="px-5 py-4"><EtiquetaEstado estado={usuario.status} /></td>
                      <td className="px-5 py-4 text-muted-foreground">{usuario.lastAccess}</td>
                      {mostrarAcciones && (
                        <td className="px-5 py-4">
                          <button
                            type="button"
                            onClick={() => void alternarEstadoUsuario(usuario.id)}
                            disabled={hayMutacionPendiente || esPropio}
                            aria-label={`${estaActivo ? 'Desactivar' : 'Activar'} a ${usuario.name}`}
                            title={esPropio
                              ? 'No puedes desactivar tu propia cuenta; debe hacerlo otro administrador'
                              : estaActivo ? 'Desactivar usuario (SUP-242: esta_activo=false)' : 'Activar usuario (SUP-242: esta_activo=true)'}
                            className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold transition-colors disabled:opacity-50 ${estaActivo ? 'border-destructive/30 text-destructive hover:bg-destructive/10' : 'border-primary/30 text-primary hover:bg-primary/10'}`}
                          >
                            <Power className="size-3.5" aria-hidden="true" />{ocupado ? 'Guardando…' : estaActivo ? 'Desactivar' : 'Activar'}
                          </button>
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {!error && usuarios.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos usuarios con esos filtros.</p>}
          {paginacion.total > 0 && (
            <div className="flex flex-col gap-3 border-t border-[#edf0eb] px-5 py-3 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
              <p>Mostrando {inicio}–{fin} de {paginacion.total} usuarios</p>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setPagina((actual) => Math.max(1, actual - 1))}
                  disabled={pagina <= 1 || Boolean(idPendiente)}
                  className="inline-flex items-center gap-1 rounded-full border px-3 py-1.5 font-bold disabled:opacity-40"
                >
                  <ChevronLeft className="size-3.5" aria-hidden="true" /> Anterior
                </button>
                <span className="font-bold">Página {paginacion.page} de {Math.max(1, paginacion.totalPages)}</span>
                <button
                  type="button"
                  onClick={() => setPagina((actual) => actual + 1)}
                  disabled={pagina >= Math.max(1, paginacion.totalPages) || Boolean(idPendiente)}
                  className="inline-flex items-center gap-1 rounded-full border px-3 py-1.5 font-bold disabled:opacity-40"
                >
                  Siguiente <ChevronRight className="size-3.5" aria-hidden="true" />
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </section>
  );
}

// SUP-255: listado de fuentes con habilitar/deshabilitar (SUP-243).
function TablaFuentes() {
  const [consulta, setConsulta] = useState('');
  const {
    sources: fuentes,
    isLoading: cargando,
    error,
    pendingId: idPendiente,
    refresh: recargar,
    toggleSource: alternarFuente,
  } = useAdminSources();
  const fuentesFiltradas = useMemo(
    () => fuentes.filter((fuente) => fuente.name.toLowerCase().includes(consulta.toLowerCase())),
    [fuentes, consulta],
  );
  // Igual que en usuarios: el hook serializa y descarta mutaciones concurrentes.
  const hayMutacionPendiente = idPendiente !== null;

  return (
    <section className="rounded-2xl border border-[#dfe5dc] bg-white shadow-sm">
      <div className="flex flex-col gap-4 border-b border-[#edf0eb] p-5 md:flex-row md:items-center md:justify-between"><div><h2 className="text-lg font-bold">Fuentes conectadas</h2><p className="text-sm text-muted-foreground">Supervisa el estado de las fuentes de precios.</p></div><div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={consulta} onChange={(evento) => setConsulta(evento.target.value)} placeholder="Buscar fuente..." className="pl-9 shadow-none" /></div></div>
      {error && <AvisoError mensaje={error} alReintentar={recargar} />}
      {cargando ? <EsqueletoTabla filas={4} columnas={5} /> : (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[820px] text-left text-sm">
              <thead className="bg-[#fafbf9] text-xs uppercase tracking-wider text-muted-foreground"><tr><th className="px-5 py-3 font-bold">Fuente</th><th className="px-5 py-3 font-bold">Estado</th><th className="px-5 py-3 font-bold">Última sincronización</th><th className="px-5 py-3 font-bold">Productos</th><th className="px-5 py-3 font-bold">Sucursales</th><th className="px-5 py-3 font-bold">Habilitada</th></tr></thead>
              <tbody className="divide-y divide-[#edf0eb]">
                {fuentesFiltradas.map((fuente) => {
                  const ocupado = idPendiente === fuente.id;
                  return (
                    <tr key={fuente.id} className="hover:bg-[#fcfdfb]">
                      <td className="px-5 py-4"><div className="flex items-center gap-3"><span className="flex size-9 items-center justify-center rounded-lg bg-[#ebe5db] text-sm font-bold text-[#4a5a3f]">{fuente.name.charAt(0)}</span><div><p className="font-bold">{fuente.name}</p><p className="text-xs text-muted-foreground">{fuente.type}</p></div></div></td>
                      <td className="px-5 py-4"><EtiquetaEstado estado={fuente.status} /></td>
                      <td className="px-5 py-4 text-muted-foreground">{fuente.lastSync}</td>
                      <td className="px-5 py-4 font-mono text-xs">{fuente.products.toLocaleString('es-CL')}</td>
                      <td className="px-5 py-4 text-muted-foreground">{fuente.branches}</td>
                      <td className="px-5 py-4">
                        <button
                          type="button"
                          onClick={() => void alternarFuente(fuente.id)}
                          disabled={hayMutacionPendiente}
                          aria-label={`${fuente.habilitada ? 'Deshabilitar' : 'Habilitar'} fuente ${fuente.name}`}
                          title={fuente.habilitada ? 'Deshabilitar fuente (SUP-243)' : 'Habilitar fuente (SUP-243)'}
                          className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold transition-colors disabled:opacity-50 ${fuente.habilitada ? 'border-destructive/30 text-destructive hover:bg-destructive/10' : 'border-primary/30 text-primary hover:bg-primary/10'}`}
                        >
                          <Power className="size-3.5" aria-hidden="true" />{ocupado ? 'Guardando…' : fuente.habilitada ? 'Deshabilitar' : 'Habilitar'}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {!error && fuentesFiltradas.length === 0 && <p className="p-8 text-center text-sm text-muted-foreground">No encontramos fuentes con esa búsqueda.</p>}
        </>
      )}
    </section>
  );
}

const formatearNumero = (numero: number) => numero.toLocaleString('es-CL');

// SUP-280: tarjetas de conteo combinando los tres servicios en paralelo.
// Si un servicio falla, su tarjeta muestra "Sin datos" sin romper la vista.
function TarjetasMetricas({ metricas }: { metricas: AdminMetricsState }) {
  const { identidad, catalogo, control } = metricas;
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <TarjetaEstadistica
        icono={Users}
        etiqueta="Usuarios registrados"
        valor={identidad.data ? formatearNumero(identidad.data.totalUsuarios) : '—'}
        detalle={identidad.data ? 'Identidad (SUP-279)' : 'Identidad sin datos'}
        acento="bg-[#e3f1e4] text-[#376344]"
        error={identidad.error}
      />
      <TarjetaEstadistica
        icono={Activity}
        etiqueta="Usuarios activos"
        valor={identidad.data ? formatearNumero(identidad.data.usuariosActivos) : '—'}
        detalle={identidad.data ? `${((identidad.data.usuariosActivos / Math.max(identidad.data.totalUsuarios, 1)) * 100).toFixed(1).replace('.', ',')}% del total` : 'Identidad sin datos'}
        acento="bg-[#e8f0f3] text-[#416978]"
        error={identidad.error}
      />
      <TarjetaEstadistica
        icono={Database}
        etiqueta="Fuentes activas"
        valor={catalogo.data ? String(catalogo.data.cadenas) : '—'}
        detalle={catalogo.data ? `${catalogo.data.sucursales} sucursales · Catálogo` : 'Catálogo sin datos'}
        acento="bg-[#fff0d8] text-[#9a641c]"
        error={catalogo.error}
      />
      <TarjetaEstadistica
        icono={Server}
        etiqueta="Productos indexados"
        valor={catalogo.data ? formatearNumero(catalogo.data.productos) : '—'}
        detalle={catalogo.data ? `${formatearNumero(catalogo.data.capturas)} capturas` : 'Catálogo sin datos'}
        acento="bg-[#ebe3ea] text-[#765b72]"
        error={catalogo.error}
      />
      {(identidad.error || catalogo.error || control.error) && (
        <p className="text-xs text-muted-foreground sm:col-span-2 xl:col-span-4">
          Fallo parcial tolerado: {control.error ? 'Control' : identidad.error ? 'Identidad' : 'Catálogo'} no disponible.
          <button type="button" onClick={metricas.refresh} className="ml-2 font-bold text-primary hover:underline">Reintentar</button>
        </p>
      )}
    </div>
  );
}

// SUP-280: barras y tabla sin librería pesada, con tokens --chart-*.
function PanelControl({ metricas }: { metricas: AdminMetricsState }) {
  const { control } = metricas;
  const entradas = Object.entries(control.data?.trabajosPorEstado ?? {});
  const maximo = Math.max(1, ...entradas.map(([, valor]) => valor));
  const barrasGrafico = ['bg-[var(--chart-1)]', 'bg-[var(--chart-2)]', 'bg-[var(--chart-3)]', 'bg-[var(--chart-4)]', 'bg-[var(--chart-5)]'];

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
        {entradas.length === 0 && !control.error && <p className="text-sm text-white/60">Sin trabajos registrados.</p>}
        {entradas.map(([estado, total], indice) => (
          <div key={estado}>
            <div className="mb-1 flex justify-between text-xs">
              <span className="capitalize text-white/70">{estado.replace('_', ' ')}</span>
              <span className="font-bold">{formatearNumero(total)}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-white/10">
              <div className={`h-full rounded-full ${barrasGrafico[indice % barrasGrafico.length]}`} style={{ width: `${(total / maximo) * 100}%` }} />
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

function ResumenGeneral() {
  // Auditoría H-04: las métricas solo se piden al montar esta vista,
  // no en la raíz del dashboard (evita 3 llamadas en /admin?section=users).
  const metricas = useAdminMetrics();
  return (
    <>
      <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="mb-2 text-sm font-bold text-primary">{new Date().toLocaleDateString('es-CL', { dateStyle: 'full' })}</p>
          <h2 className="font-serif text-3xl font-bold tracking-tight md:text-4xl">Hola, equipo de administración 👋</h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">Este es el estado general de tu plataforma. Revisa usuarios y fuentes desde un mismo lugar.</p>
        </div>
        <button
          type="button"
          onClick={metricas.refresh}
          disabled={metricas.isLoading}
          className="inline-flex items-center gap-2 rounded-full border border-border bg-white px-4 py-2 text-xs font-bold text-muted-foreground hover:bg-secondary disabled:opacity-50"
        >
          <RefreshCw className={`size-3.5 ${metricas.isLoading ? 'animate-spin' : ''}`} aria-hidden="true" />
          {metricas.isLoading ? 'Actualizando…' : 'Refrescar métricas'}
        </button>
      </div>
      {metricas.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, indice) => <div key={indice} className="h-36 animate-pulse rounded-2xl border border-[#dfe5dc] bg-white" />)}
        </div>
      ) : <TarjetasMetricas metricas={metricas} />}
      <div className="mt-6 grid gap-6 xl:grid-cols-[1.3fr_0.7fr]">
        {/* Vista general: solo lectura, sin columna de acción (va en sección Usuarios). */}
        <TablaUsuarios />
        <PanelControl metricas={metricas} />
      </div>
      <p className="mt-6 text-sm text-muted-foreground">
        ¿Necesitas el detalle por servicio? <Link to={`${ROUTES.ADMIN}?section=metrics`} className="font-bold text-primary hover:underline">Ver métricas</Link>
      </p>
    </>
  );
}

function SeccionMetricas() {
  // Auditoría H-04: idem ResumenGeneral; el hook vive en la sección visible.
  const metricas = useAdminMetrics();
  return (
    <>
      <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="font-serif text-3xl font-bold">Métricas</h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">
            Tres endpoints en paralelo (SUP-279), uno por servicio. El Gateway no agrega: el frontend combina.
            {!ADMIN_HAS_BACKEND && ' Configura VITE_API_URL para consultar los servicios.'}
          </p>
        </div>
        <button
          type="button"
          onClick={metricas.refresh}
          disabled={metricas.isLoading}
          className="inline-flex items-center gap-2 rounded-full bg-primary px-4 py-2 text-xs font-bold text-primary-foreground hover:brightness-110 disabled:opacity-50"
        >
          <RefreshCw className={`size-3.5 ${metricas.isLoading ? 'animate-spin' : ''}`} aria-hidden="true" />
          {metricas.isLoading ? 'Actualizando…' : 'Refrescar ahora'}
        </button>
      </div>
      {metricas.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, indice) => <div key={indice} className="h-36 animate-pulse rounded-2xl border border-[#dfe5dc] bg-white" />)}
        </div>
      ) : <TarjetasMetricas metricas={metricas} />}
      <div className="mt-6 grid gap-6 xl:grid-cols-2">
        <PanelControl metricas={metricas} />
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

/** Resuelve los alias de ruta y consulta sin aceptar secciones desconocidas. */
function resolverSeccion(rutaActual: string, busquedaUrl: string): AdminSection {
  if (rutaActual.endsWith('/metricas')) return 'metrics';
  const seccionCruda = new URLSearchParams(busquedaUrl).get('section');
  if (seccionCruda === 'metricas' || seccionCruda === 'metrics') return 'metrics';
  if (seccionCruda === 'users' || seccionCruda === 'sources') return seccionCruda;
  return 'overview';
}

export default function AdminDashboard() {
  const [parametrosBusqueda] = useSearchParams();
  const { search: busquedaUrl, pathname: rutaActual } = useLocation();
  const seccion = resolverSeccion(rutaActual, busquedaUrl || `?${parametrosBusqueda.toString()}`);

  // SUP-256: el RBAC vive en el router (ProtectedRoute roles=['admin']).
  // Este componente solo monta con rol admin; los errores no se sustituyen por datos de ejemplo.
  const contenido = seccion === 'users'
    ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Usuarios</h2><p className="mt-2 text-muted-foreground">Gestiona las cuentas y permisos de la plataforma.</p></div><TablaUsuarios mostrarAcciones /></>
    : seccion === 'sources'
      ? <><div className="mb-7"><h2 className="font-serif text-3xl font-bold">Fuentes</h2><p className="mt-2 text-muted-foreground">Visualiza las conexiones que alimentan el catálogo de precios.</p></div><TablaFuentes /></>
      : seccion === 'metrics'
        ? <SeccionMetricas />
        : <ResumenGeneral />;

  return (
    <AdminLayout>
      {!ADMIN_HAS_BACKEND && (
        <p className="mb-4 rounded-xl border border-[#dfe5dc] bg-[#fafbf9] px-4 py-2.5 text-xs text-muted-foreground" role="note">
          Administración no disponible: falta configurar VITE_API_URL. No se muestran datos de ejemplo.
        </p>
      )}
      {contenido}
    </AdminLayout>
  );
}
