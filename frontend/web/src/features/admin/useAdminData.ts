// Hooks del panel admin (SUP-255 / SUP-280).
// Sin VITE_API_URL no hay HTTP (retorno temprano a mocks). SUP-242 (usuarios)
// ya tiene backend; SUP-243 (fuentes) y SUP-279 (métricas) se integran cuando
// estén disponibles. 401/403 nunca inyectan mocks (fail-closed, auditoría
// H-03): se cierra la sesión local y se vuelve a /login (SUP-256).

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { ROUTES } from '@/core/routes';
import {
  ADMIN_HAS_BACKEND,
  adminPlanToBackend,
  adminRoleToBackend,
  fetchAdminSources,
  fetchAdminUsers,
  fetchCatalogoMetrics,
  fetchControlMetrics,
  fetchIdentidadMetrics,
  isAdminApiError,
  isAdminAuthError,
  isPersistableId,
  updateAdminSource,
  updateAdminUser,
  type CatalogoMetrics,
  type ControlMetrics,
  type IdentidadMetrics,
} from './admin-api';
import {
  adminMetricsMock,
  adminSources,
  adminUsers,
  type AdminPlan,
  type AdminRole,
  type AdminSource,
  type AdminUser,
} from './admin-mock-data';

export type ServiceState<T> = {
  data: T | null;
  error: string | null;
};

function toMessage(error: unknown): string {
  if (error instanceof DOMException && error.name === 'AbortError') return 'Petición cancelada.';
  return error instanceof Error ? error.message : 'Error de red.';
}

// Mensaje fail-closed para 401/403: sin mocks; la sesión se cierra y se
// vuelve a /login (SUP-256).
function authBlockedMessage(feature: string, err: unknown): string {
  return `Acceso denegado al cargar ${feature} (${toMessage(err)}). La sesión expiró o no tiene permisos; vuelve a iniciar sesión.`;
}

// Expiración de sesión (SUP-256): ante un 401/403 se cierra la sesión local y
// se redirige a /login. logout/navigate se leen desde un ref actualizado tras
// cada render para no recrear los callbacks de carga (y no re-disparar fetch).
function useExpireSession(): () => void {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const sessionRef = useRef({ logout, navigate });

  useEffect(() => {
    sessionRef.current = { logout, navigate };
  });

  return useCallback(() => {
    sessionRef.current.logout();
    sessionRef.current.navigate(ROUTES.LOGIN, { replace: true });
  }, []);
}

// --- Usuarios (SUP-242: listar + activar/desactivar + rol + plan) ---
export type AdminUsersParams = {
  page?: number;
  limit?: number;
  q?: string;
  rol?: AdminRole | 'Todos';
};

export type AdminUsersPagination = {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
};

function userMutationMessage(prefix: string, err: unknown): string {
  if (isAdminApiError(err)) {
    if (err.status === 409) return err.message;
    return `${prefix} ${err.message}${err.detail ? ` (${err.detail})` : ''}`;
  }
  return `${prefix} (${toMessage(err)})`;
}

export function useAdminUsers(params: AdminUsersParams = {}) {
  const page = Math.max(1, params.page ?? 1);
  const limit = params.limit ?? 20;
  const q = (params.q ?? '').trim();
  const rolFiltro = params.rol && params.rol !== 'Todos' ? adminRoleToBackend(params.rol) : '';

  const [users, setUsers] = useState<AdminUser[]>(ADMIN_HAS_BACKEND ? [] : adminUsers);
  const [pagination, setPagination] = useState<AdminUsersPagination>({ page, limit, total: 0, totalPages: 0 });
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const seq = useRef(0);
  const pendingRef = useRef<string | null>(null);
  const expireSession = useExpireSession();

  const load = useCallback(async (signal?: AbortSignal) => {
    // Un refresh durante una mutación podría devolver el estado anterior y
    // pisar la actualización optimista. La mutación invalida cualquier load.
    if (pendingRef.current) return;

    if (!ADMIN_HAS_BACKEND) {
      const texto = q.toLowerCase();
      const filtrados = adminUsers.filter((user) => {
        const coincideTexto = !texto || `${user.name} ${user.email}`.toLowerCase().includes(texto);
        const coincideRol = !rolFiltro
          || (user.role !== 'Desconocido' && adminRoleToBackend(user.role) === rolFiltro);
        return coincideTexto && coincideRol;
      });
      const inicio = (page - 1) * limit;
      const totalPaginas = filtrados.length === 0 ? 0 : Math.ceil(filtrados.length / limit);
      setUsers(filtrados.slice(inicio, inicio + limit));
      setPagination({ page, limit, total: filtrados.length, totalPages: totalPaginas });
      setIsLoading(false);
      setError(null);
      return;
    }

    const current = ++seq.current;
    setIsLoading(true);
    setError(null);
    try {
      const result = await fetchAdminUsers({ page, limit, q, rol: rolFiltro }, signal);
      if (seq.current === current) {
        setUsers(result.users);
        setPagination({ page: result.page, limit: result.limit, total: result.total, totalPages: result.totalPages });
      }
    } catch (err) {
      if (signal?.aborted) return;
      // Auditoría H-03 (fail-closed): 401/403 no inyectan mocks ni mantienen
      // datos previos operativos; se vacía, se cierra sesión y se va a /login.
      if (seq.current === current) {
        if (isAdminAuthError(err)) {
          setUsers([]);
          setError(authBlockedMessage('usuarios', err));
          expireSession();
        } else {
          setUsers([]);
          setPagination({ page: 1, limit, total: 0, totalPages: 0 });
          setError(`No se pudo cargar usuarios del backend. (${toMessage(err)})`);
        }
      }
    } finally {
      if (seq.current === current) setIsLoading(false);
    }
  }, [expireSession, page, limit, q, rolFiltro]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const beginMutation = useCallback((id: string) => {
    // Invalida cualquier GET en vuelo y bloquea refresh hasta terminar.
    seq.current += 1;
    pendingRef.current = id;
    setPendingId(id);
    setIsLoading(false);
  }, []);

  const endMutation = useCallback(() => {
    pendingRef.current = null;
    setPendingId(null);
  }, []);

  const aplicarActualizado = useCallback((id: string, actualizado: AdminUser) => {
    if (rolFiltro && (actualizado.role === 'Desconocido' || adminRoleToBackend(actualizado.role) !== rolFiltro)) {
      setUsers((prev) => prev.filter((user) => user.id !== id));
      setPagination((prev) => {
        const total = Math.max(0, prev.total - 1);
        return { ...prev, total, totalPages: total === 0 ? 0 : Math.ceil(total / prev.limit) };
      });
      return;
    }
    setUsers((prev) => prev.map((user) => (user.id === id ? actualizado : user)));
  }, [rolFiltro]);

  const toggleUserStatus = useCallback(async (id: string) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingRef.current) return;
    const nextActive = target.status !== 'Activo';
    beginMutation(id);
    setUsers((prev) =>
      prev.map((user): AdminUser => (user.id === id ? { ...user, status: nextActive ? 'Activo' : 'Inactivo' } : user)),
    );
    // Con backend sólo se persisten usuarios reales; los mocks de respaldo
    // conservan el cambio en memoria para no enviar un id inválido.
    if (ADMIN_HAS_BACKEND && isPersistableId(id)) {
      try {
        const actualizado = await updateAdminUser(id, { esta_activo: nextActive });
        aplicarActualizado(id, actualizado);
        setError(null);
      } catch (err) {
        setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? target : user)));
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('el usuario', err));
          expireSession();
        } else {
          setError(userMutationMessage('No se pudo actualizar el usuario.', err));
        }
      } finally {
        endMutation();
      }
    } else {
      endMutation();
    }
  }, [users, beginMutation, endMutation, aplicarActualizado, expireSession]);

  const changeUserRole = useCallback(async (id: string, role: AdminRole) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingRef.current || target.role === role) return;
    beginMutation(id);
    setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, role } : user)));
    if (ADMIN_HAS_BACKEND && isPersistableId(id)) {
      try {
        const actualizado = await updateAdminUser(id, { rol: adminRoleToBackend(role) });
        aplicarActualizado(id, actualizado);
        setError(null);
      } catch (err) {
        setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? target : user)));
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('el rol', err));
          expireSession();
        } else {
          setError(userMutationMessage('No se pudo asignar el rol.', err));
        }
      } finally {
        endMutation();
      }
    } else {
      endMutation();
    }
  }, [users, beginMutation, endMutation, aplicarActualizado, expireSession]);

  const changeUserPlan = useCallback(async (id: string, plan: AdminPlan) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingRef.current || target.plan === plan) return;
    beginMutation(id);
    setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, plan } : user)));
    if (ADMIN_HAS_BACKEND && isPersistableId(id)) {
      try {
        const actualizado = await updateAdminUser(id, { plan: adminPlanToBackend(plan) });
        setUsers((prev) => prev.map((user) => (user.id === id ? actualizado : user)));
        setError(null);
      } catch (err) {
        setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? target : user)));
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('el plan', err));
          expireSession();
        } else {
          setError(userMutationMessage('No se pudo asignar el plan.', err));
        }
      } finally {
        endMutation();
      }
    } else {
      endMutation();
    }
  }, [users, beginMutation, endMutation, expireSession]);

  const refresh = useCallback(() => {
    if (!pendingRef.current) void load();
  }, [load]);

  return { users, pagination, isLoading, error, pendingId, refresh, toggleUserStatus, changeUserRole, changeUserPlan };
}

// --- Fuentes (SUP-243: listar + habilitar/deshabilitar) ---
export function useAdminSources() {
  const [sources, setSources] = useState<AdminSource[]>(ADMIN_HAS_BACKEND ? [] : adminSources);
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const seq = useRef(0);
  const expireSession = useExpireSession();

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!ADMIN_HAS_BACKEND) {
      setSources(adminSources);
      setIsLoading(false);
      setError(null);
      return;
    }
    const current = ++seq.current;
    setIsLoading(true);
    setError(null);
    try {
      const data = await fetchAdminSources(signal);
      if (seq.current === current) setSources(data);
    } catch (err) {
      if (signal?.aborted) return;
      if (seq.current === current) {
        // Auditoría H-03 (fail-closed): 401/403 no inyectan mocks; se cierra
        // sesión y se vuelve a /login.
        if (isAdminAuthError(err)) {
          setSources([]);
          setError(authBlockedMessage('fuentes', err));
          expireSession();
        } else {
          setSources([]);
          setError(`No se pudo cargar fuentes del backend. (${toMessage(err)})`);
        }
      }
    } finally {
      if (seq.current === current) setIsLoading(false);
    }
  }, [expireSession]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const toggleSource = useCallback(async (id: string) => {
    const target = sources.find((source) => source.id === id);
    if (!target || pendingId) return;
    const habilitada = !target.habilitada;
    setPendingId(id);
    setSources((prev) =>
      prev.map((source): AdminSource => (
        source.id === id
          ? { ...source, habilitada, status: habilitada ? (source.status === 'Inactiva' ? 'Operativa' : source.status) : 'Inactiva' }
          : source
      )),
    );
    if (ADMIN_HAS_BACKEND) {
      try {
        await updateAdminSource(id, { habilitada });
      } catch (err) {
        setSources((prev) => prev.map((source) => (source.id === id ? target : source)));
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('la fuente', err));
          expireSession();
        } else {
          setError(`No se pudo actualizar la fuente. (${toMessage(err)})`);
        }
      } finally {
        setPendingId(null);
      }
    } else {
      // TODO backend (SUP-243): PUT /api/v1/admin/productos/fuentes/:id { habilitada }.
      setPendingId(null);
    }
  }, [sources, pendingId, expireSession]);

  return { sources, isLoading, error, pendingId, refresh: () => { void load(); }, toggleSource };
}

// --- Métricas por servicio (SUP-279 -> SUP-280) ---
export type AdminMetricsState = {
  identidad: ServiceState<IdentidadMetrics>;
  catalogo: ServiceState<CatalogoMetrics>;
  control: ServiceState<ControlMetrics>;
  isLoading: boolean;
  refresh: () => void;
};

export function useAdminMetrics(): AdminMetricsState {
  const [identidad, setIdentidad] = useState<ServiceState<IdentidadMetrics>>(
    ADMIN_HAS_BACKEND ? { data: null, error: null } : { data: adminMetricsMock.identidad, error: null },
  );
  const [catalogo, setCatalogo] = useState<ServiceState<CatalogoMetrics>>(
    ADMIN_HAS_BACKEND ? { data: null, error: null } : { data: adminMetricsMock.catalogo, error: null },
  );
  const [control, setControl] = useState<ServiceState<ControlMetrics>>(
    ADMIN_HAS_BACKEND ? { data: null, error: null } : { data: adminMetricsMock.control, error: null },
  );
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const seq = useRef(0);
  const expireSession = useExpireSession();

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!ADMIN_HAS_BACKEND) {
      setIdentidad({ data: adminMetricsMock.identidad, error: null });
      setCatalogo({ data: adminMetricsMock.catalogo, error: null });
      setControl({ data: adminMetricsMock.control, error: null });
      setIsLoading(false);
      return;
    }
    const current = ++seq.current;
    setIsLoading(true);
    try {
      // Tres llamadas en paralelo; el fallo de una no rompe las otras (SUP-280).
      // Auditoría H-08: try/finally para no congelar isLoading ante abort/carrera.
      const [identidadRes, catalogoRes, controlRes] = await Promise.allSettled([
        fetchIdentidadMetrics(signal),
        fetchCatalogoMetrics(signal),
        fetchControlMetrics(signal),
      ]);
      if (seq.current !== current || signal?.aborted) return;
      setIdentidad(
        identidadRes.status === 'fulfilled'
          ? { data: identidadRes.value, error: null }
          // H-03: 401/403 sin mocks (fail-closed); resto con tarjeta "sin datos".
          : { data: null, error: isAdminAuthError(identidadRes.reason) ? authBlockedMessage('métricas de identidad', identidadRes.reason) : toMessage(identidadRes.reason) },
      );
      setCatalogo(
        catalogoRes.status === 'fulfilled'
          ? { data: catalogoRes.value, error: null }
          : { data: null, error: isAdminAuthError(catalogoRes.reason) ? authBlockedMessage('métricas de catálogo', catalogoRes.reason) : toMessage(catalogoRes.reason) },
      );
      setControl(
        controlRes.status === 'fulfilled'
          ? { data: controlRes.value, error: null }
          : { data: null, error: isAdminAuthError(controlRes.reason) ? authBlockedMessage('métricas de control', controlRes.reason) : toMessage(controlRes.reason) },
      );
      // SUP-256: si alguna llamada respondió 401/403, se cierra la sesión y se
      // vuelve a /login (fail-closed) en vez de quedar en estado degradado.
      const authExpired = [identidadRes, catalogoRes, controlRes].some(
        (result) => result.status === 'rejected' && isAdminAuthError(result.reason),
      );
      if (authExpired) expireSession();
    } finally {
      if (seq.current === current) setIsLoading(false);
    }
  }, [expireSession]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  return { identidad, catalogo, control, isLoading, refresh: () => { void load(); } };
}
