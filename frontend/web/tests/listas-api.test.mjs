import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

// Ejecuta el cliente real con la variable que Vite sustituye al compilar.
const source = await readFile(new URL('../src/features/listas/listas-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const { listasApi } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('CRUD usa Gateway y cookie, mantiene los datos de la API y acepta DELETE 204', async t => {
  const lista = { id: 'lista-1', nombre: 'Semana', estado: 'activa', total_articulos: 0 };
  const calls = [];
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    calls.push({ url, ...init });
    if (init.method === 'DELETE') return new Response(null, { status: 204 });
    const body = init.method ? lista : url.endsWith('/lista-1') ? { ...lista, articulos: [] } : { listas: [lista] };
    return Response.json(body);
  });
  assert.deepEqual(await listasApi.listar(), { listas: [lista] });
  assert.deepEqual(await listasApi.crear('Semana'), lista);
  assert.deepEqual(await listasApi.obtener('lista-1'), { ...lista, articulos: [] });
  await listasApi.actualizar('lista-1', 'Semana', 'archivada');
  assert.equal(await listasApi.eliminar('lista-1'), undefined);
  assert.ok(calls.every(call => call.credentials === 'include' && call.url.startsWith('/api/v1/listas')));
  assert.equal(calls[0].headers, undefined);
  assert.deepEqual(JSON.parse(calls[1].body), { nombre: 'Semana' });
  assert.deepEqual(JSON.parse(calls[3].body), { nombre: 'Semana', estado: 'archivada' });
  assert.equal(calls[3].method, 'PUT');
});

test('propaga errores reales sin reemplazarlos por éxito o datos simulados', async t => {
  for (const [status, body, expected] of [
    [401, {}, /sesión expiró/],
    [404, { mensaje: 'La lista no existe.' }, /La lista no existe/],
    [409, { mensaje: 'Alcanzaste el máximo de 100 listas.' }, /máximo de 100/],
    [422, { mensaje: 'Datos inválidos.' }, /Datos inválidos/],
  ]) {
    const mock = t.mock.method(globalThis, 'fetch', async () => Response.json(body, { status }));
    await assert.rejects(listasApi.crear('Semana'), expected);
    mock.mock.restore();
  }
  t.mock.method(globalThis, 'fetch', async () => new Response('<html>Bad gateway</html>', { status: 502 }));
  await assert.rejects(listasApi.listar(), /No se pudo completar/);
});

test('permite cancelar una lectura al salir o cambiar de lista', async t => {
  const controller = new AbortController();
  controller.abort();
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    assert.equal(init.signal, controller.signal);
    init.signal.throwIfAborted();
  });
  await assert.rejects(listasApi.obtener('lista-1', controller.signal), { name: 'AbortError' });
});
