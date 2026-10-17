import { Navigate, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { hasAllowedRole } from '@/lib/roles';
import { ShieldAlert, ArrowLeft } from 'lucide-react';

// Guardia de autenticación + rol (SUP-256, RN-11).
// - Invitado (sin sesión): muestra la pantalla de acceso (ir a /login).
// - Logueado sin el rol requerido: redirige a home (/), no al panel.
// - Sin `roles`: solo exige sesión (conducta anterior).
export function ProtectedRoute({ roles }: { roles?: string[] }) {
  const { isGuest, user } = useAuth();
  const navigate = useNavigate();

  // If the user is a guest (not authenticated), show the auth gating UI
  if (isGuest) {
    return (
      <div className="flex h-full min-h-[calc(100vh-4rem)] items-center justify-center p-4 bg-background">
        <div className="bg-card border border-border rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-in zoom-in-95 duration-200">
          <div className="p-6 text-center space-y-4">
            <div className="mx-auto w-12 h-12 bg-primary rounded-full flex items-center justify-center mb-2">
              <ShieldAlert className="h-6 w-6 text-primary-foreground" />
            </div>
            <h2 className="text-xl font-bold text-foreground">
              Inicia sesión para acceder
            </h2>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Esta sección requiere una cuenta activa. Inicia sesión o crea una cuenta gratuita para continuar.
            </p>

            <div className="pt-4 space-y-3">
              <button
                onClick={() => navigate('/login')}
                className="w-full bg-primary hover:brightness-110 text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] font-medium py-2.5 rounded-xl transition-all cursor-pointer hover:-translate-y-0.5 active:translate-y-0 active:shadow-none"
              >
                Iniciar Sesión
              </button>
              <button
                onClick={() => navigate('/login', { state: { tab: 'signup' } })}
                className="w-full bg-background border border-border hover:bg-accent text-foreground font-medium py-2.5 rounded-xl transition-colors cursor-pointer"
              >
                Crear Cuenta Gratuita
              </button>
            </div>
            <div className="pt-2">
              <button
                onClick={() => navigate(-1)}
                className="flex items-center justify-center gap-2 w-full text-sm text-muted-foreground hover:text-foreground transition-colors py-2"
              >
                <ArrowLeft className="h-4 w-4" />
                Volver
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (roles && roles.length > 0 && !hasAllowedRole(user?.rol, roles)) {
    return <Navigate to="/" replace />;
  }

  // Otherwise, render the child routes
  return <Outlet />;
}
