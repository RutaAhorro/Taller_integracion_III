// Evalúa la complejidad de una contraseña y muestra una orientación visual, no una validación del servidor.
import { cn } from "@/lib/utils"

function evaluarContrasena(contrasena: string) {
  let puntaje = 0
  if (!contrasena) return 0
  if (contrasena.length >= 8) puntaje++
  if (/[a-z]/.test(contrasena) && /[A-Z]/.test(contrasena)) puntaje++
  if (/\d/.test(contrasena)) puntaje++
  if (/[^A-Za-z0-9]/.test(contrasena)) puntaje++
  return puntaje
}

const NIVELES = [
  { label: "Muy débil", color: "bg-destructive" },
  { label: "Débil", color: "bg-destructive" },
  { label: "Aceptable", color: "bg-amber-500" },
  { label: "Fuerte", color: "bg-primary/80" },
  { label: "Excelente", color: "bg-primary" },
]

export function PasswordStrength({ password }: { password: string }) {
  const puntaje = evaluarContrasena(password)
  const nivel = NIVELES[puntaje]
  const activa = password.length > 0

  return (
    <div className="flex flex-col gap-2" aria-live="polite">
      <div className="flex gap-1.5">
        {Array.from({ length: 4 }).map((_, indice) => (
          <span
            key={indice}
            className={cn(
              "h-1.5 flex-1 rounded-full transition-colors duration-300",
              activa && indice < puntaje ? nivel.color : "bg-border",
            )}
          />
        ))}
      </div>
      {activa && (
        <p className="text-xs font-medium text-muted-foreground">
          Seguridad:{" "}
          <span
            className={cn(
              "font-semibold",
              puntaje <= 1 && "text-destructive",
              puntaje === 2 && "text-amber-600",
              puntaje >= 3 && "text-primary",
            )}
          >
            {nivel.label}
          </span>
        </p>
      )}
    </div>
  )
}
