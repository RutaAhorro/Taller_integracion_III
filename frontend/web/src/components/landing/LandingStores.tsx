import { Plus, Store } from "lucide-react";
import landingData from "@/data/landing.json";

export function LandingStores() {
  return (
    <section id="supermercados" className="py-[3.5rem] md:py-[5rem] bg-card border-y-2 border-border">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="max-w-[680px] mx-auto mb-[3.5rem] text-center">
          <span className="inline-block text-[0.6875rem] font-bold tracking-[0.18em] uppercase text-brand-dark px-[0.85rem] py-[0.35rem] bg-[var(--landing-amber-soft)] border-2 border-[var(--landing-amber)] rounded-full mb-[1rem]">Cobertura</span>
          <h2 className="font-['Fraunces'] text-[1.75rem] md:text-[clamp(1.75rem,3.2vw,2.4rem)] font-bold text-foreground mb-[0.875rem] leading-[1.15]">Los supermercados que ya conoces</h2>
          <p className="text-muted-foreground text-[1rem] leading-[1.6]">
            Estamos integrando continuamente nuevas cadenas y sucursales en la
            región para que tengas el panorama completo.
          </p>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-[repeat(auto-fit,minmax(170px,1fr))] gap-[0.75rem] md:gap-[1rem]">
          {landingData.stores.map((s, index) => {
            const isLast = index === landingData.stores.length - 1;
            return (
              <div key={s.name} className={`relative p-[1rem] md:p-[1.5rem] text-center border-2 rounded-[var(--radius)] shadow-[3px_3px_0_var(--color-border)] hover:-translate-x-[2px] hover:-translate-y-[2px] hover:shadow-[6px_6px_0_var(--color-border)] transition-all duration-[0.2s] ease bg-muted ${isLast ? 'border-dashed border-border opacity-70' : 'border-border'}`}>
                <div className="flex justify-center mb-[0.5rem] text-[1.5rem] md:text-[2rem] text-foreground">
                  {isLast ? <Plus size={32} aria-hidden="true" /> : <Store size={32} aria-hidden="true" />}
                </div>
                <div className="font-['Fraunces'] font-bold text-[1rem] text-foreground mb-[0.25rem]">{s.name}</div>
                <div className="text-[0.75rem] text-muted-foreground">{s.desc}</div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
