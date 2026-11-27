-- =======================================================
-- Índices GIN trigram para búsquedas sobre scraper.productos_crudos
-- =======================================================
-- Ejecutar como operador (POSTGRES_USER). Idempotente: segura en bases nuevas y
-- existentes. Complementa 03_indices_optimizacion.sql incorporando el índice de
-- sku y actualizando las estadísticas del planificador con ANALYZE.

-- ===== Extensión pg_trgm =====
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===== Índices GIN con gin_trgm_ops =====
CREATE INDEX IF NOT EXISTS idx_productos_crudos_titulo_trgm
    ON scraper.productos_crudos USING gin (titulo_crudo gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_productos_crudos_marca_trgm
    ON scraper.productos_crudos USING gin (marca_cruda gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_productos_crudos_categoria_trgm
    ON scraper.productos_crudos USING gin (categoria_cruda gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_productos_crudos_sku_trgm
    ON scraper.productos_crudos USING gin (sku gin_trgm_ops);

-- ===== Actualización de estadísticas =====
ANALYZE scraper.productos_crudos;
