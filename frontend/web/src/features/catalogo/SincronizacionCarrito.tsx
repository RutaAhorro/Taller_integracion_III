// Explica qué está confirmado, qué sólo vive en el navegador y cómo resolver conflictos.
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { ROUTES } from "@/app/routes";
import { ErrorSolicitudListas } from "@/features/listas/listas-api";
import { useAuth } from "@/features/autenticacion/useAuth";
import { useCart } from "./CartContext";

/** Distingue la persistencia remota de la copia local pendiente de sincronizar. */
export function SincronizacionCarrito() {
  const { user } = useAuth();
  const { persistencia } = useCart();
  const { estado, almacen } = persistencia;
  if (!user)
    return (
      <p className="mb-4 text-sm">
        Inicia sesión para recuperar tu carrito guardado.
      </p>
    );
  return (
    <section
      className="mb-4 space-y-2 rounded-lg border border-border p-3 text-sm"
      aria-label="Sincronización del carrito"
    >
      <p role="status">
        {estado.cargando
          ? "Recuperando carrito…"
          : estado.sincronizando
            ? "Sincronizando carrito…"
            : estado.intento
              ? "Envío pendiente de confirmar. Conservamos su clave para recuperarlo."
              : estado.sucio
                ? "Cambios pendientes de sincronización."
                : estado.error
                  ? "No se pudo verificar el estado del servidor. Conservamos el borrador disponible."
                  : estado.listo
                    ? "Carrito guardado en tu cuenta."
                    : "Todavía no se pudo recuperar el carrito de tu cuenta."}
      </p>
      {estado.errorLocal && (
        <p role="alert" className="text-destructive">
          {estado.errorLocal}
        </p>
      )}
      {estado.error && (
        <p role="alert" className="text-destructive">
          {estado.error.message}
        </p>
      )}
      {estado.error instanceof ErrorSolicitudListas &&
        estado.error.estado === 401 && (
          <Link className="underline" to={ROUTES.LOGIN}>
            Iniciar sesión para recuperar el carrito
          </Link>
        )}
      {(estado.error || estado.sucio || !estado.listo) && (
        <Button
          variant="outline"
          disabled={estado.cargando || estado.sincronizando}
          onClick={() => void almacen.recuperar()}
        >
          Recuperar y reintentar sincronización
        </Button>
      )}
      {estado.conflicto && (
        <div className="space-y-2">
          <p>
            No sobrescribimos los cambios de otro dispositivo. Compara los dos
            borradores antes de elegir:
          </p>
          <details open>
            <summary>
              Mi borrador local ({estado.items.length} productos)
            </summary>
            <ul>
              {estado.items.map((elemento) => (
                <li key={elemento.product.id}>
                  {elemento.product.name}: {elemento.quantity} envases
                </li>
              ))}
            </ul>
          </details>
          <details open>
            <summary>
              Carrito del servidor ({estado.conflicto.articulos.length}{" "}
              productos)
            </summary>
            <ul>
              {estado.conflicto.articulos.map((articulo) => (
                <li key={articulo.producto_id}>
                  {estado.conflicto?.disponibilidad[articulo.producto_id]
                    ?.producto?.nombre ?? articulo.producto_id}
                  : {articulo.cantidad} envases
                </li>
              ))}
            </ul>
          </details>
          <Button
            variant="outline"
            onClick={() => almacen.resolverConflicto(false)}
          >
            Usar servidor y descartar mi borrador local
          </Button>
          <Button
            variant="outline"
            disabled={!!estado.conflicto.envio || !!estado.intento}
            onClick={() => almacen.resolverConflicto(true)}
          >
            Conservar mi borrador y reemplazar el servidor
          </Button>
          {(estado.conflicto.envio || estado.intento) && (
            <p>
              Hay un envío pendiente. Recupera el carrito del servidor antes de
              editar; no se puede reemplazar un envío preparado.
            </p>
          )}
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        La copia del navegador pertenece a esta cuenta. Sólo lo sincronizado
        puede recuperarse desde otro dispositivo; limpiar los datos del
        navegador elimina los cambios aún no enviados.
      </p>
    </section>
  );
}
