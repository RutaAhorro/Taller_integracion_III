import { FeatureUnavailable } from '@/components/FeatureUnavailable';

export default function Planes() {
  return (
    <FeatureUnavailable
      title="Suscripciones no disponibles"
      description="Los planes comerciales no están habilitados. No se ofrecen precios, pagos ni cambios de suscripción desde esta vista."
    />
  );
}
