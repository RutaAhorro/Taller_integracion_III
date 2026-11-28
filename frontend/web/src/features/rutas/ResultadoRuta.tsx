// Presenta la compra por tienda sin recalcular la recomendación ni inventar cobertura o recorridos.
import {
  ArrowUpRight,
  Check,
  ChevronDown,
  Info,
  MapPin,
  Package,
  Route as IconoRuta,
  ShoppingBag,
  Store,
  TrendingDown,
} from "lucide-react";
import { formatPrice } from "@/features/catalogo/formatters";
import {
  agruparArticulosParada,
  type ArticuloAgrupadoRuta,
} from "./presentacion-ruta";
import type {
  ArticuloNoCubierto,
  MotivoNoCubierto,
  ParadaRuta,
  RespuestaOptimizacion,
} from "./rutas-api";

// Una compra larga se despliega a petición del usuario; la ruta nunca oculta cuántas líneas quedan.
const MAXIMO_PRODUCTOS_VISIBLES = 4;

/** Construye una ubicación pública sin claves y omite coordenadas inválidas. */
export function enlaceGoogleMaps(
  latitud: number,
  longitud: number,
): string | null {
  if (
    !Number.isFinite(latitud) ||
    !Number.isFinite(longitud) ||
    Math.abs(latitud) > 90 ||
    Math.abs(longitud) > 180
  )
    return null;
  const parametros = new URLSearchParams({
    api: "1",
    query: `${latitud},${longitud}`,
  });
  return `https://www.google.com/maps/search/?${parametros}`;
}

/** Texto breve por motivo; no afirma causas que el motor no informó. */
const DETALLE_MOTIVO_NO_CUBIERTO: Record<MotivoNoCubierto, string> = {
  sin_oferta_en_radio:
    "Sin oferta vigente disponible en las sucursales evaluadas.",
  fuera_del_maximo_de_paradas:
    "Tiene oferta, pero comprarlo exigiría visitar más tiendas que el máximo de paradas.",
};

/** Usa los nombres que entrega el motor cuando la lista aún no los resolvió. */
function nombresNoCubiertos(
  noCubiertos: ArticuloNoCubierto[],
): Record<string, string> {
  return Object.fromEntries(
    noCubiertos
      .filter((articulo) => articulo.nombre)
      .map((articulo) => [articulo.producto_id, articulo.nombre as string]),
  );
}

/** Comparte las exclusiones entre errores y resultados parciales sin convertirlas en una compra. */
export function ArticulosSinCobertura({
  identificadores,
  nombres,
  motivos = {},
}: {
  identificadores: string[];
  nombres: Record<string, string>;
  /** Motivo informado por el Motor; sin él se asume falta de oferta. */
  motivos?: Record<string, MotivoNoCubierto>;
}) {
  return (
    <ul className="space-y-2">
      {identificadores.map((identificador) => (
        <li
          key={identificador}
          className="flex items-start gap-3 rounded-xl border border-border bg-background/70 px-3 py-3 text-sm"
        >
          <Package
            className="mt-0.5 size-4 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <p className="break-words font-medium">
              {nombres[identificador] ?? `Producto ${identificador}`}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {
                DETALLE_MOTIVO_NO_CUBIERTO[
                  motivos[identificador] ?? "sin_oferta_en_radio"
                ]
              }
            </p>
          </div>
        </li>
      ))}
    </ul>
  );
}

/** Destaca cantidad y subtotal una sola vez por oferta, incluso si proviene de varias líneas de lista. */
function FilaCompra({
  articulo,
  nombres,
}: {
  articulo: ArticuloAgrupadoRuta;
  nombres: Record<string, string>;
}) {
  const nombre =
    articulo.nombre ||
    articulo.producto_ids
      .map((productoID) => nombres[productoID])
      .find(Boolean) ||
    `Producto ${articulo.producto_id}`;
  return (
    <li className="flex items-start gap-3 py-4 sm:items-center sm:gap-4">
      <div className="grid size-10 shrink-0 place-items-center rounded-xl border border-primary/15 bg-primary/5 text-sm font-bold tabular-nums text-primary">
        <span aria-label={`Cantidad: ${articulo.cantidad}`}>
          ×{articulo.cantidad}
        </span>
      </div>
      <div className="min-w-0 flex-1">
        <p className="break-words text-sm font-semibold leading-relaxed">
          {nombre}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          {articulo.cantidad} {articulo.cantidad === 1 ? "envase" : "envases"} ×{" "}
          {formatPrice(articulo.precio_unitario)}
        </p>
      </div>
      <p className="shrink-0 pt-1 text-right text-sm font-bold tabular-nums sm:pt-0 sm:text-base">
        <span className="sr-only">Subtotal del producto: </span>
        {formatPrice(articulo.subtotal)}
      </p>
    </li>
  );
}

/** Una parada funciona como lista de compra compacta, con el excedente accesible sin alargar toda la página. */
function TarjetaParada({
  parada,
  nombres,
  conMapa = false,
}: {
  parada: ParadaRuta;
  nombres: Record<string, string>;
  conMapa?: boolean;
}) {
  const articulos = agruparArticulosParada(parada.articulos);
  const cantidadEnvases = articulos.reduce(
    (cantidad, articulo) => cantidad + articulo.cantidad,
    0,
  );
  const visibles = articulos.slice(0, MAXIMO_PRODUCTOS_VISIBLES);
  const adicionales = articulos.slice(MAXIMO_PRODUCTOS_VISIBLES);
  const enlace = enlaceGoogleMaps(parada.lat, parada.lon);
  return (
    <li
      id={`parada-${parada.sucursal_id}`}
      aria-label={`Parada ${parada.orden}: ${parada.cadena}`}
      className="relative min-w-0 scroll-mt-4"
    >
      <span
        aria-hidden="true"
        className="absolute -left-12 top-7 hidden size-9 items-center justify-center rounded-full border-4 border-background bg-primary text-xs font-bold text-primary-foreground sm:flex"
      >
        {parada.orden}
      </span>
      <article className="overflow-hidden rounded-2xl border border-border bg-card shadow-[0_6px_24px_-16px_var(--color-foreground)]">
        <header className="flex flex-wrap items-center justify-between gap-4 border-b border-border bg-secondary/35 p-4 sm:px-6 sm:py-5">
          <div className="flex min-w-0 items-center gap-3">
            <span className="grid size-11 shrink-0 place-items-center rounded-xl border border-border bg-card text-primary">
              <Store className="size-5" aria-hidden="true" />
            </span>
            <div className="min-w-0">
              <p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">
                Parada {parada.orden}
              </p>
              <h3 className="mt-0.5 break-words font-display text-lg font-bold sm:text-xl">
                {parada.cadena}
              </h3>
              <p className="mt-1 break-words text-xs text-muted-foreground">
                {parada.nombre}
              </p>
            </div>
          </div>
          <div className="ml-auto shrink-0 text-right">
            <p className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
              Subtotal de la parada
            </p>
            <p className="mt-1 text-xl font-bold tabular-nums tracking-tight">
              {formatPrice(parada.subtotal)}
            </p>
          </div>
        </header>
        <div className="px-4 sm:px-6">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/70 py-3 text-xs text-muted-foreground">
            <p className="inline-flex items-center gap-1.5 font-medium">
              <ShoppingBag className="size-3.5" aria-hidden="true" />
              Qué comprar aquí
            </p>
            <p>
              {articulos.length}{" "}
              {articulos.length === 1 ? "producto" : "productos"} ·{" "}
              {cantidadEnvases} {cantidadEnvases === 1 ? "envase" : "envases"}
            </p>
          </div>
          <ul
            aria-label={`Compra en ${parada.nombre}`}
            className="divide-y divide-border/70"
          >
            {visibles.map((articulo) => (
              <FilaCompra
                key={articulo.clave}
                articulo={articulo}
                nombres={nombres}
              />
            ))}
          </ul>
          {adicionales.length > 0 && (
            <details className="group border-t border-border/70">
              <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 rounded-lg py-3 text-sm font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
                Ver {adicionales.length}{" "}
                {adicionales.length === 1 ? "producto más" : "productos más"}
                <ChevronDown
                  className="size-4 shrink-0 transition-transform group-open:rotate-180"
                  aria-hidden="true"
                />
              </summary>
              <ul
                aria-label={`Más productos en ${parada.nombre}`}
                className="divide-y divide-border/70"
              >
                {adicionales.map((articulo) => (
                  <FilaCompra
                    key={articulo.clave}
                    articulo={articulo}
                    nombres={nombres}
                  />
                ))}
              </ul>
            </details>
          )}
        </div>
        <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-border bg-secondary/15 px-4 py-3 sm:px-6">
          <p className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <MapPin className="size-3.5 shrink-0" aria-hidden="true" />
            Ubicación de la sucursal
          </p>
          {enlace ? (
            <a
              href={enlace}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-primary/25 bg-primary/5 px-4 py-2 text-xs font-bold text-primary transition-colors hover:bg-primary/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              aria-label={`Abrir ${parada.nombre} en Google Maps (nueva pestaña)`}
            >
              <ArrowUpRight className="size-4" aria-hidden="true" />
              Abrir en Google Maps
            </a>
          ) : (
            <p className="text-xs text-muted-foreground">
              Ubicación no disponible.
            </p>
          )}
          {conMapa && (
            <a
              href="#mapa-ruta"
              className="inline-flex min-h-10 items-center gap-2 px-4 py-2 text-xs font-bold text-primary underline"
            >
              Ver parada {parada.orden} en el mapa
            </a>
          )}
        </footer>
      </article>
    </li>
  );
}

/** Resume la compra y enlaza al mapa solo cuando Motor confirma geometría vial. */
export function ResultadoRuta({
  datos,
  nombres = {},
}: {
  datos: RespuestaOptimizacion;
  nombres?: Record<string, string>;
}) {
  const { ruta, base_una_tienda: base, ahorro } = datos;
  const vial = datos.fuente_recorrido === "ors_osm";
  const cantidadEnvases = ruta.paradas.reduce(
    (cantidad, parada) =>
      cantidad +
      parada.articulos.reduce(
        (envases, articulo) => envases + articulo.cantidad,
        0,
      ),
    0,
  );
  const origen = datos.origen.por_defecto
    ? "Centro de Temuco"
    : `${datos.origen.lat}, ${datos.origen.lon}`;
  return (
    <div className="space-y-7">
      {vial && (
        <a
          href="#mapa-ruta"
          className="inline-flex min-h-11 items-center gap-2 rounded-xl border border-primary bg-primary px-5 py-3 font-semibold text-primary-foreground"
          onClick={(evento) => {
            const mapa = document.getElementById("mapa-ruta");
            if (!mapa) return;
            evento.preventDefault();
            mapa.scrollIntoView({ behavior: "smooth", block: "start" });
            mapa.focus({ preventScroll: true });
          }}
        >
          <MapPin className="size-4" aria-hidden="true" /> Ver recorrido en el
          mapa
        </a>
      )}
      {datos.no_cubiertos.length > 0 && (
        <section
          role="alert"
          className="space-y-3 rounded-2xl border border-border border-l-4 border-l-primary bg-secondary/40 p-4 sm:p-5"
        >
          <h2 className="flex items-center gap-2 font-semibold">
            <Info className="size-4 shrink-0 text-primary" aria-hidden="true" />
            Compra parcial: artículos no cubiertos
          </h2>
          <p className="text-sm leading-relaxed text-muted-foreground">
            Estos productos no están incluidos en el total. Cada uno indica el
            motivo informado por el motor; cuando falta una oferta, puede
            deberse a falta de stock, precios vencidos o ausencia de ofertas
            cercanas, sin que se confirme una causa individual.
          </p>
          <ArticulosSinCobertura
            identificadores={datos.no_cubiertos.map(
              (articulo) => articulo.producto_id,
            )}
            nombres={{ ...nombresNoCubiertos(datos.no_cubiertos), ...nombres }}
            motivos={Object.fromEntries(
              datos.no_cubiertos.map((articulo) => [
                articulo.producto_id,
                articulo.motivo,
              ]),
            )}
          />
        </section>
      )}

      <section
        aria-label="Resumen de la ruta"
        className="grid gap-3 md:grid-cols-3"
      >
        <div className="relative isolate overflow-hidden rounded-2xl border border-primary bg-primary p-5 text-primary-foreground shadow-[0_10px_28px_-18px_var(--color-primary)] sm:p-6">
          <ShoppingBag
            aria-hidden="true"
            className="pointer-events-none absolute -bottom-4 -right-3 -z-10 size-32 -rotate-12 opacity-[0.07]"
          />
          <p className="mb-5 inline-flex items-center gap-1.5 rounded-full border border-primary-foreground/25 bg-primary-foreground/10 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider">
            <Check className="size-3" aria-hidden="true" /> Tu compra organizada
          </p>
          <h2 className="text-sm font-medium">Total de productos incluidos</h2>
          <p className="mt-1 text-4xl font-bold tabular-nums tracking-tight">
            {formatPrice(ruta.total)}
          </p>
          <p className="mt-3 text-xs">
            {cantidadEnvases} {cantidadEnvases === 1 ? "envase" : "envases"} ·
            Solo productos cubiertos · sin traslado
          </p>
          {/* Motor suma solo artículos pendientes y cubiertos con el precio de la tienda asignada;
              la lista suma todos sus artículos con el precio guardado, por eso pueden diferir. */}
          <p className="mt-2 text-xs leading-relaxed text-primary-foreground/80">
            Puede diferir del total de tu lista: aquí solo cuentan los artículos
            pendientes que entran en la ruta, con el precio de la tienda donde
            se compran.
          </p>
        </div>
        <div className="flex flex-col rounded-2xl border border-border bg-card p-5 sm:p-6">
          <span className="mb-4 grid size-9 place-items-center rounded-xl bg-primary/10 text-primary">
            <TrendingDown className="size-4" aria-hidden="true" />
          </span>
          <h2 className="text-sm text-muted-foreground">
            Ahorro frente a una sola tienda
          </h2>
          <p className="mt-2 text-xl font-bold tabular-nums tracking-tight sm:text-2xl">
            {base !== null && ahorro !== null
              ? formatPrice(ahorro)
              : "Sin tienda única para comparar"}
          </p>
          <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
            {base !== null && ahorro !== null
              ? `Comparado con ${base.cadena || base.nombre}: ${formatPrice(base.total)} para los mismos artículos cubiertos.`
              : "Estos artículos no están todos en una misma tienda cercana, así que no hay una compra en un solo lugar con la cual comparar."}
          </p>
        </div>
        <div className="flex flex-col rounded-2xl border border-border bg-card p-5 sm:p-6">
          <span className="mb-4 grid size-9 place-items-center rounded-xl bg-secondary text-primary">
            <IconoRuta className="size-4" aria-hidden="true" />
          </span>
          <h2 className="text-sm text-muted-foreground">Distancia estimada</h2>
          <p className="mt-2 text-3xl font-bold tabular-nums tracking-tight">
            {ruta.distancia_km.toLocaleString("es-CL", {
              maximumFractionDigits: 2,
            })}{" "}
            <span className="text-base font-medium text-muted-foreground">
              km
            </span>
          </p>
          <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
            {ruta.paradas.length}{" "}
            {ruta.paradas.length === 1 ? "parada" : "paradas"} · Incluye el
            retorno al origen
            {vial && ruta.duracion_segundos !== undefined && (
              <>
                {" "}
                · Aproximadamente {Math.ceil(ruta.duracion_segundos / 60)} min
                de viaje, sin tiempo de compra
              </>
            )}
          </p>
        </div>
      </section>

      <section
        aria-label="Alcance de la recomendación"
        className="space-y-3 rounded-xl bg-secondary/40 px-4 py-3 text-xs leading-relaxed text-muted-foreground"
      >
        <p className="flex items-start gap-2">
          <Info
            className="mt-0.5 size-4 shrink-0 text-primary"
            aria-hidden="true"
          />
          <span>
            {vial
              ? "Recorrido por calles según OpenStreetMap y el medio elegido; tiempos estimados, sin tráfico en vivo. "
              : "Distancias estimadas; no representan un recorrido por calles. "}
            El total no incluye combustible ni otros gastos de traslado.
          </span>
        </p>
        <details className="group border-t border-border/70 pt-2">
          <summary className="flex min-h-8 cursor-pointer list-none items-center justify-between gap-2 rounded-md font-semibold text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
            Cómo se calcula esta recomendación
            <ChevronDown
              className="size-3.5 transition-transform group-open:rotate-180"
              aria-hidden="true"
            />
          </summary>
          <div className="space-y-2 pb-1 pt-2">
            <p>
              Origen:{" "}
              {datos.origen.por_defecto
                ? "centro de Temuco (por defecto)."
                : `${origen}.`}{" "}
              La distancia incluye el retorno al origen.
            </p>
            <p>
              Radio de búsqueda:{" "}
              {datos.parametros.radio_km.toLocaleString("es-CL")} km · Hasta{" "}
              {datos.parametros.max_paradas} paradas.
            </p>
            <p>
              Recomendación entre {datos.candidatas_evaluadas} sucursales
              evaluadas, no entre todas las tiendas existentes. Los precios y el
              stock pueden cambiar; recalcula si modificas la lista.
            </p>
          </div>
        </details>
      </section>
      {ruta.paradas.length === 1 && ahorro === 0 && (
        <p className="flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/5 px-4 py-3 text-sm font-medium">
          <Check className="size-4 shrink-0 text-primary" aria-hidden="true" />
          Conviene comprar en una sola tienda.
        </p>
      )}

      <section aria-label="Paradas recomendadas" className="space-y-5">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="mb-1 text-[10px] font-bold uppercase tracking-[0.16em] text-primary">
              Tu recorrido de compra
            </p>
            <h2 className="font-display text-xl font-bold sm:text-2xl">
              Orden de visita recomendado
            </h2>
          </div>
          <span className="rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-muted-foreground">
            {ruta.paradas.length}{" "}
            {ruta.paradas.length === 1 ? "tienda" : "tiendas"} por visitar
          </span>
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          Google Maps abre la ubicación de la tienda.
          {vial
            ? " Para ver el recorrido calculado, usa el mapa integrado."
            : " No es un recorrido calculado por calles."}
        </p>

        <div className="flex items-start gap-2.5 text-xs text-muted-foreground sm:ml-0.5">
          <span className="grid size-8 shrink-0 place-items-center rounded-full border border-border bg-secondary">
            <MapPin className="size-3.5 text-primary" aria-hidden="true" />
          </span>
          <div className="pt-0.5">
            <p className="font-semibold text-foreground">Punto de partida</p>
            <p className="mt-0.5">{origen}</p>
          </div>
        </div>
        <ol className="space-y-5 sm:ml-4 sm:border-l-2 sm:border-dashed sm:border-primary/25 sm:pl-9">
          {ruta.paradas.map((parada) => (
            <TarjetaParada
              key={parada.sucursal_id}
              parada={parada}
              nombres={nombres}
              conMapa={vial}
            />
          ))}
        </ol>
        <div className="flex items-center gap-2.5 text-xs text-muted-foreground">
          <span className="grid size-8 shrink-0 place-items-center rounded-full border border-primary/20 bg-primary/5">
            <Check className="size-3.5 text-primary" aria-hidden="true" />
          </span>
          <div>
            <p className="font-semibold text-foreground">Regreso al origen</p>
            <p className="mt-0.5">
              La compra termina con el retorno a tu punto de partida.
            </p>
          </div>
        </div>
      </section>
    </div>
  );
}
