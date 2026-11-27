// Presenta un producto del catálogo y comunica la selección al carrito mediante onAdd.
'use client'

import { Link } from 'react-router-dom'
import { Plus, Package, AlertCircle, TrendingDown, Store } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatPrice, calculateDiscountPct, calculateSavings } from '@/features/catalogo/formatters'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'
import { ROUTES } from '@/app/routes'
import { cn } from '@/lib/utils'

type PropiedadesTarjetaProducto = {
  product: Producto
  // Permite usar metadatos visuales entregados por el componente padre.
  marketName?: string
  marketLogo?: string
  onAdd?: (producto: Producto) => void
}

const INTERFAZ_INVENTARIO = {
  in:  { label: 'En stock',  cls: 'bg-green-100 text-green-700',  icon: Package      },
  low: { label: 'Stock bajo', cls: 'bg-amber-100 text-amber-700', icon: AlertCircle  },
  out: { label: 'Sin stock',  cls: 'bg-red-100 text-red-700', icon: AlertCircle  },
} as const

export function ProductCard({ product, marketName, marketLogo, onAdd }: PropiedadesTarjetaProducto) {
  // Conserva una etiqueta legible cuando falta el nombre de la cadena.
  const nombreSupermercadoVisible = marketName || product.supermarketName || 'Tienda'
  
  const estadoInventario = product.inStock === false ? 'out' : 'in'
  const configuracion = INTERFAZ_INVENTARIO[estadoInventario]
  const IconoInventario = configuracion.icon
  
  const ahorro = calculateSavings(product.originalPrice, product.price)
  const porcentajeDescuento = calculateDiscountPct(product.originalPrice, product.price)

  return (
    <Link 
      to={ROUTES.PRODUCTO_DETALLE(product.id)}
      className="group flex h-full flex-col overflow-hidden rounded-xl border-2 border-border bg-card transition-all hover:-translate-y-0.5 hover:shadow-[4px_4px_0px_var(--color-border)]"
    >
      <div className="relative aspect-square bg-white flex items-center justify-center overflow-hidden">
        {porcentajeDescuento > 0 && (
          <span className="absolute top-2 left-2 z-10 rounded bg-discount px-1.5 py-0.5 text-[10px] font-bold text-discount-foreground shadow-sm">
            -{porcentajeDescuento}%
          </span>
        )}

        <span className="absolute bottom-2 left-2 z-10 flex items-center gap-1.5 rounded border border-border bg-card/95 px-2 py-1 text-[10px] font-medium text-foreground backdrop-blur-sm shadow-sm">
          {marketLogo ? (
            <img
              src={marketLogo}
              alt={`Logo de ${nombreSupermercadoVisible}`}
              className="h-3 w-3 rounded-sm object-cover"
              onError={(evento) => { evento.currentTarget.style.display = 'none'; }}
            />
          ) : (
             <Store className="h-3 w-3 text-muted-foreground shrink-0" />
          )}
          <span className="truncate max-w-[80px]">{nombreSupermercadoVisible}</span>
        </span>

        <img
          src={product.image || '/placeholder.svg'}
          alt={product.name ? `Imagen de ${product.name}` : 'Imagen de producto'}
          loading="lazy"
          onError={(evento) => { evento.currentTarget.src = '/placeholder.svg'; }}
          className="h-full w-full object-contain p-6 transition-transform duration-300 group-hover:scale-105 mix-blend-multiply"
        />
      </div>

      {/* ---------- Contenido ---------- */}
      <div className="flex flex-1 flex-col p-4 pb-0">
        <div className="space-y-1">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider line-clamp-1">
            {product.brand || 'Genérico'}
          </p>
          <h3 className="text-sm font-semibold leading-tight line-clamp-2" title={product.name}>
            {product.name}
          </h3>
          <p className="text-xs text-muted-foreground">{product.unit}</p>
        </div>

        {/* Meta: stock + ahorro */}
        <div className="mt-3 flex flex-wrap gap-2">
          <span className={cn('flex items-center gap-1 rounded-sm px-1.5 py-0.5 text-[10px] font-medium', configuracion.cls)}>
            <IconoInventario className="h-3 w-3" />
            {configuracion.label}
          </span>
          {porcentajeDescuento > 0 && (
            <span className="flex items-center gap-1 rounded-sm bg-discount/10 px-1.5 py-0.5 text-[10px] font-medium text-discount">
              <TrendingDown className="h-3 w-3" />
              Ahorras {formatPrice(ahorro)}
            </span>
          )}
        </div>
      </div>

      <div className="flex items-end justify-between p-4 pt-3 mt-auto">
        <div className="flex flex-col">
          <span className="text-lg font-bold leading-none tracking-tight">
            {formatPrice(product.price)}
          </span>
          {porcentajeDescuento > 0 && (
            <span className="text-xs text-muted-foreground line-through mt-1">
              {formatPrice(product.originalPrice)}
            </span>
          )}
        </div>
        <Button
          size="icon"
          className="h-9 w-9 shrink-0 rounded-full border-2 border-border bg-brand text-white shadow-[2px_2px_0px_var(--color-border)] hover:bg-brand-dark"
          aria-label={`Añadir ${product.name} a la lista`}
          onClick={(evento) => {
            evento.preventDefault()
            onAdd?.(product)
          }}
        >
          <Plus className="h-5 w-5" aria-hidden="true" />
        </Button>
      </div>
    </Link>
  )
}
