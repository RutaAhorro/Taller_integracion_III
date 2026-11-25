// Coordina lecturas y escrituras de una lista; publica cambios solo tras confirmación del backend.
import { useEffect, useRef, useState } from "react";
import { useToast } from "@/components/ToastContext";
import {
  listasApi,
  MAXIMO_ENVASES_ARTICULO,
  type Articulo,
  type EstadoLista,
  type Lista,
  type ListaDetalle,
} from "@/features/listas/listas-api";

// ===== Contrato de la vista =====

/** Comunica al listado los cambios confirmados y bloquea cambios de lista durante una escritura. */
export type OpcionesDetalleLista = {
  id: string;
  onUpdated: (lista: Lista) => void;
  onDeleted: () => void;
  onBusy: (ocupado: boolean) => void;
};

/** Normaliza errores sin perder el estado HTTP que usa la vista para ofrecer iniciar sesión. */
function convertirError(errorObtenido: unknown): Error {
  return errorObtenido instanceof Error
    ? errorObtenido
    : new Error("No se pudo completar la operación de la lista.");
}

/** Impide enviar decimales, cantidades vacías y valores fuera del contrato de envases. */
export function esCantidadValida(cantidad: number): boolean {
  return (
    Number.isInteger(cantidad) &&
    cantidad >= 1 &&
    cantidad <= MAXIMO_ENVASES_ARTICULO
  );
}

// ===== Lectura y operaciones serializadas =====

/** Conserva la lista y los formularios ante fallos; las respuestas de una cuenta desmontada se descartan. */
export function useDetalleLista({
  id,
  onUpdated,
  onDeleted,
  onBusy,
}: OpcionesDetalleLista) {
  const { toast } = useToast();
  const [lista, setLista] = useState<ListaDetalle | null>(null);
  const [nombre, setNombre] = useState("");
  const [estado, setEstado] = useState<EstadoLista>("activa");
  const [cargando, setCargando] = useState(true);
  const [guardando, setGuardando] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [revision, setRevision] = useState(0);
  const montado = useRef(false);
  const escrituraEnCurso = useRef(false);
  const intentosAgregar = useRef(new Map<string, string>());

  useEffect(() => {
    montado.current = true;
    return () => {
      montado.current = false;
      onBusy(false);
    };
  }, [onBusy]);

  useEffect(() => {
    const controlador = new AbortController();
    setCargando(true);
    setError(null);
    listasApi
      .obtener(id, controlador.signal)
      .then((datos) => {
        if (controlador.signal.aborted) return;
        setLista(datos);
        setNombre(datos.nombre);
        setEstado(datos.estado);
      })
      .catch((errorObtenido: unknown) => {
        if (!controlador.signal.aborted) {
          setLista(null);
          setError(convertirError(errorObtenido));
        }
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargando(false);
      });
    return () => controlador.abort();
  }, [id, revision]);

  /** El ref bloquea incluso dos eventos ocurridos antes del siguiente render de React. */
  async function ejecutar(accion: () => Promise<void>): Promise<boolean> {
    if (escrituraEnCurso.current || cargando || !lista) return false;
    escrituraEnCurso.current = true;
    setGuardando(true);
    onBusy(true);
    setError(null);
    try {
      await accion();
      return montado.current;
    } catch (errorObtenido) {
      if (montado.current) setError(convertirError(errorObtenido));
      return false;
    } finally {
      escrituraEnCurso.current = false;
      if (montado.current) {
        setGuardando(false);
        onBusy(false);
      }
    }
  }

  /** Mantiene el contador del listado de acuerdo con los artículos confirmados, no con envases. */
  function publicarDetalle(detalle: ListaDetalle) {
    if (!montado.current) return;
    setLista(detalle);
    onUpdated(detalle);
  }

  /** Edita metadatos sin borrar artículos ni aceptar éxito antes de recibir la respuesta. */
  async function guardar(): Promise<boolean> {
    if (!nombre.trim()) return false;
    return ejecutar(async () => {
      const actualizada = await listasApi.actualizar(id, nombre.trim(), estado);
      if (!montado.current || !lista) return;
      publicarDetalle({ ...lista, ...actualizada });
      setNombre(actualizada.nombre);
      setEstado(actualizada.estado);
      toast("Lista actualizada", "success");
    });
  }

  /** Elimina solo tras la confirmación del usuario realizada por la vista. */
  async function eliminar(): Promise<boolean> {
    return ejecutar(async () => {
      await listasApi.eliminar(id);
      if (!montado.current) return;
      onDeleted();
      toast("Lista eliminada", "success");
    });
  }

  /** Reutiliza la clave de un envío no confirmado; un reintento no vuelve a sumar envases. */
  async function agregarArticulo(
    productoID: string,
    cantidad: number,
  ): Promise<boolean> {
    if (!esCantidadValida(cantidad)) return false;
    return ejecutar(async () => {
      const firma = JSON.stringify([productoID, cantidad]);
      const claveOperacion =
        intentosAgregar.current.get(firma) ?? crypto.randomUUID();
      intentosAgregar.current.set(firma, claveOperacion);
      const detalle = await listasApi.agregarArticulos(
        id,
        [{ producto_id: productoID, cantidad }],
        claveOperacion,
      );
      if (!montado.current) return;
      intentosAgregar.current.delete(firma);
      publicarDetalle(detalle);
      toast("Producto guardado en la lista", "success");
    });
  }

  /** PUT devuelve solo campos persistidos: conserva la presentación y las advertencias de Catálogo. */
  async function actualizarArticulo(
    articuloID: number,
    cambios: { cantidad?: number; esta_comprado?: boolean },
  ): Promise<boolean> {
    if (cambios.cantidad !== undefined && !esCantidadValida(cambios.cantidad))
      return false;
    return ejecutar(async () => {
      const actualizado: Articulo = await listasApi.actualizarArticulo(
        id,
        articuloID,
        cambios,
      );
      if (!lista) return;
      publicarDetalle({
        ...lista,
        articulos: lista.articulos.map((articulo) =>
          articulo.id === articuloID
            ? { ...articulo, ...actualizado }
            : articulo,
        ),
      });
    });
  }

  /** Quita el artículo de la pantalla únicamente si Motor confirma DELETE 204. */
  async function eliminarArticulo(articuloID: number): Promise<boolean> {
    return ejecutar(async () => {
      await listasApi.eliminarArticulo(id, articuloID);
      if (!lista) return;
      const articulos = lista.articulos.filter(
        (articulo) => articulo.id !== articuloID,
      );
      publicarDetalle({
        ...lista,
        articulos,
        total_articulos: articulos.length,
      });
      if (montado.current) toast("Artículo eliminado", "success");
    });
  }

  /** Una recarga explícita consulta otra vez stock y precios; nunca compite con una escritura. */
  function recargar() {
    if (!escrituraEnCurso.current) setRevision((valor) => valor + 1);
  }

  return {
    lista,
    nombre,
    setNombre,
    estado,
    setEstado,
    cargando,
    guardando,
    error,
    guardar,
    eliminar,
    agregarArticulo,
    actualizarArticulo,
    eliminarArticulo,
    recargar,
  };
}
