-- ============================================================================
-- SUP-243: Concesión de UPDATE a app_catalogo en scraper.cadenas_supermercado
-- Permite que el servicio de Catálogo gestione el estado de activación de fuentes.
-- El 06 vigente ya incluye este GRANT; este script lo reaplica de forma
-- idempotente en bases que aplicaron una versión anterior de la migración.
-- ============================================================================

BEGIN;
SET LOCAL statement_timeout = '30s';

-- Conceder UPDATE sobre la tabla scraper.cadenas_supermercado al rol app_catalogo
GRANT UPDATE ON scraper.cadenas_supermercado TO app_catalogo;

COMMIT;
