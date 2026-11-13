// Permite revisar y editar el carrito temporal; no equivale a una lista persistida.
import { X, Minus, Plus, ShoppingBag, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useCart } from '@/contexts/CartContext'
import { formatPrice } from '@/lib/formatters'

export function CartSidebar() {
  const { isCartOpen, setIsCartOpen, items = [], updateQuantity, totalPrice } = useCart()

  if (!isCartOpen) return null

  return (
    <>
      <div 
        className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm transition-opacity"
        onClick={() => setIsCartOpen(false)}
        aria-hidden="true"
      />

      <aside 
        className="fixed inset-y-0 right-0 z-50 flex w-full max-w-sm flex-col border-l border-border bg-card shadow-2xl duration-300 animate-in slide-in-from-right"
        role="dialog"
        aria-label="Carrito de compras"
      >
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <div className="flex items-center gap-2 font-semibold">
            <ShoppingBag className="h-5 w-5 text-primary" />
            <h2>Tu Lista de Compras</h2>
          </div>
          <Button 
            variant="ghost" 
            size="icon" 
            onClick={() => setIsCartOpen(false)}
            aria-label="Cerrar carrito"
          >
            <X className="h-5 w-5" />
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto p-5">
          {items.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center gap-3 text-center text-muted-foreground">
              <ShoppingBag className="h-12 w-12 opacity-20" />
              <p>Tu lista está vacía.</p>
              <p className="text-sm">¡Añade productos para empezar a ahorrar!</p>
            </div>
          ) : (
            <ul className="flex flex-col gap-6">
              {items.map(({ product, quantity }) => (
                <li key={product.id} className="flex gap-4">
                  <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-xl border border-border bg-secondary p-2">
                    <img 
                      src={product.image || '/placeholder.svg'} 
                      alt={product.name ? `Imagen de ${product.name}` : 'Imagen de producto'}
                      loading="lazy"
                      onError={(e) => { e.currentTarget.src = '/placeholder.svg'; }}
                      className="h-full w-full object-contain"
                    />
                  </div>

                  <div className="flex flex-1 flex-col">
                    <h3 className="text-sm font-semibold leading-tight line-clamp-2" title={product.name}>
                      {product.name}
                    </h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {product.brand || 'Genérico'} • {product.unit}
                    </p>
                    
                    <div className="mt-auto flex items-end justify-between pt-2">
                      <span className="font-mono font-bold">
                        {formatPrice(product.price * quantity)}
                      </span>
                      
                      <div className="flex items-center gap-2 rounded-lg border border-border bg-background p-1">
                        <button
                          className="flex h-6 w-6 items-center justify-center rounded-md hover:bg-secondary disabled:opacity-50"
                          onClick={() => updateQuantity(product.id, Math.max(0, quantity - 1))}
                          aria-label="Reducir cantidad"
                        >
                          {quantity <= 1 ? <Trash2 className="h-3.5 w-3.5 text-destructive" /> : <Minus className="h-3.5 w-3.5" />}
                        </button>
                        <span className="min-w-[20px] text-center text-xs font-medium">
                          {quantity}
                        </span>
                        <button
                          className="flex h-6 w-6 items-center justify-center rounded-md hover:bg-secondary"
                          onClick={() => updateQuantity(product.id, quantity + 1)}
                          aria-label="Aumentar cantidad"
                        >
                          <Plus className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        {items.length > 0 && (
          <div className="border-t border-border bg-secondary/30 p-5">
            <div className="mb-4 flex items-center justify-between">
              <span className="text-sm font-medium text-muted-foreground">Total estimado</span>
              <span className="text-xl font-bold text-foreground">{formatPrice(totalPrice)}</span>
            </div>
            <Button className="w-full text-base h-12 rounded-xl bg-primary text-primary-foreground hover:bg-primary/90">
              Continuar a Optimización
            </Button>
          </div>
        )}
      </aside>
    </>
  )
}
