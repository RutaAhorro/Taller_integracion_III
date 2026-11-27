// Comparte filtros, productos y facetas para que el lateral y el listado usen la misma consulta.
import { useCallback, useEffect, useState } from "react";
import { Outlet, useOutletContext, useLocation } from "react-router-dom";
import { TopNav } from "@/layouts/TopNav";
import { SideBar } from "@/layouts/Sidebar";
import { CartSidebar } from "@/features/catalogo/CartSidebar";
import BottomNav from "@/layouts/BottomNav";
import { useCatalog } from "@/features/catalogo/useCatalog";
import { obtenerCategorias } from "@/features/catalogo/products-api";
import type { UiSupermarket } from "@/features/catalogo/tipos";
import { ShoppingBasket } from "lucide-react";
import { useCart } from "@/features/catalogo/CartContext";
type TipoContextoDiseno = {
  query: string;
  activeMarket: string | null;
  activeMarketName: string | null;
  category: string;
  markets: UiSupermarket[];
  categorias: string[];
  cargandoCategorias: boolean;
  errorCategorias: boolean;
  reintentarCategorias: () => void;
  catalogo: ReturnType<typeof useCatalog>;
  onSelectCategory: (categoria: string) => void;
  onResetFilters: () => void;
  resetVersion: number;
};

export function useLayoutContext() {
  return useOutletContext<TipoContextoDiseno>();
}

// Ciudad de referencia compartida entre la cabecera y el menú lateral.
const CIUDAD_ACTIVA = "Temuco";

export function MainLayout() {
  const [versionReinicio, setVersionReinicio] = useState(0);
  const [consulta, setConsulta] = useState("");
  const [categoria, setCategoria] = useState("");
  // El diálogo solo se abre por acción del usuario; no captura el foco al cargar.
  const [barraLateralAbierta, setBarraLateralAbierta] = useState(false);
  // El nombre seleccionado no depende de las facetas que se vacían durante una carga.
  const [supermercadoActivo, setSupermercadoActivo] =
    useState<UiSupermarket | null>(null);
  const { pathname, search } = useLocation();
  const vistaCatalogoActiva = new URLSearchParams(search).get("vista") === "catalogo";
  const catalogoHabilitado = Boolean(
    consulta.trim() || categoria || supermercadoActivo || vistaCatalogoActiva || barraLateralAbierta,
  );
  const catalogo = useCatalog(consulta, categoria, supermercadoActivo, catalogoHabilitado);
  const [categorias, setCategorias] = useState<string[]>([]);
  const [cargandoCategorias, setCargandoCategorias] = useState(true);
  const [errorCategorias, setErrorCategorias] = useState(false);
  const [intentoCategorias, setIntentoCategorias] = useState(0);

  // Limpia la búsqueda al salir del catálogo y sus fichas para no arrastrarla a otras vistas.
  useEffect(() => {
    if (!pathname.startsWith("/dashboard") && !pathname.startsWith("/producto/")) {
      setConsulta("");
    }
  }, [pathname]);

  useEffect(() => {
    // Sólo una respuesta confirmada puede invalidar la selección, nunca una carga o un error.
    if (
      catalogo.marketsReady &&
      supermercadoActivo &&
      !catalogo.markets.some(
        (supermercado) => supermercado.id === supermercadoActivo.id,
      )
    ) {
      setSupermercadoActivo(null);
    }
  }, [catalogo.marketsReady, catalogo.markets, supermercadoActivo]);

  useEffect(() => {
    // Una sola carga de categorías comparte estado con el lateral y la portada del catálogo.
    const controlador = new AbortController();
    setCargandoCategorias(true);
    setErrorCategorias(false);
    obtenerCategorias(controlador.signal)
      .then((resultado) => {
        if (!controlador.signal.aborted) setCategorias(resultado);
      })
      .catch(() => {
        if (!controlador.signal.aborted) {
          setCategorias([]);
          setErrorCategorias(true);
        }
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargandoCategorias(false);
      });
    return () => controlador.abort();
  }, [intentoCategorias]);
  const cerrarBarraLateral = useCallback(
    () => setBarraLateralAbierta(false),
    [],
  );
  const restablecerFiltros = () => {
    setConsulta("");
    setCategoria("");
    setSupermercadoActivo(null);
    setVersionReinicio((anterior) => anterior + 1);
  };
  // Comparte la misma selección entre el lateral y la portada del catálogo.
  const seleccionarCategoria = useCallback((valor: string) => {
    setConsulta("");
    setCategoria(valor);
  }, []);
  const { totalItems, totalPrice, isCartOpen, setIsCartOpen } = useCart();
  const pestanaActiva = pathname.startsWith("/chat")
    ? "chat"
    : pathname.startsWith("/listas")
      ? "history"
      : pathname.startsWith("/route")
        ? "route"
        : pathname.startsWith("/colaborador")
          ? "colaborador"
          : pathname.startsWith("/profile")
            ? "profile"
            : "dashboard";

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-background">
      <CartSidebar />
      <SideBar
        activeCity={CIUDAD_ACTIVA}
        open={barraLateralAbierta}
        onClose={cerrarBarraLateral}
        onResetFilters={restablecerFiltros}
        activeMarket={supermercadoActivo?.id ?? null}
        availableMarkets={catalogo.markets}
        cargandoSupermercados={catalogo.isLoading}
        errorSupermercados={catalogo.error !== null}
        avisoSupermercados={catalogo.notice}
        alReintentarSupermercados={catalogo.retry}
        onSelectCategory={seleccionarCategoria}
        onSelectMarket={(id) => {
          setSupermercadoActivo(
            catalogo.markets.find((supermercado) => supermercado.id === id) ??
              null,
          );
          setBarraLateralAbierta(false);
        }}
        activeCategory={categoria}
        categoriasDisponibles={categorias}
        cargandoCategorias={cargandoCategorias}
        errorCategorias={errorCategorias}
        alReintentarCategorias={() => setIntentoCategorias((anterior) => anterior + 1)}
      />

      <TopNav
        activeCity={CIUDAD_ACTIVA}
        sidebarOpen={barraLateralAbierta}
        query={consulta}
        onQueryChange={(valor) => {
          setConsulta(valor);
          if (valor.trim()) setCategoria("");
        }}
        onMenuClick={() => setBarraLateralAbierta(true)}
        onResetFilters={restablecerFiltros}
      />

      <div className="min-h-0 flex-1 flex flex-col overflow-y-auto overflow-x-hidden pb-[calc(4rem+env(safe-area-inset-bottom,0px))] md:pb-0">
        <Outlet
          context={
            {
              query: consulta,
              activeMarket: supermercadoActivo?.id ?? null,
              activeMarketName: supermercadoActivo?.name ?? null,
              category: categoria,
              markets: catalogo.markets,
              categorias,
              cargandoCategorias,
              errorCategorias,
              reintentarCategorias: () => setIntentoCategorias((anterior) => anterior + 1),
              catalogo,
              onSelectCategory: seleccionarCategoria,
              onResetFilters: restablecerFiltros,
              resetVersion: versionReinicio,
            } satisfies TipoContextoDiseno
          }
        />
      </div>

      {!isCartOpen && (
        <button
          onClick={() => setIsCartOpen(true)}
          className={`fixed ${pathname.startsWith("/chat") ? "bottom-[calc(11.25rem+env(safe-area-inset-bottom,0px))]" : "bottom-[calc(5.5rem+env(safe-area-inset-bottom,0px))]"} right-4 z-50 flex items-center gap-2.5 rounded-full border border-primary-foreground/20 bg-primary px-4 py-2.5 text-primary-foreground shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all hover:-translate-y-1 hover:shadow-[0_8px_30px_rgb(0,0,0,0.2)] active:scale-95 md:bottom-6`}
          aria-label="Ver carrito"
        >
          <div className="relative flex items-center justify-center">
            <ShoppingBasket className="h-5 w-5" />
            <span className="absolute -right-2 -top-2 flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-500 px-1 text-[9px] font-bold text-white shadow-sm ring-2 ring-background">
              {totalItems}
            </span>
          </div>
          <div className="ml-0.5 flex flex-col text-left">
            <span className="text-[9px] font-medium leading-none opacity-90">
              Ver Carrito
            </span>
            <span className="mt-0.5 text-xs font-bold leading-tight">
              {totalItems === 0
                ? "Revisar carrito"
                : totalPrice === null
                  ? "Sin total vigente"
                  : `$${totalPrice.toLocaleString("es-CL")}`}
            </span>
          </div>
        </button>
      )}

      <div className="md:hidden">
        <BottomNav active={pestanaActiva} />
      </div>
    </div>
  );
}
