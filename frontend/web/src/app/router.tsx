import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { GoogleOAuthProvider } from '@react-oauth/google';
import { ROUTES, MOCK_ROUTES } from '@/core/routes';

// Layouts
import { MainLayout } from '@/layouts/MainLayout';
import { CartProvider } from '@/contexts/CartContext';
import { ToastProvider } from '@/contexts/ToastContext';
import { AuthProvider } from '@/contexts/AuthContext';
import { ProtectedRoute } from '@/components/auth/ProtectedRoute';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ROLE_ADMIN, ROLE_COLABORADOR } from '@/lib/roles';

// Sprint 1 — vistas funcionales
import Home from '@/pages/Home';
import Dashboard from '@/pages/Dashboard';
import ProductDetail from '@/pages/ProductDetail';
import Login from '@/pages/Login';
import Onboarding from '@/pages/Onboarding';
import Profile from '@/pages/Profile';
import NotFound from '@/pages/NotFound';
import Terms from '@/pages/Terms';

// Sprint 2+ — componentes ya construidos, envueltos como mock
import Chatbot from '@/pages/Chatbot';
import Crowdsourcing from '@/pages/Crowdsourcing';
import RouteViewer from '@/pages/RouteViewer';
import History from '@/pages/History';
import MockShell from '@/pages/mocks/MockShell';
import Planes from '@/pages/Planes';
import AdminDashboard from '@/features/admin/AdminDashboard';

const chatSprint = MOCK_ROUTES[ROUTES.CHAT];
const colabSprint = MOCK_ROUTES[ROUTES.COLABORADOR];
const rutaSprint = MOCK_ROUTES[ROUTES.RUTA];
const histSprint = MOCK_ROUTES[ROUTES.HISTORIAL];

const router = createBrowserRouter([
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
        element: (
          <MockShell sprint={chatSprint}>
            <Chatbot />
          </MockShell>
        ),
      },
    ],
  },

  // ───────────────────────────────────────────────────────────
  // Grupo B: MainLayout (BottomNav + página con header propio + SideBar global) - PROTEGIDAS
  // ───────────────────────────────────────────────────────────
  {
    element: <MainLayout />,
    errorElement: <ErrorBoundary />,
    children: [
      {
        element: <ProtectedRoute />,
        children: [
          { path: ROUTES.PERFIL, element: <Profile /> },
          { path: ROUTES.PLANES, element: <Planes /> },
          {
            // SUP-256: /colaborador solo colaborador o admin.
            element: <ProtectedRoute roles={[ROLE_COLABORADOR, ROLE_ADMIN]} />,
            children: [
              {
                path: ROUTES.COLABORADOR,
                element: (
                  <MockShell sprint={colabSprint}>
                    <Crowdsourcing />
                  </MockShell>
                ),
              },
            ],
          },
          {
            path: ROUTES.RUTA,
            element: (
              <MockShell sprint={rutaSprint}>
                <RouteViewer />
              </MockShell>
            ),
          },
          {
            path: ROUTES.HISTORIAL,
            element: (
              <MockShell sprint={histSprint}>
                <History />
              </MockShell>
            ),
          },
        ]
      }
    ],
  },

  // ───────────────────────────────────────────────────────────
  // Grupo C: sin layout (pantalla completa)
  // ───────────────────────────────────────────────────────────
  { path: ROUTES.HOME, element: <Home />, errorElement: <ErrorBoundary /> },
  { path: ROUTES.LOGIN, element: <Login /> },
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

  // Catch-all
  { path: '*', element: <NotFound /> },
]);

export default function AppRouter() {
  const googleClientId = import.meta.env.VITE_GOOGLE_CLIENT_ID || 'your_google_client_id_here';

  return (
    <GoogleOAuthProvider clientId={googleClientId}>
      <AuthProvider>
        <ToastProvider>
          <CartProvider>
            <RouterProvider router={router} />
          </CartProvider>
        </ToastProvider>
      </AuthProvider>
    </GoogleOAuthProvider>
  );
}