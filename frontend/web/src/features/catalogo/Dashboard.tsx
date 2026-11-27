// Presenta el catálogo filtrado, sus estados de carga y la continuación del listado paginado.
import { Store, Loader2 } from 'lucide-react'
import { useLayoutContext } from '@/layouts/MainLayout'
import { ProductCard } from '@/features/catalogo/ProductCard'
import { ProductSkeleton } from '@/features/catalogo/ProductSkeleton'
import { Footer } from '@/components/Footer'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'
import { useCart } from '@/features/catalogo/CartContext'

function formatearCategoriaActiva(categoria: string): string {
  if (categoria === 'ofertas') return 'Mejores ofertas'
  if (categoria === 'descuentos') return 'Mayor descuento'
  if (categoria === 'marcas') return 'Marcas blancas'

  const textoCategoria = categoria.startsWith('categoria:')
    ? categoria.slice('categoria:'.length)
    : categoria
  const textoNormalizado = textoCategoria.replace(/[-_]+/g, ' ').trim()
  if (!textoNormalizado) return ''
  return textoNormalizado.charAt(0).toUpperCase() + textoNormalizado.slice(1)
}

export default function Page() {
  const { query, category, activeMarket, markets, catalogo } = useLayoutContext()
  const { addToCart } = useCart()
  const supermercadoSeleccionado = markets.find((mercado) => mercado.id === activeMarket)?.name
  const consultaActiva = query.trim()
  const categoriaActiva = formatearCategoriaActiva(category)
  const hayFiltroActivo = Boolean(consultaActiva || category || supermercadoSeleccionado)
  const tituloHero = consultaActiva
    ? `Resultados para “${consultaActiva}”`
    : category
      ? categoriaActiva
      : supermercadoSeleccionado
        ? `Catálogo en ${supermercadoSeleccionado}`
        : 'Compara precios antes de salir a comprar'
  const subtituloHero = consultaActiva
    ? `Mostrando coincidencias para “${consultaActiva}”${supermercadoSeleccionado ? ` en ${supermercadoSeleccionado}` : ''}${category ? ` · ${categoriaActiva}` : ''}.`
    : category
      ? `Mostrando ${categoriaActiva.toLowerCase()}${supermercadoSeleccionado ? ` en ${supermercadoSeleccionado}` : ''}.`
      : supermercadoSeleccionado
        ? `Filtrando el catálogo en ${supermercadoSeleccionado}.`
        : 'Explora los precios vigentes o abre el menú ☰ para filtrar por categoría o supermercado.'

  const manejarDesplazamiento = (evento: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = evento.currentTarget;
    if (scrollHeight - scrollTop <= clientHeight + 50) {
      catalogo.fetchMore();
    }
  };

  const manejarAgregar = (producto: Producto) => {
    addToCart(producto)
  }
  return (
    <div className="relative flex flex-1 w-full h-full bg-background overflow-hidden">
      {/* Fondo de imagen geométrica vibrante */}
      <div
        className="absolute inset-0 z-0 opacity-[0.60] bg-[url('/images/dashboard-seamless.jpg')] bg-cover bg-center bg-no-repeat pointer-events-none"
        aria-hidden="true"
      />
      <main className="relative z-10 mx-auto flex w-full max-w-[1600px] flex-1 flex-col gap-6 overflow-hidden px-4 py-6 md:px-6">
        <section
          aria-label="Productos en oferta"
          className="flex min-h-0 flex-1 flex-col"
        >
          <div 
            className="min-h-0 flex-1 overflow-y-auto pb-2 pr-1 -mr-1 scrollbar-hide"
            onScroll={manejarDesplazamiento}
          >
            <section className="mb-5 rounded-2xl border-2 border-border bg-card p-5 shadow-[6px_6px_0px_var(--color-border)] md:p-6">
              <div className="min-w-0">
                <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-primary/10 px-3 py-1 text-xs font-bold text-primary">
                  {catalogo.supermarketCount}{' '}
                  {catalogo.supermarketCount === 1 ? 'supermercado activo' : 'supermercados activos'}
                </span>
                <h1 className="mt-2 font-['Fraunces'] text-2xl font-bold tracking-tight text-foreground md:text-3xl">
                  {tituloHero}
                </h1>
                <p className="mt-1 text-sm text-muted-foreground">
                  {subtituloHero}
                </p>
              </div>
            </section>

            {catalogo.isLoading ? (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 md:gap-4">
                {Array.from({ length: 12 }).map((_, indice) => (
                  <ProductSkeleton key={indice} />
                ))}
              </div>
             ) : catalogo.notice ? (
               <p role="status" className="py-12 text-center text-sm text-muted-foreground">{catalogo.notice}</p>
             ) : catalogo.error ? (
              <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
                <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                 <p className="text-sm text-muted-foreground">{catalogo.error}</p>
                 <button type="button" className="text-sm underline" onClick={catalogo.retry}>Reintentar</button>
              </div>
            ) : catalogo.filtered.length > 0 ? (
              <div className="flex flex-col pb-6">
                <div className="mb-4 flex flex-wrap items-center justify-between gap-2 px-1">
                  <h2 className="text-sm font-bold text-foreground">
                    {hayFiltroActivo ? 'Resultados encontrados' : 'Explorar por menor precio'}
                  </h2>
                  <p className="text-xs text-muted-foreground">
                    {catalogo.filtered.length} productos cargados · Ordenados de menor a mayor precio
                  </p>
                </div>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 md:gap-4">
                  {catalogo.filtered.map((producto) => (
                    <ProductCard key={producto.id} product={producto} onAdd={manejarAgregar} />
                  ))}
                </div>
                 {catalogo.isFetchingMore && (
                  <div className="flex justify-center items-center py-6">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                    <span className="ml-2 text-sm font-medium text-muted-foreground">Cargando más productos...</span>
                  </div>
                 )}
                 {catalogo.loadMoreError && (
                   <div role="alert" className="py-4 text-center text-sm">
                     <p>{catalogo.loadMoreError}</p>
                     <button type="button" className="underline" onClick={catalogo.fetchMore}>Reintentar</button>
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
