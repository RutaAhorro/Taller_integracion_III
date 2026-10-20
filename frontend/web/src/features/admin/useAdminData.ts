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
  adminRoleToBackend,
  fetchAdminSources,
  fetchAdminUsers,
  fetchCatalogoMetrics,
  fetchControlMetrics,
  fetchIdentidadMetrics,
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

// --- Usuarios (SUP-242: listar + activar/desactivar + rol colaborador) ---
export function useAdminUsers() {
  const [users, setUsers] = useState<AdminUser[]>(adminUsers);
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const seq = useRef(0);
  const expireSession = useExpireSession();

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!ADMIN_HAS_BACKEND) {
      setUsers(adminUsers);
      setIsLoading(false);
      setError(null);
      return;
    }
    const current = ++seq.current;
    setIsLoading(true);
    setError(null);
    try {
      const data = await fetchAdminUsers(signal);
      if (seq.current === current) setUsers(data);
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
          setUsers(adminUsers);
          setError(`No se pudo cargar usuarios del backend. Mostrando datos de ejemplo. (${toMessage(err)})`);
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

  const toggleUserStatus = useCallback(async (id: string) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingId) return;
    const nextActive = target.status !== 'Activo';
    setPendingId(id);
    setUsers((prev) =>
      prev.map((user): AdminUser => (user.id === id ? { ...user, status: nextActive ? 'Activo' : 'Inactivo' } : user)),
    );
    // Con backend sólo se persisten usuarios reales; los mocks de respaldo
    // conservan el cambio en memoria para no enviar un id inválido.
    if (ADMIN_HAS_BACKEND && isPersistableId(id)) {
      try {
        await updateAdminUser(id, { esta_activo: nextActive });
      } catch (err) {
        // Revierte el optimista y muestra el error.
        setUsers((prev) =>
          prev.map((user): AdminUser => (user.id === id ? { ...user, status: target.status } : user)),
        );
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('el usuario', err));
          expireSession();
        } else {
          setError(`No se pudo actualizar el usuario. (${toMessage(err)})`);
        }
      } finally {
        setPendingId(null);
      }
    } else {
      // Sin backend el cambio queda solo en el estado local.
      setPendingId(null);
    }
  }, [users, pendingId, expireSession]);

  const changeUserRole = useCallback(async (id: string, role: AdminRole) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingId || target.role === role) return;
    const previous = target.role;
    setPendingId(id);
    setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, role } : user)));
    if (ADMIN_HAS_BACKEND && isPersistableId(id)) {
      try {
        await updateAdminUser(id, { rol: adminRoleToBackend(role) });
      } catch (err) {
        setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, role: previous } : user)));
        if (isAdminAuthError(err)) {
          setError(authBlockedMessage('el rol', err));
          expireSession();
        } else {
          setError(`No se pudo asignar el rol. (${toMessage(err)})`);
        }
      } finally {
        setPendingId(null);
      }
    } else {
      // Sin backend el cambio queda solo en el estado local.
      setPendingId(null);
    }
  }, [users, pendingId, expireSession]);

  // Plan comercial separado del rol (auditoría H-06). api.usuarios todavía no
  // expone el campo, así que con backend configurado el cambio no se envía:
  // enviarlo devolvía 400. En modo mock el cambio queda en memoria.
  const changeUserPlan = useCallback((id: string, plan: AdminPlan) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingId || target.plan === plan) return;
    if (ADMIN_HAS_BACKEND) {
      setError('El plan comercial todavía no está disponible en el backend.');
      return;
    }
    // Sin backend el cambio queda solo en el estado local.
    setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, plan } : user)));
  }, [users, pendingId]);

  return { users, isLoading, error, pendingId, refresh: () => { void load(); }, toggleUserStatus, changeUserRole, changeUserPlan };
}

// --- Fuentes (SUP-243: listar + habilitar/deshabilitar) ---
export function useAdminSources() {
  const [sources, setSources] = useState<AdminSource[]>(adminSources);
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
          setSources(adminSources);
          setError(`No se pudo cargar fuentes del backend. Mostrando datos de ejemplo. (${toMessage(err)})`);
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
  const [identidad, setIdentidad] = useState<ServiceState<IdentidadMetrics>>({ data: adminMetricsMock.identidad, error: null });
  const [catalogo, setCatalogo] = useState<ServiceState<CatalogoMetrics>>({ data: adminMetricsMock.catalogo, error: null });
  const [control, setControl] = useState<ServiceState<ControlMetrics>>({ data: adminMetricsMock.control, error: null });
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
