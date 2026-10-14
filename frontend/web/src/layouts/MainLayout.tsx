import { useEffect, useState } from "react";
import { Outlet, useOutletContext, useLocation } from "react-router-dom";
import { TopNav } from "@/components/TopNav";
import { SideBar } from "@/components/Sidebar";
import { CartSidebar } from "@/components/CartSidebar";
import BottomNav from "@/components/BottomNav";
import { getProducts, marketMeta, slugifyMarket } from "@/lib/products-api";
import type { UiSupermarket } from "@/types";
import { ShoppingBasket } from "lucide-react";
import { useCart } from "@/contexts/CartContext";
type LayoutContextType = {
  query: string;
  activeMarket: string | null;
  category: string;
  markets: UiSupermarket[];
};

export function useLayoutContext() {
  return useOutletContext<LayoutContextType>();
}

export function MainLayout() {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(window.innerWidth > 1024);
  const [activeMarket, setActiveMarket] = useState<string | null>(null);
  const [availableMarkets, setAvailableMarkets] = useState<UiSupermarket[]>([]);
  const { totalItems, totalPrice, setIsCartOpen } = useCart();
  const { pathname } = useLocation();
  const activeTab = 
    pathname.startsWith('/chat') ? 'chat' :
    pathname.startsWith('/history') ? 'history' :
    pathname.startsWith('/route') ? 'route' :
    pathname.startsWith('/colaborador') ? 'colaborador' :
    pathname.startsWith('/profile') ? 'profile' :
    'dashboard';

  useEffect(() => {
    let isMounted = true;

    getProducts('?limit=100&en_stock=true')
      .then(({ products }) => {
        if (!isMounted) return;

        // Cadenas obtenidas del catálogo scrapeado (no hardcodeadas):
        // cada producto trae su supermercado real y aquí solo se deducen.
        const markets = new Map<string, UiSupermarket>();
        products.forEach((product) => {
          if (!product.supermarketName || markets.has(product.supermarketId)) return;

          markets.set(product.supermarketId, {
            id: product.supermarketId,
            dbId: 0,
            name: product.supermarketName,
            ...marketMeta(product.supermarketName),
          });
        });
        setAvailableMarkets(Array.from(markets.values()));
      })
      .catch(() => {
        if (isMounted) setAvailableMarkets([]);
      });

    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <div className="flex h-dvh flex-col bg-background">
      <CartSidebar />
      <SideBar
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        activeMarket={activeMarket}
        availableMarkets={availableMarkets}
        onSelectCategory={(value) => {
          setQuery("");
          setCategory(value);
        }}
        onSelectMarket={(id) => {
          setActiveMarket(id);
          setSidebarOpen(false);
        }}
        activeCategory={category}
      />
      
      <TopNav
        query={query}
        onQueryChange={(value) => {
          setQuery(value);
          if (value.trim()) setCategory("");
        }}
        onMenuClick={() => setSidebarOpen(true)}
        onResetFilters={() => {
          setQuery("");
          setCategory("");
        }}
      />

      <div className="flex-1 flex flex-col overflow-y-auto overflow-x-hidden pb-[calc(4rem+env(safe-area-inset-bottom,0px))] md:pb-0">
        {/* Main Content Area */}
        <Outlet context={{ query, activeMarket, category, markets: availableMarkets } satisfies LayoutContextType} />
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
        <BottomNav active={activeTab} />
      </div>
    </div>
  );
}
