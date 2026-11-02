import { useState, useEffect } from 'react';
import { supermarkets, products } from '@/data/mock';

export interface OptimizedStore {
  id: string;
  name: string;
  lat: number;
  lng: number;
  color: string;
  cost: number;
}

export type TransportMode = 'car' | 'bus';

const CENTER_LAT = -38.7359;
const CENTER_LNG = -72.5904;

const MIN_AMOUNT = 5000;
const RENTABILITY_THRESHOLD = 500;

// Tiendas de ejemplo (mock) con el gasto estimado de sus productos.
const ALL_STORES: OptimizedStore[] = supermarkets.map(s => {
  const cost = products
    .filter(p => p.supermarketId === s.id)
    .reduce((acc, p) => acc + p.price, 0);
  return {
    id: s.id,
    name: s.name,
    lat: s.coords[0],
    lng: s.coords[1],
    color: s.color, // CSS puro, no Tailwind dinámico
    cost: cost || Math.floor(Math.random() * 10000) + 2000
  };
});

const logisticCostFor = (stops: number, mode: TransportMode) =>
  mode === 'car' ? stops * 250 : 800 + stops * 50;

export function useRouteOptimization(initialStops = 3, initialMode: TransportMode = 'car') {
  const [maxStops, setMaxStops] = useState(initialStops);
  const [transportMode, setTransportMode] = useState<TransportMode>(initialMode);
  const [activeStores, setActiveStores] = useState<OptimizedStore[]>([]);
  const [isCalculating, setIsCalculating] = useState(true);

  // Simula la latencia del motor de rutas mientras no exista el endpoint real.
  useEffect(() => {
    setIsCalculating(true);
    const timer = setTimeout(() => {
      setActiveStores(ALL_STORES.slice(0, maxStops));
      setIsCalculating(false);
    }, 800);
    return () => clearTimeout(timer);
  }, [maxStops, transportMode]);

  // Costos derivados de las tiendas activas: no necesitan estado propio.
  const productCost = activeStores.reduce((acc, store) => acc + store.cost, 0);
  const logisticCost = activeStores.length ? logisticCostFor(activeStores.length, transportMode) : 0;
  const totalCost = productCost + logisticCost;
  // Ahorro frente a comprar todo en una sola tienda (estimado +25%).
  const savings = productCost * 1.25 - totalCost;

  return {
    maxStops,
    setMaxStops,
    transportMode,
    setTransportMode,
    activeStores,
    isCalculating,
    productCost,
    logisticCost,
    totalCost,
    savings,
    isValidAmount: productCost >= MIN_AMOUNT,
    isProfitable: savings >= RENTABILITY_THRESHOLD,
    minAmount: MIN_AMOUNT,
    rentabilityThreshold: RENTABILITY_THRESHOLD,
    centerLat: CENTER_LAT,
    centerLng: CENTER_LNG
  };
}
