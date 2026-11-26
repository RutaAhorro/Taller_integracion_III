// Envía consultas al asistente por el Gateway; no simula recetas ni guardados en listas.
import { useState, useRef, useEffect } from 'react';
import { useAuth } from '@/features/autenticacion/useAuth';

const URL_API = (import.meta.env.VITE_API_URL || '/api/v1').replace(/\/$/, '');

// Distingue respuestas, errores y espera para no presentar fallos como resultados del asistente.
export type MessageType = 'welcome' | 'text' | 'processing' | 'error';

// Producto recomendado que entrega el asistente en la respuesta de compra (MVP P2).
export type ProductoRecomendadoChat = {
  id: string;
  nombre: string;
  marca: string;
  categoria?: string;
  supermercado: string;
  precio: number;
  precioNormal: number;
  enOferta: boolean;
  urlImagen: string;
  unidad: string;
  enStock: boolean;
};

// Identifica cada mensaje para sustituir solo la espera correspondiente a su solicitud.
export type Message = {
  id: string;
  role: 'user' | 'assistant';
  type: MessageType;
  content?: string;
  products?: ProductoRecomendadoChat[];
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

// Normaliza los productos recomendados de la respuesta del asistente, descartando ítems inválidos.
function normalizarProductosRecomendados(valor: unknown): ProductoRecomendadoChat[] {
  if (!Array.isArray(valor)) return [];
  const identificadoresVistos = new Set<string>();
  const productos: ProductoRecomendadoChat[] = [];
  for (const elemento of valor) {
    if (typeof elemento !== 'object' || elemento === null) continue;
    const registro = elemento as Record<string, unknown>;
    const identificador = String(registro.id ?? registro.producto_id ?? '').trim();
    const nombre = typeof registro.nombre === 'string' ? registro.nombre.trim() : '';
    const precio = Number(registro.precio);
    if (!identificador || !nombre || !Number.isFinite(precio) || precio <= 0) continue;
    if (identificadoresVistos.has(identificador)) continue;
    identificadoresVistos.add(identificador);
    const precioNormal = Number(registro.precio_normal);
    const marca = typeof registro.marca === 'string' ? registro.marca.trim() : '';
    const categoria = typeof registro.categoria === 'string' ? registro.categoria.trim() : '';
    const supermercado = typeof registro.supermercado === 'string' ? registro.supermercado.trim() : '';
    const urlImagen = typeof registro.url_imagen === 'string' ? registro.url_imagen.trim() : '';
    const unidad = typeof registro.unidad === 'string' ? registro.unidad.trim() : '';
    productos.push({
      id: identificador,
      nombre,
      marca,
      categoria: categoria || undefined,
      supermercado,
      precio,
      precioNormal: Number.isFinite(precioNormal) && precioNormal > 0 ? precioNormal : precio,
      enOferta: Boolean(registro.en_oferta),
      urlImagen,
      unidad,
      enStock: registro.en_stock !== false,
    });
  }
  return productos;
}

// Centraliza conversación, sesión y solicitudes para mantener la vista separada del contrato HTTP.
export function useChatbot() {
  const { isGuest } = useAuth();
  const [mostrarModalAutenticacion, setMostrarModalAutenticacion] = useState(false);
  const [valorEntrada, setValorEntrada] = useState('');
  const [procesando, setProcesando] = useState(false);
  const [mensajes, setMensajes] = useState<Message[]>(() => [mensajeBienvenida()]);
  const referenciaDesplazamiento = useRef<HTMLDivElement>(null);
  const solicitudActiva = useRef<AbortController | null>(null);

  // Mantiene visible el último mensaje al cambiar la conversación.
  useEffect(() => {
    if (referenciaDesplazamiento.current) referenciaDesplazamiento.current.scrollTop = referenciaDesplazamiento.current.scrollHeight;
  }, [mensajes]);

  // Salir del chat cancela la espera, sin aplicar respuestas sobre una vista desmontada.
  useEffect(() => () => solicitudActiva.current?.abort(), []);

  // Envía una sola consulta con sesión y conserva el texto si falla para permitir su reintento.
  const manejarEnvio = async (texto: string) => {
    const mensaje = texto.trim();
    if (!mensaje || solicitudActiva.current) return;
    if (isGuest) {
      setMostrarModalAutenticacion(true);
      return;
    }

    // El ref bloquea también dos envíos que ocurran antes del siguiente render.
    const controlador = new AbortController();
    solicitudActiva.current = controlador;
    const identificadorEspera = nuevoIdentificador();
    // Sustituye la espera por el resultado o error, ignorando respuestas de solicitudes canceladas.
    const responder = (contenido: string, tipo: 'text' | 'error', productos: ProductoRecomendadoChat[] = []) => {
      if (controlador.signal.aborted) return;
      const mensajeRespuesta: Message = { id: nuevoIdentificador(), role: 'assistant', type: tipo, content: contenido };
      if (productos.length > 0) mensajeRespuesta.products = productos;
      setMensajes((anteriores) => [
        ...anteriores.filter((anterior) => anterior.id !== identificadorEspera),
        mensajeRespuesta,
      ]);
      if (tipo === 'error') setValorEntrada(mensaje);
    };

    setMensajes((anteriores) => [
      ...anteriores,
      { id: nuevoIdentificador(), role: 'user', type: 'text', content: mensaje },
      { id: identificadorEspera, role: 'assistant', type: 'processing' },
    ]);
    setValorEntrada('');
    setProcesando(true);

    try {
      const respuesta = await fetch(`${URL_API}/chat`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mensaje }),
        signal: controlador.signal,
      });
      if (controlador.signal.aborted) return;
      if (!respuesta.ok) {
        if (respuesta.status === 401) setMostrarModalAutenticacion(true);
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
      const productosRecomendados = normalizarProductosRecomendados(
        typeof datos === 'object' && datos !== null && 'productos_recomendados' in datos
          ? (datos as { productos_recomendados?: unknown }).productos_recomendados
          : undefined,
      );
      responder(datos.respuesta, 'text', productosRecomendados);
    } catch {
      if (!controlador.signal.aborted) responder(ERROR_CONEXION, 'error');
    } finally {
      if (solicitudActiva.current === controlador) {
        solicitudActiva.current = null;
        setProcesando(false);
      }
    }
  };

  // Cancela la espera antes de reiniciar el chat para que una respuesta tardía no lo repueble.
  const manejarVaciado = () => {
    solicitudActiva.current?.abort();
    solicitudActiva.current = null;
    setProcesando(false);
    setMensajes([mensajeBienvenida()]);
  };

  return {
    messages: mensajes,
    inputValue: valorEntrada,
    setInputValue: setValorEntrada,
    isProcessing: procesando,
    showAuthModal: mostrarModalAutenticacion,
    setShowAuthModal: setMostrarModalAutenticacion,
    scrollRef: referenciaDesplazamiento,
    handleSend: manejarEnvio,
    handleClear: manejarVaciado,
  };
}
