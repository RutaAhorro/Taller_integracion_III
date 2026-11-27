-- =======================================================
-- Índice GIN trigram de SKU para scraper.productos_crudos
-- =======================================================
-- Ejecutar como operador (POSTGRES_USER). Idempotente: segura en bases nuevas y
-- existentes. Complementa 03_indices_optimizacion.sql (que ya crea los índices
-- trigram sobre titulo_crudo, marca_cruda y categoria_cruda) incorporando el
-- índice de sku y actualizando las estadísticas del planificador con ANALYZE.

-- ===== Extensión pg_trgm =====
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===== Índice GIN con gin_trgm_ops para SKU =====
CREATE INDEX IF NOT EXISTS idx_productos_crudos_sku_trgm
    ON scraper.productos_crudos USING gin (sku gin_trgm_ops);

-- ===== Actualización de estadísticas =====
ANALYZE scraper.productos_crudos;
