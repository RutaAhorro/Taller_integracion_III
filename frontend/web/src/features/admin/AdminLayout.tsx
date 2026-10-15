import { useState, type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Menu, PanelLeftClose, PanelLeftOpen, ShieldCheck, X } from 'lucide-react';
import { ROUTES } from '@/core/routes';
import type { AdminSection } from './admin-mock-data';

type AdminLayoutProps = {
  children: ReactNode;
};

const navigation: { id: AdminSection; label: string; description: string }[] = [
  { id: 'overview', label: 'Resumen', description: 'Vista general' },
  { id: 'users', label: 'Usuarios', description: 'Cuentas y permisos' },
  { id: 'sources', label: 'Fuentes', description: 'Cadenas y conexiones' },
];

export function AdminLayout({ children }: AdminLayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const { search } = useLocation();
  const activeSection = (new URLSearchParams(search).get('section') as AdminSection) || 'overview';

  return (
    <div className="min-h-dvh bg-[#f5f6f2] text-foreground">
      <div className="flex min-h-dvh">
        {sidebarOpen && (
          <button
            type="button"
            aria-label="Cerrar menú"
            className="fixed inset-0 z-30 bg-[#2a3324]/30 lg:hidden"
            onClick={() => setSidebarOpen(false)}
          />
        )}

        <aside
          className={`fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-[#dfe5dc] bg-[#263524] text-white transition-transform duration-200 lg:static lg:translate-x-0 ${
            sidebarOpen ? 'translate-x-0' : '-translate-x-full'
          } ${sidebarCollapsed ? 'lg:w-24' : ''}`}
        >
          <div className="flex h-20 items-center justify-between border-b border-white/10 px-5">
            <Link to={ROUTES.ADMIN} className={`flex items-center gap-3 ${sidebarCollapsed ? 'lg:mx-auto' : ''}`}>
              <span className="flex size-10 items-center justify-center rounded-xl bg-[#d4a373] text-[#263524]">
                <ShieldCheck className="size-5" />
              </span>
              <span className={`font-serif text-xl font-bold ${sidebarCollapsed ? 'lg:hidden' : ''}`}>RutaAhorro</span>
            </Link>
            <button type="button" className="rounded-lg p-2 hover:bg-white/10 lg:hidden" aria-label="Cerrar menú" onClick={() => setSidebarOpen(false)}>
              <X className="size-5" />
            </button>
          </div>

          <div className={`px-5 pb-4 pt-7 ${sidebarCollapsed ? 'lg:px-3' : ''}`}>
            <p className={`mb-3 text-[11px] font-bold uppercase tracking-[0.18em] text-white/45 ${sidebarCollapsed ? 'lg:hidden' : ''}`}>Administración</p>
            <nav className="space-y-2">
              {navigation.map((item) => {
                const isActive = activeSection === item.id;
                const href = item.id === 'overview' ? ROUTES.ADMIN : `${ROUTES.ADMIN}?section=${item.id}`;
                return (
                  <Link
                    key={item.id}
                    to={href}
                    onClick={() => setSidebarOpen(false)}
                    title={sidebarCollapsed ? item.label : undefined}
                    className={`flex items-center gap-3 rounded-xl px-3 py-3 transition-colors ${
                      isActive ? 'bg-[#d4a373] text-[#263524]' : 'text-white/75 hover:bg-white/10 hover:text-white'
                    } ${sidebarCollapsed ? 'lg:justify-center lg:px-2' : ''}`}
                  >
                    <span className="flex size-9 items-center justify-center rounded-lg bg-white/10 text-sm font-bold">{item.label.charAt(0)}</span>
                    <span className={`${sidebarCollapsed ? 'lg:hidden' : ''}`}>
                      <span className="block text-sm font-bold">{item.label}</span>
                      <span className="block text-xs opacity-60">{item.description}</span>
                    </span>
                  </Link>
                );
              })}
            </nav>
          </div>

          <div className={`mt-auto border-t border-white/10 p-5 ${sidebarCollapsed ? 'lg:p-3' : ''}`}>
            <div className={`flex items-center gap-3 ${sidebarCollapsed ? 'lg:justify-center' : ''}`}>
              <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-[#8e6c8a] text-xs font-bold">AD</span>
              <div className={`min-w-0 ${sidebarCollapsed ? 'lg:hidden' : ''}`}>
                <p className="truncate text-sm font-bold">Administrador</p>
                <p className="truncate text-xs text-white/55">Panel interno</p>
              </div>
            </div>
          </div>
        </aside>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex h-20 items-center justify-between border-b border-[#dfe5dc] bg-white/80 px-4 backdrop-blur md:px-8">
            <div className="flex items-center gap-3">
              <button type="button" className="rounded-lg border border-border p-2 lg:hidden" aria-label="Abrir menú" onClick={() => setSidebarOpen(true)}>
                <Menu className="size-5" />
              </button>
              <button
                type="button"
                className="hidden rounded-lg border border-border p-2 text-muted-foreground hover:bg-secondary lg:block"
                aria-label={sidebarCollapsed ? 'Expandir menú' : 'Contraer menú'}
                onClick={() => setSidebarCollapsed((collapsed) => !collapsed)}
              >
                {sidebarCollapsed ? <PanelLeftOpen className="size-5" /> : <PanelLeftClose className="size-5" />}
              </button>
              <div>
                <p className="text-xs font-bold uppercase tracking-[0.15em] text-muted-foreground">Panel de control</p>
                <h1 className="text-lg font-bold md:text-xl">Administración</h1>
              </div>
            </div>
            <Link to="/" className="text-sm font-bold text-primary hover:underline">Volver al sitio</Link>
          </header>
          <main className="flex-1 p-4 md:p-8">{children}</main>
        </div>
      </div>
    </div>
  );
}
