// Mantiene la ruta de misiones sin inventar reportes, puntos ni recompensas.
import { FeatureUnavailable } from '@/components/FeatureUnavailable';

export default function Crowdsourcing() {
  return (
    <FeatureUnavailable
      title="Misiones no disponibles"
      description="Las misiones y recompensas no están habilitadas. No se pueden enviar reportes ni otorgar puntos desde esta vista."
    />
  );
}
