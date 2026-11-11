// Evalúa la complejidad de una contraseña y muestra una orientación visual, no una validación del servidor.
import { cn } from "@/lib/utils"

function scorePassword(password: string) {
  let score = 0
  if (!password) return 0
  if (password.length >= 8) score++
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++
  if (/\d/.test(password)) score++
  if (/[^A-Za-z0-9]/.test(password)) score++
  return score
}

const LEVELS = [
  { label: "Muy débil", color: "bg-destructive" },
  { label: "Débil", color: "bg-destructive" },
  { label: "Aceptable", color: "bg-amber-500" },
  { label: "Fuerte", color: "bg-primary/80" },
  { label: "Excelente", color: "bg-primary" },
]

export function PasswordStrength({ password }: { password: string }) {
  const score = scorePassword(password)
  const level = LEVELS[score]
  const active = password.length > 0

  return (
    <div className="flex flex-col gap-2" aria-live="polite">
      <div className="flex gap-1.5">
        {Array.from({ length: 4 }).map((_, i) => (
          <span
            key={i}
            className={cn(
              "h-1.5 flex-1 rounded-full transition-colors duration-300",
              active && i < score ? level.color : "bg-border",
            )}
          />
        ))}
      </div>
      {active && (
        <p className="text-xs font-medium text-muted-foreground">
          Seguridad:{" "}
          <span
            className={cn(
              "font-semibold",
              score <= 1 && "text-destructive",
              score === 2 && "text-amber-600",
              score >= 3 && "text-primary",
            )}
          >
            {level.label}
          </span>
        </p>
      )}
    </div>
  )
}
