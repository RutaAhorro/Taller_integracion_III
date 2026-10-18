import { useEffect } from 'react';
import { Navigate, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/contexts/ToastContext';
import { ROUTES } from '@/core/routes';
import { hasAllowedRole, normalizeRole, ROLE_ADMIN, ROLE_COLABORADOR } from '@/lib/roles';
import { ShieldAlert, ArrowLeft } from 'lucide-react';

// Guardia de autenticación + rol (SUP-256, RN-11).
// - Invitado (sin sesión): muestra la pantalla de acceso (ir a /login). En rutas
//   privilegiadas (admin/colaborador) no se ofrece registro público.
// - Logueado sin el rol requerido: avisa y redirige al inicio autenticado
//   (/dashboard), nunca a la landing pública.
// - Sin `roles`: solo exige sesión (conducta anterior).
export function ProtectedRoute({ roles }: { roles?: string[] }) {
  const { isGuest, user } = useAuth();
  const navigate = useNavigate();
  const { toast } = useToast();

  const requiredRoles = roles ?? [];
  const hasRoleGuard = requiredRoles.length > 0;
  // Rutas que exigen admin o colaborador: sin registro público en el modal.
  const privilegedOnly = requiredRoles.some((role) => {
    const normalized = normalizeRole(role);
    return normalized === ROLE_ADMIN || normalized === ROLE_COLABORADOR;
  });
  const denied = !isGuest && hasRoleGuard && !hasAllowedRole(user?.rol, requiredRoles);

  useEffect(() => {
    if (denied) toast('No tienes permisos para acceder a esa sección.', 'warning');
  }, [denied, toast]);

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
              {privilegedOnly ? 'Acceso restringido' : 'Inicia sesión para acceder'}
            </h2>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {privilegedOnly
                ? 'Esta sección requiere una cuenta con rol de administrador o colaborador. Inicia sesión con una cuenta autorizada.'
                : 'Esta sección requiere una cuenta activa. Inicia sesión o crea una cuenta gratuita para continuar.'}
            </p>

            <div className="pt-4 space-y-3">
              <button
                onClick={() => navigate(ROUTES.LOGIN)}
                className="w-full bg-primary hover:brightness-110 text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] font-medium py-2.5 rounded-xl transition-all cursor-pointer hover:-translate-y-0.5 active:translate-y-0 active:shadow-none"
              >
                Iniciar Sesión
              </button>
              {!privilegedOnly && (
                <button
                  onClick={() => navigate(ROUTES.LOGIN, { state: { tab: 'signup' } })}
                  className="w-full bg-background border border-border hover:bg-accent text-foreground font-medium py-2.5 rounded-xl transition-colors cursor-pointer"
                >
                  Crear Cuenta Gratuita
                </button>
              )}
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

  if (denied) {
    return <Navigate to={ROUTES.CATALOGO} replace />;
  }

  // Otherwise, render the child routes
  return <Outlet />;
}
