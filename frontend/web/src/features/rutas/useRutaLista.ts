// Coordina un cálculo por selección y descarta respuestas de listas o cuentas abandonadas.
import { useEffect, useState } from "react";
import { listasApi } from "@/features/listas/listas-api";
import {
  esListaIDValida,
  productosSinCobertura,
  rutasApi,
  type RespuestaOptimizacion,
} from "./rutas-api";

/** El estado único evita presentar simultáneamente un resultado anterior y un error nuevo. */
export type EstadoRuta =
  | { tipo: "sin_lista" }
  | { tipo: "lista_invalida" }
  | { tipo: "cargando" }
  | { tipo: "error"; error: unknown }
  | { tipo: "resultado"; datos: RespuestaOptimizacion };

/** Lee los nombres faltantes como complemento; la recomendación no depende de esa segunda lectura. */
export function useRutaLista(listaID: string | null) {
  const [estado, setEstado] = useState<EstadoRuta>(
    !listaID
      ? { tipo: "sin_lista" }
      : esListaIDValida(listaID)
        ? { tipo: "cargando" }
        : { tipo: "lista_invalida" },
  );
  const [nombres, setNombres] = useState<Record<string, string>>({});
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    setNombres({});
    if (!listaID) {
      setEstado({ tipo: "sin_lista" });
      return;
    }
    if (!esListaIDValida(listaID)) {
      setEstado({ tipo: "lista_invalida" });
      return;
    }
    const listaSeleccionada = listaID;
    const controlador = new AbortController();
    setEstado({ tipo: "cargando" });
    /** Enriquece exclusiones confirmadas; un fallo complementario no sustituye el estado principal. */
    function cargarNombres() {
      void listasApi
        .obtener(listaSeleccionada, controlador.signal)
        .then((lista) => {
          if (controlador.signal.aborted) return;
          const nombresConfirmados: Record<string, string> = {};
          for (const articulo of lista.articulos) {
            if (articulo.producto_id && articulo.producto?.nombre)
              nombresConfirmados[articulo.producto_id] =
                articulo.producto.nombre;
          }
          setNombres(nombresConfirmados);
        })
        .catch(() => {
          // Si no hay nombres confirmados, la vista conserva los identificadores públicos.
        });
    }
    rutasApi
      .optimizar({ lista_id: listaID }, controlador.signal)
      .then((datos) => {
        if (controlador.signal.aborted) return;
        setEstado({ tipo: "resultado", datos });
        if (datos.no_cubiertos.length === 0) return;
        cargarNombres();
      })
      .catch((errorObtenido: unknown) => {
        if (controlador.signal.aborted) return;
        setEstado({ tipo: "error", error: errorObtenido });
        if (productosSinCobertura(errorObtenido).length > 0) cargarNombres();
      });
    return () => controlador.abort();
  }, [listaID, revision]);

  return {
    estado,
    nombres,
    recalcular: () => setRevision((actual) => actual + 1),
  };
}
