// Portada informativa con contenido editorial y ejemplos ilustrativos de landing.json, sin consultar la API.
import { LandingNavbar } from '@/components/landing/LandingNavbar';
import { LandingHero } from '@/components/landing/LandingHero';
import { LandingTrustStrip } from '@/components/landing/LandingTrustStrip';
import { LandingHowItWorks } from '@/components/landing/LandingWorks';
import { LandingFeatures } from '@/components/landing/LandingFeatures';
import { LandingSavings } from '@/components/landing/LandingSavings';
import { LandingStores } from '@/components/landing/LandingStores';
import { LandingCTA } from '@/components/landing/LandingCTA';
import { LandingFooter } from '@/components/landing/LandingFooter';

export default function Home() {
  return (
    <div
      className="min-h-screen w-full overflow-x-clip bg-background text-foreground"
      style={{
        backgroundImage: 'repeating-linear-gradient(90deg, oklch(0.618 0.078 65.5 / 0.025) 0 2px, transparent 2px 8px), repeating-linear-gradient(0deg, oklch(0.618 0.078 65.5 / 0.02) 0 2px, transparent 2px 8px)',
      }}
    >
      <LandingNavbar />
      <LandingHero />
      <LandingTrustStrip />
      <LandingHowItWorks />
      <LandingSavings />
      <LandingFeatures />
      <LandingStores />
      <LandingCTA />
      <LandingFooter />
    </div>
  );
}
