# Frontend — contexto IA

- Leer `README.md` de este directorio. React 19, TS, Vite, Tailwind 4, Oxlint.
  Desarrollo Vite y producción Nginx en puerto 3000; alias `@/` → `src/`.
- Router real: `src/app/router.tsx`; constantes: `src/app/routes.ts`.
  No asumir que `App.tsx` contiene las rutas ni clasificar archivos por su nombre.
- Estructura por tema en `src/features/` (`autenticacion`, `catalogo`, `chat`,
  `landing`, `listas`, `admin`); layouts en `src/layouts`; primitivas en
  `src/components/ui`; sin mocks ni carpetas `hooks/`, `contexts/` o `types/`.
- Sesión en `src/features/autenticacion/AuthContext.tsx`: cookie HttpOnly `jwt`
  enviada con `credentials: 'include'`; usuario de UI en localStorage no acredita sesión.
- `ProtectedRoute` y `src/features/autenticacion/roles.ts` controlan navegación; el
  backend autoriza. Roles `admin`/`colaborador`/`registrado`; premium es un plan
  (`basico`/`premium`, `tipos.ts`), no un rol. Sin alias heredados de rol ni plan.
- Catálogo: `src/features/catalogo/useCatalog.ts` y `products-api.ts`.
  Admin: `src/features/admin`; listas: `src/features/listas` (`ShoppingLists.tsx`).
- `VITE_API_URL` incluye `/api/v1`; no duplicar prefijo. Variables VITE públicas:
  jamás incluir contraseñas, claves IA ni tokens internos de scraping.
- Carrito en Context es temporal, no está sincronizado con listas persistidas.
- Las vistas pendientes usan FeatureUnavailable, sin costos, puntos ni resultados
  simulados. Administración no sustituye errores por datos ficticios.
- La portada es informativa y usa `src/features/landing/landing.json`; sus ejemplos no son resultados de
  la API. El catálogo y el lateral comparten `useCatalog` desde `MainLayout`: las
  cadenas vienen de las facetas `supermercados`, no de los productos de una página.
  No reutilizar ejemplos de la portada como datos reales.
- Marcas blancas envía `solo_marcas_blancas=true`; la clasificación pertenece al backend.
- Búsquedas cortas (menos de 3 bytes UTF-8) muestran aviso sin filtrar una página local.
  Cancelar y descartar respuestas obsoletas; paginar conserva las facetas confirmadas.
- Chat llama a `${VITE_API_URL}/chat`, que Gateway enruta a IA (SUP-278); no
  hay `VITE_IA_URL`. No afirmar flujo receta → lista → ruta terminado.
- `/verificar-email` (SUP-249) sólo prepara la vista, pendiente de SUP-231:
  no verifica tokens ni modifica el registro. Confirmación y aviso de correo
  enviado sólo se conectarán a resultados reales de Identidad, nunca a parámetros de URL.
- Reutilizar estilos/componentes del proyecto; comprobar imports antes de borrar.
- Validar `npm run lint`, `npx tsc -b --noEmit` y
  `npm test`. Build tolera errores de tsc: su éxito
  no demuestra tipos correctos. No modificar esa política sin una tarea explícita.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

72 archivos fuente, incluidos los de pruebas. Distribución:
- `(raíz)`: 1 archivos.
- `app`: 2 archivos.
- `components`: 11 archivos.
- `features`: 45 archivos.
- `layouts`: 4 archivos.
- `lib`: 2 archivos.
- `pages`: 7 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
