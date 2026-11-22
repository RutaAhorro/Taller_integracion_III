// Comparte la sesión de Identidad; el usuario local no sustituye la autorización por cookie del servidor.
import { createContext, useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import type { Usuario, LoginPayload, RegisterPayload } from '@/features/autenticacion/tipos';
import { useToast } from '@/components/ToastContext';

// Clave del usuario de interfaz almacenado entre recargas.
const CLAVE_USUARIO_AUTENTICADO = 'auth_user';

interface TipoContextoAutenticacion {
  user: Usuario | null;
  isGuest: boolean;
  isLoading: boolean;
  isLoggingOut: boolean;
  error: string | null;
  login: (datosAcceso: LoginPayload) => Promise<Usuario | null>;
  register: (datosRegistro: RegisterPayload) => Promise<boolean>;
  googleLogin: (tokenId: string) => Promise<Usuario | null>;
  logout: () => Promise<void>;
}

export const AuthContext = createContext<TipoContextoAutenticacion | undefined>(undefined);

const URL_API = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');

// Un valor corrupto en localStorage no debe impedir que cargue la app.
function leerUsuarioAlmacenado(): Usuario | null {
  try {
    const almacenado = localStorage.getItem(CLAVE_USUARIO_AUTENTICADO);
    return almacenado ? JSON.parse(almacenado) : null;
  } catch {
    return null;
  }
}

// POST a Identidad. El JWT viaja en una cookie HttpOnly (credentials: include).
// Lanza un Error con el mensaje del backend, o con `mensajeRespaldo` si no lo hay.
async function enviarAutenticacion(ruta: string, cuerpo: unknown, mensajeRespaldo: string) {
  const respuesta = await fetch(`${URL_API}${ruta}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cuerpo),
    credentials: 'include',
  });
  // Un 502/504 del Gateway puede no traer JSON.
  const resultado = await respuesta.json().catch(() => ({}));
  if (!respuesta.ok) throw new Error(resultado.error || resultado.mensaje || mensajeRespaldo);
  return resultado;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [usuarioActual, setUsuarioActual] = useState<Usuario | null>(leerUsuarioAlmacenado);
  const [error, setError] = useState<string | null>(null);
  const [cargando, setCargando] = useState(false);
  const [cerrandoSesion, setCerrandoSesion] = useState(false);
  const peticionCierreSesion = useRef<Promise<void> | null>(null);
  const { toast } = useToast();

  // Sincroniza el usuario de interfaz con el almacenamiento local.
  useEffect(() => {
    if (usuarioActual) {
      localStorage.setItem(CLAVE_USUARIO_AUTENTICADO, JSON.stringify(usuarioActual));
    } else {
      localStorage.removeItem(CLAVE_USUARIO_AUTENTICADO);
    }
  }, [usuarioActual]);

  // Envuelve una llamada de autenticación con el manejo común de carga y error.
  const ejecutar = async <Resultado,>(accion: () => Promise<Resultado>, mensajeErrorRed = 'Error de red.'): Promise<Resultado | null> => {
    setCargando(true);
    setError(null);
    try {
      return await accion();
    } catch (errorObtenido) {
      setError(errorObtenido instanceof Error && errorObtenido.message ? errorObtenido.message : mensajeErrorRed);
      return null;
    } finally {
      setCargando(false);
    }
  };

  // Devuelve la identidad recién validada, nunca la recuperada de localStorage.
  const autenticar = (ruta: string, cuerpo: unknown, mensajeError: string) =>
    ejecutar(async () => {
      const resultado = await enviarAutenticacion(ruta, cuerpo, mensajeError);
      const usuario: Usuario = resultado.usuario ?? resultado;
      if (!usuario?.id) throw new Error('Respuesta de autenticación inválida.');
      setUsuarioActual(usuario);
      return usuario;
    });

  const login = (datosAcceso: LoginPayload) =>
    autenticar('/auth/login', datosAcceso, 'Error al iniciar sesión');

  const register = async (datosRegistro: RegisterPayload) =>
    Boolean(await ejecutar(async () => {
      await enviarAutenticacion('/auth/register', datosRegistro, 'Error al crear la cuenta');
      return true;
    }));

  const googleLogin = (tokenId: string) =>
    autenticar('/auth/google', { id_token: tokenId }, 'Error en Google Login');

  // Expira la cookie HttpOnly en Identidad antes de salir. Los cierres simultáneos comparten petición.
  const logout = useCallback((): Promise<void> => {
    if (peticionCierreSesion.current) return peticionCierreSesion.current;
    setCerrandoSesion(true);
    setError(null);
    peticionCierreSesion.current = (async () => {
      try {
        const respuesta = await fetch(`${URL_API}/auth/logout`, {
          method: 'POST',
          credentials: 'include',
        });
        // El contrato responde 204 sin JSON; una página HTML con 200 no confirma el cierre.
        if (respuesta.status !== 204) throw new Error('No se confirmó el cierre de sesión.');
      } catch {
        const mensaje = 'Se cerró la sesión local, pero no se pudo confirmar el cierre en el servidor. La cookie podría seguir activa; vuelve a intentar cerrar sesión cuando haya conexión.';
        setError(mensaje);
        toast(mensaje, 'warning');
      } finally {
        setUsuarioActual(null);
        localStorage.removeItem(CLAVE_USUARIO_AUTENTICADO);
        setCerrandoSesion(false);
      }
    })().finally(() => {
      peticionCierreSesion.current = null;
    });
    return peticionCierreSesion.current;
  }, [toast]);

  return (
    <AuthContext.Provider
      value={{
        user: usuarioActual,
        isGuest: !usuarioActual?.id,
        isLoading: cargando || cerrandoSesion,
        isLoggingOut: cerrandoSesion,
        error,
        login,
        register,
        googleLogin,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
