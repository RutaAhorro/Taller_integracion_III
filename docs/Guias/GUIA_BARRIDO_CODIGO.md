# Guía del barrido de código (Sprint 3)

> **Estado:** borrador para revisión del equipo · SUP-344
> **Aplica a:** los barridos SUP-347 y SUP-349 a SUP-354, a la migración de Motor de rutas a Go (SUP-348) y a todo el código nuevo del Sprint 3.

## Para qué existe esta guía

En el Sprint 3 cada integrante limpia sus propios servicios antes de sumar
funciones nuevas. Para que el resultado sea parejo, los cinco necesitamos
aplicar el mismo criterio: dónde van los tests, qué cuenta como hardcodeo y qué
no se puede tocar. Esta guía fija ese criterio. Si algo no está cubierto aquí,
se consulta antes de decidirlo por cuenta propia.

El barrido persigue tres objetivos que pidió el profesor:

1. **Cero tests mezclados con el código fuente.** Las pruebas siguen
   existiendo y se ejecutan en cada PR, pero viven en su propia carpeta.
2. **Cero mocks o datos falsos en el código de la aplicación.**
3. **Cero hardcodeo:** ningún valor que dependa del entorno o del negocio
   escrito directamente en el código.

## 1. Dónde van los tests

Cada servicio tiene una carpeta `tests/` en su raíz, separada del código:

```
backend/<servicio>/
  cmd/  internal/  app/  ...   ← solo código de la aplicación
  tests/
    unit/                      ← pruebas de funciones y paquetes aislados
    integration/               ← pruebas que levantan base de datos o HTTP
frontend/web/
  src/                         ← solo código de la aplicación
  tests/
```

### Go: qué cambia al mover los tests

Hoy los 70 archivos `_test.go` están junto al código y usan el mismo paquete
(`package catalogo`), así que pueden llamar funciones internas. Al moverlos a
`tests/` pasan a ser **pruebas de caja negra** (`package <nombre>_test`): solo
pueden usar lo que el paquete exporta (nombres con mayúscula).

- Go permite que `backend/<servicio>/tests/` importe los paquetes de
  `backend/<servicio>/internal/`, porque está dentro del mismo servicio.
- Si un test probaba una función interna, se reescribe para probar el
  comportamiento a través de la función pública que la usa.
- **No se exporta una función solo para poder testearla.** Si algo importante
  no se puede probar desde fuera, es señal de que esa lógica merece su propio
  paquete con una API pública clara.

### Python y frontend

- `ia_conversacional` es el único servicio en Python. Ya tiene `tests/`; solo
  se reorganiza en `unit/` e `integration/`.
- `motor_rutas` se migra a Go (SUP-348) en lugar de barrerse en Python. El
  servicio nuevo se escribe directamente con esta estructura y estas reglas.
- En el frontend, los tests van en `frontend/web/tests/` y nada de pruebas o
  datos de ejemplo queda dentro de `src/`, salvo el contenido editorial de la
  portada descrito en la excepción de la sección 2.1.

### CI

Los workflows de CI (SUP-346) ejecutan las pruebas desde estas rutas en cada
PR hacia `develop`. Mover los tests no debe obligar a tocar los workflows.

## 2. Mocks y datos de prueba

| Situación                                                                                                               | ¿Permitido? |
| ----------------------------------------------------------------------------------------------------------------------- | ----------- |
| Datos falsos, respuestas simuladas o pantallas "de ejemplo" en el código de la aplicación (`src/`, `internal/`, `app/`) | **No**      |
| Comandos o seeds que cargan datos inventados (por ejemplo `catalogo/cmd/seed_productos`)                                | **No**      |
| Un test que crea, dentro del propio test, los pocos datos que necesita                                                  | Sí          |
| Pruebas de integración contra una base de datos temporal que se crea y se destruye en CI                                | Sí          |
| Archivos de datos sueltos (`fixtures/`, `mock.ts`, `productos.json`)                                                    | **No**      |

Si una pantalla o servicio dependía de un mock, al quitarlo debe pasar a una de
estas tres opciones: **datos reales**, un **estado vacío explícito** ("aún no
hay datos") o **desaparecer** si la función está fuera del alcance del sprint.
Nunca se deja una pantalla rota o en blanco.

### 2.1 Excepción propuesta: contenido informativo de la portada (SUP-354)

La conservación de `frontend/web/src/data/landing.json`, su justificación y sus
límites se definen en el
[contrato de la landing informativa](../Contratos/CONTRATO_LANDING_INFORMATIVA.md).
Consultar ese contrato antes de proponer eliminar el archivo durante el barrido.
La excepción está propuesta para ratificación del equipo en el PR #141; no
autoriza simulaciones en las vistas funcionales.

## 3. Qué cuenta como hardcodeo

Un valor está hardcodeado cuando está escrito en el código pero en realidad
depende del entorno donde corre la aplicación o de una regla de negocio que
puede cambiar.

| Tipo de valor | Dónde va | Ejemplo actual del repositorio |
| --- | --- | --- |
| URLs, hosts y puertos | Variable de entorno | `useChatbot.ts` usa `http://localhost:8002` como respaldo |
| Credenciales, tokens y claves | Variable de entorno (nunca en el repositorio) | — |
| Timeouts, límites, tamaños de página, orígenes CORS | Variable de entorno con valor por defecto documentado | `server.go` de Catálogo fija `2*time.Second` |
| Reglas de negocio que pueden cambiar | Configuración o datos (base de datos o API) | Lista de marcas blancas en `useCatalog.ts` |
| Textos o valores repetidos en varios archivos | Una constante con nombre en un solo lugar | — |
| `0`, `1`, `""`, códigos HTTP, nombres de campos de un contrato | **Se quedan en el código** | — |

La última fila es importante: convertir *todo* en variable deja el código más
difícil de leer, no más limpio. El criterio es si el valor puede cambiar sin
que cambie la lógica.

### Cómo se lee la configuración

- Cada servicio lee su configuración **en un solo lugar** al arrancar (por
  ejemplo `internal/config` en Go o `app/config.py` en `ia_conversacional`).
- Si falta una variable obligatoria, el servicio **falla al arrancar** con un
  mensaje claro, en vez de usar un valor inventado.
- Cada servicio mantiene un `.env.example` con todas sus variables, una
  descripción corta y un valor de ejemplo **sin secretos reales**.
- Las variables que usan varios servicios (`JWT_SECRET`, URLs internas, token
  de servicio, Redis, base de datos) tienen **un único nombre** acordado en la
  [guía de configuración](GUIA_CONFIGURACION.md) (SUP-345). Ningún servicio las
  renombra por su cuenta.

## 4. Código muerto

Se elimina todo lo que no tiene referencias: funciones, rutas, componentes,
archivos y comandos que nadie usa. Antes de borrar algo, se busca su nombre en
**todo** el repositorio (incluidos compose, k8s, CI y scripts), no solo en el
servicio. El historial queda en git, así que no hace falta comentar código "por
si acaso".

## 5. Documentación del servicio

- El `README.md` de cada servicio se queda en su carpeta, pero el barrido lo
  deja **limpio y verificado contra el código**: qué hace el servicio, cómo se
  configura, cómo se prueba y qué endpoints expone. Lo que no se pueda
  verificar se elimina.
- **Ningún documento nuevo se crea fuera de `docs/`**, salvo el `README.md` de
  cada servicio y el `README.md` raíz. Guías, decisiones e investigaciones van
  en `docs/` y se enlazan desde `docs/README.md`.
- Los archivos de contexto para asistentes de IA (`CLAUDE.md`, `GEMINI.md`) no
  se tocan en el barrido: se unifican en una tarea aparte.

## 6. Qué NO se toca durante el barrido

El barrido es interno a cada servicio. Estos puntos los comparten varios
integrantes y cambiarlos rompería a otros:

- **Contratos HTTP:** rutas y nombres de campos JSON.
- **Base de datos:** nombres de tablas y columnas (el esquema `scraper` lo
  comparten Catálogo y Control).
- **Variables de entorno compartidas** (ver [guía de configuración](GUIA_CONFIGURACION.md) y SUP-345).
- **Archivos compartidos:** compose, k8s y CI (responsable: Renato) e
  `infrastructure/db` (responsable: Esban). Los cambios ahí se le piden a su
  responsable.

Si encuentras uno de estos puntos mal nombrado, anótalo en la descripción de tu
PR. Se corrige después, en un cambio coordinado entre quien lo produce y quien
lo consume.

## 7. Cómo se entrega cada barrido

1. Rama desde `develop`: `[área]/refactor/sup-XXX-barrido-<nombre>`.
2. Un PR por servicio, **solo con limpieza**: sin funciones nuevas mezcladas.
3. El PR está listo cuando:
   - [ ] CI en verde.
   - [ ] No queda ningún test dentro del código fuente.
   - [ ] No quedan mocks ni datos inventados en el código de la aplicación.
   - [ ] Las URLs, credenciales, timeouts y reglas de negocio salen de la configuración.
   - [ ] `.env.example` actualizado.
   - [ ] `README.md` del servicio limpio y verificado contra el código.
   - [ ] La descripción del PR lista lo que se eliminó y cualquier punto compartido mal nombrado que se haya encontrado.
4. Tope de **dos días** por servicio. Lo que quede pendiente se anota en el
   issue y se termina en paralelo con la ola siguiente.

## Pendientes antes de cerrar esta guía

- [ ] Confirmar con el profesor si los datos que un test crea para sí mismo cuentan como mock.
- [ ] Confirmar con Renato que los workflows de SUP-346 leen las rutas `tests/unit` y `tests/integration`.
- [ ] Revisión del equipo.
