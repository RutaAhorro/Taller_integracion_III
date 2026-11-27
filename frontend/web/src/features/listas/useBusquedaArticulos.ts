// Busca páginas del catálogo para una lista, sin filtrar una muestra local ni conservar respuestas obsoletas.
import { useEffect, useState } from "react";
import { getProducts } from "@/features/catalogo/products-api";
import type { UiProduct } from "@/features/catalogo/tipos";

// ===== Límites y estado de búsqueda =====

export const MINIMO_BYTES_BUSQUEDA_ARTICULOS = 3;
const PRODUCTOS_POR_PAGINA = 8;

/** Conserva los productos junto con la solicitud que los produjo. */
type ResultadoBusqueda = {
  clave: string;
  productos: UiProduct[];
  totalPaginas: number;
  cargando: boolean;
  error: string | null;
};

/** Consulta solo al enviar el formulario; aborta al editar, paginar o desmontar la lista. */
export function useBusquedaArticulos() {
  const [consulta, setConsulta] = useState("");
  const [peticion, setPeticion] = useState<{
    texto: string;
    pagina: number;
    intento: number;
  } | null>(null);
  const [resultado, setResultado] = useState<ResultadoBusqueda | null>(null);
  const clave = peticion ? JSON.stringify(peticion) : "";
  const texto = peticion?.texto;
  const pagina = peticion?.pagina ?? 1;
  const busquedaCorta =
    new TextEncoder().encode(consulta.trim()).length <
    MINIMO_BYTES_BUSQUEDA_ARTICULOS;

  useEffect(() => {
    if (!texto) return;
    const controlador = new AbortController();
    setResultado({
      clave,
      productos: [],
      totalPaginas: 1,
      cargando: true,
      error: null,
    });
    const parametros = new URLSearchParams({
      q: texto,
      page: String(pagina),
      limit: String(PRODUCTOS_POR_PAGINA),
    });
    getProducts(`?${parametros}`, controlador.signal)
      .then((datos) => {
        if (controlador.signal.aborted) return;
        setResultado({
          clave,
          productos: datos.products,
          totalPaginas: datos.totalPages,
          cargando: false,
          error: null,
        });
      })
      .catch(() => {
        if (!controlador.signal.aborted) {
          setResultado({
            clave,
            productos: [],
            totalPaginas: 1,
            cargando: false,
            error: "No se pudieron buscar productos. Intenta nuevamente.",
          });
        }
      });
    return () => controlador.abort();
  }, [clave, texto, pagina]);

  /** Oculta resultados al editar para no seleccionar productos de una consulta anterior. */
  function cambiarConsulta(valor: string) {
    setConsulta(valor);
    setPeticion(null);
  }

  /** Limpia consulta y resultados tras completar el agregado a la lista. */
  function limpiarBusqueda() {
    setConsulta("");
    setPeticion(null);
    setResultado(null);
  }

  /** Repetir una búsqueda fallida genera una nueva solicitud, sin perder el texto escrito. */
  function buscar() {
    if (!busquedaCorta)
      setPeticion((actual) => ({
        texto: consulta.trim(),
        pagina: 1,
        intento: (actual?.intento ?? 0) + 1,
      }));
  }

  /** Mantiene los filtros en el servidor al cambiar de página. */
  function cambiarPagina(numeroPagina: number) {
    if (numeroPagina < 1 || numeroPagina > (resultado?.totalPaginas ?? 1))
      return;
    setPeticion((actual) =>
      actual ? { ...actual, pagina: numeroPagina } : null,
    );
  }

  const vigente = peticion !== null && resultado?.clave === clave;
  return {
    consulta,
    cambiarConsulta,
    limpiarBusqueda,
    buscar,
    cambiarPagina,
    busquedaCorta,
    consultado: peticion !== null,
    productos: vigente ? resultado.productos : [],
    cargando: peticion !== null && (!vigente || resultado.cargando),
    error: vigente ? resultado.error : null,
    pagina,
    totalPaginas: vigente ? resultado.totalPaginas : 1,
  };
}
