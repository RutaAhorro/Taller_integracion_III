// Comparte la sesión de Identidad; el usuario local no sustituye la autorización por cookie del servidor.
import { createContext, useState, useEffect, type ReactNode } from 'react';
import type { Usuario, LoginPayload, RegisterPayload } from '@/types';

// Clave del usuario de interfaz almacenado entre recargas.
const AUTH_USER_KEY = 'auth_user';

interface AuthContextType {
  user: Usuario | null;
  isGuest: boolean;
  isLoading: boolean;
  error: string | null;
  login: (payload: LoginPayload) => Promise<Usuario | null>;
  register: (payload: RegisterPayload) => Promise<boolean>;
  googleLogin: (idToken: string) => Promise<Usuario | null>;
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

  // Sincroniza el usuario de interfaz con el almacenamiento local.
  useEffect(() => {
    if (user) {
      localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
    } else {
      localStorage.removeItem(AUTH_USER_KEY);
    }
  }, [user]);

  // Envuelve una llamada de autenticación con el manejo común de carga y error.
  const run = async <Resultado,>(action: () => Promise<Resultado>, networkError = 'Error de red.'): Promise<Resultado | null> => {
    setIsLoading(true);
    setError(null);
    try {
      return await action();
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : networkError);
      return null;
    } finally {
      setIsLoading(false);
    }
  };

  // Devuelve la identidad recién validada, nunca la recuperada de localStorage.
  const autenticar = (ruta: string, cuerpo: unknown, mensajeError: string) =>
    run(async () => {
      const result = await postAuth(ruta, cuerpo, mensajeError);
      const usuario: Usuario = result.usuario ?? result;
      if (!usuario?.id) throw new Error('Respuesta de autenticación inválida.');
      setUser(usuario);
      return usuario;
    });

  const login = (payload: LoginPayload) =>
    autenticar('/auth/login', payload, 'Error al iniciar sesión');

  const register = async (payload: RegisterPayload) =>
    Boolean(await run(async () => {
      await postAuth('/auth/register', payload, 'Error al crear la cuenta');
      return true;
    }));

  const googleLogin = (idToken: string) =>
    autenticar('/auth/google', { id_token: idToken }, 'Error en Google Login');

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
