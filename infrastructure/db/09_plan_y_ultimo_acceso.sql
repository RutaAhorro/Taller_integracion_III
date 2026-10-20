-- SUP-242: plan comercial y último acceso en api.usuarios.
-- Aplicar después de 08_normalizar_rol_admin_y_auditoria.sql. Es idempotente.

BEGIN;

ALTER TABLE api.usuarios
    ADD COLUMN IF NOT EXISTS plan VARCHAR(20),
    ADD COLUMN IF NOT EXISTS ultimo_acceso TIMESTAMPTZ;

UPDATE api.usuarios
SET plan = 'basico'
WHERE plan IS NULL OR BTRIM(plan) = '';

ALTER TABLE api.usuarios
    ALTER COLUMN plan SET DEFAULT 'basico',
    ALTER COLUMN plan SET NOT NULL;

ALTER TABLE api.usuarios
    DROP CONSTRAINT IF EXISTS usuarios_plan_check;
ALTER TABLE api.usuarios
    ADD CONSTRAINT usuarios_plan_check CHECK (plan IN ('basico', 'premium'));

COMMIT;
