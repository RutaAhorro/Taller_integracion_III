// Prueba el decodificador usado por Leaflet, sin DOM ni red.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import ts from "typescript";

const fuente = await readFile(
  new URL("../src/features/rutas/geometria-ruta.ts", import.meta.url),
  "utf8",
);
const { outputText } = ts.transpileModule(fuente, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ES2022,
  },
});
const { decodificarPolilinea } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
);

test("decodifica latitud/longitud con precisión de cinco decimales", () => {
  assert.deepEqual(decodificarPolilinea("_p~iF~ps|U_ulLnnqC_mqNvxq`@"), [
    [38.5, -120.2],
    [40.7, -120.95],
    [43.252, -126.453],
  ]);
});

test("no dibuja geometrías vacías, truncadas o excesivas", () => {
  for (const texto of ["", "_", "?", "x".repeat(200001), "\n"]) {
    assert.throws(() => decodificarPolilinea(texto));
  }
});
