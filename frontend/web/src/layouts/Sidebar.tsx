// Reúne navegación y filtros: categorías desde Catálogo y cadenas de los productos recibidos.
import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'

import {
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
import { obtenerCategorias, PREFIJO_CATEGORIA } from '@/features/catalogo/products-api'

// Mantiene la selección en el layout para compartirla con el listado paginado.
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

// ===== Navegación estática: no son categorías del catálogo =====
const destacados = [
  { id: 'ofertas', label: 'Mejores ofertas', icon: Flame },
  { id: 'descuentos', label: 'Mayor descuento', icon: Percent },
  { id: 'marcas', label: 'Marcas blancas', icon: Tag },
]

const enlacesHerramientas = [
  { to: '/chat', label: 'Chat Inteligente', icon: Bot },
  { to: '/history', label: 'Listas y Rutas', icon: Map },
  { to: '/recetas', label: 'Recetas IA', icon: ChefHat },
  { to: '/colaborador', label: 'Misiones', icon: Users },
]

// Comparte los estilos de navegación sin duplicar la indicación de selección.
const claseEnlaceNavegacion = (activo: boolean, pesoFuente: string) =>
  `flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm ${pesoFuente} transition-colors ${
    activo
      ? 'text-primary-foreground bg-primary shadow-[4px_4px_0px_var(--color-border)] border-2 border-border font-bold hover:bg-primary/90'
      : 'text-foreground hover:bg-accent hover:text-accent-foreground'
  }`

// Separa visualmente las secciones de filtros y navegación.
function TituloSeccion({ children }: { children: string }) {
  return <p className="px-2 pb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{children}</p>
}

// Describe el control común de destacados y supermercados.
type PropiedadesBotonFiltro = { icon: LucideIcon; label: string; active: boolean; onClick: () => void; disabled?: boolean }

// Conserva una presentación común para las opciones con icono.
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

// Carga categorías reales y ofrece los filtros sin mantener una clasificación fija.
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
  const [categorias, setCategorias] = useState<string[]>([])
  const [cargandoCategorias, setCargandoCategorias] = useState(true)
  const [errorCategorias, setErrorCategorias] = useState(false)
  const [intentoCategorias, setIntentoCategorias] = useState(0)
  const referenciaPanel = useRef<HTMLElement>(null)
  const referenciaBotonCerrar = useRef<HTMLButtonElement>(null)
  const navegar = useNavigate()
  const { pathname } = useLocation()
  // SUP-256: Misiones solo colaborador o admin.
  const { canAccessColaborador } = useRole()

  // Carga una lista completa por montaje/reintento, sin repetirla al paginar
  // o abrir el panel. Cancela al salir para no aplicar respuestas tardías.
  useEffect(() => {
    const controlador = new AbortController()
    setCargandoCategorias(true)
    setErrorCategorias(false)
    obtenerCategorias(controlador.signal)
      .then((resultado) => {
        if (!controlador.signal.aborted) setCategorias(resultado)
      })
      .catch(() => {
        if (!controlador.signal.aborted) {
          setCategorias([])
          setErrorCategorias(true)
        }
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargandoCategorias(false)
      })
    return () => controlador.abort()
  }, [intentoCategorias])

  // Lleva la selección al catálogo y libera el espacio del panel en móvil.
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

          <nav aria-label="Categorías">
            <TituloSeccion>Categorías</TituloSeccion>
            {cargandoCategorias && <p role="status" className="px-2 text-sm text-muted-foreground">Cargando categorías…</p>}
            {errorCategorias && (
              <div role="alert" className="px-2 text-sm">
                <p>No se pudieron cargar las categorías.</p>
                <button type="button" className="underline" onClick={() => setIntentoCategorias((anterior) => anterior + 1)}>Reintentar</button>
              </div>
            )}
            {!cargandoCategorias && !errorCategorias && categorias.length === 0 && (
              <p className="px-2 text-sm text-muted-foreground">No hay categorías disponibles.</p>
            )}
            <ul className="flex flex-col gap-0.5">
              {categorias.map((categoria) => {
                const seleccion = `${PREFIJO_CATEGORIA}${categoria}`
                const activa = activeCategory === seleccion
                return (
                  <li key={categoria}>
                    <button
                      type="button"
                      aria-pressed={activa}
                      onClick={() => manejarSeleccionCategoria(activa ? '' : seleccion)}
                      className={`flex w-full rounded-md px-3 py-2 text-left text-sm transition-colors ${
                        activa ? 'bg-accent text-primary font-bold' : 'text-muted-foreground hover:bg-accent hover:text-primary'
                      }`}
                    >
                      {categoria}
                    </button>
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
