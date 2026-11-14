// Comprueba el destino con los roles reales, sin credenciales ni cuentas del clúster.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const moduleUrl = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const authUrl = moduleUrl(`
  export const session = { user: null };
  export const useAuth = () => ({ user: session.user, isGuest: !session.user?.id });
`);

async function load(path, replacements = []) {
  let source = await readFile(new URL(path, import.meta.url), 'utf8');
  for (const [from, to] of replacements) source = source.replaceAll(from, to);
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  return moduleUrl(outputText.replace('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime'))));
}

const rolesUrl = await load('../src/lib/roles.ts');
const roleHookUrl = await load('../src/hooks/useRole.ts', [
  ["'./useAuth'", JSON.stringify(authUrl)],
  ["'@/lib/roles'", JSON.stringify(rolesUrl)],
]);
const routesUrl = await load('../src/core/routes.ts');
const componentsUrl = moduleUrl('export const AuthVisual = () => null; export const AuthPanel = () => null; export const Navigate = () => null;');
const loginUrl = await load('../src/pages/Login.tsx', [
  ['"@/components/auth/AuthVisual"', JSON.stringify(componentsUrl)],
  ['"@/components/auth/AuthPanel"', JSON.stringify(componentsUrl)],
  ['"react-router-dom"', JSON.stringify(componentsUrl)],
  ['"@/hooks/useRole"', JSON.stringify(roleHookUrl)],
  ['"@/core/routes"', JSON.stringify(routesUrl)],
]);
const { default: Login } = await import(loginUrl);
const { session } = await import(authUrl);
const { Navigate } = await import(componentsUrl);

test('el invitado ve el formulario, incluso sin una identidad válida que respalde el rol', () => {
  for (const user of [null, { rol: 'admin' }]) {
    session.user = user;
    assert.equal(Login().type, 'main');
  }
});

test('el destino depende del rol recibido, no del correo ni de la sesión anterior', () => {
  for (const [rol, expected] of [
    ['registrado', '/dashboard'], ['admin', '/admin'], [' ADMIN ', '/admin'],
    ['colaborador', '/dashboard'], ['', '/dashboard'], ['desconocido', '/dashboard'],
  ]) {
    session.user = { id: 'cuenta-prueba', correo: 'cuenta@example.com', rol };
    const result = Login();
    assert.equal(result.type, Navigate);
    assert.equal(result.props.to, expected);
    assert.equal(result.props.replace, true);
  }
});
