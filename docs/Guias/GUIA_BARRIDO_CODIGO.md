# Guía del barrido de código (Sprint 3)

> **Estado:** vigente: es el criterio para revisar los PR de barrido (SUP-344) y de compactación (SUP-384). Quedan abiertos los pendientes del final.
> **Aplica a:** los barridos SUP-347 y SUP-349 a SUP-354, a la migración de Motor de rutas a Go (SUP-348) y a todo el código nuevo del Sprint 3.
> La segunda fase (compactación) suma los criterios de la [sección 8](#8-anexo-criterios-de-compactación-segunda-fase).

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
- Los archivos de contexto para asistentes de IA no se tocaron en la primera
  fase. Mientras SUP-360 no se fusione, se siguen editando como indica el
  `CLAUDE.md` raíz (ver sección 8.7).

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

En la compactación hay dos excepciones acotadas, descritas en la sección 8:
agregar restricciones en `infrastructure/db` coordinadas con su responsable
(8.4) y responder 400 ante un parámetro retirado del propio servicio (8.5).
Ninguna cambia nombres de rutas, campos, tablas, columnas ni variables
compartidas.

## 7. Cómo se entrega cada barrido

1. Rama desde `develop`: `[área]/refactor/sup-XXX-barrido-<nombre>` (en la compactación, `[área]/refactor/sup-XXX-compactacion-<nombre>`).
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

## 8. Anexo: criterios de compactación (segunda fase)

Los barridos de la primera fase se cerraron con las secciones 1 a 7 de esta
guía y las cumplieron. El barrido de Identidad (SUP-347) mostró que se puede ir
más lejos sin perder funcionalidad: de 15 paquetes de producción a 6 y de 31
archivos Go de producción a 15 (de 51 a 28 contando las pruebas, que pasan a
`tests/unit` y `tests/integration`).
Esta sección recoge esos criterios para aplicarlos al resto de los servicios.
No reemplaza las secciones anteriores: se suma a ellas.

### 8.1 Diagnóstico antes de tocar nada

- Partir del diagnóstico adjunto a la tarea paraguas y **confirmar cada
  hallazgo** antes de actuar. El diagnóstico se generó con IA y no ejecutó
  pruebas ni revisó todo el CI.
- Confirmar significa buscar el símbolo en todo el repositorio, incluidas las
  pruebas, `.github/workflows`, `scripts/`, Compose y K8s (sección 4).
- Si el servicio ya cumple los criterios, la tarea se cierra como "sin cambios
  necesarios". No se inventa trabajo.

### 8.2 Estructura

- La referencia es Identidad: `cmd/<servicio>` y, bajo `internal/`, `api`
  (rutas, handlers, sesión, errores), `services`, `repositories`, `domain` y
  `config`.
- **Un archivo por tema**, no por tipo: `productos.go` reúne lo de productos en
  cada capa, en vez de `producto_handler.go`, `producto_service.go`, etc.
- **Sin paquetes de un solo archivo** salvo `cmd/<servicio>` y `config`. Un
  paquete `utils`, `auth` o `server` con un archivo se fusiona en la capa que lo
  usa.
- Python y frontend aplican la misma idea con módulos y carpetas por tema.

### 8.3 Poda

- **Funcionalidad armada sin consumidores** se elimina aunque funcione: código
  que se construye al arrancar pero que ningún endpoint, comando o job expone.
  En Identidad fue la recuperación de contraseña y el correo.
- Si es trabajo a medias de otra tarea abierta, **no se poda**: se anota en el
  PR con el número de esa tarea. Ejemplo: `motor_rutas/internal/optimizacion`
  espera el endpoint de SUP-316.
- Una funcionalidad que sí se poda se acuerda antes con su autor y con el
  equipo, y queda una tarea para reimplementarla después.
- **Configuración huérfana:** se quita la variable que el código lee y ningún
  manifiesto define, y la que los manifiestos definen y el código no lee. Si es
  compartida, se coordina en la tarea de cierre (sección 6).

### 8.4 Reglas de datos en la base, no en el código

- Si el código normaliza o corrige datos guardados (alias de roles, `LOWER`,
  `TRIM`, valores heredados), la regla se mueve a la base: una migración en
  `infrastructure/db` que normaliza los datos y agrega una restricción (`CHECK`,
  `NOT NULL`, `UNIQUE`).
- La migración aborta si encuentra un valor sin traducción segura, para que un
  operador lo revise en vez de asignar algo a ciegas.
- El servicio comprueba al arrancar que la restricción exista. Así el orden de
  despliegue lo exige el propio código.
- Con la regla garantizada, se borra la normalización del código.
- **Excepción a la sección 6:** agregar restricciones no cambia nombres de
  tablas ni columnas, pero toca `infrastructure/db`. Se coordina con su
  responsable (Esban) y se documenta como paso de despliegue en el PR.

### 8.5 Contratos

- Un parámetro o campo retirado **responde 400** con un mensaje que diga qué
  usar en su lugar. Ignorarlo en silencio devolvería otro resultado sin avisar.
- Cada cambio de contrato va en una tabla "Antes / Ahora" en el README del
  servicio y en la descripción del PR.
- Los contratos entre servicios (HTTP interno, Redis Streams, esquema `scraper`)
  siguen sin cambiar dentro de una tarea de servicio (sección 6). Si hace falta,
  se avisa en el PR y se resuelve en la tarea de cierre.

### 8.6 Código

- Comentario de paquete o archivo que explique su rol, comentario en cada
  función o tipo (qué hace y por qué) y bloques `// ===== Sección =====` por tema.
- Constantes con nombre en vez de números o textos sueltos, incluidos códigos
  externos como SQLSTATE (`codigoViolacionUnicidad = "23505"`) y estados HTTP
  (`http.StatusOK`, no `200`).
- Los mensajes que citan un límite se construyen con la constante, no repiten
  el número.
- Nombres descriptivos en español, sin excepciones por convención del lenguaje
  (`peticion`/`respuesta`, `contexto`, `errorObtenido`, `prueba`, `indice`).
- Sin comprobaciones redundantes en cada petición contra estados que el propio
  arranque impide (por ejemplo, un repositorio `nil` que `main` nunca pasa).
  En cambio, un componente **sí debe fallar al construirse** si le falta una
  dependencia de seguridad obligatoria (por ejemplo, la revalidación de sesión):
  eso no es una guarda "por si acaso", es un error de configuración que no debe
  llegar a producción.
- En el frontend, si el renombrado al español es grande, va en un PR aparte del
  de compactación para que ambos se puedan revisar.

### 8.7 Documentación y contexto

- El `README.md` del servicio es la fuente única: configuración, contratos,
  operación y pruebas. Las guías duplicadas se fusionan en él (sección 5).
- Contexto para asistentes de IA: **hoy** se edita el `CLAUDE.md` del servicio
  y se corre `python3 scripts/gen_service_map.py`, que sincroniza `GEMINI.md` y
  `.github/copilot-instructions.md`, como indica el `CLAUDE.md` raíz.
- **Cuando SUP-360 se fusione**, cada servicio pasará a un único `AGENTS.md` y
  un `CLAUDE.md` que solo contiene `@AGENTS.md`. Hasta entonces, no se crean
  esos archivos en los PR de compactación.

### 8.8 Validación

- Go: `go test -race ./...` y `go vet ./...` en Docker, igual que el CI.
- Python: `pytest` y `ruff`. Frontend: pruebas, `tsc` por separado y build.
- Pruebas contra PostgreSQL y Redis reales y aislados cuando el servicio los usa,
  con su rol de base de datos de runtime. Si una variable `*_TEST_*` documentada
  no tiene prueba que la use, se agrega la prueba o se quita la variable.
- Una prueba HTTP en vivo sobre la imagen construida, con el resultado en el PR.
- Nunca contra una base o un clúster compartido.

### 8.9 Cómo se entrega cada compactación

Igual que la sección 7, más estas casillas:

- [ ] Diagnóstico confirmado; cada hallazgo descartado tiene su motivo en el PR.
- [ ] Estructura con la forma de 8.2, sin paquetes de un solo archivo.
- [ ] Sin funcionalidad armada sin consumidores, o con su tarea pendiente anotada.
- [ ] Normalizaciones de datos movidas a la base, con migración y comprobación al arrancar.
- [ ] Tabla "Antes / Ahora" de contratos en el README, si hubo cambios.
- [ ] Constantes con nombre, nombres en español y comentarios por sección.
- [ ] Contexto de IA actualizado según 8.7 (`CLAUDE.md` + `gen_service_map.py` hasta que se fusione SUP-360).
- [ ] Validación de 8.8 en verde, con los resultados en el PR.
- [ ] Pasos de despliegue (migraciones, variables) en la descripción del PR.

## Pendientes antes de cerrar esta guía

- [ ] Confirmar con el profesor si los datos que un test crea para sí mismo cuentan como mock.
- [ ] Confirmar con Renato que los workflows de SUP-346 leen las rutas `tests/unit` y `tests/integration`.
- [ ] Revisión del equipo.
