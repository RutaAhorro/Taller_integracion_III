# D12 — El artículo de lista guarda el id de producto de Catálogo

[← Índice de decisiones](README.md)

**Contexto:** `rutas.articulos_lista_compras` apunta a `api.productos_normalizados`, una tabla que nada llena, mientras Catálogo entrega productos de `scraper.productos_crudos`. Listas, IA y Rutas necesitan una sola forma de identificar un producto.

**Decisión:** el artículo guarda el `producto_id` de Catálogo (UUID de `productos_crudos`), sin clave foránea entre esquemas y validado por HTTP. Catálogo resuelve los equivalentes en otras sucursales por EAN. Detalle en el [contrato de artículos](../Contratos/CONTRATO_ARTICULOS_LISTA.md) (SUP-321).

**Motivo:**
- Respeta los límites de dominio: Listas no conoce el EAN ni lee tablas de `scraper`.
- Permite agregar productos que no tienen EAN.
- Es el id que ya devuelve `GET /api/v1/productos`, y el que esperan SUP-244 y SUP-317.

**Alternativa descartada:** guardar el EAN directamente. Es la identidad real entre tiendas, pero deja fuera a los productos sin EAN y acopla Listas a un dato interno de Catálogo.

**Consecuencias:**
- Un `producto_id` es un producto en una sucursal concreta; si deja de extraerse, la lista lo muestra como `no_encontrado`.
- La migración de las referencias actuales va en SUP-327.

**Estado:** propuesta, en revisión del equipo (PR #133).
