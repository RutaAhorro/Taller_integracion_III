import { useParams, useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { PackageCheck, Plus, ShoppingCart, Store, Tag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatPrice, calculateDiscountPct } from '@/lib/formatters'
import { getProductDetail, getProducts } from '@/lib/products-api'
import type { UiProduct as Product } from '@/types'
import { ProductCarousel } from '@/components/ProductCarousel'
import { useCart } from '@/contexts/CartContext'
import { Footer } from '@/components/Footer'

export default function ProductDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { addToCart } = useCart()
  const [product, setProduct] = useState<Product | null>(null)
  const [related, setRelated] = useState<Product[]>([])
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!id) return
    const controller = new AbortController()
    setIsLoading(true)
    Promise.all([getProductDetail(id), getProducts('?limit=100')])
      .then(([detail, catalog]) => {
        if (controller.signal.aborted) return
        setProduct(detail.product)
        const currentCategory = detail.product.category?.trim().toLowerCase()
        const currentBrand = detail.product.brand.trim().toLowerCase()
        const hasUsefulCategory = currentCategory && currentCategory !== 'scraped category'
        const relatedProducts = catalog.products
          .filter((item) => item.id !== id && item.inStock === true)
          .map((item) => {
            const itemCategory = item.category?.trim().toLowerCase()
            const itemBrand = item.brand.trim().toLowerCase()
            const sameCategory = Boolean(hasUsefulCategory && itemCategory === currentCategory)
            const sameBrand = Boolean(currentBrand && itemBrand === currentBrand)

            return {
              item,
              score: (sameCategory ? 2 : 0) + (sameBrand ? 1 : 0),
            }
          })
          .filter(({ score }) => score > 0)
          .sort((left, right) => right.score - left.score)
          .slice(0, 8)
          .map(({ item }) => item)

        if (relatedProducts.length === 0) {
          const fallback = catalog.products
            .filter((item) => item.id !== id && item.inStock === true)
            .slice(0, 8)
          setRelated(fallback)
        } else {
          setRelated(relatedProducts)
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoading(false)
      })
    return () => controller.abort()
  }, [id])

  if (isLoading) {
    return <main className="flex flex-1 items-center justify-center p-6">Cargando producto...</main>
  }

  if (!product) {
    return (
      <main className="flex flex-1 flex-col items-center justify-center p-6 text-center">
        <Store className="mb-4 h-12 w-12 text-muted-foreground" aria-hidden="true" />
        <h2 className="text-xl font-semibold">Producto no encontrado</h2>
        <p className="mb-6 mt-2 text-muted-foreground">
          El producto que buscas no existe o ha sido retirado.
        </p>
        <Button onClick={() => navigate('/dashboard')}>
          Volver al Dashboard
        </Button>
      </main>
    )
  }

  const marketName = product.supermarketName || 'Tienda'
  const pct = calculateDiscountPct(product.originalPrice, product.price)

  return (
    <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col px-4 py-6 pb-32 sm:pb-6 md:px-6">

      <div className="flex flex-1 flex-col gap-6 md:flex-row md:items-center lg:gap-10">
        <section className="flex w-full md:w-5/12 flex-col items-center justify-center overflow-hidden rounded-3xl border border-border bg-secondary/50 p-6 shadow-inner">
          <div className="relative aspect-square w-full max-w-[200px] lg:max-w-[260px]">
            {pct > 0 && (
              <span className="hidden sm:inline-block absolute left-0 top-0 z-10 rounded-lg bg-discount px-3 py-1.5 text-sm font-bold text-discount-foreground shadow-sm">
                -{pct}% Descuento
              </span>
            )}
            <img
              src={product.image || '/placeholder.svg'}
              alt={product.name}
              className="h-full w-full object-contain drop-shadow-xl"
            />
          </div>
        </section>

        <section className="flex w-full md:w-7/12 flex-col justify-center py-2 md:py-4">
          <div className="mb-4 flex items-center gap-2">
            <span className="flex items-center gap-1.5 rounded-full border border-border bg-card py-1.5 pl-3 pr-3 text-xs font-medium shadow-sm">
              <Store className="h-4 w-4 text-muted-foreground" />
              Disponible en {marketName}
            </span>
          </div>

          <div className="mb-2">
            <p className="text-sm font-medium tracking-widest text-primary uppercase">
              {product.brand}
            </p>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl lg:text-4xl">
              {product.name}
            </h1>
            <p className="mt-3 text-lg text-muted-foreground">{product.unit}</p>
          </div>

          <div className="my-8 h-px w-full bg-border" />

          <div className="mb-8">
            <p className="text-sm font-medium text-muted-foreground">Precio actual</p>
            <div className="mt-1 flex flex-wrap items-center gap-3 sm:items-baseline sm:gap-4">
              <span className="text-4xl font-extrabold tracking-tight text-foreground sm:text-5xl">
                {formatPrice(product.price)}
              </span>
              {pct > 0 && (
                <>
                  <span className="text-xl font-medium text-muted-foreground line-through">
                    {formatPrice(product.originalPrice)}
                  </span>
                  <span className="sm:hidden rounded-md bg-discount px-2 py-1 text-xs font-bold text-discount-foreground shadow-sm">
                    -{pct}% OFF
                  </span>
                </>
              )}
            </div>
            {pct > 0 && (
              <p className="mt-2 text-sm text-discount">
                Ahorras {formatPrice(product.originalPrice - product.price)} en este producto
              </p>
            )}
          </div>

          <div className="mb-8 grid grid-cols-1 gap-3 rounded-2xl border border-border bg-muted/40 p-4 text-sm sm:grid-cols-2">
            <div className="flex items-start gap-2">
              <Tag className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Categoría</p>
                <p className="font-semibold">{product.category || 'Sin categoría'}</p>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <PackageCheck className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Disponibilidad</p>
                <p className={product.inStock ? 'font-semibold text-green-700' : 'font-semibold text-destructive'}>
                  {product.inStock ? 'En stock' : 'Sin stock'}
                </p>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <Store className="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
              <div>
                <p className="text-xs text-muted-foreground">Supermercado</p>
                <p className="font-semibold">{marketName}</p>
              </div>
            </div>
          </div>

          {/* Mobile Bottom Bar / Desktop Buttons */}
          <div 
            className="fixed left-0 right-0 z-40 rounded-t-2xl border-t border-border bg-background p-4 shadow-[0_-8px_30px_rgba(0,0,0,0.08)] sm:static sm:z-auto sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0 sm:shadow-none mt-auto flex flex-col gap-3"
            style={{ bottom: 'calc(4rem + env(safe-area-inset-bottom, 0px))' }}
          >
            
            <div className="flex items-center justify-between sm:hidden mb-1 px-1">
              <span className="font-bold text-foreground">Tu producto</span>
              <div className="flex items-baseline gap-2">
                {pct > 0 && (
                  <span className="text-xs font-medium text-muted-foreground line-through">
                    {formatPrice(product.originalPrice)}
                  </span>
                )}
                <span className="text-xl font-extrabold text-foreground">
                  {formatPrice(product.price)}
                </span>
              </div>
            </div>
            
            <div className="flex gap-3 flex-row w-full">
              <Button size="lg" variant="outline" className="flex-1 gap-1.5 rounded-xl text-sm sm:text-base h-12 sm:h-14">
                <Plus className="h-4 w-4 sm:h-5 sm:w-5" />
                <span className="hidden sm:inline">Comparar precio</span>
                <span className="sm:hidden">Comparar</span>
              </Button>
              <Button 
                size="lg" 
                className="flex-[1.5] gap-1.5 rounded-xl text-sm sm:text-base h-12 sm:h-14 shadow-[4px_4px_0px_var(--color-border)] sm:shadow-[4px_4px_0px_var(--color-border)]"
                onClick={() => addToCart(product)}
              >
                <ShoppingCart className="h-4 w-4 sm:h-5 sm:w-5" />
                <span className="hidden sm:inline">Añadir a la lista</span>
                <span className="sm:hidden">Añadir a lista</span>
              </Button>
            </div>
          </div>
        </section>
      </div>

      <ProductCarousel products={related} />
      <Footer />
    </main>
  )
}
