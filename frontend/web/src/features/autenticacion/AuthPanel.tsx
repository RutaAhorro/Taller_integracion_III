// Acceso con Google y salida como invitado durante el MVP de RutaAhorro.
import { useNavigate, Link } from "react-router-dom"
import { Lock, ShoppingBasket } from "lucide-react"
import { useAuth } from "@/features/autenticacion/useAuth"
import { SocialButtons } from "@/features/autenticacion/SocialButtons"

import { Button } from "@/components/ui/button"

function AvisoErrorAutenticacion({ message }: { message: string }) {
  return (
    <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2">
      <Lock className="size-4 shrink-0" /> {message}
    </div>
  )
}

function EncabezadoFormulario({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-6">
      <h2 className="font-display text-2xl font-bold tracking-tight text-foreground text-balance">{title}</h2>
      <p className="mt-1.5 text-sm text-muted-foreground">{subtitle}</p>
    </div>
  )
}

export function AuthPanel() {
  const { logout, isLoading, error: errorAutenticacion } = useAuth()
  const navegar = useNavigate()

  return (
    <div className="relative flex h-full items-center justify-center overflow-y-auto bg-background px-5 py-10 sm:px-8">
      <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
        <div
          className="absolute inset-0 bg-cover bg-center bg-no-repeat transition-opacity duration-500 grayscale opacity-30 bg-[url('/street_art_supermarket.jpg')]"
        />
      </div>

      <div className="relative z-10 w-full max-w-md">
        <div className="mb-8 flex items-center justify-center gap-2.5 lg:hidden">
          <Link to="/" className="flex items-center gap-2.5">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <ShoppingBasket className="size-5" />
            </span>
            <span className="font-display text-lg font-bold tracking-tight text-foreground">
              Ruta<span className="text-primary">Ahorro</span>
            </span>
          </Link>
        </div>

        <div className="rounded-xl border-4 border-border bg-background p-6 shadow-[8px_8px_0px_var(--color-border)] sm:p-8">
          <EncabezadoFormulario title="Bienvenido a RutaAhorro" subtitle="Inicia sesión con tu cuenta de Google para guardar listas y comparar precios." />

          {errorAutenticacion && <AvisoErrorAutenticacion message={errorAutenticacion} />}

          <SocialButtons />

          <p className="mt-6 text-center text-sm text-muted-foreground">
            Al continuar aceptas los{" "}
            <Link to="/terminos" target="_blank" className="text-primary font-medium underline-offset-4 hover:underline">
              Términos y Condiciones
            </Link>
          </p>

          <div className="mt-6 flex justify-center">
            <Button
              variant="ghost"
              className="text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-transparent"
              disabled={isLoading}
              onClick={async () => { await logout(); navegar('/dashboard'); }}
            >
              Continuar como invitado
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
