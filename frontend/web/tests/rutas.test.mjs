// Verifica cliente, coordinación y presentación reales con HTTP controlado, sin BD ni claves.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import ts from "typescript";

// ===== Carga del código real =====

/** Aísla imports de prueba sin añadir dependencias o mocks al runtime. */
const crearModulo = (codigo) =>
  `data:text/javascript;base64,${Buffer.from(codigo).toString("base64")}`;

/** Transpila únicamente para Node y sustituye las dependencias indicadas. */
async function cargar(ruta, dependencias = {}) {
  let fuente = await readFile(new URL(ruta, import.meta.url), "utf8");
  fuente = fuente.replace("import.meta.env.VITE_API_URL", '"/api/v1/"');
  for (const [nombre, destino] of Object.entries(dependencias)) {
    fuente = fuente
      .replaceAll(`from '${nombre}'`, `from '${destino}'`)
      .replaceAll(`from "${nombre}"`, `from "${destino}"`);
  }
  const { outputText } = ts.transpileModule(fuente, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  const url = crearModulo(
    outputText.replaceAll(
      '"react/jsx-runtime"',
      JSON.stringify(import.meta.resolve("react/jsx-runtime")),
    ),
  );
  return { url, modulo: await import(url) };
}

const ejemplo = JSON.parse(
  await readFile(
    new URL("./fixtures/ruta-dos-paradas.json", import.meta.url),
    "utf8",
  ),
);
const api = await cargar("../src/features/rutas/rutas-api.ts");
const { rutasApi, ErrorSolicitudRutas, mensajeErrorRuta } = api.modulo;
const listas = await cargar("../src/features/listas/listas-api.ts");
const formato = await cargar("../src/features/catalogo/formatters.ts");
const resultado = await cargar("../src/features/rutas/ResultadoRuta.tsx", {
  "@/features/catalogo/formatters": formato.url,
});
const { ResultadoRuta, enlaceGoogleMaps } = resultado.modulo;

// ===== Cliente público =====

test("optimización usa POST, Gateway, cookie y solo lista_id; conserva toda la respuesta", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", async (url, opciones) => {
    assert.equal(url, "/api/v1/rutas/optimizar");
    assert.equal(opciones.method, "POST");
    assert.equal(opciones.credentials, "include");
    assert.equal(opciones.headers["Content-Type"], "application/json");
    assert.deepEqual(JSON.parse(opciones.body), { lista_id: "lista-real" });
    assert.equal(opciones.headers.Authorization, undefined);
    return Response.json(ejemplo);
  });
  assert.deepEqual(
    await rutasApi.optimizar({ lista_id: "lista-real" }),
    ejemplo,
  );
});

test("acepta origen opcional explícito o null, sin agregar campos históricos", async (prueba) => {
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (_url, opciones) => {
    llamadas.push(JSON.parse(opciones.body));
    return Response.json(ejemplo);
  });
  await rutasApi.optimizar({ lista_id: "lista", origen: { lat: 0, lon: 0 } });
  await rutasApi.optimizar({ lista_id: "lista", origen: null });
  assert.deepEqual(llamadas, [
    { lista_id: "lista", origen: { lat: 0, lon: 0 } },
    { lista_id: "lista", origen: null },
  ]);
});

test("conserva código y detalle de errores; solo 401 solicita iniciar sesión", async (prueba) => {
  for (const estado of [400, 401, 404, 422, 503, 502, 504]) {
    const simulacion = prueba.mock.method(globalThis, "fetch", async () =>
      Response.json(
        {
          mensaje: "La lista no tiene artículos por comprar.",
          detalle: "causa",
        },
        { status: estado },
      ),
    );
    await assert.rejects(
      rutasApi.optimizar({ lista_id: "lista" }),
      (errorObtenido) => {
        assert.ok(errorObtenido instanceof ErrorSolicitudRutas);
        assert.equal(errorObtenido.estado, estado);
        assert.equal(errorObtenido.detalle, "causa");
        assert.equal(
          mensajeErrorRuta(errorObtenido).includes("sesión"),
          estado === 401,
        );
        return true;
      },
    );
    simulacion.mock.restore();
  }
});

test("distingue causas 422 y tolera errores no JSON sin fabricar una recomendación", async (prueba) => {
  for (const mensaje of [
    "La lista no tiene artículos por comprar.",
    "La lista supera el máximo de artículos para calcular una ruta.",
    "No hay supermercados cercanos para calcular una ruta.",
    "No se encontró una forma de comprar la lista en las tiendas cercanas.",
  ]) {
    assert.equal(
      mensajeErrorRuta(new ErrorSolicitudRutas(422, mensaje)),
      mensaje,
    );
  }
  prueba.mock.method(
    globalThis,
    "fetch",
    async () => new Response("<html>Bad gateway</html>", { status: 502 }),
  );
  await assert.rejects(
    rutasApi.optimizar({ lista_id: "lista" }),
    (errorObtenido) =>
      errorObtenido.estado === 502 &&
      /conectar/.test(mensajeErrorRuta(errorObtenido)),
  );
});

test("propaga cancelación y diferencia un fallo de red de una sesión vencida", async (prueba) => {
  const controlador = new AbortController();
  controlador.abort();
  const simulacion = prueba.mock.method(
    globalThis,
    "fetch",
    async (_url, opciones) => {
      assert.equal(opciones.signal, controlador.signal);
      opciones.signal.throwIfAborted();
    },
  );
  await assert.rejects(
    rutasApi.optimizar({ lista_id: "lista" }, controlador.signal),
    { name: "AbortError" },
  );
  simulacion.mock.restore();
  prueba.mock.method(globalThis, "fetch", async () => {
    throw new TypeError("Failed to fetch");
  });
  await assert.rejects(
    rutasApi.optimizar({ lista_id: "lista" }),
    (errorObtenido) =>
      /conectar/.test(mensajeErrorRuta(errorObtenido)) &&
      !(errorObtenido instanceof ErrorSolicitudRutas),
  );
});

// ===== Presentación =====

/** Renderiza el componente real sin ejecutar peticiones o iniciar una sesión ficticia. */
const presentar = (datos, nombres) =>
  renderToStaticMarkup(createElement(ResultadoRuta, { datos, nombres }));

test("presenta dos paradas, montos confirmados, origen y avisos de estimación y compra parcial", () => {
  const html = presentar(ejemplo, {
    [ejemplo.no_cubiertos[0].producto_id]: "Quinoa",
  });
  assert.ok(html.indexOf("A Cuenta") < html.indexOf("2. Cugat"));
  for (const texto of [
    "8.670",
    "920",
    "9.590",
    "2,92",
    "2.180",
    "6.490",
    "1.090",
    "Arroz grado 1",
    "Detergente",
    "2 envases",
    "centro de Temuco",
    "retorno al origen",
    "no representan un recorrido por calles",
    "no incluye combustible",
    "no están incluidos en el total",
    "Quinoa",
  ])
    assert.ok(html.includes(texto), texto);
  assert.equal((html.match(/Abrir en Google Maps<\/a>/g) ?? []).length, 2);
  assert.ok(html.includes('rel="noopener noreferrer"'));
  assert.ok(html.includes('target="_blank"'));
});

test("una parada muestra ahorro cero, sin inventar un ahorro extra", () => {
  const datos = structuredClone(ejemplo);
  datos.ruta.paradas = [datos.ruta.paradas[1]];
  datos.ruta.total = datos.base_una_tienda.total;
  datos.ahorro = 0;
  datos.no_cubiertos = [];
  const html = presentar(datos);
  assert.match(html, /Conviene comprar en una sola tienda/);
  assert.match(html, /\$0/);
  assert.doesNotMatch(html, /Compra parcial/);
});

test("sin base comparativa no presenta ahorro cero y conserva referencias sin nombre", () => {
  const datos = { ...ejemplo, base_una_tienda: null, ahorro: null };
  const html = presentar(datos);
  assert.match(html, /Sin comparación disponible/);
  assert.match(html, /Ninguna tienda sola/);
  assert.doesNotMatch(html, /\$0/);
  assert.ok(html.includes(`Producto ${datos.no_cubiertos[0].producto_id}`));
});

test("Maps usa coordenadas, admite cero y omite ubicaciones inválidas", () => {
  const enlace = new URL(enlaceGoogleMaps(-38.7338383, -72.5806422));
  assert.equal(enlace.origin, "https://www.google.com");
  assert.equal(enlace.searchParams.get("api"), "1");
  assert.equal(enlace.searchParams.get("query"), "-38.7338383,-72.5806422");
  assert.ok(enlaceGoogleMaps(0, 0));
  for (const coordenadas of [
    [NaN, 0],
    [0, Infinity],
    [91, 0],
    [0, -181],
  ])
    assert.equal(enlaceGoogleMaps(...coordenadas), null);
});

// ===== Ciclo controlado del hook =====

const reactHooks = crearModulo(`
  const estados = [], efectos = [], pendientes = [];
  let indiceEstado = 0, indiceEfecto = 0;
  export function desmontar() {
    for (const efecto of efectos) efecto?.limpiar?.();
    efectos.length = 0;
  }
  export function preparar(reiniciar = false) {
    if (reiniciar) { desmontar(); estados.length = 0; pendientes.length = 0; }
    indiceEstado = indiceEfecto = 0;
  }
  export function useState(inicial) {
    const indice = indiceEstado++;
    if (indice === estados.length) estados.push(inicial);
    return [estados[indice], valor => { estados[indice] = typeof valor === 'function' ? valor(estados[indice]) : valor; }];
  }
  export function useEffect(accion, dependencias) {
    const indice = indiceEfecto++, anterior = efectos[indice];
    if (!anterior || dependencias.some((valor, posicion) => !Object.is(valor, anterior.dependencias[posicion]))) {
      pendientes.push(() => {
        anterior?.limpiar?.();
        efectos[indice] = { dependencias, limpiar: accion() };
      });
    }
  }
  export function confirmar() { for (const pendiente of pendientes.splice(0)) pendiente(); }
`);
const ciclo = await import(reactHooks);
const hook = await cargar("../src/features/rutas/useRutaLista.ts", {
  react: reactHooks,
  "@/features/listas/listas-api": listas.url,
  "./rutas-api": api.url,
});
const { useRutaLista } = hook.modulo;

/** Simula un render y la confirmación de sus efectos como los demás tests de hooks del proyecto. */
function renderHook(listaID, reiniciar = false) {
  ciclo.preparar(reiniciar);
  const estado = useRutaLista(listaID);
  ciclo.confirmar();
  return estado;
}

/** Resuelve las promesas pendientes sin esperar relojes o servicios externos. */
const resolver = () => new Promise((continuar) => setImmediate(continuar));

test("sin lista no consulta; éxito permite recalcular y el fallo secundario no invalida la recomendación", async (prueba) => {
  let consultas = 0;
  prueba.mock.method(globalThis, "fetch", async (url) => {
    consultas++;
    if (url.includes("/listas/"))
      throw new TypeError("lectura complementaria caída");
    return Response.json(ejemplo);
  });
  assert.equal(renderHook(null, true).estado.tipo, "sin_lista");
  assert.equal(consultas, 0);
  assert.equal(renderHook("lista", true).estado.tipo, "cargando");
  await resolver();
  let estado = renderHook("lista");
  assert.equal(estado.estado.tipo, "resultado");
  assert.deepEqual(estado.nombres, {});
  assert.equal(consultas, 2);
  estado.recalcular();
  renderHook("lista");
  assert.equal(renderHook("lista").estado.tipo, "cargando");
  await resolver();
  estado = renderHook("lista");
  assert.equal(estado.estado.tipo, "resultado");
  assert.equal(consultas, 4);
  ciclo.desmontar();
});

test("carga nombres de no cubiertos y no consulta detalles cuando la cobertura es completa", async (prueba) => {
  const productoID = ejemplo.no_cubiertos[0].producto_id;
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (url) => {
    llamadas.push(url);
    return Response.json(
      url.includes("/listas/")
        ? {
            articulos: [
              { producto_id: productoID, producto: { nombre: "Quinoa" } },
            ],
          }
        : ejemplo,
    );
  });
  renderHook("lista", true);
  await resolver();
  assert.equal(renderHook("lista").nombres[productoID], "Quinoa");
  ciclo.desmontar();
  prueba.mock.method(globalThis, "fetch", async (url) => {
    llamadas.push(url);
    return Response.json({ ...ejemplo, no_cubiertos: [] });
  });
  renderHook("otra", true);
  await resolver();
  assert.equal(llamadas.length, 3);
  ciclo.desmontar();
});

test("descarta respuestas antiguas incluso cuando fetch ignora la cancelación", async (prueba) => {
  const pendientes = [];
  prueba.mock.method(
    globalThis,
    "fetch",
    (url, opciones) =>
      new Promise((resolverPeticion) =>
        pendientes.push({ url, opciones, resolverPeticion }),
      ),
  );
  renderHook("anterior", true);
  renderHook("actual");
  assert.equal(pendientes[0].opciones.signal.aborted, true);
  pendientes[1].resolverPeticion(
    Response.json({ ...ejemplo, no_cubiertos: [], ahorro: 1000 }),
  );
  await resolver();
  pendientes[0].resolverPeticion(Response.json(ejemplo));
  await resolver();
  assert.equal(renderHook("actual").estado.datos.ahorro, 1000);
  ciclo.desmontar();
});

test("al desmontar por cambio de cuenta cancela peticiones y no conserva sus datos", async (prueba) => {
  let completar;
  let senal;
  prueba.mock.method(globalThis, "fetch", (_url, opciones) => {
    senal = opciones.signal;
    return new Promise((continuar) => {
      completar = continuar;
    });
  });
  renderHook("lista", true);
  ciclo.desmontar();
  assert.equal(senal.aborted, true);
  completar(Response.json(ejemplo));
  await resolver();
  assert.equal(renderHook(null, true).estado.tipo, "sin_lista");
  ciclo.desmontar();
});

test("un error de optimización publica error sin consultar detalles ni simular éxito", async (prueba) => {
  let llamadas = 0;
  prueba.mock.method(globalThis, "fetch", async () => {
    llamadas++;
    return Response.json(
      { mensaje: "La lista no tiene artículos por comprar." },
      { status: 422 },
    );
  });
  renderHook("lista", true);
  await resolver();
  const { estado } = renderHook("lista");
  assert.equal(estado.tipo, "error");
  assert.equal(estado.error.estado, 422);
  assert.equal(llamadas, 1);
  ciclo.desmontar();
});

// ===== Página y navegación =====

const pagina = await cargar("../src/features/rutas/RutaLista.tsx", {
  "react-router-dom": import.meta.resolve("react-router-dom"),
  "@/app/routes": crearModulo(
    'export const ROUTES = { LOGIN: "/login", LISTAS: "/listas" };',
  ),
  "@/components/ui/button": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export const Button = ({ children, variant, ...propiedades }) => createElement('button', propiedades, children);
  `),
  "@/features/autenticacion/useAuth": crearModulo(
    'export const useAuth = () => ({ user: { id: "cuenta" } });',
  ),
  "./rutas-api": api.url,
  "./ResultadoRuta": resultado.url,
  "./useRutaLista": crearModulo(
    'export const useRutaLista = () => ({ estado: { tipo: "sin_lista" }, nombres: {}, recalcular() {} });',
  ),
});

/** Renderiza estados explícitos sin manipular el orden interno de hooks de la página. */
function presentarEstado(estado) {
  return renderToStaticMarkup(
    createElement(
      MemoryRouter,
      null,
      createElement(pagina.modulo.EstadoRecomendacion, {
        estado,
        nombres: {},
        recalcular() {},
      }),
    ),
  );
}

test("página muestra carga, ausencia de selección y acciones adecuadas a cada error", () => {
  assert.match(presentarEstado({ tipo: "cargando" }), /role="status"/);
  assert.match(presentarEstado({ tipo: "sin_lista" }), /Selecciona una lista/);
  for (const codigo of [400, 401, 404, 422, 503, 502]) {
    const html = presentarEstado({
      tipo: "error",
      error: new ErrorSolicitudRutas(codigo, "Sin precios disponibles."),
    });
    assert.match(html, /role="alert"/);
    assert.equal(html.includes('href="/login"'), codigo === 401);
    assert.equal(
      html.includes("Reintentar cálculo"),
      codigo === 503 || codigo === 502,
    );
    if (codigo === 422) assert.match(html, /Sin precios disponibles/);
  }
});

test("la página conserva lista al volver y la ruta sigue bajo ProtectedRoute", async () => {
  const html = renderToStaticMarkup(
    createElement(
      MemoryRouter,
      { initialEntries: ["/route?lista=lista-real"] },
      createElement(pagina.modulo.default),
    ),
  );
  assert.match(html, /href="\/listas\?lista=lista-real"/);
  const router = await readFile(
    new URL("../src/app/router.tsx", import.meta.url),
    "utf8",
  );
  assert.match(
    router,
    /element: <ProtectedRoute \/>[\s\S]*path: ROUTES\.RUTA,[\s\S]*element: <RutaLista \/>/,
  );
  // Un merge con la antigua pantalla no debe reintroducir el archivo retirado y romper Vite.
  assert.doesNotMatch(router, /@\/pages\/RouteViewer/);
  const listasFuente = await readFile(
    new URL("../src/features/listas/ShoppingLists.tsx", import.meta.url),
    "utf8",
  );
  assert.match(
    listasFuente,
    /disabled=\{guardando\}[\s\S]*ROUTES\.RUTA\}\?lista=\$\{encodeURIComponent\(lista\.id\)\}/,
  );
  assert.match(listasFuente, /Calcular ruta/);
  const paginaFuente = await readFile(
    new URL("../src/features/rutas/RutaLista.tsx", import.meta.url),
    "utf8",
  );
  assert.match(paginaFuente, /key=\{`\$\{user\?\.id/);
});

test("Calcular ruta navega con la lista persistida y se bloquea al guardar", async () => {
  const datosLista = {
    id: "lista/real",
    nombre: "Semana",
    estado: "activa",
    total_articulos: 1,
    articulos: [],
  };
  const entorno = crearModulo(`
    export const escenario = { indice: 0, guardando: false, botones: [], destinos: [] };
    export const lista = ${JSON.stringify(datosLista)};
  `);
  const { escenario } = await import(entorno);
  const listasVista = await cargar("../src/features/listas/ShoppingLists.tsx", {
    react: crearModulo(`
      import { escenario, lista } from ${JSON.stringify(entorno)};
      export const useEffect = () => {};
      export const useRef = valor => ({ current: valor });
      export const useState = () => {
        const valores = [[lista], lista.id, "", false, false, false, null, 0];
        return [valores[escenario.indice++], () => {}];
      };
    `),
    "react-router-dom": crearModulo(`
      import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
      import { escenario } from ${JSON.stringify(entorno)};
      export const useSearchParams = () => [new URLSearchParams()];
      export const useNavigate = () => destino => escenario.destinos.push(destino);
      export const Link = ({ to, children, ...propiedades }) => createElement("a", { href: to, ...propiedades }, children);
    `),
    "@/components/ui/button": crearModulo(`
      import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
      import { escenario } from ${JSON.stringify(entorno)};
      export const Button = ({ children, variant, ...propiedades }) => {
        escenario.botones.push({ children, ...propiedades });
        return createElement("button", propiedades, children);
      };
    `),
    "@/components/ui/input": crearModulo(`
      import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
      export const Input = propiedades => createElement("input", propiedades);
    `),
    "@/features/autenticacion/useAuth": crearModulo(
      'export const useAuth = () => ({ user: { id: "cuenta" } });',
    ),
    "@/components/ToastContext": crearModulo(
      "export const useToast = () => ({ toast() {} });",
    ),
    "@/app/routes": crearModulo(
      'export const ROUTES = { LOGIN: "/login", RUTA: "/route" };',
    ),
    "@/features/listas/listas-api": listas.url,
    "@/features/listas/ArticulosLista": crearModulo(
      "export const ArticulosLista = () => null;",
    ),
    "@/features/listas/useDetalleLista": crearModulo(`
      import { escenario, lista } from ${JSON.stringify(entorno)};
      export const useDetalleLista = () => ({
        lista, nombre: lista.nombre, estado: lista.estado, cargando: false,
        guardando: escenario.guardando, error: null,
      });
    `),
  });
  renderToStaticMarkup(createElement(listasVista.modulo.default));
  const boton = escenario.botones.find(
    (elemento) => elemento.children === "Calcular ruta",
  );
  assert.equal(boton.disabled, false);
  boton.onClick();
  assert.deepEqual(escenario.destinos, ["/route?lista=lista%2Freal"]);
  escenario.indice = 0;
  escenario.botones = [];
  escenario.guardando = true;
  renderToStaticMarkup(createElement(listasVista.modulo.default));
  assert.equal(
    escenario.botones.find((elemento) => elemento.children === "Calcular ruta")
      .disabled,
    true,
  );
});
