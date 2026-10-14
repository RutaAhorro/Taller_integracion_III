import { Link } from "react-router-dom";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import landingData from "@/data/landing.json";

export function LandingSavings() {
  return (
    <section id="ahorro" className="py-[3.5rem] md:py-[5rem]">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-[2rem] md:gap-[3rem] items-center bg-card border-2 border-border rounded-[var(--radius)] p-[2rem] md:p-[3rem] shadow-[8px_8px_0_var(--color-border)]">
          <div>
            <span className="inline-block text-[0.6875rem] font-bold tracking-[0.18em] uppercase text-brand-dark px-[0.85rem] py-[0.35rem] bg-[var(--landing-amber-soft)] border-2 border-[var(--landing-amber)] rounded-full mb-[1rem]">Caso real</span>
            <h2 className="font-['Fredoka'] text-[1.75rem] md:text-[clamp(1.5rem,2.8vw,2rem)] font-bold text-foreground mb-[1rem] leading-[1.15]">El ahorro que sientes en el bolsillo</h2>
            <p className="text-muted-foreground mb-[1.5rem] leading-[1.6]">
              Una familia en Temuco compra los mismos 8 productos cada semana.
              Mira lo que pasa cuando considera el costo de traslado:
            </p>

            <ul className="list-none p-0 m-[0_0_1.5rem]">
              {landingData.savings.items.map((item) => (
                <li key={item} className="flex gap-[0.75rem] items-start mb-[0.875rem] text-[0.9375rem] text-foreground">
                  <span className="shrink-0 w-[24px] h-[24px] rounded-[6px] bg-[var(--landing-olive-soft)] border-2 border-[var(--landing-olive)] flex items-center justify-center text-[0.75rem] font-bold text-brand-dark mt-[1px]">✓</span>
                  <span>{item}</span>
                </li>
              ))}
            </ul>

            <Link className={buttonVariants({ size: "lg" }) + " w-full sm:w-auto"} to="/login" state={{ tab: "signup" }}>
              Quiero ver mi ahorro <ArrowRight className="ml-2 w-4 h-4" />
            </Link>
          </div>

          <div className="bg-gradient-to-br from-[var(--landing-amber-soft)] to-[var(--landing-terracotta-soft)] border-2 border-border rounded-[var(--radius)] p-[1.75rem] shadow-[4px_4px_0_var(--color-border)] mt-6 md:mt-0">
            {landingData.savings.bars.map((b) => {
              const bgGradient = b.variant === 'worst' ? 'bg-gradient-to-r from-[var(--landing-terracotta)] to-[oklch(0.7_0.1_40)]' : 
                                 b.variant === 'mid' ? 'bg-gradient-to-r from-[var(--landing-amber)] to-[oklch(0.78_0.09_75)]' : 
                                 'bg-gradient-to-r from-[var(--landing-olive)] to-[oklch(0.6_0.08_130)]';
              return (
              <div key={b.label} className="mb-[1.125rem]">
                <div className="flex justify-between text-[0.8125rem] font-bold mb-[0.4rem] text-foreground">
                  <span>{b.label}</span>
                  <span className="font-mono text-brand-dark">{b.value}</span>
                </div>
                <div className="h-[14px] rounded-full bg-card border-2 border-border overflow-hidden">
                  <div className={`h-full rounded-full transition-[width] duration-[0.6s] ease ${bgGradient}`} style={{ width: b.width }} />
                </div>
              </div>
              );
            })}

            <div className="mt-[1.25rem] pt-[1rem] border-t-2 border-dashed border-border flex flex-col sm:flex-row sm:justify-between sm:items-center gap-2">
              <span className="text-[0.8125rem] text-muted-foreground font-semibold">Ahorro neto semanal</span>
              <span className="font-mono text-[1.5rem] font-bold text-[var(--landing-olive)]">{landingData.savings.weeklySaving}</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
