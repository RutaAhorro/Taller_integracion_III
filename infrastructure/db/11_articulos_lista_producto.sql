-- SUP-327: los artículos de lista identifican el producto por el UUID que publica
-- Catálogo (scraper.productos_crudos.id), según docs/Contratos/CONTRATO_ARTICULOS_LISTA.md.
-- Aplicar después de 10. Idempotente: se puede re-ejecutar en una base existente.
--
-- - producto_id no lleva FK: el producto pertenece a Catálogo y Motor no consulta
--   sus tablas. La existencia y disponibilidad se validan por HTTP.
-- - producto_normalizado_id se elimina con su FK hacia api.productos_normalizados,
--   tabla que nunca se llenó.
-- - Filas previas: se conservan con producto_id NULL (no resolubles) en vez de
--   inventarles un producto. Los CHECK son NOT VALID para que solo apliquen a
--   filas nuevas o modificadas.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

ALTER TABLE rutas.articulos_lista_compras ADD COLUMN IF NOT EXISTS producto_id UUID;
ALTER TABLE rutas.articulos_lista_compras DROP COLUMN IF EXISTS producto_normalizado_id;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'rutas.articulos_lista_compras'::regclass
                     AND conname = 'articulos_lista_producto_requerido') THEN
        ALTER TABLE rutas.articulos_lista_compras
            ADD CONSTRAINT articulos_lista_producto_requerido CHECK (producto_id IS NOT NULL) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'rutas.articulos_lista_compras'::regclass
                     AND conname = 'articulos_lista_cantidad_positiva') THEN
        ALTER TABLE rutas.articulos_lista_compras
            ADD CONSTRAINT articulos_lista_cantidad_positiva CHECK (cantidad >= 1) NOT VALID;
    END IF;
END $$;

-- "Sumar si ya existe": un producto aparece una sola vez por lista. Los NULL
-- previos no chocan entre sí.
CREATE UNIQUE INDEX IF NOT EXISTS uq_articulos_lista_producto
    ON rutas.articulos_lista_compras (lista_id, producto_id);

-- Motor agrega, cambia y quita artículos. La secuencia la usa el SERIAL de id.
GRANT SELECT, INSERT, UPDATE, DELETE ON rutas.articulos_lista_compras TO app_rutas;
GRANT USAGE ON SEQUENCE rutas.articulos_lista_compras_id_seq TO app_rutas;

COMMIT;
