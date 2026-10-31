-- Migración aditiva para trabajos que ya existen en volúmenes PostgreSQL.
-- En una instalación nueva la columna ya la crea 02_schema_scraper.sql.
ALTER TABLE scraper.trabajos_scraper
    ADD COLUMN IF NOT EXISTS respuestas_bloqueadas INTEGER DEFAULT 0;
