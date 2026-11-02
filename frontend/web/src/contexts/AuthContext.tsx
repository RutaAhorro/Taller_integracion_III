import { createContext, useState, useEffect, type ReactNode } from 'react';
import type { Usuario, LoginPayload, RegisterPayload } from '@/types';

// Local storage key for persistence
const AUTH_USER_KEY = 'auth_user';

interface AuthContextType {
  user: Usuario | null;
  isGuest: boolean;
  isLoading: boolean;
  error: string | null;
  login: (payload: LoginPayload) => Promise<boolean>;
  register: (payload: RegisterPayload) => Promise<boolean>;
  googleLogin: (idToken: string) => Promise<boolean>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_URL = import.meta.env.VITE_API_URL || '';

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

  // Sync state changes with localStorage
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

  const logout = () => {
    setUser(null);
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isGuest: !user?.id,
        isLoading,
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
