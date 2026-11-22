// Panel visual de bienvenida que acompaña los formularios de autenticación.
import { ShoppingBasket } from "lucide-react"
import { Link } from "react-router-dom"

export function AuthVisual() {
  return (
    <div className="relative hidden overflow-hidden lg:flex lg:flex-col lg:justify-between bg-primary text-primary-foreground transition-colors duration-500">

      <Link to="/" className="relative z-10 flex w-fit items-center gap-2.5 p-10 hover:opacity-80 transition-opacity">
        <span className="flex size-10 items-center justify-center rounded-xl bg-primary-foreground text-primary border-2 border-primary-foreground">
          <ShoppingBasket className="size-5" />
        </span>
        <span className="font-display text-lg font-bold tracking-tight">
          RutaAhorro
        </span>
      </Link>

      <div className="relative z-10 flex flex-col gap-10 px-10 pb-4">
        <div className="max-w-md">
          <h1 className="font-display text-4xl leading-[1.1] font-extrabold tracking-tight text-balance xl:text-5xl">
            Descubre la ruta más barata para tus compras
          </h1>
          <p className="mt-5 max-w-sm text-base leading-relaxed opacity-80">
            Consulta productos del catálogo y organiza tus listas de compras.
            La integración del cálculo de rutas está en desarrollo.
          </p>
        </div>

      </div>

      <footer className="relative z-10 flex items-center gap-6 p-10 text-xs font-bold opacity-70">
        <span>Proyecto académico en desarrollo</span>
        <span className="size-1.5 rounded-full bg-primary-foreground" />
        <Link to="/" className="hover:opacity-100 transition-opacity">
          ¿Que ofrecemos?
        </Link>
      </footer>
    </div>
  )
}
