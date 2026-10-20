import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/features/chat/chat-api.ts', import.meta.url), 'utf8');
async function client(base) {
  // Sustituye únicamente la variable inyectada por Vite; ejecuta el cliente real.
  const { outputText } = ts.transpileModule(source.replace('import.meta.env.VITE_API_URL', JSON.stringify(base)), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}
const { sendChatMessage, ChatApiError } = await client('/api/v1/');
const producto = { id: 'producto-1', nombre: 'Arroz', supermercado: 'Jumbo', precio: 1500, en_stock: true };

test('envía mensaje al Gateway con cookie y conserva respuesta/productos del contrato real', async t => {
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    assert.equal(url, '/api/v1/chat');
    assert.equal(init.method, 'POST');
    assert.equal(init.credentials, 'include');
    assert.deepEqual(init.headers, { 'Content-Type': 'application/json' });
    assert.deepEqual(JSON.parse(init.body), { mensaje: 'arroz económico' });
    assert.ok(init.signal instanceof AbortSignal);
    return Response.json({ respuesta: 'Encontré arroz.', productos: [producto], productos_recomendados: [producto] });
  });
  assert.deepEqual(await sendChatMessage('  arroz económico  '), { respuesta: 'Encontré arroz.', productos: [producto] });
});

test('sin configuración usa /api/v1; permite respuesta vacía de productos y el alias recomendado', async t => {
  const api = await client('');
  t.mock.method(globalThis, 'fetch', async url => {
    assert.equal(url, '/api/v1/chat');
    return Response.json({ respuesta: 'Reformula tu búsqueda.', productos_recomendados: [] });
  });
  assert.deepEqual(await api.sendChatMessage('hola'), { respuesta: 'Reformula tu búsqueda.', productos: [] });
});

test('distingue sesión expirada, Gateway pendiente, validación y fallos de servicio incluso con HTML', async t => {
  for (const [status, expected] of [
    [401, /sesión/], [403, /permiso/], [404, /Gateway/], [422, /no es válido/],
    [429, /demasiadas/], [502, /conectar/], [503, /no está disponible/], [504, /demasiado/], [500, /procesar/],
  ]) {
    const mock = t.mock.method(globalThis, 'fetch', async () => new Response('<html>Error</html>', { status }));
    await assert.rejects(sendChatMessage('arroz'), error => {
      assert.ok(error instanceof ChatApiError);
      assert.equal(error.status, status);
      assert.match(error.message, expected);
      return true;
    });
    mock.mock.restore();
  }
});

test('rechaza respuestas inválidas y no construye productos a partir del texto del LLM', async t => {
  for (const body of [
    null, { explicacion: 'Contrato antiguo', productos: [] },
    { respuesta: '', productos: [] }, { respuesta: 'Arroz $1000' },
    { respuesta: 'Arroz', productos: [{ nombre: 'Inventado' }] },
    { respuesta: 'Arroz', productos: [{ ...producto, precio: '1500' }] },
  ]) {
    const mock = t.mock.method(globalThis, 'fetch', async () => Response.json(body));
    await assert.rejects(sendChatMessage('arroz'), /respuesta inválida/);
    mock.mock.restore();
  }
  t.mock.method(globalThis, 'fetch', async () => new Response('<html>SPA</html>'));
  await assert.rejects(sendChatMessage('arroz'), /respuesta inválida/);
});

test('rechaza mensajes vacíos o largos antes de realizar la petición', async t => {
  const mock = t.mock.method(globalThis, 'fetch');
  await assert.rejects(sendChatMessage('   '), /entre 1 y 1000/);
  await assert.rejects(sendChatMessage('a'.repeat(1001)), /entre 1 y 1000/);
  assert.equal(mock.mock.callCount(), 0);
});

test('propaga cancelación para que vaciar/salir no produzca un mensaje tardío', async t => {
  const controller = new AbortController();
  t.mock.method(globalThis, 'fetch', (_url, init) => new Promise((_resolve, reject) => {
    init.signal.addEventListener('abort', () => reject(init.signal.reason), { once: true });
  }));
  const pending = sendChatMessage('arroz', controller.signal);
  controller.abort();
  await assert.rejects(pending, { name: 'AbortError' });
});

test('traduce timeout y fallo de red a errores comprensibles', async t => {
  const mock = t.mock.method(globalThis, 'fetch', async () => { throw new DOMException('Timeout', 'TimeoutError'); });
  await assert.rejects(sendChatMessage('arroz'), /demasiado/);
  mock.mock.restore();
  t.mock.method(globalThis, 'fetch', async () => { throw new TypeError('Failed to fetch'); });
  await assert.rejects(sendChatMessage('arroz'), /Revisa tu conexión/);
});
