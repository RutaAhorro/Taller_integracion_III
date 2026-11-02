import { Store, Loader2 } from 'lucide-react'
import { useLayoutContext } from '@/layouts/MainLayout'
import { ProductCard } from '@/components/ProductCard'
import { ProductSkeleton } from '@/components/ui/ProductSkeleton'
import { Footer } from '@/components/Footer'
import type { UiProduct as Product } from '@/types'
import { useCart } from '@/contexts/CartContext'
import { useCatalog } from '@/hooks/useCatalog'

export default function Page() {
  const { query, activeMarket, category } = useLayoutContext()
  const { addToCart } = useCart()
  const catalog = useCatalog(query, category, activeMarket)

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
    if (scrollHeight - scrollTop <= clientHeight + 50) {
      catalog.fetchMore();
    }
  };

  const handleAdd = (product: Product) => {
    addToCart(product)
  }
  return (
    <div className="relative flex flex-1 w-full h-full bg-background overflow-hidden">
      {/* Fondo de imagen geométrica vibrante */}
      <div
        className="absolute inset-0 z-0 opacity-[0.60] bg-[url('/images/dashboard-seamless.jpg')] bg-cover bg-center bg-no-repeat pointer-events-none"
        aria-hidden="true"
      />
      <main className="relative z-10 mx-auto flex w-full max-w-[1600px] flex-1 flex-col gap-6 overflow-hidden px-4 py-6 md:px-6">
        {/* Left: product grid */}
        <section
          aria-label="Productos en oferta"
          className="flex min-h-0 flex-1 flex-col"
        >
          <div 
            className="min-h-0 flex-1 overflow-y-auto pb-2 pr-1 -mr-1 scrollbar-hide"
            onScroll={handleScroll}
          >
            <div className="relative mb-4 md:mb-6 flex flex-col justify-end overflow-hidden rounded-xl md:rounded-2xl border-2 border-border shadow-[2px_2px_0px_var(--color-border)] md:shadow-[4px_4px_0px_var(--color-border)] bg-card p-4 md:p-5 min-h-[90px] md:min-h-[140px]">
              <div
                className="absolute inset-0 z-0 opacity-[0.25] mix-blend-multiply bg-[url('/images/dashboard-bg.jpg')] bg-cover bg-center transition-opacity"
                aria-hidden="true"
              />
              <div className="absolute inset-0 z-0 bg-gradient-to-r from-card/80 via-card/50 to-transparent pointer-events-none" />
              <div className="relative z-10 flex items-end justify-between w-full">
                <div>
                  <h1 className="text-balance text-xl md:text-2xl font-extrabold tracking-tight text-foreground drop-shadow-sm">
                    Productos comparados en {catalog.supermarketCount}{' '}
                    {catalog.supermarketCount === 1 ? 'supermercado' : 'supermercados'}
                  </h1>
                  <p className="text-sm font-semibold text-muted-foreground mt-1">
                    Catálogo disponible cerca de ti
                  </p>
                </div>
              </div>
            </div>

            {catalog.isLoading ? (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 md:gap-4">
                {Array.from({ length: 12 }).map((_, i) => (
                  <ProductSkeleton key={i} />
                ))}
              </div>
            ) : catalog.error ? (
              <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
                <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                <p className="text-sm text-muted-foreground">{catalog.error}</p>
              </div>
            ) : catalog.filtered.length > 0 ? (
              <div className="flex flex-col gap-6 pb-6">
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 md:gap-4">
                  {catalog.filtered.map((p) => (
                    <ProductCard key={p.id} product={p} onAdd={handleAdd} />
                  ))}
                </div>
                {catalog.isFetchingMore && (
                  <div className="flex justify-center items-center py-6">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                    <span className="ml-2 text-sm font-medium text-muted-foreground">Cargando más productos...</span>
                  </div>
                )}
              </div>
            ) : (
              <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
                <Store
                  className="h-8 w-8 text-muted-foreground"
                  aria-hidden="true"
                />
                <p className="text-sm text-muted-foreground">
                  No encontramos productos para{' '}
                  <span className="font-medium text-foreground">
                    &ldquo;{query}&rdquo;
                  </span>
                </p>
              </div>
            )}
            <Footer />
          </div>
        </section>

      </main>

    </div>
  )
}
