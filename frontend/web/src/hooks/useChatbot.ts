import { useState, useRef, useEffect } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { ChatApiError, sendChatMessage, type ChatProduct } from '@/features/chat/chat-api';

export type MessageType = 'welcome' | 'text' | 'processing' | 'error';

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  type: MessageType;
  content?: string;
  products?: ChatProduct[];
};

export function useChatbot() {
  const { isGuest, user } = useAuth();
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const activeRequest = useRef<AbortController | null>(null);
  const sequence = useRef(0);
  
  const [messages, setMessages] = useState<Message[]>([
    { id: '1', role: 'assistant', type: 'welcome' }
  ]);

  // No conservar respuestas de otra sesión ni peticiones al abandonar la vista.
  useEffect(() => {
    setMessages([{ id: 'welcome', role: 'assistant', type: 'welcome' }]);
    setInputValue('');
    setIsProcessing(false);
    setShowAuthModal(false);
    return () => {
      activeRequest.current?.abort();
      activeRequest.current = null;
    };
  }, [user?.id]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSend = async (text: string) => {
    if (isGuest) {
      setShowAuthModal(true);
      return;
    }
    
    if (!text.trim() || activeRequest.current) return;

    const controller = new AbortController();
    activeRequest.current = controller;
    const id = String(++sequence.current);
    const newMsg: Message = { id: `user-${id}`, role: 'user', type: 'text', content: text.trim() };
    setMessages(prev => [...prev, newMsg]);
    setInputValue('');
    setIsProcessing(true);

    const processingId = `processing-${id}`;
    setMessages(prev => [...prev, { id: processingId, role: 'assistant', type: 'processing' }]);

    try {
      const data = await sendChatMessage(text, controller.signal);
      if (activeRequest.current !== controller) return;
      setMessages(prev => [...prev.filter(m => m.id !== processingId), {
        id: `assistant-${id}`, role: 'assistant', type: 'text', content: data.respuesta, products: data.productos,
      }]);
    } catch (error) {
      if (activeRequest.current !== controller) return;
      if (error instanceof ChatApiError && error.status === 401) setShowAuthModal(true);
      setMessages(prev => [...prev.filter(m => m.id !== processingId), {
        id: `error-${id}`, role: 'assistant', type: 'error',
        content: error instanceof Error ? error.message : 'No se pudo procesar tu consulta. Intenta nuevamente.',
      }]);
    } finally {
      if (activeRequest.current === controller) {
        activeRequest.current = null;
        setIsProcessing(false);
      }
    }
  };

  const handleClear = () => {
    activeRequest.current?.abort();
    activeRequest.current = null;
    setIsProcessing(false);
    setMessages([{ id: 'welcome', role: 'assistant', type: 'welcome' }]);
  };

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
    handleClear
  };
}
