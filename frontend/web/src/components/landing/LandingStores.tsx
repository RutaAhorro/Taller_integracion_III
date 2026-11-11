// Muestra únicamente cadenas presentes en los productos recibidos, sin prometer cobertura completa.
import { Store } from 'lucide-react';
import type { UiSupermarket } from '@/types';

type LandingStoresProps = { markets: UiSupermarket[]; isLoading: boolean; error: string | null };

export function LandingStores({ markets, isLoading, error }: LandingStoresProps) {
  return (
    <section id="supermercados" className="py-[3.5rem] md:py-[5rem] bg-card border-y-2 border-border">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="max-w-[680px] mx-auto mb-[3.5rem] text-center">
          <h2 className="font-['Fraunces'] text-[1.75rem] font-bold mb-4">Supermercados presentes en el catálogo</h2>
          <p className="text-muted-foreground">Estas cadenas aparecen en la muestra cargada. El listado completo está pendiente de integración.</p>
        </div>
        {isLoading ? <p role="status">Cargando supermercados…</p> : error ? <p role="alert">{error}</p> : markets.length === 0 ? <p role="status">No hay supermercados disponibles en esta muestra.</p> : (
          <div className="grid grid-cols-2 sm:grid-cols-[repeat(auto-fit,minmax(170px,1fr))] gap-4">
            {markets.map(market => (
              <div key={market.id} className="p-6 text-center border-2 border-border rounded-[var(--radius)] bg-muted">
                <Store className="mx-auto mb-2" aria-hidden="true" />
                <h3 className="font-bold">{market.name}</h3>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
