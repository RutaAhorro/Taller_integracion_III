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
        className="space-y-2 rounded-xl border border-border bg-secondary/40 p-4 text-sm"
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
          Recomendación entre {datos.candidatas_evaluadas} sucursales evaluadas,
          no entre todas las tiendas existentes. Los precios y el stock pueden
          cambiar; recalcula si modificas la lista.
        </p>
      </section>
      {datos.no_cubiertos.length > 0 && (
        <section
          role="alert"
          className="space-y-2 rounded-xl border border-border bg-secondary/40 p-4"
        >
          <h2 className="font-semibold">
            Compra parcial: artículos no cubiertos
          </h2>
          <p>
            Estos productos no tienen una oferta disponible en el radio de
            búsqueda y no están incluidos en el total:
          </p>
          <ul className="list-inside list-disc space-y-1 break-words">
            {datos.no_cubiertos.map((articulo) => (
              <li key={articulo.producto_id}>
                {nombres[articulo.producto_id] ??
                  `Producto ${articulo.producto_id}`}
              </li>
            ))}
          </ul>
        </section>
      )}
      <section
        aria-label="Resumen de la ruta"
        className="grid gap-3 sm:grid-cols-3"
      >
        <div className="rounded-xl border border-border bg-card p-4">
          <h2 className="text-sm text-muted-foreground">
            Total de productos incluidos
          </h2>
          <p className="text-2xl font-bold">{formatPrice(ruta.total)}</p>
        </div>
        <div className="rounded-xl border border-border bg-card p-4">
          <h2 className="text-sm text-muted-foreground">
            Ahorro frente a una sola tienda
          </h2>
          <p className="font-bold">
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
        <div className="rounded-xl border border-border bg-card p-4">
          <h2 className="text-sm text-muted-foreground">Distancia estimada</h2>
          <p className="text-2xl font-bold">
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
        <p className="font-medium">Conviene comprar en una sola tienda.</p>
      )}
      <section aria-label="Paradas recomendadas" className="space-y-3">
        <h2 className="text-lg font-semibold">Orden de visita recomendado</h2>
        <ol className="space-y-4">
          {ruta.paradas.map((parada) => {
            const enlace = enlaceGoogleMaps(parada.lat, parada.lon);
            return (
              <li
                key={parada.sucursal_id}
                className="space-y-3 rounded-xl border border-border bg-card p-4"
              >
                <h3 className="font-semibold">
                  {parada.orden}. {parada.cadena} — {parada.nombre}
                </h3>
                <p>
                  Subtotal: <strong>{formatPrice(parada.subtotal)}</strong>
                </p>
                <ul className="divide-y divide-border">
                  {parada.articulos.map((articulo) => (
                    <li
                      key={articulo.producto_id}
                      className="flex flex-col justify-between gap-1 py-3 sm:flex-row sm:gap-4"
                    >
                      <div className="min-w-0">
                        <p className="break-words font-medium">
                          {articulo.nombre ||
                            nombres[articulo.producto_id] ||
                            `Producto ${articulo.producto_id}`}
                        </p>
                        <p className="text-sm text-muted-foreground">
                          {articulo.cantidad} envases ×{" "}
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
                    className="inline-block text-sm font-medium underline"
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
