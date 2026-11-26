// Acceso con Google y salida como invitado durante el MVP de RutaAhorro.
import { useNavigate, Link } from "react-router-dom"
import { Lock, ShoppingBasket } from "lucide-react"
import { useAuth } from "@/features/autenticacion/useAuth"
import { SocialButtons } from "@/features/autenticacion/SocialButtons"

import { Button } from "@/components/ui/button"

function AvisoErrorAutenticacion({ message }: { message: string }) {
  return (
    <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2 mb-4">
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
          className="absolute inset-0 bg-cover bg-center bg-no-repeat transition-opacity duration-500 grayscale opacity-15 lg:opacity-30 bg-[url('/street_art_supermarket.jpg')]"
        />
      </div>

      <div className="relative z-10 w-full max-w-md">
        <div className="mb-6 flex justify-center lg:hidden">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 rounded-xl border-2 border-border bg-primary px-4 py-2.5 text-primary-foreground shadow-[4px_4px_0px_var(--color-border)]"
          >
            <ShoppingBasket className="size-5" aria-hidden="true" />
            <span className="font-display text-lg font-bold tracking-tight">RutaAhorro</span>
          </Link>
        </div>

        <div className="rounded-xl border-4 border-border bg-background p-6 shadow-[8px_8px_0px_var(--color-border)] sm:p-8">
          <EncabezadoFormulario title="Bienvenido a RutaAhorro" subtitle="Inicia sesión con tu cuenta de Google para guardar listas y comparar precios." />

          {errorAutenticacion && <AvisoErrorAutenticacion message={errorAutenticacion} />}

          <SocialButtons />

          <div className="my-5 flex items-center gap-3">
            <span className="h-px flex-1 bg-border" aria-hidden="true" />
            <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">o</span>
            <span className="h-px flex-1 bg-border" aria-hidden="true" />
          </div>

          <div className="mt-6 flex justify-center">
            <Button
              variant="outline"
              className="h-11 w-full rounded-xl border-2 border-border text-sm font-semibold"
              disabled={isLoading}
              onClick={async () => { await logout(); navegar('/dashboard'); }}
            >
              Continuar como invitado
            </Button>
          </div>

          <p className="mt-5 text-center text-xs text-muted-foreground">
            Al continuar aceptas los{" "}
            <Link to="/terminos" target="_blank" className="text-primary font-medium underline-offset-4 hover:underline">
              Términos y Condiciones
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
