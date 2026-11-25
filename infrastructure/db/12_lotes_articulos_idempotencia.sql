-- SUP-318: claves Idempotency-Key del agregado en lote de artículos
-- (POST /api/v1/listas/{lista_id}/articulos/lote, CONTRATO_ARTICULOS_LISTA.md §3.4).
-- Aplicar después de 11. Idempotente: se puede re-ejecutar en una base existente.
--
-- - Una fila por lote aplicado: si el frontend reintenta con la misma clave,
--   Motor la encuentra y no vuelve a sumar las cantidades.
-- - huella es el SHA-256 (hex) del lote normalizado: la misma clave con otro
--   contenido se rechaza en vez de devolver un resultado que no corresponde.
-- - Las filas se borran con su lista (FK interna del esquema rutas). La acción
--   de la FK corre con permisos del dueño, por eso app_rutas no necesita DELETE.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS rutas.lotes_articulos_aplicados (
    lista_id UUID NOT NULL REFERENCES rutas.listas_compras(id) ON DELETE CASCADE,
    clave VARCHAR(128) NOT NULL,
    huella CHAR(64) NOT NULL,
    creado_el TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lista_id, clave)
);

-- Motor sólo consulta y registra claves; nunca las modifica ni las borra.
GRANT SELECT, INSERT ON rutas.lotes_articulos_aplicados TO app_rutas;

COMMIT;
