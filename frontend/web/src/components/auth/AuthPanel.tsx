import { useState, type ReactNode } from "react"
import { useNavigate, useLocation, Link } from "react-router-dom"
import { ArrowRight, Eye, EyeOff, Lock, Mail, Loader2, ShoppingBasket } from "lucide-react"
import { useToast } from "@/contexts/ToastContext"
import { useAuth } from "@/hooks/useAuth"

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
import { PasswordStrength } from "@/components/auth/PasswordStrength"
import { SocialButtons } from "@/components/auth/SocialButtons"

type FormErrors = {
  loginEmail?: string
  loginPassword?: string
  signupEmail?: string
  signupPassword?: string
  terms?: string
  age?: string
}

// Los mensajes de validación se ocultan en móvil para no desplazar el formulario.
function InlineError({ message }: { message?: string }) {
  return <div className="hidden sm:block">{message && <FieldError errors={[{ message }]} />}</div>
}

function AuthErrorBanner({ message }: { message: string }) {
  return (
    <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2">
      <Lock className="size-4 shrink-0" /> {message}
    </div>
  )
}

type EmailFieldProps = { id: string; value: string; onChange: (value: string) => void; error?: string }

function EmailField({ id, value, onChange, error }: EmailFieldProps) {
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={id}>Correo electrónico</FieldLabel>
      <div className="relative">
        <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type="email"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="tu@correo.com"
          autoComplete="email"
          className={`h-11 rounded-xl pl-10 ${error ? 'border-destructive border-2' : ''}`}
        />
      </div>
      <InlineError message={error} />
    </Field>
  )
}

type PasswordFieldProps = {
  id: string
  value: string
  onChange: (value: string) => void
  error?: string
  placeholder: string
  autoComplete: "current-password" | "new-password"
  // Contenido junto a la etiqueta (ej. enlace de recuperación) y bajo el input.
  labelExtra?: ReactNode
  children?: ReactNode
}

function PasswordField({ id, value, onChange, error, placeholder, autoComplete, labelExtra, children }: PasswordFieldProps) {
  const [visible, setVisible] = useState(false)
  return (
    <Field data-invalid={!!error}>
      <div className="flex items-center justify-between">
        <FieldLabel htmlFor={id}>Contraseña</FieldLabel>
        {labelExtra}
      </div>
      <div className="relative">
        <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          id={id}
          type={visible ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          autoComplete={autoComplete}
          className={`h-11 rounded-xl pr-10 pl-10 ${error ? 'border-destructive border-2' : ''}`}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-label={visible ? "Ocultar contraseña" : "Mostrar contraseña"}
          className="cursor-pointer absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
        >
          {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
      <InlineError message={error} />
      {children}
    </Field>
  )
}

type CheckboxFieldProps = { id: string; checked: boolean; onChange: (checked: boolean) => void; error?: string; children: ReactNode }

function CheckboxField({ id, checked, onChange, error, children }: CheckboxFieldProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-start gap-2">
        <input
          type="checkbox"
          id={id}
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
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

function SubmitButton({ isLoading, label, loadingLabel }: { isLoading: boolean; label: string; loadingLabel: string }) {
  return (
    <Button type="submit" disabled={isLoading} className="h-11 w-full rounded-xl text-sm font-semibold hover:brightness-110 transition-all">
      {isLoading ? loadingLabel : label}
      {isLoading ? <Loader2 className="animate-spin" data-icon="inline-end" /> : <ArrowRight data-icon="inline-end" />}
    </Button>
  )
}

function FormHeader({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="mb-6">
      <h2 className="font-display text-2xl font-bold tracking-tight text-foreground text-balance">{title}</h2>
      <p className="mt-1.5 text-sm text-muted-foreground">{subtitle}</p>
    </div>
  )
}

export function AuthPanel() {
  const location = useLocation()
  const [tab, setTab] = useState(location.state?.tab === "signup" ? "signup" : "login")

  const [loginEmail, setLoginEmail] = useState("")
  const [loginPassword, setLoginPassword] = useState("")
  const [signupEmail, setSignupEmail] = useState("")
  const [signupPassword, setSignupPassword] = useState("")
  const [acceptTerms, setAcceptTerms] = useState(false)
  const [acceptAge, setAcceptAge] = useState(false)
  const [errors, setErrors] = useState<FormErrors>({})

  const { login, register, logout, isLoading, error: authError } = useAuth()
  const navigate = useNavigate()
  const { toast } = useToast()

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()

    const newErrors: FormErrors = {}
    if (!loginEmail.includes("@")) newErrors.loginEmail = "Ingresa un correo electrónico válido"
    if (!loginPassword) newErrors.loginPassword = "Contraseña requerida"
    setErrors(newErrors)
    if (Object.keys(newErrors).length > 0) return

    if (await login({ correo: loginEmail, password: loginPassword })) {
      toast("Inicio de sesión exitoso", "success")
      navigate("/dashboard")
    }
  }

  const handleSignup = async (e: React.FormEvent) => {
    e.preventDefault()

    const newErrors: FormErrors = {}
    if (!signupEmail.includes("@")) newErrors.signupEmail = "Ingresa un correo electrónico válido"
    if (signupPassword.length < 8) newErrors.signupPassword = "Usa al menos 8 caracteres para mayor seguridad"
    if (!acceptTerms) newErrors.terms = "Debes aceptar los Términos y Condiciones"
    if (!acceptAge) newErrors.age = "Debes confirmar que eres mayor de edad"
    setErrors(newErrors)
    if (Object.keys(newErrors).length > 0) return

    // Nombre provisorio a partir del correo; el usuario lo completa en el onboarding.
    const exito = await register({
      correo: signupEmail,
      password: signupPassword,
      nombre_completo: `Usuario ${signupEmail.split('@')[0]}`,
    })

    if (exito) {
      toast("Cuenta creada con éxito. Revisa tu correo.", "success")
      navigate("/onboarding")
    }
  }

  return (
    <div className="relative flex h-full items-center justify-center overflow-y-auto bg-background px-5 py-10 sm:px-8">
      {/* Street art background without blur */}
      <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
        <div
          className="absolute inset-0 bg-cover bg-center bg-no-repeat transition-opacity duration-500 grayscale opacity-30 bg-[url('/street_art_supermarket.jpg')]"
        />
      </div>

      <div className="relative z-10 w-full max-w-md">
        {/* Mobile brand */}
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
          <Tabs value={tab} onValueChange={(v: string) => setTab(v)}>
            <TabsList className="mb-7 grid h-11 w-full grid-cols-2 rounded-xl bg-primary/20 p-1 border-2 border-border shadow-[4px_4px_0px_var(--color-border)]">
              <TabsTrigger value="login" className="rounded-lg text-sm hover:bg-primary/40">
                Iniciar Sesión
              </TabsTrigger>
              <TabsTrigger value="signup" className="rounded-lg text-sm hover:bg-primary/40">
                Crear Cuenta
              </TabsTrigger>
            </TabsList>

            <TabsContent value="login">
              <FormHeader title="Bienvenido de nuevo" subtitle="Accede para seguir ahorrando en tus compras." />

              <form className="flex flex-col gap-5" onSubmit={handleLogin}>
                {authError && tab === "login" && <AuthErrorBanner message={authError} />}

                <FieldGroup>
                  <EmailField id="login-email" value={loginEmail} onChange={setLoginEmail} error={errors.loginEmail} />
                  <PasswordField
                    id="login-password"
                    value={loginPassword}
                    onChange={setLoginPassword}
                    error={errors.loginPassword}
                    placeholder="••••••••"
                    autoComplete="current-password"
                    labelExtra={
                      <a href="#" className="text-xs font-medium text-primary underline-offset-4 hover:underline">
                        ¿Olvidaste tu contraseña?
                      </a>
                    }
                  />
                </FieldGroup>

                <SubmitButton isLoading={isLoading} label="Entrar" loadingLabel="Ingresando..." />
              </form>

              <FieldSeparator className="my-6">O continuar con</FieldSeparator>
              <SocialButtons />

              <div className="mt-6 flex justify-center">
                <Button
                  variant="ghost"
                  className="text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-transparent"
                  onClick={() => { logout(); navigate('/dashboard'); }}
                >
                  Continuar como invitado
                </Button>
              </div>
            </TabsContent>

            <TabsContent value="signup">
              <FormHeader title="Crea tu cuenta gratis" subtitle="Empieza a comparar precios en menos de un minuto." />

              <form className="flex flex-col gap-5" onSubmit={handleSignup}>
                {authError && tab === "signup" && <AuthErrorBanner message={authError} />}

                <FieldGroup>
                  <EmailField id="signup-email" value={signupEmail} onChange={setSignupEmail} error={errors.signupEmail} />
                  <PasswordField
                    id="signup-password"
                    value={signupPassword}
                    onChange={setSignupPassword}
                    error={errors.signupPassword}
                    placeholder="Crea una contraseña segura"
                    autoComplete="new-password"
                  >
                    <div className="mt-1">
                      <PasswordStrength password={signupPassword} />
                    </div>
                  </PasswordField>
                </FieldGroup>

                <div className="flex flex-col gap-3 mt-2 mb-2">
                  <CheckboxField id="terms" checked={acceptTerms} onChange={setAcceptTerms} error={errors.terms}>
                    He leído y acepto los{" "}
                    <Link to="/terminos" target="_blank" className="text-primary font-medium underline-offset-4 hover:underline">
                      Términos y Condiciones
                    </Link>
                  </CheckboxField>
                  <CheckboxField id="age" checked={acceptAge} onChange={setAcceptAge} error={errors.age}>
                    Confirmo que soy mayor de 18 años y tengo capacidad legal para aceptar estos términos
                  </CheckboxField>
                </div>

                <SubmitButton isLoading={isLoading} label="Registrarse" loadingLabel="Creando cuenta..." />
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
