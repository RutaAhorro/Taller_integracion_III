// Distribuye la bienvenida y los formularios de acceso o registro según el tamaño de pantalla.
import { AuthVisual } from "@/components/auth/AuthVisual"
import { AuthPanel } from "@/components/auth/AuthPanel"

export default function Page() {
  return (
    <main className="grid h-screen w-full grid-cols-1 overflow-hidden lg:grid-cols-2">
      <AuthVisual />
      <AuthPanel />
    </main>
  )
}
