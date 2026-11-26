// Gestiona avisos temporales de la interfaz y su cierre automático.
import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import { CheckCircle2, AlertCircle, Info, XCircle, X } from 'lucide-react';
import { cn } from '@/lib/utils';

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  message: string;
  type: ToastType;
}

interface TipoContextoAviso {
  toast: (mensaje: string, tipo?: ToastType) => void;
}

const ContextoAviso = createContext<TipoContextoAviso | undefined>(undefined);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [avisos, setAvisos] = useState<Toast[]>([]);

  const toast = useCallback((mensaje: string, tipo: ToastType = 'info') => {
    const id = Math.random().toString(36).substring(2, 9);
    setAvisos((anteriores) => [...anteriores, { id, message: mensaje, type: tipo }]);
    
    setTimeout(() => {
      eliminarAviso(id);
    }, 4000);
  }, []);

  const eliminarAviso = useCallback((id: string) => {
    setAvisos((anteriores) => anteriores.filter((aviso) => aviso.id !== id));
  }, []);

  return (
    <ContextoAviso.Provider value={{ toast }}>
      {children}
      <div className="fixed top-20 right-0 left-0 sm:left-auto sm:right-4 z-[10000] flex flex-col gap-2 w-full sm:w-80 pointer-events-none px-4 sm:px-0">
        {avisos.map((aviso) => (
          <ElementoAviso key={aviso.id} aviso={aviso} alCerrar={() => eliminarAviso(aviso.id)} />
        ))}
      </div>
    </ContextoAviso.Provider>
  );
}

export const useToast = () => {
  const contexto = useContext(ContextoAviso);
  if (!contexto) throw new Error('useToast must be used within ToastProvider');
  return contexto;
};

// Renderiza un aviso individual y permite descartarlo manualmente.
function ElementoAviso({ aviso, alCerrar }: { aviso: Toast; alCerrar: () => void }) {
  const iconos = {
    success: <CheckCircle2 className="h-5 w-5 text-primary" />,
    error: <XCircle className="h-5 w-5 text-destructive" />,
    info: <Info className="h-5 w-5 text-primary" />,
    warning: <AlertCircle className="h-5 w-5 text-amber-600" />,
  };

  const estilosBorde = {
    success: "border-l-4 border-l-primary",
    error: "border-l-4 border-l-destructive",
    info: "border-l-4 border-l-primary",
    warning: "border-l-4 border-l-amber-600",
  };

  return (
    <div className={cn(
      "pointer-events-auto flex items-start gap-3 p-3 rounded-xl border-2 border-border bg-card shadow-[4px_4px_0px_var(--color-border)] animate-in slide-in-from-top-5 fade-in duration-300",
      estilosBorde[aviso.type] || estilosBorde.info
    )}>
      <div className="shrink-0 mt-0.5">{iconos[aviso.type]}</div>
      <div className="flex-1">
        <p className="text-xs sm:text-sm font-semibold text-foreground leading-snug">{aviso.message}</p>
      </div>
      <button 
        type="button"
        onClick={alCerrar}
        aria-label="Cerrar aviso"
        className="shrink-0 text-muted-foreground hover:text-foreground transition-colors p-0.5 rounded-md hover:bg-black/5 dark:hover:bg-white/10 cursor-pointer"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
