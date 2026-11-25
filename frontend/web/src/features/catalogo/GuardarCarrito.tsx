// Presenta destino, confirmación y recuperación del guardado; no simula optimización.
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ROUTES } from "@/app/routes";
import { useAuth } from "@/features/autenticacion/useAuth";
import { useCart } from "./CartContext";
import { ErrorSolicitudListas } from "@/features/listas/listas-api";

// ===== Mensajes de recuperación =====

/** Mantiene las causas de negocio por producto y distingue sesión vencida de red caída. */
function ErrorGuardado({ error }: { error: Error | null }) {
  const { items } = useCart();
  if (!error) return null;
  return (
    <div role="alert" className="space-y-2 text-sm text-destructive">
      <p>{error.message}</p>
      {error instanceof ErrorSolicitudListas &&
        Array.isArray(error.detalle) && (
          <ul>
            {error.detalle.map((detalle, indice) => (
              <li key={indice}>
                {items.find(
                  (elemento) => elemento.product.id === detalle?.producto_id,
                )?.product.name ?? "Producto"}
                :{" "}
                {detalle?.motivo === "cantidad_maxima"
                  ? "La suma con la lista supera 99 envases. Reduce la cantidad o elige otra lista."
                  : detalle?.motivo === "no_encontrado"
                    ? "Ya no existe en Catálogo. Quítalo del carrito o reemplázalo."
                    : "Revisa este producto antes de reintentar."}
              </li>
            ))}
          </ul>
        )}
      {error instanceof ErrorSolicitudListas && error.estado === 404 && (
        <p>Recarga los destinos y elige una lista disponible.</p>
      )}
      {error instanceof ErrorSolicitudListas && error.estado === 401 && (
        <p>
          Tu carrito guardado se recupera al volver a iniciar sesión.{" "}
          <Link className="underline" to={ROUTES.LOGIN}>
            Iniciar sesión
          </Link>
        </p>
      )}
    </div>
  );
}

/** Exige confirmación antes de sumar envases y conserva el carrito ante cualquier fallo. */
export function GuardarCarrito() {
  const { user } = useAuth();
  const { guardado, setIsCartOpen } = useCart();
  if (!guardado.abierto) return null;
  if (!user)
    return (
      <section className="space-y-2 p-5" aria-label="Guardar carrito">
        <p>
          Inicia sesión para preparar tu compra y recuperar el carrito guardado
          de tu cuenta.
        </p>
        <Link
          to={ROUTES.LOGIN}
          onClick={() => setIsCartOpen(false)}
          className="underline"
        >
          Iniciar sesión
        </Link>
      </section>
    );

  if (guardado.resultado)
    return (
      <section className="space-y-3 p-5" aria-label="Carrito guardado">
        <p role="status">Productos guardados en la lista de destino.</p>
        <p className="text-sm">
          Revisa la lista: puede contener artículos sin stock confirmado o sin
          precio vigente.
        </p>
        <Link
          className="underline font-medium"
          to={`${ROUTES.LISTAS}?lista=${encodeURIComponent(guardado.resultado.id)}`}
          onClick={() => {
            guardado.cerrar();
            setIsCartOpen(false);
          }}
        >
          Abrir lista guardada
        </Link>
      </section>
    );

  return (
    <section
      className="space-y-3 border-t border-border p-5"
      aria-label="Guardar carrito"
    >
      <h3 className="font-semibold">Confirmar destino y cantidades</h3>
      <p className="text-sm text-muted-foreground">
        Revisa los envases arriba. Si un producto ya está en la lista, se
        sumarán sus cantidades. Guardar no garantiza stock ni precios vigentes.
      </p>
      <ErrorGuardado error={guardado.errorCarga} />
      <ErrorGuardado error={guardado.error} />
      {guardado.pendiente && !guardado.ocupado && (
        <p role="status" className="text-sm">
          No pudimos confirmar el resultado. Conservamos el carrito y bloqueamos
          los cambios para reintentar el mismo envío sin duplicarlo. La clave
          queda guardada y puede recuperarse al volver a iniciar sesión.{" "}
          <Link
            className="underline"
            to={`${ROUTES.LISTAS}?lista=${encodeURIComponent(guardado.destino)}`}
            onClick={() => setIsCartOpen(false)}
          >
            Revisar la lista de destino
          </Link>
        </p>
      )}
      <label htmlFor="destino-carrito" className="block text-sm">
        Lista de destino
      </label>
      <select
        id="destino-carrito"
        value={guardado.destino}
        disabled={guardado.bloqueado || guardado.cargando}
        onChange={(evento) => guardado.setDestino(evento.target.value)}
        className="w-full rounded-md border border-border bg-background p-2"
      >
        <option value="">Selecciona una lista</option>
        {guardado.listas.map((lista) => (
          <option value={lista.id} key={lista.id}>
            {lista.nombre} ({lista.estado})
          </option>
        ))}
      </select>
      {guardado.cargando && <p role="status">Cargando listas…</p>}
      {!guardado.cargando &&
        !guardado.errorCarga &&
        !guardado.listas.length && (
          <p className="text-sm">
            Todavía no tienes listas. Crea una para continuar.
          </p>
        )}
      <Button
        variant="outline"
        disabled={guardado.ocupado || guardado.cargando}
        onClick={guardado.recargar}
      >
        Recargar listas
      </Button>
      {!guardado.pendiente && (
        <form
          className="space-y-2"
          onSubmit={(evento) => {
            evento.preventDefault();
            void guardado.crearLista();
          }}
        >
          <label htmlFor="nombre-lista-carrito" className="block text-sm">
            O crea una lista nueva
          </label>
          <Input
            id="nombre-lista-carrito"
            maxLength={150}
            value={guardado.nombre}
            disabled={guardado.ocupado || guardado.creacionIncierta}
            onChange={(evento) => guardado.setNombre(evento.target.value)}
            placeholder="Compras de la semana"
          />
          <Button
            type="submit"
            variant="outline"
            disabled={
              guardado.ocupado ||
              guardado.creacionIncierta ||
              !guardado.nombre.trim()
            }
          >
            Crear lista de destino
          </Button>
        </form>
      )}
      {guardado.creacionIncierta && (
        <div role="status" className="space-y-2 text-sm">
          <p>
            La lista podría haberse creado aunque no recibimos la respuesta.
            Recarga y búscala antes de crear otra. Los productos aún no se
            enviaron.
          </p>
          <Button
            variant="outline"
            disabled={guardado.ocupado || guardado.cargando}
            onClick={guardado.permitirOtraCreacion}
          >
            Revisé las listas; necesito crear otra
          </Button>
        </div>
      )}
      <Button
        className="w-full"
        disabled={
          guardado.ocupado ||
          (!guardado.pendiente && (guardado.bloqueado || !guardado.destino))
        }
        onClick={() => void guardado.guardar()}
      >
        {guardado.ocupado
          ? "Procesando…"
          : guardado.pendiente
            ? "Reintentar el mismo guardado"
            : "Confirmar y guardar productos"}
      </Button>
      <Button
        variant="ghost"
        disabled={guardado.ocupado}
        onClick={guardado.cerrar}
      >
        {guardado.pendiente
          ? "Cerrar (el envío queda pendiente)"
          : "Volver al carrito"}
      </Button>
    </section>
  );
}
