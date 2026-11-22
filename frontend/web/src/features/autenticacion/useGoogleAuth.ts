// Orquesta el inicio de sesión con Google: envía la credencial a Identidad y redirige según el rol recibido.
import { useAuth } from '@/features/autenticacion/useAuth';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { rutaTrasAutenticacion } from '@/app/routes';

/** Redirige solo tras validar la credencial con Identidad, usando el rol recibido. */
export function useGoogleAuth() {
  const { googleLogin, isLoading: cargandoContexto, error: errorContexto } = useAuth();
  const navegar = useNavigate();
  const [errorInterno, setErrorInterno] = useState<string | null>(null);
  const [cargandoGoogle, setCargandoGoogle] = useState(false);

  // Valida la credencial con el servidor y navega solo si Identidad confirma la sesión.
  const manejarExitoGoogle = async (respuestaCredencial: any) => {
    setErrorInterno(null);
    setCargandoGoogle(true);
    
    try {
      if (!respuestaCredencial.credential) {
        throw new Error("No se recibió credencial de Google.");
      }

      // El servidor valida la credencial; su presencia no acredita una sesión por sí sola.
      const usuario = await googleLogin(respuestaCredencial.credential);
      
      if (usuario) {
        navegar(rutaTrasAutenticacion(usuario.rol), { replace: true });
      } else {
        setErrorInterno("Fallo en la autenticación con el servidor.");
      }
    } catch (errorObtenido: any) {
      setErrorInterno(errorObtenido.message || 'Error conectando con el servidor');
    } finally {
      setCargandoGoogle(false);
    }
  };

  // Informa el fallo del flujo de Google sin tocar la sesión existente.
  const manejarErrorGoogle = () => {
    console.error('Google Auth Error');
    setErrorInterno('Fallo al autenticar con Google. Intente nuevamente.');
  };

  return {
    handleGoogleSuccess: manejarExitoGoogle,
    handleGoogleError: manejarErrorGoogle,
    isLoading: cargandoContexto || cargandoGoogle,
    error: errorContexto || errorInterno,
  };
}
