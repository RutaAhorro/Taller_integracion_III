// Comparte una sola carga paginada de productos y facetas entre el catálogo y su lateral.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getProducts, parametrosCategoria } from '@/features/catalogo/products-api';
import { calculateDiscountPct } from '@/features/catalogo/formatters';
import type { UiProduct as Producto, UiSupermarket } from '@/features/catalogo/tipos';

const TAMANO_PAGINA = 24;
const MINIMO_BYTES_BUSQUEDA = 3; // El mínimo de q en Catálogo se mide en bytes UTF-8.
const PRODUCTOS_VACIOS: Producto[] = [];
const SUPERMERCADOS_VACIOS: UiSupermarket[] = [];

/** Conserva juntos los datos y el filtro que los produjo para no mezclar búsquedas. */
type EstadoCatalogo = {
  clave: string;
  productos: Producto[];
  supermercados: UiSupermarket[];
  pagina: number;
  totalPaginas: number;
  cargando: boolean;
  cargandoMas: boolean;
  error: string | null;
  errorMas: string | null;
};

/** Reinicia página, productos y facetas al comenzar una consulta o un reintento. */
function estadoInicial(clave: string, cargando: boolean): EstadoCatalogo {
  return {
    clave,
    productos: [],
    supermercados: [],
    pagina: 1,
    totalPaginas: 1,
    cargando,
    cargandoMas: false,
    error: null,
    errorMas: null,
  };
}

/** Calcula el ahorro sólo para conservar la presentación local de las ofertas ya descargadas. */
const calcularAhorro = (producto: Producto) => producto.originalPrice - producto.price;

/** Coordina productos y facetas; sólo una respuesta de los filtros vigentes puede actualizar la vista. */
export function useCatalog(consulta: string, categoria: string | null, supermercadoActivo: UiSupermarket | null) {
  const [intento, setIntento] = useState(0);
  const textoBusqueda = consulta.trim();
  const nombreSupermercado = supermercadoActivo?.name ?? null;
  const busquedaCorta = textoBusqueda !== '' && new TextEncoder().encode(textoBusqueda).length < MINIMO_BYTES_BUSQUEDA;
  const claveConsulta = JSON.stringify([textoBusqueda, categoria, nombreSupermercado, intento]);
  const [estado, setEstado] = useState<EstadoCatalogo>(() => estadoInicial(claveConsulta, !busquedaCorta));
  const versionPeticion = useRef(0);
  const controladorConsulta = useRef<AbortController | null>(null);
  // El ref bloquea dos eventos de scroll antes de que React publique cargandoMas.
  const cargaMasEnCurso = useRef(false);

  const parametrosFiltros = useMemo(() => {
    const parametros = new URLSearchParams({ en_stock: 'true' });
    if (textoBusqueda && !busquedaCorta) parametros.set('q', textoBusqueda);
    parametrosCategoria(categoria).forEach((valor, nombre) => parametros.set(nombre, valor));
    if (nombreSupermercado) parametros.set('supermercado', nombreSupermercado);
    return parametros.toString();
  }, [textoBusqueda, busquedaCorta, categoria, nombreSupermercado]);

  /** Usa exactamente los mismos filtros para productos y facetas, cambiando sólo la página. */
  const cargarPagina = useCallback(
    (numeroPagina: number, signal: AbortSignal) => {
      const parametros = new URLSearchParams(parametrosFiltros);
      parametros.set('page', String(numeroPagina));
      parametros.set('limit', String(TAMANO_PAGINA));
      return getProducts(`?${parametros}`, signal);
    },
    [parametrosFiltros],
  );

  useEffect(() => {
    const versionActual = ++versionPeticion.current;
    const controlador = new AbortController();
    controladorConsulta.current = controlador;
    cargaMasEnCurso.current = false;
    setEstado(estadoInicial(claveConsulta, !busquedaCorta));

    // No se sustituye una búsqueda corta por un filtro local de la primera página.
    if (!busquedaCorta) {
      cargarPagina(1, controlador.signal)
        .then((resultado) => {
          if (controlador.signal.aborted || versionPeticion.current !== versionActual) return;
          setEstado({
            ...estadoInicial(claveConsulta, false),
            productos: resultado.products,
            supermercados: resultado.markets,
            pagina: resultado.page,
            totalPaginas: resultado.totalPages,
          });
        })
        .catch(() => {
          if (controlador.signal.aborted || versionPeticion.current !== versionActual) return;
          setEstado({
            ...estadoInicial(claveConsulta, false),
            error: 'No se pudo cargar el catálogo ni sus supermercados. Intenta de nuevo.',
          });
        });
    }

    return () => controlador.abort();
  }, [claveConsulta, busquedaCorta, cargarPagina]);

  // La clave evita mostrar datos antiguos incluso antes del cleanup del efecto.
  const resultadoVigente = estado.clave === claveConsulta;
  const cargando = !resultadoVigente || estado.cargando;

  /** Agrega una sola página sin reemplazar las facetas completas de la búsqueda. */
  const cargarMas = () => {
    const controlador = controladorConsulta.current;
    if (
      busquedaCorta ||
      cargando ||
      cargaMasEnCurso.current ||
      estado.error ||
      estado.pagina >= estado.totalPaginas ||
      !controlador ||
      controlador.signal.aborted
    )
      return;

    const versionActual = versionPeticion.current;
    cargaMasEnCurso.current = true;
    setEstado((anterior) => ({ ...anterior, cargandoMas: true, errorMas: null }));

    cargarPagina(estado.pagina + 1, controlador.signal)
      .then((resultado) => {
        if (controlador.signal.aborted || versionPeticion.current !== versionActual) return;
        setEstado((anterior) => {
          if (anterior.clave !== claveConsulta) return anterior;
          const vistos = new Set(anterior.productos.map((producto) => producto.id));
          return {
            ...anterior,
            productos: [...anterior.productos, ...resultado.products.filter((producto) => !vistos.has(producto.id))],
            pagina: resultado.page,
            totalPaginas: resultado.totalPages,
          };
        });
      })
      .catch(() => {
        if (controlador.signal.aborted || versionPeticion.current !== versionActual) return;
        setEstado((anterior) => ({ ...anterior, errorMas: 'No se pudieron cargar más productos.' }));
      })
      .finally(() => {
        if (controlador.signal.aborted || versionPeticion.current !== versionActual) return;
        cargaMasEnCurso.current = false;
        setEstado((anterior) => ({ ...anterior, cargandoMas: false }));
      });
  };

  const productos = resultadoVigente && !estado.error ? estado.productos : PRODUCTOS_VACIOS;
  const supermercados = resultadoVigente && !estado.error ? estado.supermercados : SUPERMERCADOS_VACIOS;
  const filtrados = useMemo(() => {
    // La búsqueda y los filtros se aplican en Catálogo; no se vuelve a filtrar su respuesta.
    if (categoria === 'ofertas') {
      return [...productos].sort((anterior, siguiente) => calcularAhorro(siguiente) - calcularAhorro(anterior));
    }
    if (categoria === 'descuentos') {
      return [...productos].sort(
        (anterior, siguiente) =>
          calculateDiscountPct(siguiente.originalPrice, siguiente.price) -
          calculateDiscountPct(anterior.originalPrice, anterior.price),
      );
    }
    return productos;
  }, [productos, categoria]);

  /** Reintenta los filtros actuales, no una carga inicial sin búsqueda. */
  const reintentar = useCallback(() => setIntento((anterior) => anterior + 1), []);

  return {
    isLoading: cargando,
    error: resultadoVigente ? estado.error : null,
    filtered: filtrados,
    markets: supermercados,
    marketsReady: resultadoVigente && !estado.cargando && !estado.error && !busquedaCorta,
    supermarketCount: cargando ? 0 : supermercadoActivo ? (filtrados.length > 0 ? 1 : 0) : supermercados.length,
    notice: busquedaCorta ? 'Escribe una búsqueda más larga para consultar productos y supermercados.' : null,
    isFetchingMore: resultadoVigente && estado.cargandoMas,
    loadMoreError: resultadoVigente ? estado.errorMas : null,
    fetchMore: cargarMas,
    retry: reintentar,
  };
}
