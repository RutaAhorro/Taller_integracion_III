// Gestiona lecturas y escrituras administrativas sin sustituir errores por datos ficticios.
// Sin URL de API informa configuración ausente; ante 401/403 cierra la sesión local.

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/features/autenticacion/useAuth';
import { ROUTES } from '@/app/routes';
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
  type AdminPlan,
  type AdminRole,
  type AdminSource,
  type AdminUser,
} from './admin-types';

const BACKEND_NO_CONFIGURADO = 'Administración no disponible: falta configurar VITE_API_URL.';

export type ServiceState<Datos> = {
  data: Datos | null;
  error: string | null;
};

function obtenerMensajeError(errorObtenido: unknown): string {
  if (errorObtenido instanceof DOMException && errorObtenido.name === 'AbortError') return 'Petición cancelada.';
  return errorObtenido instanceof Error ? errorObtenido.message : 'Error de red.';
}

// Mensaje fail-closed para 401/403: sin mocks; la sesión se cierra y se
// vuelve a /login (SUP-256).
function mensajeAccesoDenegado(recurso: string, errorObtenido: unknown): string {
  return `Acceso denegado al cargar ${recurso} (${obtenerMensajeError(errorObtenido)}). La sesión expiró o no tiene permisos; vuelve a iniciar sesión.`;
}

// Expiración de sesión (SUP-256): ante un 401/403 se cierra la sesión local y
// se redirige a /login. logout/navigate se leen desde un ref actualizado tras
// cada actualización para no recrear las funciones de carga ni repetir las consultas.
function usarExpirarSesion(): () => void {
  const { logout: cerrarSesion } = useAuth();
  const navegar = useNavigate();
  const referenciaSesion = useRef({ cerrarSesion, navegar });

  useEffect(() => {
    referenciaSesion.current = { cerrarSesion, navegar };
  });

  return useCallback(() => {
    void referenciaSesion.current.cerrarSesion().then(() => {
      referenciaSesion.current.navegar(ROUTES.LOGIN, { replace: true });
    });
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

function mensajeMutacionUsuario(prefijo: string, errorObtenido: unknown): string {
  if (isAdminApiError(errorObtenido)) {
    if (errorObtenido.status === 409) return errorObtenido.message;
    return `${prefijo} ${errorObtenido.message}${errorObtenido.detail ? ` (${errorObtenido.detail})` : ''}`;
  }
  return `${prefijo} (${obtenerMensajeError(errorObtenido)})`;
}

// El hook serializa las mutaciones: si llega otra mientras hay una en vuelo,
// se informa en vez de descartarla en silencio (SUP-242).
const MUTACION_EN_CURSO =
  'Hay otra actualización en curso. Espera a que termine antes de modificar otro usuario.';
const MUTACION_EN_CURSO_FUENTE =
  'Hay otra actualización en curso. Espera a que termine antes de modificar otra fuente.';

export function useAdminUsers(parametros: AdminUsersParams = {}) {
  const pagina = Math.max(1, parametros.page ?? 1);
  const limite = parametros.limit ?? 20;
  const busqueda = (parametros.q ?? '').trim();
  const rolFiltro = parametros.rol && parametros.rol !== 'Todos' ? adminRoleToBackend(parametros.rol) : '';

  const [usuarios, setUsuarios] = useState<AdminUser[]>([]);
  const [paginacion, setPaginacion] = useState<AdminUsersPagination>({ page: pagina, limit: limite, total: 0, totalPages: 0 });
  const [cargando, setCargando] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [idPendiente, setIdPendiente] = useState<string | null>(null);
  const secuencia = useRef(0);
  const referenciaPendiente = useRef<string | null>(null);
  const expirarSesion = usarExpirarSesion();

  const cargar = useCallback(async (senal?: AbortSignal) => {
    // Un refresh durante una mutación podría devolver el estado anterior y
    // pisar la actualización optimista. La mutación invalida cualquier load.
    if (referenciaPendiente.current) return;

    if (!ADMIN_HAS_BACKEND) {
      setUsuarios([]);
      setPaginacion({ page: pagina, limit: limite, total: 0, totalPages: 0 });
      setCargando(false);
      setError(BACKEND_NO_CONFIGURADO);
      return;
    }

    const actual = ++secuencia.current;
    setCargando(true);
    setError(null);
    try {
      const resultado = await fetchAdminUsers({ page: pagina, limit: limite, q: busqueda, rol: rolFiltro }, senal);
      if (secuencia.current === actual) {
        setUsuarios(resultado.users);
        setPaginacion({ page: resultado.page, limit: resultado.limit, total: resultado.total, totalPages: resultado.totalPages });
      }
    } catch (errorObtenido) {
      if (senal?.aborted) return;
      // Auditoría H-03 (fail-closed): 401/403 no inyectan mocks ni mantienen
      // datos previos operativos; se vacía, se cierra sesión y se va a /login.
      if (secuencia.current === actual) {
        if (isAdminAuthError(errorObtenido)) {
          setUsuarios([]);
          setError(mensajeAccesoDenegado('usuarios', errorObtenido));
          expirarSesion();
        } else {
          setUsuarios([]);
          setPaginacion({ page: 1, limit: limite, total: 0, totalPages: 0 });
          setError(`No se pudo cargar usuarios del backend. (${obtenerMensajeError(errorObtenido)})`);
        }
      }
    } finally {
      if (secuencia.current === actual) setCargando(false);
    }
  }, [expirarSesion, pagina, limite, busqueda, rolFiltro]);

  useEffect(() => {
    const controlador = new AbortController();
    void cargar(controlador.signal);
    return () => controlador.abort();
  }, [cargar]);

  const iniciarMutacion = useCallback((id: string) => {
    // Invalida cualquier GET en vuelo y bloquea refresh hasta terminar.
    secuencia.current += 1;
    referenciaPendiente.current = id;
    setIdPendiente(id);
    setCargando(false);
  }, []);

  const finalizarMutacion = useCallback(() => {
    referenciaPendiente.current = null;
    setIdPendiente(null);
  }, []);

  const aplicarActualizado = useCallback((id: string, actualizado: AdminUser) => {
    if (rolFiltro && (actualizado.role === 'Desconocido' || adminRoleToBackend(actualizado.role) !== rolFiltro)) {
      setUsuarios((anteriores) => anteriores.filter((usuario) => usuario.id !== id));
      setPaginacion((anterior) => {
        const total = Math.max(0, anterior.total - 1);
        return { ...anterior, total, totalPages: total === 0 ? 0 : Math.ceil(total / anterior.limit) };
      });
      return;
    }
    setUsuarios((anteriores) => anteriores.map((usuario) => (usuario.id === id ? actualizado : usuario)));
  }, [rolFiltro]);

  const alternarEstadoUsuario = useCallback(async (id: string) => {
    const objetivo = usuarios.find((usuario) => usuario.id === id);
    if (!objetivo) return;
    if (!ADMIN_HAS_BACKEND || !isPersistableId(id)) {
      setError(!ADMIN_HAS_BACKEND ? BACKEND_NO_CONFIGURADO : 'Identificador de usuario inválido.');
      return;
    }
    if (referenciaPendiente.current) {
      setError(MUTACION_EN_CURSO);
      return;
    }
    const siguienteActivo = objetivo.status !== 'Activo';
    iniciarMutacion(id);
    setUsuarios((anteriores) =>
      anteriores.map((usuario): AdminUser => (usuario.id === id ? { ...usuario, status: siguienteActivo ? 'Activo' : 'Inactivo' } : usuario)),
    );
    // El cambio optimista solo se conserva si el backend confirma la escritura.
    try {
      const actualizado = await updateAdminUser(id, { esta_activo: siguienteActivo });
      aplicarActualizado(id, actualizado);
      setError(null);
    } catch (errorObtenido) {
      setUsuarios((anteriores) => anteriores.map((usuario): AdminUser => (usuario.id === id ? objetivo : usuario)));
      if (isAdminAuthError(errorObtenido)) {
        setError(mensajeAccesoDenegado('el usuario', errorObtenido));
        expirarSesion();
      } else {
        setError(mensajeMutacionUsuario('No se pudo actualizar el usuario.', errorObtenido));
      }
    } finally {
      finalizarMutacion();
    }
  }, [usuarios, iniciarMutacion, finalizarMutacion, aplicarActualizado, expirarSesion]);

  const cambiarRolUsuario = useCallback(async (id: string, rol: AdminRole) => {
    const objetivo = usuarios.find((usuario) => usuario.id === id);
    if (!objetivo || objetivo.role === rol) return;
    if (!ADMIN_HAS_BACKEND || !isPersistableId(id)) {
      setError(!ADMIN_HAS_BACKEND ? BACKEND_NO_CONFIGURADO : 'Identificador de usuario inválido.');
      return;
    }
    if (referenciaPendiente.current) {
      setError(MUTACION_EN_CURSO);
      return;
    }
    iniciarMutacion(id);
    setUsuarios((anteriores) => anteriores.map((usuario): AdminUser => (usuario.id === id ? { ...usuario, role: rol } : usuario)));
    try {
      const actualizado = await updateAdminUser(id, { rol: adminRoleToBackend(rol) });
      aplicarActualizado(id, actualizado);
      setError(null);
    } catch (errorObtenido) {
      setUsuarios((anteriores) => anteriores.map((usuario): AdminUser => (usuario.id === id ? objetivo : usuario)));
      if (isAdminAuthError(errorObtenido)) {
        setError(mensajeAccesoDenegado('el rol', errorObtenido));
        expirarSesion();
      } else {
        setError(mensajeMutacionUsuario('No se pudo asignar el rol.', errorObtenido));
      }
    } finally {
      finalizarMutacion();
    }
  }, [usuarios, iniciarMutacion, finalizarMutacion, aplicarActualizado, expirarSesion]);

  const cambiarPlanUsuario = useCallback(async (id: string, plan: AdminPlan) => {
    const objetivo = usuarios.find((usuario) => usuario.id === id);
    if (!objetivo || objetivo.plan === plan) return;
    if (!ADMIN_HAS_BACKEND || !isPersistableId(id)) {
      setError(!ADMIN_HAS_BACKEND ? BACKEND_NO_CONFIGURADO : 'Identificador de usuario inválido.');
      return;
    }
    if (referenciaPendiente.current) {
      setError(MUTACION_EN_CURSO);
      return;
    }
    iniciarMutacion(id);
    setUsuarios((anteriores) => anteriores.map((usuario): AdminUser => (usuario.id === id ? { ...usuario, plan } : usuario)));
    try {
      const actualizado = await updateAdminUser(id, { plan: adminPlanToBackend(plan) });
      setUsuarios((anteriores) => anteriores.map((usuario) => (usuario.id === id ? actualizado : usuario)));
      setError(null);
    } catch (errorObtenido) {
      setUsuarios((anteriores) => anteriores.map((usuario): AdminUser => (usuario.id === id ? objetivo : usuario)));
      if (isAdminAuthError(errorObtenido)) {
        setError(mensajeAccesoDenegado('el plan', errorObtenido));
        expirarSesion();
      } else {
        setError(mensajeMutacionUsuario('No se pudo asignar el plan.', errorObtenido));
      }
    } finally {
      finalizarMutacion();
    }
  }, [usuarios, iniciarMutacion, finalizarMutacion, expirarSesion]);

  const recargar = useCallback(() => {
    if (!referenciaPendiente.current) void cargar();
  }, [cargar]);

  return {
    users: usuarios,
    pagination: paginacion,
    isLoading: cargando,
    error,
    pendingId: idPendiente,
    refresh: recargar,
    toggleUserStatus: alternarEstadoUsuario,
    changeUserRole: cambiarRolUsuario,
    changeUserPlan: cambiarPlanUsuario,
  };
}

// --- Fuentes (SUP-243: listar + habilitar/deshabilitar) ---
export function useAdminSources() {
  const [fuentes, setFuentes] = useState<AdminSource[]>([]);
  const [cargando, setCargando] = useState(ADMIN_HAS_BACKEND);
  const [error, setError] = useState<string | null>(null);
  const [idPendiente, setIdPendiente] = useState<string | null>(null);
  const secuencia = useRef(0);
  const referenciaPendiente = useRef<string | null>(null);
  const expirarSesion = usarExpirarSesion();

  const cargar = useCallback(async (senal?: AbortSignal) => {
    if (referenciaPendiente.current) return;
    if (!ADMIN_HAS_BACKEND) {
      setFuentes([]);
      setCargando(false);
      setError(BACKEND_NO_CONFIGURADO);
      return;
    }
    const actual = ++secuencia.current;
    setCargando(true);
    setError(null);
    try {
      const datos = await fetchAdminSources(senal);
      if (secuencia.current === actual) setFuentes(datos);
    } catch (errorObtenido) {
      if (senal?.aborted) return;
      if (secuencia.current === actual) {
        // Auditoría H-03 (fail-closed): 401/403 no inyectan mocks; se cierra
        // sesión y se vuelve a /login.
        if (isAdminAuthError(errorObtenido)) {
          setFuentes([]);
          setError(mensajeAccesoDenegado('fuentes', errorObtenido));
          expirarSesion();
        } else {
          setFuentes([]);
          setError(`No se pudo cargar fuentes del backend. (${obtenerMensajeError(errorObtenido)})`);
        }
      }
    } finally {
      if (secuencia.current === actual) setCargando(false);
    }
  }, [expirarSesion]);

  useEffect(() => {
    const controlador = new AbortController();
    void cargar(controlador.signal);
    return () => controlador.abort();
  }, [cargar]);

  const alternarFuente = useCallback(async (id: string) => {
    const objetivo = fuentes.find((fuente) => fuente.id === id);
    if (!objetivo) return;
    if (!ADMIN_HAS_BACKEND) {
      setError(BACKEND_NO_CONFIGURADO);
      return;
    }
    if (referenciaPendiente.current) {
      setError(MUTACION_EN_CURSO_FUENTE);
      return;
    }
    const habilitada = !objetivo.habilitada;
    // Invalida lecturas previas y bloquea cambios simultáneos hasta confirmar el PUT.
    secuencia.current += 1;
    referenciaPendiente.current = id;
    setIdPendiente(id);
    setCargando(false);
    setFuentes((anteriores) =>
      anteriores.map((fuente): AdminSource => (
        fuente.id === id
          ? { ...fuente, habilitada, status: habilitada ? (fuente.status === 'Inactiva' ? 'Operativa' : fuente.status) : 'Inactiva' }
          : fuente
      )),
    );
    try {
      const actualizada = await updateAdminSource(id, { habilitada });
      setFuentes((anteriores) => anteriores.map((fuente) => (fuente.id === id ? actualizada : fuente)));
      setError(null);
    } catch (errorObtenido) {
      setFuentes((anteriores) => anteriores.map((fuente) => (fuente.id === id ? objetivo : fuente)));
      if (isAdminAuthError(errorObtenido)) {
        setError(mensajeAccesoDenegado('la fuente', errorObtenido));
        expirarSesion();
      } else {
        setError(`No se pudo actualizar la fuente. (${obtenerMensajeError(errorObtenido)})`);
      }
    } finally {
      referenciaPendiente.current = null;
      setIdPendiente(null);
    }
  }, [fuentes, expirarSesion]);

  return {
    sources: fuentes,
    isLoading: cargando,
    error,
    pendingId: idPendiente,
    refresh: () => { void cargar(); },
    toggleSource: alternarFuente,
  };
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
    { data: null, error: null },
  );
  const [catalogo, setCatalogo] = useState<ServiceState<CatalogoMetrics>>(
    { data: null, error: null },
  );
  const [control, setControl] = useState<ServiceState<ControlMetrics>>(
    { data: null, error: null },
  );
  const [cargando, setCargando] = useState(ADMIN_HAS_BACKEND);
  const secuencia = useRef(0);
  const expirarSesion = usarExpirarSesion();

  const cargar = useCallback(async (senal?: AbortSignal) => {
    if (!ADMIN_HAS_BACKEND) {
      setIdentidad({ data: null, error: BACKEND_NO_CONFIGURADO });
      setCatalogo({ data: null, error: BACKEND_NO_CONFIGURADO });
      setControl({ data: null, error: BACKEND_NO_CONFIGURADO });
      setCargando(false);
      return;
    }
    const actual = ++secuencia.current;
    setCargando(true);
    try {
      // Tres llamadas en paralelo; el fallo de una no rompe las otras (SUP-280).
      // Auditoría H-08: try/finally para no congelar isLoading ante abort/carrera.
      const [respuestaIdentidad, respuestaCatalogo, respuestaControl] = await Promise.allSettled([
        fetchIdentidadMetrics(senal),
        fetchCatalogoMetrics(senal),
        fetchControlMetrics(senal),
      ]);
      if (secuencia.current !== actual || senal?.aborted) return;
      setIdentidad(
        respuestaIdentidad.status === 'fulfilled'
          ? { data: respuestaIdentidad.value, error: null }
          // H-03: 401/403 sin mocks (fail-closed); resto con tarjeta "sin datos".
          : { data: null, error: isAdminAuthError(respuestaIdentidad.reason) ? mensajeAccesoDenegado('métricas de identidad', respuestaIdentidad.reason) : obtenerMensajeError(respuestaIdentidad.reason) },
      );
      setCatalogo(
        respuestaCatalogo.status === 'fulfilled'
          ? { data: respuestaCatalogo.value, error: null }
          : { data: null, error: isAdminAuthError(respuestaCatalogo.reason) ? mensajeAccesoDenegado('métricas de catálogo', respuestaCatalogo.reason) : obtenerMensajeError(respuestaCatalogo.reason) },
      );
      setControl(
        respuestaControl.status === 'fulfilled'
          ? { data: respuestaControl.value, error: null }
          : { data: null, error: isAdminAuthError(respuestaControl.reason) ? mensajeAccesoDenegado('métricas de control', respuestaControl.reason) : obtenerMensajeError(respuestaControl.reason) },
      );
      // SUP-256: si alguna llamada respondió 401/403, se cierra la sesión y se
      // vuelve a /login (fail-closed) en vez de quedar en estado degradado.
      const sesionExpirada = [respuestaIdentidad, respuestaCatalogo, respuestaControl].some(
        (resultado) => resultado.status === 'rejected' && isAdminAuthError(resultado.reason),
      );
      if (sesionExpirada) expirarSesion();
    } finally {
      if (secuencia.current === actual) setCargando(false);
    }
  }, [expirarSesion]);

  useEffect(() => {
    const controlador = new AbortController();
    void cargar(controlador.signal);
    return () => controlador.abort();
  }, [cargar]);

  return { identidad, catalogo, control, isLoading: cargando, refresh: () => { void cargar(); } };
}
