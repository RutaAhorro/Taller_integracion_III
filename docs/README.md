# Documentación de RutaAhorro

## Para entender el sistema actual

- [README principal](../README.md): arquitectura, estado y arranque local.
- [Frontend](../frontend/web/README.md): configuración, rutas reales y vistas simuladas.
- [Gateway](../backend/gateway/README.md), [Identidad](../backend/identidad/README.md)
  y [Catálogo](../backend/catalogo/README.md): responsabilidades y pruebas.
- [Scraper Go](../backend/scraper_go/README.md): protocolos, extracción y pruebas.
  Su descripción de cutover Python → Go es histórica; Compose ya usa Go.
- [Control de scraping](Guias/GUIA_CONTROL_SCRAPING_DESACOPLADO.md) y
  [datos/permisos](Guias/GUIA_DATOS_Y_PERMISOS_BACKEND.md): base del diseño
  desacoplado; contrastar detalles de configuración con código, SQL y manifiestos.
- [Migraciones](../infrastructure/db/README.md): no confundir inicialización de
  volumen vacío con migración de datos existentes.
- [Docker](Guias/GUIA_DOCKER.md) y [flujo de PR](Guias/GUIA_GITFLOW_Y_PR.md).
- [Contextos IA breves](Guias/GUIA_CONTEXTO_IA.md): mantenimiento y regeneración.

## Historial del refactor — no ejecutar como recetas actuales

Se conservan por su trazabilidad, contratos, decisiones y pruebas:

- [SUP-265: integración inicial del Gateway](Guias/GUIA_GATEWAY_DESARROLLO_PRUEBAS.md).
- [SUP-266: módulos dentro del monolito](Guias/GUIA_MODULOS_BACKEND.md).
- [SUP-267: extracción de Catálogo](Guias/GUIA_CATALOGO_INDEPENDIENTE.md).
- [SUP-268: enrutamiento de Catálogo](Guias/GUIA_CATALOGO_ENRUTAMIENTO.md).
- [SUP-269: traslado de ingesta](Guias/GUIA_CATALOGO_INGESTA.md).
- [SUP-270: recuperación](Guias/GUIA_SCRAPING_RECUPERACION.md): el protocolo sigue
  siendo útil, pero comandos Python y de transición requieren adaptación.
- [Seguimiento del refactor](Guias/SEGUIMIENTO_REFACTOR_BACKEND.md).

Los comandos con `go_service`, `api-backend` o `backend/scraper` pertenecen a
etapas anteriores. Los rollbacks requieren imágenes compatibles verificadas;
no recrear servicios o credenciales antiguos por copiar una guía.

Se retiraron las guías generales de backend/capas y las guías de OTP/algoritmos
que confundían el diseño actual con un monolito o un motor operativo inexistente.
Su contenido permanece consultable en el historial Git.

## Diseño, requisitos y estudio

- [Diagramas](Diagramas/README.md): la arquitectura Sprint 2 representa el
  inventario informado; MER y secuencias pueden describir etapas anteriores o
  funciones pendientes. No todos los diagramas son evidencia de implementación.
- [Requisitos y negocio](Requisitos/README.md): resúmenes técnicos, no documentos
  legales oficiales ni listado de funciones terminadas.
- [Orientación académica](Guias/GUIA_APRENDIZAJE.md): comprobar calendario y alcance
  con la planificación del equipo.
- Bibliografía SWEBOK: [Backend](Referencias/Backend/README.md),
  [DevOps](Referencias/DevOps/README.md), [Frontend](Referencias/Frontend/README.md)
  y [Negocio](Referencias/Vision_Negocio/README.md).
- [Investigaciones](Investigaciones/README.md): aviso de traslado a Drive.
- [Benchmark HPA](Guias/README_HPA_BENCHMARK.md): resultados históricos de un
  escenario específico, no capacidad garantizada del despliegue actual.
- [Hardware](Guias/HARDWARE_Y_OPTIMIZACION.md): recomendaciones generales;
  inventario y cifras requieren contraste con las cuotas reales.

Una captura, un manifiesto o una prueba local no confirman por sí solos qué
imagen está ejecutándose en Kubernetes. Registrar versión y entorno al validar.
