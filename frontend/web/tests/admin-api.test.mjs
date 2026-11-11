// Protege la lectura de datos reales y la propagación de errores de permisos y conexión en administración.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const source = await readFile(new URL('../src/features/admin/admin-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const api = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('administración conserva usuarios, fuentes y métricas reales del backend', async t => {
  const calls = [];
  const user = {
    id: '00000000-0000-4000-8000-000000000001',
    correo: 'prueba@example.com', nombre_completo: 'Cuenta de prueba',
    rol: 'registrado', esta_activo: true,
  };
  t.mock.method(globalThis, 'fetch', async (url, init) => {
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

test('los errores de administración no se convierten en registros de ejemplo', async t => {
  for (const status of [401, 403, 404, 500]) {
    const mock = t.mock.method(globalThis, 'fetch', async () => Response.json({ mensaje: 'No disponible' }, { status }));
    for (const load of [api.fetchAdminUsers, api.fetchAdminSources, api.fetchIdentidadMetrics, api.fetchCatalogoMetrics, api.fetchControlMetrics]) {
      await assert.rejects(() => load(), error => {
        assert.equal(error.status, status);
        assert.equal(api.isAdminAuthError(error), status === 401 || status === 403);
        return true;
      });
    }
    mock.mock.restore();
  }
  t.mock.method(globalThis, 'fetch', async () => { throw new TypeError('Failed to fetch'); });
  await assert.rejects(api.fetchAdminUsers(), /Failed to fetch/);
  await assert.rejects(api.fetchAdminSources(), /Failed to fetch/);
});
