// Cliente del borrador de Motor/Listas; la cookie acredita la cuenta, nunca el almacenamiento local.
import {
  ErrorSolicitudListas,
  MAXIMO_ARTICULOS_LISTA,
  MAXIMO_ENVASES_ARTICULO,
  type ArticuloParaAgregar,
  type EstadoProducto,
  type ProductoLista,
} from "@/features/listas/listas-api";

// ===== Contrato persistido =====

/** Clave durable que el servidor congela antes de confirmar el carrito. */
export type EnvioCarrito = { lista_id: string; clave: string };
/** Resultado de una confirmación; aplicado=false conserva los productos del borrador. */
export type ReciboCarrito = EnvioCarrito & {
  aplicado: boolean;
  motivo?: string;
  rechazados?: { producto_id: string; motivo: string }[];
};
/** Documento del servidor, sin precios persistidos; disponibilidad se consulta a Catálogo. */
export type CarritoServidor = {
  version: number;
  articulos: ArticuloParaAgregar[];
  envio: EnvioCarrito | null;
  ultimo_envio: ReciboCarrito | null;
  disponibilidad: Record<
    string,
    { estado_producto: EstadoProducto; producto: ProductoLista | null }
  >;
};

const URL_API = (import.meta.env.VITE_API_URL || "/api/v1").replace(/\/$/, "");
const TIEMPO_ESPERA_MS = 20_000;

/** Rechaza respuestas ilegibles; nunca confunde HTML/JSON parcial con un carrito vacío confirmado. */
export function validarCarrito(datos: CarritoServidor): CarritoServidor {
  if (
    !datos ||
    !Number.isSafeInteger(datos.version) ||
    datos.version < 0 ||
    !Array.isArray(datos.articulos) ||
    datos.articulos.length > MAXIMO_ARTICULOS_LISTA ||
    datos.articulos.some(
      (articulo) =>
        !articulo ||
        typeof articulo.producto_id !== "string" ||
        !Number.isInteger(articulo.cantidad) ||
        articulo.cantidad < 1 ||
        articulo.cantidad > MAXIMO_ENVASES_ARTICULO,
    ) ||
    new Set(datos.articulos.map((articulo) => articulo.producto_id)).size !==
      datos.articulos.length ||
    datos.envio === undefined ||
    datos.ultimo_envio === undefined ||
    (datos.envio !== null && (!datos.envio?.clave || !datos.envio?.lista_id)) ||
    (datos.ultimo_envio !== null &&
      (!datos.ultimo_envio?.clave ||
        !datos.ultimo_envio?.lista_id ||
        typeof datos.ultimo_envio?.aplicado !== "boolean")) ||
    !datos.disponibilidad ||
    typeof datos.disponibilidad !== "object"
  )
    throw new Error(
      "El servicio no confirmó un carrito válido. Conservamos los datos locales.",
    );
  return datos;
}

/** Una espera agotada conserva la incertidumbre: la escritura podría haber terminado en el servidor. */
async function solicitar(
  usuarioID: string,
  ruta = "",
  cuerpo?: unknown,
): Promise<CarritoServidor> {
  const respuesta = await fetch(`${URL_API}/listas/carrito${ruta}`, {
    method: cuerpo === undefined ? "GET" : ruta ? "POST" : "PUT",
    credentials: "include",
    signal: AbortSignal.timeout(TIEMPO_ESPERA_MS),
    headers: {
      "X-Carrito-Cuenta": usuarioID,
      ...(cuerpo === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
    cache: "no-store",
  });
  const datos = await respuesta.json().catch(() => null);
  if (!respuesta.ok)
    throw new ErrorSolicitudListas(
      respuesta.status,
      respuesta.status === 401
        ? "Tu sesión expiró. Inicia sesión para recuperar tu carrito."
        : datos?.mensaje ||
            "No se pudo sincronizar el carrito. Tus cambios siguen pendientes.",
      datos?.detalle,
    );
  return validarCarrito(datos);
}

/** Operaciones versionadas; confirmar recibe sólo la clave del borrador congelado. */
export function crearCarritoApi(usuarioID: string) {
  return {
    obtener: () => solicitar(usuarioID),
    guardar: (version: number, articulos: ArticuloParaAgregar[]) =>
      solicitar(usuarioID, "", { version, articulos }),
    preparar: (version: number, envio: EnvioCarrito) =>
      solicitar(usuarioID, "/preparar", {
        version,
        lista_id: envio.lista_id,
        clave: envio.clave,
      }),
    confirmar: (clave: string) => solicitar(usuarioID, "/confirmar", { clave }),
  };
}
