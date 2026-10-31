// Contrato de SUP-240. La edición de artículos se incorpora con SUP-241.
const API_URL = (import.meta.env.VITE_API_URL || '/api/v1').replace(/\/$/, '');

export type EstadoLista = 'activa' | 'completada' | 'archivada';
export type Lista = {
  id: string;
  nombre: string;
  estado: EstadoLista;
  total_articulos: number;
  creada_el: string;
  actualizada_el: string;
};
export type ListaDetalle = Lista & {
  articulos: {
    id: number;
    producto_normalizado_id: string;
    cantidad: number;
    esta_comprado: boolean;
  }[];
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}/listas${path}`, {
    ...init,
    credentials: 'include',
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
  });
  if (!response.ok) {
    const data = await response.json().catch(() => null);
    if (response.status === 401) throw new Error('Tu sesión expiró. Vuelve a iniciar sesión.');
    throw new Error(data?.mensaje || 'No se pudo completar la solicitud de listas. Intenta nuevamente.');
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export const listasApi = {
  listar: (signal?: AbortSignal) => request<{ listas: Lista[] }>('', { signal }),
  obtener: (id: string, signal?: AbortSignal) => request<ListaDetalle>(`/${encodeURIComponent(id)}`, { signal }),
  crear: (nombre: string) => request<Lista>('', { method: 'POST', body: JSON.stringify({ nombre }) }),
  actualizar: (id: string, nombre: string, estado: EstadoLista) => request<Lista>(`/${encodeURIComponent(id)}`, {
    method: 'PUT', body: JSON.stringify({ nombre, estado }),
  }),
  eliminar: (id: string) => request<void>(`/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};
