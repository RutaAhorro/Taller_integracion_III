-- =======================================================
-- ESQUEMA scraper: DOMINIO 2 (Cadenas y Sucursales) y DOMINIO 3 (Extracción Raw y Precios)
-- =======================================================

CREATE SCHEMA IF NOT EXISTS scraper;

-- 1. Cadenas de Supermercado (Dominio 2)
CREATE TABLE IF NOT EXISTS scraper.cadenas_supermercado (
    id SERIAL PRIMARY KEY,
    nombre VARCHAR(100) UNIQUE NOT NULL,
    url_sitio_web VARCHAR(255),
    url_logo VARCHAR(255),
    config_scraper_json JSONB,
    esta_activa BOOLEAN DEFAULT true,
    creado_el TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 2. Sucursales de Supermercado (Dominio 2)
CREATE TABLE IF NOT EXISTS scraper.sucursales_supermercado (
    id SERIAL PRIMARY KEY,
    cadena_id INTEGER NOT NULL REFERENCES scraper.cadenas_supermercado(id) ON DELETE CASCADE,
    codigo_sucursal VARCHAR(50),
    nombre VARCHAR(150) NOT NULL,
    direccion VARCHAR(255) NOT NULL,
    comuna VARCHAR(100),
    ciudad VARCHAR(100),
    lat DECIMAL(10,8) NOT NULL,
    lon DECIMAL(11,8) NOT NULL,
    hora_apertura TIME,
    hora_cierre TIME,
    esta_activa BOOLEAN DEFAULT true
);

-- Index geoespacial para consultas de ubicación de sucursales
CREATE INDEX IF NOT EXISTS idx_sucursales_coords ON scraper.sucursales_supermercado (lat, lon);

-- 3. Trabajos de Scraper (Dominio 3)
CREATE TABLE IF NOT EXISTS scraper.trabajos_scraper (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cadena_id INTEGER NOT NULL REFERENCES scraper.cadenas_supermercado(id) ON DELETE CASCADE,
    disparado_por_usuario_id UUID REFERENCES api.usuarios(id) ON DELETE SET NULL,
    estado VARCHAR(30) NOT NULL DEFAULT 'pendiente',
    iniciado_el TIMESTAMP NOT NULL DEFAULT NOW(),
    finalizado_el TIMESTAMP,
    elementos_extraidos INTEGER DEFAULT 0,
    respuestas_bloqueadas INTEGER DEFAULT 0,
    registro_errores TEXT
);

-- 4. Productos Crudos (Dominio 3)
CREATE TABLE IF NOT EXISTS scraper.productos_crudos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sucursal_id INTEGER NOT NULL REFERENCES scraper.sucursales_supermercado(id) ON DELETE CASCADE,
    sku VARCHAR(100) NOT NULL,
    ean_gtin VARCHAR(50),
    titulo_crudo VARCHAR(255) NOT NULL,
    marca_cruda VARCHAR(100),
    categoria_cruda VARCHAR(100),
    formato_crudo VARCHAR(100),
    url_producto TEXT,
    url_imagen TEXT,
    en_stock BOOLEAN DEFAULT true,
    ultima_extraccion_el TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_sucursal_sku UNIQUE (sucursal_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_productos_crudos_ean ON scraper.productos_crudos (ean_gtin);

-- 5. Capturas de Precios (Dominio 3 - Serie de tiempo)
CREATE TABLE IF NOT EXISTS scraper.capturas_precios (
    id BIGSERIAL PRIMARY KEY,
    producto_crudo_id UUID NOT NULL REFERENCES scraper.productos_crudos(id) ON DELETE CASCADE,
    precio_normal DECIMAL(12,2) NOT NULL,
    precio_oferta DECIMAL(12,2),
    precio_tarjeta DECIMAL(12,2),
    precio_por_unidad DECIMAL(12,2),
    metrica_unidad VARCHAR(20),
    mecanica_promocion VARCHAR(255),
    esta_disponible BOOLEAN DEFAULT true,
    capturado_el TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Índice para acelerar búsquedas en la serie de tiempo de precios
CREATE INDEX IF NOT EXISTS idx_capturas_prod_fecha ON scraper.capturas_precios (producto_crudo_id, capturado_el DESC);
