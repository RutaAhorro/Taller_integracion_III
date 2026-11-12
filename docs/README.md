# Documentación de RutaAhorro

Este índice es la única entrada a la documentación del proyecto: un documento
que no aparece aquí no se considera vigente. Si algo de esta carpeta contradice
al código, al SQL o a la configuración, vale lo que dice el repositorio y se
corrige el documento.

## Por dónde empezar

1. [Visión general de la arquitectura](Arquitectura/01_Vision_general.md): diagrama,
   estilo y estado de cada componente.
2. [Glosario](Glosario.md): siglas y términos técnicos del proyecto.
3. [Plan de realización del Sprint 3](Guias/PLAN_REALIZACION_SPRINT3.md): qué se
   está construyendo ahora y en qué orden.

## Secciones

| Sección | Qué contiene |
| --- | --- |
| [Arquitectura](Arquitectura/README.md) | Servicios, flujo de una petición, datos y permisos, redes y despliegue, seguridad |
| [Decisiones](Decisiones/README.md) | Cada decisión de arquitectura con su motivo y la alternativa descartada |
| Contratos | [Artículos de lista](Contratos/CONTRATO_ARTICULOS_LISTA.md) (SUP-321) y [landing informativa](Contratos/CONTRATO_LANDING_INFORMATIVA.md) (SUP-354, excepción propuesta) |
| [Métricas](Metricas/README.md) | Lo medido, lo configurado y lo que no se mide; incluye el [benchmark HPA](Metricas/Benchmark_HPA_Catalogo.md) |
| [Requisitos](Requisitos/README.md) | Requisitos, reglas de negocio y términos. Describen objetivos, no funciones ya terminadas |
| [Diagramas](Diagramas/README.md) | Arquitectura, MER, casos de uso y flujos, con el estado de cada uno |
| [Glosario](Glosario.md) | Siglas y términos técnicos |

## Guías de trabajo

- [Flujo de ramas y pull requests](Guias/GUIA_GITFLOW_Y_PR.md).
- [Barrido de código](Guias/GUIA_BARRIDO_CODIGO.md): tests, mocks, hardcodeo y
  documentación de cada servicio (Sprint 3).
- [Configuración y variables compartidas](Guias/GUIA_CONFIGURACION.md): nombres
  acordados y reglas de configuración compartida (Sprint 3 · SUP-345).
- [Docker para desarrollo](Guias/GUIA_DOCKER.md). Los límites de recursos están en
  [escalado y recursos](Metricas/02_Escalado_y_recursos.md).
- [Contextos para asistentes de IA](Guias/GUIA_CONTEXTO_IA.md). Se unificarán en
  `AGENTS.md` (SUP-360).

## Operación

- [Datos y permisos del backend](Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md): roles,
  propiedad de tablas y migraciones.
- [Migraciones y orden de los scripts SQL](../infrastructure/db/README.md): no
  confundir la inicialización de un volumen vacío con migrar una base existente.
- [Control de scraping](Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md) y
  [recuperación del scraping](Guias/GUIA_SCRAPING_RECUPERACION.md). En esta
  última, los comandos de Python y de transición requieren adaptación.

## Documentación de cada servicio

Cada servicio mantiene su `README.md` junto al código; su barrido lo deja
verificado. [Frontend](../frontend/web/README.md) ·
[Gateway](../backend/gateway/README.md) · [Identidad](../backend/identidad/README.md) ·
[Catálogo](../backend/catalogo/README.md) · [Scraper Go](../backend/scraper_go/README.md) ·
[Motor de rutas](../backend/motor_rutas/README.md).

Cuatro guías de la extracción de servicios siguen vigentes en parte y se fusionan
en el README de su servicio durante el barrido; después se eliminan:
[Identidad](Guias/GUIA_IDENTIDAD_DESACOPLADA.md) (SUP-347),
[Catálogo](Guias/GUIA_CATALOGO_INDEPENDIENTE.md) e
[ingesta](Guias/GUIA_CATALOGO_INGESTA.md) (SUP-349) y
[enrutamiento de Catálogo](Guias/GUIA_CATALOGO_ENRUTAMIENTO.md) (SUP-350).

## Fuera del repositorio

- **Pendientes y tareas:** en Linear, no en documentos.
- **Bibliografía SWEBOK e investigaciones:** en el Drive del equipo
  ([aviso de investigaciones](Investigaciones/README.md)).
- **Orientación académica** (guía de estudio SWEBOK): fuera del repositorio, junto
  con la bibliografía.

## Reglas para mantener esta carpeta

- **Un tema, un documento.** Antes de crear uno nuevo, buscar si ya existe.
- **Ningún documento fuera de `docs/`**, salvo el `README.md` raíz y el de cada servicio.
- **Todo documento nuevo se enlaza aquí.** Una decisión nueva va en `Decisiones/`
  con el número siguiente; una reemplazada no se borra, se marca como reemplazada.
- Una captura, un manifiesto o una prueba local no confirman qué imagen corre en
  Kubernetes. Registrar versión y entorno al validar.
