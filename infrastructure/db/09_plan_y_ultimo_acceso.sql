-- SUP-242: plan comercial y último acceso en api.usuarios.
-- Aplicar después de 08_normalizar_rol_admin_y_auditoria.sql. Es idempotente.
--
-- Se ejecuta en dos pasos transaccionales separados a propósito: el
-- AccessExclusiveLock que toman ALTER TABLE ADD COLUMN / DROP-ADD CONSTRAINT
-- se libera al hacer COMMIT. La validación del CHECK (escaneo secuencial) se
-- hace después, en su propia transacción implícita, con
-- ShareUpdateExclusiveLock, que permite SELECT/INSERT/UPDATE/DELETE.

-- Paso 1/2: DDL de metadatos. Sólo bloqueos breves.
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

-- NOT VALID evita validar toda la tabla bajo AccessExclusiveLock. La
-- validación real ocurre en el paso 2, ya sin ese lock retenido.
ALTER TABLE api.usuarios
    DROP CONSTRAINT IF EXISTS usuarios_plan_check;
ALTER TABLE api.usuarios
    ADD CONSTRAINT usuarios_plan_check CHECK (plan IN ('basico', 'premium')) NOT VALID;

COMMIT;

-- Paso 2/2: validación fuera de la transacción anterior. Toma
-- ShareUpdateExclusiveLock y permite seguir operando durante el escaneo.
-- Re-ejecutar sobre una constraint ya validada es un no-op rápido.
SET lock_timeout = '5s';
SET statement_timeout = '0';

ALTER TABLE api.usuarios VALIDATE CONSTRAINT usuarios_plan_check;
