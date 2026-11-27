// Integra la recomendación con la lista seleccionada y la sesión, sin mapa ni resultados simulados.
import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Bike, BusFront, CarFront, Footprints } from "lucide-react";
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

type MedioTransporte = "vehiculo" | "publico" | "bicicleta" | "pie";

const OPCIONES_TRANSPORTE: {
  valor: MedioTransporte;
  etiqueta: string;
  Icono: typeof CarFront;
}[] = [
  { valor: "vehiculo", etiqueta: "Vehículo particular", Icono: CarFront },
  { valor: "publico", etiqueta: "Transporte público", Icono: BusFront },
  { valor: "bicicleta", etiqueta: "Bicicleta", Icono: Bike },
  { valor: "pie", etiqueta: "A pie", Icono: Footprints },
];

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
  const [medioTransporteSeleccionado, setMedioTransporteSeleccionado] =
    useState<MedioTransporte>("vehiculo");
  return (
    <div className="relative flex h-full w-full flex-1 overflow-hidden bg-background">
      <main className="relative z-10 mx-auto flex w-full max-w-[1600px] flex-1 flex-col overflow-hidden px-4 py-6 md:px-6">
        <div className="min-h-0 flex-1 overflow-y-auto pb-2 pr-1 -mr-1 scrollbar-hide">
          <div className="mx-auto w-full max-w-5xl space-y-6 p-4 pb-24 md:p-8">
            <header className="relative isolate overflow-hidden rounded-xl border-2 border-border bg-primary px-6 py-6 text-primary-foreground shadow-[8px_8px_0px_var(--color-border)] sm:px-8 sm:py-7">
              <div
                className="absolute inset-0 -z-20 bg-[url('/images/dashboard-seamless.jpg')] bg-cover bg-center opacity-25"
                aria-hidden="true"
              />
              <div
                className="absolute inset-0 -z-10 bg-primary/85"
                aria-hidden="true"
              />
              <div
                className="pointer-events-none absolute inset-y-0 right-0 z-0 hidden w-[38%] overflow-hidden sm:block"
                aria-hidden="true"
              >
                <img
                  src="/mapa_isometrico_ruta_supermercado.png"
                  alt=""
                  className="absolute -bottom-[25%] -right-[28%] w-[135%] max-w-none opacity-35 grayscale mix-blend-multiply"
                />
              </div>
              <div className="relative z-10 max-w-2xl space-y-4">
                <span className="inline-flex rounded-full border border-primary-foreground/30 bg-primary-foreground/10 px-3 py-1 text-xs font-bold">
                  Tu lista, tienda por tienda
                </span>
                <h1 className="font-display text-3xl font-extrabold text-balance sm:text-4xl">
                  Ruta recomendada
                </h1>
                <p className="max-w-xl text-sm leading-relaxed text-primary-foreground/90">
                  Descubre dónde comprar los artículos pendientes y cómo se
                  reparte tu compra. La recomendación usa los precios y
                  sucursales disponibles al calcular.
                </p>
                <div className="flex flex-wrap items-center gap-4 border-t border-primary-foreground/30 pt-4">
                  <Link
                    className="font-semibold underline underline-offset-4"
                    to={
                      seleccionValida
                        ? `${ROUTES.LISTAS}?lista=${encodeURIComponent(listaID)}`
                        : ROUTES.LISTAS
                    }
                  >
                    {seleccionValida ? "Volver a mi lista" : "Ir a mis listas"}
                  </Link>
                  {estado.tipo === "resultado" && (
                    <Button
                      variant="outline"
                      onClick={recalcular}
                      className="border-primary-foreground/40 bg-primary-foreground/10 text-primary-foreground hover:bg-primary-foreground/20 hover:text-primary-foreground"
                    >
                      Recalcular ruta
                    </Button>
                  )}
                </div>
              </div>
            </header>
            {seleccionValida && (
              <section
                aria-labelledby="medio-transporte-titulo"
                className="space-y-3 rounded-2xl border border-border bg-card p-4 sm:p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <h2
                      id="medio-transporte-titulo"
                      className="font-display text-lg font-bold"
                    >
                      Medio de transporte
                    </h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      Puedes explorar estas opciones visuales; no modifican el
                      cálculo.
                    </p>
                  </div>
                  <span className="rounded-full border border-border bg-secondary/60 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wide text-muted-foreground">
                    Vista previa
                  </span>
                </div>
                <fieldset>
                  <legend className="sr-only">
                    Elige un medio de transporte
                  </legend>
                  <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                    {OPCIONES_TRANSPORTE.map(({ valor, etiqueta, Icono }) => (
                      <label
                        key={valor}
                        className={`flex min-h-14 cursor-pointer items-center gap-2 rounded-xl border px-3 py-2.5 text-xs font-semibold transition-colors focus-within:ring-4 focus-within:ring-ring/20 ${medioTransporteSeleccionado === valor ? "border-primary bg-primary/10 text-foreground" : "border-border bg-card hover:border-primary/50"}`}
                      >
                        <input
                          type="radio"
                          name="medio-transporte"
                          value={valor}
                          checked={medioTransporteSeleccionado === valor}
                          onChange={() => setMedioTransporteSeleccionado(valor)}
                          className="sr-only"
                        />
                        <span className="grid size-9 shrink-0 place-items-center rounded-md bg-secondary text-primary">
                          <Icono className="size-5" aria-hidden="true" />
                        </span>
                        <span>{etiqueta}</span>
                      </label>
                    ))}
                  </div>
                </fieldset>
                <p className="border-l-4 border-primary bg-secondary/40 px-3 py-2 text-xs text-muted-foreground">
                  La selección es visual por ahora. La ruta y las distancias
                  todavía no cambian según el medio de transporte.
                </p>
              </section>
            )}
            <EstadoRecomendacion
              estado={estado}
              nombres={nombres}
              recalcular={recalcular}
            />
          </div>
        </div>
      </main>
    </div>
  );
}
