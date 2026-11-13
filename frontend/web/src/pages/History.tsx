// Conserva el acceso al historial sin mostrar ejecuciones o precios que no fueron guardados.
import { FeatureUnavailable } from '@/components/FeatureUnavailable';

export default function History() {
  return (
    <FeatureUnavailable
      title="Historial no disponible"
      description="El historial de rutas aún no está integrado. Puedes consultar tus listas guardadas en Mis listas de compras."
    />
  );
}
