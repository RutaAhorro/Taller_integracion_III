// Presenta artículos reales y formularios de búsqueda/edición; ninguna acción simula persistencia.
import { useState, type FormEvent } from "react";
import { Check, Package, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatPrice, formatearUnidad } from "@/features/catalogo/formatters";
import type { UiProduct } from "@/features/catalogo/tipos";
import {
  MAXIMO_ARTICULOS_LISTA,
  MAXIMO_ENVASES_ARTICULO,
  type ArticuloDetalle,
  type EstadoProducto,
} from "@/features/listas/listas-api";
import { esCantidadValida } from "@/features/listas/useDetalleLista";
import {
  useBusquedaArticulos,
} from "@/features/listas/useBusquedaArticulos";

// ===== Presentación de disponibilidad =====

const AVISOS_PRODUCTO: Record<EstadoProducto, string> = {
  disponible:
    "Stock confirmado al consultar. Confirma precio y disponibilidad antes de comprar.",
  sin_stock:
    "Sin stock: puedes conservarlo en la lista, pero hoy no hay disponibilidad confirmada.",
  desconocido:
    "Disponibilidad desconocida: no se pudo confirmar stock o precio vigente.",
  no_encontrado:
    "Producto no encontrado en Catálogo. Puedes quitarlo de la lista.",
};

/** Calcula el subtotal sin atribuir precios a referencias sin precio vigente confirmado. */
function calcularTotalArticulos(articulos: ArticuloDetalle[]) {
  let totalPesos = 0;
  let conPrecio = 0;
  let sinPrecio = 0;

  for (const articulo of articulos) {
    const precio = articulo.producto?.precio;
    if (typeof precio === "number" && Number.isFinite(precio)) {
      totalPesos += precio * articulo.cantidad;
      conPrecio += 1;
    } else {
      sinPrecio += 1;
    }
  }

  return { totalPesos, conPrecio, sinPrecio };
}

/** Contrato de los controles; los callbacks devuelven si el backend confirmó la operación. */
type PropiedadesArticulos = {
  articulos: ArticuloDetalle[];
  ocupado: boolean;
  alAgregar: (productoID: string, cantidad: number) => Promise<boolean>;
  alActualizar: (
    articuloID: number,
    cambios: { cantidad?: number; esta_comprado?: boolean },
  ) => Promise<boolean>;
  alEliminar: (articuloID: number) => Promise<boolean>;
};

/** Separa la selección temporal del buscador de los artículos ya guardados. */
export function ArticulosLista({
  articulos,
  ocupado,
  alAgregar,
  alActualizar,
  alEliminar,
}: PropiedadesArticulos) {
  const { totalPesos, conPrecio, sinPrecio } =
    calcularTotalArticulos(articulos);

  return (
    <div className="space-y-5">
      <h3 className="font-semibold">
        Artículos guardados ({articulos.length}/{MAXIMO_ARTICULOS_LISTA})
      </h3>
      <p className="text-sm text-muted-foreground">
        Las cantidades son envases: entre 1 y {MAXIMO_ENVASES_ARTICULO} por
        producto. Puedes guardar productos sin stock con una advertencia.
      </p>
      {articulos.length > 0 && (
        <div className="rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm">
          {sinPrecio === 0 ? (
            <p className="font-semibold">
              Total estimado: {formatPrice(totalPesos)}
            </p>
          ) : conPrecio > 0 ? (
            <>
              <p className="font-semibold">
                Subtotal con precio vigente: {formatPrice(totalPesos)}
              </p>
              <p className="text-muted-foreground">
                {sinPrecio} {sinPrecio === 1 ? "artículo no tiene" : "artículos no tienen"}{" "}
                precio vigente.
              </p>
            </>
          ) : (
            <p>
              Total no disponible: los artículos no tienen precio vigente
              confirmado.
            </p>
          )}
        </div>
      )}
      <BuscadorArticulos ocupado={ocupado} alAgregar={alAgregar} />
      {articulos.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          Esta lista aún no tiene artículos.
        </p>
      ) : (
        <>
          <ul className="space-y-3">
            {articulos.map((articulo) => (
              <li key={`${articulo.id}:${articulo.cantidad}`}>
                <ArticuloGuardado
                  articulo={articulo}
                  ocupado={ocupado}
                  alActualizar={alActualizar}
                  alEliminar={alEliminar}
                />
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

// ===== Búsqueda y alta =====

/** Busca en el servidor y conserva producto/cantidad seleccionados si el guardado falla. */
function BuscadorArticulos({
  ocupado,
  alAgregar,
}: Pick<PropiedadesArticulos, "ocupado" | "alAgregar">) {
  const busqueda = useBusquedaArticulos();
  const [seleccionado, setSeleccionado] = useState<UiProduct | null>(null);
  const [cantidad, setCantidad] = useState("1");
  const cantidadNumerica = Number(cantidad);
  const cantidadValida = esCantidadValida(cantidadNumerica);

  /** Limpia el formulario únicamente si Motor confirmó la persistencia. */
  async function agregar(evento: FormEvent) {
    evento.preventDefault();
    if (ocupado || !seleccionado || !cantidadValida) return;
    if (await alAgregar(seleccionado.id, cantidadNumerica)) {
      setSeleccionado(null);
      setCantidad("1");
      busqueda.limpiarBusqueda();
    }
  }

  return (
    <fieldset
      disabled={ocupado}
      className="min-w-0 space-y-3 rounded-xl border border-border p-3"
    >
      <legend className="px-1 text-sm font-semibold">
        Agregar producto del catálogo
      </legend>
      <form
        onSubmit={(evento) => {
          evento.preventDefault();
          setSeleccionado(null);
          busqueda.buscar();
        }}
        className="space-y-2"
      >
        <label htmlFor="buscar-articulo" className="block text-sm">
          Buscar producto
        </label>
        <div className="flex flex-wrap gap-2">
          <Input
            id="buscar-articulo"
            value={busqueda.consulta}
            onChange={(evento) => {
              busqueda.cambiarConsulta(evento.target.value);
              setSeleccionado(null);
            }}
            placeholder="Por ejemplo: arroz"
            className="min-w-0 flex-1"
          />
          <Button
            type="submit"
            disabled={ocupado || busqueda.busquedaCorta || busqueda.cargando}
          >
            Buscar
          </Button>
        </div>
        {busqueda.busquedaCorta && (
          <p className="text-xs text-muted-foreground">
            Escribe un poco más para buscar (por ejemplo, «arroz»).
          </p>
        )}
      </form>
      {busqueda.cargando && (
        <p role="status" className="text-sm">
          Buscando productos…
        </p>
      )}
      {busqueda.error && (
        <p role="alert" className="text-sm text-destructive">
          {busqueda.error} Vuelve a pulsar Buscar.
        </p>
      )}
      {busqueda.consultado &&
        !busqueda.cargando &&
        !busqueda.error &&
        busqueda.productos.length === 0 && (
          <p className="text-sm">
            No se encontraron productos para esta búsqueda.
          </p>
        )}
      <ul aria-label="Resultados de productos" className="space-y-2">
        {busqueda.productos.map((producto) => (
          <li key={producto.id}>
            <button
              type="button"
              disabled={ocupado}
              aria-pressed={seleccionado?.id === producto.id}
              onClick={() => setSeleccionado(producto)}
              className={`w-full rounded-lg border p-3 text-left text-sm disabled:opacity-60 ${seleccionado?.id === producto.id ? "border-primary bg-primary/10" : "border-border"}`}
            >
              <span className="block font-semibold">{producto.name}</span>
              <span className="block">
                {[producto.supermarketName, formatearUnidad(producto.unit), formatPrice(producto.price)].filter(Boolean).join(" · ")}
              </span>
              {producto.inStock !== true && (
                <span className="block text-muted-foreground">
                  Sin stock confirmado. Se guardará con su disponibilidad
                  actual.
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>
      {busqueda.totalPaginas > 1 && (
        <nav
          aria-label="Páginas de productos"
          className="flex flex-wrap items-center gap-2"
        >
          <Button
            type="button"
            variant="outline"
            disabled={ocupado || busqueda.cargando || busqueda.pagina <= 1}
            onClick={() => {
              setSeleccionado(null);
              busqueda.cambiarPagina(busqueda.pagina - 1);
            }}
          >
            Anterior
          </Button>
          <span className="text-xs">
            Página {busqueda.pagina} de {busqueda.totalPaginas}
          </span>
          <Button
            type="button"
            variant="outline"
            disabled={
              ocupado ||
              busqueda.cargando ||
              busqueda.pagina >= busqueda.totalPaginas
            }
            onClick={() => {
              setSeleccionado(null);
              busqueda.cambiarPagina(busqueda.pagina + 1);
            }}
          >
            Siguiente
          </Button>
        </nav>
      )}
      {seleccionado && (
        <form
          onSubmit={agregar}
          className="space-y-2 border-t border-border pt-3"
        >
          <p className="text-sm font-semibold">
            Seleccionado: {seleccionado.name}
          </p>
          <label htmlFor="cantidad-nuevo-articulo" className="block text-sm">
            Cantidad de envases
          </label>
          <Input
            id="cantidad-nuevo-articulo"
            type="number"
            min={1}
            max={MAXIMO_ENVASES_ARTICULO}
            step={1}
            required
            value={cantidad}
            onChange={(evento) => setCantidad(evento.target.value)}
            aria-invalid={!cantidadValida}
          />
          {!cantidadValida && (
            <p className="text-sm text-destructive">
              Ingresa un entero entre 1 y {MAXIMO_ENVASES_ARTICULO}.
            </p>
          )}
          <Button type="submit" disabled={ocupado || !cantidadValida}>
            {ocupado ? "Guardando…" : "Agregar a esta lista"}
          </Button>
          <p className="text-xs text-muted-foreground">
            Si el producto ya está guardado, se suman los envases.
          </p>
        </form>
      )}
    </fieldset>
  );
}

// ===== Edición y eliminación =====

/** Mantiene el borrador de cantidad ante errores y muestra referencias antiguas sin inventar productos. */
function ArticuloGuardado({
  articulo,
  ocupado,
  alActualizar,
  alEliminar,
}: {
  articulo: ArticuloDetalle;
} & Pick<PropiedadesArticulos, "ocupado" | "alActualizar" | "alEliminar">) {
  const [cantidad, setCantidad] = useState(String(articulo.cantidad));
  const cantidadNumerica = Number(cantidad);
  const cantidadValida = esCantidadValida(cantidadNumerica);
  const producto = articulo.producto;
  const nombre = producto?.nombre || "Producto sin información disponible";

  /** Envía el reemplazo de cantidad, no una suma, solo cuando el borrador es válido. */
  function guardar(evento: FormEvent) {
    evento.preventDefault();
    if (!ocupado && cantidadValida)
      void alActualizar(articulo.id, { cantidad: cantidadNumerica });
  }

  return (
    <fieldset
      disabled={ocupado}
      className="min-w-0 space-y-4 rounded-lg border border-border bg-card p-4 text-sm shadow-sm sm:p-5"
    >
      <legend className="sr-only">{nombre}</legend>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="relative grid size-20 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-background sm:size-24">
            <Package className="size-5" aria-hidden="true" />
            {producto?.url_imagen && (
              <img
                src={producto.url_imagen}
                alt={`Imagen de ${nombre}`}
                loading="lazy"
                onError={(evento) => {
                  evento.currentTarget.style.display = "none";
                }}
                className="absolute inset-0 size-full object-contain p-1"
              />
            )}
          </span>
          <div className="min-w-0 space-y-1">
            <h4 className="break-words font-semibold leading-snug">{nombre}</h4>
            {producto && (
              <p className="break-words text-xs text-muted-foreground sm:text-sm">
                {[producto.marca, producto.supermercado, formatearUnidad(producto.unidad)]
                  .filter(Boolean)
                  .join(" · ")}
              </p>
            )}
          </div>
        </div>
        <p className="max-w-full rounded-md bg-secondary/70 px-3 py-1.5 text-xs font-semibold text-foreground sm:text-sm">
          {producto?.precio === null || !producto
            ? "Precio vigente no disponible"
            : `${formatPrice(producto.precio)} / envase`}
        </p>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 border-y border-border py-3">
        <p
          className={`min-w-0 flex-1 text-xs sm:text-sm ${articulo.estado_producto === "disponible" ? "text-muted-foreground" : "font-medium text-foreground"}`}
        >
          {AVISOS_PRODUCTO[articulo.estado_producto] ??
            AVISOS_PRODUCTO.desconocido}
        </p>
        <span className="shrink-0 text-xs font-medium text-muted-foreground">
          {articulo.cantidad} {articulo.cantidad === 1 ? "envase" : "envases"}
        </span>
      </div>

      <div className="flex flex-wrap items-end justify-between gap-4">
        <form onSubmit={guardar} className="space-y-2">
          <label
            htmlFor={`cantidad-articulo-${articulo.id}`}
            className="block text-xs font-medium text-muted-foreground"
          >
            Cambiar cantidad
          </label>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              id={`cantidad-articulo-${articulo.id}`}
              type="number"
              min={1}
              max={MAXIMO_ENVASES_ARTICULO}
              step={1}
              value={cantidad}
              onChange={(evento) => setCantidad(evento.target.value)}
              required
              aria-invalid={!cantidadValida}
              className="w-24"
            />
            <Button
              type="submit"
              variant="outline"
              disabled={
                ocupado ||
                !cantidadValida ||
                cantidadNumerica === articulo.cantidad
              }
            >
              Guardar cantidad
            </Button>
          </div>
          {!cantidadValida && (
            <p className="text-destructive">
              Ingresa un entero entre 1 y {MAXIMO_ENVASES_ARTICULO}.
            </p>
          )}
        </form>

        <div className="flex flex-wrap items-center gap-3">
          <label className="inline-flex min-h-10 items-center gap-2 rounded-md border border-border px-3 font-medium">
            <input
              type="checkbox"
              checked={articulo.esta_comprado}
              onChange={(evento) => {
                void alActualizar(articulo.id, {
                  esta_comprado: evento.target.checked,
                });
              }}
            />
            <Check className="size-4 text-primary" aria-hidden="true" />
            Comprado
          </label>
          <Button
            type="button"
            variant="destructive"
            disabled={ocupado}
            onClick={() => {
              void alEliminar(articulo.id);
            }}
          >
            <Trash2 aria-hidden="true" />
            Quitar de la lista
          </Button>
        </div>
      </div>
    </fieldset>
  );
}
