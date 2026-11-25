import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import ts from "typescript";

// Ejecuta el cliente real con la variable que Vite sustituye al compilar.
const codigoFuente = await readFile(
  new URL("../src/features/listas/listas-api.ts", import.meta.url),
  "utf8",
);
const { outputText: codigoTranspilado } = ts.transpileModule(
  codigoFuente.replace("import.meta.env.VITE_API_URL", '"/api/v1/"'),
  {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
    },
  },
);
const { listasApi, ErrorSolicitudListas } = await import(
  `data:text/javascript;base64,${Buffer.from(codigoTranspilado).toString("base64")}`
);

test("CRUD usa Gateway y cookie, mantiene los datos de la API y acepta DELETE 204", async (prueba) => {
  const lista = {
    id: "lista-1",
    nombre: "Semana",
    estado: "activa",
    total_articulos: 0,
  };
  const llamadas = [];
  prueba.mock.method(globalThis, "fetch", async (url, opciones) => {
    llamadas.push({ url, ...opciones });
    if (opciones.method === "DELETE")
      return new Response(null, { status: 204 });
    const cuerpo = opciones.method
      ? lista
      : url.endsWith("/lista-1")
        ? { ...lista, articulos: [] }
        : { listas: [lista] };
    return Response.json(cuerpo);
  });
  assert.deepEqual(await listasApi.listar(), { listas: [lista] });
  assert.deepEqual(await listasApi.crear("Semana"), lista);
  assert.deepEqual(await listasApi.obtener("lista-1"), {
    ...lista,
    articulos: [],
  });
  await listasApi.actualizar("lista-1", "Semana", "archivada");
  assert.equal(await listasApi.eliminar("lista-1"), undefined);
  assert.ok(
    llamadas.every(
      (llamada) =>
        llamada.credentials === "include" &&
        llamada.url.startsWith("/api/v1/listas"),
    ),
  );
  assert.equal(llamadas[0].headers, undefined);
  assert.deepEqual(JSON.parse(llamadas[1].body), { nombre: "Semana" });
  assert.deepEqual(JSON.parse(llamadas[3].body), {
    nombre: "Semana",
    estado: "archivada",
  });
  assert.equal(llamadas[3].method, "PUT");
});

test("propaga errores reales sin reemplazarlos por éxito o datos simulados", async (prueba) => {
  for (const [estado, cuerpo, esperado] of [
    [401, {}, /sesión expiró/],
    [404, { mensaje: "La lista no existe." }, /La lista no existe/],
    [409, { mensaje: "Alcanzaste el máximo de 100 listas." }, /máximo de 100/],
    [422, { mensaje: "Datos inválidos." }, /Datos inválidos/],
  ]) {
    const simulacion = prueba.mock.method(globalThis, "fetch", async () =>
      Response.json(cuerpo, { status: estado }),
    );
    await assert.rejects(listasApi.crear("Semana"), esperado);
    simulacion.mock.restore();
  }
  prueba.mock.method(
    globalThis,
    "fetch",
    async () => new Response("<html>Bad gateway</html>", { status: 502 }),
  );
  await assert.rejects(listasApi.listar(), /No se pudo completar/);
});

test("permite cancelar una lectura al salir o cambiar de lista", async (prueba) => {
  const controlador = new AbortController();
  controlador.abort();
  prueba.mock.method(globalThis, "fetch", async (_url, opciones) => {
    assert.equal(opciones.signal, controlador.signal);
    opciones.signal.throwIfAborted();
  });
  await assert.rejects(listasApi.obtener("lista-1", controlador.signal), {
    name: "AbortError",
  });
});

test("conserva el estado HTTP y solo identifica 401 como sesión expirada", async (prueba) => {
  for (const estado of [401, 403, 404, 409, 502, 503, 504]) {
    const simulacion = prueba.mock.method(globalThis, "fetch", async () =>
      Response.json(
        { mensaje: "El motor de rutas no está disponible." },
        { status: estado },
      ),
    );
    await assert.rejects(listasApi.listar(), (errorObtenido) => {
      assert.ok(errorObtenido instanceof ErrorSolicitudListas);
      assert.equal(errorObtenido.estado, estado);
      assert.equal(
        errorObtenido.message.includes("sesión expiró"),
        estado === 401,
      );
      return true;
    });
    simulacion.mock.restore();
  }
});

test("un fallo de red no se transforma en un error de sesión", async (prueba) => {
  prueba.mock.method(globalThis, "fetch", async () => {
    throw new TypeError("Failed to fetch");
  });
  await assert.rejects(
    listasApi.listar(),
    (errorObtenido) =>
      errorObtenido instanceof TypeError &&
      !(errorObtenido instanceof ErrorSolicitudListas),
  );
});

test("artículos usa el contrato actual, cookie y clave de operación sin enviar el dueño", async (prueba) => {
  const llamadas = [];
  const articulo = {
    id: 41,
    producto_id: "producto-real",
    cantidad: 2,
    esta_comprado: false,
  };
  prueba.mock.method(globalThis, "fetch", async (url, opciones) => {
    llamadas.push({ url, ...opciones });
    return opciones.method === "DELETE"
      ? new Response(null, { status: 204 })
      : Response.json(articulo);
  });
  await listasApi.agregarArticulos(
    "lista/1",
    [{ producto_id: articulo.producto_id, cantidad: 2 }],
    "operacion-1",
  );
  assert.deepEqual(
    await listasApi.actualizarArticulo("lista/1", 41, { cantidad: 3 }),
    articulo,
  );
  await listasApi.actualizarArticulo("lista/1", 41, { esta_comprado: true });
  assert.equal(await listasApi.eliminarArticulo("lista/1", 41), undefined);
  assert.equal(llamadas[0].url, "/api/v1/listas/lista%2F1/articulos/lote");
  assert.equal(llamadas[0].headers["Idempotency-Key"], "operacion-1");
  assert.deepEqual(JSON.parse(llamadas[0].body), {
    articulos: [{ producto_id: articulo.producto_id, cantidad: 2 }],
  });
  assert.equal(llamadas[1].method, "PUT");
  assert.deepEqual(JSON.parse(llamadas[1].body), { cantidad: 3 });
  assert.deepEqual(JSON.parse(llamadas[2].body), { esta_comprado: true });
  assert.equal(llamadas[3].method, "DELETE");
  assert.ok(llamadas.every((llamada) => llamada.credentials === "include"));
  assert.ok(
    llamadas.every(
      (llamada) => !llamada.body || !llamada.body.includes("usuario_id"),
    ),
  );
});

test("conserva el detalle del rechazo de un lote para explicar el conflicto", async (prueba) => {
  const detalle = [{ producto_id: "producto-real", motivo: "cantidad_maxima" }];
  prueba.mock.method(globalThis, "fetch", async () =>
    Response.json(
      { mensaje: "No se pudo agregar el lote.", detalle },
      { status: 409 },
    ),
  );
  await assert.rejects(
    listasApi.agregarArticulos(
      "lista-1",
      [{ producto_id: "producto-real", cantidad: 99 }],
      "operacion-1",
    ),
    (errorObtenido) => {
      assert.equal(errorObtenido.estado, 409);
      assert.deepEqual(errorObtenido.detalle, detalle);
      return true;
    },
  );
});
