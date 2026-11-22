// Protege el acceso a /login con usuario local antiguo y el destino tras autenticar.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const crearUrlModulo = codigoFuente => `data:text/javascript;base64,${Buffer.from(codigoFuente).toString('base64')}`;
const urlReact = crearUrlModulo(`
  export const estados = [];
  export function useState(inicial) {
    const indice = estados.length;
    estados.push(typeof inicial === 'function' ? inicial() : inicial);
    return [estados[indice], valor => {
      estados[indice] = typeof valor === 'function' ? valor(estados[indice]) : valor;
    }];
  }
  export const useEffect = () => {};
  export const useRef = inicial => ({ current: inicial });
  export const useCallback = funcion => funcion;
  export const createContext = () => ({ Provider: 'proveedor' });
`);
const entornoUrl = crearUrlModulo(`
  export const sesion = { contexto: null };
  export const navegaciones = [];
  export const almacenamiento = {
    usuario: null,
    getItem: () => almacenamiento.usuario ? JSON.stringify(almacenamiento.usuario) : null,
    setItem: () => {},
    removeItem: () => {},
  };
  export const useAuth = () => sesion.contexto;
  export const useToast = () => ({ toast: () => {} });
  export const useNavigate = () => (destino, opciones) => navegaciones.push({ destino, opciones });
`);

async function cargar(ruta, reemplazos = []) {
  let codigo = await readFile(new URL(ruta, import.meta.url), 'utf8');
  for (const [desde, hasta] of reemplazos) codigo = codigo.replaceAll(desde, hasta);
  const { outputText: codigoTranspilado } = ts.transpileModule(codigo, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  return crearUrlModulo(codigoTranspilado.replace('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime'))));
}

const rolesUrl = await cargar('../src/features/autenticacion/roles.ts');
const rutasUrl = await cargar('../src/app/routes.ts', [["'@/features/autenticacion/roles'", JSON.stringify(rolesUrl)]]);
const contextoUrl = await cargar('../src/features/autenticacion/AuthContext.tsx', [
  ["'react'", JSON.stringify(urlReact)],
  ["'@/components/ToastContext'", JSON.stringify(entornoUrl)],
  ["const CLAVE_USUARIO_AUTENTICADO = 'auth_user';", `import { almacenamiento } from '${entornoUrl}'; const CLAVE_USUARIO_AUTENTICADO = 'auth_user';`],
  ['localStorage.', 'almacenamiento.'],
  ['import.meta.env.VITE_API_URL', '"/api/v1"'],
]);
const googleUrl = await cargar('../src/features/autenticacion/useGoogleAuth.ts', [
  ["'react'", JSON.stringify(urlReact)],
  ["'@/features/autenticacion/useAuth'", JSON.stringify(entornoUrl)],
  ["'react-router-dom'", JSON.stringify(entornoUrl)],
  ["'@/app/routes'", JSON.stringify(rutasUrl)],
]);
const componentesUrl = crearUrlModulo('export const AuthVisual = () => null; export const AuthPanel = () => null;');
const urlAcceso = await cargar('../src/features/autenticacion/Login.tsx', [
  ['"@/features/autenticacion/AuthVisual"', JSON.stringify(componentesUrl)],
  ['"@/features/autenticacion/AuthPanel"', JSON.stringify(componentesUrl)],
]);
const { default: Login } = await import(urlAcceso);
const { AuthProvider } = await import(contextoUrl);
const { useGoogleAuth } = await import(googleUrl);
const { rutaTrasAutenticacion } = await import(rutasUrl);
const { estados } = await import(urlReact);
const { sesion, navegaciones, almacenamiento } = await import(entornoUrl);

function iniciarContexto(usuarioGuardado) {
  estados.length = 0;
  navegaciones.length = 0;
  almacenamiento.usuario = usuarioGuardado;
  sesion.contexto = AuthProvider({ children: null }).props.value;
  return sesion.contexto;
}

test('entrar a /login muestra el formulario aunque auth_user conserve una sesión antigua', () => {
  for (const usuario of [null, { id: 'antiguo', rol: 'registrado' }, { id: 'antiguo', rol: 'admin' }]) {
    const contexto = iniciarContexto(usuario);
    assert.deepEqual(contexto.user, usuario);
    const pagina = Login();
    assert.equal(pagina.type, 'main');
    assert.equal(pagina.props.children[1].type.name, 'AuthPanel');
    assert.deepEqual(navegaciones, []);
  }
});

test('login devuelve la identidad nueva y el destino depende de su rol, no del usuario guardado', async prueba => {
  for (const [rol, destino] of [
    ['registrado', '/dashboard'], ['admin', '/admin'], [' ADMIN ', '/admin'],
    ['colaborador', '/dashboard'], ['', '/dashboard'], ['desconocido', '/dashboard'],
  ]) {
    const contexto = iniciarContexto({ id: 'anterior', rol: rol === 'admin' ? 'registrado' : 'admin' });
    const usuario = { id: 'nuevo', rol };
    const peticion = prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
      assert.equal(url, '/api/v1/auth/login');
      assert.equal(opciones.credentials, 'include');
      return Response.json({ usuario });
    });
    const recibido = await contexto.login({ correo: 'cuenta@example.com', password: 'clave-de-prueba' });
    assert.deepEqual(recibido, usuario);
    assert.equal(rutaTrasAutenticacion(recibido.rol), destino);
    peticion.mock.restore();
  }
});

test('Google redirige solo después de una respuesta exitosa y usa el mismo criterio de rol', async prueba => {
  for (const [rol, destino] of [['admin', '/admin'], ['registrado', '/dashboard']]) {
    iniciarContexto({ id: 'anterior', rol: rol === 'admin' ? 'registrado' : 'admin' });
    const peticion = prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
      assert.equal(url, '/api/v1/auth/google');
      assert.equal(opciones.credentials, 'include');
      return Response.json({ usuario: { id: 'nuevo', rol } });
    });
    const google = useGoogleAuth();
    assert.deepEqual(navegaciones, []);
    await google.handleGoogleSuccess({ credential: 'credencial-de-prueba' });
    assert.deepEqual(navegaciones, [{ destino, opciones: { replace: true } }]);
    peticion.mock.restore();
  }
});

test('un login rechazado no devuelve la identidad antigua ni Google redirige por ella', async prueba => {
  const contexto = iniciarContexto({ id: 'antiguo', rol: 'admin' });
  prueba.mock.method(globalThis, 'fetch', async () => Response.json({ mensaje: 'Credenciales inválidas.' }, { status: 401 }));
  assert.equal(await contexto.login({ correo: 'cuenta@example.com', password: 'incorrecta' }), null);
  await useGoogleAuth().handleGoogleSuccess({ credential: 'credencial-rechazada' });
  assert.deepEqual(navegaciones, []);
});

test('el registro conserva su resultado booleano y una identidad inválida no autentica', async prueba => {
  const contexto = iniciarContexto(null);
  prueba.mock.method(globalThis, 'fetch', async url => url.endsWith('/register')
    ? Response.json({ mensaje: 'Cuenta creada.' }, { status: 201 })
    : Response.json({ mensaje: 'Sin identidad.' }));
  assert.equal(await contexto.register({ correo: 'cuenta@example.com', password: 'clave-de-prueba', nombre_completo: 'Cuenta de prueba' }), true);
  assert.equal(await contexto.login({ correo: 'cuenta@example.com', password: 'clave-de-prueba' }), null);
});
