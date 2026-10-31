import { Link } from "react-router-dom";
import { buttonVariants } from "@/components/ui/button";

export function LandingCTA() {
  return (
    <section className="py-[2.5rem] md:py-[3.5rem] px-0 md:px-0 pt-0">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="relative overflow-hidden rounded-[var(--radius)] p-[2.5rem_1.5rem] md:p-[3.5rem_3rem] bg-gradient-to-br from-brand-dark-2 to-brand-dark text-primary-foreground border-2 border-border shadow-[10px_10px_0_var(--color-border)] text-center">
          <div className="absolute inset-0 pointer-events-none" style={{ background: "repeating-linear-gradient(45deg, transparent 0 40px, oklch(1 0 0 / 0.04) 40px 41px)" }} />
          
          <h2 className="font-['Fraunces'] text-[1.75rem] md:text-[clamp(1.75rem,3vw,2.4rem)] mb-[0.875rem] relative text-primary-foreground font-bold">Tu hogar merece compras más inteligentes</h2>
          <p className="text-[1rem] opacity-85 mb-[2rem] max-w-[560px] mx-auto relative leading-[1.6]">
            Crea tu cuenta gratis y descubre cuánto puedes ahorrar esta semana.
            Sin tarjetas, sin compromisos.
          </p>
          <div className="relative flex flex-col md:flex-row gap-[0.875rem] justify-center flex-wrap">
            <Link
              className={buttonVariants({ size: "lg" }) + " w-full md:w-auto bg-secondary text-secondary-foreground hover:bg-secondary/90"}
              to="/login"
              state={{ tab: "signup" }}
            >Crear cuenta gratis</Link>
            <Link
              className={buttonVariants({ variant: "outline", size: "lg" }) + " w-full md:w-auto !bg-transparent !text-white !border-white/40 hover:!bg-white/10"}
              to="/dashboard"
            >Explorar como invitado</Link>
          </div>
        </div>
      </div>
    </section>
  );
}
