import { useState, useRef, useEffect } from 'react';
import { useAuth } from '@/hooks/useAuth';

// TODO SUP-253: el backend ya no expone /chat/gemini; migrar a /api/v1/chat.
const IA_API_URL = (import.meta.env.VITE_IA_URL || 'http://localhost:8002').replace(/\/$/, '');

export type MessageType = 'welcome' | 'text' | 'processing' | 'rich-recipe' | 'out-of-stock';

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  type: MessageType;
  content?: string;
  step?: number;
  query?: string;
};

const ERROR_IA = 'Lo siento, en este momento no puedo procesar tu solicitud. Por favor intenta más tarde.';
const ERROR_CONEXION = 'Lo siento, hay problemas de conexión con mis servicios en este momento.';

let nextId = 0;
const newId = () => `${Date.now()}-${nextId++}`;

const welcomeMessage = (): Message => ({ id: newId(), role: 'assistant', type: 'welcome' });

export function useChatbot() {
  const { isGuest, user } = useAuth();
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const [messages, setMessages] = useState<Message[]>(() => [welcomeMessage()]);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Mantiene visible el último mensaje.
  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages]);

  const handleSend = async (text: string) => {
    if (isGuest) {
      setShowAuthModal(true);
      return;
    }
    if (!text.trim() || isProcessing) return;

    const processingId = newId();
    const setStep = (step: number) =>
      setMessages((prev) => prev.map((m) => (m.id === processingId ? { ...m, step } : m)));
    // Reemplaza el indicador de "procesando" por la respuesta final.
    const reply = (content: string) =>
      setMessages((prev) => [
        ...prev.filter((m) => m.id !== processingId),
        { id: newId(), role: 'assistant', type: 'text', content },
      ]);

    setMessages((prev) => [
      ...prev,
      { id: newId(), role: 'user', type: 'text', content: text },
      { id: processingId, role: 'assistant', type: 'processing', step: 2 },
    ]);
    setInputValue('');
    setIsProcessing(true);

    try {
      const response = await fetch(`${IA_API_URL}/chat/gemini`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mensaje: text }),
      });
      setStep(3);
      const data = await response.json();

      if (data.error) {
        console.error('Error devuelto por la IA:', data.error);
        reply(ERROR_IA);
      } else {
        reply(data.explicacion || 'Procesado.');
      }
    } catch (error) {
      console.error('Excepción al contactar la API:', error);
      reply(ERROR_CONEXION);
    } finally {
      setIsProcessing(false);
    }
  };

  const handleClear = () => setMessages([welcomeMessage()]);

  return {
    user,
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
