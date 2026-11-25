// Comparte el carrito persistente de la cuenta y sus cambios pendientes de sincronización.
import {
  createContext,
  useContext,
  useState,
  useMemo,
  type ReactNode,
} from "react";
import type { UiProduct as Producto } from "@/features/catalogo/tipos";
import { useToast } from "@/components/ToastContext";
import { useAuth } from "@/features/autenticacion/useAuth";
import { useGuardarCarrito } from "./useGuardarCarrito";
import { useCarritoPersistente } from "./useCarritoPersistente";
import type { ItemCarrito } from "./carrito-persistente";
import {
  MAXIMO_ENVASES_ARTICULO,
  MAXIMO_ARTICULOS_LISTA,
} from "@/features/listas/listas-api";

export type CartItem = ItemCarrito;

type TipoContextoCarrito = {
  items: CartItem[];
  totalItems: number;
  totalPrice: number | null;
  isCartOpen: boolean;
  setIsCartOpen: (abierto: boolean) => void;
  addToCart: (producto: Producto) => void;
  removeFromCart: (idProducto: string) => void;
  updateQuantity: (idProducto: string, cantidad: number) => void;
  clearCart: () => void;
  guardado: ReturnType<typeof useGuardarCarrito>;
  persistencia: ReturnType<typeof useCarritoPersistente>;
};

const ContextoCarrito = createContext<TipoContextoCarrito | undefined>(
  undefined,
);

/** Aísla carrito y envíos pendientes por cuenta; nunca reutiliza la clave de otro usuario. */
export function CartProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  return (
    <CarritoDeCuenta key={user?.id ?? "invitado"} usuarioID={user?.id}>
      {children}
    </CarritoDeCuenta>
  );
}

/** Recupera el carrito de Motor/Listas; el borrador local nunca acredita identidad. */
function CarritoDeCuenta({
  children,
  usuarioID,
}: {
  children: ReactNode;
  usuarioID?: string;
}) {
  const persistencia = useCarritoPersistente(usuarioID);
  const elementos = persistencia.estado.items;
  /** Sólo el coordinador puede editar: persiste primero y serializa la sincronización. */
  const setElementos = (cambiar: (actuales: CartItem[]) => CartItem[]) =>
    persistencia.almacen.editar(cambiar);
  const [carritoAbierto, setCarritoAbierto] = useState(false);
  const guardado = useGuardarCarrito(usuarioID, persistencia);

  const { toast } = useToast();

  const agregarAlCarrito = (producto: Producto) => {
    if (
      !usuarioID ||
      !persistencia.estado.listo ||
      persistencia.estado.conflicto ||
      guardado.bloqueado
    ) {
      toast(
        !usuarioID
          ? "Inicia sesión para guardar productos en tu carrito."
          : "Recupera el carrito o resuelve su sincronización antes de modificarlo.",
        "error",
      );
      return;
    }
    setElementos((actuales) => {
      const existente = actuales.find(
        (elemento) => elemento.product.id === producto.id,
      );
      if (existente) {
        return actuales.map((elemento) =>
          elemento.product.id === producto.id
            ? {
                ...elemento,
                quantity: Math.min(
                  MAXIMO_ENVASES_ARTICULO,
                  elemento.quantity + 1,
                ),
              }
            : elemento,
        );
      }
      return actuales.length >= MAXIMO_ARTICULOS_LISTA
        ? actuales
        : [...actuales, { product: producto, quantity: 1 }];
    });
    if (
      elementos.length >= MAXIMO_ARTICULOS_LISTA &&
      !elementos.some((elemento) => elemento.product.id === producto.id)
    ) {
      toast(
        `El carrito admite hasta ${MAXIMO_ARTICULOS_LISTA} productos.`,
        "error",
      );
    } else if (
      elementos.some(
        (elemento) =>
          elemento.product.id === producto.id &&
          elemento.quantity >= MAXIMO_ENVASES_ARTICULO,
      )
    ) {
      toast(`Máximo ${MAXIMO_ENVASES_ARTICULO} envases por producto.`, "error");
    } else {
      toast(
        `${producto.name} agregado al carrito; consulta el estado de sincronización.`,
        "success",
      );
    }
  };

  const eliminarDelCarrito = (idProducto: string) => {
    if (guardado.bloqueado) return;
    setElementos((actuales) =>
      actuales.filter((elemento) => elemento.product.id !== idProducto),
    );
  };

  const actualizarCantidad = (idProducto: string, cantidad: number) => {
    if (
      guardado.bloqueado ||
      !Number.isInteger(cantidad) ||
      cantidad > MAXIMO_ENVASES_ARTICULO
    )
      return;
    if (cantidad < 1) {
      eliminarDelCarrito(idProducto);
      return;
    }
    setElementos((actuales) =>
      actuales.map((elemento) =>
        elemento.product.id === idProducto
          ? { ...elemento, quantity: cantidad }
          : elemento,
      ),
    );
  };

  const vaciarCarrito = () => {
    if (guardado.bloqueado) return;
    setElementos(() => []);
  };

  const totalElementos = useMemo(
    () => elementos.reduce((suma, elemento) => suma + elemento.quantity, 0),
    [elementos],
  );

  const precioTotal = useMemo(
    () =>
      elementos.some((elemento) => elemento.product.price === null)
        ? null
        : elementos.reduce(
            (suma, elemento) =>
              suma + (elemento.product.price ?? 0) * elemento.quantity,
            0,
          ),
    [elementos],
  );

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
        guardado,
        persistencia,
      }}
    >
      {children}
    </ContextoCarrito.Provider>
  );
}

export function useCart() {
  const contexto = useContext(ContextoCarrito);
  if (contexto === undefined) {
    throw new Error("useCart debe usarse dentro de CartProvider");
  }
  return contexto;
}
