// Mantiene productos y cantidades del carrito en memoria; no persiste listas en el servidor.
import { createContext, useContext, useState, useMemo, type ReactNode } from 'react'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'
import { useToast } from '@/components/ToastContext'

export type CartItem = {
  product: Producto
  quantity: number
}

type TipoContextoCarrito = {
  items: CartItem[]
  totalItems: number
  totalPrice: number
  isCartOpen: boolean
  setIsCartOpen: (abierto: boolean) => void
  addToCart: (producto: Producto) => void
  removeFromCart: (idProducto: string) => void
  updateQuantity: (idProducto: string, cantidad: number) => void
  clearCart: () => void
}

const ContextoCarrito = createContext<TipoContextoCarrito | undefined>(undefined)

export function CartProvider({ children }: { children: ReactNode }) {
  const [elementos, setElementos] = useState<CartItem[]>([])
  const [carritoAbierto, setCarritoAbierto] = useState(false)

  const { toast } = useToast()

  const agregarAlCarrito = (producto: Producto) => {
    setElementos((actuales) => {
      const existente = actuales.find((elemento) => elemento.product.id === producto.id)
      if (existente) {
        return actuales.map((elemento) =>
          elemento.product.id === producto.id
            ? { ...elemento, quantity: elemento.quantity + 1 }
            : elemento
        )
      }
      return [...actuales, { product: producto, quantity: 1 }]
    })
    toast(`${producto.name} agregado a tu lista`, "success")
  }

  const eliminarDelCarrito = (idProducto: string) => {
    setElementos((actuales) => actuales.filter((elemento) => elemento.product.id !== idProducto))
  }

  const actualizarCantidad = (idProducto: string, cantidad: number) => {
    if (cantidad < 1) {
      eliminarDelCarrito(idProducto)
      return
    }
    setElementos((actuales) =>
      actuales.map((elemento) =>
        elemento.product.id === idProducto ? { ...elemento, quantity: cantidad } : elemento
      )
    )
  }

  const vaciarCarrito = () => {
    setElementos([])
  }

  const totalElementos = useMemo(
    () => elementos.reduce((suma, elemento) => suma + elemento.quantity, 0),
    [elementos]
  )

  const precioTotal = useMemo(
    () => elementos.reduce((suma, elemento) => suma + elemento.product.price * elemento.quantity, 0),
    [elementos]
  )

  return (
    <ContextoCarrito.Provider
      value={{
        items: elementos,
        totalItems: totalElementos,
        totalPrice: precioTotal,
        isCartOpen: carritoAbierto,
        setIsCartOpen: setCarritoAbierto,
        addToCart: agregarAlCarrito,
        removeFromCart: eliminarDelCarrito,
        updateQuantity: actualizarCantidad,
        clearCart: vaciarCarrito,
      }}
    >
      {children}
    </ContextoCarrito.Provider>
  )
}

export function useCart() {
  const contexto = useContext(ContextoCarrito)
  if (contexto === undefined) {
    throw new Error('useCart debe usarse dentro de CartProvider')
  }
  return contexto
}
