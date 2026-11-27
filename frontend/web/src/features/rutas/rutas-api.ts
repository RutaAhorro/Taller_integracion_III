// Consume la recomendación del Motor por Gateway; nunca calcula precios ni simula respuestas.
const URL_API = (import.meta.env.VITE_API_URL || "/api/v1").replace(/\/$/, "");

// ===== Contrato MVP (sección 0 de CONTRATO_MOTOR_RUTAS.md) =====

/** El origen se omite normalmente para que Motor aplique su valor por defecto. */
export type SolicitudOptimizacion = {
  lista_id: string;
  origen?: { lat: number; lon: number } | null;
};

/** Conserva la identidad de la lista y la oferta equivalente elegida por Motor. */
export type ArticuloRuta = {
  producto_id: string;
  producto_id_oferta: string;
  nombre?: string;
  cantidad: number;
  precio_unitario: number;
  subtotal: number;
};

/** Sucursal física y compra asignada, en el orden recomendado. */
export type ParadaRuta = {
  orden: number;
  sucursal_id: number;
  nombre: string;
  cadena: string;
  lat: number;
  lon: number;
  subtotal: number;
  articulos: ArticuloRuta[];
};

/** Los tramos incluyen la salida y el retorno al origen, no calles reales. */
export type TramoRuta = {
  desde: number | "origen";
  hasta: number | "origen";
  distancia_km: number;
};

/** Total confirmado para los artículos cubiertos, sin combustible en este MVP. */
export type RutaCalculada = {
  paradas: ParadaRuta[];
  tramos: TramoRuta[];
  distancia_km: number;
  costo_productos: number;
  costo_traslado: number;
  total: number;
};

/** Puede no existir una tienda que cubra todos los artículos comparados. */
export type BaseUnaTienda = {
  sucursal_id: number;
  nombre: string;
  cadena?: string;
  total: number;
};

/** Un producto excluido no forma parte del total de la recomendación. */
export type ArticuloNoCubierto = {
  producto_id: string;
  motivo: "sin_oferta_en_radio";
};

/** Respuesta vigente; no incluye los parámetros históricos de combustible. */
export type RespuestaOptimizacion = {
  estimado: true;
  origen: { lat: number; lon: number; por_defecto: boolean };
  parametros: {
    factor_distancia: number;
    max_paradas: number;
    umbral_ahorro: number;
    radio_km: number;
  };
  ruta: RutaCalculada;
  base_una_tienda: BaseUnaTienda | null;
  ahorro: number | null;
  no_cubiertos: ArticuloNoCubierto[];
  candidatas_evaluadas: number;
};

// ===== Solicitud y errores =====

/** Mantiene el código HTTP para distinguir sesión, negocio y servicio no disponible. */
export class ErrorSolicitudRutas extends Error {
  readonly estado: number;
  readonly detalle: unknown;

  constructor(estado: number, mensaje: string, detalle?: unknown) {
    super(mensaje);
    this.name = "ErrorSolicitudRutas";
    this.estado = estado;
    this.detalle = detalle;
  }
}

/** Presenta causas de negocio del servidor sin convertir un fallo en una ruta de ejemplo. */
export function mensajeErrorRuta(errorObtenido: unknown): string {
  if (errorObtenido instanceof ErrorSolicitudRutas) {
    if (errorObtenido.estado === 401)
      return "Tu sesión expiró. Vuelve a iniciar sesión.";
    if (errorObtenido.estado === 404)
      return "La lista no existe o no está disponible para tu cuenta.";
    if (errorObtenido.estado === 400)
      return "No se pudo solicitar la ruta. Vuelve a seleccionar la lista.";
    if (errorObtenido.estado === 422) return errorObtenido.message;
    if (errorObtenido.estado === 503)
      return "No pudimos consultar los precios. Intenta nuevamente.";
  }
  return "No se pudo conectar con el servicio de rutas. Intenta nuevamente.";
}

/** Envía solo el contrato público con la cookie HttpOnly; permite cancelar al cambiar de vista. */
async function optimizar(
  solicitud: SolicitudOptimizacion,
  senal?: AbortSignal,
): Promise<RespuestaOptimizacion> {
  const respuesta = await fetch(`${URL_API}/rutas/optimizar`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(solicitud),
    signal: senal,
  });
  if (!respuesta.ok) {
    const datos = await respuesta.json().catch(() => null);
    throw new ErrorSolicitudRutas(
      respuesta.status,
      typeof datos?.mensaje === "string"
        ? datos.mensaje
        : "No se pudo calcular una ruta para esta lista.",
      datos?.detalle,
    );
  }
  return respuesta.json();
}

export const rutasApi = { optimizar };
