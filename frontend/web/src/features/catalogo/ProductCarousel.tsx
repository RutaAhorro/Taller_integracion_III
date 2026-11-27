// Carrusel horizontal para los productos recibidos, con controles de desplazamiento.
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef } from 'react'
import { ProductCard } from '@/features/catalogo/ProductCard'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'

type PropiedadesCarruselProductos = {
  products: Producto[]
  title?: string
  subtitle?: string
  ariaLabel?: string
  actionLabel?: string
  onAction?: () => void
  onAdd?: (producto: Producto) => void
  className?: string
}

export function ProductCarousel({
  products,
  title = 'También te puede interesar',
  subtitle,
  ariaLabel = 'Productos relacionados',
  actionLabel,
  onAction,
  onAdd,
  className = 'mt-8 border-t border-border pt-6',
}: PropiedadesCarruselProductos) {
  const referenciaContenedor = useRef<HTMLDivElement>(null)

  const desplazar = (direccion: number) => {
    referenciaContenedor.current?.scrollBy({ left: direccion * 280, behavior: 'smooth' })
  }

  if (products.length === 0) return null

  return (
    <section className={className} aria-label={ariaLabel}>
      <div className="mb-4 flex items-end justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-xl font-bold">{title}</h2>
          {subtitle && <p className="mt-0.5 text-sm text-muted-foreground">{subtitle}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {actionLabel && onAction && (
            <button
              type="button"
              className="rounded-full px-2 py-1 text-sm font-bold text-primary underline-offset-4 hover:underline"
              onClick={onAction}
            >
              {actionLabel}
            </button>
          )}
          <button type="button" className="rounded-full border border-border p-2" onClick={() => desplazar(-1)} aria-label="Productos anteriores">
            <ChevronLeft className="h-4 w-4" />
          </button>
          <button type="button" className="rounded-full border border-border p-2" onClick={() => desplazar(1)} aria-label="Productos siguientes">
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div ref={referenciaContenedor} className="flex snap-x gap-4 overflow-x-auto pb-3 scrollbar-hide">
        {products.map((producto) => (
          <div key={producto.id} className="w-48 shrink-0 snap-start sm:w-56">
            <ProductCard product={producto} onAdd={onAdd} />
          </div>
        ))}
      </div>
    </section>
  )
}
