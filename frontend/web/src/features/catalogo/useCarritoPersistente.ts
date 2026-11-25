// Adapta el coordinador durable a React; cada proveedor se desmonta al cambiar de cuenta.
import { useEffect, useState, useSyncExternalStore } from "react";
import { CarritoPersistente } from "./carrito-persistente";

/** El navegador puede denegar localStorage; eso no debe impedir recuperar la copia del servidor. */
function almacenamientoDisponible(): Storage | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/** Recupera al entrar, reconectar o volver a la ventana; nunca inicia red para invitados. */
export function useCarritoPersistente(usuarioID?: string) {
  const [almacen] = useState(
    () =>
      new CarritoPersistente(
        usuarioID ?? "invitado",
        usuarioID ? almacenamientoDisponible() : null,
      ),
  );
  const estado = useSyncExternalStore(
    almacen.suscribir,
    almacen.obtenerEstado,
    almacen.obtenerEstado,
  );
  useEffect(() => {
    if (!usuarioID) return;
    almacen.activar();
    void almacen.recuperar();
    /** Reconcilia antes de reenviar para detectar otras pestañas o dispositivos. */
    const recuperar = () => {
      void almacen.recuperar();
    };
    window.addEventListener("online", recuperar);
    window.addEventListener("focus", recuperar);
    return () => {
      almacen.suspender();
      window.removeEventListener("online", recuperar);
      window.removeEventListener("focus", recuperar);
    };
  }, [almacen, usuarioID]);
  return { almacen, estado };
}
