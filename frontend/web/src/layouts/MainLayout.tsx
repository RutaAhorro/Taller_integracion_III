// Comparte navegación, búsqueda y carrito; las cadenas se derivan de productos devueltos por Catálogo.
import { useCallback, useEffect, useState } from "react";
import { Outlet, useOutletContext, useLocation } from "react-router-dom";
import { TopNav } from "@/layouts/TopNav";
import { SideBar } from "@/layouts/Sidebar";
import { CartSidebar } from "@/features/catalogo/CartSidebar";
import BottomNav from "@/layouts/BottomNav";
import { getProducts, getMarketsFromProducts } from "@/features/catalogo/products-api";
import type { UiSupermarket } from "@/features/catalogo/tipos";
import { ShoppingBasket } from "lucide-react";
import { useCart } from "@/features/catalogo/CartContext";
type TipoContextoDiseno = {
  query: string;
  activeMarket: string | null;
  category: string;
  markets: UiSupermarket[];
};

export function useLayoutContext() {
  return useOutletContext<TipoContextoDiseno>();
}

export function MainLayout() {
  const [consulta, setConsulta] = useState("");
  const [categoria, setCategoria] = useState("");
  // El diálogo solo se abre por acción del usuario; no captura el foco al cargar.
  const [barraLateralAbierta, setBarraLateralAbierta] = useState(false);
  const [supermercadoActivo, setSupermercadoActivo] = useState<string | null>(null);
  const [supermercadosDisponibles, setSupermercadosDisponibles] = useState<UiSupermarket[]>([]);
  const cerrarBarraLateral = useCallback(() => setBarraLateralAbierta(false), []);
  const restablecerFiltros = () => {
    setConsulta("");
    setCategoria("");
    setSupermercadoActivo(null);
  };
  const { totalItems, totalPrice, setIsCartOpen } = useCart();
  const { pathname } = useLocation();
  const pestanaActiva = 
    pathname.startsWith('/chat') ? 'chat' :
    pathname.startsWith('/history') ? 'history' :
    pathname.startsWith('/route') ? 'route' :
    pathname.startsWith('/colaborador') ? 'colaborador' :
    pathname.startsWith('/profile') ? 'profile' :
    'dashboard';

  useEffect(() => {
    let estaMontado = true;

    getProducts('?limit=100&en_stock=true')
      .then(({ products }) => {
        if (!estaMontado) return;

        // Hasta SUP-371 solo se conocen las cadenas de esta página; no se descarga todo el catálogo.
        setSupermercadosDisponibles(getMarketsFromProducts(products));
      })
      .catch(() => {
        if (estaMontado) setSupermercadosDisponibles([]);
      });

    return () => {
      estaMontado = false;
    };
  }, []);

  return (
    <div className="flex h-dvh flex-col bg-background">
      <CartSidebar />
      <SideBar
        open={barraLateralAbierta}
        onClose={cerrarBarraLateral}
        onResetFilters={restablecerFiltros}
        activeMarket={supermercadoActivo}
        availableMarkets={supermercadosDisponibles}
        onSelectCategory={(valor) => {
          setConsulta("");
          setCategoria(valor);
        }}
        onSelectMarket={(id) => {
          setSupermercadoActivo(id);
          setBarraLateralAbierta(false);
        }}
        activeCategory={categoria}
      />
      
      <TopNav
        sidebarOpen={barraLateralAbierta}
        query={consulta}
        onQueryChange={(valor) => {
          setConsulta(valor);
          if (valor.trim()) setCategoria("");
        }}
        onMenuClick={() => setBarraLateralAbierta(true)}
        onResetFilters={restablecerFiltros}
      />

      <div className="flex-1 flex flex-col overflow-y-auto overflow-x-hidden pb-[calc(4rem+env(safe-area-inset-bottom,0px))] md:pb-0">
        <Outlet context={{ query: consulta, activeMarket: supermercadoActivo, category: categoria, markets: supermercadosDisponibles } satisfies TipoContextoDiseno} />
      </div>

      {totalItems > 0 && (
        <button
          onClick={() => setIsCartOpen(true)}
          className="fixed bottom-[5.5rem] right-4 z-50 flex items-center gap-2.5 rounded-full border border-primary-foreground/20 bg-primary px-4 py-2.5 text-primary-foreground shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all hover:-translate-y-1 hover:shadow-[0_8px_30px_rgb(0,0,0,0.2)] active:scale-95 md:bottom-6"
          aria-label="Ver carrito"
        >
          <div className="relative flex items-center justify-center">
            <ShoppingBasket className="h-5 w-5" />
            <span className="absolute -right-2 -top-2 flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-500 px-1 text-[9px] font-bold text-white shadow-sm ring-2 ring-background">
              {totalItems}
            </span>
          </div>
          <div className="ml-0.5 flex flex-col text-left">
            <span className="text-[9px] font-medium leading-none opacity-90">Ver Carrito</span>
            <span className="mt-0.5 text-xs font-bold leading-tight">${totalPrice.toLocaleString('es-CL')}</span>
          </div>
        </button>
      )}
      
      <div className="md:hidden">
        <BottomNav active={pestanaActiva} />
      </div>
    </div>
  );
}
