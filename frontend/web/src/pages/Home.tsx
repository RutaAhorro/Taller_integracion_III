// Presenta la aplicación y una muestra real del catálogo, sin estadísticas ni ahorros simulados.
import { useEffect, useState } from 'react';
import { getProducts, getMarketsFromProducts } from '@/lib/products-api';
import type { UiProduct } from '@/types';
import { LandingNavbar } from "@/components/landing/LandingNavbar";
import { LandingHero } from "@/components/landing/LandingHero";
import { LandingHowItWorks } from "@/components/landing/LandingWorks";
import { LandingFeatures } from "@/components/landing/LandingFeatures";
import { LandingStores } from "@/components/landing/LandingStores";
import { LandingCTA } from "@/components/landing/LandingCTA";
import { LandingFooter } from "@/components/landing/LandingFooter";

export default function Home() {
  const [products, setProducts] = useState<UiProduct[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Una sola página alimenta productos y cadenas; SUP-371 aportará la cobertura completa.
  useEffect(() => {
    const controller = new AbortController();
    getProducts('?limit=100&en_stock=true', controller.signal)
      .then(({ products }) => {
        if (!controller.signal.aborted) setProducts(products);
      })
      .catch(() => {
        if (!controller.signal.aborted) setError('No se pudo consultar el catálogo.');
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoading(false);
      });
    return () => controller.abort();
  }, []);

  return (
    <div 
      className="min-h-screen w-full overflow-x-clip bg-background text-foreground"
      style={{
        backgroundImage: "repeating-linear-gradient(90deg, oklch(0.618 0.078 65.5 / 0.025) 0 2px, transparent 2px 8px), repeating-linear-gradient(0deg, oklch(0.618 0.078 65.5 / 0.02) 0 2px, transparent 2px 8px)"
      }}
    >
      <LandingNavbar />
      <LandingHero products={products.filter(product => Number.isFinite(product.price) && product.price > 0).slice(0, 3)} isLoading={isLoading} error={error} />
      <LandingHowItWorks />
      <LandingFeatures />
      <LandingStores markets={getMarketsFromProducts(products)} isLoading={isLoading} error={error} />
      <LandingCTA />
      <LandingFooter />
    </div>
  );
}
