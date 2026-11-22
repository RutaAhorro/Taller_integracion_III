// Formularios de acceso y registro con validación y errores devueltos por Identidad.
import { useState, type ReactNode } from "react"
import { useNavigate, useLocation, Link } from "react-router-dom"
import { ArrowRight, Eye, EyeOff, Lock, Mail, Loader2, ShoppingBasket } from "lucide-react"
import { useToast } from "@/components/ToastContext"
import { useAuth } from "@/features/autenticacion/useAuth"
import { ROUTES, rutaTrasAutenticacion } from "@/app/routes"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSeparator,
} from "@/components/ui/field"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PasswordStrength } from "@/features/autenticacion/PasswordStrength"
import { SocialButtons } from "@/features/autenticacion/SocialButtons"

type ErroresFormulario = {
  loginEmail?: string
  loginPassword?: string
  signupEmail?: string
  signupPassword?: string
  terms?: string
  age?: string
}

/** Muestra validación junto al campo sin desplazar el formulario móvil. */
function ErrorEnLinea({ message }: { message?: string }) {
  return <div className="hidden sm:block">{message && <FieldError errors={[{ message }]} />}</div>
}

function AvisoErrorAutenticacion({ message }: { message: string }) {
  return (
    <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2">
      <Lock className="size-4 shrink-0" /> {message}
    </div>
  )
}

type PropiedadesCampoCorreo = { id: string; value: string; onChange: (valor: string) => void; error?: string }

function CampoCorreo({ id, value, onChange, error }: PropiedadesCampoCorreo) {
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={id}>Correo electrónico</FieldLabel>
      <div className="relative">
        <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type="email"
          value={value}
          onChange={(evento) => onChange(evento.target.value)}
          placeholder="tu@correo.com"
          autoComplete="email"
          className={`h-11 rounded-xl pl-10 ${error ? 'border-destructive border-2' : ''}`}
        />
      </div>
      <ErrorEnLinea message={error} />
    </Field>
  )
}

type PropiedadesCampoContrasena = {
  id: string
  value: string
  onChange: (valor: string) => void
  error?: string
  placeholder: string
  autoComplete: "current-password" | "new-password"
  children?: ReactNode
}

function CampoContrasena({ id, value, onChange, error, placeholder, autoComplete, children }: PropiedadesCampoContrasena) {
  const [visible, setVisible] = useState(false)
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={id}>Contraseña</FieldLabel>
      <div className="relative">
        <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type={visible ? "text" : "password"}
          value={value}
          onChange={(evento) => onChange(evento.target.value)}
          placeholder={placeholder}
          autoComplete={autoComplete}
          className={`h-11 rounded-xl pr-10 pl-10 ${error ? 'border-destructive border-2' : ''}`}
        />
        <button
          type="button"
          onClick={() => setVisible((visibleAnterior) => !visibleAnterior)}
          aria-label={visible ? "Ocultar contraseña" : "Mostrar contraseña"}
          className="cursor-pointer absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
        >
          {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
      <ErrorEnLinea message={error} />
      {children}
    </Field>
  )
}

type PropiedadesCampoCasilla = { id: string; checked: boolean; onChange: (marcado: boolean) => void; error?: string; children: ReactNode }

function CampoCasilla({ id, checked, onChange, error, children }: PropiedadesCampoCasilla) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-start gap-2">
        <input
          type="checkbox"
          id={id}
          checked={checked}
          onChange={(evento) => onChange(evento.target.checked)}
          className={`mt-1 h-4 w-4 rounded bg-background focus:ring-primary focus:ring-offset-2 shrink-0 cursor-pointer ${error ? 'border-destructive border-2 text-destructive' : 'border-border text-primary'}`}
        />
        <label htmlFor={id} className={`text-sm leading-snug cursor-pointer select-none ${error ? 'text-destructive font-medium' : 'text-muted-foreground'}`}>
          {children}
        </label>
      </div>
      <div className="hidden sm:block">
        {error && <span className="text-xs text-destructive font-medium pl-6">{error}</span>}
      </div>
    </div>
  )
}

function BotonEnviar({ isLoading, label, loadingLabel }: { isLoading: boolean; label: string; loadingLabel: string }) {
  return (
    <Button type="submit" disabled={isLoading} className="h-11 w-full rounded-xl text-sm font-semibold hover:brightness-110 transition-all">
      {isLoading ? loadingLabel : label}
      {isLoading ? <Loader2 className="animate-spin" data-icon="inline-end" /> : <ArrowRight data-icon="inline-end" />}
    </Button>
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
  const ubicacion = useLocation()
  const [pestana, setPestana] = useState(ubicacion.state?.tab === "signup" ? "signup" : "login")

  const [correoAcceso, setCorreoAcceso] = useState("")
  const [contrasenaAcceso, setContrasenaAcceso] = useState("")
  const [correoRegistro, setCorreoRegistro] = useState("")
  const [contrasenaRegistro, setContrasenaRegistro] = useState("")
  const [aceptaTerminos, setAceptaTerminos] = useState(false)
  const [aceptaEdad, setAceptaEdad] = useState(false)
  const [errores, setErrores] = useState<ErroresFormulario>({})

  const { login, register, logout, isLoading, error: errorAutenticacion } = useAuth()
  const navegar = useNavigate()
  const { toast } = useToast()

  // Valida los campos antes de delegar la autenticación en Identidad.
  const manejarInicioSesion = async (evento: React.FormEvent) => {
    evento.preventDefault()

    const nuevosErrores: ErroresFormulario = {}
    if (!correoAcceso.includes("@")) nuevosErrores.loginEmail = "Ingresa un correo electrónico válido"
    if (!contrasenaAcceso) nuevosErrores.loginPassword = "Contraseña requerida"
    setErrores(nuevosErrores)
    if (Object.keys(nuevosErrores).length > 0) return

    const usuario = await login({ correo: correoAcceso, password: contrasenaAcceso })
    if (usuario) {
      toast("Inicio de sesión exitoso", "success")
      navegar(rutaTrasAutenticacion(usuario.rol), { replace: true })
    }
  }

  // Registra la cuenta solo si se cumplen las validaciones locales y los consentimientos.
  const manejarRegistro = async (evento: React.FormEvent) => {
    evento.preventDefault()

    const nuevosErrores: ErroresFormulario = {}
    if (!correoRegistro.includes("@")) nuevosErrores.signupEmail = "Ingresa un correo electrónico válido"
    if (contrasenaRegistro.length < 8) nuevosErrores.signupPassword = "Usa al menos 8 caracteres para mayor seguridad"
    if (!aceptaTerminos) nuevosErrores.terms = "Debes aceptar los Términos y Condiciones"
    if (!aceptaEdad) nuevosErrores.age = "Debes confirmar que eres mayor de edad"
    setErrores(nuevosErrores)
    if (Object.keys(nuevosErrores).length > 0) return

    // Nombre inicial requerido por Identidad; la edición de perfil aún no está integrada.
    const exito = await register({
      correo: correoRegistro,
      password: contrasenaRegistro,
      nombre_completo: `Usuario ${correoRegistro.split('@')[0]}`,
    })

    if (exito) {
      toast("Cuenta creada con éxito. Revisa tu correo.", "success")
      navegar(ROUTES.CATALOGO)
    }
  }

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
          <Tabs value={pestana} onValueChange={(valor: string) => setPestana(valor)}>
            <TabsList className="mb-7 grid h-11 w-full grid-cols-2 rounded-xl bg-primary/20 p-1 border-2 border-border shadow-[4px_4px_0px_var(--color-border)]">
              <TabsTrigger value="login" className="rounded-lg text-sm hover:bg-primary/40">
                Iniciar Sesión
              </TabsTrigger>
              <TabsTrigger value="signup" className="rounded-lg text-sm hover:bg-primary/40">
                Crear Cuenta
              </TabsTrigger>
            </TabsList>

            <TabsContent value="login">
              <EncabezadoFormulario title="Bienvenido de nuevo" subtitle="Accede para seguir ahorrando en tus compras." />

              <form className="flex flex-col gap-5" onSubmit={manejarInicioSesion}>
                {errorAutenticacion && pestana === "login" && <AvisoErrorAutenticacion message={errorAutenticacion} />}

                <FieldGroup>
                  <CampoCorreo id="login-email" value={correoAcceso} onChange={setCorreoAcceso} error={errores.loginEmail} />
                  <CampoContrasena
                    id="login-password"
                    value={contrasenaAcceso}
                    onChange={setContrasenaAcceso}
                    error={errores.loginPassword}
                    placeholder="••••••••"
                    autoComplete="current-password"
                  />
                </FieldGroup>

                <BotonEnviar isLoading={isLoading} label="Entrar" loadingLabel="Ingresando..." />
              </form>

              <FieldSeparator className="my-6">O continuar con</FieldSeparator>
              <SocialButtons />

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
            </TabsContent>

            <TabsContent value="signup">
              <EncabezadoFormulario title="Crea tu cuenta gratis" subtitle="Empieza a comparar precios en menos de un minuto." />

              <form className="flex flex-col gap-5" onSubmit={manejarRegistro}>
                {errorAutenticacion && pestana === "signup" && <AvisoErrorAutenticacion message={errorAutenticacion} />}

                <FieldGroup>
                  <CampoCorreo id="signup-email" value={correoRegistro} onChange={setCorreoRegistro} error={errores.signupEmail} />
                  <CampoContrasena
                    id="signup-password"
                    value={contrasenaRegistro}
                    onChange={setContrasenaRegistro}
                    error={errores.signupPassword}
                    placeholder="Crea una contraseña segura"
                    autoComplete="new-password"
                  >
                    <div className="mt-1">
                      <PasswordStrength password={contrasenaRegistro} />
                    </div>
                  </CampoContrasena>
                </FieldGroup>

                <div className="flex flex-col gap-3 mt-2 mb-2">
                  <CampoCasilla id="terms" checked={aceptaTerminos} onChange={setAceptaTerminos} error={errores.terms}>
                    He leído y acepto los{" "}
                    <Link to="/terminos" target="_blank" className="text-primary font-medium underline-offset-4 hover:underline">
                      Términos y Condiciones
                    </Link>
                  </CampoCasilla>
                  <CampoCasilla id="age" checked={aceptaEdad} onChange={setAceptaEdad} error={errores.age}>
                    Confirmo que soy mayor de 18 años y tengo capacidad legal para aceptar estos términos
                  </CampoCasilla>
                </div>

                <BotonEnviar isLoading={isLoading} label="Registrarse" loadingLabel="Creando cuenta..." />
              </form>

              <FieldSeparator className="my-6">O continuar con</FieldSeparator>
              <SocialButtons />
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}
