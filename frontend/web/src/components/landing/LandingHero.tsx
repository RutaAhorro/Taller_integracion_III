import { Link } from "react-router-dom";
import { ArrowRight, Leaf, Lock, MapPin, ShoppingBasket, Car, PiggyBank } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import landingData from "@/data/landing.json";

export function LandingHero() {
  return (
    <section className="relative py-20 overflow-hidden">
      {/* Deco orbs pastel eliminadas a petición del usuario */}

      <div className="max-w-[1300px] mx-auto px-4 md:px-6 grid grid-cols-1 md:grid-cols-[1.1fr_1fr] gap-[3rem] md:gap-[4rem] items-center relative z-10">
        {/* Columna izquierda */}
        <div>
          <h1 className="font-['Fredoka'] italic text-[2.25rem] md:text-[clamp(2.25rem,4.5vw,3.5rem)] font-bold text-foreground leading-[1.1] md:leading-[1.08] mb-4 md:mb-6 tracking-[-0.01em]">
            {/* Fix #22: box-decoration-break:clone para que el resaltado no se corte en móvil */}
            Compra para tu hogar <em className="italic text-brand-dark bg-gradient-to-b from-transparent from-60% to-[var(--landing-amber)] to-60% px-[0.15em] rounded-sm [box-decoration-break:clone]">sin pagar de más</em> por el traslado.
          </h1>

          <p className="text-[1.0625rem] leading-[1.65] text-muted-foreground max-w-[34rem] mb-8">
            Reunimos los precios de tus supermercados, calculamos el costo real de
            desplazarte y te mostramos la alternativa que <strong className="font-bold text-foreground">de verdad</strong> te
            conviene. Sin hojas de cálculo, sin recorrer media ciudad, sin sorpresas.
          </p>

          <div className="flex flex-col sm:flex-row gap-[0.875rem] mb-8">
            <Link className={buttonVariants({ size: "lg" }) + " w-full sm:w-auto justify-center"} to="/dashboard">
              Comenzar gratis <ArrowRight className="ml-2 w-4 h-4" aria-hidden="true" />
            </Link>
            <a className={buttonVariants({ variant: "outline", size: "lg" }) + " w-full sm:w-auto justify-center"} href="#como-funciona">Ver cómo funciona</a>
          </div>

          {/* Fix #26: apiladas en móvil, en fila desde sm */}
          <div className="flex flex-col sm:flex-row gap-3 sm:gap-5 text-[0.78125rem] text-muted-foreground font-semibold">
            <span className="inline-flex items-center gap-[0.4rem]"><Lock size={14} aria-hidden="true" /> No guardamos tu ubicación</span>
            <span className="inline-flex items-center gap-[0.4rem]"><Leaf size={14} aria-hidden="true" /> Datos verificados en tiempo real</span>
            <span className="inline-flex items-center gap-[0.4rem]"><MapPin size={14} aria-hidden="true" /> Hecho en Chile</span>
          </div>
        </div>

        {/* Columna derecha: mockup */}
        <div className="relative max-w-[360px] md:max-w-none mx-auto w-full">
          {/* Floating chips */}
          {/* Fix #1 (chip viaje): ya posicionado arriba-izquierda, fuera de la tarjeta */}
          <div className="absolute z-10 px-4 py-3 bg-[var(--landing-amber-soft)] border-2 border-[var(--landing-amber)] rounded-[var(--radius)] shadow-[4px_4px_0_var(--border)] flex items-center gap-2 text-[0.78125rem] font-bold text-foreground -top-[10px] -left-[30px] animate-[float-y_4s_ease-in-out_infinite] hidden md:flex">
            <Car size={16} aria-hidden="true" /> Viaje: 2.1 km · 6 min
          </div>
          {/* Fix #1 (chip ahorro): movido a bottom:-14px para que no tape el precio total */}
          {/* Fix #2: ahorro corregido a $9.560 */}
          <div className="absolute z-10 px-4 py-3 bg-[var(--landing-olive-soft)] border-2 border-[var(--landing-olive)] rounded-[var(--radius)] shadow-[4px_4px_0_var(--border)] flex items-center gap-2 text-[0.78125rem] font-bold text-foreground bottom-[-14px] right-[24px] animate-[float-y_5s_ease-in-out_infinite_reverse] hidden md:flex">
            <PiggyBank size={16} aria-hidden="true" /> Ahorro neto: $9.560
          </div>

          <div className="relative z-[2] bg-card border-2 border-border rounded-[var(--radius)] p-4 md:p-6 shadow-[6px_6px_0_var(--color-border)] md:shadow-[8px_8px_0_var(--color-border)]">
            <div className="flex items-center justify-between mb-[1.125rem] pb-4 border-b-2 border-dashed border-border">
              {/* Fix #3: Fredoka → Fraunces */}
              <span className="font-['Fraunces'] text-base font-bold text-foreground">Tu lista de compras</span>
              <span className="text-[11px] font-bold px-2.5 py-1 rounded-full bg-[var(--landing-olive-soft)] border-2 border-[var(--landing-olive)] text-[var(--brand-dark)]">
                {/* Fix #53: aria-hidden en el punto decorativo */}
                <span aria-hidden="true">● </span>8 productos
              </span>
            </div>

            {landingData.heroProducts.map((p) => (
              <div key={p.name} className="flex items-center gap-3 p-[0.7rem] rounded-[calc(var(--radius)-4px)] mb-2 bg-muted border-2 border-transparent transition-all duration-150 hover:bg-secondary hover:border-border">
                <div className="w-[42px] h-[42px] rounded-[calc(var(--radius)-4px)] bg-card border-2 border-border flex items-center justify-center text-[1.25rem] shrink-0">
                  <ShoppingBasket size={20} aria-hidden="true" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="font-bold text-[0.8125rem] text-foreground whitespace-nowrap overflow-hidden text-ellipsis">{p.name}</div>
                  <div className="text-[0.6875rem] text-muted-foreground mt-[2px]">{p.meta}</div>
                </div>
                <div>
                  <div className="font-mono font-bold text-[0.875rem] text-brand-dark">{p.price}</div>
                  <div className="text-[0.6875rem] font-bold text-[var(--landing-olive)] text-right mt-[2px]">{p.save}</div>
                </div>
              </div>
            ))}

            <div className="mt-4 p-[1rem_1.125rem] rounded-[calc(var(--radius)-2px)] bg-gradient-to-br from-brand-dark to-brand text-primary-foreground border-2 border-brand-dark shadow-[4px_4px_0_var(--color-brand-dark)] flex items-center justify-between">
              <div>
                <div className="text-[0.6875rem] tracking-[0.08em] uppercase opacity-85 font-bold">Costo total estimado</div>
                <div className="text-[11px] opacity-75 mt-0.5">
                  Productos + desplazamiento
                </div>
              </div>
              <div className="font-mono text-2xl font-bold">$45.780</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
