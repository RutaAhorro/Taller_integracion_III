// Campo de entrada compartido con estilos y atributos nativos del formulario.
import * as React from "react"
import { Input as InputPrimitive } from "@base-ui/react/input"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-10 w-full min-w-0 rounded-md border-2 border-border bg-background px-3 py-1 text-base font-medium text-foreground transition-all outline-none shadow-[4px_4px_0px_var(--color-border)] focus-visible:translate-x-[2px] focus-visible:translate-y-[2px] focus-visible:shadow-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-bold file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-muted disabled:opacity-50 aria-invalid:border-destructive aria-invalid:bg-destructive/10 md:text-sm",
        className
      )}
      {...props}
    />
  )
}

export { Input }
