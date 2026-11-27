// Consulta el producto seleccionado y muestra precios e historial recibidos del backend.
import { useParams, useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { PackageCheck, ShoppingCart, Store, Tag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatPrice, calculateDiscountPct } from '@/features/catalogo/formatters'
import { getProductDetail, getProducts } from '@/features/catalogo/products-api'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'
import { ProductCarousel } from '@/features/catalogo/ProductCarousel'
import { useCart } from '@/features/catalogo/CartContext'
import { Footer } from '@/components/Footer'

export default function ProductDetail() {
  const { id } = useParams()
  const navegar = useNavigate()
  const { addToCart } = useCart()
  const [producto, setProducto] = useState<Producto | null>(null)
  const [relacionados, setRelacionados] = useState<Producto[]>([])
  const [cargando, setCargando] = useState(true)

  useEffect(() => {
    if (!id) return
    const controlador = new AbortController()
    setCargando(true)
    Promise.all([getProductDetail(id), getProducts('?limit=100')])
      .then(([detalle, catalogo]) => {
        if (controlador.signal.aborted) return
        setProducto(detalle.product)
        const categoriaActual = detalle.product.category?.trim().toLowerCase()
        const marcaActual = detalle.product.brand.trim().toLowerCase()
        const tieneCategoriaUtil = categoriaActual && categoriaActual !== 'scraped category'
        const productosRelacionados = catalogo.products
          .filter((elemento) => elemento.id !== id && elemento.inStock === true)
          .map((elemento) => {
            const categoriaElemento = elemento.category?.trim().toLowerCase()
            const marcaElemento = elemento.brand.trim().toLowerCase()
            const mismaCategoria = Boolean(tieneCategoriaUtil && categoriaElemento === categoriaActual)
            const mismaMarca = Boolean(marcaActual && marcaElemento === marcaActual)

            return {
              elemento,
              puntaje: (mismaCategoria ? 2 : 0) + (mismaMarca ? 1 : 0),
            }
          })
          .filter(({ puntaje }) => puntaje > 0)
          .sort((izquierdo, derecho) => derecho.puntaje - izquierdo.puntaje)
          .slice(0, 8)
          .map(({ elemento }) => elemento)

        if (productosRelacionados.length === 0) {
          const respaldo = catalogo.products
            .filter((elemento) => elemento.id !== id && elemento.inStock === true)
            .slice(0, 8)
          setRelacionados(respaldo)
        } else {
          setRelacionados(productosRelacionados)
        }
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargando(false)
      })
    return () => controlador.abort()
  }, [id])

  if (cargando) {
    return <main className="flex flex-1 items-center justify-center p-6">Cargando producto...</main>
  }

  if (!producto) {
    return (
      <main className="flex flex-1 flex-col items-center justify-center p-6 text-center">
        <Store className="mb-4 h-12 w-12 text-muted-foreground" aria-hidden="true" />
        <h2 className="text-xl font-semibold">Producto no encontrado</h2>
        <p className="mb-6 mt-2 text-muted-foreground">
          El producto que buscas no existe o ha sido retirado.
        </p>
        <Button onClick={() => navegar('/dashboard')}>
          Volver al Dashboard
        </Button>
      </main>
    )
  }

  const nombreSupermercado = producto.supermarketName || 'Tienda'
  const porcentajeDescuento = calculateDiscountPct(producto.originalPrice, producto.price)

  return (
    <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col px-4 py-6 pb-32 sm:pb-6 md:px-6">

      <div className="flex flex-1 flex-col gap-6 md:flex-row md:items-center lg:gap-10">
        <section className="flex w-full md:w-5/12 flex-col items-center justify-center overflow-hidden rounded-3xl border border-border bg-secondary/50 p-6 shadow-inner">
          <div className="relative aspect-square w-full max-w-[200px] lg:max-w-[260px]">
            {porcentajeDescuento > 0 && (
              <span className="hidden sm:inline-block absolute left-0 top-0 z-10 rounded-lg bg-discount px-3 py-1.5 text-sm font-bold text-discount-foreground shadow-sm">
                -{porcentajeDescuento}% Descuento
              </span>
            )}
            <img
              src={producto.image || '/placeholder.svg'}
              alt={producto.name}
              className="h-full w-full object-contain drop-shadow-xl"
            />
          </div>
        </section>

        <section className="flex w-full md:w-7/12 flex-col justify-center py-2 md:py-4">
          <div className="mb-4 flex items-center gap-2">
            <span className="flex items-center gap-1.5 rounded-full border border-border bg-card py-1.5 pl-3 pr-3 text-xs font-medium shadow-sm">
              <Store className="h-4 w-4 text-muted-foreground" />
              Disponible en {nombreSupermercado}
            </span>
          </div>

          <div className="mb-2">
            <p className="text-sm font-medium tracking-widest text-primary uppercase">
              {producto.brand}
            </p>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl lg:text-4xl">
              {producto.name}
            </h1>
            <p className="mt-3 text-lg text-muted-foreground">{producto.unit}</p>
          </div>

          <div className="my-8 h-px w-full bg-border" />

          <div className="mb-8">
            <p className="text-sm font-medium text-muted-foreground">Precio actual</p>
            <div className="mt-1 flex flex-wrap items-center gap-3 sm:items-baseline sm:gap-4">
              <span className="text-4xl font-extrabold tracking-tight text-foreground sm:text-5xl">
                {formatPrice(producto.price)}
              </span>
              {porcentajeDescuento > 0 && (
                <>
                  <span className="text-xl font-medium text-muted-foreground line-through">
                    {formatPrice(producto.originalPrice)}
                  </span>
                  <span className="sm:hidden rounded-md bg-discount px-2 py-1 text-xs font-bold text-discount-foreground shadow-sm">
                    -{porcentajeDescuento}% OFF
                  </span>
                </>
              )}
            </div>
            {porcentajeDescuento > 0 && (
              <p className="mt-2 text-sm text-discount">
                Ahorras {formatPrice(producto.originalPrice - producto.price)} en este producto
              </p>
            )}
          </div>

          <div className="mb-8 grid grid-cols-1 gap-3 rounded-2xl border border-border bg-muted/40 p-4 text-sm sm:grid-cols-2">
            <div className="flex items-start gap-2">
              <Tag className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Categoría</p>
                <p className="font-semibold">{producto.category || 'Sin categoría'}</p>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <PackageCheck className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Disponibilidad</p>
                <p className={producto.inStock ? 'font-semibold text-green-700' : 'font-semibold text-destructive'}>
                  {producto.inStock ? 'En stock' : 'Sin stock'}
                </p>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <Store className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Supermercado</p>
                <p className="font-semibold">{nombreSupermercado}</p>
              </div>
            </div>
          </div>

          <div 
            className="fixed left-0 right-0 z-40 rounded-t-2xl border-t border-border bg-background p-4 shadow-[0_-8px_30px_rgba(0,0,0,0.08)] sm:static sm:z-auto sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0 sm:shadow-none mt-auto flex flex-col gap-3"
            style={{ bottom: 'calc(4rem + env(safe-area-inset-bottom, 0px))' }}
          >
            
            <div className="flex items-center justify-between sm:hidden mb-1 px-1">
              <span className="font-bold text-foreground">Tu producto</span>
              <div className="flex items-baseline gap-2">
                {porcentajeDescuento > 0 && (
                  <span className="text-xs font-medium text-muted-foreground line-through">
                    {formatPrice(producto.originalPrice)}
                  </span>
                )}
                <span className="text-xl font-extrabold text-foreground">
                  {formatPrice(producto.price)}
                </span>
              </div>
            </div>
            
            <div className="flex gap-3 flex-row w-full">
              <Button 
                size="lg" 
                className="w-full gap-1.5 rounded-xl text-sm sm:text-base h-12 sm:h-14 shadow-[4px_4px_0px_var(--color-border)] sm:shadow-[4px_4px_0px_var(--color-border)]"
                onClick={() => addToCart(producto)}
              >
                <ShoppingCart className="h-4 w-4 sm:h-5 sm:w-5" />
                <span className="hidden sm:inline">Añadir a la lista</span>
                <span className="sm:hidden">Añadir a lista</span>
              </Button>
            </div>
          </div>
        </section>
      </div>

      <ProductCarousel products={relacionados} />
      <Footer />
    </main>
  )
}
