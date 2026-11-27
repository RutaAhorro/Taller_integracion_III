// Integra la recomendación con la lista seleccionada y la sesión, sin mapa ni resultados simulados.
import { Link, useSearchParams } from "react-router-dom";
import { ROUTES } from "@/app/routes";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/features/autenticacion/useAuth";
import {
  ErrorSolicitudRutas,
  esListaIDValida,
  mensajeErrorRuta,
  orientacionErrorRuta,
  productosSinCobertura,
} from "./rutas-api";
import { ArticulosSinCobertura, ResultadoRuta } from "./ResultadoRuta";
import { useRutaLista, type EstadoRuta } from "./useRutaLista";

/** Remonta el cálculo por cuenta y lista para no mostrar una recomendación de la selección anterior. */
export default function RutaLista() {
  const [parametros] = useSearchParams();
  const listaID = parametros.get("lista")?.trim() || null;
  const { user } = useAuth();
  return (
    <ContenidoRuta
      key={`${user?.id ?? ""}:${listaID ?? ""}`}
      listaID={listaID}
    />
  );
}

/** Distingue errores recuperables de sesión o negocio con acciones explícitas. */
export function EstadoRecomendacion({
  estado,
  nombres,
  recalcular,
}: {
  estado: EstadoRuta;
  nombres: Record<string, string>;
  recalcular: () => void;
}) {
  if (estado.tipo === "sin_lista" || estado.tipo === "lista_invalida")
    return (
      <section
        role={estado.tipo === "lista_invalida" ? "alert" : undefined}
        className="space-y-3 rounded-2xl border-2 border-dashed border-border bg-card p-6 sm:p-8"
      >
        <h2 className="text-xl font-bold">
          {estado.tipo === "lista_invalida"
            ? "El enlace de la lista no es válido"
            : "Empieza con tu lista de compra"}
        </h2>
        <p className="text-sm text-muted-foreground">
          Selecciona una lista para calcular su ruta. No se ha enviado ninguna
          solicitud de cálculo.
        </p>
      </section>
    );
  if (estado.tipo === "cargando")
    return (
      <section
        role="status"
        aria-live="polite"
        className="space-y-4 rounded-2xl border-2 border-border bg-card p-6"
      >
        <p className="font-semibold">Calculando la ruta de tu lista…</p>
        <p className="text-sm text-muted-foreground">
          Estamos consultando precios y comparando las tiendas disponibles.
        </p>
        <div
          aria-hidden="true"
          className="grid animate-pulse gap-3 sm:grid-cols-3"
        >
          {[1, 2, 3].map((indice) => (
            <div key={indice} className="h-20 rounded-xl bg-secondary" />
          ))}
        </div>
      </section>
    );
  if (estado.tipo === "resultado")
    return <ResultadoRuta datos={estado.datos} nombres={nombres} />;

  const codigo =
    estado.error instanceof ErrorSolicitudRutas ? estado.error.estado : null;
  return (
    <section
      role="alert"
      className="space-y-4 rounded-2xl border-2 border-border bg-card p-5 shadow-[4px_4px_0px_var(--color-border)] sm:p-7"
    >
      <p className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
        Revisa tu compra
      </p>
      <h2 className="text-lg font-bold">{mensajeErrorRuta(estado.error)}</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        {orientacionErrorRuta(estado.error)}
      </p>
      {productosSinCobertura(estado.error).length > 0 && (
        <div className="space-y-3">
          <h3 className="font-semibold">Artículos sin cobertura en el radio</h3>
          <ArticulosSinCobertura
            identificadores={productosSinCobertura(estado.error)}
            nombres={nombres}
          />
        </div>
      )}
      {codigo === 401 ? (
        <Link to={ROUTES.LOGIN} className="underline">
          Ir a iniciar sesión
        </Link>
      ) : codigo === 400 ||
        codigo === 403 ||
        codigo === 404 ||
        codigo === 422 ? (
        <Link
          to={ROUTES.LISTAS}
          className="inline-block text-sm font-semibold underline underline-offset-4"
        >
          Revisar mis listas
        </Link>
      ) : (
        <Button variant="outline" onClick={recalcular}>
          Reintentar cálculo
        </Button>
      )}
    </section>
  );
}

/** El enlace de regreso conserva la selección; recalcular nunca modifica la lista. */
function ContenidoRuta({ listaID }: { listaID: string | null }) {
  const { estado, nombres, recalcular } = useRutaLista(listaID);
  const seleccionValida = esListaIDValida(listaID);
  return (
    <main className="mx-auto w-full max-w-5xl space-y-6 p-4 pb-24 md:p-8">
      <header className="space-y-4 rounded-2xl border-2 border-border bg-card p-5 shadow-[4px_4px_0px_var(--color-border)] sm:p-7">
        <span className="inline-block rounded-full border border-border bg-primary/10 px-3 py-1 text-xs font-bold">
          Tu lista, tienda por tienda
        </span>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
          Ruta recomendada
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Descubre dónde comprar los artículos pendientes y cómo se reparte tu
          compra. La recomendación usa los precios y sucursales disponibles al
          calcular.
        </p>
        <div className="flex flex-wrap items-center gap-4 border-t border-border pt-4">
          <Link
            className="font-medium underline"
            to={
              seleccionValida
                ? `${ROUTES.LISTAS}?lista=${encodeURIComponent(listaID)}`
                : ROUTES.LISTAS
            }
          >
            {seleccionValida ? "Volver a mi lista" : "Ir a mis listas"}
          </Link>
          {estado.tipo === "resultado" && (
            <Button variant="outline" onClick={recalcular}>
              Recalcular ruta
            </Button>
          )}
        </div>
      </header>
      <EstadoRecomendacion
        estado={estado}
        nombres={nombres}
        recalcular={recalcular}
      />
    </main>
  );
}
