-- SUP-317: sucursales físicas reales de A Cuenta y Cugat en Temuco y filtro
-- geoespacial para precios comparables por sucursal.
--
-- Aplicar después de 04 (que crea las filas 5 y 6) y antes de init.sql, que
-- respeta las filas existentes con ON CONFLICT DO NOTHING. Idempotente: se
-- puede re-ejecutar sobre una base ya migrada.
--
-- Fuentes de los datos (no inventados):
--   A Cuenta: PDF oficial "Locales y horarios SBA 2025", local
--     "Manuel Antonio Matta 025, Temuco"; horario lun-sáb 08:00-21:30,
--     domingo y feriados 08:30-21:00. Coordenada de OSM way 426693011
--     (Supermercado Acuenta), datos © OpenStreetMap contributors, ODbL.
--   Cugat: ficha oficial https://cugat.cl/ubicaciones/av-barros-arana-375-temuco
--     "Av. Barros Arana 375, Temuco"; horario lun-sáb 08:00-21:00,
--     domingo 08:30-17:30. Coordenada de OSM way 426439021 (Cugat),
--     datos © OpenStreetMap contributors, ODbL.
-- El esquema guarda un solo par de horarios: se consigna el de lunes a sábado
-- y el domingo queda documentado aquí para no perderlo.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- init.sql corre después en una base nueva; crearla aquí permite aplicar la
-- migración sola sobre un volumen existente que ya tenga las extensiones o no.
CREATE EXTENSION IF NOT EXISTS postgis;

UPDATE scraper.sucursales_supermercado
SET nombre        = 'SuperBodega aCuenta Temuco Matta',
    direccion     = 'Manuel Antonio Matta 025',
    comuna        = 'Temuco',
    ciudad        = 'Temuco',
    lat           = -38.7338383,
    lon           = -72.5806422,
    hora_apertura = '08:00:00',
    hora_cierre   = '21:30:00'
WHERE codigo_sucursal = 'ACUENTA-CENTRAL';

UPDATE scraper.sucursales_supermercado
SET nombre        = 'Cugat Temuco Barros Arana',
    direccion     = 'Av. Barros Arana 375',
    comuna        = 'Temuco',
    ciudad        = 'Temuco',
    lat           = -38.7385969,
    lon           = -72.5813372,
    hora_apertura = '08:00:00',
    hora_cierre   = '21:00:00'
WHERE codigo_sucursal = 'CUGAT-CENTRAL';

-- Índice funcional sobre geography: ST_DWithin en metros usa el índice GiST y
-- no añade columnas a la tabla compartida con Control. El predicado parcial
-- coincide con el WHERE del repositorio de Catálogo.
CREATE INDEX IF NOT EXISTS idx_sucursales_supermercado_geo
    ON scraper.sucursales_supermercado
    USING GIST ((ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography))
    WHERE lat IS NOT NULL AND lon IS NOT NULL;

COMMIT;
