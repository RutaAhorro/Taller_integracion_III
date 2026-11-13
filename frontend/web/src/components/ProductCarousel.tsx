// Carrusel horizontal para los productos recibidos, con controles de desplazamiento.
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef } from 'react'
import { ProductCard } from '@/components/ProductCard'
import type { UiProduct as Product } from '@/types'

export function ProductCarousel({ products }: { products: Product[] }) {
  const containerRef = useRef<HTMLDivElement>(null)

  const scroll = (direction: number) => {
    containerRef.current?.scrollBy({ left: direction * 280, behavior: 'smooth' })
  }

  if (products.length === 0) return null

  return (
    <section className="mt-8 border-t border-border pt-6" aria-label="Productos relacionados">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-xl font-bold">También te puede interesar</h2>
        <div className="flex gap-2">
          <button type="button" className="rounded-full border border-border p-2" onClick={() => scroll(-1)} aria-label="Productos anteriores">
            <ChevronLeft className="h-4 w-4" />
          </button>
          <button type="button" className="rounded-full border border-border p-2" onClick={() => scroll(1)} aria-label="Productos siguientes">
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div ref={containerRef} className="flex snap-x gap-4 overflow-x-auto pb-3 scrollbar-hide">
        {products.map((product) => (
          <div key={product.id} className="w-48 shrink-0 snap-start sm:w-56">
            <ProductCard product={product} />
          </div>
        ))}
      </div>
    </section>
  )
}
