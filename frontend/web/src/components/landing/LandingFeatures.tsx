import { Filter, Car, Bot, ShieldCheck, House, Gauge } from "lucide-react";
import landingData from "@/data/landing.json";

export function LandingFeatures() {
  const getIcon = (i: number) => {
    switch(i) {
      case 0: return <Filter size={24} aria-hidden="true" />;   // Comparación real
      case 1: return <Car size={24} aria-hidden="true" />;      // Costo de traslado
      case 2: return <Bot size={24} aria-hidden="true" />;      // Asistente IA
      case 3: return <ShieldCheck size={24} aria-hidden="true" />; // Privacidad
      case 4: return <House size={24} aria-hidden="true" />;    // Adaptado al hogar
      case 5: return <Gauge size={24} aria-hidden="true" />;    // Rápido y liviano
      default: return null;
    }
  };

  return (
    <section id="beneficios" className="py-[3.5rem] md:py-[5rem]">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="max-w-[680px] mx-auto mb-[3.5rem] text-center">
          <span className="inline-block text-[0.6875rem] font-bold tracking-[0.18em] uppercase text-brand-dark px-[0.85rem] py-[0.35rem] bg-[var(--landing-amber-soft)] border-2 border-[var(--landing-amber)] rounded-full mb-[1rem]">Características</span>
          <h2 className="font-['Fraunces'] text-[1.75rem] md:text-[clamp(1.75rem,3.2vw,2.4rem)] font-bold text-foreground mb-[0.875rem] leading-[1.15]">Todo lo que necesitas para ahorrar</h2>
          <p className="text-muted-foreground text-[1rem] leading-[1.6]">
            Diseñado para que gastes menos tiempo planificando y menos dinero
            comprando.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-[1.5rem]">
          {landingData.features.map((f, index) => {
            const bgClass = index === 0 ? 'bg-[var(--landing-amber-soft)]' : 
                            index === 1 ? 'bg-[var(--landing-sky-soft)]' : 
                            index === 2 ? 'bg-[var(--landing-malva-soft)]' : 
                            index === 3 ? 'bg-[var(--landing-olive-soft)]' : 
                            index === 4 ? 'bg-[var(--landing-terracotta-soft)]' : 
                            'bg-[var(--landing-amber-soft)]';
            
            return (
              <div key={f.title} className="relative bg-card border-2 border-border rounded-[var(--radius)] p-[1.75rem] transition-all duration-[0.25s] ease overflow-hidden hover:shadow-[6px_6px_0_var(--color-border)] hover:-translate-x-[2px] hover:-translate-y-[2px] group">
                <div className="absolute top-0 left-0 w-full h-[4px] bg-gradient-to-r from-brand to-[var(--landing-amber)] scale-x-0 origin-left transition-transform duration-300 ease group-hover:scale-x-100" />
                <div className={`w-[56px] h-[56px] rounded-[calc(var(--radius)-2px)] flex items-center justify-center text-[1.5rem] mb-[1.125rem] border-2 border-border shadow-[3px_3px_0_var(--color-border)] text-foreground ${bgClass}`}>
                  {getIcon(index)}
                </div>
                <h3 className="font-['Fraunces'] text-[1.0625rem] font-bold text-foreground mb-[0.5rem]">{f.title}</h3>
                <p className="text-[0.875rem] text-muted-foreground leading-[1.6]">{f.desc}</p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
