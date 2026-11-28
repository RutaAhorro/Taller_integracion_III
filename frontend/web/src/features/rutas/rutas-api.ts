// Consume la recomendación del Motor por Gateway; nunca calcula precios ni simula respuestas.
const URL_API = (import.meta.env.VITE_API_URL || "/api/v1").replace(/\/$/, "");

// ===== Contrato MVP (sección 0 de CONTRATO_MOTOR_RUTAS.md) =====

/** Perfiles viales admitidos; sin alias de scooter ni transporte público. */
export type MedioTransporte = "vehiculo" | "bicicleta" | "pie";

/** El origen se omite normalmente para que Motor aplique su valor por defecto. */
export type SolicitudOptimizacion = {
  lista_id: string;
  origen?: { lat: number; lon: number } | null;
  medio_transporte?: MedioTransporte;
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
  duracion_segundos?: number;
  geometrias?: string[];
};

/** Total confirmado para los artículos cubiertos, sin combustible en este MVP. */
export type RutaCalculada = {
  paradas: ParadaRuta[];
  tramos: TramoRuta[];
  distancia_km: number;
  costo_productos: number;
  costo_traslado: number;
  total: number;
  duracion_segundos?: number;
};

/** Puede no existir una tienda que cubra todos los artículos comparados. */
export type BaseUnaTienda = {
  sucursal_id: number;
  nombre: string;
  cadena?: string;
  total: number;
};

/**
 * Por qué un artículo quedó fuera: sin oferta con stock en el radio, o con
 * oferta pero en más tiendas que el máximo de paradas (ruta parcial).
 */
export type MotivoNoCubierto =
  | "sin_oferta_en_radio"
  | "fuera_del_maximo_de_paradas";

/** Un producto excluido no forma parte del total de la recomendación. */
export type ArticuloNoCubierto = {
  producto_id: string;
  /** Nombre de Catálogo; el Motor solo lo conoce si el producto tiene oferta. */
  nombre?: string;
  motivo: MotivoNoCubierto;
};

/** Respuesta vigente; no incluye los parámetros históricos de combustible. */
export type RespuestaOptimizacion = {
  fuente_recorrido?: "ors_osm" | "haversine";
  medio_transporte?: MedioTransporte;
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

// UUID canónico: admite cualquier versión, igual que los identificadores emitidos por Motor.
const PATRON_UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const LONGITUD_UUID_CANONICO = 36;
const PREFIJO_SIN_COBERTURA = "productos sin cobertura: ";
const MENSAJE_SIN_COBERTURA =
  "No se encontró una forma de comprar la lista en las tiendas cercanas.";

/** Valida la selección antes de consultar; el backend conserva la autorización por dueño. */
export function esListaIDValida(listaID: string | null): listaID is string {
  return (
    typeof listaID === "string" &&
    listaID.length === LONGITUD_UUID_CANONICO &&
    PATRON_UUID.test(listaID)
  );
}

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

/** Lee solo el detalle público conocido del Motor, nunca SQL, texto arbitrario o un contrato supuesto. */
export function productosSinCobertura(errorObtenido: unknown): string[] {
  if (
    !(errorObtenido instanceof ErrorSolicitudRutas) ||
    errorObtenido.estado !== 422 ||
    errorObtenido.message !== MENSAJE_SIN_COBERTURA ||
    typeof errorObtenido.detalle !== "string" ||
    !errorObtenido.detalle.startsWith(PREFIJO_SIN_COBERTURA)
  )
    return [];
  const identificadores = errorObtenido.detalle
    .slice(PREFIJO_SIN_COBERTURA.length)
    .split(",")
    .map((identificador) => identificador.trim());
  return identificadores.every(esListaIDValida)
    ? [...new Set(identificadores)]
    : [];
}

/** Explica qué revisar sin afirmar que la falta de ofertas prueba una causa concreta. */
export function orientacionErrorRuta(errorObtenido: unknown): string {
  if (!(errorObtenido instanceof ErrorSolicitudRutas))
    return "Comprueba tu conexión y vuelve a calcular. No se ha modificado tu lista.";
  if (errorObtenido.estado === 403)
    return "Vuelve a tus listas y selecciona una disponible para tu cuenta. Iniciar sesión de nuevo no garantiza acceso a esta lista.";
  if (errorObtenido.estado === 422) {
    if (errorObtenido.message === "La lista no tiene artículos por comprar.")
      return "Agrega productos o revisa si todos los artículos ya están marcados como comprados.";
    if (
      errorObtenido.message ===
      "La lista supera el máximo de artículos para calcular una ruta."
    )
      return "Divide los artículos pendientes en listas más pequeñas y vuelve a calcular.";
    if (
      errorObtenido.message ===
      "No hay supermercados cercanos para calcular una ruta."
    )
      return "No se encontraron sucursales físicas candidatas dentro del radio usado por el motor. Si no indicaste otro origen, se usa el centro de Temuco.";
    if (errorObtenido.message === MENSAJE_SIN_COBERTURA) {
      if (productosSinCobertura(errorObtenido).length > 0)
        return "No se obtuvo una combinación válida con los límites del motor. Los artículos indicados no tienen ofertas vigentes disponibles en las sucursales evaluadas. Puede deberse a falta de stock, precios vencidos o ausencia de ofertas en el radio. El resultado no permite distinguir cuál de esas causas aplica. Revisa o reemplaza los artículos en tu lista.";
      if (
        errorObtenido.detalle === "ningún conjunto de tiendas cubre la compra"
      )
        return "Ninguna combinación de tiendas dentro del máximo de paradas configurado cubre todos los artículos con oferta. Prueba dividiendo la lista o revisando los productos.";
      return "No hay una combinación válida de ofertas y tiendas cercanas para completar la compra con los límites del motor. La falta de ofertas puede deberse a stock, precios vencidos o cobertura; no se confirmó una causa individual. Revisa la lista antes de recalcular.";
    }
    return "Revisa los artículos pendientes y su disponibilidad en tu lista antes de volver a calcular.";
  }
  if (
    errorObtenido.estado === 503 &&
    errorObtenido.message ===
      "El servicio de recorridos por calles no está disponible."
  )
    return "No se obtuvo un recorrido por calles. El proveedor ORS puede no estar disponible, tener la cuota agotada o faltar configuración en Motor. No se ha sustituido por líneas rectas.";
  if (errorObtenido.estado === 503)
    return "El servicio de precios no está disponible ahora. Tu lista sigue guardada; vuelve a intentarlo más tarde.";
  if (errorObtenido.estado === 404)
    return "La lista pudo haberse eliminado o pertenecer a otra cuenta. Selecciona una desde Mis listas.";
  if (errorObtenido.estado === 401)
    return "Inicia sesión para volver a consultar tus listas. No se ha calculado una recomendación.";
  return "Vuelve a seleccionar tu lista o reintenta cuando el servicio esté disponible. No se ha modificado tu compra.";
}

/** Presenta causas de negocio del servidor sin convertir un fallo en una ruta de ejemplo. */
export function mensajeErrorRuta(errorObtenido: unknown): string {
  if (errorObtenido instanceof ErrorSolicitudRutas) {
    if (errorObtenido.estado === 401)
      return "Tu sesión expiró. Vuelve a iniciar sesión.";
    if (errorObtenido.estado === 403)
      return "No tienes permisos para consultar la ruta de esta lista.";
    if (errorObtenido.estado === 404)
      return "La lista no existe o no está disponible para tu cuenta.";
    if (errorObtenido.estado === 400)
      return "No se pudo solicitar la ruta. Vuelve a seleccionar la lista.";
    if (errorObtenido.estado === 422) {
      if (errorObtenido.message === MENSAJE_SIN_COBERTURA)
        return productosSinCobertura(errorObtenido).length > 0
          ? "Faltan ofertas vigentes para completar tu compra."
          : "No pudimos cubrir tu compra en las tiendas cercanas.";
      return errorObtenido.message;
    }
    if (
      errorObtenido.estado === 503 &&
      errorObtenido.message ===
        "El servicio de recorridos por calles no está disponible."
    )
      return "No pudimos consultar el recorrido por calles.";
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
  if (!esListaIDValida(solicitud.lista_id))
    throw new ErrorSolicitudRutas(
      400,
      "El identificador de la lista no es válido.",
    );
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
