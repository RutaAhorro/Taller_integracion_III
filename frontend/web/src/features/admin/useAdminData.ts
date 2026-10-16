// Hooks del panel admin (SUP-255 / SUP-280).
// Estrategia: si no hay VITE_API_URL o el fetch falla/aborta, se usan los mocks
// y se expone el error para el banner visual. Cuando el backend exista
// (SUP-242/243/279), los hooks ya piden en paralelo y combinan sin cambios en UI.

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ADMIN_HAS_BACKEND,
  adminRoleToBackend,
  fetchAdminSources,
  fetchAdminUsers,
  fetchCatalogoMetrics,
  fetchControlMetrics,
  fetchIdentidadMetrics,
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

// --- Usuarios (SUP-242: listar + activar/desactivar + rol colaborador) ---
export function useAdminUsers() {
  const [users, setUsers] = useState<AdminUser[]>(adminUsers);
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const seq = useRef(0);

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
      // Fallback a mocks para no romper la vista; el banner explica la causa.
      if (seq.current === current) {
        setUsers(adminUsers);
        setError(`No se pudo cargar usuarios del backend. Mostrando datos de ejemplo. (${toMessage(err)})`);
      }
    } finally {
      if (seq.current === current) setIsLoading(false);
    }
  }, []);

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
    if (ADMIN_HAS_BACKEND) {
      try {
        await updateAdminUser(id, { esta_activo: nextActive });
      } catch (err) {
        // Revierte el optimista y muestra el error.
        setUsers((prev) =>
          prev.map((user): AdminUser => (user.id === id ? { ...user, status: target.status } : user)),
        );
        setError(`No se pudo actualizar el usuario. (${toMessage(err)})`);
      } finally {
        setPendingId(null);
      }
    } else {
      // TODO backend (SUP-242): PUT /api/v1/admin/usuarios/:id { esta_activo }.
      setPendingId(null);
    }
  }, [users, pendingId]);

  const changeUserRole = useCallback(async (id: string, role: AdminRole) => {
    const target = users.find((user) => user.id === id);
    if (!target || pendingId || target.role === role) return;
    const previous = target.role;
    setPendingId(id);
    setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, role } : user)));
    if (ADMIN_HAS_BACKEND) {
      try {
        await updateAdminUser(id, { rol: adminRoleToBackend(role) });
      } catch (err) {
        setUsers((prev) => prev.map((user): AdminUser => (user.id === id ? { ...user, role: previous } : user)));
        setError(`No se pudo asignar el rol. (${toMessage(err)})`);
      } finally {
        setPendingId(null);
      }
    } else {
      // TODO backend (SUP-242): PUT /api/v1/admin/usuarios/:id { rol }.
      setPendingId(null);
    }
  }, [users, pendingId]);

  return { users, isLoading, error, pendingId, refresh: () => { void load(); }, toggleUserStatus, changeUserRole };
}

// --- Fuentes (SUP-243: listar + habilitar/deshabilitar) ---
export function useAdminSources() {
  const [sources, setSources] = useState<AdminSource[]>(adminSources);
  const [isLoading, setIsLoading] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const seq = useRef(0);

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
        setSources(adminSources);
        setError(`No se pudo cargar fuentes del backend. Mostrando datos de ejemplo. (${toMessage(err)})`);
      }
    } finally {
      if (seq.current === current) setIsLoading(false);
    }
  }, []);

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
        setError(`No se pudo actualizar la fuente. (${toMessage(err)})`);
      } finally {
        setPendingId(null);
      }
    } else {
      // TODO backend (SUP-243): PUT /api/v1/admin/productos/fuentes/:id { habilitada }.
      setPendingId(null);
    }
  }, [sources, pendingId]);

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
    // Tres llamadas en paralelo; el fallo de una no rompe las otras (SUP-280).
    const [identidadRes, catalogoRes, controlRes] = await Promise.allSettled([
      fetchIdentidadMetrics(signal),
      fetchCatalogoMetrics(signal),
      fetchControlMetrics(signal),
    ]);
    if (seq.current !== current || signal?.aborted) return;
    setIdentidad(
      identidadRes.status === 'fulfilled'
        ? { data: identidadRes.value, error: null }
        : { data: null, error: toMessage(identidadRes.reason) },
    );
    setCatalogo(
      catalogoRes.status === 'fulfilled'
        ? { data: catalogoRes.value, error: null }
        : { data: null, error: toMessage(catalogoRes.reason) },
    );
    setControl(
      controlRes.status === 'fulfilled'
        ? { data: controlRes.value, error: null }
        : { data: null, error: toMessage(controlRes.reason) },
    );
    setIsLoading(false);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  return { identidad, catalogo, control, isLoading, refresh: () => { void load(); } };
}
