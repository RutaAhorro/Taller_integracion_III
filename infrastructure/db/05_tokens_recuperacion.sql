-- SUP-184. Ejecutar como operador ANTES de re-ejecutar 06_propiedad_permisos.sql,
-- que concede los permisos de esta tabla a app_identity. Idempotente.
-- Tokens de un solo uso para restablecer contraseña: se guarda sólo el SHA-256;
-- el token en claro viaja únicamente en el correo.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS api.tokens_recuperacion (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL REFERENCES api.usuarios(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expira_el TIMESTAMPTZ NOT NULL,
    usado_el TIMESTAMPTZ,
    creado_el TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tokens_recuperacion_usuario
    ON api.tokens_recuperacion (usuario_id, creado_el DESC);

COMMENT ON TABLE api.tokens_recuperacion IS 'SUP-184: tokens de restablecimiento de contraseña; dueño Identidad (app_identity)';
COMMENT ON COLUMN api.tokens_recuperacion.token_hash IS 'SHA-256 del token enviado por correo; el token en claro nunca se guarda';
COMMENT ON COLUMN api.tokens_recuperacion.usado_el IS 'Consumido (SUP-185) o reemplazado por una solicitud más nueva; con valor, el token ya no sirve';
COMMIT;
