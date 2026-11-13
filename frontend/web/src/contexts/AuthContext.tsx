// Comparte la sesión de Identidad; el usuario local no sustituye la autorización por cookie del servidor.
import { createContext, useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import type { Usuario, LoginPayload, RegisterPayload } from '@/types';
import { useToast } from '@/contexts/ToastContext';

// Clave del usuario de interfaz almacenado entre recargas.
const AUTH_USER_KEY = 'auth_user';

interface AuthContextType {
  user: Usuario | null;
  isGuest: boolean;
  isLoading: boolean;
  isLoggingOut: boolean;
  error: string | null;
  login: (payload: LoginPayload) => Promise<boolean>;
  register: (payload: RegisterPayload) => Promise<boolean>;
  googleLogin: (idToken: string) => Promise<boolean>;
  logout: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_URL = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '');

// Un valor corrupto en localStorage no debe impedir que cargue la app.
function readStoredUser(): Usuario | null {
  try {
    const stored = localStorage.getItem(AUTH_USER_KEY);
    return stored ? JSON.parse(stored) : null;
  } catch {
    return null;
  }
}

// POST a Identidad. El JWT viaja en una cookie HttpOnly (credentials: include).
// Lanza un Error con el mensaje del backend, o con `fallback` si no lo hay.
async function postAuth(path: string, body: unknown, fallback: string) {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    credentials: 'include',
  });
  // Un 502/504 del Gateway puede no traer JSON.
  const result = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(result.error || result.mensaje || fallback);
  return result;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Usuario | null>(readStoredUser);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const logoutRequest = useRef<Promise<void> | null>(null);
  const { toast } = useToast();

  // Sincroniza el usuario de interfaz con el almacenamiento local.
  useEffect(() => {
    if (user) {
      localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
    } else {
      localStorage.removeItem(AUTH_USER_KEY);
    }
  }, [user]);

  // Envuelve una llamada de autenticación con el manejo común de carga y error.
  const run = async (action: () => Promise<void>, networkError = 'Error de red.') => {
    setIsLoading(true);
    setError(null);
    try {
      await action();
      return true;
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : networkError);
      return false;
    } finally {
      setIsLoading(false);
    }
  };

  const login = (payload: LoginPayload) =>
    run(async () => {
      const result = await postAuth('/auth/login', payload, 'Error al iniciar sesión');
      setUser(result.usuario ?? result);
    });

  const register = (payload: RegisterPayload) =>
    run(async () => {
      await postAuth('/auth/register', payload, 'Error al crear la cuenta');
    });

  const googleLogin = (idToken: string) =>
    run(async () => {
      const result = await postAuth('/auth/google', { id_token: idToken }, 'Error en Google Login');
      setUser(result.usuario ?? result);
    }, 'Error de red en Google Login.');

  // Expira la cookie HttpOnly en Identidad antes de salir. Los cierres simultáneos comparten petición.
  const logout = useCallback((): Promise<void> => {
    if (logoutRequest.current) return logoutRequest.current;
    setIsLoggingOut(true);
    setError(null);
    logoutRequest.current = (async () => {
      try {
        const response = await fetch(`${API_URL}/auth/logout`, {
          method: 'POST',
          credentials: 'include',
        });
        // El contrato responde 204 sin JSON; una página HTML con 200 no confirma el cierre.
        if (response.status !== 204) throw new Error('No se confirmó el cierre de sesión.');
      } catch {
        const message = 'Se cerró la sesión local, pero no se pudo confirmar el cierre en el servidor. La cookie podría seguir activa; vuelve a intentar cerrar sesión cuando haya conexión.';
        setError(message);
        toast(message, 'warning');
      } finally {
        setUser(null);
        localStorage.removeItem(AUTH_USER_KEY);
        setIsLoggingOut(false);
      }
    })().finally(() => {
      logoutRequest.current = null;
    });
    return logoutRequest.current;
  }, [toast]);

  return (
    <AuthContext.Provider
      value={{
        user,
        isGuest: !user?.id,
        isLoading: isLoading || isLoggingOut,
        isLoggingOut,
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
