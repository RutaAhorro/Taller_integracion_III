// Representa ingredientes recibidos y delega su agregado al carrito en el componente padre.
import { ShoppingCart } from 'lucide-react';
import { cn } from '@/lib/utils';
import { formatPrice } from '@/lib/formatters';

export interface RecipeItem {
  name: string;
  marketName: string;
  marketColor?: string; // Color CSS válido para la etiqueta de la cadena.
  price: number;
  inStock: boolean;
  substitute?: string;
}

interface RichRecipeCardProps {
  items?: RecipeItem[];
  title?: string;
  description?: string;
  onAddToCart?: () => void;
  className?: string;
}

export function RichRecipeCard({ 
  items = [], 
  title = "¡Excelente elección!", 
  description = "He calculado la alternativa más económica para tu comida. Aquí tienes los ingredientes optimizados según los catálogos vigentes:",
  onAddToCart, 
  className 
}: RichRecipeCardProps) {
  
  if (!items || items.length === 0) return null;

  return (
    <div className={cn("space-y-5", className)}>
      <div className="leading-relaxed text-sm">
        <strong className="block mb-1">{title}</strong>
        {description}
      </div>
      
      <div className="space-y-2.5">
        {items.map((item, idx) => (
          <div key={idx} className="flex flex-col sm:flex-row gap-3 sm:items-center justify-between p-3 rounded-xl border border-border bg-background hover:border-border transition-colors">
            <div className="flex gap-3 items-start sm:items-center">
              <input type="checkbox" defaultChecked className="mt-1 sm:mt-0 h-4 w-4 rounded border-input text-foreground focus:ring-ring" />
              <div>
                <p className="font-medium text-sm line-clamp-1" title={item.name}>{item.name}</p>
                <div className="flex flex-wrap items-center gap-2 mt-1.5">
                  <span 
                    className="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
                    style={{ 
                      backgroundColor: item.marketColor ? `color-mix(in srgb, ${item.marketColor} 15%, transparent)` : undefined,
                      color: item.marketColor,
                      borderColor: item.marketColor ? `color-mix(in srgb, ${item.marketColor} 30%, transparent)` : undefined
                    }}
                  >
                    {item.marketName}
                  </span>
                  {!item.inStock && (
                    <span className="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider bg-red-100 text-red-700 border-red-200">
                      Sin Stock
                    </span>
                  )}
                  {item.substitute && (
                    <span className="text-xs text-muted-foreground font-medium">
                      Sustituto sugerido: {item.substitute}
                    </span>
                  )}
                </div>
              </div>
            </div>
            <div className="text-right shrink-0">
              <p className="font-bold text-sm">{formatPrice(item.price)}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="flex flex-wrap gap-2 pt-2">
        <button 
          onClick={onAddToCart}
          className="flex-1 sm:flex-none flex items-center justify-center gap-2 bg-primary hover:bg-primary/90 text-primary-foreground px-4 py-2.5 rounded-xl text-sm font-bold shadow-sm border-2 border-border transition-all active:translate-y-[2px]"
        >
          <ShoppingCart className="h-4 w-4" />
          Agregar ingredientes al Carrito
        </button>
      </div>
    </div>
  );
}
