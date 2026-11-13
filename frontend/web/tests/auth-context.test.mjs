// Verifica el cierre de sesión real y el fallo seguro, sin navegador ni dependencias adicionales.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const moduleUrl = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const reactUrl = moduleUrl(`
  export const states = [];
  export const createContext = () => ({ Provider: 'auth-provider' });
  export function useState(initial) {
    const index = states.length;
    states.push(typeof initial === 'function' ? initial() : initial);
    return [states[index], value => { states[index] = value; }];
  }
  export const useEffect = () => {};
  export const useRef = initial => ({ current: initial });
  export const useCallback = callback => callback;
`);
const toastUrl = moduleUrl(`
  export const notices = [];
  export const useToast = () => ({ toast: (message, type) => notices.push({ message, type }) });
`);
const source = await readFile(new URL('../src/contexts/AuthContext.tsx', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(
  source.replace("from 'react'", `from '${reactUrl}'`)
    .replace("from '@/contexts/ToastContext'", `from '${toastUrl}'`)
    .replace('import.meta.env.VITE_API_URL', '"/api/v1/"'),
  { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, jsx: ts.JsxEmit.ReactJSX } },
);
const { AuthProvider } = await import(moduleUrl(
  outputText.replace('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime'))),
));
const { states } = await import(reactUrl);
const { notices } = await import(toastUrl);
const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');

function session(prueba) {
  states.length = 0;
  notices.length = 0;
  const user = { id: 'cuenta-prueba', nombre_completo: 'Cuenta de prueba' };
  const stored = new Map([['auth_user', JSON.stringify(user)]]);
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: key => stored.get(key) ?? null,
      setItem: (key, value) => stored.set(key, value),
      removeItem: key => stored.delete(key),
    },
  });
  prueba.after(() => {
    if (storageDescriptor) Object.defineProperty(globalThis, 'localStorage', storageDescriptor);
    else delete globalThis.localStorage;
  });
  return { stored, auth: AuthProvider({ children: null }).props.value, user };
}

test('logout espera el 204 sin leer JSON, comparte petición y elimina el usuario local', async prueba => {
  const { auth, stored, user } = session(prueba);
  let resolve;
  let calls = 0;
  prueba.mock.method(globalThis, 'fetch', (url, init) => {
    calls += 1;
    assert.equal(url, '/api/v1/auth/logout');
    assert.deepEqual(init, { method: 'POST', credentials: 'include' });
    if (calls > 1) return Promise.resolve(new Response(null, { status: 204 }));
    return new Promise(done => { resolve = done; });
  });
  const first = auth.logout();
  assert.equal(auth.logout(), first);
  assert.deepEqual(states[0], user, 'no limpia la cuenta antes de recibir respuesta');
  assert.equal(stored.has('auth_user'), true);
  // El adaptador sigue el orden de estado del proveedor: usuario, error, carga y cierre.
  assert.equal(states[3], true);
  const response = new Response(null, { status: 204 });
  response.json = () => { throw new Error('Un 204 no contiene JSON'); };
  resolve(response);
  await first;
  assert.equal(calls, 1);
  assert.equal(states[0], null);
  assert.equal(states[1], null);
  assert.equal(states[3], false);
  assert.equal(stored.has('auth_user'), false);
  assert.deepEqual(notices, []);
  await auth.logout(); // Libera la petición compartida y permite otro intento.
  assert.equal(calls, 2);
});

test('un fallo HTTP o de red limpia la sesión local y advierte sin fingir cierre del servidor', async prueba => {
  for (const status of [200, 401, 403, 502, null]) {
    const { auth, stored } = session(prueba);
    const mock = prueba.mock.method(globalThis, 'fetch', async () => {
      if (status === null) throw new TypeError('Sin conexión');
      return new Response('<html>Error</html>', { status });
    });
    await auth.logout();
    assert.equal(states[0], null);
    assert.equal(states[3], false);
    assert.equal(stored.has('auth_user'), false);
    assert.match(states[1], /no se pudo confirmar el cierre en el servidor/);
    assert.equal(notices.length, 1);
    assert.equal(notices[0].type, 'warning');
    assert.match(notices[0].message, /cookie podría seguir activa/);
    mock.mock.restore();
  }
});
