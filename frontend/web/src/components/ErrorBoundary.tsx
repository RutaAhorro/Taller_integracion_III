// Registra errores de navegación y ofrece una salida segura hacia el inicio.
import { useRouteError } from 'react-router-dom';
import { AlertTriangle, RefreshCw, Home } from 'lucide-react';
import { logger } from '@/lib/logger';

export function ErrorBoundary() {
  const error = useRouteError() as any;
  
  // Registramos el error en el logger profesional
  logger.error('Error atrapado por el router:', error);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="bg-card border border-destructive/20 rounded-2xl shadow-xl w-full max-w-lg overflow-hidden animate-in zoom-in-95 duration-300">
        <div className="p-8 text-center space-y-6">
          <div className="mx-auto w-16 h-16 bg-destructive/10 rounded-full flex items-center justify-center mb-2">
            <AlertTriangle className="h-8 w-8 text-destructive" />
          </div>
          
          <div className="space-y-2">
            <h2 className="text-2xl font-bold text-foreground tracking-tight">
              Algo salió mal
            </h2>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Ha ocurrido un error inesperado en la aplicación. Nuestro equipo ya ha sido notificado.
            </p>
          </div>

          {import.meta.env.DEV && error && (
            <div className="text-left bg-muted p-4 rounded-lg overflow-auto max-h-32 text-xs font-mono text-muted-foreground border border-border">
              {error.statusText || error.message || error.toString()}
            </div>
          )}

          <div className="pt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              onClick={() => window.location.reload()}
              className="flex items-center justify-center gap-2 w-full bg-primary hover:brightness-110 text-primary-foreground font-medium py-2.5 rounded-xl transition-all cursor-pointer shadow-sm hover:shadow active:scale-[0.98]"
            >
              <RefreshCw className="h-4 w-4" />
              Recargar página
            </button>
            <button
              onClick={() => { window.location.href = '/'; }}
              className="flex items-center justify-center gap-2 w-full bg-background border border-border hover:bg-accent text-foreground font-medium py-2.5 rounded-xl transition-all cursor-pointer active:scale-[0.98]"
            >
              <Home className="h-4 w-4" />
              Volver al inicio
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
