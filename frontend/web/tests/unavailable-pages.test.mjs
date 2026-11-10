import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router-dom';
import ts from 'typescript';

// Renderiza los componentes reales sin navegador ni una nueva dependencia de pruebas.
async function componentUrl(path, imports = {}) {
  let source = await readFile(new URL(path, import.meta.url), 'utf8');
  for (const [specifier, url] of Object.entries(imports)) source = source.replace(specifier, url);
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
    fileName: path,
  });
  const resolved = outputText
    .replaceAll('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime')))
    .replaceAll("'react-router-dom'", JSON.stringify(import.meta.resolve('react-router-dom')));
  return `data:text/javascript;base64,${Buffer.from(resolved).toString('base64')}`;
}

const routesUrl = await componentUrl('../src/core/routes.ts');
const unavailableUrl = await componentUrl('../src/components/FeatureUnavailable.tsx', { '@/core/routes': routesUrl });

for (const [page, title] of [
  ['RouteViewer', 'Ruta no disponible'],
  ['History', 'Historial no disponible'],
  ['Crowdsourcing', 'Misiones no disponibles'],
  ['Planes', 'Suscripciones no disponibles'],
]) {
  test(`${page} informa indisponibilidad sin resultados ni acciones simuladas`, async () => {
    const url = await componentUrl(`../src/pages/${page}.tsx`, { '@/components/FeatureUnavailable': unavailableUrl });
    const { default: Page } = await import(url);
    const html = renderToStaticMarkup(createElement(MemoryRouter, null, createElement(Page)));
    assert.ok(html.includes(title));
    assert.match(html, /role="status"/);
    assert.match(html, /href="\/listas"/);
    assert.doesNotMatch(html, /<button|<input|<iframe|\$[\d.]|150 pts|50 puntos|12 Productos/);
  });
}
