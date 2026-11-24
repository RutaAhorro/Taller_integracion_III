// Gestiona la carga paginada del catálogo y los filtros disponibles en la API.
import { useState, useEffect, useMemo, useRef } from 'react';
import { useLayoutContext } from '@/layouts/MainLayout';
import { getProducts, parametrosCategoria } from '@/features/catalogo/products-api';
import { calculateDiscountPct } from '@/features/catalogo/formatters';
import type { UiProduct as Producto } from '@/features/catalogo/tipos';

const TAMANO_PAGINA = 24;

const calcularAhorro = (producto: Producto) => producto.originalPrice - producto.price;

export function useCatalog(consulta: string, categoria: string | null, supermercadoActivo: string | null) {
  const { markets } = useLayoutContext();
  const [productos, setProductos] = useState<Producto[]>([]);
  const [pagina, setPagina] = useState(1);
  const [totalPaginas, setTotalPaginas] = useState(1);
  const [cargando, setCargando] = useState(true);
  const [cargandoMas, setCargandoMas] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Descarta respuestas de peticiones que quedaron obsoletas al cambiar filtros.
  const idPeticion = useRef(0);

  const nombreSupermercado = markets.find((supermercado) => supermercado.id === supermercadoActivo)?.name ?? null;

  // Construye una página con los filtros admitidos por Catálogo.
  const cargarPagina = (numeroPagina: number) => {
    const parametros = new URLSearchParams({ page: String(numeroPagina), limit: String(TAMANO_PAGINA), en_stock: 'true' });
    const textoBusqueda = consulta.trim();
    if (textoBusqueda.length >= 3) parametros.set('q', textoBusqueda);
    parametrosCategoria(categoria).forEach((valor, nombre) => parametros.set(nombre, valor));
    if (nombreSupermercado) parametros.set('supermercado', nombreSupermercado);
    return getProducts(`?${parametros}`);
  };

  // Primera página cada vez que cambian los filtros.
  useEffect(() => {
    const peticionActual = ++idPeticion.current;
    setCargando(true);
    setError(null);

    cargarPagina(1)
      .then((resultado) => {
        if (idPeticion.current !== peticionActual) return;
        setProductos(resultado.products);
        setPagina(resultado.page);
        setTotalPaginas(resultado.totalPages);
      })
      .catch(() => {
        if (idPeticion.current !== peticionActual) return;
        setProductos([]);
        setError('No se pudo cargar el catálogo desde la base de datos.');
      })
      .finally(() => {
        if (idPeticion.current === peticionActual) setCargando(false);
      });

    return () => {
      idPeticion.current++;
    };
  }, [consulta, categoria, nombreSupermercado]);

  // Agrega la página siguiente al continuar el desplazamiento, sin duplicar productos.
  const cargarMas = () => {
    if (cargandoMas || cargando || pagina >= totalPaginas) return;
    const peticionActual = idPeticion.current;
    setCargandoMas(true);

    cargarPagina(pagina + 1)
      .then((resultado) => {
        if (idPeticion.current !== peticionActual) return;
        setProductos((anteriores) => {
          const vistos = new Set(anteriores.map((producto) => producto.id));
          return [...anteriores, ...resultado.products.filter((producto) => !vistos.has(producto.id))];
        });
        setPagina(resultado.page);
        setTotalPaginas(resultado.totalPages);
      })
      .catch(() => {
        if (idPeticion.current === peticionActual) setError('No se pudieron cargar más productos.');
      })
      .finally(() => setCargandoMas(false));
  };

  const filtrados = useMemo(() => {
    // Búsquedas de 1-2 letras no viajan a la API; se filtran aquí.
    const busqueda = consulta.trim().toLowerCase();
    const resultado = busqueda
      ? productos.filter((producto) => producto.name.toLowerCase().includes(busqueda) || producto.brand.toLowerCase().includes(busqueda))
      : productos;

    // "Mejores ofertas" ordena por ahorro en $; "Mayor descuento", por %.
    if (categoria === 'ofertas') return [...resultado].sort((productoAnterior, productoSiguiente) => calcularAhorro(productoSiguiente) - calcularAhorro(productoAnterior));
    if (categoria === 'descuentos') {
      return [...resultado].sort(
        (productoAnterior, productoSiguiente) => calculateDiscountPct(productoSiguiente.originalPrice, productoSiguiente.price) - calculateDiscountPct(productoAnterior.originalPrice, productoAnterior.price),
      );
    }
    return resultado;
  }, [productos, consulta, categoria]);

  const cantidadSupermercados = useMemo(() => new Set(filtrados.map((producto) => producto.supermarketId)).size, [filtrados]);

  return {
    isLoading: cargando,
    error,
    filtered: filtrados,
    supermarketCount: cantidadSupermercados,
    isFetchingMore: cargandoMas,
    fetchMore: cargarMas,
  };
}
