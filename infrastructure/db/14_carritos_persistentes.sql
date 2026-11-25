-- Carrito borrador por cuenta, separado de las listas confirmadas.
-- Ejecutar como operador: no modifica listas ni datos de otros dominios.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS rutas.carritos (
    usuario_id UUID PRIMARY KEY,
    version BIGINT NOT NULL DEFAULT 0 CHECK (version >= 0),
    articulos JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(articulos) = 'array' AND jsonb_array_length(articulos) <= 100),
    envio JSONB CHECK (envio IS NULL OR jsonb_typeof(envio) = 'object'),
    ultimo_envio JSONB CHECK (ultimo_envio IS NULL OR jsonb_typeof(ultimo_envio) = 'object'),
    actualizado_el TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE rutas.carritos IS 'Borrador por cuenta de Motor/Listas, con versión y envío recuperable; sin precios persistidos';
COMMENT ON COLUMN rutas.carritos.usuario_id IS 'UUID externo de Identidad, obtenido sólo del JWT; sin FK entre dominios';
-- No se elimina la fila al vaciar: la versión y el último resultado protegen reintentos.
GRANT SELECT, INSERT, UPDATE ON rutas.carritos TO app_rutas;
COMMIT;
