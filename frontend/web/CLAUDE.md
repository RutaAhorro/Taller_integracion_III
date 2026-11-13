# Frontend — contexto IA

- Leer `README.md` de este directorio. React 19, TS, Vite, Tailwind 4, Oxlint.
  Desarrollo Vite y producción Nginx en puerto 3000; alias `@/` → `src/`.
- Router real: `src/app/router.tsx`; constantes: `src/core/routes.ts`.
  No asumir que `App.tsx` contiene las rutas ni clasificar archivos por su nombre.
- Sesión en `src/contexts/AuthContext.tsx`: cookie HttpOnly `jwt` enviada con
  `credentials: 'include'`; usuario de UI en localStorage no acredita sesión.
- `ProtectedRoute` y `src/lib/roles.ts` controlan navegación; el backend autoriza.
  Roles `admin`/`colaborador`/`registrado`; premium es un plan, no un rol.
- Catálogo: `src/hooks/useCatalog.ts` y `src/lib/products-api.ts`.
  Admin: `src/features/admin`; listas: `src/features/listas` y `ShoppingLists.tsx`.
- `VITE_API_URL` incluye `/api/v1`; no duplicar prefijo. Variables VITE públicas:
  jamás incluir contraseñas, claves IA ni tokens internos de scraping.
- Carrito en Context es temporal, no está sincronizado con listas persistidas.
- Las vistas pendientes usan FeatureUnavailable, sin costos, puntos ni resultados
  simulados. Administración no sustituye errores por datos ficticios.
- La portada es informativa y usa landing.json; sus ejemplos no son resultados de
  la API. En el catálogo, las cadenas corresponden a los productos recibidos;
  cobertura completa pendiente de SUP-371. No reutilizar ejemplos de la portada como datos reales.
- Marcas blancas está deshabilitado hasta disponer de clasificación en el backend.
- Chat usa URL directa `/chat/gemini`; backend IA expone `/api/v1/chat`, sin ruta
  Gateway. No afirmar flujo receta → lista → ruta terminado.
- Reutilizar estilos/componentes del proyecto; comprobar imports antes de borrar.
- Validar `npm run lint`, `npx tsc -b --noEmit` y
  `npm test`. Build tolera errores de tsc: su éxito
  no demuestra tipos correctos. No modificar esa política sin una tarea explícita.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

84 archivos fuente, incluidos los de pruebas. Distribución:
- `(raíz)`: 1 archivos.
- `app`: 1 archivos.
- `components`: 41 archivos.
- `contexts`: 3 archivos.
- `core`: 1 archivos.
- `data`: 1 archivos.
- `features`: 6 archivos.
- `hooks`: 6 archivos.
- `layouts`: 1 archivos.
- `lib`: 7 archivos.
- `pages`: 15 archivos.
- `types`: 1 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
