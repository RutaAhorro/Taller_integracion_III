// SUP-247: el navegador usa el Gateway y la cookie HttpOnly de Identidad.
const API_URL = (import.meta.env.VITE_API_URL || '/api/v1').replace(/\/$/, '');
export const CHAT_MAX_LENGTH = 1000;

export type ChatProduct = {
  id: string | null;
  nombre: string;
  supermercado: string;
  precio: number;
  en_stock: boolean;
};

export type ChatResponse = {
  respuesta: string;
  productos: ChatProduct[];
};

export class ChatApiError extends Error {
  readonly status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'ChatApiError';
    this.status = status;
  }
}

function isProduct(value: unknown): value is ChatProduct {
  if (!value || typeof value !== 'object') return false;
  const product = value as Record<string, unknown>;
  return (typeof product.id === 'string' || product.id === null)
    && typeof product.nombre === 'string'
    && typeof product.supermercado === 'string'
    && typeof product.precio === 'number' && Number.isFinite(product.precio) && product.precio >= 0
    && typeof product.en_stock === 'boolean';
}

export async function sendChatMessage(text: string, signal?: AbortSignal): Promise<ChatResponse> {
  const mensaje = text.trim();
  if (!mensaje || mensaje.length > CHAT_MAX_LENGTH) {
    throw new ChatApiError(`Escribe un mensaje de entre 1 y ${CHAT_MAX_LENGTH} caracteres.`);
  }

  const timeout = AbortSignal.timeout(60_000);
  const requestSignal = signal ? AbortSignal.any([signal, timeout]) : timeout;
  try {
    const response = await fetch(`${API_URL}/chat`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mensaje }),
      signal: requestSignal,
    });

    if (!response.ok) {
      const messages: Record<number, string> = {
        401: 'Tu sesión expiró o no es válida. Inicia sesión para continuar.',
        403: 'No tienes permiso para consultar al asistente.',
        404: 'El chat todavía no está disponible en el Gateway. Intenta más tarde.',
        422: 'El mensaje no es válido. Revisa su contenido y longitud.',
        429: 'Hay demasiadas consultas. Espera un momento antes de volver a intentar.',
        502: 'No se pudo conectar con el asistente. Intenta más tarde.',
        503: 'El asistente no está disponible en este momento. Intenta más tarde.',
        504: 'El asistente tardó demasiado en responder. Intenta nuevamente.',
      };
      throw new ChatApiError(messages[response.status] || 'No se pudo procesar tu consulta. Intenta más tarde.', response.status);
    }

    const data: unknown = await response.json().catch(() => null);
    if (!data || typeof data !== 'object') throw new ChatApiError('El asistente devolvió una respuesta inválida. Intenta nuevamente.');
    const payload = data as Record<string, unknown>;
    // SUP-247 devuelve ambos campos como alias. Nunca extraer productos del texto del LLM.
    const productos = payload.productos ?? payload.productos_recomendados;
    if (typeof payload.respuesta !== 'string' || !payload.respuesta.trim()
      || !Array.isArray(productos) || !productos.every(isProduct)) {
      throw new ChatApiError('El asistente devolvió una respuesta inválida. Intenta nuevamente.');
    }
    return { respuesta: payload.respuesta, productos };
  } catch (error) {
    if (signal?.aborted) throw error;
    if (timeout.aborted || (error instanceof Error && error.name === 'TimeoutError')) {
      throw new ChatApiError('El asistente tardó demasiado en responder. Intenta nuevamente.');
    }
    if (error instanceof ChatApiError) throw error;
    throw new ChatApiError('No se pudo conectar con el asistente. Revisa tu conexión e intenta nuevamente.');
  }
}
