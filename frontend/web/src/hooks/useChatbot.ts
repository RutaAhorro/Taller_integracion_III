// Envía consultas al asistente por el Gateway; no simula recetas ni guardados en listas.
import { useState, useRef, useEffect } from 'react';
import { useAuth } from '@/hooks/useAuth';

const API_URL = (import.meta.env.VITE_API_URL || '/api/v1').replace(/\/$/, '');

// Distingue respuestas, errores y espera para no presentar fallos como resultados del asistente.
export type MessageType = 'welcome' | 'text' | 'processing' | 'error';

// Identifica cada mensaje para sustituir solo la espera correspondiente a su solicitud.
export type Message = {
  id: string;
  role: 'user' | 'assistant';
  type: MessageType;
  content?: string;
};

const ERROR_IA = 'Lo siento, en este momento no puedo procesar tu solicitud. Por favor intenta más tarde.';
const ERROR_CONEXION = 'No se pudo contactar al asistente. Revisa la conexión e intenta nuevamente.';
const ERROR_SESION = 'Tu sesión expiró o no es válida. Vuelve a iniciar sesión para consultar al asistente.';

let siguienteIdentificador = 0;
// Combina fecha y secuencia para evitar IDs repetidos entre mensajes creados en el mismo milisegundo.
const nuevoIdentificador = () => `${Date.now()}-${siguienteIdentificador++}`;
// Crea una bienvenida nueva al iniciar o vaciar el chat, sin simular una respuesta del servicio.
const mensajeBienvenida = (): Message => ({ id: nuevoIdentificador(), role: 'assistant', type: 'welcome' });

// Diferencia errores HTTP para orientar el reintento; un 401 indica que la sesión ya no es válida.
function mensajeErrorHTTP(estado: number): string {
  if (estado === 401) return ERROR_SESION;
  if (estado === 403) return 'No tienes permiso para consultar al asistente.';
  if (estado === 404) return 'El asistente no está habilitado en este entorno. Intenta más tarde.';
  if (estado === 422) return 'El mensaje no es válido. Revisa su contenido o reduce su longitud.';
  if (estado === 429) return 'Hay demasiadas solicitudes. Espera un momento antes de volver a intentar.';
  return ERROR_IA;
}

// Centraliza conversación, sesión y solicitudes para mantener la vista separada del contrato HTTP.
export function useChatbot() {
  const { isGuest } = useAuth();
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const [messages, setMessages] = useState<Message[]>(() => [mensajeBienvenida()]);
  const scrollRef = useRef<HTMLDivElement>(null);
  const solicitudActiva = useRef<AbortController | null>(null);

  // Mantiene visible el último mensaje al cambiar la conversación.
  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages]);

  // Salir del chat cancela la espera, sin aplicar respuestas sobre una vista desmontada.
  useEffect(() => () => solicitudActiva.current?.abort(), []);

  // Envía una sola consulta con sesión y conserva el texto si falla para permitir su reintento.
  const handleSend = async (texto: string) => {
    const mensaje = texto.trim();
    if (!mensaje || solicitudActiva.current) return;
    if (isGuest) {
      setShowAuthModal(true);
      return;
    }

    // El ref bloquea también dos envíos que ocurran antes del siguiente render.
    const controlador = new AbortController();
    solicitudActiva.current = controlador;
    const identificadorEspera = nuevoIdentificador();
    // Sustituye la espera por el resultado o error, ignorando respuestas de solicitudes canceladas.
    const responder = (contenido: string, tipo: 'text' | 'error') => {
      if (controlador.signal.aborted) return;
      setMessages((anteriores) => [
        ...anteriores.filter((anterior) => anterior.id !== identificadorEspera),
        { id: nuevoIdentificador(), role: 'assistant', type: tipo, content: contenido },
      ]);
      if (tipo === 'error') setInputValue(mensaje);
    };

    setMessages((anteriores) => [
      ...anteriores,
      { id: nuevoIdentificador(), role: 'user', type: 'text', content: mensaje },
      { id: identificadorEspera, role: 'assistant', type: 'processing' },
    ]);
    setInputValue('');
    setIsProcessing(true);

    try {
      const respuesta = await fetch(`${API_URL}/chat`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mensaje }),
        signal: controlador.signal,
      });
      if (controlador.signal.aborted) return;
      if (!respuesta.ok) {
        if (respuesta.status === 401) setShowAuthModal(true);
        responder(mensajeErrorHTTP(respuesta.status), 'error');
        return;
      }

      const datos: unknown = await respuesta.json().catch(() => null);
      if (
        !datos || typeof datos !== 'object' ||
        !('respuesta' in datos) || typeof datos.respuesta !== 'string' ||
        !datos.respuesta.trim() || ('error' in datos && datos.error)
      ) {
        responder(ERROR_IA, 'error');
        return;
      }
      responder(datos.respuesta, 'text');
    } catch {
      if (!controlador.signal.aborted) responder(ERROR_CONEXION, 'error');
    } finally {
      if (solicitudActiva.current === controlador) {
        solicitudActiva.current = null;
        setIsProcessing(false);
      }
    }
  };

  // Cancela la espera antes de reiniciar el chat para que una respuesta tardía no lo repueble.
  const handleClear = () => {
    solicitudActiva.current?.abort();
    solicitudActiva.current = null;
    setIsProcessing(false);
    setMessages([mensajeBienvenida()]);
  };

  return {
    messages,
    inputValue,
    setInputValue,
    isProcessing,
    showAuthModal,
    setShowAuthModal,
    scrollRef,
    handleSend,
    handleClear,
  };
}
