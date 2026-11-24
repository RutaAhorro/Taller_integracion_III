// Verifica facetas completas y cancelación sin depender de Catálogo ni modificar el lateral.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const fuente = await readFile(new URL('../src/features/catalogo/products-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(fuente.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const { getProducts, slugifyMarket } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

// Producto de una sola cadena para detectar si se deducen facetas de esta muestra.
const producto = {
  id: 'producto-leche',
  nombre: 'Leche entera',
  marca: 'Marca de prueba',
  categoria: 'Lácteos',
  supermercado: 'Líder',
  precio: 1000,
  precio_normal: 1200,
  en_oferta: true,
  url_imagen: '/leche.png',
  unidad: '1 L',
  en_stock: true,
};
const supermercados = [
  { id: 1, nombre: 'Jumbo' },
  { id: 2, nombre: 'Líder' },
];

/** Construye la respuesta pública con metadatos independientes de la página recibida. */
function pagina(productos, cadenas, numeroPagina = 1) {
  return {
    data: productos,
    supermercados: cadenas,
    total_registros: 50,
    pagina_actual: numeroPagina,
    total_paginas: 3,
  };
}

test('productos expone todas las facetas, aunque la página solo incluya una cadena', async (prueba) => {
  prueba.mock.method(globalThis, 'fetch', async (url) => {
    assert.equal(url, '/api/v1/productos?q=leche&supermercado=L%C3%ADder');
    return Response.json(pagina([producto], supermercados));
  });

  const resultado = await getProducts('?q=leche&supermercado=L%C3%ADder');
  assert.deepEqual(resultado.markets, [
    { id: 'jumbo', name: 'Jumbo' },
    { id: 'lider', name: 'Líder' },
  ]);
  assert.equal(resultado.products.length, 1);
  assert.equal(resultado.products[0].supermarketId, slugifyMarket('Líder'));
  assert.equal(resultado.products[0].price, 1000);
  assert.equal(resultado.total, 50);
  assert.equal(resultado.page, 1);
  assert.equal(resultado.totalPages, 3);
});

test('paginar conserva las facetas enviadas por el servidor, sin acumular cadenas de los productos', async (prueba) => {
  prueba.mock.method(globalThis, 'fetch', async (url) => {
    const numeroPagina = Number(new URL(url, 'http://frontend.test').searchParams.get('page'));
    return Response.json(pagina(numeroPagina === 1 ? [producto] : [], supermercados, numeroPagina));
  });

  const primera = await getProducts('?q=leche&page=1');
  const segunda = await getProducts('?q=leche&page=2');
  assert.deepEqual(segunda.markets, primera.markets);
  assert.equal(segunda.page, 2);
  assert.deepEqual(segunda.products, []);
});

test('una búsqueda vacía de resultados devuelve facetas vacías, sin conservar las anteriores', async (prueba) => {
  let cuerpo = pagina([producto], supermercados);
  prueba.mock.method(globalThis, 'fetch', async () => Response.json(cuerpo));
  assert.equal((await getProducts('?q=leche')).markets.length, 2);

  cuerpo = pagina([], []);
  cuerpo.total_registros = 0;
  cuerpo.total_paginas = 0;
  const resultado = await getProducts('?q=inexistente');
  assert.deepEqual(resultado.markets, []);
  assert.deepEqual(resultado.products, []);
  assert.equal(resultado.total, 0);
});

test('facetas ausentes o inválidas producen error, sin usar productos como respaldo', async (prueba) => {
  let cuerpo;
  prueba.mock.method(globalThis, 'fetch', async () => Response.json(cuerpo));
  for (const cadenas of [
    undefined,
    null,
    {},
    ['Líder'],
    [{ id: 2 }],
    [{ id: '2', nombre: 'Líder' }],
    [{ id: 0, nombre: 'Líder' }],
    [{ id: 2.5, nombre: 'Líder' }],
    [{ id: 2, nombre: ' ' }],
  ]) {
    cuerpo = pagina([producto], cadenas);
    await assert.rejects(getProducts(), /Respuesta de supermercados inválida/);
  }
});

test('productos propaga fallos HTTP', async (prueba) => {
  prueba.mock.method(globalThis, 'fetch', async () => Response.json({}, { status: 503 }));
  await assert.rejects(getProducts(), /503/);
});

test('productos reenvía AbortSignal y propaga la cancelación', async (prueba) => {
  const controlador = new AbortController();
  const cancelacion = new DOMException('Petición cancelada', 'AbortError');
  prueba.mock.method(globalThis, 'fetch', (_url, opciones) => {
    assert.equal(opciones.signal, controlador.signal);
    return new Promise((_resolver, rechazar) => {
      opciones.signal.addEventListener('abort', () => rechazar(cancelacion), { once: true });
    });
  });
  const pendiente = getProducts('?q=leche', controlador.signal);
  controlador.abort();
  await assert.rejects(pendiente, (error) => error === cancelacion);
});
