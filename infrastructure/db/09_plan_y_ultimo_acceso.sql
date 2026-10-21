-- SUP-242: plan comercial y último acceso en api.usuarios.
-- Aplicar después de 08_normalizar_rol_admin_y_auditoria.sql. Es idempotente.

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- En una base nueva ambas columnas se agregan en una operación de metadatos
-- (PG 11+), sin reescribir la tabla. Si `plan` ya existía nullable, se rellena
-- una sola vez y luego se fija NOT NULL.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'api' AND table_name = 'usuarios' AND column_name = 'plan'
    ) THEN
        ALTER TABLE api.usuarios ADD COLUMN plan VARCHAR(20) NOT NULL DEFAULT 'basico';
    ELSE
        ALTER TABLE api.usuarios ALTER COLUMN plan SET DEFAULT 'basico';
        UPDATE api.usuarios SET plan = 'basico' WHERE plan IS NULL OR BTRIM(plan) = '';
        ALTER TABLE api.usuarios ALTER COLUMN plan SET NOT NULL;
    END IF;
END $$;

ALTER TABLE api.usuarios
    ADD COLUMN IF NOT EXISTS ultimo_acceso TIMESTAMPTZ;

-- NOT VALID evita validar toda la tabla bajo AccessExclusiveLock; VALIDATE usa
-- un lock más débil y permite seguir operando durante la comprobación.
ALTER TABLE api.usuarios
    DROP CONSTRAINT IF EXISTS usuarios_plan_check;
ALTER TABLE api.usuarios
    ADD CONSTRAINT usuarios_plan_check CHECK (plan IN ('basico', 'premium')) NOT VALID;
ALTER TABLE api.usuarios
    VALIDATE CONSTRAINT usuarios_plan_check;

COMMIT;
