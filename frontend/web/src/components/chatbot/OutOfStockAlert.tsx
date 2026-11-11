// Explica la falta de disponibilidad del producto indicado sin proponer existencias ficticias.
import { AlertTriangle } from 'lucide-react';
import { cn } from '@/lib/utils';

interface OutOfStockAlertProps {
  query?: string;
  className?: string;
}

export function OutOfStockAlert({ query = 'el producto', className }: OutOfStockAlertProps) {
  return (
    <div className={cn("rounded-xl border-2 border-border bg-amber-50 p-4 dark:bg-amber-950/30 shadow-[4px_4px_0px_var(--color-border)]", className)}>
      <div className="flex items-start gap-3">
        <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-500 mt-0.5 shrink-0" />
        <div>
          <h4 className="font-bold text-amber-900 dark:text-amber-100">Producto sin stock en la zona</h4>
          <p className="text-amber-800/90 dark:text-amber-200/90 text-sm mt-1 leading-relaxed">
            No encontramos disponibilidad para <span className="font-bold">'{query}'</span> en ningún supermercado de Temuco en este momento. ¿Deseas buscar un reemplazo como Harina de Avena o Nuez?
          </p>
          <div className="flex gap-2 mt-4">
            <button className="px-4 py-2 bg-background hover:bg-accent text-foreground shadow-[2px_2px_0px_var(--color-border)] border-2 border-border text-xs font-bold rounded-lg transition-all active:translate-y-[2px] active:shadow-none">
              Buscar reemplazos
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
