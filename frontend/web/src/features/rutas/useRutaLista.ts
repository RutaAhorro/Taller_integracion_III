// Coordina un cálculo por selección y descarta respuestas de listas o cuentas abandonadas.
import { useEffect, useState } from "react";
import { listasApi } from "@/features/listas/listas-api";
import { rutasApi, type RespuestaOptimizacion } from "./rutas-api";

/** El estado único evita presentar simultáneamente un resultado anterior y un error nuevo. */
export type EstadoRuta =
  | { tipo: "sin_lista" }
  | { tipo: "cargando" }
  | { tipo: "error"; error: unknown }
  | { tipo: "resultado"; datos: RespuestaOptimizacion };

/** Lee los nombres faltantes como complemento; la recomendación no depende de esa segunda lectura. */
export function useRutaLista(listaID: string | null) {
  const [estado, setEstado] = useState<EstadoRuta>(
    listaID ? { tipo: "cargando" } : { tipo: "sin_lista" },
  );
  const [nombres, setNombres] = useState<Record<string, string>>({});
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    if (!listaID) return;
    const controlador = new AbortController();
    setEstado({ tipo: "cargando" });
    setNombres({});
    rutasApi
      .optimizar({ lista_id: listaID }, controlador.signal)
      .then((datos) => {
        if (controlador.signal.aborted) return;
        setEstado({ tipo: "resultado", datos });
        if (datos.no_cubiertos.length === 0) return;
        // El endpoint de optimización solo entrega los identificadores de no cubiertos.
        void listasApi
          .obtener(listaID, controlador.signal)
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
            // Una lectura complementaria fallida no invalida el cálculo; se muestran sus UUID.
          });
      })
      .catch((errorObtenido: unknown) => {
        if (!controlador.signal.aborted)
          setEstado({ tipo: "error", error: errorObtenido });
      });
    return () => controlador.abort();
  }, [listaID, revision]);

  return {
    estado,
    nombres,
    recalcular: () => setRevision((actual) => actual + 1),
  };
}
