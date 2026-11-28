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
  // El CSS de Leaflet lo procesa Vite; Node solo verifica el componente sin DOM.
  fuente = fuente.replace('import "leaflet/dist/leaflet.css";', "");
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
const LISTA_ID = "3f6c2a10-8b1e-4d2f-9a3c-7e5d4c3b2a10";
const OTRA_LISTA_ID = "4f6c2a10-8b1e-4d2f-9a3c-7e5d4c3b2a11";
const api = await cargar("../src/features/rutas/rutas-api.ts");
const {
  rutasApi,
  ErrorSolicitudRutas,
  mensajeErrorRuta,
  esListaIDValida,
  productosSinCobertura,
  orientacionErrorRuta,
} = api.modulo;
const listas = await cargar("../src/features/listas/listas-api.ts");
const formato = await cargar("../src/features/catalogo/formatters.ts");
const presentacion = await cargar("../src/features/rutas/presentacion-ruta.ts");
const { agruparArticulosParada } = presentacion.modulo;
const resultado = await cargar("../src/features/rutas/ResultadoRuta.tsx", {
  "lucide-react": import.meta.resolve("lucide-react"),
  "@/features/catalogo/formatters": formato.url,
  "./presentacion-ruta": presentacion.url,
});
const { ResultadoRuta, enlaceGoogleMaps } = resultado.modulo;
const selectorOrigen = await cargar(
  "../src/features/rutas/SelectorOrigen.tsx",
  {
    react: import.meta.resolve("react"),
    "@/components/ui/button": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export const Button = ({ children, variant, ...propiedades }) => createElement("button", propiedades, children);
  `),
  },
);

test("el selector manual exige marcar y confirmar sin solicitar ubicación al renderizar", () => {
  const html = renderToStaticMarkup(
    createElement(selectorOrigen.modulo.SelectorOrigen, {
      inicial: null,
      confirmar: () => assert.fail("No debe confirmar automáticamente."),
      cancelar: () => {},
    }),
  );
  assert.match(html, /Todavía no has marcado un punto/);
  assert.match(html, /disabled=""[^>]*>Confirmar punto de partida/);
  assert.match(html, /Usar centro del mapa/);
  assert.match(html, /Cancelar selección/);
});

// ===== Cliente público =====

test("optimización usa POST, Gateway, cookie y solo lista_id; conserva toda la respuesta", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", async (url, opciones) => {
    assert.equal(url, "/api/v1/rutas/optimizar");
    assert.equal(opciones.method, "POST");
    assert.equal(opciones.credentials, "include");
    assert.equal(opciones.headers["Content-Type"], "application/json");
    assert.deepEqual(JSON.parse(opciones.body), { lista_id: LISTA_ID });
    assert.equal(opciones.headers.Authorization, undefined);
    return Response.json(ejemplo);
  });
  assert.deepEqual(await rutasApi.optimizar({ lista_id: LISTA_ID }), ejemplo);
});

test("acepta origen opcional explícito o null, sin agregar campos históricos", async (prueba) => {
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (_url, opciones) => {
    llamadas.push(JSON.parse(opciones.body));
    return Response.json(ejemplo);
  });
  await rutasApi.optimizar({ lista_id: LISTA_ID, origen: { lat: 0, lon: 0 } });
  await rutasApi.optimizar({ lista_id: LISTA_ID, origen: null });
  assert.deepEqual(llamadas, [
    { lista_id: LISTA_ID, origen: { lat: 0, lon: 0 } },
    { lista_id: LISTA_ID, origen: null },
  ]);
});

test("conserva código y detalle de errores; solo 401 solicita iniciar sesión", async (prueba) => {
  for (const estado of [400, 401, 403, 404, 422, 503, 502, 504]) {
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
      rutasApi.optimizar({ lista_id: LISTA_ID }),
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
    rutasApi.optimizar({ lista_id: LISTA_ID }),
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
    rutasApi.optimizar({ lista_id: LISTA_ID }, controlador.signal),
    { name: "AbortError" },
  );
  simulacion.mock.restore();
  prueba.mock.method(globalThis, "fetch", async () => {
    throw new TypeError("Failed to fetch");
  });
  await assert.rejects(
    rutasApi.optimizar({ lista_id: LISTA_ID }),
    (errorObtenido) =>
      /conectar/.test(mensajeErrorRuta(errorObtenido)) &&
      !(errorObtenido instanceof ErrorSolicitudRutas),
  );
});

test("UUID inválido se rechaza sin red; admite UUID canónico con mayúsculas", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", () =>
    assert.fail("No debe consultar con UUID inválido"),
  );
  assert.equal(esListaIDValida(LISTA_ID), true);
  assert.equal(esListaIDValida(LISTA_ID.toUpperCase()), true);
  for (const listaID of [
    null,
    "",
    "lista-real",
    "../listas",
    `${LISTA_ID}x`,
    `${LISTA_ID}\n`,
    LISTA_ID.replaceAll("-", ""),
  ]) {
    assert.equal(esListaIDValida(listaID), false);
    await assert.rejects(
      rutasApi.optimizar({ lista_id: listaID }),
      (errorObtenido) =>
        errorObtenido instanceof ErrorSolicitudRutas &&
        errorObtenido.estado === 400,
    );
  }
});

test("cobertura reconoce solo el detalle público validado y diferencia límites de paradas", () => {
  const mensaje =
    "No se encontró una forma de comprar la lista en las tiendas cercanas.";
  const productoID = ejemplo.no_cubiertos[0].producto_id;
  const errorCobertura = new ErrorSolicitudRutas(
    422,
    mensaje,
    `productos sin cobertura: ${productoID}, ${productoID}`,
  );
  assert.deepEqual(productosSinCobertura(errorCobertura), [productoID]);
  assert.match(mensajeErrorRuta(errorCobertura), /Faltan ofertas vigentes/);
  assert.match(
    orientacionErrorRuta(errorCobertura),
    /Puede deberse.*stock.*precios vencidos.*radio/,
  );
  assert.match(orientacionErrorRuta(errorCobertura), /no permite distinguir/);
  const errorParadas = new ErrorSolicitudRutas(
    422,
    mensaje,
    "ningún conjunto de tiendas cubre la compra",
  );
  assert.match(orientacionErrorRuta(errorParadas), /máximo de paradas/);
  assert.doesNotMatch(orientacionErrorRuta(errorParadas), /precios vencidos/);
  assert.doesNotMatch(
    mensajeErrorRuta(errorParadas),
    /no se encontró una forma/i,
  );
  for (const detalle of [
    null,
    [],
    { producto_id: productoID },
    "error SQL privado",
    "productos sin cobertura: ",
    `productos sin cobertura: ${productoID}, dato-no-valido`,
    "productos sin cobertura: <script>alert(1)</script>",
  ]) {
    assert.deepEqual(
      productosSinCobertura(new ErrorSolicitudRutas(422, mensaje, detalle)),
      [],
    );
  }
  assert.deepEqual(
    productosSinCobertura(
      new ErrorSolicitudRutas(
        503,
        mensaje,
        `productos sin cobertura: ${productoID}`,
      ),
    ),
    [],
  );
  assert.deepEqual(
    productosSinCobertura(
      new ErrorSolicitudRutas(
        422,
        "Otro error",
        `productos sin cobertura: ${productoID}`,
      ),
    ),
    [],
  );
});

test("cada causa 422 orienta sin diagnosticar vencimiento confirmado y 403 no es un fallo de red", () => {
  for (const [mensaje, esperado] of [
    ["La lista no tiene artículos por comprar.", /marcados como comprados/],
    [
      "La lista supera el máximo de artículos para calcular una ruta.",
      /listas más pequeñas/,
    ],
    [
      "No hay supermercados cercanos para calcular una ruta.",
      /sucursales físicas.*radio/,
    ],
    ["Otra restricción de negocio.", /artículos pendientes/],
  ])
    assert.match(
      orientacionErrorRuta(new ErrorSolicitudRutas(422, mensaje)),
      esperado,
    );
  assert.match(
    mensajeErrorRuta(new ErrorSolicitudRutas(403, "Forbidden")),
    /No tienes permisos/,
  );
  assert.doesNotMatch(
    mensajeErrorRuta(new ErrorSolicitudRutas(403, "Forbidden")),
    /conectar|sesión expiró/,
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
  assert.ok(
    html.indexOf('aria-label="Parada 1: A Cuenta"') <
      html.indexOf('aria-label="Parada 2: Cugat"'),
  );
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

test("pluraliza envases sin alterar cantidad, importes ni el orden de Motor", () => {
  const html = presentar(ejemplo);
  assert.match(html, /1 envase ×/);
  assert.match(html, /2 envases ×/);
  assert.doesNotMatch(html, /1 envases/);
  assert.ok(
    html.indexOf('aria-label="Parada 1: A Cuenta"') <
      html.indexOf('aria-label="Parada 2: Cugat"'),
  );
});

test("diez repeticiones de la misma oferta se presentan como una compra con diez envases", () => {
  const base = {
    ...ejemplo.ruta.paradas[0].articulos[0],
    cantidad: 1,
    subtotal: 1090,
  };
  const articulos = Object.freeze(
    Array.from({ length: 10 }, () => Object.freeze({ ...base })),
  );
  const agrupados = agruparArticulosParada(articulos);
  assert.equal(agrupados.length, 1);
  assert.equal(agrupados[0].cantidad, 10);
  assert.equal(agrupados[0].subtotal, 10900);
  assert.equal(articulos[0].cantidad, 1);
  const datos = structuredClone(ejemplo);
  datos.ruta.paradas = [
    { ...datos.ruta.paradas[0], articulos, subtotal: 10900 },
  ];
  datos.ruta.total = 10900;
  const html = presentar(datos);
  assert.equal(html.split(base.nombre).length - 1, 1);
  assert.match(html, /10 envases ×/);
  assert.match(html, /1 producto · 10 envases/);
  assert.match(html, /10\.900/);
});

test("equivalentes enviados por Motor se agrupan por oferta, preservando referencias e importes", () => {
  const articulo = ejemplo.ruta.paradas[0].articulos[0];
  const agrupados = agruparArticulosParada([
    {
      ...articulo,
      nombre: undefined,
      producto_id: "lista-primero",
      cantidad: 2,
      subtotal: 2180,
    },
    { ...articulo, producto_id: "lista-segundo", cantidad: 3, subtotal: 3270 },
  ]);
  assert.equal(agrupados.length, 1);
  assert.deepEqual(agrupados[0].producto_ids, [
    "lista-primero",
    "lista-segundo",
  ]);
  assert.equal(agrupados[0].cantidad, 5);
  assert.equal(agrupados[0].subtotal, 5450);
  assert.equal(agrupados[0].nombre, articulo.nombre);
});

test("nombres iguales no prueban equivalencia; ofertas y precios distintos no se mezclan", () => {
  const articulo = ejemplo.ruta.paradas[0].articulos[0];
  const agrupados = agruparArticulosParada([
    articulo,
    { ...articulo, producto_id_oferta: "otra-oferta" },
    { ...articulo, precio_unitario: articulo.precio_unitario + 100 },
  ]);
  assert.equal(agrupados.length, 3);
  assert.equal(new Set(agrupados.map((agrupado) => agrupado.clave)).size, 3);
  const subtotales = agruparArticulosParada([
    { ...articulo, subtotal: 2000 },
    { ...articulo, subtotal: 3000 },
  ]);
  assert.equal(
    subtotales[0].subtotal,
    5000,
    "no recalcula precio × cantidad para sustituir el subtotal recibido",
  );
  assert.equal(
    agruparArticulosParada([
      { ...articulo, producto_id_oferta: "", producto_id: "origen-uno" },
      { ...articulo, producto_id_oferta: "", producto_id: "origen-dos" },
    ]).length,
    2,
    "sin oferta identificada no se deduce equivalencia por nombre",
  );
  assert.deepEqual(agruparArticulosParada([]), []);
});

test("las compras se agrupan dentro de cada parada, sin borrar productos de otra tienda", () => {
  const datos = structuredClone(ejemplo);
  const articulo = datos.ruta.paradas[0].articulos[0];
  datos.ruta.paradas[1].articulos = [{ ...articulo }];
  const html = presentar(datos);
  assert.equal(html.split(articulo.nombre).length - 1, 2);
});

test("una parada larga conserva totales y deja los productos adicionales en un desplegable nativo", () => {
  const datos = structuredClone(ejemplo);
  const parada = datos.ruta.paradas[0];
  parada.articulos = Array.from({ length: 10 }, (_, indice) => ({
    ...parada.articulos[0],
    producto_id: `producto-${indice}`,
    producto_id_oferta: `oferta-${indice}`,
    nombre: `Producto de prueba ${indice}`,
    cantidad: 1,
    subtotal: 1090,
  }));
  parada.subtotal = 10900;
  const html = presentar(datos);
  assert.match(html, /10 productos · 10 envases/);
  assert.match(html, /Ver 6 productos más/);
  assert.match(html, /aria-label="Más productos en/);
  assert.match(html, /Producto de prueba 9/);
  assert.doesNotMatch(html, /<details[^>]*\bopen(?:=|>)/);
  assert.match(html, /10\.900/);
});

test("sin base comparativa no presenta ahorro cero y conserva referencias sin nombre", () => {
  const datos = { ...ejemplo, base_una_tienda: null, ahorro: null };
  const html = presentar(datos);
  assert.match(html, /Sin tienda única para comparar/);
  assert.match(html, /no hay una compra en un solo lugar con la cual comparar/);
  assert.doesNotMatch(html, /\$0/);
  // La aclaración del total vale también para rutas parciales y no promete un precio menor.
  assert.match(html, /Puede diferir del total de tu lista/);
  assert.doesNotMatch(html, /siempre|más barat/i);
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
function renderHook(listaID, reiniciar = false, medio, origen) {
  ciclo.preparar(reiniciar);
  const estado = useRutaLista(listaID, medio, origen);
  ciclo.confirmar();
  return estado;
}

/** Resuelve las promesas pendientes sin esperar relojes o servicios externos. */
const resolver = () => new Promise((continuar) => setImmediate(continuar));

test("el hook no consulta ni optimización ni detalle con selección inválida", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", () =>
    assert.fail("No debe consultar"),
  );
  assert.equal(
    renderHook("lista-invalida", true).estado.tipo,
    "lista_invalida",
  );
  await resolver();
  assert.equal(renderHook("lista-invalida").estado.tipo, "lista_invalida");
  assert.equal(renderHook(null).estado.tipo, "lista_invalida");
  assert.equal(renderHook(null).estado.tipo, "sin_lista");
  ciclo.desmontar();
});

test("el hook envía modo y origen y cancela el cálculo anterior al cambiar de perfil", async (prueba) => {
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (url, opciones) => {
    llamadas.push({
      cuerpo: JSON.parse(opciones.body),
      senal: opciones.signal,
    });
    return Response.json({ ...ejemplo, no_cubiertos: [] });
  });
  const origen = { lat: -38.7359, lon: -72.5904 };
  renderHook(LISTA_ID, true, "vehiculo", origen);
  await resolver();
  renderHook(LISTA_ID, false, "bicicleta", origen);
  await resolver();
  assert.deepEqual(
    llamadas.map((llamada) => llamada.cuerpo),
    [
      { lista_id: LISTA_ID, medio_transporte: "vehiculo", origen },
      { lista_id: LISTA_ID, medio_transporte: "bicicleta", origen },
    ],
  );
  assert.equal(llamadas[0].senal.aborted, true);
  ciclo.desmontar();
});

test("un 422 de cobertura carga nombres como complemento sin alterar el rechazo", async (prueba) => {
  const productoID = ejemplo.no_cubiertos[0].producto_id;
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (url) => {
    llamadas.push(url);
    if (url.includes("/listas/"))
      return Response.json({
        articulos: [
          { producto_id: productoID, producto: { nombre: "Quinoa" } },
        ],
      });
    return Response.json(
      {
        mensaje:
          "No se encontró una forma de comprar la lista en las tiendas cercanas.",
        detalle: `productos sin cobertura: ${productoID}`,
      },
      { status: 422 },
    );
  });
  renderHook(LISTA_ID, true);
  await resolver();
  const { estado, nombres } = renderHook(LISTA_ID);
  assert.equal(estado.tipo, "error");
  assert.equal(estado.error.estado, 422);
  assert.equal(nombres[productoID], "Quinoa");
  assert.equal(llamadas.length, 2);
  ciclo.desmontar();
});

test("un rechazo sigue visible si falla la lectura complementaria de nombres", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", async (url) => {
    if (url.includes("/listas/")) throw new TypeError("sin conexión");
    return Response.json(
      {
        mensaje:
          "No se encontró una forma de comprar la lista en las tiendas cercanas.",
        detalle: `productos sin cobertura: ${ejemplo.no_cubiertos[0].producto_id}`,
      },
      { status: 422 },
    );
  });
  renderHook(LISTA_ID, true);
  await resolver();
  const { estado, nombres } = renderHook(LISTA_ID);
  assert.equal(estado.error.estado, 422);
  assert.deepEqual(nombres, {});
  ciclo.desmontar();
});

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
  assert.equal(renderHook(LISTA_ID, true).estado.tipo, "cargando");
  await resolver();
  let estado = renderHook(LISTA_ID);
  assert.equal(estado.estado.tipo, "resultado");
  assert.deepEqual(estado.nombres, {});
  assert.equal(consultas, 2);
  estado.recalcular();
  renderHook(LISTA_ID);
  assert.equal(renderHook(LISTA_ID).estado.tipo, "cargando");
  await resolver();
  estado = renderHook(LISTA_ID);
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
  renderHook(LISTA_ID, true);
  await resolver();
  assert.equal(renderHook(LISTA_ID).nombres[productoID], "Quinoa");
  ciclo.desmontar();
  prueba.mock.method(globalThis, "fetch", async (url) => {
    llamadas.push(url);
    return Response.json({ ...ejemplo, no_cubiertos: [] });
  });
  renderHook(OTRA_LISTA_ID, true);
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
  renderHook(LISTA_ID, true);
  renderHook(OTRA_LISTA_ID);
  assert.equal(pendientes[0].opciones.signal.aborted, true);
  pendientes[1].resolverPeticion(
    Response.json({ ...ejemplo, no_cubiertos: [], ahorro: 1000 }),
  );
  await resolver();
  pendientes[0].resolverPeticion(Response.json(ejemplo));
  await resolver();
  assert.equal(renderHook(OTRA_LISTA_ID).estado.datos.ahorro, 1000);
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
  renderHook(LISTA_ID, true);
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
  renderHook(LISTA_ID, true);
  await resolver();
  const { estado } = renderHook(LISTA_ID);
  assert.equal(estado.tipo, "error");
  assert.equal(estado.error.estado, 422);
  assert.equal(llamadas, 1);
  ciclo.desmontar();
});

// ===== Página y navegación =====

const pagina = await cargar("../src/features/rutas/RutaLista.tsx", {
  react: import.meta.resolve("react"),
  "react-router-dom": import.meta.resolve("react-router-dom"),
  "lucide-react": crearModulo(`
    export const Bike = () => null;
    export const BusFront = () => null;
    export const CarFront = () => null;
    export const Footprints = () => null;
    export const ShoppingBasket = () => null;
  `),
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
  "./MapaRuta": crearModulo("export const MapaRuta = () => null;"),
  "./SelectorOrigen": crearModulo("export const SelectorOrigen = () => null;"),
  "./useRutaLista": crearModulo(
    'export const useRutaLista = () => ({ estado: { tipo: "sin_lista" }, nombres: {}, recalcular() {} });',
  ),
});

/** Renderiza estados explícitos sin manipular el orden interno de hooks de la página. */
function presentarEstado(estado, nombres = {}) {
  return renderToStaticMarkup(
    createElement(
      MemoryRouter,
      null,
      createElement(pagina.modulo.EstadoRecomendacion, {
        estado,
        nombres,
        recalcular() {},
      }),
    ),
  );
}

test("página muestra carga, ausencia de selección y acciones adecuadas a cada error", () => {
  assert.match(presentarEstado({ tipo: "cargando" }), /role="status"/);
  assert.match(presentarEstado({ tipo: "sin_lista" }), /Selecciona una lista/);
  for (const codigo of [400, 401, 403, 404, 422, 503, 502]) {
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

test("la página de rutas presenta solo los tres medios viales con selección accesible", () => {
  const html = renderToStaticMarkup(
    createElement(
      MemoryRouter,
      { initialEntries: [`/route?lista=${LISTA_ID}`] },
      createElement(pagina.modulo.default),
    ),
  );

  for (const etiqueta of ["Vehículo particular", "Bicicleta", "A pie"])
    assert.ok(html.includes(etiqueta));
  assert.match(html, /type="radio"/);
  assert.equal((html.match(/type="radio"/g) || []).length, 3);
  assert.doesNotMatch(html, /Transporte público|scooter/i);
  assert.match(html, /Usar mi ubicación actual/);
  assert.match(html, /Recorrido vial/);
});

test("un error de cobertura muestra nombres, alcance y acciones, nunca el detalle técnico arbitrario", () => {
  const productoID = ejemplo.no_cubiertos[0].producto_id;
  const mensaje =
    "No se encontró una forma de comprar la lista en las tiendas cercanas.";
  const error = new ErrorSolicitudRutas(
    422,
    mensaje,
    `productos sin cobertura: ${productoID}`,
  );
  const html = presentarEstado(
    { tipo: "error", error },
    { [productoID]: "Quinoa" },
  );
  assert.match(html, /Faltan ofertas vigentes/);
  assert.match(html, /Quinoa/);
  assert.match(html, /Artículos sin cobertura/);
  assert.match(html, /precios vencidos/);
  assert.match(html, /no permite distinguir/);
  assert.doesNotMatch(html, /Reintentar cálculo|Resumen de la ruta/);
  assert.match(html, /Revisar mis listas/);
  const desconocido = presentarEstado({
    tipo: "error",
    error: new ErrorSolicitudRutas(422, mensaje, "error SQL privado"),
  });
  assert.doesNotMatch(desconocido, /error SQL privado/);
  assert.match(
    presentarEstado({ tipo: "lista_invalida" }),
    /enlace de la lista no es válido/,
  );
});

test("la URL inválida vuelve al listado sin conservar el parámetro rechazado", () => {
  const html = renderToStaticMarkup(
    createElement(
      MemoryRouter,
      { initialEntries: ["/route?lista=invalido"] },
      createElement(pagina.modulo.default),
    ),
  );
  assert.match(html, /href="\/listas"/);
  assert.doesNotMatch(html, /\/listas\?lista=invalido/);
  assert.match(html, /Ir a mis listas/);
});

test("la página conserva lista al volver y la ruta sigue bajo ProtectedRoute", async () => {
  const html = renderToStaticMarkup(
    createElement(
      MemoryRouter,
      { initialEntries: [`/route?lista=${LISTA_ID}`] },
      createElement(pagina.modulo.default),
    ),
  );
  assert.ok(html.includes(`href="/listas?lista=${LISTA_ID}"`));
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
    "lucide-react": crearModulo(`
      export const Info = () => null;
      export const ListChecks = () => null;
      export const ListPlus = () => null;
      export const RefreshCw = () => null;
      export const ShoppingBasket = () => null;
      export const Trash2 = () => null;
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

test("ruta parcial explica el motivo de cada artículo y usa el nombre del motor", () => {
  const datos = {
    ...ejemplo,
    no_cubiertos: [
      { producto_id: "sin-oferta", motivo: "sin_oferta_en_radio" },
      {
        producto_id: "fuera-de-paradas",
        nombre: "Aceite maravilla 1 L",
        motivo: "fuera_del_maximo_de_paradas",
      },
    ],
  };
  const html = presentar(datos);
  assert.match(html, /Compra parcial/);
  assert.match(html, /Aceite maravilla 1 L/);
  assert.match(html, /exigiría visitar más tiendas que el máximo de paradas/);
  assert.match(
    html,
    /Sin oferta vigente disponible en las sucursales evaluadas/,
  );
  assert.ok(html.includes("Producto sin-oferta"));
});
