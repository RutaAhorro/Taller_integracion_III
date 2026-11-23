// Prueba el cliente y la traducción usados por Sidebar/useCatalog sin llamar servicios.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const fuente = await readFile(new URL('../src/features/catalogo/products-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(fuente.replace('import.meta.env.VITE_API_URL', '"/api/v1/"'), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const { obtenerCategorias, parametrosCategoria, PREFIJO_CATEGORIA } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

test('categorías usa Gateway, conserva valores, vacío y propaga errores', async prueba => {
  let cuerpo = { categorias: ['Lácteos / huevos', 'Pan & masas', 'ofertas'] };
  let estado = 200;
  prueba.mock.method(globalThis, 'fetch', async (url) => {
    assert.equal(url, '/api/v1/productos/categorias');
    return Response.json(cuerpo, { status: estado });
  });
  assert.deepEqual(await obtenerCategorias(), cuerpo.categorias);
  cuerpo = { categorias: [] };
  assert.deepEqual(await obtenerCategorias(), []);
  cuerpo = { categorias: null };
  await assert.rejects(obtenerCategorias(), /inválida/);
  estado = 500;
  await assert.rejects(obtenerCategorias(), /500/);
});

test('selección usa igualdad exacta y no confunde categorías con destacados', () => {
  for (const categoria of ['Pan & masas', 'Lácteos / huevos', 'ofertas', 'marcas', 'Pan_%', 'categoria:otra']) {
    const parametros = parametrosCategoria(`${PREFIJO_CATEGORIA}${categoria}`);
    assert.deepEqual([...parametros], [['categoria_exacta', categoria]]);
    assert.equal(new URLSearchParams(parametros.toString()).get('categoria_exacta'), categoria);
  }
  assert.deepEqual([...parametrosCategoria('ofertas')], [['en_oferta', 'true']]);
  assert.deepEqual([...parametrosCategoria('descuentos')], [['en_oferta', 'true']]);
  assert.deepEqual([...parametrosCategoria('marcas')], [['solo_marcas_blancas', 'true']]);
  assert.deepEqual([...parametrosCategoria('Leches')], [['categoria', 'Leches']]);
  assert.deepEqual([...parametrosCategoria('')], []);
});
