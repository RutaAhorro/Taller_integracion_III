-- ============================================================================
-- SUP-243: Concesión de UPDATE acotado a app_catalogo en cadenas de scraping
-- Permite que Catálogo gestione únicamente el flag esta_activa de las fuentes.
-- El 06 vigente ya incluye este GRANT; este script lo reaplica de forma
-- idempotente en bases que aplicaron una versión anterior de la migración.
-- ============================================================================

BEGIN;
SET LOCAL statement_timeout = '30s';

-- Retirar un grant de tabla completa de versiones anteriores antes de acotarlo.
REVOKE UPDATE ON scraper.cadenas_supermercado FROM app_catalogo;
GRANT UPDATE (esta_activa) ON scraper.cadenas_supermercado TO app_catalogo;

COMMIT;
