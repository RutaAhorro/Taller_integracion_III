// Prueba las utilidades de presentación compartidas por las vistas del frontend.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import ts from 'typescript';

const fuente = await readFile(new URL('../src/features/catalogo/formatters.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(fuente, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
});
const { formatearCategoria, formatearUnidad, tienePrecioVigente } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

test('formatea categorías normalizadas con excepciones y palabras acentuadas', () => {
  assert.equal(formatearCategoria('frescos-y-lacteos'), 'Frescos y lácteos');
  assert.equal(formatearCategoria('Dulces_mermeladas_y_manjar'), 'Dulces, mermeladas y manjar');
  assert.equal(formatearCategoria('panaderia_y_pasteleria'), 'Panadería y pastelería');
  assert.equal(formatearCategoria('aseo-y-bano'), 'Aseo y baño');
});

test('devuelve vacío para categorías nulas o vacías', () => {
  assert.equal(formatearCategoria(''), '');
  assert.equal(formatearCategoria(null), '');
});

test('oculta unidades inválidas y conserva unidades válidas', () => {
  assert.equal(formatearUnidad('Format'), null);
  assert.equal(formatearUnidad(' format '), null);
  assert.equal(formatearUnidad('N/A'), null);
  assert.equal(formatearUnidad(''), null);
  assert.equal(formatearUnidad(null), null);
  assert.equal(formatearUnidad('1 kg'), '1 kg');
  assert.equal(formatearUnidad('500 ml'), '500 ml');
});

test('solo considera vigentes los precios positivos y finitos', () => {
  for (const precio of [0, -100, NaN, Infinity, null, undefined])
    assert.equal(tienePrecioVigente(precio), false);
  assert.equal(tienePrecioVigente(1500), true);
});
