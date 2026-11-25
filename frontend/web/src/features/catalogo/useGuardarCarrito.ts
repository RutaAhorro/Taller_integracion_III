// Coordina destino y confirmación; el coordinador persistente conserva productos, clave y resultado.
import { useEffect, useRef, useState } from "react";
import {
  listasApi,
  ErrorSolicitudListas,
  MAXIMO_ARTICULOS_LISTA,
  type Lista,
} from "@/features/listas/listas-api";
import type { useCarritoPersistente } from "./useCarritoPersistente";
import type { ReciboCarrito } from "./carrito-api";

// ===== Estado del flujo =====

// Una espera agotada no demuestra que el servidor haya revertido la escritura.
const TIEMPO_ESPERA_ESCRITURA_MS = 20_000;

/** La creación de listas no es idempotente: sólo estos rechazos confirman que no se creó un destino. */
function esRechazoDefinitivo(errorObtenido: unknown): boolean {
  return (
    errorObtenido instanceof ErrorSolicitudListas &&
    [400, 401, 403, 404, 409, 422].includes(errorObtenido.estado)
  );
}

/** Traduce el recibo de negocio en una acción concreta sin mostrar códigos internos como instrucciones. */
function errorDelRecibo(recibo: ReciboCarrito): ErrorSolicitudListas {
  const mensajes: Record<string, string> = {
    max_articulos: `El destino supera ${MAXIMO_ARTICULOS_LISTA} productos distintos. Elige otra lista o quita productos del carrito.`,
    lista_no_existe:
      "La lista de destino ya no está disponible. Recarga las listas y elige otra.",
    productos_rechazados:
      "No se guardaron productos en la lista. Revisa los artículos indicados; conservamos todo el carrito.",
    idempotency_conflict:
      "La clave del envío ya corresponde a otro contenido. Conservamos el carrito; recupera su estado antes de confirmar de nuevo.",
  };
  return new ErrorSolicitudListas(
    409,
    mensajes[recibo.motivo ?? ""] ??
      "No se pudo confirmar la compra. Conservamos los productos; revisa el destino y las cantidades.",
    recibo.rechazados,
  );
}

/** Mantiene errores legibles sin descartar su estado HTTP. */
function normalizarError(errorObtenido: unknown): Error {
  if (errorObtenido instanceof TypeError)
    return new Error(
      "No se pudo conectar con el servicio. Conservamos tu carrito.",
    );
  if (
    errorObtenido instanceof Error &&
    ["TimeoutError", "AbortError"].includes(errorObtenido.name)
  )
    return new Error(
      "Se agotó el tiempo de espera. El servidor podría haber recibido la operación.",
    );
  return errorObtenido instanceof Error
    ? errorObtenido
    : new Error("No se pudo conectar. Conservamos tu carrito.");
}

/** Comprueba los campos que necesita el selector antes de mostrar una respuesta HTTP como destino. */
function esDestinoValido(lista: Lista): boolean {
  return (
    !!lista &&
    typeof lista.id === "string" &&
    !!lista.id &&
    typeof lista.nombre === "string" &&
    ["activa", "completada", "archivada"].includes(lista.estado)
  );
}

/** Mantiene el flujo durante la navegación; la cuenta propietaria lo desmonta al cambiar de usuario. */
export function useGuardarCarrito(
  usuarioID: string | undefined,
  persistencia: ReturnType<typeof useCarritoPersistente>,
) {
  const [abierto, setAbierto] = useState(false);
  const [listas, setListas] = useState<Lista[]>([]);
  const [destino, setDestino] = useState("");
  const [nombre, setNombre] = useState("");
  const [cargando, setCargando] = useState(false);
  const [ocupado, setOcupado] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [errorCarga, setErrorCarga] = useState<Error | null>(null);
  const [resultado, setResultado] = useState<{ id: string } | null>(null);
  const pendiente = persistencia.estado.intento !== null;
  const [creacionIncierta, setCreacionIncierta] = useState(false);
  const [revision, setRevision] = useState(0);
  const enCurso = useRef(false);
  const vigente = useRef(true);

  useEffect(() => {
    vigente.current = true;
    return () => {
      vigente.current = false;
    };
  }, []);

  useEffect(() => {
    const intento = persistencia.estado.intento;
    if (intento) setDestino(intento.lista_id);
    const recibo = persistencia.estado.recibo;
    if (recibo?.aplicado) setResultado({ id: recibo.lista_id });
    else if (recibo) setError(errorDelRecibo(recibo));
  }, [persistencia.estado.intento, persistencia.estado.recibo]);

  useEffect(() => {
    if (!abierto || !usuarioID) return;
    const controlador = new AbortController();
    setCargando(true);
    setErrorCarga(null);
    listasApi
      .listar(controlador.signal)
      .then(({ listas: recibidas }) => {
        if (!Array.isArray(recibidas) || !recibidas.every(esDestinoValido))
          throw new Error(
            "El servicio devolvió destinos inválidos. Recarga las listas.",
          );
        if (!controlador.signal.aborted) setListas(recibidas);
      })
      .catch((errorObtenido: unknown) => {
        if (!controlador.signal.aborted)
          setErrorCarga(normalizarError(errorObtenido));
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargando(false);
      });
    return () => controlador.abort();
  }, [abierto, usuarioID, revision]);

  // ===== Escrituras confirmadas por el servidor =====

  /** Crea sólo el destino, separado del lote para que un reintento no cree otra lista. */
  async function crearLista() {
    if (
      enCurso.current ||
      !usuarioID ||
      !nombre.trim() ||
      pendiente ||
      creacionIncierta
    )
      return;
    enCurso.current = true;
    setOcupado(true);
    setError(null);
    try {
      const creada = await listasApi.crear(
        nombre.trim(),
        AbortSignal.timeout(TIEMPO_ESPERA_ESCRITURA_MS),
      );
      if (!esDestinoValido(creada))
        throw new Error(
          "No se pudo confirmar la lista creada. Comprueba los destinos antes de crear otra.",
        );
      if (!vigente.current) return;
      setListas((actuales) => [creada, ...actuales]);
      setDestino(creada.id);
      setNombre("");
    } catch (errorObtenido) {
      if (!vigente.current) return;
      setError(normalizarError(errorObtenido));
      if (!esRechazoDefinitivo(errorObtenido)) {
        setCreacionIncierta(true);
        setRevision((actual) => actual + 1);
      }
    } finally {
      enCurso.current = false;
      if (vigente.current) setOcupado(false);
    }
  }

  /** Un doble clic comparte el bloqueo sincrónico; los reintentos comparten la operación congelada. */
  async function guardar() {
    if (enCurso.current || !usuarioID) return;
    enCurso.current = true;
    setOcupado(true);
    setError(null);
    try {
      const recibo = await persistencia.almacen.confirmar(destino);
      if (!vigente.current) return;
      if (!recibo.aplicado) throw errorDelRecibo(recibo);
      setResultado({ id: recibo.lista_id });
    } catch (errorObtenido) {
      if (!vigente.current) return;
      setError(normalizarError(errorObtenido));
    } finally {
      enCurso.current = false;
      if (vigente.current) setOcupado(false);
    }
  }

  /** Inicia una nueva revisión sin ocultar el resultado incierto de un envío anterior. */
  function abrir() {
    if (!pendiente) {
      setResultado(null);
      setError(null);
    }
    setAbierto(true);
  }

  return {
    abierto,
    abrir,
    cerrar: () => setAbierto(false),
    listas,
    destino,
    setDestino,
    nombre,
    setNombre,
    cargando,
    ocupado,
    error,
    errorCarga,
    resultado,
    pendiente,
    creacionIncierta,
    crearLista,
    guardar,
    recargar: () => setRevision((actual) => actual + 1),
    permitirOtraCreacion: () => {
      setCreacionIncierta(false);
      setError(null);
    },
    bloqueado:
      ocupado ||
      pendiente ||
      !persistencia.estado.listo ||
      !!persistencia.estado.conflicto,
  };
}
