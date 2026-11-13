// Conserva la ruta del visor sin generar paradas, costos o ahorros simulados.
import { FeatureUnavailable } from '@/components/FeatureUnavailable';

export default function RouteViewer() {
  return (
    <FeatureUnavailable
      title="Ruta no disponible"
      description="El cálculo de rutas aún no está integrado. No hay paradas, costos ni ahorros calculados para mostrar."
    />
  );
}
