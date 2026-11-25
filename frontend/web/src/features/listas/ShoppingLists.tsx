// Gestiona listas y artículos persistidos de la cuenta actual mediante Motor y Catálogo.
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
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
        if (!controlador.signal.aborted) setListas(datos);
      })
      .catch((errorObtenido: unknown) => {
        if (!controlador.signal.aborted)
          setError(normalizarError(errorObtenido));
      })
      .finally(() => {
        if (!controlador.signal.aborted) setCargando(false);
      });
    return () => controlador.abort();
  }, [revision]);

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
    <main className="mx-auto w-full max-w-5xl space-y-6 p-4 md:p-8">
      <header>
        <h1 className="text-2xl font-bold">Mis listas de compra</h1>
        <p className="mt-2 text-muted-foreground">
          Gestiona tus listas guardadas en tu cuenta y consúltalas entre
          sesiones.
        </p>
      </header>
      <p className="rounded-xl border border-border bg-secondary/40 p-4 text-sm">
        Crear, consultar, editar y eliminar listas requiere que el servicio de
        listas esté habilitado en este entorno. Selecciona una lista para buscar
        y agregar productos, cambiar cantidades o quitarlos. El carrito temporal
        del catálogo no se guarda automáticamente en estas listas.
      </p>
      <form onSubmit={crear} className="space-y-2">
        <label htmlFor="new-list-name" className="text-sm font-medium">
          Nombre de la nueva lista
        </label>
        <div className="flex flex-col gap-2 sm:flex-row">
          <Input
            id="new-list-name"
            value={nombre}
            onChange={(evento) => setNombre(evento.target.value)}
            maxLength={150}
            required
            disabled={guardando || cargando || detalleOcupado}
            placeholder="Por ejemplo: compras de la semana"
          />
          <Button
            type="submit"
            disabled={guardando || cargando || detalleOcupado || !nombre.trim()}
          >
            {guardando ? "Creando…" : "Crear lista"}
          </Button>
        </div>
      </form>
      <AvisoErrorLista error={error} />
      <Button
        variant="outline"
        disabled={cargando || guardando || detalleOcupado}
        onClick={() => setRevision((valor) => valor + 1)}
      >
        Actualizar listas
      </Button>
      {cargando ? (
        <p role="status">Cargando listas…</p>
      ) : (
        <div className="grid gap-6 md:grid-cols-2">
          <section aria-label="Listas guardadas">
            {listas.length === 0 ? (
              <p>
                {error
                  ? "No se pudieron cargar las listas."
                  : "Aún no tienes listas. Crea la primera para empezar."}
              </p>
            ) : (
              <ul className="space-y-3">
                {listas.map((lista) => (
                  <li key={lista.id}>
                    <button
                      disabled={guardando || detalleOcupado}
                      onClick={() => setIdSeleccionada(lista.id)}
                      aria-pressed={idSeleccionada === lista.id}
                      className={`w-full rounded-xl border p-4 text-left disabled:opacity-60 ${idSeleccionada === lista.id ? "border-primary bg-primary/5" : "border-border bg-card"}`}
                    >
                      <span className="block break-words font-semibold">
                        {lista.nombre}
                      </span>
                      <span className="text-sm text-muted-foreground">
                        {estados[lista.estado]} · {lista.total_articulos}{" "}
                        artículos
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>
          {idSeleccionada ? (
            <fieldset disabled={guardando} className="min-w-0">
              <DetalleLista
                key={idSeleccionada}
                id={idSeleccionada}
                onBusy={setDetalleOcupado}
                onUpdated={(lista) =>
                  setListas((actuales) =>
                    actuales.map((elemento) =>
                      elemento.id === lista.id ? lista : elemento,
                    ),
                  )
                }
                onDeleted={() => {
                  setListas((actuales) =>
                    actuales.filter(
                      (elemento) => elemento.id !== idSeleccionada,
                    ),
                  );
                  setIdSeleccionada(null);
                }}
              />
            </fieldset>
          ) : (
            <p className="text-muted-foreground">
              Selecciona una lista para consultar su detalle.
            </p>
          )}
        </div>
      )}
    </main>
  );
}

/** Consulta y edita una lista; comunica cambios al padre solo después de confirmarlos en la API. */
function DetalleLista(propiedades: OpcionesDetalleLista) {
  const detalle = useDetalleLista(propiedades);
  const { lista, nombre, estado, cargando, guardando, error } = detalle;
  return (
    <section
      aria-label="Detalle de la lista"
      className="min-w-0 space-y-4 rounded-xl border border-border bg-card p-4"
    >
      <h2 className="text-lg font-semibold">Detalle de la lista</h2>
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
                onClick={() => {
                  if (
                    window.confirm(
                      `¿Eliminar la lista “${lista.nombre}” y todos sus artículos?`,
                    )
                  )
                    void detalle.eliminar();
                }}
              >
                Eliminar lista
              </Button>
            </div>
          </form>
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
    </section>
  );
}
