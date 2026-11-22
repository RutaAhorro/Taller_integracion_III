// Ejecuta el hook real con estado y refs en memoria, sin DOM ni llamadas al proveedor.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const crearUrlModulo = codigo => `data:text/javascript;base64,${Buffer.from(codigo).toString('base64')}`;
const urlReact = crearUrlModulo(`
  const estados = [];
  const referencias = [];
  let indiceEstado = 0;
  let indiceReferencia = 0;
  export function prepararRender(reiniciar = false) {
    indiceEstado = 0;
    indiceReferencia = 0;
    if (reiniciar) { estados.length = 0; referencias.length = 0; }
  }
  export function useState(inicial) {
    const indice = indiceEstado++;
    if (indice === estados.length) estados.push(typeof inicial === 'function' ? inicial() : inicial);
    return [estados[indice], valor => {
      estados[indice] = typeof valor === 'function' ? valor(estados[indice]) : valor;
    }];
  }
  export function useRef(inicial) {
    const indice = indiceReferencia++;
    if (indice === referencias.length) referencias.push({ current: inicial });
    return referencias[indice];
  }
  export const useEffect = () => {};
`);
const urlAutenticacion = crearUrlModulo(`
  export const sesion = { isGuest: false };
  export const useAuth = () => sesion;
`);
const codigo = await readFile(new URL('../src/features/chat/useChatbot.ts', import.meta.url), 'utf8');
const { outputText: codigoTranspilado } = ts.transpileModule(
  codigo.replace("from 'react'", `from '${urlReact}'`)
    .replace("from '@/features/autenticacion/useAuth'", `from '${urlAutenticacion}'`)
    .replace('import.meta.env.VITE_API_URL', '"/api/v1/"'),
  { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } },
);
const { useChatbot } = await import(crearUrlModulo(codigoTranspilado));
const { prepararRender } = await import(urlReact);
const { sesion } = await import(urlAutenticacion);
const ERROR_IA = 'Lo siento, en este momento no puedo procesar tu solicitud. Por favor intenta más tarde.';

function montarChat(invitado = false) {
  sesion.isGuest = invitado;
  prepararRender(true);
  return () => {
    prepararRender();
    return useChatbot();
  };
}

function ultimaRespuesta(chat) {
  assert.equal(chat.isProcessing, false);
  assert.ok(chat.messages.every(mensaje => mensaje.type !== 'processing'));
  return chat.messages.at(-1);
}

test('envía el mensaje por Gateway con cookie y muestra respuesta, no explicacion', async prueba => {
  const leerChat = montarChat();
  prueba.mock.method(globalThis, 'fetch', async (url, opciones) => {
    assert.equal(url, '/api/v1/chat');
    assert.equal(opciones.method, 'POST');
    assert.equal(opciones.credentials, 'include');
    assert.deepEqual(opciones.headers, { 'Content-Type': 'application/json' });
    assert.deepEqual(JSON.parse(opciones.body), { mensaje: 'Busco leche' });
    assert.ok(opciones.signal instanceof AbortSignal);
    return Response.json({
      respuesta: 'Encontré leche en el catálogo.',
      explicacion: 'No corresponde al contrato actual.',
      productos_recomendados: [],
    });
  });
  await leerChat().handleSend('  Busco leche  ');
  assert.equal(ultimaRespuesta(leerChat()).content, 'Encontré leche en el catálogo.');
  assert.equal(ultimaRespuesta(leerChat()).type, 'text');
});

test('invitados y mensajes vacíos no hacen peticiones', async prueba => {
  const peticiones = prueba.mock.method(globalThis, 'fetch', async () => { throw new Error('No debe consultar'); });
  const leerChat = montarChat(true);
  await leerChat().handleSend('Busco leche');
  assert.equal(leerChat().showAuthModal, true);
  const leerSesion = montarChat();
  await leerSesion().handleSend('  ');
  assert.equal(leerSesion().messages.length, 1);
  assert.equal(peticiones.mock.callCount(), 0);
});

test('ningún error HTTP se presenta como respuesta real del asistente', async prueba => {
  for (const estado of [400, 500, 502, 503, 504]) {
    const leerChat = montarChat();
    const peticion = prueba.mock.method(globalThis, 'fetch', async () =>
      Response.json({ respuesta: 'No debe mostrarse', detail: 'Error' }, { status: estado }));
    await leerChat().handleSend('Busco leche');
    const respuesta = ultimaRespuesta(leerChat());
    assert.equal(respuesta.type, 'error');
    assert.equal(respuesta.content, ERROR_IA);
    assert.equal(leerChat().inputValue, 'Busco leche');
    peticion.mock.restore();
  }
});

test('sesión vencida, ruta ausente, validación y saturación tienen mensajes explícitos', async prueba => {
  for (const [estado, esperado] of [
    [401, /sesión expiró/], [403, /permiso/], [404, /no está habilitado/],
    [422, /mensaje no es válido/], [429, /demasiadas solicitudes/],
  ]) {
    const leerChat = montarChat();
    const peticion = prueba.mock.method(globalThis, 'fetch', async () => new Response(null, { status: estado }));
    await leerChat().handleSend('Busco arroz');
    assert.match(ultimaRespuesta(leerChat()).content, esperado);
    assert.equal(leerChat().showAuthModal, estado === 401);
    peticion.mock.restore();
  }
});

test('JSON inválido, HTML y contratos antiguos no se convierten en éxito', async prueba => {
  for (const cuerpo of [
    {}, null, { respuesta: '' }, { respuesta: '  ' }, { respuesta: 123 },
    { explicacion: 'Solo contrato antiguo' }, { error: 'No disponible', respuesta: 'Ignorar' },
  ]) {
    const leerChat = montarChat();
    const peticion = prueba.mock.method(globalThis, 'fetch', async () => Response.json(cuerpo));
    await leerChat().handleSend('Busco arroz');
    assert.equal(ultimaRespuesta(leerChat()).content, ERROR_IA);
    peticion.mock.restore();
  }
  const leerChat = montarChat();
  prueba.mock.method(globalThis, 'fetch', async () => new Response('<html>No es la API</html>'));
  await leerChat().handleSend('Busco arroz');
  assert.equal(ultimaRespuesta(leerChat()).content, ERROR_IA);
});

test('un fallo de red permite reintentar sin inventar una respuesta', async prueba => {
  const leerChat = montarChat();
  const peticion = prueba.mock.method(globalThis, 'fetch', async () => { throw new TypeError('Sin conexión'); });
  await leerChat().handleSend('Busco arroz');
  assert.match(ultimaRespuesta(leerChat()).content, /Revisa la conexión/);
  assert.equal(leerChat().inputValue, 'Busco arroz');
  peticion.mock.restore();
  prueba.mock.method(globalThis, 'fetch', async () => Response.json({ respuesta: 'Respuesta real' }));
  await leerChat().handleSend(leerChat().inputValue);
  assert.equal(ultimaRespuesta(leerChat()).content, 'Respuesta real');
});

test('dos envíos inmediatos comparten el bloqueo hasta terminar la primera petición', async prueba => {
  const leerChat = montarChat();
  let resolver;
  const peticion = prueba.mock.method(globalThis, 'fetch', () => new Promise(completar => { resolver = completar; }));
  const chat = leerChat();
  const pendiente = chat.handleSend('Busco arroz');
  await chat.handleSend('Busco leche');
  assert.equal(peticion.mock.callCount(), 1);
  assert.equal(leerChat().isProcessing, true);
  resolver(Response.json({ respuesta: 'Arroz disponible' }));
  await pendiente;
  assert.equal(ultimaRespuesta(leerChat()).content, 'Arroz disponible');
});

test('vaciar cancela la espera y descarta respuestas antiguas sin pisar una consulta nueva', async prueba => {
  const leerChat = montarChat();
  const pendientes = [];
  prueba.mock.method(globalThis, 'fetch', (_url, opciones) => new Promise(resolver => {
    pendientes.push({ resolver, signal: opciones.signal });
  }));
  const anterior = leerChat().handleSend('Consulta antigua');
  leerChat().handleClear();
  assert.equal(pendientes[0].signal.aborted, true);
  assert.equal(leerChat().messages.length, 1);
  const nueva = leerChat().handleSend('Consulta nueva');
  pendientes[0].resolver(Response.json({ respuesta: 'No debe reaparecer' }));
  await anterior;
  assert.equal(leerChat().isProcessing, true);
  pendientes[1].resolver(Response.json({ respuesta: 'Resultado nuevo' }));
  await nueva;
  assert.equal(ultimaRespuesta(leerChat()).content, 'Resultado nuevo');
  assert.equal(leerChat().messages.some(mensaje => mensaje.content === 'No debe reaparecer'), false);
});
