// Protege la lectura de datos reales y la propagación de errores de permisos y conexión en administración.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/features/admin/admin-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const moduleUrl = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const apiUrl = moduleUrl(outputText);
const api = await import(apiUrl);

test('administración conserva usuarios, fuentes y métricas reales del backend', async prueba => {
  const calls = [];
  const user = {
    id: '00000000-0000-4000-8000-000000000001',
    correo: 'prueba@example.com', nombre_completo: 'Cuenta de prueba',
    rol: 'registrado', esta_activo: true,
  };
  prueba.mock.method(globalThis, 'fetch', async (url, init) => {
    calls.push({ url, ...init });
    if (url.endsWith('/usuarios/metricas')) {
      return Response.json({ total_usuarios: 0, usuarios_activos: 0 });
    }
    if (url.endsWith('/fuentes')) return Response.json([]);
    return Response.json({
      data: [user], paginacion: { total_registros: 1, pagina_actual: 1, total_paginas: 1, limite: 20 },
    });
  });

  const result = await api.fetchAdminUsers();
  assert.equal(result.users.length, 1);
  assert.equal(result.users[0].id, user.id);
  assert.equal(result.users[0].name, user.nombre_completo);
  assert.equal(result.users[0].plan, null);
  assert.equal(result.users[0].lastAccess, 'Sin registros');
  assert.deepEqual(await api.fetchAdminSources(), []);
  assert.deepEqual(await api.fetchIdentidadMetrics(), { totalUsuarios: 0, usuariosActivos: 0 });
  assert.ok(calls.every(call => call.credentials === 'include' && call.url.startsWith('/api/v1/')));
});

test('los errores de administración no se convierten en registros de ejemplo', async prueba => {
  for (const status of [401, 403, 404, 500]) {
    const mock = prueba.mock.method(globalThis, 'fetch', async () => Response.json({ mensaje: 'No disponible' }, { status }));
    for (const load of [api.fetchAdminUsers, api.fetchAdminSources, api.fetchIdentidadMetrics, api.fetchCatalogoMetrics, api.fetchControlMetrics]) {
      await assert.rejects(() => load(), error => {
        assert.equal(error.status, status);
        assert.equal(api.isAdminAuthError(error), status === 401 || status === 403);
        return true;
      });
    }
    mock.mock.restore();
  }
  prueba.mock.method(globalThis, 'fetch', async () => { throw new TypeError('Failed to fetch'); });
  await assert.rejects(api.fetchAdminUsers(), /Failed to fetch/);
  await assert.rejects(api.fetchAdminSources(), /Failed to fetch/);
});

test('los cambios de usuario y fuente usan PUT con cookie y conservan lo confirmado por el servidor', async prueba => {
  const user = {
    id: '00000000-0000-4000-8000-000000000001',
    correo: 'prueba@example.com', nombre_completo: 'Cuenta de prueba',
    rol: 'registrado', plan: 'basico', esta_activo: true,
  };
  const fuente = { id: 1, nombre: 'Fuente de prueba', habilitada: true, productos: 12, sucursales: 0 };
  prueba.mock.method(globalThis, 'fetch', async (url, init) => {
    assert.equal(init.credentials, 'include');
    if (init.method === 'PUT') {
      assert.equal(init.headers['Content-Type'], 'application/json');
      Object.assign(url.includes('/usuarios/') ? user : fuente, JSON.parse(init.body));
      return Response.json(url.includes('/usuarios/') ? user : fuente);
    }
    if (url.endsWith('/fuentes')) return Response.json([fuente]);
    return Response.json({
      data: [user], paginacion: { total_registros: 1, pagina_actual: 1, total_paginas: 1, limite: 20 },
    });
  });
  assert.equal((await api.updateAdminUser(user.id, { esta_activo: false })).status, 'Inactivo');
  assert.equal((await api.updateAdminUser(user.id, { rol: 'colaborador' })).role, 'Colaborador');
  assert.equal((await api.updateAdminUser(user.id, { plan: 'premium' })).plan, 'Premium');
  assert.equal((await api.fetchAdminUsers()).users[0].role, 'Colaborador');
  const actualizada = await api.updateAdminSource('1', { habilitada: false });
  assert.equal(actualizada.habilitada, false);
  assert.equal(actualizada.branches, 0);
  assert.deepEqual((await api.fetchAdminSources())[0], actualizada);
});

test('las escrituras propagan rechazos y los mensajes de Identidad y Catálogo', async prueba => {
  for (const [status, body, message] of [
    [409, { mensaje: 'No se puede degradar al último administrador.' }, 'último administrador'],
    [404, { error: 'Fuente no encontrada' }, 'Fuente no encontrada'],
    [500, { error: 'Error interno al actualizar fuente' }, 'Error interno'],
  ]) {
    const mock = prueba.mock.method(globalThis, 'fetch', async () => Response.json(body, { status }));
    await assert.rejects(api.updateAdminUser('cuenta', { rol: 'registrado' }), new RegExp(message));
    await assert.rejects(api.updateAdminSource('1', { habilitada: false }), new RegExp(message));
    mock.mock.restore();
  }
  for (const status of [401, 403]) {
    const mock = prueba.mock.method(globalThis, 'fetch', async () => new Response(null, { status }));
    await assert.rejects(api.updateAdminSource('1', { habilitada: false }), api.isAdminAuthError);
    mock.mock.restore();
  }
});

test('fuentes descarta lecturas antiguas, serializa cambios y revierte un guardado fallido', async prueba => {
  // Adaptador mínimo del hook, sin DOM ni nuevas dependencias.
  const reactUrl = moduleUrl(`
    export const states = [], refs = [];
    let stateIndex = 0, refIndex = 0;
    export function render() { stateIndex = 0; refIndex = 0; }
    export function useState(initial) {
      const index = stateIndex++;
      if (!(index in states)) states[index] = initial;
      return [states[index], value => { states[index] = typeof value === 'function' ? value(states[index]) : value; }];
    }
    export function useRef(initial) {
      const index = refIndex++;
      return refs[index] ??= { current: initial };
    }
    export const useEffect = () => {};
    export const useCallback = callback => callback;
  `);
  const sessionUrl = moduleUrl(`
    export const useAuth = () => ({ logout() {} });
    export const useNavigate = () => () => {};
    export const ROUTES = { LOGIN: '/login' };
  `);
  const hookSource = await readFile(new URL('../src/features/admin/useAdminData.ts', import.meta.url), 'utf8');
  const compiled = ts.transpileModule(
    hookSource.replace("'react'", JSON.stringify(reactUrl))
      .replace("'react-router-dom'", JSON.stringify(sessionUrl))
      .replace("'@/hooks/useAuth'", JSON.stringify(sessionUrl))
      .replace("'@/core/routes'", JSON.stringify(sessionUrl))
      .replace("'./admin-api'", JSON.stringify(apiUrl)),
    { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } },
  );
  const { useAdminSources } = await import(moduleUrl(compiled.outputText));
  const { render } = await import(reactUrl);
  const view = () => { render(); return useAdminSources(); };
  const flush = () => new Promise(resolve => setImmediate(resolve));
  const fuente = { id: 1, nombre: 'Fuente de prueba', habilitada: true, productos: 1, sucursales: 1 };
  let resolveGet, resolvePut;
  let reads = 0, writes = 0;
  prueba.mock.method(globalThis, 'fetch', (url, init) => {
    if (init.method === 'PUT') {
      writes++;
      return new Promise(resolve => { resolvePut = resolve; });
    }
    reads++;
    if (reads === 1) return Promise.resolve(Response.json([fuente]));
    return new Promise(resolve => { resolveGet = resolve; });
  });
  view().refresh();
  await flush();
  view().refresh(); // Deja en vuelo un GET con el estado anterior al cambio.
  const loaded = view();
  const saving = loaded.toggleSource('1');
  await loaded.toggleSource('1');
  view().refresh();
  assert.equal(writes, 1);
  assert.equal(reads, 2);
  const confirmada = { ...fuente, habilitada: false, productos: 12 };
  resolvePut(Response.json(confirmada));
  await saving;
  resolveGet(Response.json([fuente]));
  await flush();
  assert.deepEqual(view().sources, [api.adminSourceFromBackend(confirmada)]);
  assert.equal(view().error, null);
  assert.equal(view().pendingId, null);
  const rejected = view().toggleSource('1');
  resolvePut(Response.json({ error: 'Error al guardar' }, { status: 500 }));
  await rejected;
  assert.deepEqual(view().sources, [api.adminSourceFromBackend(confirmada)]);
  assert.match(view().error, /Error al guardar/);
  assert.equal(view().pendingId, null);
});
