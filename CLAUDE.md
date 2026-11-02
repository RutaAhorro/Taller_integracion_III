# Contexto de RutaAhorro para asistentes IA

## Leer primero

- `README.md`: arquitectura, configuración y límites actuales.
- `docs/README.md`: documentos vigentes frente a históricos.
- El contexto del servicio afectado; no cargar todos los archivos del repositorio.
- Código, SQL y configuración prevalecen sobre planes o mapas desactualizados.

## Límites de arquitectura

- Gateway Go (`net/http`) solo enruta: sin JWT, SQL, Redis ni reglas de negocio.
- Identidad: cuentas, sesión y administración; `app_identity`.
- Catálogo: productos, precios, fuentes e ingesta idempotente; `app_catalogo`.
- Control: trabajos y outboxes, Redis Streams y HTTP a Catálogo; `app_scraping`.
- Scraper Go: extracción y spool SQLite, resultados por HTTP. Sin SQL PostgreSQL.
- Una instancia PostgreSQL/PostGIS, permisos por tabla. El esquema `scraper`
  contiene tablas de Catálogo y de Control: no equivale a un único servicio.
- Bot y worker usan token de servicio; nunca exponerlo al navegador.
- Identidad emite cookie HttpOnly `jwt`; la autorización pertenece a cada servicio.
- Frontend React: distinguir datos de API, mocks y estado local. Rol y plan son
  distintos (`admin`/`colaborador`/`registrado` frente a `basico`/`premium`).
- IA consulta Catálogo por HTTP, nunca PostgreSQL. Pipeline: extracción LLM,
  filtrado determinista y explicación solo con datos validados.
- Motor FastAPI: CRUD de listas; artículos/optimización pendientes. IA/listas no
  están acreditados como servicios desplegados en el inventario de Sprint 2.
- No hay un OTP operativo ni un servicio monolítico `backend/api` vigente.

## Reglas de trabajo

- Verificar rama y `git status`; preservar cambios del usuario. No commit, push,
  merge, despliegue o migración sin autorización para esa operación.
- Ramas de trabajo: `[área]/[tipo]/[tarea]-[nombre]`; commits convencionales.
  Consultar `docs/Guias/GUIA_GITFLOW_Y_PR.md` para PR hacia `develop`.
- No imprimir secretos ni copiar `.env`, JWT, cookies o credenciales a logs/docs.
- No hacer SQL cruzado entre dominios, AutoMigrate ni ampliar permisos por comodidad.
- No ejecutar fixtures/destrucción sobre BD o clúster compartidos. Probar aislado.
- No asegurar integración, despliegue o rendimiento por un test unitario exitoso.
- Frontend: `npm run build` tolera fallos de `tsc`; comprobar tipos por separado.
- Para localizar código usar búsquedas de símbolos/imports, no inventarios completos.

## Mantenimiento de este contexto

Editar las instrucciones manuales en `CLAUDE.md`; ejecutar
`python3 scripts/gen_service_map.py` para actualizar índices breves y sincronizar
`GEMINI.md` y `.github/copilot-instructions.md`. No editar la sección generada.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Contextos por servicio

- Gateway: `backend/gateway/CLAUDE.md`.
- Identidad: `backend/identidad/CLAUDE.md`.
- Catálogo: `backend/catalogo/CLAUDE.md`.
- Control de scraping: `backend/control_scraping/CLAUDE.md`.
- Scraper Go: `backend/scraper_go/CLAUDE.md`.
- Bot Discord: `backend/bot_discord/CLAUDE.md`.
- IA conversacional: `backend/ia_conversacional/CLAUDE.md`.
- Motor de rutas / Listas: `backend/motor_rutas/CLAUDE.md`.
- Frontend: `frontend/web/CLAUDE.md`.
