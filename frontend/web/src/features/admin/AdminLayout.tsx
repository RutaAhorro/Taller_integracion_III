// Estructura del panel administrativo con navegación adaptable y cierre de sesión.
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { LogOut, Menu, PanelLeftClose, PanelLeftOpen, ShoppingBasket, X } from 'lucide-react';
import { ROUTES } from '@/app/routes';
import { useAuth } from '@/features/autenticacion/useAuth';
import type { AdminSection } from './admin-types';

type PropiedadesDisenoAdmin = {
  children: ReactNode;
};

const navegacion: { id: AdminSection; label: string; description: string }[] = [
  { id: 'users', label: 'Usuarios', description: 'Cuentas y permisos' },
  { id: 'sources', label: 'Fuentes', description: 'Cadenas y conexiones' },
];

export function AdminLayout({ children }: PropiedadesDisenoAdmin) {
  const [barraLateralAbierta, setBarraLateralAbierta] = useState(false);
  const [barraLateralContraida, setBarraLateralContraida] = useState(false);
  const { search: busquedaUrl } = useLocation();
  const navegar = useNavigate();
  const { logout: cerrarSesion, isLoggingOut: cerrandoSesion } = useAuth();
  const seccionCruda = new URLSearchParams(busquedaUrl).get('section');
  const seccionActiva: AdminSection = seccionCruda === 'sources' ? 'sources' : 'users';
  const referenciaBotonCerrar = useRef<HTMLButtonElement>(null);

  // Auditoría (a11y): Escape cierra el drawer móvil y se devuelve el foco.
  useEffect(() => {
    if (!barraLateralAbierta) return;
    referenciaBotonCerrar.current?.focus();
    const manejarTecla = (evento: KeyboardEvent) => {
      if (evento.key === 'Escape') setBarraLateralAbierta(false);
    };
    document.addEventListener('keydown', manejarTecla);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', manejarTecla);
      document.body.style.overflow = '';
    };
  }, [barraLateralAbierta]);

  return (
    <div className="min-h-dvh bg-[#f5f6f2] text-foreground">
      <div className="flex min-h-dvh">
        {barraLateralAbierta && (
          <button
            type="button"
            aria-label="Cerrar menú"
            className="fixed inset-0 z-30 bg-[#2a3324]/30 lg:hidden"
            onClick={() => setBarraLateralAbierta(false)}
          />
        )}

        <aside
          role="dialog"
          aria-modal={barraLateralAbierta ? true : undefined}
          aria-label="Menú de administración"
          className={`fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-[#dfe5dc] bg-[#263524] text-white transition-transform duration-200 lg:static lg:translate-x-0 ${
            barraLateralAbierta ? 'translate-x-0' : '-translate-x-full'
          } ${barraLateralContraida ? 'lg:w-24' : ''}`}
        >
          <div className="flex h-20 items-center justify-between border-b border-white/10 px-5">
            <Link to={ROUTES.ADMIN} aria-label="Ir al panel de administración" className={`flex items-center gap-2 shrink-0 ${barraLateralContraida ? 'lg:mx-auto' : ''}`}>
              <span className="flex size-10 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
                <ShoppingBasket className="size-5" aria-hidden="true" />
              </span>
              <span className={`text-lg font-semibold tracking-tight text-white ${barraLateralContraida ? 'lg:hidden' : ''}`}>
                RutaAhorro
              </span>
            </Link>
            <button ref={referenciaBotonCerrar} type="button" className="rounded-lg p-2 hover:bg-white/10 lg:hidden" aria-label="Cerrar menú" onClick={() => setBarraLateralAbierta(false)}>
              <X className="size-5" />
            </button>
          </div>

          <div className={`px-5 pb-4 pt-7 ${barraLateralContraida ? 'lg:px-3' : ''}`}>
            <p className={`mb-3 text-[11px] font-bold uppercase tracking-[0.18em] text-white/45 ${barraLateralContraida ? 'lg:hidden' : ''}`}>Administración</p>
            <nav className="space-y-2">
              {navegacion.map((elemento) => {
                const estaActiva = seccionActiva === elemento.id;
                const enlace = elemento.id === 'users' ? ROUTES.ADMIN : `${ROUTES.ADMIN}?section=${elemento.id}`;
                return (
                  <Link
                    key={elemento.id}
                    to={enlace}
                    onClick={() => setBarraLateralAbierta(false)}
                    title={barraLateralContraida ? elemento.label : undefined}
                    className={`flex items-center gap-3 rounded-xl px-3 py-3 transition-colors ${
                      estaActiva ? 'bg-[#d4a373] text-[#263524]' : 'text-white/75 hover:bg-white/10 hover:text-white'
                    } ${barraLateralContraida ? 'lg:justify-center lg:px-2' : ''}`}
                  >
                    <span className="flex size-9 items-center justify-center rounded-lg bg-white/10 text-sm font-bold">{elemento.label.charAt(0)}</span>
                    <span className={`${barraLateralContraida ? 'lg:hidden' : ''}`}>
                      <span className="block text-sm font-bold">{elemento.label}</span>
                      <span className="block text-xs opacity-60">{elemento.description}</span>
                    </span>
                  </Link>
                );
              })}
            </nav>
          </div>

          <div className={`mt-auto border-t border-white/10 p-5 ${barraLateralContraida ? 'lg:p-3' : ''}`}>
            <div className={`flex items-center gap-3 ${barraLateralContraida ? 'lg:justify-center' : ''}`}>
              <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-[#8e6c8a] text-xs font-bold">AD</span>
              <div className={`min-w-0 ${barraLateralContraida ? 'lg:hidden' : ''}`}>
                <p className="truncate text-sm font-bold">Administrador</p>
                <p className="truncate text-xs text-white/55">Panel interno</p>
              </div>
            </div>
          </div>
        </aside>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex h-20 items-center justify-between border-b border-[#dfe5dc] bg-white/80 px-4 backdrop-blur md:px-8">
            <div className="flex items-center gap-3">
              <button type="button" className="rounded-lg border border-border p-2 lg:hidden" aria-label="Abrir menú" onClick={() => setBarraLateralAbierta(true)}>
                <Menu className="size-5" />
              </button>
              <button
                type="button"
                className="hidden rounded-lg border border-border p-2 text-muted-foreground hover:bg-secondary lg:block"
                aria-label={barraLateralContraida ? 'Expandir menú' : 'Contraer menú'}
                onClick={() => setBarraLateralContraida((contraida) => !contraida)}
              >
                {barraLateralContraida ? <PanelLeftOpen className="size-5" /> : <PanelLeftClose className="size-5" />}
              </button>
              <div>
                <p className="text-xs font-bold uppercase tracking-[0.15em] text-muted-foreground">Panel de control</p>
                <h1 className="text-lg font-bold md:text-xl">Administración</h1>
              </div>
            </div>
            <button
              type="button"
              disabled={cerrandoSesion}
              onClick={async () => {
                await cerrarSesion();
                navegar(ROUTES.LOGIN);
              }}
              className="flex items-center gap-2 text-sm font-bold text-destructive hover:underline"
            >
              <LogOut className="size-4" aria-hidden="true" />
              {cerrandoSesion ? 'Cerrando sesión…' : 'Cerrar sesión'}
            </button>
          </header>
          <main className="flex-1 p-4 md:p-8">{children}</main>
        </div>
      </div>
    </div>
  );
}
