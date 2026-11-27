// Integra la recomendación con la lista seleccionada y la sesión, sin mapa ni resultados simulados.
import { Link, useSearchParams } from "react-router-dom";
import { ROUTES } from "@/app/routes";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/features/autenticacion/useAuth";
import { ErrorSolicitudRutas, mensajeErrorRuta } from "./rutas-api";
import { ResultadoRuta } from "./ResultadoRuta";
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
  if (estado.tipo === "sin_lista")
    return <p>Selecciona una lista para calcular su ruta.</p>;
  if (estado.tipo === "cargando")
    return <p role="status">Calculando la ruta de tu lista…</p>;
  if (estado.tipo === "resultado")
    return <ResultadoRuta datos={estado.datos} nombres={nombres} />;

  const codigo =
    estado.error instanceof ErrorSolicitudRutas ? estado.error.estado : null;
  return (
    <section
      role="alert"
      className="space-y-3 rounded-xl border border-border bg-card p-4"
    >
      <p className="text-destructive">{mensajeErrorRuta(estado.error)}</p>
      {codigo === 401 ? (
        <Link to={ROUTES.LOGIN} className="underline">
          Ir a iniciar sesión
        </Link>
      ) : codigo === 400 || codigo === 404 || codigo === 422 ? (
        <p className="text-sm text-muted-foreground">
          Vuelve a tus listas para revisar los artículos pendientes y su
          disponibilidad.
        </p>
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
  return (
    <main className="mx-auto w-full max-w-5xl space-y-6 p-4 md:p-8">
      <header className="space-y-3">
        <h1 className="text-2xl font-bold">Ruta recomendada</h1>
        <div className="flex flex-wrap items-center gap-4">
          <Link
            className="font-medium underline"
            to={
              listaID
                ? `${ROUTES.LISTAS}?lista=${encodeURIComponent(listaID)}`
                : ROUTES.LISTAS
            }
          >
            {listaID ? "Volver a mi lista" : "Ir a mis listas"}
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
