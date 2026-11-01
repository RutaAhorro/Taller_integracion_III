import { MapContainer, TileLayer, Marker, Popup, Polyline } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';
import L from 'leaflet';
import { MapPin, Navigation, TrendingDown, AlertTriangle, CheckCircle2, Car, Bus } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { useRouteOptimization, type TransportMode } from '@/hooks/useRouteOptimization';
import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';

// Fix Leaflet default icon issues in React
delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-icon-2x.png',
  iconUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-icon.png',
  shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.7.1/images/marker-shadow.png',
});

const TRANSPORT_OPTIONS: { mode: TransportMode; label: string; icon: LucideIcon }[] = [
  { mode: 'car', label: 'Particular', icon: Car },
  { mode: 'bus', label: 'Transporte Público', icon: Bus },
];

// Aviso de una regla de negocio (RN) que impide o desaconseja la ruta.
function RuleAlert({ icon: Icon, title, children }: { icon: LucideIcon; title: string; children: ReactNode }) {
  return (
    <div className="bg-primary p-3 rounded-xl border border-border flex items-start gap-2">
      <Icon className="h-4 w-4 text-primary-foreground mt-0.5 shrink-0"/>
      <div>
        <p className="text-sm font-bold text-primary-foreground">{title}</p>
        <p className="text-xs text-primary-foreground/90 mt-0.5">{children}</p>
      </div>
    </div>
  );
}

function CostRow({ label, value, valueClassName = 'text-foreground' }: { label: string; value: string; valueClassName?: string }) {
  return (
    <div className="flex justify-between items-center text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn('font-medium', valueClassName)}>{value}</span>
    </div>
  );
}

export default function RouteViewer() {
  const opt = useRouteOptimization(3, 'car');

  const polylinePositions: [number, number][] = opt.activeStores.map(store => [store.lat, store.lng]);

  return (
    <div className="flex flex-col-reverse md:flex-row flex-1 bg-background text-foreground overflow-hidden">
      {/* Left Panel: Logistics & Business Rules */}
      <div className="w-full md:w-[400px] lg:w-[450px] bg-card border-r border-border flex flex-col h-[40vh] md:h-full z-10 shadow-xl overflow-y-auto shrink-0">
        
        {/* Header Area */}
        <div className="bg-gradient-to-b from-secondary to-transparent dark:from-secondary px-4 pt-4 pb-3 border-b border-border sticky top-0 bg-card z-20 backdrop-blur-md">
          <div>
            <h1 className="text-lg font-bold mt-0 text-foreground">Ruta Optimizada</h1>
            <p className="text-xs text-muted-foreground mt-0.5">3 Supermercados • 12 Productos</p>
          </div>
        </div>

        <div className="p-3 sm:p-4 space-y-4 flex-1 pb-4">
          
          {/* Transport Mode */}
          <section className="space-y-3">
            <h3 className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground mb-1">Medio de Transporte</h3>
            <div className="grid grid-cols-2 gap-3">
              {TRANSPORT_OPTIONS.map(({ mode, label, icon: Icon }) => (
                <button
                  key={mode}
                  onClick={() => opt.setTransportMode(mode)}
                  className={cn("flex flex-col items-center justify-center p-3 rounded-xl border-2 transition-all", opt.transportMode === mode ? "border-border bg-primary text-primary-foreground shadow-[4px_4px_0px_var(--color-border)]" : "border-border bg-background text-muted-foreground hover:border-zinc-300 dark:hover:border-zinc-700")}
                >
                  <Icon className="h-6 w-6 mb-2"/>
                  <span className="text-xs font-semibold">{label}</span>
                </button>
              ))}
            </div>
          </section>

          {/* RN-07: Stops Limit */}
          <section className="space-y-3">
            <h3 className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground mb-2 px-1">Costos de Logística</h3>
            <div className="bg-background border border-border rounded-xl p-3 shadow-sm">
              <div className="flex justify-between mb-2">
                <span className="text-xs font-medium">Máximo de paradas:</span>
                <span className="text-xs font-bold text-foreground">{opt.maxStops}</span>
              </div>
              <input
                type="range"
                min="1" max="3" step="1"
                value={opt.maxStops}
                onChange={(e) => opt.setMaxStops(parseInt(e.target.value))}
                className="w-full accent-black dark:accent-white"
              />
            </div>
          </section>

          {opt.isCalculating ? (
            <div className="space-y-4 pt-2">
              <Skeleton className="h-[72px] w-full rounded-xl" />
              <div className="space-y-2">
                <Skeleton className="h-3 w-32 rounded mb-2" />
                <Skeleton className="h-[120px] w-full rounded-xl" />
              </div>
              <Skeleton className="h-11 w-full rounded-xl mt-4" />
            </div>
          ) : (
            <>
              {/* RN-20: monto mínimo */}
              {!opt.isValidAmount && (
                <RuleAlert icon={AlertTriangle} title="Monto Mínimo (RN-20)">
                  Tu compra de ${opt.productCost.toLocaleString()} no supera el mínimo de ${opt.minAmount.toLocaleString()} para justificar esta ruta.
                </RuleAlert>
              )}

              {/* RN-08: umbral de rentabilidad */}
              {opt.isValidAmount && !opt.isProfitable && (
                <RuleAlert icon={TrendingDown} title="Micro-ahorro Detectado (RN-08)">
                  El ahorro proyectado (${opt.savings.toLocaleString()}) no supera el umbral de rentabilidad (${opt.rentabilityThreshold.toLocaleString()}) por el costo de desplazamiento. Sugerimos agrupar en menos tiendas.
                </RuleAlert>
              )}

              {/* RN-00: Cost Function Breakdown */}
              <section className="space-y-2">
                <h3 className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground mb-2 px-1">Función de Costo Total</h3>
                <div className="bg-background border border-border rounded-xl p-3 shadow-sm">
                  <div className="space-y-1.5 mb-2">
                    <CostRow label="Precio Productos" value={`$${opt.productCost.toLocaleString()}`} />
                    <CostRow label="Costo Desplazamiento" value={`+ $${opt.logisticCost.toLocaleString()}`} valueClassName="text-primary" />
                    <div className="border-t border-border mt-2 pt-2 flex justify-between items-center">
                      <span className="font-semibold text-foreground text-sm">Costo Objetivo Total</span>
                      <span className="font-bold text-foreground text-base">${opt.totalCost.toLocaleString()}</span>
                    </div>
                  </div>

                  {opt.isValidAmount && opt.isProfitable && (
                    <div className="bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] mt-3 p-3 rounded-xl flex items-start gap-2">
                      <CheckCircle2 className="h-4 w-4 mt-0.5 shrink-0"/>
                      <div>
                        <p className="text-sm font-bold">Ruta Rentable</p>
                        <p className="text-xs mt-0.5">El ahorro en productos (${opt.savings.toLocaleString()}) supera el costo de traslado.</p>
                      </div>
                    </div>
                  )}
                </div>
              </section>

              {/* Action Button */}
              <button
                disabled={!opt.isValidAmount || !opt.isProfitable}
                className="w-full mt-2 flex items-center justify-center gap-2 bg-primary hover:brightness-110 disabled:bg-muted disabled:text-muted-foreground text-primary-foreground py-2.5 px-3 rounded-xl text-sm font-bold transition-all shadow-[4px_4px_0px_var(--color-border)] border-2 border-border disabled:shadow-none active:translate-y-1 active:shadow-none"
              >
                <Navigation className="h-5 w-5"/>
                Iniciar Ruta Turn-by-Turn
              </button>
            </>
          )}
        </div>
      </div>

      {/* Right Panel: Map Area */}
      <div className="flex-1 min-h-[45vh] md:h-full relative bg-muted z-0">
        <MapContainer
          center={[opt.centerLat, opt.centerLng]}
          zoom={14}
          scrollWheelZoom={true}
          className="h-full w-full"
          zoomControl={false}
        >
          <TileLayer
            url="https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png"
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
          />

          {opt.activeStores.map((store, index) => (
            <Marker key={store.id} position={[store.lat, store.lng]}>
              <Popup className="rounded-xl font-sans">
                <div className="text-center pb-1">
                  <div className="font-bold text-sm mb-1">{store.name}</div>
                  <div className="text-xs text-foreground font-semibold mb-2" style={{ color: store.color }}>
                    Parada {index + 1} • Gasto: ${store.cost.toLocaleString()}
                  </div>
                </div>
              </Popup>
            </Marker>
          ))}

          {opt.activeStores.length > 1 && (
            <Polyline
              positions={polylinePositions}
              color="#10b981"
              weight={4}
              dashArray="8, 8"
              opacity={0.8}
            />
          )}
        </MapContainer>

        {/* Map Overlay info */}
        <div className="absolute bottom-6 left-1/2 -translate-x-1/2 bg-card/90 backdrop-blur shadow-lg border border-border px-4 py-2 rounded-full flex items-center gap-3 z-[1000] pointer-events-none">
          <MapPin className="h-4 w-4 text-foreground"/>
          <span className="text-sm font-semibold text-foreground">Plataforma Logística Temuco</span>
        </div>
      </div>
    </div>
  );
}
