# 📜 Términos y Condiciones (Impacto Técnico)

Este documento traduce los Términos y Condiciones legales de RutaAhorro en **requerimientos técnicos** para el equipo de desarrollo. Si un término legal existe, es porque debe haber código que lo respalde.

---

## 1. Restricción de Edad (Mayor de 18 años)
* **Backend:** El endpoint de registro (`POST /auth/register`) debe validar la edad mediante una fecha de nacimiento (o al menos un booleano `is_adult`).
* **Frontend:** El formulario de registro debe incluir un checkbox obligatorio o selector de fecha. Menores de 18 no pueden tener cuenta, pero sí usar la app como invitados.

## 2. Tipos de Cuenta y Límites
La ley y los costos exigen controlar el acceso.
* **Impacto técnico:** Implementar Roles en el JWT (`guest`, `user`, `colaborador`, `premium`).
* **Rate Limiting:** El middleware de Go y Python debe limitar la cantidad de peticiones a la IA y de cálculos de ruta según el rol (ej. `user`: 10/día, `colaborador`: 50/día).

## 3. Protección Anti-Scraping (Uso Permitido)
Los términos prohíben bots extrayendo nuestro catálogo.
* **Backend/DevOps:** Implementar rate limiting estricto por IP en endpoints de búsqueda. Si es posible, integrar Cloudflare Turnstile o reCAPTCHA en rutas públicas repetitivas.

## 4. Datos Aportados por Colaboradores (Crowdsourcing)
* **Base de Datos:** Los precios reportados por usuarios (`colaboradores`) deben almacenarse en una tabla transaccional con un estado (`pending`, `verified`, `rejected`) antes de mezclarse con el catálogo oficial para evitar sabotajes.

## 5. Descargos de Responsabilidad (Precios y Rutas)
RutaAhorro no garantiza que el precio no cambie en la tienda física, ni se hace responsable por accidentes en la ruta sugerida.
* **Frontend:** Es OBLIGATORIO que la interfaz (UI) muestre un *disclaimer* claro en la pantalla de pago/ruta advirtiendo que los precios son referenciales y que el trayecto es bajo la propia responsabilidad del usuario.

## 6. Privacidad de Ubicación
* **Base de Datos:** ¡CRÍTICO! La ubicación GPS del usuario para calcular rutas **NO DEBE** guardarse de forma permanente en la base de datos a menos que el usuario guarde una "Dirección Frecuente". Las coordenadas de paso solo deben existir en la memoria/sesión temporal de la solicitud.