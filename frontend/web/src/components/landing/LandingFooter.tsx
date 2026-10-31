import { Link } from "react-router-dom";
import { Coffee, Brain, ShoppingBasket } from "lucide-react";

export function LandingFooter() {
  return (
    <footer className="bg-[var(--brand-dark-2)] text-[oklch(0.85_0.02_83)] p-[2.5rem_0_1.25rem] mt-[1.5rem] border-t-[3px] border-border">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <div className="grid grid-cols-1 md:grid-cols-[2fr_1fr] gap-[1.5rem] md:gap-[2.25rem] mb-[2rem]">

          <div>
            <div className="flex items-center gap-2 mb-3 text-white font-['Fraunces'] font-bold text-lg">
              <ShoppingBasket size={20} className="text-[var(--landing-amber)]" aria-hidden="true" />
              RutaAhorro
            </div>
            <p className="text-[0.8125rem] opacity-70 max-w-[320px] leading-[1.6] mt-[0.875rem]">
              Optimizador de rutas y comparador de precios para supermercados en Temuco, Chile.
              Proyecto académico — Universidad Católica de Temuco.
            </p>
          </div>

          <div>
            <h4 className="font-['Fraunces'] text-[oklch(0.98_0.01_83)] text-[0.8125rem] font-bold tracking-[0.08em] uppercase mb-[1rem]">Producto</h4>
            <a href="#como-funciona" className="block text-[oklch(0.85_0.02_83)] no-underline text-[0.8125rem] mb-[0.625rem] transition-colors duration-200 opacity-80 hover:text-[oklch(0.98_0.01_83)] hover:opacity-100">Cómo funciona</a>
            <a href="#beneficios" className="block text-[oklch(0.85_0.02_83)] no-underline text-[0.8125rem] mb-[0.625rem] transition-colors duration-200 opacity-80 hover:text-[oklch(0.98_0.01_83)] hover:opacity-100">Beneficios</a>
            <a href="#ahorro" className="block text-[oklch(0.85_0.02_83)] no-underline text-[0.8125rem] mb-[0.625rem] transition-colors duration-200 opacity-80 hover:text-[oklch(0.98_0.01_83)] hover:opacity-100">Ahorro real</a>
            <Link to="/dashboard" className="block text-[oklch(0.85_0.02_83)] no-underline text-[0.8125rem] mb-[0.625rem] transition-colors duration-200 opacity-80 hover:text-[oklch(0.98_0.01_83)] hover:opacity-100">Catálogo</Link>
          </div>
        </div>

        <div className="pt-[1.25rem] border-t-2 border-[oklch(0.4_0.02_60)] flex flex-wrap justify-between gap-[0.75rem] text-[0.75rem] opacity-65">
          <span>© 2026 RutaAhorro · Todos los derechos reservados</span>
          <span className="inline-flex items-center gap-1.5">Hecho con <Coffee size={14} aria-hidden="true" /> y <Brain size={14} aria-hidden="true" /> en Temuco, Chile.</span>
        </div>
      </div>
    </footer>
  );
}
