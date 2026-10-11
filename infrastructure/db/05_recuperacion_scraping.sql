-- SUP-270. Aplicar antes de desplegar API/Catálogo/worker nuevos.
-- Tablas aditivas; no alteran ni eliminan los datos existentes.
BEGIN;
-- Propiedad exclusiva de Catálogo. Sin referencias a trabajos ni usuarios.
CREATE TABLE IF NOT EXISTS scraper.lotes_catalogo (
    id UUID PRIMARY KEY,
    payload_hash CHAR(64) NOT NULL,
    resultado JSONB NOT NULL,
    confirmado_el TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Propiedad de coordinación. La entrega y su contador se confirman juntos.
CREATE TABLE IF NOT EXISTS scraper.entregas_ingesta (
    id UUID PRIMARY KEY,
    trabajo_id UUID REFERENCES scraper.trabajos_scraper(id),
    cadena_id INTEGER NOT NULL DEFAULT 0,
    sucursal_id INTEGER,
    payload_hash CHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    estado TEXT NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','confirmada','fallida')),
    intentos INTEGER NOT NULL DEFAULT 0,
    proximo_intento TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_token UUID,
    lease_hasta TIMESTAMPTZ,
    ultimo_error TEXT,
    resultado JSONB,
    creado_el TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmado_el TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_entregas_ingesta_pendientes
    ON scraper.entregas_ingesta(proximo_intento) WHERE estado = 'pendiente';
CREATE INDEX IF NOT EXISTS idx_entregas_ingesta_trabajo ON scraper.entregas_ingesta(trabajo_id);

-- Outbox de ejecuciones: permite reconstruir Redis sin perder trabajos.
CREATE TABLE IF NOT EXISTS scraper.entregas_trabajos (
    trabajo_id UUID PRIMARY KEY REFERENCES scraper.trabajos_scraper(id),
    payload_hash CHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    proxima_publicacion TIMESTAMPTZ NOT NULL DEFAULT now(),
    creado_el TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_entregas_trabajos_publicacion
    ON scraper.entregas_trabajos(proxima_publicacion);
COMMIT;
