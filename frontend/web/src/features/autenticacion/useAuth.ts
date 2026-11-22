// Acceso al estado y operaciones de sesión compartidos por AuthProvider.
import { useContext } from 'react';
import { AuthContext } from '@/features/autenticacion/AuthContext';

export function useAuth() {
  const contexto = useContext(AuthContext);
  if (contexto === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return contexto;
}
