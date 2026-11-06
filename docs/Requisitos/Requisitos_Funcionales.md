# Requisitos funcionales

Qué debe poder hacer el sistema. Los identificadores `RF-xx` son los del
documento formal del Sprint 0 y los usan los issues de Linear para su
trazabilidad: **no se renumeran**. Las reglas citadas (`RN-xx`) están en
[Reglas de negocio](Reglas_de_Negocio.md).

**Herencia de permisos (RN-11):** el usuario registrado hereda lo que puede
hacer el invitado, y el colaborador hereda lo del usuario registrado.

**Estados:**

- **Implementado:** existe y funciona en `develop`.
- **Parcial:** existe una parte; lo que falta se indica.
- **Sprint 3:** se construye en este sprint (issue indicado).
- **Fuera de alcance:** no se construye en la entrega final; queda registrado.

## Etapa 1 — Catálogo, autenticación y listas (Sprints 1 y 2)

| ID | Funcionalidad | Descripción | Actor | Estado |
| --- | --- | --- | --- | --- |
| RF-01.1 | Buscar y filtrar el catálogo | Buscar productos y aplicar filtros. Se ocultan los precios vencidos (RN-01) | Invitado | Parcial: búsqueda y filtros implementados; los productos sin stock aparecen primero (SUP-331) y la vigencia de precios se define en SUP-312 |
| RF-01.2 | Ver un producto y su fuente | Vista individual con precio y formato, y enlace a la tienda de origen | Invitado | Parcial: la vista existe, pero la API pública no expone `url_producto`, que sí se guarda en Catálogo |
| RF-01.3 | Gestión de credenciales | Registro, inicio de sesión y recuperación de contraseña | Invitado, Registrado | Parcial: registro, login y Google implementados; verificación de correo en Sprint 3 (SUP-231, SUP-249); endpoints de recuperación pendientes (SUP-233, SUP-234) |
| RF-01.4 | Preferencias dietéticas | Guardar restricciones alimentarias en el perfil | Invitado, Registrado | Fuera de alcance (SUP-238, SUP-251, SUP-262) |
| RF-01.5 | Ejecutar el scraping | Lanzar la extracción de precios manualmente | Admin | Parcial: el bot de Discord lo hace; el botón del panel admin está pendiente (SUP-148) |
| RF-02.1 | Listas de compra | Crear listas y agregar productos. Se excluyen los productos sin stock (RN-02) | Registrado | Parcial: CRUD de listas implementado; artículos en Sprint 3 (SUP-241, SUP-318; [contrato](../Contratos/CONTRATO_ARTICULOS_LISTA.md)) |
| RF-02.2 | Asistente IA de recetas | Pedir una receta y recibir productos reales del catálogo, con stock validado por el backend (RN-06) | Registrado | Sprint 3 (SUP-325, SUP-253, SUP-329). Sin filtro dietético |
| RF-02.3 | Misiones de validación | Reportar precios, stock o costos tras una compra y sumar puntos (RN-25, RN-26) | Colaborador | Fuera de alcance (crowdsourcing) |
| RF-02.4 | Estado del colaborador | Panel con misiones, vigencia y tokens de IA disponibles (RN-25) | Colaborador | Fuera de alcance (crowdsourcing) |

## Etapa 2 — Optimización y administración (Sprints 3 y 4)

| ID | Funcionalidad | Descripción | Actor | Estado |
| --- | --- | --- | --- | --- |
| RF-03.1 | Configurar el traslado | Medio de transporte y rendimiento para convertir distancia en dinero (RN-22) | Registrado | Sprint 3, acotado: el rendimiento se envía en cada solicitud, sin perfil guardado (SUP-323) |
| RF-03.2 | Definir la ubicación | Ingresar el punto de origen y el rango | Registrado | Sprint 3 (SUP-330) |
| RF-03.3 | Optimizar la ruta de compra | Minimizar el costo total con una o varias tiendas, sujeto a monto mínimo y ahorro (RN-00, RN-08) | Registrado | Sprint 3 (SUP-313, SUP-328, SUP-316) |
| RF-03.4 | Horarios de tiendas | Ver horarios de las sucursales de la ruta | Registrado | Fuera de alcance |
| RF-04.1 | Mostrar resultados | Desglose de productos por parada y ruta en un mapa | Registrado | Sprint 3 (SUP-261) |
| RF-04.2 | Homologar el catálogo | Reconocer el mismo producto en distintas tiendas | Admin | Sprint 3, automático por EAN (SUP-244). La herramienta manual queda fuera de alcance (SUP-245) |
| RF-04.3 | Monitoreo del sistema | Panel con éxito o error del scraping, uso de IA, ahorro promedio y productos más agregados | Admin | Parcial: panel existente; las métricas por servicio son *Could* (SUP-279, SUP-280) |
| RF-04.4 | Gestión de fuentes y usuarios | Activar o desactivar fuentes de scraping y cuentas (RN-11) | Admin | Implementado (SUP-242, SUP-243); validación pendiente (SUP-255) |

## Notas

- Las reglas de negocio originales citan `RF-01`, `RF-02`, `RF-04`, `RF-05`,
  `RF-06` y `RF-07` como áreas (optimización, stock, perfil, IA, accesos y
  rendimiento), no como requisitos de esta tabla. Se conservan tal cual por
  trazabilidad.
- El documento formal completo (Sprint 0) está en el Drive del equipo y en el
  historial de git (antes del PR #89).
