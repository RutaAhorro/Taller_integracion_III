// Ejecuta hooks y vistas reales con HTTP controlado; no modifica la BD ni acredita un despliegue.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

// ===== Carga y ciclo de hooks =====

/** Crea un módulo aislado para sustituir solo las dependencias de prueba. */
const crearModulo = (codigo) =>
  `data:text/javascript;base64,${Buffer.from(codigo).toString("base64")}`;

/** Transpila el código real conservando sus funciones, salvo los imports y la variable de Vite. */
async function cargar(ruta, dependencias = {}) {
  let fuente = await readFile(new URL(ruta, import.meta.url), "utf8");
  fuente = fuente.replace("import.meta.env.VITE_API_URL", '"/api/v1"');
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

const urlReactHooks = crearModulo(`
  const estados = [], referencias = [], efectos = [], pendientes = [];
  let indiceEstado = 0, indiceReferencia = 0, indiceEfecto = 0;
  export function desmontar() {
    for (const efecto of efectos) efecto?.limpiar?.();
    efectos.length = 0;
  }
  export function preparar(reiniciar = false) {
    if (reiniciar) { desmontar(); estados.length = 0; referencias.length = 0; pendientes.length = 0; }
    indiceEstado = indiceReferencia = indiceEfecto = 0;
  }
  export function useState(inicial) {
    const indice = indiceEstado++;
    if (indice === estados.length) estados.push(typeof inicial === 'function' ? inicial() : inicial);
    return [estados[indice], valor => { estados[indice] = typeof valor === 'function' ? valor(estados[indice]) : valor; }];
  }
  export function useRef(inicial) {
    const indice = indiceReferencia++;
    if (indice === referencias.length) referencias.push({ current: inicial });
    return referencias[indice];
  }
  export function useEffect(accion, dependencias) {
    const indice = indiceEfecto++;
    const anterior = efectos[indice];
    if (!anterior || dependencias.some((valor, posicion) => !Object.is(valor, anterior.dependencias[posicion]))) {
      pendientes.push(() => {
        anterior?.limpiar?.();
        efectos[indice] = { dependencias, limpiar: accion() };
      });
    }
  }
  export function confirmarEfectos() { for (const pendiente of pendientes.splice(0)) pendiente(); }
`);
const { preparar, confirmarEfectos, desmontar } = await import(urlReactHooks);
const urlToast = crearModulo(
  "export const avisos = []; export const useToast = () => ({ toast: (...datos) => avisos.push(datos) });",
);
const { avisos } = await import(urlToast);
const api = await cargar("../src/features/listas/listas-api.ts");
const detalle = await cargar("../src/features/listas/useDetalleLista.ts", {
  react: urlReactHooks,
  "@/components/ToastContext": urlToast,
  "@/features/listas/listas-api": api.url,
});
const { useDetalleLista, esCantidadValida } = detalle.modulo;
const urlProductos = crearModulo(`
  export const catalogo = { consultar: async () => ({ products: [], totalPages: 1 }) };
  export const getProducts = (...datos) => catalogo.consultar(...datos);
`);
const { catalogo } = await import(urlProductos);
const busqueda = await cargar(
  "../src/features/listas/useBusquedaArticulos.ts",
  {
    react: urlReactHooks,
    "@/features/catalogo/products-api": urlProductos,
  },
);

/** Espera las promesas de las peticiones, no un reloj ni un servicio externo. */
const resolverPeticiones = () =>
  new Promise((resolver) => setImmediate(resolver));

/** Simula renders y cleanup; los callbacks y el cliente HTTP siguen siendo los reales del hook. */
function montarHook(obtener) {
  preparar(true);
  return () => {
    preparar();
    const resultado = obtener();
    confirmarEfectos();
    return resultado;
  };
}

const producto = {
  nombre: "Arroz 1 kg",
  marca: "Marca real",
  supermercado: "Lider",
  unidad: "1 kg",
  precio: 1190,
  url_imagen: "",
};
const articulo = {
  id: 41,
  producto_id: "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
  cantidad: 2,
  esta_comprado: false,
  estado_producto: "sin_stock",
  producto,
};
const lista = {
  id: "lista-1",
  nombre: "Semana",
  estado: "activa",
  total_articulos: 1,
  articulos: [articulo],
};

/** Monta una lista recibida de HTTP y registra solo publicaciones confirmadas. */
async function montarDetalle(prueba, responder) {
  const cambios = [],
    ocupaciones = [];
  const opciones = {
    id: lista.id,
    onUpdated: (valor) => cambios.push(valor),
    onDeleted() {},
    onBusy: (valor) => ocupaciones.push(valor),
  };
  prueba.mock.method(globalThis, "fetch", async (url, peticion) => {
    if (!peticion.method) return Response.json(lista);
    return responder(url, peticion);
  });
  const leer = montarHook(() => useDetalleLista(opciones));
  avisos.length = 0;
  leer();
  await resolverPeticiones();
  assert.equal(leer().lista.articulos[0].producto.nombre, producto.nombre);
  return { leer, cambios, ocupaciones };
}

// ===== Escrituras y persistencia de estado confirmado =====

test("editar cantidad conserva producto y advertencia, y quitar espera la confirmación", async (prueba) => {
  const { leer, cambios } = await montarDetalle(
    prueba,
    async (_url, peticion) => {
      if (peticion.method === "DELETE")
        return new Response(null, { status: 204 });
      return Response.json({
        id: 41,
        producto_id: articulo.producto_id,
        cantidad: 3,
        esta_comprado: false,
      });
    },
  );
  assert.equal(await leer().actualizarArticulo(41, { cantidad: 3 }), true);
  assert.equal(leer().lista.articulos[0].cantidad, 3);
  assert.deepEqual(leer().lista.articulos[0].producto, producto);
  assert.equal(leer().lista.articulos[0].estado_producto, "sin_stock");
  assert.equal(await leer().eliminarArticulo(41), true);
  assert.deepEqual(leer().lista.articulos, []);
  assert.equal(cambios.at(-1).total_articulos, 0);
});

test("fallos 401, 404, 409 y 503 no alteran artículos ni generan éxito", async (prueba) => {
  for (const estado of [401, 404, 409, 503]) {
    const { leer, cambios } = await montarDetalle(prueba, async () =>
      Response.json(
        { mensaje: "Rechazo real del servidor" },
        { status: estado },
      ),
    );
    assert.equal(await leer().actualizarArticulo(41, { cantidad: 3 }), false);
    assert.equal(leer().lista.articulos[0].cantidad, 2);
    assert.equal(leer().error.estado, estado);
    assert.equal(cambios.length, 0);
    assert.equal(avisos.length, 0);
    assert.equal(await leer().eliminarArticulo(41), false);
    assert.equal(leer().lista.articulos.length, 1);
    prueba.mock.restoreAll();
  }
});

test("un agregado reintentado conserva la clave y dos envíos inmediatos no duplican escrituras", async (prueba) => {
  const claves = [];
  let confirmar;
  let fallar = true;
  const { leer, cambios } = await montarDetalle(
    prueba,
    async (_url, peticion) => {
      claves.push(peticion.headers["Idempotency-Key"]);
      if (fallar) throw new TypeError("Conexión interrumpida");
      return new Promise((resolver) => {
        confirmar = () =>
          resolver(
            Response.json({
              ...lista,
              articulos: [{ ...articulo, cantidad: 4 }],
            }),
          );
      });
    },
  );
  assert.equal(await leer().agregarArticulo(articulo.producto_id, 2), false);
  assert.equal(cambios.length, 0);
  assert.equal(leer().lista.articulos[0].cantidad, 2);
  fallar = false;
  const pendiente = leer().agregarArticulo(articulo.producto_id, 2);
  assert.equal(await leer().agregarArticulo(articulo.producto_id, 2), false);
  assert.equal(claves.length, 2);
  assert.equal(claves[0], claves[1]);
  confirmar();
  assert.equal(await pendiente, true);
  assert.equal(leer().lista.articulos[0].cantidad, 4);
  assert.equal(leer().lista.articulos[0].estado_producto, "sin_stock");
});

test("cambiar de cuenta descarta una respuesta de escritura de la vista desmontada", async (prueba) => {
  let confirmar;
  const { leer, cambios } = await montarDetalle(
    prueba,
    async () =>
      new Promise((resolver) => {
        confirmar = () => resolver(Response.json({ ...articulo, cantidad: 3 }));
      }),
  );
  const pendiente = leer().actualizarArticulo(41, { cantidad: 3 });
  desmontar();
  confirmar();
  assert.equal(await pendiente, false);
  assert.equal(cambios.length, 0);
  assert.equal(avisos.length, 0);
});

test("los envases solo aceptan enteros de 1 a 99, sin enviar cantidades inválidas", async (prueba) => {
  for (const cantidad of [0, -1, 1.5, 100, NaN, Infinity])
    assert.equal(esCantidadValida(cantidad), false);
  assert.equal(esCantidadValida(1), true);
  assert.equal(esCantidadValida(99), true);
  let escrituras = 0;
  const { leer } = await montarDetalle(prueba, async () => {
    escrituras++;
    return Response.json(articulo);
  });
  assert.equal(await leer().agregarArticulo(articulo.producto_id, 0), false);
  assert.equal(await leer().actualizarArticulo(41, { cantidad: 100 }), false);
  assert.equal(escrituras, 0);
});

// ===== Búsqueda y descarte de respuestas =====

test("buscar respeta el mínimo UTF-8, consulta al servidor y pagina con los mismos filtros", async () => {
  const consultas = [];
  catalogo.consultar = async (ruta, senal) => {
    consultas.push({ ruta, senal });
    return { products: [{ id: "producto-1", name: "Arroz" }], totalPages: 2 };
  };
  const leer = montarHook(() => busqueda.modulo.useBusquedaArticulos());
  leer().cambiarConsulta("te");
  leer().buscar();
  await resolverPeticiones();
  assert.equal(consultas.length, 0);
  leer().cambiarConsulta("té");
  leer().buscar();
  leer();
  await resolverPeticiones();
  assert.equal(consultas.length, 1);
  assert.match(consultas[0].ruta, /q=t%C3%A9/);
  assert.match(consultas[0].ruta, /page=1/);
  assert.equal(leer().productos[0].id, "producto-1");
  leer().cambiarPagina(2);
  leer();
  await resolverPeticiones();
  assert.match(consultas[1].ruta, /q=t%C3%A9/);
  assert.match(consultas[1].ruta, /page=2/);
  leer().limpiarBusqueda();
  assert.equal(leer().consulta, "");
  assert.equal(leer().consultado, false);
  assert.deepEqual(leer().productos, []);
  assert.equal(leer().error, null);
});

test("editar o desmontar aborta la búsqueda y una respuesta antigua no repone productos", async () => {
  let resolverAnterior;
  let senalAnterior;
  catalogo.consultar = (_ruta, senal) => {
    senalAnterior = senal;
    return new Promise((resolver) => {
      resolverAnterior = resolver;
    });
  };
  const leer = montarHook(() => busqueda.modulo.useBusquedaArticulos());
  leer().cambiarConsulta("arroz");
  leer().buscar();
  leer();
  leer().cambiarConsulta("leche");
  assert.deepEqual(leer().productos, []);
  assert.equal(senalAnterior.aborted, true);
  resolverAnterior({ products: [{ id: "respuesta-antigua" }], totalPages: 1 });
  await resolverPeticiones();
  assert.deepEqual(leer().productos, []);
  leer().buscar();
  leer();
  desmontar();
  assert.equal(senalAnterior.aborted, true);
});

// ===== Vistas de artículos =====

const formatters = await cargar("../src/features/catalogo/formatters.ts");
const vista = await cargar("../src/features/listas/ArticulosLista.tsx", {
  react: import.meta.resolve("react"),
  "lucide-react": crearModulo(`
    export const Check = () => null;
    export const Package = () => null;
    export const Trash2 = () => null;
  `),
  "@/features/listas/listas-api": api.url,
  "@/features/listas/useDetalleLista": detalle.url,
  "@/features/catalogo/formatters": formatters.url,
  "@/features/listas/useBusquedaArticulos": crearModulo(`
    export const MINIMO_BYTES_BUSQUEDA_ARTICULOS = 3;
    export const useBusquedaArticulos = () => ({
      consulta: "", productos: [], busquedaCorta: true, totalPaginas: 1,
    });
  `),
  "@/components/ui/button": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export function Button({ children, variant, ...datos }) { return createElement('button', datos, children); }
  `),
  "@/components/ui/input": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export const Input = datos => createElement('input', datos);
  `),
});

test("la vista muestra productos, precios ausentes y advertencias sin inventar disponibilidad", () => {
  for (const [estado, aviso] of [
    ["disponible", "Stock confirmado"],
    ["sin_stock", "Sin stock"],
    ["desconocido", "Disponibilidad desconocida"],
    ["no_encontrado", "Producto no encontrado"],
  ]) {
    const contenido = renderToStaticMarkup(
      createElement(vista.modulo.ArticulosLista, {
        articulos: [
          {
            ...articulo,
            estado_producto: estado,
            producto:
              estado === "no_encontrado" ? null : { ...producto, precio: null },
          },
        ],
        ocupado: false,
        alAgregar() {},
        alActualizar() {},
        alEliminar() {},
      }),
    );
    assert.match(contenido, new RegExp(aviso));
    assert.match(contenido, /Precio vigente no disponible/);
    assert.match(contenido, /Guardar cantidad/);
    assert.match(contenido, /Quitar de la lista/);
    assert.doesNotMatch(contenido, /producto_normalizado_id|\$0/);
  }
});

test("la tarjeta de artículo presenta la imagen real del catálogo con texto alternativo", () => {
  const contenido = renderToStaticMarkup(
    createElement(vista.modulo.ArticulosLista, {
      articulos: [
        {
          ...articulo,
          producto: {
            ...producto,
            url_imagen: "https://catalogo.example/arroz.jpg",
          },
        },
      ],
      ocupado: false,
      alAgregar: async () => true,
      alActualizar: async () => true,
      alEliminar: async () => true,
    }),
  );

  assert.match(contenido, /src="https:\/\/catalogo\.example\/arroz\.jpg"/);
  assert.match(contenido, /alt="Imagen de Arroz 1 kg"/);
  assert.match(contenido, /loading="lazy"/);
});

test("los artículos con precio cero o negativo se ignoran en totales y etiquetas", () => {
  for (const precioInvalido of [0, -1500]) {
    const contenido = renderToStaticMarkup(
      createElement(vista.modulo.ArticulosLista, {
        articulos: [
          {
            ...articulo,
            producto: { ...producto, precio: precioInvalido },
          },
        ],
        ocupado: false,
        alAgregar() {},
        alActualizar() {},
        alEliminar() {},
      }),
    );
    assert.match(contenido, /Total no disponible/);
    assert.match(contenido, /Precio vigente no disponible/);
    assert.doesNotMatch(contenido, /\$0/);
  }

  const contenido = renderToStaticMarkup(
    createElement(vista.modulo.ArticulosLista, {
      articulos: [
        {
          ...articulo,
          producto: { ...producto, precio: 1500 },
        },
        {
          ...articulo,
          id: 42,
          cantidad: 1,
          producto: { ...producto, precio: 0 },
        },
      ],
      ocupado: false,
      alAgregar() {},
      alActualizar() {},
      alEliminar() {},
    }),
  );
  assert.match(contenido, /Subtotal con precio vigente: \$3\.000/);
  assert.match(contenido, /1 artículo no tiene precio vigente/);
  assert.match(contenido, /Precio vigente no disponible/);
  assert.doesNotMatch(contenido, /\$0/);
});
