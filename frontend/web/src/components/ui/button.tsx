// Botón reutilizable con variantes visuales y comportamiento accesible de Base UI.
import { Button as ButtonPrimitive } from '@base-ui/react/button'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '@/lib/utils'

const buttonVariants = cva(
  "cursor-pointer group/button inline-flex shrink-0 items-center justify-center rounded-md border-2 border-border font-bold whitespace-nowrap outline-none select-none transition-all focus-visible:ring-4 focus-visible:ring-ring/20 hover:-translate-y-1 hover:shadow-[4px_4px_0px_var(--color-border)] active:translate-y-0 active:translate-x-0 active:shadow-[0px_0px_0px_var(--color-border)] disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-5",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground',
        outline: 'bg-background text-foreground hover:bg-secondary hover:text-secondary-foreground',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost: 'border-transparent hover:border-border hover:bg-accent hover:shadow-[4px_4px_0px_var(--color-border)] hover:text-accent-foreground',
        destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/80',
        link: 'border-transparent text-primary underline-offset-4 hover:underline hover:-translate-y-0 hover:shadow-none',
      },
      size: {
        default: 'h-10 gap-1.5 px-4 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3',
        xs: "h-7 gap-1 px-2 text-xs",
        sm: "h-9 gap-1 px-3 text-sm",
        lg: 'h-12 gap-2 px-8 text-lg',
        icon: 'size-10',
        'icon-xs': 'size-7',
        'icon-sm': 'size-9',
        'icon-lg': 'size-12',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

function Button({
  className,
  variant = 'default',
  size = 'default',
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
