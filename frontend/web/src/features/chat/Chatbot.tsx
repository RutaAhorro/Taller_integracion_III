// Vista del asistente: presenta mensajes de la API y delega las solicitudes en useChatbot.
import { useState } from 'react';
import { 
  Trash2,
  User, Bot, X, ShieldAlert, Info, Send, ShoppingCart, Package, RotateCcw
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { Link, useNavigate } from 'react-router-dom';
import { useCart } from '@/features/catalogo/CartContext';
import { formatPrice, formatearUnidad } from '@/features/catalogo/formatters';
import { slugifyMarket } from '@/features/catalogo/products-api';
import { type UiProduct } from '@/features/catalogo/tipos';
import { useChatbot, type ProductoRecomendadoChat } from '@/features/chat/useChatbot';
import { TypingIndicator } from '@/features/chat/TypingIndicator';
import ReactMarkdown from 'react-markdown';

const MAXIMO_PRODUCTOS_VISIBLES = 6;

const SUGERENCIAS = [
  'Busco leche por menos de $2.000',
  '¿Qué arroz está disponible?',
  'Busco harina de almendras',
];

// Convierte una recomendación del asistente al producto del catálogo para el carrito.
function aProductoCarrito(producto: ProductoRecomendadoChat): UiProduct {
  return {
    id: producto.id,
    name: producto.nombre,
    brand: producto.marca,
    image: producto.urlImagen,
    category: producto.categoria,
    unit: producto.unidad,
    price: producto.precio,
    originalPrice: producto.precioNormal,
    supermarketId: slugifyMarket(producto.supermercado),
    supermarketName: producto.supermercado || undefined,
    inStock: producto.enStock,
  };
}

export default function Chatbot() {
  const navegar = useNavigate();
  const { addToCart } = useCart();
  const [mensajesExpandidos, setMensajesExpandidos] = useState<Record<string, boolean>>({});
  const [imagenesRotas, setImagenesRotas] = useState<Record<string, boolean>>({});

  const {
    messages,
    inputValue,
    setInputValue,
    isProcessing,
    showAuthModal,
    setShowAuthModal,
    scrollRef,
    handleSend,
    handleRetry,
    handleClear
  } = useChatbot();
  // Solo el último error ofrece reintento: reenviar uno antiguo desordenaría la conversación.
  const identificadorUltimoMensaje = messages.at(-1)?.id;

  return (
    <div className="flex-1 flex flex-col bg-background text-foreground overflow-hidden relative">
      <header className="flex-none flex items-center justify-between px-4 py-2 bg-card border-b border-border shadow-sm z-10">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="relative flex h-8 w-8 shrink-0 overflow-hidden rounded-full bg-primary items-center justify-center">
            <Bot className="h-5 w-5 text-primary-foreground"/>
          </div>
          <div className="flex flex-wrap items-center gap-2 min-w-0">
            <h1 className="font-semibold text-sm sm:text-base leading-tight">Chef & Shopper IA</h1>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 border-l border-border pl-2 ml-2">
            <button onClick={handleClear} className="p-1.5 rounded-md hover:bg-accent text-muted-foreground transition-colors cursor-pointer" title="Vaciar chat">
              <Trash2 className="h-4 w-4"/>
            </button>
          </div>
        </div>
      </header>

      <main ref={scrollRef} className="flex-1 overflow-y-auto scrollbar-hide px-3 py-4 sm:px-6 sm:py-5 bg-background scroll-smooth">
        <div className="max-w-5xl mx-auto space-y-4">
          {messages.map((mensaje) => (
            <div key={mensaje.id} className={cn("flex gap-2.5 sm:gap-3", mensaje.role === 'user' ? "justify-end" : "justify-start")}>
              
              {mensaje.role === 'assistant' && (
                <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-full bg-primary text-primary-foreground border-2 border-border shadow-[3px_3px_0px_var(--color-border)] mt-1">
                  <Bot className="h-5 w-5"/>
                </div>
              )}

              <div className={cn(
                "relative rounded-2xl px-4 py-3 text-sm",
                mensaje.role === 'user' 
                  ? "max-w-[85%] sm:max-w-[70%] bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] rounded-br-none"
                  : "max-w-[90%] sm:max-w-[85%] bg-card border border-border text-foreground rounded-bl-none shadow-sm"
              )}>
                {(mensaje.type === 'text' || mensaje.type === 'error') && (
                  <div role={mensaje.type === 'error' ? 'alert' : undefined} className="leading-relaxed text-sm [&>p]:mb-2 last:[&>p]:mb-0 [&>ul]:list-disc [&>ul]:pl-5 [&>ol]:list-decimal [&>ol]:pl-5 [&>h3]:font-bold [&>h3]:mt-3 [&_strong]:font-bold [&_strong]:text-foreground">
                    <ReactMarkdown>{mensaje.content || ''}</ReactMarkdown>
                  </div>
                )}

                {mensaje.type === 'error' && mensaje.textoReintento && mensaje.id === identificadorUltimoMensaje && (
                  <button
                    type="button"
                    disabled={isProcessing}
                    onClick={() => handleRetry(mensaje.id)}
                    className="mt-3 inline-flex items-center gap-1.5 rounded-lg border-2 border-border bg-background px-3 py-1.5 text-xs font-bold text-foreground shadow-[2px_2px_0px_var(--color-border)] transition-colors hover:bg-accent cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <RotateCcw className="h-3.5 w-3.5" aria-hidden="true" />
                    Reintentar
                  </button>
                )}

                {mensaje.type === 'text' && mensaje.products && mensaje.products.length > 0 && (() => {
                  const estaExpandido = Boolean(mensajesExpandidos[mensaje.id]);
                  const productosVisibles = estaExpandido
                    ? mensaje.products
                    : mensaje.products.slice(0, MAXIMO_PRODUCTOS_VISIBLES);
                  const cantidadOculta = mensaje.products.length - MAXIMO_PRODUCTOS_VISIBLES;
                  return (
                    <div className="mt-3 space-y-2.5">
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5">
                        {productosVisibles.map((producto) => (
                          <div key={producto.id} className="flex flex-col justify-between rounded-xl border-2 border-border bg-background p-3 shadow-[3px_3px_0px_var(--color-border)]">
                            <div className="flex items-start gap-2">
                              {producto.urlImagen && !imagenesRotas[producto.id] ? (
                                <img
                                  src={producto.urlImagen}
                                  alt={producto.nombre}
                                  className="h-12 w-12 shrink-0 object-contain"
                                  onError={() =>
                                    setImagenesRotas((anteriores) => ({
                                      ...anteriores,
                                      [producto.id]: true,
                                    }))
                                  }
                                />
                              ) : (
                                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-muted">
                                  <Package className="h-6 w-6 text-muted-foreground" />
                                </div>
                              )}
                              <div className="min-w-0 flex-1">
                                <p className="line-clamp-2 text-xs font-semibold leading-snug text-foreground">{producto.nombre}</p>
                                <p className="truncate text-xs text-muted-foreground">{[producto.marca, formatearUnidad(producto.unidad)].filter((detalle) => detalle).join(' · ')}</p>
                                {producto.supermercado && (
                                  <span className="mt-1 inline-flex items-center rounded-full border border-border bg-card px-2 py-0.5 text-[10px] font-bold text-foreground shadow-[2px_2px_0px_var(--color-border)]">
                                    {producto.supermercado}
                                  </span>
                                )}
                              </div>
                            </div>
                            <div className="mt-2 flex items-baseline gap-2">
                              <span className="text-base font-extrabold text-foreground">{formatPrice(producto.precio)}</span>
                              {producto.enOferta && producto.precioNormal > producto.precio && (
                                <span className="text-xs font-medium text-muted-foreground line-through">{formatPrice(producto.precioNormal)}</span>
                              )}
                            </div>
                            <div className="mt-3 flex items-center justify-between gap-2">
                              <Link
                                to={`/producto/${producto.id}`}
                                className="rounded-lg border-2 border-border bg-background px-3 py-1.5 text-xs font-bold text-foreground shadow-[2px_2px_0px_var(--color-border)] transition-all duration-200 hover:bg-primary hover:text-primary-foreground"
                              >
                                Ver
                              </Link>
                              <button
                                type="button"
                                onClick={() => addToCart(aProductoCarrito(producto))}
                                className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-xs font-bold text-primary-foreground border-2 border-border shadow-[2px_2px_0px_var(--color-border)] transition-colors hover:bg-primary cursor-pointer"
                              >
                                <ShoppingCart className="h-3.5 w-3.5" />
                                Agregar
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                      {cantidadOculta > 0 && (
                        <button
                          type="button"
                          onClick={() =>
                            setMensajesExpandidos((anteriores) => ({
                              ...anteriores,
                              [mensaje.id]: !estaExpandido,
                            }))
                          }
                          className="w-full rounded-xl border-2 border-border bg-background px-3 py-2 text-xs font-bold text-foreground shadow-[2px_2px_0px_var(--color-border)] transition-colors hover:bg-accent cursor-pointer"
                        >
                          {estaExpandido
                            ? 'Mostrar menos'
                            : `Ver ${cantidadOculta} ${cantidadOculta === 1 ? 'producto más' : 'productos más'}`}
                        </button>
                      )}
                    </div>
                  );
                })()}
                
                {mensaje.type === 'welcome' && (
                  <div className="space-y-4">
                    <p className="leading-relaxed">
                      ¡Hola! Puedes consultar productos del catálogo y pedir recomendaciones de compra.
                      Cuando el asistente recomienda productos, verás tarjetas con acceso a la ficha y al carrito.
                      El chat todavía no guarda ingredientes ni calcula rutas.
                    </p>
                    <div className="flex flex-col gap-2 mt-4">
                      {SUGERENCIAS.map((sugerencia) => (
                        <button
                          key={sugerencia}
                          disabled={isProcessing}
                          onClick={() => handleSend(sugerencia)}
                          className="text-left px-3 py-2 text-sm bg-background hover:bg-primary border border-border hover:border-border rounded-xl transition-all duration-200 text-foreground hover:text-primary-foreground font-medium disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
                        >
                          "{sugerencia}"
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {mensaje.type === 'processing' && (
                  <div role="status" className="space-y-3 py-1">
                    <p>Consultando al asistente…</p>
                    <TypingIndicator className="mt-2" />
                  </div>
                )}

              </div>

              {mensaje.role === 'user' && (
                <div className="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-full bg-zinc-200 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-300 mt-1">
                  <User className="h-5 w-5"/>
                </div>
              )}
            </div>
          ))}
        </div>
      </main>

      <div className="flex-none bg-card border-t border-border px-3 py-2.5 sm:px-4">
        <div className="max-w-3xl mx-auto">
          <form 
            onSubmit={(evento) => { evento.preventDefault(); handleSend(inputValue); }}
            className="relative flex items-end gap-2 bg-background border border-border rounded-2xl p-1.5 shadow-sm focus-within:ring-2 focus-within:ring-ring focus-within:border-border transition-all"
          >
            <textarea 
              value={inputValue}
              disabled={isProcessing}
              onChange={(evento) => setInputValue(evento.target.value)}
              onKeyDown={(evento) => {
                if (evento.key === 'Enter' && !evento.shiftKey) {
                  evento.preventDefault();
                  handleSend(inputValue);
                }
              }}
              aria-label="Mensaje para el asistente"
              placeholder="Pregunta por un producto…"
              className="w-full max-h-28 min-h-[40px] bg-transparent resize-none outline-none py-2 px-3 text-sm text-foreground placeholder:text-muted-foreground"
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
              Las recomendaciones dependen de los datos disponibles en Catálogo; confirma precio y stock antes de comprar.
          </p>
        </div>
      </div>

      {showAuthModal && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="relative bg-card border border-border rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-in zoom-in-95 duration-200">
            <div className="p-6 text-center space-y-4">
              <div className="mx-auto w-12 h-12 bg-primary rounded-full flex items-center justify-center mb-2">
                <ShieldAlert className="h-6 w-6 text-primary-foreground"/>
              </div>
              <h2 className="text-xl font-bold text-foreground">
                Inicia sesión para consultar al Asistente IA
              </h2>
              <p className="text-sm text-muted-foreground leading-relaxed">
                Para consultar al asistente necesitas una sesión válida. Si expiró, vuelve a iniciar sesión.
              </p>
              
              <div className="pt-4 space-y-3">
                <button 
                  onClick={() => navegar('/login')}
                  className="w-full bg-primary hover:bg-primary text-primary-foreground border-2 border-border shadow-[4px_4px_0px_var(--color-border)] font-medium py-2.5 rounded-xl transition-colors cursor-pointer"
                >
                  Iniciar Sesión
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
