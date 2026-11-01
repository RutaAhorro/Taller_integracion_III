"use client"

import { useState } from "react"
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

export function AuthPanel() {
  const location = useLocation()
  const initialTab = location.state?.tab === "signup" ? "signup" : "login"
  const [tab, setTab] = useState(initialTab)

  const [loginEmail, setLoginEmail] = useState("")
  const [loginPassword, setLoginPassword] = useState("")

  const [signupEmail, setSignupEmail] = useState("")
  const [signupPassword, setSignupPassword] = useState("")

  const [showLoginPw, setShowLoginPw] = useState(false)
  const [showSignupPw, setShowSignupPw] = useState(false)
  const [acceptTerms, setAcceptTerms] = useState(false)
  const [acceptAge, setAcceptAge] = useState(false)
  const { login, register, logout, isLoading, error: authError } = useAuth()

  const [errors, setErrors] = useState<{ loginEmail?: string; loginPassword?: string; signupEmail?: string; signupPassword?: string; terms?: string; age?: string }>({})

  const navigate = useNavigate()
  const { toast } = useToast()

  const handleLogin = async (e?: React.FormEvent) => {
    e?.preventDefault()

    const newErrors: typeof errors = {}
    if (!loginEmail.includes("@")) {
      newErrors.loginEmail = "Ingresa un correo electrónico válido"
    }
    if (!loginPassword) {
      newErrors.loginPassword = "Contraseña requerida"
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors)
      return
    }

    setErrors({})

    const exito = await login({ correo: loginEmail, password: loginPassword })

    if (exito) {
      toast("Inicio de sesión exitoso", "success")
      navigate("/dashboard")
    }
  }

  const handleSignup = async (e?: React.FormEvent) => {
    e?.preventDefault()

    const newErrors: typeof errors = {}
    if (!signupEmail.includes("@")) {
      newErrors.signupEmail = "Ingresa un correo electrónico válido"
    }
    if (signupPassword.length < 8) {
      newErrors.signupPassword = "Usa al menos 8 caracteres para mayor seguridad"
    }
    if (!acceptTerms) {
      newErrors.terms = "Debes aceptar los Términos y Condiciones"
    }
    if (!acceptAge) {
      newErrors.age = "Debes confirmar que eres mayor de edad"
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors)
      return
    }

    setErrors({})

    // Generamos un sufijo a partir del correo para evitar IDs aleatorios si es posible
    const prefijoCorreo = signupEmail.split('@')[0];
    const nombreTemporal = `Usuario ${prefijoCorreo}`;

    const exito = await register({
      correo: signupEmail,
      password: signupPassword,
      nombre_completo: nombreTemporal
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

            {/* LOGIN */}
            <TabsContent value="login">
              <div className="mb-6">
                <h2 className="font-display text-2xl font-bold tracking-tight text-foreground text-balance">
                  Bienvenido de nuevo
                </h2>
                <p className="mt-1.5 text-sm text-muted-foreground">
                  Accede para seguir ahorrando en tus compras.
                </p>
              </div>

              <form className="flex flex-col gap-5" onSubmit={handleLogin}>
                {authError && tab === "login" && (
                  <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2">
                    <Lock className="size-4 shrink-0" /> {authError}
                  </div>
                )}

                <FieldGroup>
                  <Field data-invalid={!!errors.loginEmail}>
                    <FieldLabel htmlFor="login-email">Correo electrónico</FieldLabel>
                    <div className="relative">
                      <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        id="login-email"
                        type="email"
                        value={loginEmail}
                        onChange={(e) => setLoginEmail(e.target.value)}
                        placeholder="tu@correo.com"
                        autoComplete="email"
                        className={`h-11 rounded-xl pl-10 ${errors.loginEmail ? 'border-destructive border-2' : ''}`}
                      />
                    </div>
                    <div className="hidden sm:block">
                      {errors.loginEmail && <FieldError errors={[{ message: errors.loginEmail }]} />}
                    </div>
                  </Field>

                  <Field data-invalid={!!errors.loginPassword}>
                    <div className="flex items-center justify-between">
                      <FieldLabel htmlFor="login-password">Contraseña</FieldLabel>
                      <a
                        href="#"
                        className="text-xs font-medium text-primary underline-offset-4 hover:underline"
                      >
                        ¿Olvidaste tu contraseña?
                      </a>
                    </div>
                    <div className="relative">
                      <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        id="login-password"
                        type={showLoginPw ? "text" : "password"}
                        value={loginPassword}
                        onChange={(e) => setLoginPassword(e.target.value)}
                        placeholder="••••••••"
                        autoComplete="current-password"
                        className={`h-11 rounded-xl pr-10 pl-10 ${errors.loginPassword ? 'border-destructive border-2' : ''}`}
                      />
                      <button
                        type="button"
                        onClick={() => setShowLoginPw((s) => !s)}
                        aria-label={showLoginPw ? "Ocultar contraseña" : "Mostrar contraseña"}
                        className="cursor-pointer absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {showLoginPw ? (
                          <EyeOff className="size-4" />
                        ) : (
                          <Eye className="size-4" />
                        )}
                      </button>
                    </div>
                    <div className="hidden sm:block">
                      {errors.loginPassword && <FieldError errors={[{ message: errors.loginPassword }]} />}
                    </div>
                  </Field>
                </FieldGroup>

                <Button type="submit" disabled={isLoading} className="h-11 w-full rounded-xl text-sm font-semibold hover:brightness-110 transition-all">
                  {isLoading ? "Ingresando..." : "Entrar"}
                  {isLoading ? <Loader2 className="animate-spin" data-icon="inline-end" /> : <ArrowRight data-icon="inline-end" />}
                </Button>
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

            {/* SIGNUP */}
            <TabsContent value="signup">
              <div className="mb-6">
                <h2 className="font-display text-2xl font-bold tracking-tight text-foreground text-balance">
                  Crea tu cuenta gratis
                </h2>
                <p className="mt-1.5 text-sm text-muted-foreground">
                  Empieza a comparar precios en menos de un minuto.
                </p>
              </div>

              <form className="flex flex-col gap-5" onSubmit={handleSignup}>
                {authError && tab === "signup" && (
                  <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive font-semibold border-2 border-destructive flex items-center gap-2">
                    <Lock className="size-4 shrink-0" /> {authError}
                  </div>
                )}

                <FieldGroup>
                  <Field data-invalid={!!errors.signupEmail}>
                    <FieldLabel htmlFor="signup-email">Correo electrónico</FieldLabel>
                    <div className="relative">
                      <Mail className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        id="signup-email"
                        type="email"
                        value={signupEmail}
                        onChange={(e) => setSignupEmail(e.target.value)}
                        placeholder="tu@correo.com"
                        autoComplete="email"
                        className={`h-11 rounded-xl pl-10 ${errors.signupEmail ? 'border-destructive border-2' : ''}`}
                      />
                    </div>
                    <div className="hidden sm:block">
                      {errors.signupEmail && <FieldError errors={[{ message: errors.signupEmail }]} />}
                    </div>
                  </Field>

                  <Field data-invalid={!!errors.signupPassword}>
                    <FieldLabel htmlFor="signup-password">Contraseña</FieldLabel>
                    <div className="relative">
                      <Lock className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        id="signup-password"
                        type={showSignupPw ? "text" : "password"}
                        placeholder="Crea una contraseña segura"
                        autoComplete="new-password"
                        value={signupPassword}
                        onChange={(e) => setSignupPassword(e.target.value)}
                        className={`h-11 rounded-xl pr-10 pl-10 ${errors.signupPassword ? 'border-destructive border-2' : ''}`}
                      />
                      <button
                        type="button"
                        onClick={() => setShowSignupPw((s) => !s)}
                        aria-label={showSignupPw ? "Ocultar contraseña" : "Mostrar contraseña"}
                        className="cursor-pointer absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {showSignupPw ? (
                          <EyeOff className="size-4" />
                        ) : (
                          <Eye className="size-4" />
                        )}
                      </button>
                    </div>
                    <div className="hidden sm:block">
                      {errors.signupPassword && <FieldError errors={[{ message: errors.signupPassword }]} />}
                    </div>
                    <div className="mt-1">
                      <PasswordStrength password={signupPassword} />
                    </div>
                  </Field>
                </FieldGroup>

                <div className="flex flex-col gap-3 mt-2 mb-2">
                  <div className="flex flex-col gap-1.5">
                    <div className="flex items-start gap-2">
                      <input
                        type="checkbox"
                        id="terms"
                        checked={acceptTerms}
                        onChange={(e) => setAcceptTerms(e.target.checked)}
                        className={`mt-1 h-4 w-4 rounded bg-background focus:ring-primary focus:ring-offset-2 shrink-0 cursor-pointer ${errors.terms ? 'border-destructive border-2 text-destructive' : 'border-border text-primary'}`}
                      />
                      <label htmlFor="terms" className={`text-sm leading-snug cursor-pointer select-none ${errors.terms ? 'text-destructive font-medium' : 'text-muted-foreground'}`}>
                        He leído y acepto los{" "}
                        <Link to="/terminos" target="_blank" className="text-primary font-medium underline-offset-4 hover:underline">
                          Términos y Condiciones
                        </Link>
                      </label>
                    </div>
                    <div className="hidden sm:block">
                      {errors.terms && <span className="text-xs text-destructive font-medium pl-6">{errors.terms}</span>}
                    </div>
                  </div>

                  <div className="flex flex-col gap-1.5">
                    <div className="flex items-start gap-2">
                      <input
                        type="checkbox"
                        id="age"
                        checked={acceptAge}
                        onChange={(e) => setAcceptAge(e.target.checked)}
                        className={`mt-1 h-4 w-4 rounded bg-background focus:ring-primary focus:ring-offset-2 shrink-0 cursor-pointer ${errors.age ? 'border-destructive border-2 text-destructive' : 'border-border text-primary'}`}
                      />
                      <label htmlFor="age" className={`text-sm leading-snug cursor-pointer select-none ${errors.age ? 'text-destructive font-medium' : 'text-muted-foreground'}`}>
                        Confirmo que soy mayor de 18 años y tengo capacidad legal para aceptar estos términos
                      </label>
                    </div>
                    <div className="hidden sm:block">
                      {errors.age && <span className="text-xs text-destructive font-medium pl-6">{errors.age}</span>}
                    </div>
                  </div>
                </div>

                <Button type="submit" disabled={isLoading} className="h-11 w-full rounded-xl text-sm font-semibold hover:brightness-110 transition-all">
                  {isLoading ? "Creando cuenta..." : "Registrarse"}
                  {isLoading ? <Loader2 className="animate-spin" data-icon="inline-end" /> : <ArrowRight data-icon="inline-end" />}
                </Button>
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
