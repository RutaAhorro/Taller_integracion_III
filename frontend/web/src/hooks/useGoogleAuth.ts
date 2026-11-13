import { useAuth } from './useAuth';
import { useNavigate } from 'react-router-dom';
import { useState } from 'react';

/** Envía la credencial de Google a Identidad y coordina carga, errores y navegación tras el acceso. */
export function useGoogleAuth() {
  const { googleLogin, isLoading: isContextLoading, error: contextError } = useAuth();
  const navigate = useNavigate();
  const [internalError, setInternalError] = useState<string | null>(null);
  const [isGoogleLoading, setIsGoogleLoading] = useState(false);

  const handleGoogleSuccess = async (credentialResponse: any) => {
    setInternalError(null);
    setIsGoogleLoading(true);
    
    try {
      if (!credentialResponse.credential) {
        throw new Error("No se recibió credencial de Google.");
      }

      // El servidor valida la credencial; su presencia no acredita una sesión por sí sola.
      const success = await googleLogin(credentialResponse.credential);
      
      if (success) {
        navigate('/dashboard');
      } else {
        setInternalError("Fallo en la autenticación con el servidor.");
      }
    } catch (err: any) {
      setInternalError(err.message || 'Error conectando con el servidor');
    } finally {
      setIsGoogleLoading(false);
    }
  };

  const handleGoogleError = () => {
    console.error('Google Auth Error');
    setInternalError('Fallo al autenticar con Google. Intente nuevamente.');
  };

  return {
    handleGoogleSuccess,
    handleGoogleError,
    isLoading: isContextLoading || isGoogleLoading,
    error: contextError || internalError,
  };
}
