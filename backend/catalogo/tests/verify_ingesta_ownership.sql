-- Ejecutado sólo en PostgreSQL efímero del smoke test, nunca en el clúster.
DO $$
BEGIN
    IF (SELECT count(*) FROM scraper.cadenas_supermercado WHERE nombre = 'SUP269 Auto Source') <> 1
       OR (SELECT count(*) FROM scraper.sucursales_supermercado ss JOIN scraper.cadenas_supermercado cs
           ON ss.cadena_id = cs.id WHERE cs.nombre = 'SUP269 Auto Source') <> 1 THEN
        RAISE EXCEPTION 'La resolución concurrente duplicó cadena o sucursal';
    END IF;
    IF (SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc
        ON pc.id = cp.producto_crudo_id WHERE pc.sku = 'SUP269-COUNTER-PENDING') <> 1 THEN
        RAISE EXCEPTION 'El lote confirmado con contador pendiente no conservó una captura';
    END IF;
    IF has_table_privilege('app_scraping', 'scraper.productos_crudos', 'INSERT')
       OR has_table_privilege('app_scraping', 'scraper.capturas_precios', 'UPDATE')
       OR has_table_privilege('app_scraping', 'scraper.cadenas_supermercado', 'SELECT')
       OR has_table_privilege('app_catalogo', 'scraper.trabajos_scraper', 'SELECT')
       OR has_table_privilege('app_catalogo', 'api.usuarios', 'SELECT') THEN
        RAISE EXCEPTION 'Los roles del smoke permiten cruzar propietarios';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM scraper.productos_crudos WHERE sku = 'SUP269-CONCURRENT')
       OR (SELECT count(*) FROM scraper.productos_crudos WHERE sku = 'SUP269-CONCURRENT') <> 1
       OR (SELECT count(*) FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc
           ON pc.id = cp.producto_crudo_id WHERE pc.sku = 'SUP269-CONCURRENT') <> 12 THEN
        RAISE EXCEPTION 'Upsert concurrente incorrecto';
    END IF;
    IF EXISTS (SELECT 1 FROM scraper.productos_crudos WHERE sku = 'SUP269-ROLLBACK-FIRST') THEN
        RAISE EXCEPTION 'El lote fallido dejó un producto parcial';
    END IF;
    IF (SELECT en_stock FROM scraper.productos_crudos WHERE sku = 'SUP269-STOCK') IS DISTINCT FROM false
       OR EXISTS (SELECT 1 FROM scraper.capturas_precios cp JOIN scraper.productos_crudos pc
           ON pc.id = cp.producto_crudo_id WHERE pc.sku = 'SUP269-STOCK' AND cp.esta_disponible <> false) THEN
        RAISE EXCEPTION 'No se conservó stock=false';
    END IF;
END $$;
