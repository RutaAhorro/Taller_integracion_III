-- Únicamente en la base temporal del humo SUP-267.
INSERT INTO api.usuarios (correo, password_hash, nombre_completo, rol, esta_activo)
VALUES
  ('admin.sup267@example.test', crypt('Sup267FixturePassword123!', gen_salt('bf')), 'Admin SUP267', 'admin', true),
  ('registered.sup267@example.test', crypt('Sup267FixturePassword123!', gen_salt('bf')), 'Registered SUP267', 'registrado', true);

INSERT INTO scraper.productos_crudos
  (id, sucursal_id, sku, titulo_crudo, marca_cruda, categoria_cruda, formato_crudo, en_stock, ultima_extraccion_el)
VALUES
  ('00000000-0000-4000-8000-000000002671', 1, 'SUP267-FRESH', 'Leche Contrato SUP267', 'MarcaContrato', 'Lacteos', '1 L', true, now()),
  ('00000000-0000-4000-8000-000000002672', 1, 'SUP267-STALE', 'Arroz Contrato SUP267', 'MarcaContrato', 'Despensa', '1 kg', false, now() - interval '10 days');

INSERT INTO scraper.capturas_precios (producto_crudo_id, precio_normal, precio_oferta, esta_disponible, capturado_el)
VALUES
  ('00000000-0000-4000-8000-000000002671', 1490, NULL, true, now() - interval '2 days'),
  ('00000000-0000-4000-8000-000000002671', 1290, 990, true, now() - interval '1 hour'),
  ('00000000-0000-4000-8000-000000002672', 1990, NULL, false, now() - interval '10 days');

-- SUP-376: Catálogo ya no crea sucursales. El humo de ingesta necesita una
-- cadena nueva con una sucursal REAL registrada de antemano (ingesta y
-- resolución concurrente de sucursal), distinta de la cadena "SUP269 Auto
-- Source", que se deja a propósito sin sucursal para comprobar el 404.
INSERT INTO scraper.cadenas_supermercado (nombre, esta_activa) VALUES ('SUP376 Seeded Source', true);
INSERT INTO scraper.sucursales_supermercado (cadena_id, codigo_sucursal, nombre, direccion, comuna, ciudad, lat, lon, esta_activa)
SELECT id, 'SUP376-SEEDED', 'SUP376 Seeded Source Sucursal', 'Dirección de prueba SUP376 123', 'Temuco', 'Temuco', -38.7410, -72.6000, true
FROM scraper.cadenas_supermercado WHERE nombre = 'SUP376 Seeded Source';
