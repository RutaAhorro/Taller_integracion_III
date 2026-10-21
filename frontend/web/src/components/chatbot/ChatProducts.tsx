import type { ChatProduct } from '@/features/chat/chat-api';
import { formatPrice } from '@/lib/formatters';

export function ChatProducts({ products }: { products: ChatProduct[] }) {
  if (products.length === 0) return <p className="mt-3 text-sm text-muted-foreground">Esta respuesta no incluye productos para mostrar.</p>;

  return <section className="mt-4 space-y-3" aria-label="Productos devueltos por el asistente">
    <h2 className="font-semibold">Productos de la respuesta</h2>
    <ul className="space-y-2">
      {products.map((product, index) => <li key={`${product.id ?? 'producto'}-${index}`} className="rounded-lg border border-border p-3">
        <p className="break-words font-medium">{product.nombre}</p>
        <p className="text-sm text-muted-foreground">{product.supermercado || 'Supermercado no informado'}</p>
        <p className="mt-1 font-semibold">{formatPrice(product.precio)}</p>
        <p className="text-xs text-muted-foreground">{product.en_stock ? 'Con stock según catálogo' : 'Sin stock según catálogo'}</p>
      </li>)}
    </ul>
    <p className="text-xs text-muted-foreground">Guardar estos productos en una lista estará disponible en una próxima entrega.</p>
  </section>;
}
