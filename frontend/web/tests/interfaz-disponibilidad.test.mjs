// Comprueba las vistas reales con dependencias controladas, sin red ni servicios desplegados.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import ts from "typescript";

// ===== Carga de componentes =====

/** Construye módulos de prueba sin añadir dependencias al frontend. */
const crearModulo = (codigo) =>
  `data:text/javascript;base64,${Buffer.from(codigo).toString("base64")}`;

/** Sustituye únicamente las dependencias indicadas y transpila el componente real. */
async function cargarComponente(ruta, dependencias) {
  let fuente = await readFile(new URL(ruta, import.meta.url), "utf8");
  for (const [nombre, destino] of Object.entries(dependencias)) {
    fuente = fuente
      .replaceAll(`from '${nombre}'`, `from '${destino}'`)
      .replaceAll(`from "${nombre}"`, `from "${destino}"`);
  }
  const { outputText } = ts.transpileModule(fuente, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      jsx: ts.JsxEmit.ReactJSX,
    },
  });
  return import(
    crearModulo(
      outputText.replaceAll(
        '"react/jsx-runtime"',
        JSON.stringify(import.meta.resolve("react/jsx-runtime")),
      ),
    )
  );
}

const dependenciasComunes = {
  react: import.meta.resolve("react"),
  "react-router-dom": import.meta.resolve("react-router-dom"),
  "lucide-react": import.meta.resolve("lucide-react"),
};
const urlCarrito = crearModulo(`
  export const carrito = { totalItems: 2, totalPrice: 194, isCartOpen: false, setIsCartOpen() {} };
  export const useCart = () => carrito;
`);
const { carrito } = await import(urlCarrito);
const { MainLayout } = await cargarComponente("../src/layouts/MainLayout.tsx", {
  ...dependenciasComunes,
  "@/features/catalogo/CartContext": urlCarrito,
  "@/features/catalogo/useCatalog": crearModulo(
    "export const useCatalog = () => ({ marketsReady: false });",
  ),
  "@/layouts/TopNav": crearModulo("export const TopNav = () => null;"),
  "@/layouts/Sidebar": crearModulo("export const SideBar = () => null;"),
  "@/layouts/BottomNav": crearModulo("export default () => null;"),
  "@/features/catalogo/CartSidebar": crearModulo(
    "export const CartSidebar = () => null;",
  ),
});

// La vista de listas tiene ocho estados; solo se controlan carga y error para probar su presentación.
const INDICE_CARGA_LISTAS = 3;
const INDICE_ERROR_LISTAS = 6;
const urlReactListas = crearModulo(`
  export const escenario = { indice: 0, error: null };
  export const useEffect = () => {};
  export function useState(inicial) {
    const indice = escenario.indice++;
    const valor = indice === ${INDICE_CARGA_LISTAS} ? false :
      indice === ${INDICE_ERROR_LISTAS} ? escenario.error : inicial;
    return [valor, () => {}];
  }
`);
const { escenario } = await import(urlReactListas);
const urlApiListas = crearModulo(`
  export const listasApi = {};
  export class ErrorSolicitudListas extends Error {
    constructor(estado, mensaje) { super(mensaje); this.estado = estado; }
  }
`);
const { ErrorSolicitudListas } = await import(urlApiListas);
const { default: ShoppingLists } = await cargarComponente(
  "../src/features/listas/ShoppingLists.tsx",
  {
    ...dependenciasComunes,
    react: urlReactListas,
    "@/features/listas/listas-api": urlApiListas,
    "@/features/autenticacion/useAuth": crearModulo(
      'export const useAuth = () => ({ user: { id: "cuenta-activa" } });',
    ),
    "@/components/ToastContext": crearModulo(
      "export const useToast = () => ({ toast() {} });",
    ),
    "@/app/routes": crearModulo('export const ROUTES = { LOGIN: "/login" };'),
    "@/components/ui/button": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export function Button({ children, variant, ...propiedades }) {
      return createElement('button', propiedades, children);
    }
  `),
    "@/components/ui/input": crearModulo(`
    import { createElement } from ${JSON.stringify(import.meta.resolve("react"))};
    export const Input = propiedades => createElement('input', propiedades);
  `),
  },
);

const urlSesionPerfil = crearModulo(`
  export const sesion = { user: {
    nombre_completo: "Cuenta de prueba",
    correo: "perfil@example.com",
    plan: "basico",
    cuota_tokens_ia: 10,
  }, isGuest: false };
  export const useAuth = () => sesion;
`);
const { sesion: sesionPerfil } = await import(urlSesionPerfil);
const { default: Profile } = await cargarComponente(
  "../src/features/autenticacion/Profile.tsx",
  {
    ...dependenciasComunes,
    "@/features/autenticacion/useAuth": urlSesionPerfil,
    "@/features/autenticacion/tipos": crearModulo(
      'export const PLAN_PREMIUM = "premium";',
    ),
    "@/lib/utils": crearModulo(
      'export const cn = (...clases) => clases.filter(Boolean).join(" ");',
    ),
  },
);

/** Renderiza una pantalla con navegación real, sin montar efectos ni consultar la API. */
function renderizar(componente) {
  return renderToStaticMarkup(
    createElement(MemoryRouter, null, createElement(componente)),
  );
}

// ===== Regresiones de interfaz =====

test("el acceso flotante al carrito desaparece mientras su panel está abierto", () => {
  carrito.totalItems = 2;
  carrito.isCartOpen = false;
  assert.match(renderizar(MainLayout), /aria-label="Ver carrito"/);
  carrito.isCartOpen = true;
  assert.doesNotMatch(renderizar(MainLayout), /aria-label="Ver carrito"/);
  carrito.isCartOpen = false;
  carrito.totalItems = 0;
  assert.doesNotMatch(renderizar(MainLayout), /aria-label="Ver carrito"/);
});

test("listas ofrece iniciar sesión únicamente cuando la API devuelve 401", () => {
  for (const estado of [401, 403, 404, 502, 503, 504]) {
    escenario.indice = 0;
    escenario.error = new ErrorSolicitudListas(
      estado,
      "Error informado por la API",
    );
    const contenido = renderizar(ShoppingLists);
    assert.match(contenido, /Error informado por la API/);
    assert.equal(contenido.includes("Ir a iniciar sesión"), estado === 401);
  }
});

test("listas no ofrece iniciar sesión ante fallos de red y explica el límite del carrito", () => {
  escenario.indice = 0;
  escenario.error = new TypeError("No se pudo conectar");
  const contenido = renderizar(ShoppingLists);
  assert.match(contenido, /No se pudo conectar/);
  assert.doesNotMatch(
    contenido,
    /Ir a iniciar sesión|SUP-241|Disponible: crear/,
  );
  assert.match(contenido, /no se guarda automáticamente/);
});

test("el resumen de perfil mantiene un fondo primario y textos contrastantes en ambos planes", () => {
  for (const plan of ["basico", "premium"]) {
    sesionPerfil.user.plan = plan;
    const contenido = renderizar(Profile);
    assert.match(
      contenido,
      /<section class="bg-primary [^"]*text-primary-foreground/,
    );
    assert.match(contenido, /text-primary-foreground\/90/);
    assert.match(contenido, /Cuenta de prueba/);
    assert.match(contenido, /perfil@example.com/);
    assert.doesNotMatch(
      contenido,
      /bg-gradient-to-r from-secondary to-background/,
    );
  }
});
