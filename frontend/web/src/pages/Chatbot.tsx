import { 
  Sparkles,
  Trash2, Maximize2, 
  User, Bot, X, ShieldAlert, Info, Send
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useNavigate } from 'react-router-dom';
import { ChatProducts } from '@/components/chatbot/ChatProducts';
import { TypingIndicator } from '@/components/chatbot/TypingIndicator';
import ReactMarkdown from 'react-markdown';
import { useChatbot } from '@/hooks/useChatbot';
import { CHAT_MAX_LENGTH } from '@/features/chat/chat-api';

export default function Chatbot() {
  const navigate = useNavigate();
  
  const {
    user,
    messages,
    inputValue,
    setInputValue,
    isProcessing,
    showAuthModal,
    setShowAuthModal,
    scrollRef,
    handleSend,
    handleClear
  } = useChatbot();

  return (
    <div className="flex-1 flex flex-col bg-background text-foreground overflow-hidden relative">
      {/* Header */}
      <header className="flex-none flex items-center justify-between px-4 py-3 bg-card border-b border-border shadow-sm z-10">
        <div className="flex items-center gap-3">
          <div className="relative flex h-10 w-10 shrink-0 overflow-hidden rounded-full bg-primary items-center justify-center">
            <Bot className="h-6 w-6 text-primary-foreground"/>
          </div>
          <div>
            <h1 className="font-semibold text-base leading-tight">Chef & Shopper IA</h1>
            <div className="flex items-center gap-2 mt-0.5">
              <span className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] sm:text-xs font-bold transition-colors border-2 border-border bg-primary text-primary-foreground shadow-[2px_2px_0px_var(--color-border)] sm:shadow-[4px_4px_0px_var(--color-border)] whitespace-nowrap">
                <span className="w-1.5 h-1.5 rounded-full bg-primary-foreground/50 mr-1.5"></span>
                 Asistente de compras
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <div className="hidden sm:flex group relative items-center justify-center rounded-md border border-border bg-card px-3 py-1.5 text-xs font-medium cursor-pointer hover:bg-accent transition-colors">
            <Sparkles className="h-3.5 w-3.5 mr-1.5 text-primary"/>
            Consultas IA: {user?.cuota_tokens_ia ?? 0}/5 disponibles
            {/* Tooltip */}
            <div className="absolute top-full mt-2 w-48 rounded-md border border-border bg-card p-2 text-center text-xs text-muted-foreground opacity-0 shadow-md transition-opacity group-hover:opacity-100 pointer-events-none z-50">
              Conviértete en Colaborador para cuota extendida
            </div>
          </div>

          <div className="flex items-center gap-1 border-l border-border pl-2 ml-2">
            <button onClick={handleClear} className="p-2 rounded-md hover:bg-accent text-muted-foreground transition-colors" title="Vaciar chat">
              <Trash2 className="h-4 w-4"/>
            </button>
            <button className="p-2 rounded-md hover:bg-accent text-muted-foreground transition-colors hidden sm:block" title="Pantalla completa">
              <Maximize2 className="h-4 w-4"/>
            </button>
          </div>
        </div>
      </header>

      <p className="flex-none border-b border-border bg-secondary/40 px-4 py-2 text-xs text-muted-foreground">
        Integración parcial: el chat requiere que la ruta del Gateway esté habilitada (SUP-278). Guardar ingredientes en listas aún está pendiente.
      </p>

      {/* Main Chat Area */}
      <main ref={scrollRef} className="flex-1 overflow-y-auto scrollbar-hide p-4 sm:p-6 space-y-6 bg-background scroll-smooth">
        <div className="max-w-3xl mx-auto space-y-6">
          {messages.map((msg) => (
            <div key={msg.id} className={cn("flex gap-3 sm:gap-4", msg.role === 'user' ? "justify-end" : "justify-start")}>
              
              {/* Assistant Avatar */}
              {msg.role === 'assistant' && (
                <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-full bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] mt-1">
                  <Bot className="h-5 w-5"/>
                </div>
              )}

              {/* Message Content */}
              <div className={cn(
                "relative max-w-[85%] sm:max-w-[75%] rounded-2xl px-4 py-3 text-sm",
                msg.role === 'user' 
                  ? "bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] rounded-br-none"
                  : "bg-card border border-border text-foreground rounded-bl-none shadow-sm"
              )}>
                {msg.type === 'text' && (
                  <div className="break-words leading-relaxed text-[clamp(0.875rem,2vw,1.125rem)] [&>p]:mb-2 last:[&>p]:mb-0 [&>ul]:list-disc [&>ul]:pl-5 [&>ol]:list-decimal [&>ol]:pl-5 [&>h3]:font-bold [&>h3]:mt-3 [&_strong]:font-bold [&_strong]:text-foreground">
                    <ReactMarkdown>{msg.content || ''}</ReactMarkdown>
                    {msg.products && <ChatProducts products={msg.products} />}
                  </div>
                )}

                {msg.type === 'error' && <p role="alert" className="text-destructive">{msg.content}</p>}
                
                {msg.type === 'welcome' && (
                  <div className="space-y-4">
                    <p className="leading-relaxed">
                      ¡Hola! Soy tu asistente inteligente de compras. Puedo ayudarte a planificar tus comidas optimizando tu presupuesto con los precios reales de Temuco. ¿Qué tienes en mente hoy?
                    </p>
                    <div className="flex flex-col gap-2 mt-4">
                      {[
                        "Almuerzo saludable para 4 por menos de $15.000",
                        "Asado familiar económico para este fin de semana",
                        "Busco Harina de Almendras 1kg"
                      ].map((chip, idx) => (
                        <button
                          key={idx}
                           onClick={() => handleSend(chip)}
                           disabled={isProcessing}
                          className="text-left px-3 py-2 text-[clamp(0.875rem,2vw,1rem)] bg-background hover:bg-primary border border-border hover:border-border rounded-xl transition-all duration-200 text-foreground hover:text-primary-foreground font-medium"
                        >
                          "{chip}"
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {msg.type === 'processing' && (
                  <div role="status" className="space-y-3 py-1">
                    <p>Consultando al asistente…</p>
                    <TypingIndicator className="mt-2" />
                  </div>
                )}
              </div>

              {/* User Avatar */}
              {msg.role === 'user' && (
                <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-full bg-zinc-200 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-300 mt-1">
                  <User className="h-5 w-5"/>
                </div>
              )}
            </div>
          ))}
        </div>
      </main>

      {/* Input Area */}
      <div className="flex-none bg-card border-t border-border p-4">
        <div className="max-w-3xl mx-auto">
          <form 
            onSubmit={(e) => { e.preventDefault(); handleSend(inputValue); }}
            className="relative flex items-end gap-2 bg-background border border-border rounded-2xl p-2 shadow-sm focus-within:ring-2 focus-within:ring-ring focus-within:border-border transition-all"
          >
            <textarea 
              aria-label="Mensaje para el asistente"
              maxLength={CHAT_MAX_LENGTH}
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  handleSend(inputValue);
                }
              }}
              placeholder="Pide una receta, ingredientes o presupuesto..."
              className="w-full max-h-32 min-h-[44px] bg-transparent resize-none outline-none py-2.5 px-3 text-[clamp(0.875rem,2vw,1.125rem)] text-foreground placeholder:text-muted-foreground"
              rows={1}
            />
            <button 
              type="submit"
              aria-label="Enviar mensaje"
              disabled={!inputValue.trim() || isProcessing}
              className="p-2.5 mb-0.5 rounded-xl bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] hover:bg-primary disabled:bg-muted disabled:text-muted-foreground transition-colors shrink-0 flex items-center justify-center"
            >
              <Send className="h-4 w-4 ml-0.5"/>
            </button>
          </form>
          <p className="text-center text-[10px] text-muted-foreground mt-2 flex items-center justify-center gap-1">
            <Info className="h-3 w-3"/>
            Precios y stock según el catálogo consultado por el asistente. Máximo {CHAT_MAX_LENGTH} caracteres por mensaje.
          </p>
        </div>
      </div>

      {/* Auth Gating Modal Overlay */}
      {showAuthModal && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="bg-card border border-border rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-in zoom-in-95 duration-200">
            <div className="p-6 text-center space-y-4">
              <div className="mx-auto w-12 h-12 bg-primary rounded-full flex items-center justify-center mb-2">
                <ShieldAlert className="h-6 w-6 text-primary-foreground"/>
              </div>
              <h2 className="text-xl font-bold text-foreground">
                Inicia sesión para consultar al Asistente IA
              </h2>
              <p className="text-sm text-muted-foreground leading-relaxed">
                El cálculo inteligente de recetas y optimización de rutas requiere una cuenta activa para personalizar tu experiencia.
              </p>
              
              <div className="pt-4 space-y-3">
                <button 
                  onClick={() => navigate('/login')}
                  className="w-full bg-primary hover:bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] font-medium py-2.5 rounded-xl transition-colors cursor-pointer"
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
            </div>
            <button 
              onClick={() => setShowAuthModal(false)}
              className="absolute top-4 right-4 p-1 rounded-full text-muted-foreground hover:text-foreground transition-colors"
            >
              <X className="h-5 w-5"/>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
