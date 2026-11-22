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
      <div className="fixed top-4 right-0 left-0 sm:left-auto sm:right-4 z-[10000] flex flex-col gap-2 w-full sm:w-80 pointer-events-none px-4 sm:px-0">
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
    success: <CheckCircle2 className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />,
    error: <XCircle className="h-5 w-5 text-destructive dark:text-red-400" />,
    info: <Info className="h-5 w-5 text-blue-600 dark:text-blue-400" />,
    warning: <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400" />,
  };

  const estilosFondo = {
    success: "bg-emerald-50 border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-900",
    error: "bg-red-50 border-red-200 dark:bg-red-950/30 dark:border-red-900",
    info: "bg-blue-50 border-blue-200 dark:bg-blue-950/30 dark:border-blue-900",
    warning: "bg-amber-50 border-amber-200 dark:bg-amber-950/30 dark:border-amber-900",
  };

  return (
    <div className={cn(
      "pointer-events-auto flex items-start gap-3 p-4 rounded-xl border shadow-lg backdrop-blur-md animate-in slide-in-from-top-5 fade-in duration-300",
      estilosFondo[aviso.type] || estilosFondo.info
    )}>
      <div className="shrink-0 mt-0.5">{iconos[aviso.type]}</div>
      <div className="flex-1">
        <p className="text-sm font-bold text-foreground leading-snug">{aviso.message}</p>
      </div>
      <button 
        onClick={alCerrar}
        className="shrink-0 text-muted-foreground hover:text-foreground transition-colors p-0.5 rounded-md hover:bg-black/5 dark:hover:bg-white/10"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
