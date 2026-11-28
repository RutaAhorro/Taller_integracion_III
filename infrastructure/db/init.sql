-- Inicialización de una base PostgreSQL + PostGIS nueva (SUP-370).
--
-- Este archivo sólo siembra CONFIGURACIÓN: las extensiones, los esquemas y las
-- seis cadenas de supermercado con sus sucursales, con ids estables porque los
-- humos (fixtures de Catálogo, ingesta, enrutamiento y control) y la ingesta
-- del scraper los referencian por id (cadena 1 'Jumbo', sucursal 1, etc.).
--
-- NO siembra productos, precios, marcas, categorías ni mapeos de IA. Los
-- productos y precios llegan por la ingesta del scraper, y los tests siembran
-- sus propios datos (por ejemplo backend/catalogo/tests/fixtures.sql). Antes de
-- SUP-370 este archivo cargaba 45 productos de demo de Jumbo; ese seed se retiró
-- para que una base nueva no mezcle datos ficticios con datos reales.
--
-- Las sucursales 5 y 6 (A Cuenta y Cugat, tiendas en línea) tienen datos de
-- ubicación provisionales (dirección y coordenadas del centro de Temuco); el
-- pendiente está documentado en backend/catalogo/README.md.
--
-- Las bases ya desplegadas conservan los productos de demo anteriores: retirarlos
-- requiere una migración autorizada, fuera del alcance de este archivo.

-- ===== Extensiones y esquemas =====
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "postgis";

CREATE SCHEMA IF NOT EXISTS api;
CREATE SCHEMA IF NOT EXISTS scraper;
CREATE SCHEMA IF NOT EXISTS rutas;

COMMENT ON SCHEMA api IS 'Dominio 1 (Identidad, Usuarios) y Dominio 4 (Catálogo Maestro y Recetas)';
COMMENT ON SCHEMA scraper IS 'Dominio 2 (Cadenas y Sucursales) y Dominio 3 (Extracción y Capturas de Precios)';
COMMENT ON SCHEMA rutas IS 'Dominio 5 (Listas de Compras, Rutas Geoespaciales y Optimización TSP)';

-- ===== Cadenas de supermercado (scraper.cadenas_supermercado) =====
INSERT INTO scraper.cadenas_supermercado (id, nombre, url_sitio_web, url_logo, esta_activa) VALUES
(1, 'Jumbo', 'https://www.jumbo.cl', '/logos/verdemart.png', true),
(2, 'Lider', 'https://www.lider.cl', '/logos/superahorro.png', true),
(3, 'Unimarc', 'https://www.unimarc.cl', '/logos/mercadia.png', true),
(4, 'Santa Isabel', 'https://www.santaisabel.cl', '/logos/colmadoplus.png', true),
(5, 'A Cuenta', 'https://www.acuenta.cl', '/logos/acuenta.png', true),
(6, 'Cugat', 'https://cugat.cl', '/logos/cugat.png', true)
ON CONFLICT (id) DO NOTHING;

SELECT setval('scraper.cadenas_supermercado_id_seq', (SELECT MAX(id) FROM scraper.cadenas_supermercado));

-- ===== Sucursales de supermercado (scraper.sucursales_supermercado) =====
INSERT INTO scraper.sucursales_supermercado (id, cadena_id, codigo_sucursal, nombre, direccion, comuna, ciudad, lat, lon, hora_apertura, hora_cierre, esta_activa) VALUES
(1, 1, 'JUMBO-LOS-PABLOS', 'Jumbo Los Pablos', 'Av. Los Pablos 01860', 'Temuco', 'Temuco', -38.74100000, -72.60000000, '08:30:00', '21:00:00', true),
(2, 2, 'LIDER-PRIETO-NORTE', 'Lider Temuco Prieto Norte', 'Prieto Norte 0320', 'Temuco', 'Temuco', -38.72886760, -72.60039270, '08:00:00', '21:30:00', true),
(3, 3, 'UNIMARC-ALEMANIA', 'Unimarc Av. Alemania', 'Av. Alemania 0655', 'Temuco', 'Temuco', -38.74500000, -72.61000000, '08:30:00', '21:00:00', true),
(4, 4, 'SANTA-ISABEL-SAN-MARTIN', 'Santa Isabel Temuco Carrera', 'Carrera 499', 'Temuco', 'Temuco', -38.73712400, -72.59662570, '08:30:00', '21:00:00', true),
(5, 5, 'ACUENTA-CENTRAL', 'A Cuenta Online', 'Casa Matriz / Online', 'Temuco', 'Temuco', -38.73590000, -72.59040000, NULL, NULL, true),
(6, 6, 'CUGAT-CENTRAL', 'Cugat Online', 'Casa Matriz / Online', 'Temuco', 'Temuco', -38.73590000, -72.59040000, NULL, NULL, true)
ON CONFLICT (id) DO NOTHING;

SELECT setval('scraper.sucursales_supermercado_id_seq', (SELECT MAX(id) FROM scraper.sucursales_supermercado));
