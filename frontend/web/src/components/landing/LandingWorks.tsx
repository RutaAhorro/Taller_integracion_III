import { Search, Map, PiggyBank } from "lucide-react";
import landingData from "@/data/landing.json";

export function LandingHowItWorks() {
  const getIcon = (i: number) => {
    if (i === 0) return <Search size={18} aria-hidden="true" />;
    if (i === 1) return <Map size={18} aria-hidden="true" />;
    return <PiggyBank size={18} aria-hidden="true" />;
  };

  return (
    <section id="como-funciona" className="py-[3.5rem] md:py-[5rem] bg-card border-y-2 border-border">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="max-w-[680px] mx-auto mb-[3.5rem] text-center">
          <span className="inline-block text-[0.6875rem] font-bold tracking-[0.18em] uppercase text-brand-dark px-[0.85rem] py-[0.35rem] bg-[var(--landing-amber-soft)] border-2 border-[var(--landing-amber)] rounded-full mb-[1rem]">Metodología</span>
          <h2 className="font-['Fraunces'] text-[1.75rem] md:text-[clamp(1.75rem,3.2vw,2.4rem)] font-bold text-foreground mb-[0.875rem] leading-[1.15]">No es magia, es optimización matemática</h2>
          <p className="text-muted-foreground text-[1rem] leading-[1.6]">
            Unimos datos de precios con motores de ruteo para entregarte un
            análisis de costos real.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-[1.75rem]">
          {landingData.steps.map((s, index) => {
            const numBgClass = index === 0 ? 'bg-[var(--landing-olive-soft)] border-[var(--landing-olive)] text-[#5E7250]' : index === 1 ? 'bg-[var(--landing-amber-soft)] border-[var(--landing-amber)]' : 'bg-[var(--landing-malva-soft)] border-[var(--landing-malva)]';
            return (
            <div key={s.title} className="bg-card border-2 border-border rounded-[var(--radius)] p-[2rem_1.75rem] shadow-[6px_6px_0_var(--color-border)] transition-all duration-[0.25s] ease-in hover:-translate-x-[2px] hover:-translate-y-[2px] hover:shadow-[10px_10px_0_var(--color-border)]">
              <div className="flex items-center gap-3 mb-[1.25rem]">
                <div className={`w-[52px] h-[52px] rounded-[calc(var(--radius)-2px)] border-2 flex items-center justify-center font-['Fraunces'] text-[1.75rem] font-bold text-[var(--brand-dark)] shrink-0 ${numBgClass}`}>
                  {s.num}
                </div>
                <span className="text-[var(--brand-dark)] opacity-50">{getIcon(index)}</span>
              </div>
              <h3 className="font-bold text-[1.0625rem] text-foreground mb-[0.5rem]">{s.title}</h3>
              <p className="text-[0.875rem] text-muted-foreground leading-[1.6]">{s.desc}</p>
            </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
