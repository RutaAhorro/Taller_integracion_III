// Cliente de listas persistidas: envía la cookie de sesión y propaga errores sin simular guardados.
const URL_API = (import.meta.env.VITE_API_URL || "/api/v1").replace(/\/$/, "");

export type EstadoLista = "activa" | "completada" | "archivada";
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

// ===== Errores HTTP =====

/** Conserva el estado HTTP para no confundir un servicio caído con una sesión expirada. */
export class ErrorSolicitudListas extends Error {
  readonly estado: number;

  constructor(estado: number, mensaje: string) {
    super(mensaje);
    this.name = "ErrorSolicitudListas";
    this.estado = estado;
  }
}

// ===== Solicitudes =====

/** Ejecuta una operación de listas y conserva los errores de autenticación o negocio del servidor. */
async function solicitar<Respuesta>(
  ruta: string,
  opciones?: RequestInit,
): Promise<Respuesta> {
  const respuesta = await fetch(`${URL_API}/listas${ruta}`, {
    ...opciones,
    credentials: "include",
    headers: opciones?.body
      ? { "Content-Type": "application/json" }
      : undefined,
  });
  if (!respuesta.ok) {
    const datos = await respuesta.json().catch(() => null);
    if (respuesta.status === 401)
      throw new ErrorSolicitudListas(
        401,
        "Tu sesión expiró. Vuelve a iniciar sesión.",
      );
    throw new ErrorSolicitudListas(
      respuesta.status,
      datos?.mensaje ||
        "No se pudo completar la solicitud de listas. Intenta nuevamente.",
    );
  }
  if (respuesta.status === 204) return undefined as Respuesta;
  return respuesta.json();
}

export const listasApi = {
  listar: (senal?: AbortSignal) =>
    solicitar<{ listas: Lista[] }>("", { signal: senal }),
  obtener: (id: string, senal?: AbortSignal) =>
    solicitar<ListaDetalle>(`/${encodeURIComponent(id)}`, { signal: senal }),
  crear: (nombre: string) =>
    solicitar<Lista>("", { method: "POST", body: JSON.stringify({ nombre }) }),
  actualizar: (id: string, nombre: string, estado: EstadoLista) =>
    solicitar<Lista>(`/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify({ nombre, estado }),
    }),
  eliminar: (id: string) =>
    solicitar<void>(`/${encodeURIComponent(id)}`, { method: "DELETE" }),
};
