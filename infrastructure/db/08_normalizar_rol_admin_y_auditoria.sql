-- SUP-242: normalizar el alias histórico de rol y habilitar auditoría administrativa.
-- Aplicar después de 06_propiedad_permisos.sql. Es idempotente.

BEGIN;

-- El alias "administrador" deja de existir como valor de negocio: se persiste
-- siempre como "admin". El código también lo normaliza al leer datos históricos.
UPDATE api.usuarios
SET rol = 'admin'
WHERE LOWER(TRIM(rol)) = 'administrador'
  AND rol <> 'admin';

CREATE TABLE IF NOT EXISTS api.auditoria_admin_usuarios (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    UUID NOT NULL,
    objetivo_id UUID NOT NULL,
    cambios     JSONB NOT NULL,
    creado_el   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auditoria_admin_usuarios_objetivo
    ON api.auditoria_admin_usuarios (objetivo_id, creado_el DESC);

GRANT SELECT, INSERT ON api.auditoria_admin_usuarios TO app_identity;
GRANT USAGE, SELECT ON SEQUENCE api.auditoria_admin_usuarios_id_seq TO app_identity;

COMMIT;
