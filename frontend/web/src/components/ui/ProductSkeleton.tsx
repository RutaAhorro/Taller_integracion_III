// Reserva el espacio de una tarjeta de producto mientras llegan sus datos.
import { Skeleton } from "@/components/ui/skeleton"

export function ProductSkeleton() {
  return (
    <div className="group flex flex-col overflow-hidden rounded-xl border border-border bg-card">
      <div className="relative aspect-square bg-muted/50 p-2">
        <Skeleton className="absolute top-2 left-2 h-4 w-10 rounded bg-muted-foreground/20" />
        
        <Skeleton className="absolute bottom-2 left-2 h-5 w-20 rounded bg-muted-foreground/20" />
        
        <div className="absolute inset-0 flex items-center justify-center">
          <Skeleton className="h-2/3 w-2/3 rounded-lg" />
        </div>
      </div>

      <div className="flex flex-1 flex-col p-3">
        <div className="mb-2 flex items-baseline gap-2">
          <Skeleton className="h-6 w-20 rounded" />
          <Skeleton className="h-4 w-12 rounded" />
        </div>

        <Skeleton className="mb-1 h-4 w-3/4 rounded" />
        <Skeleton className="mb-2 h-3 w-1/2 rounded" />

        <Skeleton className="mt-1 h-5 w-24 rounded" />

        <div className="mt-auto pt-3">
          <div className="flex items-center justify-between gap-2 border-t border-border pt-3">
            <Skeleton className="h-3 w-28 rounded" />
            <Skeleton className="h-8 w-8 rounded-full" />
          </div>
        </div>
      </div>
    </div>
  )
}
