# Informe de `develop` — RutaAhorro

Relevamiento de solo lectura de la rama `develop` de `RutaAhorro/Taller_integracion_III`, hecho en tres fases el
27-11-2026. Este directorio está fuera del repo y no forma parte de él.

| Dato | Valor |
| --- | --- |
| Commit analizado | `e398cb49c4c5c0acdce5979b6af9d55b7e1eb7a3` |
| Último PR incluido | #207 `fix(ia): relevancia de la búsqueda y claves sin espacios` (también incluye #208, fusionado justo antes) |
| Origen | `origin/develop` local, **sin `git fetch`**: los PR posteriores a #207 no están incluidos |
| Copia del código | `~/scan-develop` (worktree desacoplado, sin `.env`) |
| Informe anterior | commit `20b2b37` (#174, 24-11-2026) → `REPORTE_ANTERIOR.md` |

## Por dónde empezar

| Si quieres… | Lee |
| --- | --- |
| El estado del proyecto y qué cambió desde el 24-nov | `REPORTE_DEVELOP_NUEVO.md` §1 y §2 |
| Qué falta para el MVP | `REPORTE_DEVELOP_NUEVO.md` §11 (brechas contra `docs/OBJETIVO_MVP.md`) y §12 (recortes) |
| Los riesgos de seguridad | `AUDITORIA_SEGURIDAD.md` §1 y §5 |
| El detalle de un servicio | `fichas/<servicio>.md`, contrastado con `REPORTE_DEVELOP_NUEVO.md` §15 |

## Contenido

```
informe_develop/
├── README.md                  este archivo
├── BRIEF_FASE1.md             reglas y plantilla que siguieron los subagentes de la Fase 1
├── fichas/                    12 fichas por área (Fase 1, sin verificar)
├── REPORTE_ANTERIOR.md        informe del commit 20b2b37, solo para comparar
├── REPORTE_DEVELOP_NUEVO.md   análisis y síntesis (Fase 2)
└── AUDITORIA_SEGURIDAD.md     auditoría de seguridad (Fase 3)
```

---

## Fase 1 — Lectura por áreas

**Qué se hizo.** Se creó un worktree desacoplado de `origin/develop` en `~/scan-develop` y se lanzaron 12
subagentes en paralelo con **Gemini 4 Hydrogen** (razonamiento alto), uno por área: frontend, gateway, identidad,
catalogo, control_scraping, scraper_go, motor_rutas (incluye `/api/v1/rutas/optimizar`), ia_conversacional,
bot_discord, infraestructura, documentación contra código e historia de PR desde el #175.

Cada subagente escribió `fichas/<area>.md` con la plantilla de `BRIEF_FASE1.md`:
1. Propósito y archivos principales.
2. Endpoints o pantallas con `archivo:línea` y estado.
3. Quién consume cada cosa.
4. Tests: comando y resultado.
5. Contradicciones entre documentación y código.
6. Preguntas para el analista, sin resolver.

Estados: `IMPLEMENTADO Y CONECTADO`, `IMPLEMENTADO PERO NO CONECTADO`, `PARCIAL`, `STUB-MOCK`, `SOLO DOCUMENTADO`,
`NO DETERMINADO`. Evidencia: `[código]`, `[ejecutado]`, `[inferido]`.

**Incidencias.**
- El primer lanzamiento falló: los subagentes de Gemini solo pueden acceder al repo principal y a
  `~/.t3/userdata/attachments`. Se canceló el resto para que no leyeran el repo principal (otro commit), se movió el
  worktree temporalmente a esa carpeta y se relanzaron. Al terminar se copiaron las fichas aquí y el worktree volvió a `~/scan-develop`.
- En las fichas se quitaron los prefijos `file:///…/attachments/…` de los enlaces; las rutas quedaron relativas al repo.

**Advertencia.** Las fichas las escribió un modelo más pequeño. La Fase 2 revisó 25 de sus afirmaciones: 20 eran
correctas y 5 tenían errores (ver `REPORTE_DEVELOP_NUEVO.md` §15). Léelas como material de trabajo.

---

## Fase 2 — Análisis y síntesis

**Qué se hizo** (Claude Opus 5.5). Se leyeron las fichas y el informe anterior, se verificaron en el código los
puntos importantes y se corrieron los tests de Go. El resultado es `REPORTE_DEVELOP_NUEVO.md`, con la misma
estructura que el informe anterior más tres secciones nuevas:
- §2.a Tabla "qué cambió": cada hallazgo anterior como CORREGIDO, IGUAL o CAMBIÓ, con PR y evidencia.
- §11 Brechas contra `docs/OBJETIVO_MVP.md`: cada paso de la historia contra los 5 puntos de "terminado".
- §12 Qué quedó fuera del MVP y si sigue accesible.
- §15 Verificación de 25 afirmaciones de las fichas y respuesta a sus preguntas.

**Conclusiones principales.**
- En tres días (24 PR, #184-#208) el MVP quedó implementado en código: ruta óptima conectada de la lista a `/route`,
  listas con artículos, total y carrito, chat con tarjetas, login solo con Google en la UI, `ADMIN_EMAILS`, Motor e IA
  en Compose por defecto y en k8s, y secciones fuera del MVP ocultas.
- No cumple todos los puntos de "terminado": los pasos 1 (Google) y 6 (panel admin) no tienen paso en
  `scripts/recorrido_mvp.sh`; los tests de la IA no se pudieron correr; `CLAUDE.md` y el `README.md` del repo están
  desactualizados; "un solo `docker compose up`" requiere un `.env` completo y un dump privado del catálogo.
- Riesgos nuevos: login y registro locales abiertos en la API; `ADMIN_EMAILS` re-promueve en cada login; ninguna capa
  limita cuerpos ni envía cabeceras de seguridad; re-ejecutar `06_propiedad_permisos.sql` pierde un permiso del Motor;
  el worker de scraping tiene 0 réplicas en k8s.

**Tests ejecutados** (`GOFLAGS=-mod=mod GOPROXY=off go test -count=1 ./...`, sin red, BD ni Redis):

| Servicio | Pasan | Omitidos | Fallan |
| --- | --- | --- | --- |
| gateway | 28 | 0 | 0 |
| identidad | 78 | 13 | 0 |
| catalogo | 79 | 15 | 0 |
| control_scraping | 42 | 2 | 0 |
| motor_rutas | 53 | 12 | 0 |
| bot_discord, scraper_go | todos los paquetes `ok` | — | 0 |
| ia_conversacional | NO DETERMINADO (`fastapi` no instalado) | | |
| frontend | NO DETERMINADO (sin `node_modules`) | | |

Los omitidos son los que necesitan PostgreSQL o Redis.

---

## Fase 3 — Auditoría de seguridad

**Qué se hizo** (Claude Opus 5.5). Lectura directa del código de cabeceras, CORS, validación de entradas, manejo del
JWT y tokens de servicio, `/api/v1/rutas/optimizar` y el chat, más el estado de S1-S10 del informe anterior. Resultado:
`AUDITORIA_SEGURIDAD.md`. No se ejecutó ningún servicio ni ataque; los escenarios se deducen del código.

**Hallazgos (A1-A10).**

| # | Severidad | Hallazgo |
| --- | --- | --- |
| A1 | Alta | Apropiación previa de cuenta: un registro local con el correo de otra persona sobrevive a su vinculación con Google; si el correo está en `ADMIN_EMAILS`, el atacante obtiene rol admin en Identidad |
| A2 | Alta (si no se rotó) | El `JWT_SECRET` que estuvo versionado sigue en el historial de git |
| A3 | Media | Login y registro locales abiertos en la API, contra el recorte "solo Google" |
| A4 | Media | Las sesiones no se revocan: logout, desactivación o degradación no invalidan el JWT (24 h) en Catálogo, Motor e IA |
| A5 | Media | Webhook de GitHub sin firma; botones de Discord sin autorización |
| A6 | Media | Sin cabeceras de seguridad; Ingress sin TLS en el repo |
| A7 | Baja | `ADMIN_EMAILS` re-promueve en cada login |
| A8 | Baja | El registro permite enumerar correos |
| A9 | Baja | El mensaje del usuario entra al prompt de explicación del chat (render seguro, sin herramientas) |
| A10 | Baja | El JWT llega al pod de PostgreSQL; scraper y frontend corren como root |

**Lo que está bien:** CORS con orígenes exactos en los 5 servicios; JWT con algoritmo fijo y `exp` obligatoria;
secretos de al menos 32 bytes; tokens internos comparados en tiempo constante; SQL parametrizado; listas, carrito y
direcciones filtrados por dueño; `/api/v1/rutas/optimizar` con sesión, validación y topes que el cliente no puede cambiar.

**S1-S10 del informe anterior:** S1 mitigado a medias (archivo retirado, rotación NO DETERMINADO); S3 mitigado
(PostgreSQL y Redis en `127.0.0.1`); S2 abierto pero menos expuesto; S4-S10 abiertos.

**Prioridades antes de la entrega:**
1. Cerrar `register` y `login` locales en Identidad (resuelve A3, neutraliza A1 y A8; ajustar `recorrido_mvp.sh --registro-local`).
2. Confirmar la rotación de `JWT_SECRET` en el clúster.
3. Agregar cabeceras básicas en `frontend/web/nginx.conf`.

---

## Reglas que se respetaron en las tres fases

- Solo lectura: sin commit, push, checkout ni cambios en el repo. El worktree quedó limpio (`git status` vacío).
- Sin leer `.env`, kubeconfig ni valores de secretos; de `secret.example.yaml` solo se citan marcadores `CAMBIAR_*`.
- Sin historial de git con contenido: solo `git log --oneline`, `--format` y `--stat` (este último solo en la ficha de historia de PR).
- Sin `kubectl`, `docker` ni red: no se instalaron dependencias ni se consultó GitHub.
- Solo tests unitarios sin BD, Redis ni red.
- Lo no verificable quedó como NO DETERMINADO.

## Qué quedó NO DETERMINADO

- Estado del clúster: imágenes desplegadas (por ejemplo, si el Motor ya incluye #195), réplicas reales, Secrets y rotación, TLS en `proxy.inf.uct.cl`.
- Si `docker compose up` levanta sano con el `.env` real, y el contenido del dump de demo.
- Tests de la IA y del frontend.
- PR #176 y #196 (ausentes en `develop`).
- Intent de contenido de mensajes del bot en Discord.

## Cómo repetir el análisis sobre un commit nuevo

1. `git fetch` y crear un worktree desacoplado: `git worktree add --detach ~/scan-develop origin/develop`
   (si ya existe, quitar antes el anterior con `git worktree remove ~/scan-develop`).
2. Copiar el `REPORTE_DEVELOP_NUEVO.md` actual como `REPORTE_ANTERIOR.md`.
3. Lanzar la Fase 1 con `BRIEF_FASE1.md` (actualizar el commit). Si los subagentes corren con acceso restringido,
   poner el worktree y la salida dentro de una carpeta permitida.
4. Repetir las Fases 2 y 3 con estos documentos como plantilla.
