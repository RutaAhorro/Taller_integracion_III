// Verifica el cierre de sesión real y el fallo seguro, sin navegador ni dependencias adicionales.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const crearUrlModulo = codigo => `data:text/javascript;base64,${Buffer.from(codigo).toString('base64')}`;
const urlReact = crearUrlModulo(`
  export const estados = [];
  export const createContext = () => ({ Provider: 'auth-provider' });
  export function useState(inicial) {
    const indice = estados.length;
    estados.push(typeof inicial === 'function' ? inicial() : inicial);
    return [estados[indice], valor => { estados[indice] = valor; }];
  }
  export const useEffect = () => {};
  export const useRef = inicial => ({ current: inicial });
  export const useCallback = funcion => funcion;
`);
const urlAvisos = crearUrlModulo(`
  export const avisos = [];
  export const useToast = () => ({ toast: (mensaje, tipo) => avisos.push({ message: mensaje, type: tipo }) });
`);
const codigoFuente = await readFile(new URL('../src/features/autenticacion/AuthContext.tsx', import.meta.url), 'utf8');
const { outputText: codigoTranspilado } = ts.transpileModule(
  codigoFuente.replace("from 'react'", `from '${urlReact}'`)
    .replace("from '@/components/ToastContext'", `from '${urlAvisos}'`)
    .replace('import.meta.env.VITE_API_URL', '"/api/v1/"'),
  { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, jsx: ts.JsxEmit.ReactJSX } },
);
const { AuthProvider } = await import(crearUrlModulo(
  codigoTranspilado.replace('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime'))),
));
const { estados } = await import(urlReact);
const { avisos } = await import(urlAvisos);
const descriptorAlmacenamiento = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');

function prepararSesion(prueba) {
  estados.length = 0;
  avisos.length = 0;
  const usuario = { id: 'cuenta-prueba', nombre_completo: 'Cuenta de prueba' };
  const almacenado = new Map([['auth_user', JSON.stringify(usuario)]]);
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: clave => almacenado.get(clave) ?? null,
      setItem: (clave, valor) => almacenado.set(clave, valor),
      removeItem: clave => almacenado.delete(clave),
    },
  });
  prueba.after(() => {
    if (descriptorAlmacenamiento) Object.defineProperty(globalThis, 'localStorage', descriptorAlmacenamiento);
    else delete globalThis.localStorage;
  });
  return { almacenado, autenticacion: AuthProvider({ children: null }).props.value, usuario };
}

test('logout espera el 204 sin leer JSON, comparte petición y elimina el usuario local', async prueba => {
  const { autenticacion, almacenado, usuario } = prepararSesion(prueba);
  let resolver;
  let llamadas = 0;
  prueba.mock.method(globalThis, 'fetch', (url, opciones) => {
    llamadas += 1;
    assert.equal(url, '/api/v1/auth/logout');
    assert.deepEqual(opciones, { method: 'POST', credentials: 'include' });
    if (llamadas > 1) return Promise.resolve(new Response(null, { status: 204 }));
    return new Promise(completar => { resolver = completar; });
  });
  const primeraPeticion = autenticacion.logout();
  assert.equal(autenticacion.logout(), primeraPeticion);
  assert.deepEqual(estados[0], usuario, 'no limpia la cuenta antes de recibir respuesta');
  assert.equal(almacenado.has('auth_user'), true);
  // El adaptador sigue el orden de estado del proveedor: usuario, error, carga y cierre.
  assert.equal(estados[3], true);
  const respuesta = new Response(null, { status: 204 });
  respuesta.json = () => { throw new Error('Un 204 no contiene JSON'); };
  resolver(respuesta);
  await primeraPeticion;
  assert.equal(llamadas, 1);
  assert.equal(estados[0], null);
  assert.equal(estados[1], null);
  assert.equal(estados[3], false);
  assert.equal(almacenado.has('auth_user'), false);
  assert.deepEqual(avisos, []);
  await autenticacion.logout(); // Libera la petición compartida y permite otro intento.
  assert.equal(llamadas, 2);
});

test('un fallo HTTP o de red limpia la sesión local y advierte sin fingir cierre del servidor', async prueba => {
  for (const estado of [200, 401, 403, 502, null]) {
    const { autenticacion, almacenado } = prepararSesion(prueba);
    const simulacion = prueba.mock.method(globalThis, 'fetch', async () => {
      if (estado === null) throw new TypeError('Sin conexión');
      return new Response('<html>Error</html>', { status: estado });
    });
    await autenticacion.logout();
    assert.equal(estados[0], null);
    assert.equal(estados[3], false);
    assert.equal(almacenado.has('auth_user'), false);
    assert.match(estados[1], /no se pudo confirmar el cierre en el servidor/);
    assert.equal(avisos.length, 1);
    assert.equal(avisos[0].type, 'warning');
    assert.match(avisos[0].message, /cookie podría seguir activa/);
    simulacion.mock.restore();
  }
});
