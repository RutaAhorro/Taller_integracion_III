import { Link } from "react-router-dom";
import { ArrowRight, MapPin, ShoppingBasket } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { formatPrice } from '@/lib/formatters';
import type { UiProduct } from '@/types';

type LandingHeroProps = { products: UiProduct[]; isLoading: boolean; error: string | null };

/** Muestra productos de la API como vista previa, sin fingir una lista personal o una ruta calculada. */
export function LandingHero({ products, isLoading, error }: LandingHeroProps) {
  return (
    <section className="relative py-20 overflow-hidden">

      <div className="max-w-[1300px] mx-auto px-4 md:px-6 grid grid-cols-1 md:grid-cols-[1.1fr_1fr] gap-[3rem] md:gap-[4rem] items-center relative z-10">
        <div>
          <h1 className="font-['Fredoka'] italic text-[2.25rem] md:text-[clamp(2.25rem,4.5vw,3.5rem)] font-bold text-foreground leading-[1.1] md:leading-[1.08] mb-4 md:mb-6 tracking-[-0.01em]">
            Compra para tu hogar <em className="italic text-brand-dark bg-gradient-to-b from-transparent from-60% to-[var(--landing-amber)] to-60% px-[0.15em] rounded-sm [box-decoration-break:clone]">sin pagar de más</em> por el traslado.
          </h1>

          <p className="text-[1.0625rem] leading-[1.65] text-muted-foreground max-w-[34rem] mb-8">
            Consulta productos de supermercados y organiza tu lista de compras.
            El asistente y el cálculo de rutas se están integrando para ayudarte a planificar.
          </p>

          <div className="flex flex-col sm:flex-row gap-[0.875rem] mb-8">
            <Link className={buttonVariants({ size: "lg" }) + " w-full sm:w-auto justify-center"} to="/dashboard">
              Comenzar gratis <ArrowRight className="ml-2 w-4 h-4" aria-hidden="true" />
            </Link>
            <a className={buttonVariants({ variant: "outline", size: "lg" }) + " w-full sm:w-auto justify-center"} href="#como-funciona">Ver cómo funciona</a>
          </div>

          <div className="flex flex-col sm:flex-row gap-3 sm:gap-5 text-[0.78125rem] text-muted-foreground font-semibold">
            <span className="inline-flex items-center gap-[0.4rem]"><ShoppingBasket size={14} aria-hidden="true" /> Productos del catálogo</span>
            <span className="inline-flex items-center gap-[0.4rem]"><MapPin size={14} aria-hidden="true" /> Hecho en Chile</span>
          </div>
        </div>

        <div className="relative max-w-[360px] md:max-w-none mx-auto w-full">
          <div className="relative z-[2] bg-card border-2 border-border rounded-[var(--radius)] p-4 md:p-6 shadow-[6px_6px_0_var(--color-border)] md:shadow-[8px_8px_0_var(--color-border)]">
            <div className="flex items-center justify-between mb-[1.125rem] pb-4 border-b-2 border-dashed border-border">
              <span className="font-['Fraunces'] text-base font-bold text-foreground">Productos del catálogo</span>
              <span className="text-[11px] font-bold px-2.5 py-1 rounded-full bg-[var(--landing-olive-soft)] border-2 border-[var(--landing-olive)] text-[var(--brand-dark)]">
                <span aria-hidden="true">● </span>Vista previa
              </span>
            </div>

            {isLoading ? <p role="status">Cargando productos…</p> : error ? <p role="alert">{error}</p> : products.length === 0 ? <p role="status">No hay productos disponibles para mostrar.</p> : products.map((p) => (
              <Link to={`/producto/${encodeURIComponent(p.id)}`} key={p.id} className="flex items-center gap-3 p-[0.7rem] rounded-[calc(var(--radius)-4px)] mb-2 bg-muted border-2 border-transparent transition-all duration-150 hover:bg-secondary hover:border-border">
                <div className="w-[42px] h-[42px] rounded-[calc(var(--radius)-4px)] bg-card border-2 border-border flex items-center justify-center text-[1.25rem] shrink-0">
                  <ShoppingBasket size={20} aria-hidden="true" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="font-bold text-[0.8125rem] text-foreground whitespace-nowrap overflow-hidden text-ellipsis">{p.name}</div>
                  <div className="text-[0.6875rem] text-muted-foreground mt-[2px]">{p.supermarketName} · {p.unit}</div>
                </div>
                <div>
                  <div className="font-mono font-bold text-[0.875rem] text-brand-dark">{formatPrice(p.price)}</div>
                </div>
              </Link>
            ))}

            <p className="mt-4 text-xs text-muted-foreground">Los precios corresponden a capturas del catálogo; no representan un ahorro ni un costo de traslado calculado.</p>
          </div>
        </div>
      </div>
    </section>
  );
}
