// Carrusel horizontal para los productos recibidos, con controles de desplazamiento.
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef } from 'react'
import { ProductCard } from '@/features/catalogo/ProductCard'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'

export function ProductCarousel({ products }: { products: Producto[] }) {
  const referenciaContenedor = useRef<HTMLDivElement>(null)

  const desplazar = (direccion: number) => {
    referenciaContenedor.current?.scrollBy({ left: direccion * 280, behavior: 'smooth' })
  }

  if (products.length === 0) return null

  return (
    <section className="mt-8 border-t border-border pt-6" aria-label="Productos relacionados">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-xl font-bold">También te puede interesar</h2>
        <div className="flex gap-2">
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
            <ProductCard product={producto} />
          </div>
        ))}
      </div>
    </section>
  )
}
