// Distribuye la bienvenida y los formularios de acceso o registro según el tamaño de pantalla.
import { AuthVisual } from "@/components/auth/AuthVisual"
import { AuthPanel } from "@/components/auth/AuthPanel"
import { Navigate } from "react-router-dom"
import { useRole } from "@/hooks/useRole"
import { ROUTES } from "@/core/routes"

export default function Page() {
  const { isGuest, isAdmin } = useRole()

  // Espera el usuario actualizado por Identidad; ambos métodos de login comparten destino.
  if (!isGuest) {
    return <Navigate to={isAdmin ? ROUTES.ADMIN : ROUTES.CATALOGO} replace />
  }

  return (
    <main className="grid h-screen w-full grid-cols-1 overflow-hidden lg:grid-cols-2">
      <AuthVisual />
      <AuthPanel />
    </main>
  )
}
