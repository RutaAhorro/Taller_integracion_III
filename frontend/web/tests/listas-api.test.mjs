import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

// Ejecuta el cliente real con la variable que Vite sustituye al compilar.
const codigoFuente = await readFile(new URL('../src/features/listas/listas-api.ts', import.meta.url), 'utf8');
const { outputText: codigoTranspilado } = ts.transpileModule(codigoFuente.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const { listasApi } = await import(`data:text/javascript;base64,${Buffer.from(codigoTranspilado).toString('base64')}`);

test('CRUD usa Gateway y cookie, mantiene los datos de la API y acepta DELETE 204', async prueba => {
  const lista = { id: 'lista-1', nombre: 'Semana', estado: 'activa', total_articulos: 0 };
  const llamadas = [];
  prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
    llamadas.push({ url, ...opciones });
    if (opciones.method === 'DELETE') return new Response(null, { status: 204 });
    const cuerpo = opciones.method ? lista : url.endsWith('/lista-1') ? { ...lista, articulos: [] } : { listas: [lista] };
    return Response.json(cuerpo);
  });
  assert.deepEqual(await listasApi.listar(), { listas: [lista] });
  assert.deepEqual(await listasApi.crear('Semana'), lista);
  assert.deepEqual(await listasApi.obtener('lista-1'), { ...lista, articulos: [] });
  await listasApi.actualizar('lista-1', 'Semana', 'archivada');
  assert.equal(await listasApi.eliminar('lista-1'), undefined);
  assert.ok(llamadas.every(llamada => llamada.credentials === 'include' && llamada.url.startsWith('/api/v1/listas')));
  assert.equal(llamadas[0].headers, undefined);
  assert.deepEqual(JSON.parse(llamadas[1].body), { nombre: 'Semana' });
  assert.deepEqual(JSON.parse(llamadas[3].body), { nombre: 'Semana', estado: 'archivada' });
  assert.equal(llamadas[3].method, 'PUT');
});

test('propaga errores reales sin reemplazarlos por éxito o datos simulados', async prueba => {
  for (const [estado, cuerpo, esperado] of [
    [401, {}, /sesión expiró/],
    [404, { mensaje: 'La lista no existe.' }, /La lista no existe/],
    [409, { mensaje: 'Alcanzaste el máximo de 100 listas.' }, /máximo de 100/],
    [422, { mensaje: 'Datos inválidos.' }, /Datos inválidos/],
  ]) {
    const simulacion = prueba.mock.method(globalThis, 'fetch', async () => Response.json(cuerpo, { status: estado }));
    await assert.rejects(listasApi.crear('Semana'), esperado);
    simulacion.mock.restore();
  }
  prueba.mock.method(globalThis, 'fetch', async () => new Response('<html>Bad gateway</html>', { status: 502 }));
  await assert.rejects(listasApi.listar(), /No se pudo completar/);
});

test('permite cancelar una lectura al salir o cambiar de lista', async prueba => {
  const controlador = new AbortController();
  controlador.abort();
  prueba.mock.method(globalThis, 'fetch', async (_url, opciones) => {
    assert.equal(opciones.signal, controlador.signal);
    opciones.signal.throwIfAborted();
  });
  await assert.rejects(listasApi.obtener('lista-1', controlador.signal), { name: 'AbortError' });
});
