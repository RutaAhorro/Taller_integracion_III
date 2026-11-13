// Conserva la ruta del perfil inicial sin simular el guardado de preferencias.
import { FeatureUnavailable } from '@/components/FeatureUnavailable';

export default function Onboarding() {
  return (
    <FeatureUnavailable
      title="Configuración inicial no disponible"
      description="Esta vista del perfil aún no está implementada. No se guardan ubicación, preferencias de supermercados ni restricciones alimentarias desde este recorrido."
    />
  );
}
