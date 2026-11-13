// Indicador visual de espera mientras el asistente procesa una solicitud.
import { cn } from '@/lib/utils';

export function TypingIndicator({ className }: { className?: string }) {
  return (
    <div className={cn("flex space-x-1.5 p-2 px-1 items-center h-6", className)}>
      <div className="h-2 w-2 rounded-full bg-foreground/40 animate-bounce [animation-delay:-0.3s]"></div>
      <div className="h-2 w-2 rounded-full bg-foreground/40 animate-bounce [animation-delay:-0.15s]"></div>
      <div className="h-2 w-2 rounded-full bg-foreground/40 animate-bounce"></div>
    </div>
  );
}
