// Presenta la portada editorial o el catálogo filtrado, con sus estados de carga y paginación.
import { useEffect, useState, useRef } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Store,
  Loader2,
  ChevronLeft,
  ChevronRight,
  Apple,
  Milk,
  Beef,
  Croissant,
  CupSoda,
  Snowflake,
  SprayCan,
  Package,
  Cookie,
  LayoutGrid,
  Tag,
  Sparkles,
  ShoppingBasket,
  ArrowRight,
  ArrowLeft,
} from 'lucide-react'
import { useLayoutContext } from '@/layouts/MainLayout'
import { ProductCard } from '@/features/catalogo/ProductCard'
import { ProductCarousel } from '@/features/catalogo/ProductCarousel'
import { ProductSkeleton } from '@/features/catalogo/ProductSkeleton'
import { Footer } from '@/components/Footer'
import type { UiProduct as Producto } from '@/features/catalogo/tipos'
import { useCart } from '@/features/catalogo/CartContext'
import { getProducts, PREFIJO_CATEGORIA } from '@/features/catalogo/products-api'
import { formatearCategoria } from '@/features/catalogo/formatters'

type BloqueCache<Datos> = {
  datos: Datos
  capturadoEn: number
}

type CachePortada = {
  ofertas?: BloqueCache<Producto[]>
  marcas?: BloqueCache<Producto[]>
  supermercadosPortada?: BloqueCache<number>
}

const cachePortada: CachePortada = {}

// Ventana de reuso de la instantánea de la portada antes de volver a consultar.
const TTL_CACHE_PORTADA_MS = 5 * 60 * 1000

/** Conserva una captura de la portada sólo dentro de su ventana de validez. */
const cacheVigente = (capturadoEn?: number) =>
  capturadoEn !== undefined && Date.now() - capturadoEn <= TTL_CACHE_PORTADA_MS

// La categoría técnica de BD no representa una categoría de compra.
const CATEGORIA_OCULTA = 'scraped category'
const MAXIMO_REPRESENTATIVOS = 12
const MAXIMO_POR_CATEGORIA = 2
const LIMITE_CONSULTA_PORTADA = 24
const CANTIDAD_SKELETONS_PORTADA = 5
const DESPLAZAMIENTO_CATEGORIAS_PX = 280

/** Conserva una muestra representativa de cada categoría sin repetir nombres. */
function seleccionarRepresentativos(productos: Producto[], maximo: number = MAXIMO_REPRESENTATIVOS): Producto[] {
  const vistosPorNombre = new Set<string>()
  const conteoPorCategoria: Record<string, number> = {}
  const seleccionados: Producto[] = []

  for (const producto of productos) {
    if (seleccionados.length >= maximo) break

    const nombreBase = producto.name.split(',')[0].trim().toLowerCase()
    if (vistosPorNombre.has(nombreBase)) continue

    const categoria = producto.category?.trim().toLowerCase() || 'sin-categoria'
    const conteo = conteoPorCategoria[categoria] || 0
    if (conteo >= MAXIMO_POR_CATEGORIA) continue

    seleccionados.push(producto)
    vistosPorNombre.add(nombreBase)
    conteoPorCategoria[categoria] = conteo + 1
  }
  return seleccionados
}

/** Traduce la etiqueta visible de la categoría activa o del identificador interno. */
function formatearCategoriaActiva(categoria: string): string {
  if (categoria === 'ofertas') return 'Mejores ofertas'
  if (categoria === 'descuentos') return 'Mayor descuento'
  if (categoria === 'marcas') return 'Marcas blancas'

  const textoCategoria = categoria.startsWith(PREFIJO_CATEGORIA)
    ? categoria.slice(PREFIJO_CATEGORIA.length)
    : categoria
  return formatearCategoria(textoCategoria)
}

// Tarjetas editoriales destacadas con las imágenes convertidas a WebP.
const CATEGORIAS_DESTACADAS = [
  {
    slug: 'despensa',
    titulo: 'Despensa',
    imagen: '/images/categorias/despensa.webp',
    grande: true,
  },
  {
    slug: 'frutas-y-verduras',
    titulo: 'Frutas y verduras',
    imagen: '/images/categorias/frutas-y-verduras.webp',
    grande: true,
  },
  { slug: 'lacteos', titulo: 'Lácteos', imagen: '/images/categorias/lacteos.webp', grande: false },
  {
    slug: 'carnes-y-pescados',
    titulo: 'Carnes y pescados',
    imagen: '/images/categorias/carnes-y-pescados.webp',
    grande: false,
  },
  { slug: 'bebidas', titulo: 'Bebidas', imagen: '/images/categorias/bebidas.webp', grande: false },
  {
    slug: 'aseo-y-limpieza',
    titulo: 'Aseo y limpieza',
    imagen: '/images/categorias/aseo-y-limpieza.webp',
    grande: false,
  },
]

// Asocia la palabra de la categoría con el icono editorial correspondiente.
function iconoParaCategoria(slug: string) {
  const normalizado = slug.toLowerCase()
  if (normalizado.includes('fruta') || normalizado.includes('verdura')) return Apple
  if (
    normalizado.includes('lacteo') ||
    normalizado.includes('leche') ||
    normalizado.includes('yoghurt') ||
    normalizado.includes('queso') ||
    normalizado.includes('huevo')
  )
    return Milk
  if (
    normalizado.includes('carne') ||
    normalizado.includes('pollo') ||
    normalizado.includes('vacuno') ||
    normalizado.includes('cerdo') ||
    normalizado.includes('pescado')
  )
    return Beef
  if (
    normalizado.includes('panaderia') ||
    normalizado.includes('pasteleria') ||
    normalizado.includes('reposteria')
  )
    return Croissant
  if (
    normalizado.includes('bebida') ||
    normalizado.includes('agua') ||
    normalizado.includes('jugo') ||
    normalizado.includes('cerveza') ||
    normalizado.includes('vino')
  )
    return CupSoda
  if (normalizado.includes('congelado') || normalizado.includes('refrigerado')) return Snowflake
  if (
    normalizado.includes('aseo') ||
    normalizado.includes('limpieza') ||
    normalizado.includes('detergente') ||
    normalizado.includes('bano') ||
    normalizado.includes('hogar')
  )
    return SprayCan
  if (
    normalizado.includes('despensa') ||
    normalizado.includes('arroz') ||
    normalizado.includes('pasta') ||
    normalizado.includes('aceite') ||
    normalizado.includes('conserva') ||
    normalizado.includes('harina')
  )
    return Package
  if (
    normalizado.includes('snack') ||
    normalizado.includes('chocolate') ||
    normalizado.includes('galleta') ||
    normalizado.includes('dulce')
  )
    return Cookie
  return LayoutGrid
}

export default function Page() {
  const {
    query,
    category,
    activeMarket,
    activeMarketName,
    markets,
    categorias,
    cargandoCategorias,
    errorCategorias,
    reintentarCategorias,
    catalogo,
    onSelectCategory,
    onResetFilters,
    resetVersion,
  } = useLayoutContext()
  const { addToCart } = useCart()
  const supermercadoSeleccionado = activeMarketName ?? markets.find((mercado) => mercado.id === activeMarket)?.name
  const consultaActiva = query.trim()
  const categoriaActiva = formatearCategoriaActiva(category)
  const hayFiltroActivo = Boolean(consultaActiva || category || activeMarket)
  const [parametrosUrl, setParametrosUrl] = useSearchParams()
  const explorarTodo = parametrosUrl.get('vista') === 'catalogo'
  const enModoPortada = !hayFiltroActivo && !explorarTodo

  // El orden de presentación depende del tipo de resultado consultado.
  const ordenadoPor =
    category === 'ofertas'
      ? 'Ordenados por mayor ahorro'
      : category === 'descuentos'
        ? 'Ordenados por mayor descuento'
        : 'Ordenados de menor a mayor precio'

  const abrirCatalogoCompleto = () => setParametrosUrl({ vista: 'catalogo' })
  const volverAPortada = () => {
    onResetFilters()
    if (explorarTodo) setParametrosUrl({})
  }

  const referenciaScroll = useRef<HTMLDivElement>(null)

  useEffect(() => {
    // Al cambiar de modo, filtro o contexto se vuelve al inicio del listado.
    referenciaScroll.current?.scrollTo({ top: 0 })
  }, [enModoPortada, category, consultaActiva, activeMarket])

  const versionReinicioPrevia = useRef(resetVersion)

  useEffect(() => {
    // El logo vuelve a la portada incluso desde la vista de catálogo completo.
    if (resetVersion === versionReinicioPrevia.current) return
    versionReinicioPrevia.current = resetVersion
    if (parametrosUrl.get('vista') === 'catalogo') setParametrosUrl({})
  }, [resetVersion, parametrosUrl, setParametrosUrl])

  // ===== Datos reales de la portada =====
  const ofertasEnCacheVigentes = cacheVigente(cachePortada.ofertas?.capturadoEn)
    ? cachePortada.ofertas?.datos
    : undefined
  const marcasEnCacheVigentes = cacheVigente(cachePortada.marcas?.capturadoEn)
    ? cachePortada.marcas?.datos
    : undefined
  const supermercadosEnCacheVigentes = cacheVigente(cachePortada.supermercadosPortada?.capturadoEn)
    ? cachePortada.supermercadosPortada?.datos
    : undefined

  const [ofertasVigentes, setOfertasVigentes] = useState<Producto[]>(ofertasEnCacheVigentes ?? [])
  const [marcasPropias, setMarcasPropias] = useState<Producto[]>(marcasEnCacheVigentes ?? [])
  const [supermercadosPortada, setSupermercadosPortada] = useState<number | undefined>(
    supermercadosEnCacheVigentes,
  )
  const [cargandoOfertas, setCargandoOfertas] = useState(!ofertasEnCacheVigentes)
  const [cargandoMarcas, setCargandoMarcas] = useState(!marcasEnCacheVigentes)
  const [errorOfertas, setErrorOfertas] = useState(false)
  const [errorMarcas, setErrorMarcas] = useState(false)
  const [intentoPortada, setIntentoPortada] = useState(0)
  const referenciaCategorias = useRef<HTMLElement>(null)

  const categoriasPortada = (categorias ?? []).filter(
    (categoria) => categoria.trim().toLowerCase() !== CATEGORIA_OCULTA,
  )

  const desplazarCategorias = (direccion: number) => {
    referenciaCategorias.current?.scrollBy({ left: direccion * DESPLAZAMIENTO_CATEGORIAS_PX, behavior: 'smooth' })
  }

  // Carga progresiva independiente; cada bloque maneja su propio TTL y limpia datos vencidos al fallar.
  useEffect(() => {
    if (!enModoPortada) return
    const controlador = new AbortController()

    const registrarSupermercadosPortada = (cantidad: number) => {
      const actual = cacheVigente(cachePortada.supermercadosPortada?.capturadoEn)
        ? (cachePortada.supermercadosPortada?.datos ?? 0)
        : 0
      const maximo = Math.max(actual, cantidad)
      cachePortada.supermercadosPortada = { datos: maximo, capturadoEn: Date.now() }
      setSupermercadosPortada(maximo)
    }

    if (!cachePortada.ofertas || !cacheVigente(cachePortada.ofertas.capturadoEn)) {
      delete cachePortada.ofertas
      setOfertasVigentes([])
      setCargandoOfertas(true)
      setErrorOfertas(false)
      getProducts(`?en_stock=true&en_oferta=true&page=1&limit=${LIMITE_CONSULTA_PORTADA}`, controlador.signal)
        .then((resultado) => {
          if (controlador.signal.aborted) return
          const representativos = seleccionarRepresentativos(resultado.products)
          cachePortada.ofertas = { datos: representativos, capturadoEn: Date.now() }
          registrarSupermercadosPortada(resultado.markets.length)
          setOfertasVigentes(representativos)
          setErrorOfertas(false)
        })
        .catch(() => {
          if (controlador.signal.aborted) return
          delete cachePortada.ofertas
          setOfertasVigentes([])
          setErrorOfertas(true)
        })
        .finally(() => {
          if (!controlador.signal.aborted) setCargandoOfertas(false)
        })
    }

    if (!cachePortada.marcas || !cacheVigente(cachePortada.marcas.capturadoEn)) {
      delete cachePortada.marcas
      setMarcasPropias([])
      setCargandoMarcas(true)
      setErrorMarcas(false)
      getProducts(`?en_stock=true&solo_marcas_blancas=true&page=1&limit=${LIMITE_CONSULTA_PORTADA}`, controlador.signal)
        .then((resultado) => {
          if (controlador.signal.aborted) return
          const representativos = seleccionarRepresentativos(resultado.products)
          cachePortada.marcas = { datos: representativos, capturadoEn: Date.now() }
          registrarSupermercadosPortada(resultado.markets.length)
          setMarcasPropias(representativos)
          setErrorMarcas(false)
        })
        .catch(() => {
          if (controlador.signal.aborted) return
          delete cachePortada.marcas
          setMarcasPropias([])
          setErrorMarcas(true)
        })
        .finally(() => {
          if (!controlador.signal.aborted) setCargandoMarcas(false)
        })
    }

    return () => controlador.abort()
  }, [enModoPortada, intentoPortada])

  const portadaVacia = !cargandoOfertas && !cargandoMarcas && !errorOfertas && !errorMarcas && ofertasVigentes.length === 0 && marcasPropias.length === 0

  const contadorSupermercados =
    catalogo.supermarketCount > 0 ? catalogo.supermarketCount : supermercadosPortada ?? 0

  const tituloHero = consultaActiva
    ? `Resultados para “${consultaActiva}”`
    : category
      ? categoriaActiva
      : supermercadoSeleccionado
        ? `Catálogo en ${supermercadoSeleccionado}`
        : 'Compara precios antes de salir a comprar'
  const subtituloHero = consultaActiva
    ? `Mostrando coincidencias para “${consultaActiva}”${supermercadoSeleccionado ? ` en ${supermercadoSeleccionado}` : ''}${category ? ` · ${categoriaActiva}` : ''}.`
    : category
      ? `Mostrando ${categoriaActiva.toLowerCase()}${supermercadoSeleccionado ? ` en ${supermercadoSeleccionado}` : ''}.`
      : supermercadoSeleccionado
        ? `Filtrando el catálogo en ${supermercadoSeleccionado}.`
        : 'Explora los precios vigentes o abre el menú ☰ para filtrar por categoría o supermercado.'

  const manejarDesplazamiento = (evento: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = evento.currentTarget;
    if (!enModoPortada && scrollHeight - scrollTop <= clientHeight + 50) {
      catalogo.fetchMore();
    }
  };

  const manejarAgregar = (producto: Producto) => {
    addToCart(producto)
  }
  return (
    <div className="relative flex flex-1 w-full h-full bg-background overflow-hidden">
      {/* Fondo de imagen geométrica vibrante */}
      <div
        className="absolute inset-0 z-0 opacity-[0.60] bg-[url('/images/dashboard-seamless.jpg')] bg-cover bg-center bg-no-repeat pointer-events-none"
        aria-hidden="true"
      />
      <main className="relative z-10 mx-auto flex w-full max-w-[1600px] flex-1 flex-col gap-6 overflow-hidden px-4 py-6 md:px-6">
        <section
          aria-label={enModoPortada ? 'Portada del catálogo' : 'Resultados del catálogo'}
          className="flex min-h-0 flex-1 flex-col"
        >
          <div
            ref={referenciaScroll}
            className="min-h-0 flex-1 overflow-y-auto pb-28 md:pb-16 pr-1 -mr-1 scrollbar-hide"
            onScroll={manejarDesplazamiento}
          >
            {!enModoPortada && (
              <div className="sticky top-0 z-20 mb-4 flex items-center justify-between gap-3 rounded-xl border-2 border-border bg-card/95 px-3.5 py-2 shadow-[3px_3px_0px_var(--color-border)] backdrop-blur-sm">
                <button
                  type="button"
                  onClick={volverAPortada}
                  className="inline-flex items-center gap-1.5 rounded-full border border-border bg-card px-3.5 py-1.5 text-xs font-bold text-foreground transition-colors hover:bg-primary/10"
                >
                  <ArrowLeft className="h-4 w-4" />
                  {hayFiltroActivo ? 'Limpiar filtros' : 'Volver a portada'}
                </button>
                <span className="truncate text-xs font-semibold text-muted-foreground">
                  {consultaActiva
                    ? `“${consultaActiva}”`
                    : categoriaActiva || supermercadoSeleccionado || 'Catálogo completo'}
                </span>
              </div>
            )}
            <section className="mb-5 rounded-2xl border-2 border-border bg-card p-5 shadow-[6px_6px_0px_var(--color-border)] md:p-6">
              <div className="min-w-0">
                <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-primary/10 px-3 py-1 text-xs font-bold text-primary">
                  {contadorSupermercados > 0 ? (
                    <>
                      {contadorSupermercados}{' '}
                      {contadorSupermercados === 1 ? 'supermercado activo' : 'supermercados activos'}
                    </>
                  ) : (
                    'Comparador de precios vigente'
                  )}
                </span>
                <h1 className="mt-2 font-['Fraunces'] text-2xl font-bold tracking-tight text-foreground md:text-3xl">
                  {tituloHero}
                </h1>
                <p className="mt-1 text-sm text-muted-foreground">
                  {subtituloHero}
                </p>

                {enModoPortada && (
                  <>
                    <div className="mt-4 flex flex-wrap items-center gap-3">
                      <button
                        type="button"
                        onClick={() => onSelectCategory('ofertas')}
                        className="bg-primary text-primary-foreground inline-flex items-center gap-2 rounded-full px-5 py-2.5 text-sm font-bold shadow-[2px_2px_0px_var(--color-border)] transition-all hover:-translate-y-0.5"
                      >
                        Ver ofertas de hoy
                        <ArrowRight className="h-4 w-4" />
                      </button>
                      <button
                        type="button"
                        onClick={() => abrirCatalogoCompleto()}
                        className="bg-background text-foreground rounded-full border-2 border-border px-5 py-2.5 text-sm font-bold transition-colors hover:bg-primary/10"
                      >
                        Explorar todo
                      </button>
                    </div>
                    <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-border/60 pt-4 text-xs font-medium text-muted-foreground">
                      <div className="flex items-center gap-1.5">
                        <Tag className="h-4 w-4 text-primary" />
                        <span>Precios reales actualizados</span>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <Store className="h-4 w-4 text-primary" />
                        <span>Compara entre supermercados</span>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <Sparkles className="h-4 w-4 text-primary" />
                        <span>Asistente IA para tu lista</span>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <ShoppingBasket className="h-4 w-4 text-primary" />
                        <span>Marcas propias para ahorrar</span>
                      </div>
                    </div>
                  </>
                )}
              </div>
            </section>

            {enModoPortada ? (
              <>
                {cargandoCategorias && (
                  <div className="mb-6 flex items-center gap-2 rounded-2xl border-2 border-border bg-card p-2 shadow-[4px_4px_0px_var(--color-border)]">
                    <p className="px-2 text-sm text-muted-foreground">Cargando categorías…</p>
                  </div>
                )}

                {errorCategorias && (
                  <div className="mb-6 flex items-center gap-2 rounded-2xl border-2 border-border bg-card p-2 shadow-[4px_4px_0px_var(--color-border)]">
                    <div role="alert" className="px-2 text-sm">
                      <p>No pudimos cargar las categorías.</p>
                      <button type="button" className="underline" onClick={reintentarCategorias}>Reintentar</button>
                    </div>
                  </div>
                )}

                {!cargandoCategorias && !errorCategorias && categoriasPortada.length > 0 && (
                  <div className="mb-6 flex items-center gap-2 rounded-2xl border-2 border-border bg-card p-2 shadow-[4px_4px_0px_var(--color-border)]">
                    <button
                      type="button"
                      onClick={() => desplazarCategorias(-1)}
                      className="inline-flex rounded-full border border-border bg-card p-1 sm:p-1.5 text-foreground hover:bg-primary/10 shrink-0"
                      aria-label="Categorías anteriores"
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </button>
                    <nav
                      ref={referenciaCategorias}
                      className="flex flex-1 min-w-0 items-stretch gap-1.5 overflow-x-auto scrollbar-hide"
                      aria-label="Categorías del catálogo"
                    >
                      <button
                        type="button"
                        onClick={() => abrirCatalogoCompleto()}
                        className="bg-primary text-primary-foreground flex w-24 shrink-0 flex-col items-center justify-center gap-1.5 rounded-xl px-2 py-2.5 text-center text-xs font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        <LayoutGrid className="h-5 w-5" aria-hidden="true" />
                        <span>Todas</span>
                      </button>
                      {categoriasPortada.map((categoria) => {
                        const Icono = iconoParaCategoria(categoria)
                        return (
                          <button
                            key={categoria}
                            type="button"
                            onClick={() => onSelectCategory(`${PREFIJO_CATEGORIA}${categoria}`)}
                            className="flex w-24 sm:w-28 shrink-0 flex-col items-center justify-center gap-1.5 rounded-xl px-2 py-2.5 text-center text-xs font-semibold text-foreground transition-colors hover:bg-primary/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                          >
                            <Icono className="h-5 w-5 text-primary" aria-hidden="true" />
                            <span className="line-clamp-2 leading-tight">
                              {formatearCategoriaActiva(categoria)}
                            </span>
                          </button>
                        )
                      })}
                    </nav>
                    <button
                      type="button"
                      onClick={() => desplazarCategorias(1)}
                      className="inline-flex rounded-full border border-border bg-card p-1 sm:p-1.5 text-foreground hover:bg-primary/10 shrink-0"
                      aria-label="Categorías siguientes"
                    >
                      <ChevronRight className="h-4 w-4" />
                    </button>
                  </div>
                )}

                {(() => {
                  const destacadasVisibles = CATEGORIAS_DESTACADAS.filter((destacada) =>
                    categoriasPortada.includes(destacada.slug),
                  )
                  if (destacadasVisibles.length < 2) return null

                  return (
                    <section className="mb-8" aria-label="Categorías destacadas">
                      <h2 className="mb-4 font-['Fraunces'] text-xl font-bold text-foreground">
                        Categorías destacadas
                      </h2>
                      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
                        {destacadasVisibles.map((destacada) => (
                          <button
                            key={destacada.slug}
                            type="button"
                            onClick={() => onSelectCategory(`${PREFIJO_CATEGORIA}${destacada.slug}`)}
                            className={`group relative overflow-hidden rounded-2xl border-2 border-border bg-card text-left shadow-[4px_4px_0px_var(--color-border)] transition-all hover:-translate-y-0.5 hover:shadow-[6px_6px_0px_var(--color-border)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                              destacada.grande ? 'col-span-2 h-44 sm:h-52' : 'col-span-1 h-40 sm:h-48'
                            }`}
                          >
                            <img
                              src={destacada.imagen}
                              alt=""
                              aria-hidden="true"
                              loading="lazy"
                              className="absolute inset-0 h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                            />
                            <div
                              className={`absolute inset-0 ${
                                destacada.grande
                                  ? 'bg-gradient-to-r from-card/95 via-card/65 to-transparent'
                                  : 'bg-gradient-to-b from-card/95 via-card/55 to-transparent'
                              }`}
                            />
                            <div
                              className={`relative flex h-full flex-col p-4 sm:p-5 ${
                                destacada.grande ? 'justify-center' : 'justify-start'
                              }`}
                            >
                              <h3 className="text-lg font-extrabold leading-tight text-foreground sm:text-xl">
                                {destacada.titulo}
                              </h3>
                              <div className="mt-1.5 h-1 w-9 rounded-full bg-primary" />
                            </div>
                          </button>
                        ))}
                      </div>
                    </section>
                  )
                })()}

                {errorOfertas && (
                  <div
                    role="alert"
                    className="mt-6 flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-12 text-center"
                  >
                    <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                    <p className="text-sm text-muted-foreground">No pudimos cargar las ofertas vigentes.</p>
                    <button
                      type="button"
                      className="rounded-md px-2 py-1 text-sm underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      onClick={() => setIntentoPortada((anterior) => anterior + 1)}
                    >
                      Reintentar
                    </button>
                  </div>
                )}

                {cargandoOfertas ? (
                  <div className="mt-6 flex gap-4 overflow-hidden" aria-busy="true">
                    {Array.from({ length: CANTIDAD_SKELETONS_PORTADA }).map((_, indice) => (
                      <div key={indice} className="w-48 shrink-0 sm:w-56">
                        <ProductSkeleton />
                      </div>
                    ))}
                  </div>
                ) : (
                  !errorOfertas &&
                  ofertasVigentes.length > 0 && (
                    <ProductCarousel
                      products={ofertasVigentes}
                      title="Ofertas vigentes"
                      subtitle="Precios rebajados encontrados hoy en supermercados"
                      ariaLabel="Ofertas vigentes"
                      actionLabel="Ver todas"
                      onAction={() => onSelectCategory('ofertas')}
                      onAdd={manejarAgregar}
                      className="mt-6"
                    />
                  )
                )}

                {errorMarcas && (
                  <div
                    role="alert"
                    className="mt-8 flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-12 text-center"
                  >
                    <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                    <p className="text-sm text-muted-foreground">No pudimos cargar las marcas propias.</p>
                    <button
                      type="button"
                      className="rounded-md px-2 py-1 text-sm underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      onClick={() => setIntentoPortada((anterior) => anterior + 1)}
                    >
                      Reintentar
                    </button>
                  </div>
                )}

                {cargandoMarcas ? (
                  <div className="mt-8 flex gap-4 overflow-hidden border-t border-border pt-6" aria-busy="true">
                    {Array.from({ length: CANTIDAD_SKELETONS_PORTADA }).map((_, indice) => (
                      <div key={indice} className="w-48 shrink-0 sm:w-56">
                        <ProductSkeleton />
                      </div>
                    ))}
                  </div>
                ) : (
                  !errorMarcas &&
                  marcasPropias.length > 0 && (
                    <ProductCarousel
                      products={marcasPropias}
                      title="Marcas propias para ahorrar"
                      subtitle="Alternativas de marca de supermercado con stock vigente"
                      ariaLabel="Marcas propias"
                      actionLabel="Ver todas"
                      onAction={() => onSelectCategory('marcas')}
                      onAdd={manejarAgregar}
                      className="mt-8 border-t border-border pt-6"
                    />
                  )
                )}

                {portadaVacia && (
                  <div className="mt-6 flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-12 text-center">
                    <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                    <p className="text-sm text-muted-foreground">
                      No encontramos ofertas ni marcas propias disponibles ahora.
                    </p>
                    <button
                      type="button"
                      className="text-sm underline"
                      onClick={() => abrirCatalogoCompleto()}
                    >
                      Explorar todo el catálogo
                    </button>
                  </div>
                )}

                <section className="mt-8 overflow-hidden rounded-2xl border-2 border-border bg-primary p-6 text-primary-foreground shadow-[6px_6px_0px_var(--color-border)] md:p-7">
                  <div className="flex flex-col justify-between gap-4 md:flex-row md:items-center">
                    <div className="flex flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <Sparkles className="h-5 w-5" aria-hidden="true" />
                        <h2 className="font-['Fraunces'] text-xl font-bold md:text-2xl">
                          ¿Armamos tu lista de la semana?
                        </h2>
                      </div>
                      <p className="text-sm text-primary-foreground/95">
                        Pídele recetas o un presupuesto al Asistente IA usando precios reales del
                        catálogo.
                      </p>
                    </div>
                    <div className="flex flex-col items-start gap-3 md:items-end">
                      <Link
                        to="/chat"
                        className="inline-flex shrink-0 items-center justify-center gap-2 rounded-full border-2 border-border bg-card px-5 py-2.5 text-sm font-bold text-foreground shadow-[2px_2px_0px_var(--color-border)] transition-all hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        Ir al Asistente IA
                        <ArrowRight className="h-4 w-4" aria-hidden="true" />
                      </Link>
                      <button
                        type="button"
                        onClick={() => abrirCatalogoCompleto()}
                        className="rounded-md px-1 text-xs font-semibold text-primary-foreground/95 underline underline-offset-4 hover:text-primary-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        Ver todo el catálogo
                      </button>
                    </div>
                  </div>
                </section>
              </>
            ) : catalogo.isLoading ? (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 md:gap-4">
                {Array.from({ length: 12 }).map((_, indice) => (
                  <ProductSkeleton key={indice} />
                ))}
              </div>
             ) : catalogo.notice ? (
               <p role="status" className="py-12 text-center text-sm text-muted-foreground">{catalogo.notice}</p>
             ) : catalogo.error ? (
              <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
                <Store className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
                 <p className="text-sm text-muted-foreground">{catalogo.error}</p>
                 <button type="button" className="text-sm underline" onClick={catalogo.retry}>Reintentar</button>
              </div>
            ) : catalogo.filtered.length > 0 ? (
              <div className="flex flex-col pb-6">
                <div className="mb-4 flex flex-wrap items-center justify-between gap-2 px-1">
                  <h2 className="text-sm font-bold text-foreground">
                    {hayFiltroActivo ? 'Resultados encontrados' : 'Explorar por menor precio'}
                  </h2>
                  <p className="text-xs text-muted-foreground">
                    {catalogo.filtered.length} productos cargados · {ordenadoPor}
                  </p>
                </div>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 md:gap-4">
                  {catalogo.filtered.map((producto) => (
                    <ProductCard key={producto.id} product={producto} onAdd={manejarAgregar} />
                  ))}
                </div>
                 {catalogo.isFetchingMore && (
                  <div className="flex justify-center items-center py-6">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                    <span className="ml-2 text-sm font-medium text-muted-foreground">Cargando más productos...</span>
                  </div>
                 )}
                 {catalogo.loadMoreError && (
                   <div role="alert" className="py-4 text-center text-sm">
                     <p>{catalogo.loadMoreError}</p>
                     <button type="button" className="underline" onClick={catalogo.fetchMore}>Reintentar</button>
                   </div>
                 )}
              </div>
            ) : (
              <div className="flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
                <Store
                  className="h-8 w-8 text-muted-foreground"
                  aria-hidden="true"
                />
                <p className="text-sm text-muted-foreground">
                  {consultaActiva ? (
                    <>
                      No encontramos productos para{' '}
                      <span className="font-medium text-foreground">&ldquo;{consultaActiva}&rdquo;</span>
                    </>
                  ) : (
                    'No encontramos productos con los filtros actuales.'
                  )}
                </p>
              </div>
            )}
            <Footer />
          </div>
        </section>

      </main>

    </div>
  )
}
