// Ejecuta handleSend con estado en memoria, sin añadir un entorno DOM a la suite.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const moduleUrl = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const reactUrl = moduleUrl(`
  export const states = [];
  export function useState(initial) {
    const index = states.length;
    states.push(typeof initial === 'function' ? initial() : initial);
    return [states[index], value => {
      states[index] = typeof value === 'function' ? value(states[index]) : value;
    }];
  }
  export const useRef = () => ({ current: null });
  export const useEffect = () => {};
`);
const authUrl = moduleUrl('export const useAuth = () => ({ isGuest: false, user: null });');
const source = await readFile(new URL('../src/hooks/useChatbot.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(
  source.replace("from 'react'", `from '${reactUrl}'`)
    .replace("from '@/hooks/useAuth'", `from '${authUrl}'`)
    .replace('import.meta.env.VITE_IA_URL', '"http://localhost:8002"'),
  { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } },
);
const { useChatbot } = await import(moduleUrl(outputText));
const { states } = await import(reactUrl);
const ERROR_IA = 'Lo siento, en este momento no puedo procesar tu solicitud. Por favor intenta más tarde.';

async function send() {
  states.length = 0;
  await useChatbot().handleSend('Quiero hacer pebre');
  // Estas posiciones siguen el orden de useState en useChatbot: isProcessing y messages.
  // Si cambia ese orden, hay que actualizar este adaptador; no implica un fallo del chat.
  assert.equal(states[2], false, 'libera el estado de procesamiento');
  assert.ok(states[3].every(message => message.type !== 'processing'));
  return states[3].at(-1).content;
}

test('un HTTP no exitoso nunca se presenta como respuesta real del asistente', async prueba => {
  for (const status of [401, 403, 404, 500, 502]) {
    for (const body of [{ detail: 'Not Found' }, { explicacion: 'Respuesta que no debe mostrarse' }]) {
      const mock = prueba.mock.method(globalThis, 'fetch', async () => Response.json(body, { status }));
      assert.equal(await send(), ERROR_IA);
      mock.mock.restore();
    }
  }
  prueba.mock.method(globalThis, 'fetch', async () => new Response('<html>Bad gateway</html>', { status: 502 }));
  assert.equal(await send(), ERROR_IA);
});

test('un JSON sin explicación válida no se convierte en un éxito inventado', async prueba => {
  for (const body of [{}, null, { explicacion: '' }, { explicacion: '  ' }, { explicacion: 123 }, { error: 'No disponible', explicacion: 'Ignorar' }]) {
    const mock = prueba.mock.method(globalThis, 'fetch', async () => Response.json(body));
    assert.equal(await send(), ERROR_IA);
    mock.mock.restore();
  }
});

test('conserva la explicación real de una respuesta exitosa', async prueba => {
  prueba.mock.method(globalThis, 'fetch', async () => Response.json({ explicacion: 'Necesitas tomate y cebolla.' }));
  assert.equal(await send(), 'Necesitas tomate y cebolla.');
});
