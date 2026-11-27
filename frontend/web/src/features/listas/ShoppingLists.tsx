// Gestiona listas y artículos persistidos de la cuenta actual mediante Motor y Catálogo.
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  Info,
  ListChecks,
  ListPlus,
  RefreshCw,
  ShoppingBasket,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuth } from "@/features/autenticacion/useAuth";
import { useToast } from "@/components/ToastContext";
import { ROUTES } from "@/app/routes";
import {
  listasApi,
  ErrorSolicitudListas,
  MAXIMO_ENVASES_ARTICULO,
  type Lista,
  type EstadoLista,
} from "@/features/listas/listas-api";
import { ArticulosLista } from "@/features/listas/ArticulosLista";
import {
  useDetalleLista,
  type OpcionesDetalleLista,
} from "@/features/listas/useDetalleLista";

const estados: Record<EstadoLista, string> = {
  activa: "Activa",
  completada: "Completada",
  archivada: "Archivada",
};
const mensajeError = (errorObtenido: unknown) =>
  errorObtenido instanceof Error
    ? errorObtenido.message
    : "No se pudo conectar con el servicio de listas.";

/** Preserva los errores HTTP; los fallos sin estado no deben solicitar un nuevo ingreso. */
function normalizarError(errorObtenido: unknown): Error {
  return errorObtenido instanceof Error
    ? errorObtenido
    : new Error(mensajeError(errorObtenido));
}

/** Presenta la sesión de forma uniforme en lecturas y escrituras de artículos. */
function AvisoErrorLista({ error }: { error: Error | null }) {
  if (!error) return null;
  const motivos =
    error instanceof ErrorSolicitudListas && Array.isArray(error.detalle)
      ? new Set(error.detalle.map((detalle) => detalle?.motivo))
      : new Set();
  return (
    <div role="alert" className="space-y-2 text-destructive">
      <p>{error.message}</p>
      {motivos.has("no_encontrado") && (
        <p>
          El producto seleccionado ya no existe en Catálogo. Busca otra
          alternativa.
        </p>
      )}
      {motivos.has("cantidad_maxima") && (
        <p>
          La cantidad sumada a los envases guardados supera el máximo de{" "}
          {MAXIMO_ENVASES_ARTICULO} por producto.
        </p>
      )}
      {error instanceof ErrorSolicitudListas && error.estado === 401 && (
        <Link to={ROUTES.LOGIN} className="underline">
          Ir a iniciar sesión
        </Link>
      )}
    </div>
  );
}

/** Desmonta todo el estado de listas al cambiar la cuenta, sin mezclar compras entre usuarios. */
export default function ShoppingLists() {
  const { user } = useAuth();
  return <ContenidoListas key={user?.id} />;
}

/** Carga las listas del usuario y coordina creación y selección con operaciones reales. */
function ContenidoListas() {
  const [parametros] = useSearchParams();
  const destinoSolicitado = parametros.get("lista");
  const { toast } = useToast();
  const [listas, setListas] = useState<Lista[]>([]);
  const [idSeleccionada, setIdSeleccionada] = useState<string | null>(null);
  const [nombre, setNombre] = useState("");
  const [cargando, setCargando] = useState(true);
  const [guardando, setGuardando] = useState(false);
  const [detalleOcupado, setDetalleOcupado] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [revision, setRevision] = useState(0);
  const creacionEnCurso = useRef(false);

  useEffect(() => {
    const controlador = new AbortController();
    setCargando(true);
    setError(null);
    listasApi
      .listar(controlador.signal)
      .then(({ listas: datos }) => {
        if (!controlador.signal.aborted) {
          setListas(datos);
          if (
            destinoSolicitado &&
            datos.some((lista) => lista.id === destinoSolicitado)
          )
            setIdSeleccionada(destinoSolicitado);
        }
      })
      .catch((errorObtenido: unknown) => {
        if (!controlador.signal.aborted)
          setError(normalizarError(errorObtenido));
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargando(false);
      });
    return () => controlador.abort();
  }, [revision, destinoSolicitado]);

  /** Evita dos creaciones simultáneas antes del siguiente render. */
  async function crear(evento: FormEvent) {
    evento.preventDefault();
    if (creacionEnCurso.current || guardando || !nombre.trim()) return;
    creacionEnCurso.current = true;
    setGuardando(true);
    setError(null);
    try {
      const lista = await listasApi.crear(nombre.trim());
      setListas((actuales) => [lista, ...actuales]);
      setIdSeleccionada(lista.id);
      setNombre("");
      toast("Lista creada y guardada", "success");
    } catch (errorObtenido) {
      setError(normalizarError(errorObtenido));
    } finally {
      creacionEnCurso.current = false;
      setGuardando(false);
    }
  }

  return (
    <div className="relative flex h-full w-full flex-1 overflow-hidden bg-background">
      <main className="relative z-10 mx-auto flex w-full max-w-[1600px] flex-1 flex-col overflow-hidden px-4 py-6 md:px-6">
        <div className="min-h-0 flex-1 overflow-y-auto pb-28 md:pb-16 pr-1 -mr-1 scrollbar-hide">
          <div className="mx-auto w-full max-w-6xl space-y-7 p-4 md:p-8">
            <header className="relative isolate flex min-h-52 items-end overflow-hidden rounded-xl border-2 border-border bg-primary px-6 py-7 text-primary-foreground shadow-[8px_8px_0px_var(--color-border)] sm:min-h-60 sm:px-9 sm:py-8">
              <img
                src="/street_art_supermarket.jpg"
                alt=""
                aria-hidden="true"
                className="absolute inset-0 -z-20 size-full object-cover object-center opacity-30 grayscale"
              />
              <div
                className="absolute inset-0 -z-10 bg-primary/75"
                aria-hidden="true"
              />
              <div className="max-w-2xl">
                <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-primary-foreground/30 bg-primary-foreground/10 px-3 py-1 text-xs font-semibold uppercase">
                  <ShoppingBasket className="size-4" aria-hidden="true" />
                  Compra a tu manera
                </div>
                <h1 className="font-display text-3xl font-extrabold text-balance sm:text-4xl">
                  Mis listas de compra
                </h1>
                <p className="mt-2 max-w-xl text-sm text-primary-foreground/90 sm:text-base">
                  Organiza tus productos, ajusta cantidades y compara opciones
                  cuando estés listo para comprar.
                </p>
                <p className="mt-4 text-sm font-semibold">
                  {listas.length} {listas.length === 1 ? "lista guardada" : "listas guardadas"}
                </p>
              </div>
            </header>

            <p className="flex items-start gap-3 border-l-4 border-primary bg-secondary/50 px-4 py-3 text-sm text-muted-foreground">
              <Info
                className="mt-0.5 size-4 shrink-0 text-primary"
                aria-hidden="true"
              />
              <span>
                Selecciona una lista para buscar y agregar productos, cambiar
                cantidades o quitarlos. Desde el carrito del catálogo puedes
                elegir una lista y confirmar el guardado de sus productos: no se
                guarda automáticamente.
              </span>
            </p>

            <form
              onSubmit={crear}
              className="grid gap-3 border-b-2 border-border pb-6 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
            >
              <div className="space-y-2">
                <label
                  htmlFor="new-list-name"
                  className="block text-sm font-semibold"
                >
                  Crea una lista nueva
                </label>
                <Input
                  id="new-list-name"
                  value={nombre}
                  onChange={(evento) => setNombre(evento.target.value)}
                  maxLength={150}
                  required
                  disabled={guardando || cargando || detalleOcupado}
                  placeholder="Por ejemplo: compras de la semana"
                  className="h-12"
                />
              </div>
              <Button
                type="submit"
                disabled={
                  guardando || cargando || detalleOcupado || !nombre.trim()
                }
                className="h-12"
              >
                <ListPlus aria-hidden="true" />
                {guardando ? "Creando…" : "Crear lista"}
              </Button>
            </form>

            <AvisoErrorLista error={error} />

            <section aria-label="Listas guardadas" className="space-y-4">
              <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                  <h2 className="font-display text-xl font-bold">Tus listas</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Elige una para ver y editar sus artículos.
                  </p>
                </div>
                <Button
                  variant="outline"
                  disabled={cargando || guardando || detalleOcupado}
                  onClick={() =>
                    setRevision((revisionActual) => revisionActual + 1)
                  }
                >
                  <RefreshCw aria-hidden="true" />
                  Actualizar listas
                </Button>
              </div>

              {cargando ? (
                <p role="status" className="py-4 text-muted-foreground">
                  Cargando listas…
                </p>
              ) : listas.length === 0 ? (
                <p className="border-y border-border py-5 text-muted-foreground">
                  {error
                    ? "No se pudieron cargar las listas."
                    : "Aún no tienes listas. Crea la primera para empezar."}
                </p>
              ) : (
                <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {listas.map((lista) => (
                    <li key={lista.id}>
                      <button
                        disabled={guardando || detalleOcupado}
                        onClick={() =>
                          setIdSeleccionada((idActual) =>
                            idActual === lista.id ? null : lista.id,
                          )
                        }
                        aria-pressed={idSeleccionada === lista.id}
                        className={`flex min-h-24 w-full items-start gap-3 rounded-lg border-2 p-4 text-left transition-colors disabled:opacity-60 ${idSeleccionada === lista.id ? "border-primary bg-primary/10 shadow-[4px_4px_0px_var(--color-border)]" : "border-border bg-card hover:border-primary/60"}`}
                      >
                        <span className="grid size-10 shrink-0 place-items-center rounded-md bg-secondary text-primary">
                          <ListChecks className="size-5" aria-hidden="true" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="block break-words font-semibold">
                            {lista.nombre}
                          </span>
                          <span className="mt-1 block text-sm text-muted-foreground">
                            {estados[lista.estado]} · {lista.total_articulos}{" "}
                            {lista.total_articulos === 1
                              ? "artículo"
                              : "artículos"}
                          </span>
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {cargando ? null : idSeleccionada ? (
              <fieldset disabled={guardando} className="min-w-0">
                <DetalleLista
                  key={idSeleccionada}
                  id={idSeleccionada}
                  onBusy={setDetalleOcupado}
                  onUpdated={(listaActualizada) =>
                    setListas((listasActuales) =>
                      listasActuales.map((elementoLista) =>
                        elementoLista.id === listaActualizada.id
                          ? listaActualizada
                          : elementoLista,
                      ),
                    )
                  }
                  onDeleted={() => {
                    setListas((listasActuales) =>
                      listasActuales.filter(
                        (elementoLista) => elementoLista.id !== idSeleccionada,
                      ),
                    );
                    setIdSeleccionada(null);
                  }}
                />
              </fieldset>
            ) : listas.length > 0 ? (
              <p className="border-t border-border pt-5 text-sm text-muted-foreground">
                Selecciona una lista para consultar su detalle.
              </p>
            ) : null}
          </div>
        </div>
      </main>
    </div>
  );
}

/** Consulta y edita una lista; comunica cambios al padre solo después de confirmarlos en la API. */
function DetalleLista(propiedades: OpcionesDetalleLista) {
  const navegar = useNavigate();
  const detalle = useDetalleLista(propiedades);
  const [confirmacionEliminacionAbierta, setConfirmacionEliminacionAbierta] =
    useState(false);
  const { lista, nombre, estado, cargando, guardando, error } = detalle;

  async function confirmarEliminacion() {
    const eliminacionConfirmada = await detalle.eliminar();
    if (!eliminacionConfirmada) setConfirmacionEliminacionAbierta(false);
  }

  return (
    <section
      aria-label="Detalle de la lista"
      className="mx-auto w-full max-w-4xl min-w-0 space-y-5 border-t-2 border-border bg-background/70 py-6"
    >
      <h2 className="flex items-center gap-2 font-display text-xl font-bold">
        <ListChecks className="size-5 text-primary" aria-hidden="true" />
        Detalle de la lista
      </h2>
      <AvisoErrorLista error={error} />
      {cargando ? (
        <p role="status">Cargando detalle…</p>
      ) : lista ? (
        <>
          <form
            onSubmit={(evento) => {
              evento.preventDefault();
              void detalle.guardar();
            }}
            className="space-y-3"
          >
            <label htmlFor="list-name" className="block text-sm font-medium">
              Nombre
            </label>
            <Input
              id="list-name"
              value={nombre}
              onChange={(evento) => detalle.setNombre(evento.target.value)}
              maxLength={150}
              required
              disabled={guardando}
            />
            <label htmlFor="list-state" className="block text-sm font-medium">
              Estado
            </label>
            <select
              id="list-state"
              value={estado}
              onChange={(evento) =>
                detalle.setEstado(evento.target.value as EstadoLista)
              }
              disabled={guardando}
              className="w-full rounded-md border border-input bg-background p-2"
            >
              {Object.entries(estados).map(([valor, etiqueta]) => (
                <option key={valor} value={valor}>
                  {etiqueta}
                </option>
              ))}
            </select>
            <div className="flex flex-wrap gap-2">
              <Button type="submit" disabled={guardando || !nombre.trim()}>
                {guardando ? "Procesando…" : "Guardar cambios"}
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={guardando}
                onClick={() => setConfirmacionEliminacionAbierta(true)}
              >
                <Trash2 aria-hidden="true" />
                Eliminar lista
              </Button>
            </div>
          </form>
          <Button
            type="button"
            disabled={guardando}
            onClick={() =>
              navegar(`${ROUTES.RUTA}?lista=${encodeURIComponent(lista.id)}`)
            }
          >
            Calcular ruta
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={guardando}
            onClick={detalle.recargar}
          >
            Actualizar detalle y disponibilidad
          </Button>
          {guardando && (
            <p role="status" className="text-sm">
              Guardando cambios…
            </p>
          )}
          <ArticulosLista
            articulos={lista.articulos}
            ocupado={guardando}
            alAgregar={detalle.agregarArticulo}
            alActualizar={detalle.actualizarArticulo}
            alEliminar={detalle.eliminarArticulo}
          />
        </>
      ) : (
        <Button variant="outline" onClick={detalle.recargar}>
          Reintentar detalle
        </Button>
      )}
      {confirmacionEliminacionAbierta && lista && (
        <div
          className="fixed inset-0 z-[100] grid place-items-center bg-foreground/50 p-4 backdrop-blur-sm"
          onClick={(evento) => {
            if (evento.target === evento.currentTarget)
              setConfirmacionEliminacionAbierta(false);
          }}
          onKeyDown={(evento) => {
            if (evento.key === "Escape")
              setConfirmacionEliminacionAbierta(false);
          }}
        >
          <section
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="confirmar-eliminar-lista-titulo"
            aria-describedby="confirmar-eliminar-lista-detalle"
            className="w-full max-w-md space-y-5 rounded-xl border-2 border-border bg-background p-5 shadow-[8px_8px_0px_var(--color-border)] sm:p-6"
          >
            <div className="flex items-start gap-3">
              <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-destructive/10 text-destructive">
                <Trash2 className="size-5" aria-hidden="true" />
              </span>
              <div className="space-y-1">
                <h3
                  id="confirmar-eliminar-lista-titulo"
                  className="font-display text-lg font-bold"
                >
                  ¿Eliminar esta lista?
                </h3>
                <p
                  id="confirmar-eliminar-lista-detalle"
                  className="text-sm text-muted-foreground"
                >
                  Se eliminará “{lista.nombre}” junto con todos sus artículos.
                  Esta acción no se puede deshacer.
                </p>
              </div>
            </div>
            <div className="flex flex-col-reverse justify-end gap-2 sm:flex-row">
              <Button
                type="button"
                variant="outline"
                disabled={guardando}
                autoFocus
                onClick={() => setConfirmacionEliminacionAbierta(false)}
              >
                Cancelar
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={guardando}
                onClick={() => void confirmarEliminacion()}
              >
                <Trash2 aria-hidden="true" />
                {guardando ? "Eliminando…" : "Eliminar lista"}
              </Button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
