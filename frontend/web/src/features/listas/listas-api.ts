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
// ===== Contrato de artículos (SUP-241 / SUP-318) =====

export const MAXIMO_ENVASES_ARTICULO = 99;
export const MAXIMO_ARTICULOS_LISTA = 100;

/** Estado confirmado por Catálogo; desconocido no equivale a disponible. */
export type EstadoProducto =
  "disponible" | "sin_stock" | "no_encontrado" | "desconocido";

/** Presentación del producto; el precio puede faltar aunque exista el producto. */
export type ProductoLista = {
  nombre: string;
  marca: string;
  supermercado: string;
  unidad: string;
  precio: number | null;
  url_imagen: string;
};

/** Campos persistidos que devuelve PUT, sin volver a consultar a Catálogo. */
export type Articulo = {
  id: number;
  producto_id: string | null;
  cantidad: number;
  esta_comprado: boolean;
};

/** Artículo con disponibilidad consultada por Motor; admite referencias históricas sin producto. */
export type ArticuloDetalle = Articulo & {
  estado_producto: EstadoProducto;
  producto: ProductoLista | null;
};

/** Detalle completo del usuario; nunca se completa con datos inventados del catálogo. */
export type ListaDetalle = Lista & { articulos: ArticuloDetalle[] };

/** Un envío en envases; el backend confirma existencia, límites y disponibilidad. */
export type ArticuloParaAgregar = { producto_id: string; cantidad: number };

// ===== Errores HTTP =====

/** Conserva el estado HTTP para no confundir un servicio caído con una sesión expirada. */
export class ErrorSolicitudListas extends Error {
  readonly estado: number;
  readonly detalle: unknown;

  constructor(estado: number, mensaje: string, detalle?: unknown) {
    super(mensaje);
    this.name = "ErrorSolicitudListas";
    this.estado = estado;
    this.detalle = detalle;
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
      ? { "Content-Type": "application/json", ...opciones.headers }
      : opciones?.headers,
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
      datos?.detalle,
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
  crear: (nombre: string, senal?: AbortSignal) =>
    solicitar<Lista>("", {
      method: "POST",
      body: JSON.stringify({ nombre }),
      signal: senal,
    }),
  actualizar: (id: string, nombre: string, estado: EstadoLista) =>
    solicitar<Lista>(`/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify({ nombre, estado }),
    }),
  eliminar: (id: string) =>
    solicitar<void>(`/${encodeURIComponent(id)}`, { method: "DELETE" }),
  agregarArticulos: (
    id: string,
    articulos: ArticuloParaAgregar[],
    claveOperacion: string,
    senal?: AbortSignal,
  ) =>
    solicitar<ListaDetalle>(`/${encodeURIComponent(id)}/articulos/lote`, {
      method: "POST",
      headers: { "Idempotency-Key": claveOperacion },
      body: JSON.stringify({ articulos }),
      signal: senal,
    }),
  actualizarArticulo: (
    listaID: string,
    articuloID: number,
    cambios: { cantidad?: number; esta_comprado?: boolean },
  ) =>
    solicitar<Articulo>(
      `/${encodeURIComponent(listaID)}/articulos/${articuloID}`,
      {
        method: "PUT",
        body: JSON.stringify(cambios),
      },
    ),
  eliminarArticulo: (listaID: string, articuloID: number) =>
    solicitar<void>(`/${encodeURIComponent(listaID)}/articulos/${articuloID}`, {
      method: "DELETE",
    }),
};
