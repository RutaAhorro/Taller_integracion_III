// Bloque de carga sin datos de ejemplo para reservar espacio durante una consulta.
import { cn } from "@/lib/utils"

function Skeleton({
  className,
  ...propiedades
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("animate-pulse rounded-md bg-muted", className)}
      {...propiedades}
    />
  )
}

export { Skeleton }
