// Define las rutas públicas y protegidas y reúne los proveedores de la aplicación.
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { GoogleOAuthProvider } from '@react-oauth/google';
import { ROUTES } from '@/app/routes';

// Estructuras compartidas de navegación y sesión.
import { MainLayout } from '@/layouts/MainLayout';
import { CartProvider } from '@/features/catalogo/CartContext';
import { ToastProvider } from '@/components/ToastContext';
import { AuthProvider } from '@/features/autenticacion/AuthContext';
import { ProtectedRoute } from '@/features/autenticacion/ProtectedRoute';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ROLE_ADMIN, ROLE_COLABORADOR } from '@/features/autenticacion/roles';

// Sprint 1 — vistas funcionales
import Home from '@/features/landing/Home';
import Dashboard from '@/features/catalogo/Dashboard';
import ProductDetail from '@/features/catalogo/ProductDetail';
import Login from '@/features/autenticacion/Login';
import VerificarEmail from '@/features/autenticacion/VerificarEmail';
import Onboarding from '@/pages/Onboarding';
import Profile from '@/features/autenticacion/Profile';
import ShoppingLists from '@/features/listas/ShoppingLists';
import NotFound from '@/pages/NotFound';
import Terms from '@/pages/Terms';

// Vistas integradas o con estado explícito de no disponibilidad.
import Chatbot from '@/features/chat/Chatbot';
import Crowdsourcing from '@/pages/Crowdsourcing';
import RutaLista from '@/features/rutas/RutaLista';
import History from '@/pages/History';
import Planes from '@/pages/Planes';
import AdminDashboard from '@/features/admin/AdminDashboard';


const enrutador = createBrowserRouter([
  // ───────────────────────────────────────────────────────────
  // Grupo A: MainLayout (TopNav + SideBar + BottomNav)
  // Para vistas que se benefician del buscador y filtros por supermercado
  // ───────────────────────────────────────────────────────────
  {
    element: <MainLayout />,
    errorElement: <ErrorBoundary />,
    children: [
      // Sprint 1
      { path: ROUTES.CATALOGO, element: <Dashboard /> },
      { path: '/producto/:id', element: <ProductDetail /> },

      // Sprint 2+ (con SideBar disponible)
      {
        path: ROUTES.CHAT,
        element: <Chatbot />,
      },
    ],
  },

  // ───────────────────────────────────────────────────────────
  // Grupo B: navegación compartida y páginas protegidas con cabecera propia.
  // ───────────────────────────────────────────────────────────
  {
    element: <MainLayout />,
    errorElement: <ErrorBoundary />,
    children: [
      {
        element: <ProtectedRoute />,
        children: [
          { path: ROUTES.PERFIL, element: <Profile /> },
          { path: ROUTES.LISTAS, element: <ShoppingLists /> },
          { path: ROUTES.PLANES, element: <Planes /> },
          {
            // SUP-256: /colaborador solo colaborador o admin.
            element: <ProtectedRoute roles={[ROLE_COLABORADOR, ROLE_ADMIN]} />,
            children: [
              {
                path: ROUTES.COLABORADOR,
                element: <Crowdsourcing />,
              },
            ],
          },
          {
            path: ROUTES.RUTA,
            element: <RutaLista />,
          },
          {
            path: ROUTES.HISTORIAL,
            element: <History />,
          },
        ]
      }
    ],
  },

  // ───────────────────────────────────────────────────────────
  // Grupo C: sin estructura compartida (pantalla completa).
  // ───────────────────────────────────────────────────────────
  { path: ROUTES.HOME, element: <Home />, errorElement: <ErrorBoundary /> },
  { path: ROUTES.LOGIN, element: <Login /> },
  { path: ROUTES.VERIFICAR_EMAIL, element: <VerificarEmail /> },
  { path: ROUTES.TERMINOS, element: <Terms /> },
  {
    // SUP-256: /admin solo admin. Invitado ve acceso (ir a login);
    // logueado sin el rol redirige al inicio autenticado (/dashboard).
    element: <ProtectedRoute roles={[ROLE_ADMIN]} />,
    errorElement: <ErrorBoundary />,
    children: [
      { path: ROUTES.ADMIN, element: <AdminDashboard /> },
      // SUP-280: alias /admin/metricas que abre el panel en la sección Métricas.
      { path: ROUTES.ADMIN_METRICAS, element: <AdminDashboard /> },
    ],
  },
  { 
    element: <ProtectedRoute />, 
    children: [
      { path: ROUTES.ONBOARDING, element: <Onboarding /> },
    ]
  },

  // Destino de las rutas no reconocidas.
  { path: '*', element: <NotFound /> },
]);

export default function AppRouter() {
  const idClienteGoogle = import.meta.env.VITE_GOOGLE_CLIENT_ID || 'your_google_client_id_here';

  return (
    <GoogleOAuthProvider clientId={idClienteGoogle}>
      {/* Los avisos de sesión permanecen visibles después de redirigir o limpiar la cuenta. */}
      <ToastProvider>
        <AuthProvider>
          <CartProvider>
            <RouterProvider router={enrutador} />
          </CartProvider>
        </AuthProvider>
      </ToastProvider>
    </GoogleOAuthProvider>
  );
}
