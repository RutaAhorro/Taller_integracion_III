// Protege la lectura de datos reales y la propagación de errores de permisos y conexión en administración.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const codigoFuente = await readFile(new URL('../src/features/admin/admin-api.ts', import.meta.url), 'utf8');
const { outputText: codigoTranspilado } = ts.transpileModule(codigoFuente.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const crearUrlModulo = codigo => `data:text/javascript;base64,${Buffer.from(codigo).toString('base64')}`;
const urlApi = crearUrlModulo(codigoTranspilado);
const api = await import(urlApi);

test('administración conserva usuarios, fuentes y métricas reales del backend', async prueba => {
  const llamadas = [];
  const usuario = {
    id: '00000000-0000-4000-8000-000000000001',
    correo: 'prueba@example.com', nombre_completo: 'Cuenta de prueba',
    rol: 'registrado', esta_activo: true,
  };
  prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
    llamadas.push({ url, ...opciones });
    if (url.endsWith('/usuarios/metricas')) {
      return Response.json({ total_usuarios: 0, usuarios_activos: 0 });
    }
    if (url.endsWith('/fuentes')) return Response.json([]);
    return Response.json({
      data: [usuario], paginacion: { total_registros: 1, pagina_actual: 1, total_paginas: 1, limite: 20 },
    });
  });

  const resultado = await api.fetchAdminUsers();
  assert.equal(resultado.users.length, 1);
  assert.equal(resultado.users[0].id, usuario.id);
  assert.equal(resultado.users[0].name, usuario.nombre_completo);
  assert.equal(resultado.users[0].plan, null);
  assert.equal(resultado.users[0].lastAccess, 'Sin registros');
  assert.deepEqual(await api.fetchAdminSources(), []);
  assert.deepEqual(await api.fetchIdentidadMetrics(), { totalUsuarios: 0, usuariosActivos: 0 });
  assert.ok(llamadas.every(llamada => llamada.credentials === 'include' && llamada.url.startsWith('/api/v1/')));
});

test('los errores de administración no se convierten en registros de ejemplo', async prueba => {
  for (const estado of [401, 403, 404, 500]) {
    const simulacion = prueba.mock.method(globalThis, 'fetch', async () => Response.json({ mensaje: 'No disponible' }, { status: estado }));
    for (const cargarDatos of [api.fetchAdminUsers, api.fetchAdminSources, api.fetchIdentidadMetrics, api.fetchCatalogoMetrics, api.fetchControlMetrics]) {
      await assert.rejects(() => cargarDatos(), errorObtenido => {
        assert.equal(errorObtenido.status, estado);
        assert.equal(api.isAdminAuthError(errorObtenido), estado === 401 || estado === 403);
        return true;
      });
    }
    simulacion.mock.restore();
  }
  prueba.mock.method(globalThis, 'fetch', async () => { throw new TypeError('Failed to fetch'); });
  await assert.rejects(api.fetchAdminUsers(), /Failed to fetch/);
  await assert.rejects(api.fetchAdminSources(), /Failed to fetch/);
});

test('los cambios de usuario y fuente usan PUT con cookie y conservan lo confirmado por el servidor', async prueba => {
  const usuario = {
    id: '00000000-0000-4000-8000-000000000001',
    correo: 'prueba@example.com', nombre_completo: 'Cuenta de prueba',
    rol: 'registrado', plan: 'basico', esta_activo: true,
  };
  const fuente = { id: 1, nombre: 'Fuente de prueba', habilitada: true, productos: 12, sucursales: 0 };
  prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
    assert.equal(opciones.credentials, 'include');
    if (opciones.method === 'PUT') {
      assert.equal(opciones.headers['Content-Type'], 'application/json');
      Object.assign(url.includes('/usuarios/') ? usuario : fuente, JSON.parse(opciones.body));
      return Response.json(url.includes('/usuarios/') ? usuario : fuente);
    }
    if (url.endsWith('/fuentes')) return Response.json([fuente]);
    return Response.json({
      data: [usuario], paginacion: { total_registros: 1, pagina_actual: 1, total_paginas: 1, limite: 20 },
    });
  });
  assert.equal((await api.updateAdminUser(usuario.id, { esta_activo: false })).status, 'Inactivo');
  assert.equal((await api.updateAdminUser(usuario.id, { rol: 'colaborador' })).role, 'Colaborador');
  assert.equal((await api.updateAdminUser(usuario.id, { plan: 'premium' })).plan, 'Premium');
  assert.equal((await api.fetchAdminUsers()).users[0].role, 'Colaborador');
  const actualizada = await api.updateAdminSource('1', { habilitada: false });
  assert.equal(actualizada.habilitada, false);
  assert.equal(actualizada.branches, 0);
  assert.deepEqual((await api.fetchAdminSources())[0], actualizada);
});

test('las escrituras propagan rechazos y los mensajes de Identidad y Catálogo', async prueba => {
  for (const [estado, cuerpo, mensaje] of [
    [409, { mensaje: 'No se puede degradar al último administrador.' }, 'último administrador'],
    [404, { error: 'Fuente no encontrada' }, 'Fuente no encontrada'],
    [500, { error: 'Error interno al actualizar fuente' }, 'Error interno'],
  ]) {
    const simulacion = prueba.mock.method(globalThis, 'fetch', async () => Response.json(cuerpo, { status: estado }));
    await assert.rejects(api.updateAdminUser('cuenta', { rol: 'registrado' }), new RegExp(mensaje));
    await assert.rejects(api.updateAdminSource('1', { habilitada: false }), new RegExp(mensaje));
    simulacion.mock.restore();
  }
  for (const estado of [401, 403]) {
    const simulacion = prueba.mock.method(globalThis, 'fetch', async () => new Response(null, { status: estado }));
    await assert.rejects(api.updateAdminSource('1', { habilitada: false }), api.isAdminAuthError);
    simulacion.mock.restore();
  }
});

test('fuentes descarta lecturas antiguas, serializa cambios y revierte un guardado fallido', async prueba => {
  // Adaptador mínimo del hook, sin DOM ni nuevas dependencias.
  const urlReact = crearUrlModulo(`
    export const estados = [], referencias = [];
    let indiceEstado = 0, indiceReferencia = 0;
    export function prepararRender() { indiceEstado = 0; indiceReferencia = 0; }
    export function useState(inicial) {
      const indice = indiceEstado++;
      if (!(indice in estados)) estados[indice] = inicial;
      return [estados[indice], valor => { estados[indice] = typeof valor === 'function' ? valor(estados[indice]) : valor; }];
    }
    export function useRef(inicial) {
      const indice = indiceReferencia++;
      return referencias[indice] ??= { current: inicial };
    }
    export const useEffect = () => {};
    export const useCallback = funcion => funcion;
  `);
  const urlSesion = crearUrlModulo(`
    export const useAuth = () => ({ logout() {} });
    export const useNavigate = () => () => {};
    export const ROUTES = { LOGIN: '/login' };
  `);
  const codigoHook = await readFile(new URL('../src/features/admin/useAdminData.ts', import.meta.url), 'utf8');
  const compilado = ts.transpileModule(
    codigoHook.replace("'react'", JSON.stringify(urlReact))
      .replace("'react-router-dom'", JSON.stringify(urlSesion))
      .replace("'@/features/autenticacion/useAuth'", JSON.stringify(urlSesion))
      .replace("'@/app/routes'", JSON.stringify(urlSesion))
      .replace("'./admin-api'", JSON.stringify(urlApi)),
    { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } },
  );
  const { useAdminSources } = await import(crearUrlModulo(compilado.outputText));
  const { prepararRender } = await import(urlReact);
  const leerVista = () => { prepararRender(); return useAdminSources(); };
  const vaciarCola = () => new Promise(resolver => setImmediate(resolver));
  const fuente = { id: 1, nombre: 'Fuente de prueba', habilitada: true, productos: 1, sucursales: 1 };
  let resolverLectura, resolverEscritura;
  let lecturas = 0, escrituras = 0;
  prueba.mock.method(globalThis, 'fetch', (_url, opciones) => {
    if (opciones.method === 'PUT') {
      escrituras++;
      return new Promise(resolver => { resolverEscritura = resolver; });
    }
    lecturas++;
    if (lecturas === 1) return Promise.resolve(Response.json([fuente]));
    return new Promise(resolver => { resolverLectura = resolver; });
  });
  leerVista().refresh();
  await vaciarCola();
  leerVista().refresh(); // Deja en vuelo un GET con el estado anterior al cambio.
  const vistaCargada = leerVista();
  const guardando = vistaCargada.toggleSource('1');
  await vistaCargada.toggleSource('1');
  leerVista().refresh();
  assert.equal(escrituras, 1);
  assert.equal(lecturas, 2);
  const confirmada = { ...fuente, habilitada: false, productos: 12 };
  resolverEscritura(Response.json(confirmada));
  await guardando;
  resolverLectura(Response.json([fuente]));
  await vaciarCola();
  assert.deepEqual(leerVista().sources, [api.adminSourceFromBackend(confirmada)]);
  assert.equal(leerVista().error, null);
  assert.equal(leerVista().pendingId, null);
  const rechazada = leerVista().toggleSource('1');
  resolverEscritura(Response.json({ error: 'Error al guardar' }, { status: 500 }));
  await rechazada;
  assert.deepEqual(leerVista().sources, [api.adminSourceFromBackend(confirmada)]);
  assert.match(leerVista().error, /Error al guardar/);
  assert.equal(leerVista().pendingId, null);
});

test('rol y plan solo aceptan los valores reales de Identidad, sin alias heredados', () => {
  assert.equal(api.adminRoleFromBackend('admin'), 'Administrador');
  assert.equal(api.adminRoleFromBackend('registrado'), 'Usuario');
  assert.equal(api.adminRoleFromBackend('administrador'), 'Desconocido');
  assert.equal(api.adminRoleFromBackend('usuario'), 'Desconocido');
  assert.equal(api.adminPlanFromBackend('basico'), 'Básico');
  assert.equal(api.adminPlanFromBackend('premium'), 'Premium');
  assert.equal(api.adminPlanFromBackend('plus'), null);
  assert.equal(api.adminPlanFromBackend('básico'), null);
});

test('fuentes habilitadas sin productos se muestran como Sin datos', () => {
  const casos = [
    { habilitada: true, productos: 0, estadoEsperado: 'Sin datos' },
    { habilitada: true, productos: 5, estadoEsperado: 'Operativa' },
    { habilitada: false, productos: 0, estadoEsperado: 'Inactiva' },
    { habilitada: false, productos: 5, estadoEsperado: 'Inactiva' },
  ];

  for (const caso of casos) {
    const fuente = api.adminSourceFromBackend({
      id: 1,
      nombre: 'Fuente de prueba',
      habilitada: caso.habilitada,
      sucursales: 0,
      productos: caso.productos,
    });
    assert.equal(fuente.status, caso.estadoEsperado);
  }
});
