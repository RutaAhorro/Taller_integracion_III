// Presenta exclusivamente la recomendación de Motor, sin volver a optimizar ni inventar cobertura.
import { formatPrice } from "@/features/catalogo/formatters";
import type { RespuestaOptimizacion } from "./rutas-api";

/** Construye una ubicación pública sin claves; rechaza coordenadas inválidas en vez de inventarlas. */
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

/** Comparte las exclusiones entre errores y resultados parciales sin convertirlas en una compra. */
export function ArticulosSinCobertura({
  identificadores,
  nombres,
}: {
  identificadores: string[];
  nombres: Record<string, string>;
}) {
  return (
    <ul className="space-y-2">
      {identificadores.map((identificador) => (
        <li
          key={identificador}
          className="rounded-xl border border-border bg-background/70 px-3 py-2 text-sm"
        >
          <span className="break-words font-medium">
            {nombres[identificador] ?? `Producto ${identificador}`}
          </span>
          <span className="mt-1 block text-xs text-muted-foreground">
            Sin oferta vigente disponible en las sucursales evaluadas.
          </span>
        </li>
      ))}
    </ul>
  );
}

/** Muestra la compra cubierta y advierte de exclusiones antes de presentar el total. */
export function ResultadoRuta({
  datos,
  nombres = {},
}: {
  datos: RespuestaOptimizacion;
  nombres?: Record<string, string>;
}) {
  const { ruta, base_una_tienda: base, ahorro } = datos;
  return (
    <div className="space-y-6">
      <section
        aria-label="Alcance de la recomendación"
        className="space-y-2 rounded-2xl border-2 border-border bg-secondary/40 p-4 text-sm sm:p-5"
      >
        <p>
          Distancias estimadas; no representan un recorrido por calles. El total
          no incluye combustible ni otros gastos de traslado.
        </p>
        <p>
          {datos.origen.por_defecto
            ? "Origen: centro de Temuco (por defecto)."
            : `Origen: ${datos.origen.lat}, ${datos.origen.lon}.`}{" "}
          La distancia incluye el retorno al origen.
        </p>
        <p>
          Radio de búsqueda: {datos.parametros.radio_km.toLocaleString("es-CL")}{" "}
          km · Hasta {datos.parametros.max_paradas} paradas.
        </p>
        <p>
          Recomendación entre {datos.candidatas_evaluadas} sucursales evaluadas,
          no entre todas las tiendas existentes. Los precios y el stock pueden
          cambiar; recalcula si modificas la lista.
        </p>
      </section>
      {datos.no_cubiertos.length > 0 && (
        <section
          role="alert"
          className="space-y-3 rounded-2xl border-2 border-border bg-secondary/40 p-4 sm:p-5"
        >
          <h2 className="font-semibold">
            Compra parcial: artículos no cubiertos
          </h2>
          <p>
            Estos productos no tienen una oferta vigente disponible en el radio
            de búsqueda y no están incluidos en el total. Puede deberse a falta
            de stock, precios vencidos o ausencia de ofertas cercanas; no se
            confirmó una causa individual.
          </p>
          <ArticulosSinCobertura
            identificadores={datos.no_cubiertos.map(
              (articulo) => articulo.producto_id,
            )}
            nombres={nombres}
          />
        </section>
      )}
      <section
        aria-label="Resumen de la ruta"
        className="grid gap-4 lg:grid-cols-3"
      >
        <div className="rounded-2xl border-2 border-border bg-primary p-5 text-primary-foreground shadow-[4px_4px_0px_var(--color-border)]">
          <h2 className="text-sm font-medium">Total de productos incluidos</h2>
          <p className="mt-2 text-3xl font-bold tracking-tight">
            {formatPrice(ruta.total)}
          </p>
          <p className="mt-2 text-xs">
            Solo productos cubiertos · sin traslado
          </p>
        </div>
        <div className="rounded-2xl border-2 border-border bg-card p-5">
          <h2 className="text-sm text-muted-foreground">
            Ahorro frente a una sola tienda
          </h2>
          <p className="my-2 text-xl font-bold">
            {base !== null && ahorro !== null
              ? formatPrice(ahorro)
              : "Sin comparación disponible"}
          </p>
          <p className="text-sm text-muted-foreground">
            {base !== null && ahorro !== null
              ? `Comparado con ${base.cadena || base.nombre}: ${formatPrice(base.total)} para los mismos artículos cubiertos.`
              : "Ninguna tienda sola cubre los mismos artículos."}
          </p>
        </div>
        <div className="rounded-2xl border-2 border-border bg-card p-5">
          <h2 className="text-sm text-muted-foreground">Distancia estimada</h2>
          <p className="my-2 text-2xl font-bold">
            {ruta.distancia_km.toLocaleString("es-CL", {
              maximumFractionDigits: 2,
            })}{" "}
            km
          </p>
          <p className="text-sm text-muted-foreground">
            {ruta.paradas.length}{" "}
            {ruta.paradas.length === 1 ? "parada" : "paradas"}
          </p>
        </div>
      </section>
      {ruta.paradas.length === 1 && ahorro === 0 && (
        <p className="rounded-xl border border-border bg-primary/10 p-4 font-medium">
          Conviene comprar en una sola tienda.
        </p>
      )}
      <section aria-label="Paradas recomendadas" className="space-y-3">
        <h2 className="text-xl font-bold">Orden de visita recomendado</h2>
        <p className="text-sm text-muted-foreground">
          Revisa qué comprar en cada parada. Google Maps abre la ubicación de la
          tienda, no un recorrido calculado por calles.
        </p>
        <ol className="space-y-4">
          {ruta.paradas.map((parada) => {
            const enlace = enlaceGoogleMaps(parada.lat, parada.lon);
            return (
              <li
                key={parada.sucursal_id}
                className="space-y-4 rounded-2xl border-2 border-border bg-card p-4 shadow-[3px_3px_0px_var(--color-border)] sm:p-6"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="mb-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
                      Parada {parada.orden}
                    </p>
                    <h3 className="break-words text-lg font-bold">
                      {parada.orden}. {parada.cadena}
                    </h3>
                    <p className="break-words text-sm text-muted-foreground">
                      {parada.nombre}
                    </p>
                  </div>
                  <p className="rounded-xl border border-border bg-secondary/40 px-3 py-2 text-sm">
                    Subtotal:{" "}
                    <strong className="block text-lg">
                      {formatPrice(parada.subtotal)}
                    </strong>
                  </p>
                </div>
                <ul className="divide-y divide-border">
                  {parada.articulos.map((articulo) => (
                    <li
                      key={articulo.producto_id}
                      className="flex flex-col justify-between gap-2 py-4 sm:flex-row sm:gap-4"
                    >
                      <div className="min-w-0">
                        <p className="break-words font-medium">
                          {articulo.nombre ||
                            nombres[articulo.producto_id] ||
                            `Producto ${articulo.producto_id}`}
                        </p>
                        <p className="text-sm text-muted-foreground">
                          {articulo.cantidad}{" "}
                          {articulo.cantidad === 1 ? "envase" : "envases"} ×{" "}
                          {formatPrice(articulo.precio_unitario)}
                        </p>
                      </div>
                      <p className="shrink-0 font-semibold">
                        {formatPrice(articulo.subtotal)}
                      </p>
                    </li>
                  ))}
                </ul>
                {enlace ? (
                  <a
                    href={enlace}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex min-h-11 items-center rounded-xl border-2 border-border bg-background px-4 py-2 text-sm font-bold transition-colors hover:bg-secondary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                    aria-label={`Abrir ${parada.nombre} en Google Maps (nueva pestaña)`}
                  >
                    Abrir en Google Maps
                  </a>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Ubicación no disponible.
                  </p>
                )}
              </li>
            );
          })}
        </ol>
      </section>
    </div>
  );
}
