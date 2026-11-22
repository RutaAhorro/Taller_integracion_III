// Reúne navegación y filtros; las cadenas proceden de productos reales y los permisos de la sesión.
import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'

import {
  ChevronDown,
  Flame,
  MapPin,
  Percent,
  Store,
  Tag,
  X,
  Bot,
  Map,
  Users,
  Home,
  Zap,
  ChefHat,
  type LucideIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useRole } from '@/features/autenticacion/useRole'
import type { UiSupermarket } from '@/features/catalogo/tipos'
import { PLAN_PREMIUM, type PlanUsuario } from '@/features/autenticacion/tipos'

type PropiedadesBarraLateral = {
  open: boolean
  onClose: () => void
  onResetFilters: () => void
  activeMarket: string | null
  availableMarkets: UiSupermarket[]
  onSelectCategory: (categoria: string) => void
  onSelectMarket: (id: string | null) => void
  activeCategory: string
  userPlan?: PlanUsuario
  activeCity?: string
}

const destacados = [
  { id: 'ofertas', label: 'Mejores ofertas', icon: Flame },
  { id: 'descuentos', label: 'Mayor descuento', icon: Percent },
  { id: 'marcas', label: 'Marcas blancas', icon: Tag },
]

const departamentos: { id: string; label: string; items: string[] }[] = [
  {
    id: 'frutas-verduras',
    label: 'Frutas y Verduras',
    items: ['Frutas', 'Verduras'],
  },
  {
    id: 'lacteos',
    label: 'Lácteos, Huevos y Refrigerados',
    items: ['Leches', 'Quesos', 'Yogures', 'Mantequillas', 'Huevos'],
  },
  {
    id: 'despensa',
    label: 'Despensa y Abarrotes',
    items: [
      'Arroz',
      'Legumbres',
      'Fideos',
      'Pastas',
      'Aceites',
      'Harinas',
      'Azúcares',
      'Conservas',
    ],
  },
  {
    id: 'panaderia',
    label: 'Panadería y Pastelería',
    items: ['Pan', 'Galletas', 'Tortas', 'Masas'],
  },
  {
    id: 'carnes',
    label: 'Carnes y Pescados',
    items: ['Vacuno', 'Pollo', 'Cerdo', 'Pescados', 'Mariscos'],
  },
  {
    id: 'bebidas',
    label: 'Bebidas, Aguas y Licores',
    items: [
      'Bebidas',
      'Jugos',
      'Aguas',
      'Cervezas',
      'Vinos',
    ],
  },
  {
    id: 'hogar',
    label: 'Electrodomésticos y Hogar',
    items: ['Electrodomésticos', 'Menaje', 'Limpieza', 'Aseo'],
  },
  {
    id: 'libreria',
    label: 'Librería y Oficina',
    items: ['Cuadernos', 'Papelería', 'Útiles'],
  },
]

const enlacesHerramientas = [
  { to: '/chat', label: 'Chat Inteligente', icon: Bot },
  { to: '/history', label: 'Listas y Rutas', icon: Map },
  { to: '/recetas', label: 'Recetas IA', icon: ChefHat },
  { to: '/colaborador', label: 'Misiones', icon: Users },
]

const claseEnlaceNavegacion = (activo: boolean, pesoFuente: string) =>
  `flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm ${pesoFuente} transition-colors ${
    activo
      ? 'text-primary-foreground bg-primary shadow-[4px_4px_0px_var(--color-border)] border-2 border-border font-bold hover:bg-primary/90'
      : 'text-foreground hover:bg-accent hover:text-accent-foreground'
  }`

function TituloSeccion({ children }: { children: string }) {
  return <p className="px-2 pb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{children}</p>
}

type PropiedadesBotonFiltro = { icon: LucideIcon; label: string; active: boolean; onClick: () => void; disabled?: boolean }

function BotonFiltro({ icon: Icono, label, active, onClick, disabled = false }: PropiedadesBotonFiltro) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
        active ? 'bg-accent text-accent-foreground font-bold' : 'text-foreground hover:bg-accent hover:text-accent-foreground'
      }`}
    >
      <Icono className={`h-4.5 w-4.5 ${active ? 'text-foreground' : 'text-muted-foreground'}`} aria-hidden="true" />
      {label}
    </button>
  )
}

export function SideBar({
  open,
  onClose,
  onResetFilters,
  activeMarket,
  availableMarkets,
  onSelectCategory,
  onSelectMarket,
  activeCategory,
  userPlan,
  activeCity
}: PropiedadesBarraLateral) {
  const [departamentoAbierto, setDepartamentoAbierto] = useState<string | null>(null)
  const referenciaPanel = useRef<HTMLElement>(null)
  const referenciaBotonCerrar = useRef<HTMLButtonElement>(null)
  const navegar = useNavigate()
  const { pathname } = useLocation()
  // SUP-256: Misiones solo colaborador o admin.
  const { canAccessColaborador } = useRole()

  const manejarSeleccionCategoria = (categoria: string, cerrarBarra = true) => {
    onSelectCategory(categoria)
    navegar('/dashboard')
    if (cerrarBarra) onClose()
  }

  // Mantiene el foco en el panel y lo devuelve al control de apertura.
  useEffect(() => {
    if (!open) return
    const focoAnterior = document.activeElement
    referenciaBotonCerrar.current?.focus()
    const manejarTecla = (evento: KeyboardEvent) => {
      if (evento.key === 'Escape') {
        evento.preventDefault()
        onClose()
      }
      if (evento.key !== 'Tab') return
      const controles = Array.from(referenciaPanel.current?.querySelectorAll<HTMLElement>(
        'button:not(:disabled), a[href], input:not(:disabled), [tabindex="0"]'
      ) ?? []).filter(elemento => !elemento.closest('[inert]'))
      const primero = controles[0]
      const ultimo = controles[controles.length - 1]
      if (evento.shiftKey && document.activeElement === primero) {
        evento.preventDefault()
        ultimo?.focus()
      } else if (!evento.shiftKey && document.activeElement === ultimo) {
        evento.preventDefault()
        primero?.focus()
      }
    }
    window.addEventListener('keydown', manejarTecla)
    return () => {
      window.removeEventListener('keydown', manejarTecla)
      if (focoAnterior instanceof HTMLElement && focoAnterior.isConnected) focoAnterior.focus()
    }
  }, [open, onClose])

  return (
    <>
      <div
        onClick={onClose}
        aria-hidden="true"
        className={`fixed inset-0 z-[9998] bg-foreground/40 backdrop-blur-sm transition-opacity duration-300 ${
          open ? 'opacity-100' : 'pointer-events-none opacity-0'
        }`}
      />

      <aside
        id="catalog-filters"
        ref={referenciaPanel}
        role="dialog"
        aria-modal={open ? true : undefined}
        aria-label="Menú de filtros"
        aria-hidden={!open}
        inert={!open}
        className={`fixed inset-y-0 left-0 z-[9999] flex w-[19rem] max-w-[85vw] flex-col border-r border-border bg-card shadow-2xl transition-transform duration-300 ease-out ${
          open ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <div className="flex items-center gap-2">
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <Store className="h-5 w-5" aria-hidden="true" />
            </span>
            <span className="text-base font-semibold tracking-tight">
              Explorar
            </span>
          </div>
          <Button
            ref={referenciaBotonCerrar}
            variant="ghost"
            size="icon"
            className="h-9 w-9 rounded-full"
            onClick={onClose}
            aria-label="Cerrar filtros"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </Button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-4 py-5">
          <Button
            variant="outline"
            onClick={() => {
              onResetFilters()
              navegar('/dashboard')
              onClose()
            }}
          >
            Limpiar filtros
          </Button>
          <nav aria-label="Navegación">
            <ul className="flex flex-col gap-1">
              <li>
                <Link
                  to="/dashboard"
                  onClick={() => {
                    onSelectCategory("")
                    onClose()
                  }}
                  className={claseEnlaceNavegacion(pathname === '/dashboard' && !activeCategory, 'font-bold')}
                >
                  <Home className="h-4.5 w-4.5" aria-hidden="true" />
                  Inicio (Ver todo)
                </Link>
              </li>
            </ul>
          </nav>

          <nav aria-label="Destacados">
            <TituloSeccion>Destacados</TituloSeccion>
            <ul className="flex flex-col gap-1">
              {destacados.map((destacado) => (
                <li key={destacado.id}>
                  <BotonFiltro
                    icon={destacado.icon}
                    label={destacado.label}
                    active={activeCategory === destacado.id}
                    onClick={() => manejarSeleccionCategoria(activeCategory === destacado.id ? '' : destacado.id)}
                  />
                </li>
              ))}
            </ul>
          </nav>

          <nav aria-label="Departamentos y categorías">
            <TituloSeccion>Departamentos y categorías</TituloSeccion>
            <ul className="flex flex-col gap-0.5">
              {departamentos.map((departamento) => {
                const expandido = departamentoAbierto === departamento.id
                return (
                  <li key={departamento.id}>
                    <button
                      type="button"
                      // Expandir no filtra: solo muestra las subcategorías.
                      onClick={() => setDepartamentoAbierto((anterior) => (anterior === departamento.id ? null : departamento.id))}
                      aria-expanded={expandido}
                      className="flex w-full items-center gap-2 rounded-lg px-3 py-2.5 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
                    >
                      <ChevronDown
                        className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ${
                          expandido ? 'rotate-0' : '-rotate-90'
                        }`}
                        aria-hidden="true"
                      />
                      <span className="text-pretty leading-snug">
                        {departamento.label}
                      </span>
                    </button>
                    <div
                      inert={!expandido}
                      className={`grid transition-all duration-200 ease-out ${
                        expandido
                          ? 'grid-rows-[1fr] opacity-100'
                          : 'grid-rows-[0fr] opacity-0'
                      }`}
                    >
                      <ul className="ml-6 overflow-hidden border-l border-border pl-3">
                        <li>
                          <button
                            type="button"
                            onClick={() => manejarSeleccionCategoria(departamento.label)}
                            className={`flex w-full items-center rounded-md px-3 py-2 text-sm transition-colors ${
                              activeCategory === departamento.label
                                ? 'bg-accent text-primary font-bold'
                                : 'text-foreground font-semibold hover:bg-accent hover:text-primary'
                            }`}
                          >
                            Ver todo en {departamento.label.split(',')[0]}
                          </button>
                        </li>
                        {departamento.items.map((subcategoria) => (
                          <li key={subcategoria}>
                            <button
                              type="button"
                              onClick={() => manejarSeleccionCategoria(activeCategory === subcategoria ? "" : subcategoria)}
                              className={`flex w-full items-center rounded-md px-3 py-2 text-sm transition-colors ${
                                activeCategory === subcategoria
                                  ? 'bg-accent text-primary font-bold'
                                  : 'text-muted-foreground hover:bg-accent hover:text-primary'
                              }`}
                            >
                              {subcategoria}
                            </button>
                          </li>
                        ))}
                      </ul>
                    </div>
                  </li>
                )
              })}
            </ul>
          </nav>

          <nav aria-label="Supermercados">
            <TituloSeccion>Supermercados</TituloSeccion>
            <p className="px-2 pb-2 text-xs text-muted-foreground">Cadenas de los productos cargados. El listado completo está pendiente de integración.</p>
            <ul className="flex flex-col gap-1">
              <li>
                <BotonFiltro
                  icon={Store}
                  label="Todos"
                  active={activeMarket === null}
                  onClick={() => {
                    onSelectMarket(null)
                    navegar('/dashboard')
                  }}
                />
              </li>
              {availableMarkets.map((supermercado) => (
                <li key={supermercado.id}>
                  <BotonFiltro
                    icon={Store}
                    label={supermercado.name}
                    active={activeMarket === supermercado.id}
                    onClick={() => {
                      onSelectMarket(activeMarket === supermercado.id ? null : supermercado.id)
                      navegar('/dashboard')
                    }}
                  />
                </li>
              ))}
            </ul>
          </nav>

          <nav aria-label="Asistente IA">
            <TituloSeccion>Herramientas IA</TituloSeccion>
            <ul className="flex flex-col gap-1">
              {enlacesHerramientas
                // SUP-256: Misiones solo colaborador o admin.
                .filter((enlace) => enlace.to !== '/colaborador' || canAccessColaborador)
                .map((enlace) => (
                  <li key={enlace.to}>
                    <Link to={enlace.to} onClick={onClose} className={claseEnlaceNavegacion(pathname.startsWith(enlace.to), 'font-medium')}>
                      <enlace.icon className="h-4.5 w-4.5" aria-hidden="true" />
                      {enlace.label}
                    </Link>
                  </li>
                ))}
            </ul>
          </nav>
        </div>

        <div className="border-t border-border px-4 py-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))] space-y-3">
          <Link
            to="/planes"
            onClick={onClose}
            className="flex items-center justify-between rounded-xl border-2 border-border bg-card p-3 shadow-sm transition-all hover:-translate-y-1 hover:shadow-[4px_4px_0px_var(--color-border)] group"
          >
            <div className="flex items-center gap-3">
              <div
                className={`flex h-8 w-8 items-center justify-center rounded-lg ${
                  userPlan === PLAN_PREMIUM
                    ? 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400'
                    : 'bg-muted text-muted-foreground'
                }`}
              >
                <Zap className="h-4 w-4" aria-hidden="true" />
              </div>
              <div className="flex flex-col text-left">
                <span className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground">
                  Tu Plan
                </span>
                <span
                  className={`text-sm font-bold ${
                    userPlan === PLAN_PREMIUM
                      ? 'text-amber-600 dark:text-amber-400'
                      : 'text-foreground'
                  }`}
                >
                  {userPlan === PLAN_PREMIUM ? 'Suscripción Premium' : 'Plan Básico'}
                </span>
              </div>
            </div>
            <span className="text-xs font-bold text-primary opacity-0 transition-opacity group-hover:opacity-100">
              Mejorar
            </span>
          </Link>

          <div className="flex items-center gap-2 rounded-lg bg-muted px-3 py-2.5 text-sm text-muted-foreground">
            <MapPin className="h-4 w-4 text-primary" aria-hidden="true" />
            Comparando en{' '}
            <span className="font-medium text-foreground">
              {activeCity || 'Tu Ciudad'}
            </span>
          </div>
        </div>
      </aside>
    </>
  )
}
