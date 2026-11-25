// Prueba el coordinador y las vistas reales con red/Storage controlados; no acredita despliegue.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import ts from "typescript";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";

// ===== Carga aislada y dobles de red =====

/** Transpila código productivo sin sustituir su lógica. */
async function cargar(ruta, dependencias = {}) {
  let fuente = await readFile(new URL(ruta, import.meta.url), "utf8");
  fuente = fuente.replace("import.meta.env.VITE_API_URL", '"/api/v1"');
  for (const [nombre, destino] of Object.entries(dependencias))
    fuente = fuente.replaceAll(`from "${nombre}"`, `from "${destino}"`);
  const { outputText } = ts.transpileModule(fuente, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  const url = modulo(
    outputText.replaceAll(
      '"react/jsx-runtime"',
      JSON.stringify(import.meta.resolve("react/jsx-runtime")),
    ),
  );
  return { url, modulo: await import(url) };
}
/** Módulos pequeños sólo para dependencias controladas. */
function modulo(codigo) {
  return `data:text/javascript;base64,${Buffer.from(codigo).toString("base64")}`;
}
const listas = await cargar("../src/features/listas/listas-api.ts");
const api = await cargar("../src/features/catalogo/carrito-api.ts", {
  "@/features/listas/listas-api": listas.url,
});
const persistencia = await cargar(
  "../src/features/catalogo/carrito-persistente.ts",
  {
    "./carrito-api": api.url,
    "@/features/listas/listas-api": listas.url,
  },
);
const { CarritoPersistente, claveCarrito } = persistencia.modulo;
const { ErrorSolicitudListas } = listas.modulo;
const ID_PRODUCTO = "33333333-3333-4333-8333-333333333333";
const ID_LISTA = "44444444-4444-4444-8444-444444444444";
const producto = {
  id: ID_PRODUCTO,
  name: "Arroz",
  brand: "",
  unit: "1 kg",
  image: "",
  price: 1000,
  originalPrice: 0,
  supermarketId: "",
};
const vacio = () => ({
  version: 0,
  articulos: [],
  envio: null,
  ultimo_envio: null,
  disponibilidad: {},
});
/** Storage realista aislado, nunca usa el perfil del navegador del usuario. */
function almacenMemoria() {
  const filas = new Map();
  return {
    getItem: (clave) => filas.get(clave) ?? null,
    setItem: (clave, valor) => filas.set(clave, valor),
    removeItem: (clave) => filas.delete(clave),
  };
}
/** Simula sólo el contrato remoto; la atomicidad SQL se prueba en integración Go. */
function servidor() {
  let actual = vacio();
  let confirmaciones = 0;
  const escrituras = [];
  const servicios = {
    obtener: async () => structuredClone(actual),
    guardar: async (version, articulos) => {
      escrituras.push(structuredClone({ version, articulos }));
      if (actual.envio || actual.version !== version)
        throw new ErrorSolicitudListas(409, "Conflicto");
      actual = {
        ...actual,
        version: version + 1,
        articulos: structuredClone(articulos),
      };
      return structuredClone(actual);
    },
    preparar: async (version, envio) => {
      if (
        actual.envio?.clave === envio.clave ||
        actual.ultimo_envio?.clave === envio.clave
      )
        return structuredClone(actual);
      if (actual.version !== version)
        throw new ErrorSolicitudListas(409, "Conflicto");
      actual = {
        ...actual,
        version: version + 1,
        envio: { clave: envio.clave, lista_id: envio.lista_id },
      };
      return structuredClone(actual);
    },
    confirmar: async (clave) => {
      if (actual.ultimo_envio?.clave === clave) return structuredClone(actual);
      assert.equal(actual.envio.clave, clave);
      confirmaciones++;
      actual = {
        ...actual,
        version: actual.version + 1,
        articulos: [],
        envio: null,
        ultimo_envio: { clave, lista_id: ID_LISTA, aplicado: true },
      };
      return structuredClone(actual);
    },
  };
  return {
    servicios,
    escrituras,
    get actual() {
      return actual;
    },
    set actual(valor) {
      actual = valor;
    },
    get confirmaciones() {
      return confirmaciones;
    },
  };
}
// ===== Persistencia y concurrencia =====

test("el cliente usa cookie, versión y clave, sin dueño ni datos de producto en el cuerpo", async () => {
  const original = globalThis.fetch;
  const llamadas = [];
  globalThis.fetch = async (url, opciones) => {
    llamadas.push({ url, opciones });
    return new Response(JSON.stringify(vacio()), { status: 200 });
  };
  try {
    const cliente = api.modulo.crearCarritoApi("cuenta");
    await cliente.guardar(3, [{ producto_id: ID_PRODUCTO, cantidad: 2 }]);
    await cliente.preparar(4, { lista_id: ID_LISTA, clave: "operacion" });
    await cliente.confirmar("operacion");
    assert.equal(llamadas[0].url, "/api/v1/listas/carrito");
    assert.equal(llamadas[0].opciones.method, "PUT");
    assert.equal(llamadas[0].opciones.credentials, "include");
    assert.equal(llamadas[0].opciones.headers["X-Carrito-Cuenta"], "cuenta");
    assert.deepEqual(JSON.parse(llamadas[0].opciones.body), {
      version: 3,
      articulos: [{ producto_id: ID_PRODUCTO, cantidad: 2 }],
    });
    assert.equal(llamadas[2].url, "/api/v1/listas/carrito/confirmar");
    assert.deepEqual(JSON.parse(llamadas[2].opciones.body), {
      clave: "operacion",
    });
  } finally {
    globalThis.fetch = original;
  }
});

test("recarga y otra instancia recuperan el carrito; una cuenta distinta no lee la copia local", async () => {
  const local = almacenMemoria(),
    remoto = servidor();
  const primero = new CarritoPersistente("cuenta-a", local, remoto.servicios);
  await primero.recuperar();
  primero.editar(() => [{ product: producto, quantity: 2 }]);
  await primero.sincronizar();
  const recargado = new CarritoPersistente("cuenta-a", local, remoto.servicios);
  assert.equal(recargado.obtenerEstado().items[0].quantity, 2);
  assert.equal(recargado.obtenerEstado().items[0].product.price, null);
  await recargado.recuperar();
  assert.equal(recargado.obtenerEstado().sucio, false);
  const otraCuenta = new CarritoPersistente(
    "cuenta-b",
    local,
    remoto.servicios,
  );
  assert.deepEqual(otraCuenta.obtenerEstado().items, []);
});

test("offline conserva el borrador al recargar y lo sincroniza al volver la conexión", async () => {
  const local = almacenMemoria(),
    remoto = servidor();
  const primero = new CarritoPersistente("cuenta", local, remoto.servicios);
  await primero.recuperar();
  const guardar = remoto.servicios.guardar;
  remoto.servicios.guardar = async () => {
    throw new TypeError("Offline");
  };
  primero.editar(() => [{ product: producto, quantity: 3 }]);
  await primero.sincronizar();
  assert.equal(primero.obtenerEstado().sucio, true);
  const recargado = new CarritoPersistente("cuenta", local, remoto.servicios);
  assert.equal(recargado.obtenerEstado().items[0].quantity, 3);
  remoto.servicios.guardar = guardar;
  await recargado.recuperar();
  assert.equal(remoto.actual.articulos[0].cantidad, 3);
  assert.equal(recargado.obtenerEstado().sucio, false);
});

test("un PUT aplicado con respuesta perdida se reconoce al recuperar, sin sobrescribir ni duplicar", async () => {
  const local = almacenMemoria(),
    remoto = servidor();
  const primero = new CarritoPersistente("cuenta", local, remoto.servicios);
  await primero.recuperar();
  const guardar = remoto.servicios.guardar;
  remoto.servicios.guardar = async (...datos) => {
    await guardar(...datos);
    throw new TypeError("Respuesta perdida");
  };
  primero.editar(() => [{ product: producto, quantity: 2 }]);
  await primero.sincronizar();
  const recargado = new CarritoPersistente("cuenta", local, remoto.servicios);
  await recargado.recuperar();
  assert.equal(recargado.obtenerEstado().sucio, false);
  assert.equal(remoto.escrituras.length, 1);
});

test("editar mientras se sincroniza conserva la cantidad más reciente y serializa los PUT", async () => {
  const remoto = servidor();
  const guardar = remoto.servicios.guardar;
  let liberar;
  remoto.servicios.guardar = async (...datos) => {
    if (!liberar)
      await new Promise((resolver) => {
        liberar = resolver;
      });
    return guardar(...datos);
  };
  const carrito = new CarritoPersistente(
    "cuenta",
    almacenMemoria(),
    remoto.servicios,
  );
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 1 }]);
  carrito.editar(() => [{ product: producto, quantity: 4 }]);
  liberar();
  await carrito.sincronizar();
  assert.equal(remoto.escrituras.length, 2);
  assert.equal(remoto.actual.articulos[0].cantidad, 4);
  assert.equal(carrito.obtenerEstado().sucio, false);
});

test("un conflicto conserva ambas versiones hasta una decisión explícita", async () => {
  const remoto = servidor(),
    local = almacenMemoria();
  const carrito = new CarritoPersistente("cuenta", local, remoto.servicios);
  await carrito.recuperar();
  const guardar = remoto.servicios.guardar;
  remoto.servicios.guardar = async () => {
    throw new TypeError("Offline");
  };
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  remoto.actual = {
    ...vacio(),
    version: 5,
    articulos: [{ producto_id: ID_PRODUCTO, cantidad: 8 }],
  };
  remoto.servicios.guardar = guardar;
  await carrito.recuperar();
  assert.equal(carrito.obtenerEstado().items[0].quantity, 2);
  assert.equal(carrito.obtenerEstado().conflicto.articulos[0].cantidad, 8);
  assert.equal(remoto.escrituras.length, 0);
  carrito.resolverConflicto(true);
  await carrito.sincronizar();
  assert.equal(remoto.actual.articulos[0].cantidad, 2);
  assert.equal(carrito.obtenerEstado().conflicto, null);
});

test("una preparación con respuesta perdida mantiene la misma clave después de recargar", async () => {
  const remoto = servidor(),
    local = almacenMemoria();
  const carrito = new CarritoPersistente("cuenta", local, remoto.servicios);
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  const preparar = remoto.servicios.preparar;
  remoto.servicios.preparar = async (...datos) => {
    await preparar(...datos);
    throw new TypeError("Corte");
  };
  await assert.rejects(carrito.confirmar(ID_LISTA));
  const clave = remoto.actual.envio.clave;
  assert.equal(
    carrito.editar(() => []),
    false,
  );
  remoto.servicios.preparar = preparar;
  const recargado = new CarritoPersistente("cuenta", local, remoto.servicios);
  await recargado.recuperar();
  assert.equal(recargado.obtenerEstado().intento.clave, clave);
  const recibo = await recargado.confirmar(ID_LISTA);
  assert.equal(recibo.clave, clave);
  assert.equal(recibo.aplicado, true);
  assert.equal(remoto.confirmaciones, 1);
  assert.deepEqual(recargado.obtenerEstado().items, []);
});

test("confirmación aplicada con respuesta perdida se recupera sin volver a sumar", async () => {
  const remoto = servidor(),
    local = almacenMemoria();
  const carrito = new CarritoPersistente("cuenta", local, remoto.servicios);
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  const confirmar = remoto.servicios.confirmar;
  remoto.servicios.confirmar = async (...datos) => {
    await confirmar(...datos);
    throw new TypeError("Corte");
  };
  await assert.rejects(carrito.confirmar(ID_LISTA));
  const recargado = new CarritoPersistente("cuenta", local, remoto.servicios);
  await recargado.recuperar();
  assert.equal(recargado.obtenerEstado().recibo.aplicado, true);
  assert.equal(recargado.obtenerEstado().intento, null);
  assert.deepEqual(recargado.obtenerEstado().items, []);
  assert.equal(remoto.confirmaciones, 1);
});

test("otra instancia sin copia local recupera un envío pendiente del servidor", async () => {
  const remoto = servidor();
  remoto.actual = {
    ...vacio(),
    version: 2,
    articulos: [{ producto_id: ID_PRODUCTO, cantidad: 2 }],
    envio: { lista_id: ID_LISTA, clave: "persistida" },
  };
  const carrito = new CarritoPersistente(
    "cuenta",
    almacenMemoria(),
    remoto.servicios,
  );
  await carrito.recuperar();
  assert.equal(carrito.obtenerEstado().intento.clave, "persistida");
  await carrito.confirmar(ID_LISTA);
  assert.equal(remoto.confirmaciones, 1);
});

test("rechazo de negocio conserva productos y desbloquea el borrador", async () => {
  const remoto = servidor();
  const carrito = new CarritoPersistente(
    "cuenta",
    almacenMemoria(),
    remoto.servicios,
  );
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  remoto.servicios.confirmar = async (clave) => ({
    ...remoto.actual,
    envio: null,
    ultimo_envio: {
      lista_id: ID_LISTA,
      clave,
      aplicado: false,
      motivo: "max_articulos",
    },
  });
  const recibo = await carrito.confirmar(ID_LISTA);
  assert.equal(recibo.aplicado, false);
  assert.equal(carrito.obtenerEstado().items.length, 1);
  assert.equal(carrito.obtenerEstado().intento, null);
});

test("almacenamiento denegado y JSON inválido se informan sin inventar un carrito remoto vacío", async () => {
  const local = almacenMemoria(),
    remoto = servidor();
  local.setItem(claveCarrito("cuenta"), "{roto");
  const carrito = new CarritoPersistente("cuenta", local, remoto.servicios);
  assert.ok(carrito.obtenerEstado().errorLocal);
  assert.equal(
    local.getItem(`${claveCarrito("cuenta")}.recuperacion`),
    "{roto",
  );
  const sinStorage = new CarritoPersistente("cuenta", null, remoto.servicios);
  await sinStorage.recuperar();
  assert.ok(sinStorage.obtenerEstado().errorLocal);
  for (const datos of [
    null,
    {},
    { ...vacio(), articulos: null },
    { ...vacio(), envio: {} },
  ])
    assert.throws(() => api.modulo.validarCarrito(datos));
});

test("desmontar la cuenta detiene el envío de ediciones encoladas con una cookie posterior", async () => {
  const remoto = servidor();
  let liberar;
  let escrituras = 0;
  remoto.servicios.guardar = async () => {
    escrituras++;
    await new Promise((resolver) => {
      liberar = resolver;
    });
    return { ...vacio(), version: 1 };
  };
  const carrito = new CarritoPersistente(
    "cuenta",
    almacenMemoria(),
    remoto.servicios,
  );
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 1 }]);
  carrito.editar(() => [{ product: producto, quantity: 4 }]);
  carrito.suspender();
  liberar();
  await carrito.sincronizar();
  assert.equal(escrituras, 1);
});

test("un 401 conserva el borrador y la preparación sin reenviar con otra clave", async () => {
  const remoto = servidor(),
    local = almacenMemoria();
  const carrito = new CarritoPersistente("cuenta", local, remoto.servicios);
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  remoto.servicios.confirmar = async () => {
    throw new ErrorSolicitudListas(401, "Sesión vencida");
  };
  await assert.rejects(carrito.confirmar(ID_LISTA));
  const clave = carrito.obtenerEstado().intento.clave;
  const recargado = new CarritoPersistente("cuenta", local, remoto.servicios);
  assert.equal(recargado.obtenerEstado().intento.clave, clave);
  assert.equal(recargado.obtenerEstado().items[0].quantity, 2);
  assert.equal(remoto.confirmaciones, 0);
});

test("doble confirmación no duplica peticiones y un envío remoto no se puede sobrescribir", async () => {
  const remoto = servidor();
  const carrito = new CarritoPersistente(
    "cuenta",
    almacenMemoria(),
    remoto.servicios,
  );
  await carrito.recuperar();
  carrito.editar(() => [{ product: producto, quantity: 2 }]);
  await carrito.sincronizar();
  const resultados = await Promise.allSettled([
    carrito.confirmar(ID_LISTA),
    carrito.confirmar(ID_LISTA),
  ]);
  assert.equal(
    resultados.filter((resultado) => resultado.status === "fulfilled").length,
    1,
  );
  assert.equal(remoto.confirmaciones, 1);
  remoto.actual = {
    ...vacio(),
    version: 5,
    envio: { lista_id: ID_LISTA, clave: "remota" },
    articulos: [{ producto_id: ID_PRODUCTO, cantidad: 6 }],
  };
  await carrito.recuperar();
  assert.equal(
    carrito.editar(() => []),
    false,
  );
  assert.equal(carrito.obtenerEstado().items[0].quantity, 6);
});

// ===== Vista real de confirmación =====

const urlEstado = modulo(`
  export const estado = { user: { id: "cuenta" }, guardado: {} };
  export const useAuth = () => ({ user: estado.user });
  export const useCart = () => ({ guardado: estado.guardado, items: [], setIsCartOpen() {} });
`);
const { estado: estadoVista } = await import(urlEstado);
const vista = await cargar("../src/features/catalogo/GuardarCarrito.tsx", {
  "react-router-dom": import.meta.resolve("react-router-dom"),
  "@/features/autenticacion/useAuth": urlEstado,
  "./CartContext": urlEstado,
  "@/features/listas/listas-api": listas.url,
  "@/app/routes": modulo(
    'export const ROUTES = { LOGIN: "/login", LISTAS: "/listas" };',
  ),
  "@/components/ui/button": modulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export function Button({ children, variant, ...propiedades }) { return createElement("button", propiedades, children); }
  `),
  "@/components/ui/input": modulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export const Input = propiedades => createElement("input", propiedades);
  `),
});

/** Renderiza sin red para verificar los mensajes de recuperación. */
function renderizarVista() {
  return renderToStaticMarkup(
    createElement(
      MemoryRouter,
      null,
      createElement(vista.modulo.GuardarCarrito),
    ),
  );
}

test("la vista explica la suma, la clave recuperable y el acceso a la lista confirmada", () => {
  estadoVista.guardado = {
    abierto: true,
    listas: [],
    nombre: "",
    destino: ID_LISTA,
  };
  assert.match(renderizarVista(), /se sumarán sus cantidades/);
  estadoVista.guardado.pendiente = true;
  assert.match(
    renderizarVista(),
    /puede recuperarse al volver a iniciar sesión/,
  );
  estadoVista.guardado.resultado = { id: ID_LISTA };
  const contenido = renderizarVista();
  assert.match(contenido, /Productos guardados/);
  assert.match(contenido, /sin precio vigente/);
  assert.match(contenido, new RegExp(`href="/listas\\?lista=${ID_LISTA}"`));
});
