// Franja de cifras ilustrativas de la propuesta comercial; no consulta estadísticas del sistema.
import datosPortada from '@/features/landing/landing.json';

export function LandingTrustStrip() {
  return (
    <section className="bg-[var(--brand-dark-2)] text-[oklch(0.93_0.02_83)] py-[1.75rem] border-y-[3px] border-border">
      <div className="max-w-[1300px] mx-auto px-4 md:px-6">
        <p className="mb-4 text-center text-xs opacity-75">Cifras ilustrativas de la propuesta, no estadísticas actuales de la plataforma.</p>
        <div className="flex justify-around flex-wrap gap-[1.5rem] text-center">
          {datosPortada.metrics.map((metrica, indice) => (
            <div key={indice} className="flex items-center gap-[0.75rem]">
              <div className="font-['Fredoka'] text-[1.75rem] font-bold text-[var(--landing-amber)] leading-none">{metrica.num}</div>
              <div className="text-[0.75rem] text-left leading-[1.35] opacity-85">
                {metrica.lines[0]}<br />{metrica.lines[1]}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
