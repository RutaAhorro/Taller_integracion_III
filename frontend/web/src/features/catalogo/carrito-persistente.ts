// Coordina borrador local, versión del servidor y confirmación durable sin sobrescribir conflictos.
import {
  crearCarritoApi,
  type CarritoServidor,
  type EnvioCarrito,
  type ReciboCarrito,
} from "./carrito-api";
import {
  ErrorSolicitudListas,
  MAXIMO_ARTICULOS_LISTA,
  MAXIMO_ENVASES_ARTICULO,
  type ArticuloParaAgregar,
} from "@/features/listas/listas-api";
import type { UiProduct } from "./tipos";

// ===== Estado observable y recuperación local =====

/** Precio nullable: un producto recuperado no adquiere vigencia por estar guardado. */
export type ProductoCarrito = Omit<UiProduct, "price"> & {
  price: number | null;
};
/** Envases elegidos y datos de presentación; sólo ID y cantidad llegan a PostgreSQL. */
export type ItemCarrito = { product: ProductoCarrito; quantity: number };
/** Preparación local recuperable aun si se cierra el navegador antes de recibir la respuesta. */
type IntentoCarrito = EnvioCarrito & { version: number };
/** Copia por cuenta; no contiene JWT, cookies, credenciales ni tokens internos. */
type CopiaLocal = {
  version: number;
  items: ItemCarrito[];
  sucio: boolean;
  intento: IntentoCarrito | null;
};
/** Estado que consume React; conflicto conserva ambas versiones hasta una decisión explícita. */
export type EstadoCarrito = CopiaLocal & {
  cargando: boolean;
  sincronizando: boolean;
  listo: boolean;
  error: Error | null;
  errorLocal: string | null;
  conflicto: CarritoServidor | null;
  recibo: ReciboCarrito | null;
};

/** Mensaje legible sin ocultar el estado HTTP. */
function errorCarrito(errorObtenido: unknown): Error {
  if (errorObtenido instanceof TypeError)
    return new Error(
      "Sin conexión con el servicio. Los cambios del carrito siguen pendientes de sincronización.",
    );
  if (
    errorObtenido instanceof Error &&
    ["TimeoutError", "AbortError"].includes(errorObtenido.name)
  )
    return new Error(
      "Se agotó la espera. Conservamos los datos y la clave pendiente para comprobar el resultado al reconectar.",
    );
  return errorObtenido instanceof Error
    ? errorObtenido
    : new Error("No se pudo sincronizar el carrito.");
}

/** Firma canónica para reconocer un PUT confirmado cuya respuesta se perdió. */
export function firmaArticulos(articulos: ArticuloParaAgregar[]): string {
  return JSON.stringify(
    [...articulos].sort((izquierda, derecha) =>
      izquierda.producto_id.localeCompare(derecha.producto_id),
    ),
  );
}

/** Extrae sólo lo que pertenece al dominio del borrador. */
function articulosDe(items: ItemCarrito[]): ArticuloParaAgregar[] {
  return items.map((elemento) => ({
    producto_id: elemento.product.id,
    cantidad: elemento.quantity,
  }));
}

/** Aísla por cuenta y permite evolucionar el formato sin interpretar otras claves de localStorage. */
export function claveCarrito(usuarioID: string): string {
  return `rutaahorro.carrito.v1.${usuarioID}`;
}

/** Comprueba copias dañadas antes de usarlas; los precios se invalidan al recuperar. */
function leerCopia(almacen: Storage | null, clave: string): CopiaLocal | null {
  const texto = almacen?.getItem(clave);
  if (!texto) return null;
  const datos = JSON.parse(texto) as CopiaLocal;
  if (
    !Number.isSafeInteger(datos.version) ||
    datos.version < 0 ||
    typeof datos.sucio !== "boolean" ||
    !Array.isArray(datos.items) ||
    datos.items.length > MAXIMO_ARTICULOS_LISTA ||
    datos.items.some(
      (elemento) =>
        !elemento?.product ||
        typeof elemento.product.id !== "string" ||
        typeof elemento.product.name !== "string" ||
        !Number.isInteger(elemento.quantity) ||
        elemento.quantity < 1 ||
        elemento.quantity > MAXIMO_ENVASES_ARTICULO,
    ) ||
    new Set(datos.items.map((elemento) => elemento.product.id)).size !==
      datos.items.length ||
    (datos.intento !== null &&
      (!datos.intento?.clave ||
        !datos.intento?.lista_id ||
        !Number.isSafeInteger(datos.intento?.version)))
  )
    throw new Error(
      "La copia local no es válida. No la reemplazamos automáticamente.",
    );
  return {
    ...datos,
    items: datos.items.map((elemento) => ({
      ...elemento,
      product: { ...elemento.product, price: null, inStock: undefined },
    })),
  };
}

// ===== Coordinador sin dependencias de React =====

/** Una instancia pertenece a una cuenta; serializa red y conserva cambios locales hechos durante un PUT. */
export class CarritoPersistente {
  private estado: EstadoCarrito;
  private oyentes = new Set<() => void>();
  private enCurso: Promise<void> | null = null;
  private guardandoLista = false;
  private activo = true;
  private clave: string;
  private almacen: Storage | null;
  private api: ReturnType<typeof crearCarritoApi>;

  constructor(
    usuarioID: string,
    almacen: Storage | null,
    api = crearCarritoApi(usuarioID),
  ) {
    this.almacen = almacen;
    this.api = api;
    this.clave = claveCarrito(usuarioID);
    let copia: CopiaLocal | null = null;
    let errorLocal: string | null = almacen
      ? null
      : "El navegador no permite recuperar cambios sin sincronizar. No cierres esta pestaña hasta guardarlos en el servidor.";
    try {
      copia = leerCopia(almacen, this.clave);
    } catch {
      try {
        const texto = almacen?.getItem(this.clave);
        if (texto) almacen?.setItem(`${this.clave}.recuperacion`, texto);
      } catch {
        /* Un navegador que deniega almacenamiento no puede ofrecer respaldo local. */
      }
      errorLocal =
        "No pudimos leer la copia local. Si el navegador lo permitió, se respaldó como .recuperacion. Recupera el carrito del servidor antes de continuar.";
    }
    this.estado = {
      version: 0,
      items: [],
      sucio: false,
      intento: null,
      ...copia,
      cargando: false,
      sincronizando: false,
      listo: copia !== null,
      error: null,
      errorLocal,
      conflicto: null,
      recibo: null,
    };
  }

  /** Snapshot estable requerido por useSyncExternalStore. */
  obtenerEstado = (): EstadoCarrito => this.estado;
  /** Detiene futuras peticiones de una cuenta desmontada; no reutiliza la nueva cookie. */
  suspender = () => {
    this.activo = false;
  };
  /** React StrictMode puede volver a montar la misma suscripción. */
  activar = () => {
    this.activo = true;
  };
  /** No inicia trabajo al suscribir: StrictMode no duplica escrituras. */
  suscribir = (oyente: () => void): (() => void) => {
    this.oyentes.add(oyente);
    return () => {
      this.oyentes.delete(oyente);
    };
  };

  /** Publica el estado y hace durable primero cualquier cambio local relevante. */
  private publicar(cambios: Partial<EstadoCarrito>, persistir = false) {
    this.estado = { ...this.estado, ...cambios };
    if (persistir) {
      try {
        if (!this.almacen) throw new Error("Almacenamiento no disponible");
        const { version, items, sucio, intento } = this.estado;
        this.almacen.setItem(
          this.clave,
          JSON.stringify({ version, items, sucio, intento }),
        );
      } catch {
        this.estado = {
          ...this.estado,
          errorLocal:
            "No se pudo guardar la copia del navegador. Los cambios sólo estarán protegidos cuando el servidor confirme la sincronización.",
        };
      }
    }
    for (const oyente of this.oyentes) oyente();
  }

  /** Reconstruye etiquetas desde Catálogo y no atribuye precios a referencias históricas. */
  private itemsRemotos(remoto: CarritoServidor): ItemCarrito[] {
    return remoto.articulos.map((articulo) => {
      const producto = remoto.disponibilidad[articulo.producto_id]?.producto;
      const anterior = this.estado.items.find(
        (elemento) => elemento.product.id === articulo.producto_id,
      )?.product;
      return {
        quantity: articulo.cantidad,
        product: {
          id: articulo.producto_id,
          name:
            producto?.nombre ?? anterior?.name ?? "Producto sin datos actuales",
          brand: producto?.marca ?? anterior?.brand ?? "",
          unit: producto?.unidad ?? anterior?.unit ?? "",
          image: producto?.url_imagen ?? anterior?.image ?? "",
          supermarketId: anterior?.supermarketId ?? "",
          supermarketName: producto?.supermercado ?? anterior?.supermarketName,
          price: producto?.precio ?? null,
          originalPrice: 0,
          inStock:
            remoto.disponibilidad[articulo.producto_id]?.estado_producto ===
            "disponible",
        },
      };
    });
  }

  /** Adopta una versión confirmada; jamás se usa para descartar un borrador en conflicto sin consentimiento. */
  private aceptar(remoto: CarritoServidor, recibo = this.estado.recibo) {
    this.publicar(
      {
        version: remoto.version,
        items: this.itemsRemotos(remoto),
        sucio: false,
        intento: remoto.envio
          ? { ...remoto.envio, version: remoto.version }
          : null,
        listo: true,
        conflicto: null,
        error: null,
        recibo,
      },
      true,
    );
  }

  /** Recupera tras recarga/offline; un desfase conserva ambos borradores en lugar de imponer "último gana". */
  private reconciliar(remoto: CarritoServidor) {
    const local = this.estado;
    if (local.intento) {
      if (remoto.ultimo_envio?.clave === local.intento.clave) {
        this.aceptar(remoto, remoto.ultimo_envio);
      } else if (remoto.envio?.clave === local.intento.clave) {
        this.aceptar(remoto);
      } else if (!remoto.envio && remoto.version === local.intento.version) {
        this.publicar({ listo: true, error: null });
      } else {
        this.publicar({
          conflicto: remoto,
          listo: true,
          error: new Error(
            "El carrito cambió mientras había un envío local pendiente. Revisa el carrito del servidor.",
          ),
        });
      }
    } else if (local.sucio) {
      if (
        !remoto.envio &&
        firmaArticulos(remoto.articulos) ===
          firmaArticulos(articulosDe(local.items))
      ) {
        this.aceptar(remoto);
      } else if (!remoto.envio && remoto.version === local.version) {
        this.publicar({ listo: true, error: null });
      } else {
        this.publicar({
          conflicto: remoto,
          listo: true,
          error: new Error(
            "Hay cambios en otro dispositivo. Conservamos tu borrador local para que elijas qué guardar.",
          ),
        });
      }
    } else {
      this.aceptar(remoto);
    }
  }

  /** Recuperación explícita, al entrar o al reconectar; comparte promesa con otras lecturas/escrituras. */
  recuperar = (): Promise<void> => {
    if (this.enCurso || this.guardandoLista)
      return this.enCurso ?? Promise.resolve();
    this.publicar({ cargando: true });
    const tarea = (async () => {
      try {
        const remoto = await this.api.obtener();
        if (!this.activo) return;
        this.reconciliar(remoto);
        if (this.estado.sucio && !this.estado.conflicto && !this.estado.intento)
          await this.enviarCambios();
      } catch (errorObtenido) {
        this.publicar({ error: errorCarrito(errorObtenido) });
      } finally {
        this.publicar({ cargando: false });
      }
    })();
    this.enCurso = tarea;
    void tarea.finally(() => {
      if (this.enCurso === tarea) this.enCurso = null;
    });
    return tarea;
  };

  /** Edición local protegida antes de enviar; un carrito aún no recuperado no se sustituye por vacío. */
  editar(cambiar: (actuales: ItemCarrito[]) => ItemCarrito[]): boolean {
    if (
      !this.estado.listo ||
      this.estado.intento ||
      this.estado.conflicto ||
      this.guardandoLista
    )
      return false;
    this.publicar(
      { items: cambiar(this.estado.items), sucio: true, recibo: null },
      true,
    );
    void this.sincronizar();
    return true;
  }

  /** Envía las ediciones secuencialmente, conservando cantidades añadidas mientras una petición estaba en curso. */
  private async enviarCambios() {
    this.publicar({ sincronizando: true, error: null });
    try {
      while (
        this.activo &&
        this.estado.sucio &&
        !this.estado.conflicto &&
        !this.estado.intento
      ) {
        const enviados = articulosDe(this.estado.items);
        const respuesta = await this.api.guardar(this.estado.version, enviados);
        if (!this.activo) return;
        if (
          firmaArticulos(enviados) ===
          firmaArticulos(articulosDe(this.estado.items))
        )
          this.aceptar(respuesta);
        else this.publicar({ version: respuesta.version }, true);
      }
    } catch (errorObtenido) {
      this.publicar({ error: errorCarrito(errorObtenido) });
      if (
        this.activo &&
        errorObtenido instanceof ErrorSolicitudListas &&
        errorObtenido.estado === 409
      ) {
        try {
          this.reconciliar(await this.api.obtener());
        } catch {
          /* Conserva el borrador y el error anterior si tampoco se puede leer. */
        }
      }
    } finally {
      this.publicar({ sincronizando: false });
    }
  }

  /** No aplica reintentos infinitos: red fallida requiere reconexión o acción explícita. */
  sincronizar = (): Promise<void> => {
    if (this.enCurso) return this.enCurso;
    if (!this.estado.listo) return this.recuperar();
    if (!this.estado.sucio || this.estado.intento || this.estado.conflicto)
      return Promise.resolve();
    const tarea = this.enviarCambios();
    this.enCurso = tarea;
    void tarea.finally(() => {
      if (this.enCurso === tarea) this.enCurso = null;
    });
    return tarea;
  };

  /** La decisión explícita nunca borra el otro borrador en segundo plano ni mezcla cuentas. */
  resolverConflicto(usarLocal: boolean) {
    const remoto = this.estado.conflicto;
    if (!remoto || this.guardandoLista) return;
    if (!usarLocal) this.aceptar(remoto);
    else if (!remoto.envio && !this.estado.intento) {
      this.publicar(
        { version: remoto.version, conflicto: null, sucio: true, error: null },
        true,
      );
      void this.sincronizar();
    }
  }

  /** Congela clave y destino antes de la red; el servidor confirma y vacía de forma atómica. */
  async confirmar(listaID: string): Promise<ReciboCarrito> {
    if (this.guardandoLista) throw new Error("Hay una confirmación en curso.");
    await this.sincronizar();
    if (!this.activo || this.guardandoLista)
      throw new Error("La cuenta cambió o hay otra confirmación en curso.");
    if (!this.estado.listo || this.estado.sucio || this.estado.conflicto)
      throw new Error(
        "Sincroniza o resuelve el conflicto del carrito antes de confirmar.",
      );
    this.guardandoLista = true;
    this.publicar({ sincronizando: true, error: null });
    try {
      const intento = this.estado.intento ?? {
        lista_id: listaID,
        clave: crypto.randomUUID(),
        version: this.estado.version,
      };
      this.publicar({ intento }, true);
      let remoto = await this.api.preparar(intento.version, intento);
      if (!this.activo)
        throw new Error(
          "La cuenta cambió. El envío se podrá recuperar al volver a iniciar sesión.",
        );
      // Una preparación repetida también puede recuperar un recibo ya confirmado.
      if (remoto.ultimo_envio?.clave !== intento.clave) {
        this.aceptar(remoto);
        remoto = await this.api.confirmar(intento.clave);
      }
      if (remoto.ultimo_envio?.clave !== intento.clave)
        throw new Error(
          "El servicio no confirmó el recibo del envío. Conservamos la clave para reintentar.",
        );
      this.aceptar(remoto, remoto.ultimo_envio);
      return remoto.ultimo_envio;
    } catch (errorObtenido) {
      this.publicar({ error: errorCarrito(errorObtenido) });
      if (
        this.activo &&
        errorObtenido instanceof ErrorSolicitudListas &&
        [400, 404, 409, 422].includes(errorObtenido.estado)
      ) {
        try {
          const remoto = await this.api.obtener();
          if (!remoto.envio && remoto.version === this.estado.intento?.version)
            this.publicar({ intento: null }, true);
          else this.reconciliar(remoto);
        } catch {
          /* No descartar la clave cuando no se puede verificar el servidor. */
        }
      }
      throw errorCarrito(errorObtenido);
    } finally {
      this.guardandoLista = false;
      this.publicar({ sincronizando: false });
    }
  }
}
