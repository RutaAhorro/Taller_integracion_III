// Comprueba los estados visibles del lateral; las interacciones se validan también en navegador.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router-dom';
import ts from 'typescript';

/** Permite sustituir dependencias de la vista sin añadir librerías de pruebas. */
const crearModulo = (codigo) => `data:text/javascript;base64,${Buffer.from(codigo).toString('base64')}`;
const dependencias = {
  react: import.meta.resolve('react'),
  'react-router-dom': import.meta.resolve('react-router-dom'),
  'lucide-react': import.meta.resolve('lucide-react'),
  '@/components/ui/button': crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve('react'))};
    export function Button({ children, variant, size, ...propiedades }) {
      return createElement('button', propiedades, children);
    }
  `),
  '@/features/autenticacion/useRole': crearModulo('export const useRole = () => ({ canAccessColaborador: false });'),
  '@/features/autenticacion/tipos': crearModulo('export const PLAN_PREMIUM = "premium";'),
  '@/features/catalogo/products-api': crearModulo(`
    export const PREFIJO_CATEGORIA = "categoria:";
    export const obtenerCategorias = async () => [];
  `),
};
let fuente = await readFile(new URL('../src/layouts/Sidebar.tsx', import.meta.url), 'utf8');
for (const [nombre, ruta] of Object.entries(dependencias)) {
  fuente = fuente.replaceAll(`from '${nombre}'`, `from '${ruta}'`);
}
const { outputText } = ts.transpileModule(fuente, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ES2022,
    jsx: ts.JsxEmit.ReactJSX,
  },
});
const { SideBar } = await import(
  crearModulo(outputText.replace('"react/jsx-runtime"', JSON.stringify(import.meta.resolve('react/jsx-runtime'))))
);

/** Renderiza el mismo lateral para escritorio/móvil sin montar efectos de red. */
function renderizar(cambios = {}) {
  const propiedades = {
    open: true,
    onClose() {},
    onResetFilters() {},
    activeMarket: null,
    availableMarkets: [],
    cargandoSupermercados: false,
    errorSupermercados: false,
    alReintentarSupermercados() {},
    onSelectCategory() {},
    onSelectMarket() {},
    activeCategory: '',
    ...cambios,
  };
  return renderToStaticMarkup(createElement(MemoryRouter, null, createElement(SideBar, propiedades)));
}

test('el lateral presenta las cadenas recibidas sin anunciar la muestra de productos', () => {
  const contenido = renderizar({
    availableMarkets: [
      { id: 'jumbo', name: 'Jumbo' },
      { id: 'lider', name: 'Líder' },
    ],
  });
  assert.match(contenido, /Jumbo/);
  assert.match(contenido, /Líder/);
  assert.doesNotMatch(contenido, /Cadenas de los productos cargados|listado completo está pendiente/);
  assert.doesNotMatch(contenido, /No hay supermercados disponibles|No se pudieron cargar los supermercados/);
});

test('la ubicación muestra solo el nombre de la ciudad', () => {
  const contenido = renderizar({ activeCity: 'Temuco' });
  assert.match(contenido, /Temuco/);
  assert.doesNotMatch(contenido, /Comparando en/);
});

test('el lateral distingue la carga de supermercados del resultado vacío', () => {
  const contenido = renderizar({ cargandoSupermercados: true });
  assert.match(contenido, /Cargando supermercados/);
  assert.doesNotMatch(contenido, /No hay supermercados disponibles/);
});

test('el lateral informa el fallo de supermercados y ofrece reintentar', () => {
  const contenido = renderizar({ errorSupermercados: true });
  assert.match(contenido, /No se pudieron cargar los supermercados/);
  assert.match(contenido, /role="alert"/);
  assert.match(contenido, /Reintentar/);
  assert.doesNotMatch(contenido, /No hay supermercados disponibles/);
});

test('el lateral distingue las facetas vacías de carga y error', () => {
  const contenido = renderizar();
  assert.match(contenido, /No hay supermercados disponibles/);
  assert.doesNotMatch(contenido, /Cargando supermercados|No se pudieron cargar los supermercados/);
});
