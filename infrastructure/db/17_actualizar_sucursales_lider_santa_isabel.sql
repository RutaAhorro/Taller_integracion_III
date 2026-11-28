-- MVP: coordenadas reales de Lider Prieto Norte y Santa Isabel Carrera para
-- que la distancia calculada por el motor de rutas sea creíble.
--
-- Aplicar después de 16 sobre un volumen existente. Idempotente: se puede
-- re-ejecutar sobre una base ya migrada. No agrega columnas ni permisos; el
-- índice GiST creado por la 13_ sobre lat/lon se actualiza solo.
--
-- A diferencia de la 13_, las filas 2 y 4 las siembra init.sql al final del
-- arranque, no la 04_. Por eso init.sql también trae estos mismos valores: en
-- una base nueva la 17_ corre antes de que las filas existan (UPDATE 0) y el
-- seed ya nace corregido. En una base existente, donde init.sql no se ejecuta,
-- esta migración es la que aplica el cambio.
--
-- Fuentes de los datos (no inventados):
--   Lider: listado oficial de tiendas
--     https://apps.lider.cl/landing/json/storeListHtml.html
--     "Lider | Temuco | Prieto Norte 0320, Temuco", horario 08:00-21:30.
--     El comunicado oficial de Walmart Chile
--     https://www.walmartchile.cl/un-nuevo-comienzo-en-temuco-reabrimos-nuestro-supermercado-lider-prieto
--     confirma la dirección "Avenida Prieto Norte 0320, Temuco" y su reapertura
--     en febrero de 2026. Coordenada de OSM way 94237032 (Lider), datos ©
--     OpenStreetMap contributors, ODbL; coincide con Google Maps place_id
--     ChIJYcozTPrTFJYRisxCvJ8ivbQ (-38.72917, -72.599777). El seed usaba una
--     dirección ajena a esa tienda, "Av. Caupolicán 0450".
--   Santa Isabel: la fila sembrada "San Martín 0890" no existe en el listado
--     oficial https://www.santaisabel.cl/institucional/locales-santa-isabel
--     ni en el PDF oficial de locales
--     https://assets.santaisabel.cl/uploads/2025/12/SISA-locales-abiertos-14dic.pdf
--     El local real más cercano al punto sembrado es "Temuco Carrera",
--     "Carrera 499" (Google Maps lo muestra como "Gral. Carrera 499", place_id
--     ChIJfebhjeXTFJYRzDLjjBbkmP4). Coordenada de OSM way 413350087 (Santa
--     Isabel), datos © OpenStreetMap contributors, ODbL. Horario 08:30-21:00
--     de lunes a sábado y 09:00-21:00 domingo y feriados.
-- El esquema guarda un solo par de horarios: se consigna el de lunes a sábado
-- y el domingo queda documentado aquí para no perderlo.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- init.sql corre después en una base nueva; crearla aquí permite aplicar la
-- migración sola sobre un volumen existente que ya tenga las extensiones o no.
CREATE EXTENSION IF NOT EXISTS postgis;

UPDATE scraper.sucursales_supermercado
SET nombre        = 'Lider Temuco Prieto Norte',
    direccion     = 'Prieto Norte 0320',
    comuna        = 'Temuco',
    ciudad        = 'Temuco',
    lat           = -38.7288676,
    lon           = -72.6003927,
    hora_apertura = '08:00:00',
    hora_cierre   = '21:30:00'
WHERE codigo_sucursal = 'LIDER-PRIETO-NORTE';

UPDATE scraper.sucursales_supermercado
SET nombre        = 'Santa Isabel Temuco Carrera',
    direccion     = 'Carrera 499',
    comuna        = 'Temuco',
    ciudad        = 'Temuco',
    lat           = -38.7371240,
    lon           = -72.5966257,
    hora_apertura = '08:30:00',
    hora_cierre   = '21:00:00'
WHERE codigo_sucursal = 'SANTA-ISABEL-SAN-MARTIN';

COMMIT;
