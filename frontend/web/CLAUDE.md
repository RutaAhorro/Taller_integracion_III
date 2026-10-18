# Frontend: React 19 + Vite + TypeScript + Tailwind 4

Puerto: 3000 | Dockerfile: Dockerfile | Entry: npm run dev

## Estructura activa de src/

```
src/
├── main.tsx                    → Entry point (ReactDOM.createRoot)
├── App.tsx                     → BrowserRouter + Routes
├── layouts/MainLayout.tsx      → Layout con Sidebar + TopNav + Outlet context
├── pages/
│   ├── Home.tsx                → Landing page (ruta /)
│   ├── Login.tsx               → Auth con panel visual (/login)
│   ├── Onboarding.tsx          → Wizard de configuración (/onboarding)
│   ├── Dashboard.tsx           → Comparador de precios (/dashboard)
│   ├── Chatbot.tsx             → Asistente IA (/chat)
│   ├── RouteViewer.tsx         → Mapa interactivo Leaflet (/route)
│   ├── Profile.tsx             → Config de usuario (/profile)
│   ├── Crowdsourcing.tsx       → Misiones gamificadas (/colaborador)
│   └── History.tsx             → Historial de listas (/history)
├── components/
│   ├── ui/                     → Primitives: button, input, field, label, separator, tabs
│   ├── auth/                   → AuthPanel, AuthVisual, PasswordStrength, SocialButtons
│   ├── onboarding/             → OnboardingWizard, ProgressSteps, StepLocation, StepSupermarkets, StepDiet
│   ├── ProductCard.tsx         → Tarjeta de producto con descuentos
│   ├── RouteMap.tsx            → Mapa Leaflet embebido (carga dinámica)
│   ├── Sidebar.tsx             → Navegación lateral con filtros de supermercado
│   ├── TopNav.tsx              → Barra superior con búsqueda
│   └── BottomNav.tsx           → [HUÉRFANO] Nav inferior móvil, no montado
├── data/mock.ts                → Datos ficticios: usuarios, supermercados, productos (Temuco)
├── types/index.ts              → Tipos TS del dominio (Usuario, UiProduct, UiSupermarket, etc.)
└── lib/utils.ts                → cn() helper para clases Tailwind
```

## Rutas (React Router DOM v7)

| Ruta | Componente | Layout |
|---|---|---|
| `/` | Home | Sin layout |
| `/login` | Login | Sin layout |
| `/onboarding` | Onboarding | Sin layout |
| `/dashboard` | Dashboard | MainLayout |
| `/chat` | Chatbot | MainLayout |
| `/route` | RouteViewer | MainLayout |
| `/profile` | Profile | MainLayout |
| `/colaborador` | Crowdsourcing | MainLayout |
| `/history` | History | MainLayout |

## State Management

- Sin stores externos (no Redux/Zustand)
- `useState` para estado local
- `useOutletContext<LayoutContextType>()` para comunicar Layout ↔ Vistas
- `useLayoutContext()` hook custom en MainLayout.tsx (exporta query + activeMarket)
- `useMemo` en Dashboard para filtrado en tiempo real y cálculo TSP greedy

## REGLAS IMPORTANTES

- **SOLO usar archivos PascalCase**: ProductCard.tsx, Sidebar.tsx, TopNav.tsx, AuthPanel.tsx, etc.
- **IGNORAR archivos kebab-case**: product-card.tsx, side-bar.tsx, auth-panel.tsx son obsoletos (v0)
- **IGNORAR src/lib/data.ts**: redundante con src/data/mock.ts, solo usado por componentes obsoletos
- UI primitives basadas en @base-ui/react + class-variance-authority
- Estilos: Tailwind 4 + tw-animate-css, utilidad cn() de tailwind-merge + clsx
- Iconos: lucide-react exclusivamente
- Path alias: `@` → `src/` (configurado en vite.config.ts y tsconfig)

## Dependencias clave

- react-leaflet + leaflet: mapas geoespaciales
- @radix-ui/react-*: primitivas accesibles (label, separator, slot, tabs)
- shadcn (CLI): generador de componentes UI

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Frontend (React + TypeScript)

**77 archivos activos** (excluidos kebab-case obsoletos)


### `core/routes.ts`

- Exports: MOCK_ROUTES, ROUTES

### `data/mock.ts`

- Exports: HOME, discountPct, formatPrice, mockUser, products, supermarketById, supermarkets
- Imports locales: ../types

### `hooks/useAuth.ts`

- Exports: useAuth
- Imports locales: ../contexts/AuthContext

### `hooks/useCatalog.ts`

- Exports: useCatalog
- Imports locales: @/lib/constants, @/lib/products-api, @/types

### `hooks/useChatbot.ts`

- Exports: useChatbot
- Tipos: Message, MessageType
- Imports locales: @/hooks/useAuth

### `hooks/useGoogleAuth.ts`

- Exports: useGoogleAuth
- Imports locales: ./useAuth

### `hooks/useRouteOptimization.ts`

- Exports: useRouteOptimization
- Tipos: OptimizedStore
- Imports locales: @/data/mock

### `layouts/useActiveTab.ts`

- Exports: useActiveTab
- Tipos: ActiveTab

### `lib/constants.ts`

- Exports: HOME, supermarketById, supermarkets
- Imports locales: ../types

### `lib/data.ts`

- Exports: HOME, discountPct, formatPrice, products, supermarketById, supermarkets
- Tipos: Product, Supermarket

### `lib/formatters.ts`

- Exports: calculateDiscountPct, calculateSavings, formatPrice

### `lib/logger.ts`

- Exports: logger

### `lib/products-api.ts`

- Exports: toUiProduct
- Tipos: ApiPriceHistory, ApiProduct, ApiProductDetail, ProductPage
- Imports locales: ./constants, @/types

### `lib/utils.ts`

- Exports: cn
- Tipos: ClassValue

### `types/index.ts`

- Tipos: CadenaSupermercado, CapturaPrecio, Categoria, LoginPayload, Marca, PreferenciasDieteticas, ProductoCrudo, ProductoNormalizado, RegisterPayload, SucursalSupermercado, UiProduct, UiSupermarket, Usuario

### `app/router.tsx`

- Exports: AppRouter
- Imports locales: @/components/ErrorBoundary, @/components/auth/ProtectedRoute, @/contexts/AuthContext, @/contexts/CartContext, @/contexts/ToastContext, @/core/routes, @/layouts/MainLayout, @/pages/Chatbot, @/pages/Crowdsourcing, @/pages/Dashboard, @/pages/History, @/pages/Home, @/pages/Login, @/pages/NotFound, @/pages/Onboarding, @/pages/Planes, @/pages/ProductDetail, @/pages/Profile, @/pages/RouteViewer, @/pages/Terms, @/pages/mocks/MockShell

### `components/BottomNav.tsx`

- Exports: BottomNav
- Tipos: BottomNavProps
- Imports locales: @/lib/utils

### `components/CartSidebar.tsx`

- Exports: CartSidebar
- Imports locales: @/components/ui/button, @/contexts/CartContext, @/lib/formatters

### `components/ErrorBoundary.tsx`

- Exports: ErrorBoundary
- Imports locales: @/lib/logger

### `components/Footer.tsx`

- Exports: Footer

### `components/ProductCard.tsx`

- Exports: ProductCard
- Tipos: ProductCardProps
- Imports locales: @/components/ui/button, @/core/routes, @/lib/formatters, @/lib/utils, @/types

### `components/ProductCarousel.tsx`

- Exports: ProductCarousel
- Imports locales: @/components/ProductCard, @/types

### `components/RouteMap.tsx`

- Exports: RouteMap
- Tipos: MapStop, RouteMapProps

### `components/Sidebar.tsx`

- Exports: SideBar
- Tipos: SideBarProps
- Imports locales: @/components/ui/button, @/types

### `components/TopNav.tsx`

- Exports: TopNav
- Tipos: TopNavProps
- Imports locales: @/components/ui/button, @/core/routes, @/hooks/useAuth

### `components/auth/AuthPanel.tsx`

- Exports: AuthPanel
- Imports locales: @/components/auth/PasswordStrength, @/components/auth/SocialButtons, @/components/ui/button, @/components/ui/field, @/components/ui/input, @/components/ui/tabs, @/contexts/ToastContext, @/hooks/useAuth

### `components/auth/AuthVisual.tsx`

- Exports: AuthVisual

### `components/auth/PasswordStrength.tsx`

- Exports: PasswordStrength
- Imports locales: @/lib/utils

### `components/auth/ProtectedRoute.tsx`

- Exports: ProtectedRoute
- Imports locales: @/hooks/useAuth

### `components/auth/SocialButtons.tsx`

- Exports: SocialButtons
- Imports locales: @/hooks/useGoogleAuth

### `components/chatbot/OutOfStockAlert.tsx`

- Exports: OutOfStockAlert
- Tipos: OutOfStockAlertProps
- Imports locales: @/lib/utils

### `components/chatbot/RichRecipeCard.tsx`

- Exports: RichRecipeCard
- Tipos: RecipeItem, RichRecipeCardProps
- Imports locales: @/lib/formatters, @/lib/utils

### `components/chatbot/TypingIndicator.tsx`

- Exports: TypingIndicator
- Imports locales: @/lib/utils

### `components/landing/LandingCTA.tsx`

- Exports: LandingCTA
- Imports locales: @/components/ui/button

### `components/landing/LandingFeatures.tsx`

- Exports: LandingFeatures
- Imports locales: @/data/landing.json

### `components/landing/LandingFooter.tsx`

- Exports: LandingFooter

### `components/landing/LandingHero.tsx`

- Exports: LandingHero
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingNavbar.tsx`

- Exports: LandingNavbar
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingSavings.tsx`

- Exports: LandingSavings
- Imports locales: @/components/ui/button, @/data/landing.json

### `components/landing/LandingStores.tsx`

- Exports: LandingStores
- Imports locales: @/data/landing.json

### `components/landing/LandingTrustStrip.tsx`

- Exports: LandingTrustStrip
- Imports locales: @/data/landing.json

### `components/landing/LandingWorks.tsx`

- Exports: LandingHowItWorks
- Imports locales: @/data/landing.json

### `components/onboarding/OnboardingWizard.tsx`

- Exports: OnboardingWizard
- Imports locales: ./ProgressSteps, ./StepDiet, ./StepLocation, ./StepProfile, ./StepSupermarkets, @/lib/utils

### `components/onboarding/ProgressSteps.tsx`

- Exports: ProgressSteps
- Tipos: ProgressStepsProps
- Imports locales: @/lib/utils

### `components/onboarding/StepDiet.tsx`

- Exports: StepDiet
- Tipos: StepDietProps, Tag
- Imports locales: @/lib/utils

### `components/onboarding/StepLocation.tsx`

- Exports: StepLocation
- Tipos: StepLocationProps

### `components/onboarding/StepProfile.tsx`

- Exports: StepProfile
- Tipos: StepProfileProps
- Imports locales: @/lib/utils

### `components/onboarding/StepSupermarkets.tsx`

- Exports: StepSupermarkets
- Tipos: Chain, StepSupermarketsProps
- Imports locales: @/lib/utils

### `components/ui/ProductSkeleton.tsx`

- Exports: ProductSkeleton
- Imports locales: @/components/ui/skeleton

### `components/ui/SuccessCard.tsx`

- Exports: SuccessCard
- Tipos: SuccessCardProps
- Imports locales: @/lib/utils

### `components/ui/button.tsx`

- Tipos: VariantProps
- Imports locales: @/lib/utils

### `components/ui/field.tsx`

- Tipos: VariantProps
- Imports locales: @/components/ui/label, @/components/ui/separator, @/lib/utils

### `components/ui/tabs.tsx`

- Tipos: VariantProps
- Imports locales: @/lib/utils

### `contexts/AuthContext.tsx`

- Exports: AuthContext, AuthProvider
- Tipos: AuthContextType, ReactNode
- Imports locales: @/types

### `contexts/CartContext.tsx`

- Exports: CartProvider, useCart
- Tipos: CartContextType, CartItem
- Imports locales: @/contexts/ToastContext, @/types

### `contexts/ToastContext.tsx`

- Exports: ToastProvider, useToast
- Tipos: ReactNode, Toast, ToastContextType, ToastType
- Imports locales: @/lib/utils

### `layouts/AppShell.tsx`

- Exports: AppShell
- Tipos: ActiveTab
- Imports locales: @/components/BottomNav

### `layouts/MainLayout.tsx`

- Exports: MainLayout, useLayoutContext
- Tipos: LayoutContextType
- Imports locales: @/components/BottomNav, @/components/CartSidebar, @/components/Sidebar, @/components/TopNav, @/contexts/CartContext, @/lib/products-api, @/types

### `pages/Chatbot.tsx`

- Exports: Chatbot
- Imports locales: @/components/chatbot/OutOfStockAlert, @/components/chatbot/RichRecipeCard, @/components/chatbot/TypingIndicator, @/contexts/ToastContext, @/hooks/useChatbot, @/lib/utils

### `pages/Crowdsourcing.tsx`

- Exports: Crowdsourcing
- Imports locales: @/components/Footer, @/components/ui/SuccessCard, @/contexts/ToastContext, @/hooks/useAuth, @/lib/formatters, @/lib/utils

### `pages/Dashboard.tsx`

- Exports: Page
- Imports locales: @/components/Footer, @/components/ProductCard, @/components/ui/ProductSkeleton, @/contexts/CartContext, @/hooks/useCatalog, @/layouts/MainLayout, @/types

### `pages/History.tsx`

- Exports: History
- Imports locales: @/components/Footer, @/lib/utils

### `pages/Home.tsx`

- Exports: Home
- Imports locales: @/components/landing/LandingCTA, @/components/landing/LandingFeatures, @/components/landing/LandingFooter, @/components/landing/LandingHero, @/components/landing/LandingNavbar, @/components/landing/LandingSavings, @/components/landing/LandingStores, @/components/landing/LandingTrustStrip, @/components/landing/LandingWorks

### `pages/Login.tsx`

- Exports: Page
- Imports locales: @/components/auth/AuthPanel, @/components/auth/AuthVisual

### `pages/NotFound.tsx`

- Exports: NotFound
- Imports locales: @/components/ui/button, @/core/routes

### `pages/Onboarding.tsx`

- Exports: Page
- Imports locales: @/components/onboarding/OnboardingWizard

### `pages/Planes.tsx`

- Exports: Planes
- Imports locales: @/hooks/useAuth, @/pages/mocks/MockShell

### `pages/ProductDetail.tsx`

- Exports: ProductDetail
- Imports locales: @/components/Footer, @/components/ProductCarousel, @/components/ui/button, @/contexts/CartContext, @/lib/formatters, @/lib/products-api, @/types

### `pages/Profile.tsx`

- Exports: Profile
- Imports locales: @/hooks/useAuth, @/lib/utils, @/pages/mocks/MockShell

### `pages/RouteViewer.tsx`

- Exports: RouteViewer
- Tipos: as
- Imports locales: @/components/ui/skeleton, @/hooks/useRouteOptimization, @/lib/utils

### `pages/Terms.tsx`

- Exports: Terms
- Imports locales: @/components/ui/button

### `pages/mocks/MockShell.tsx`

- Exports: MockShell
- Tipos: MockShellProps
