-- SUP-347: api.usuarios.rol sólo admite los tres roles vigentes.
-- Aplicar después de 09_plan_y_ultimo_acceso.sql. Es idempotente.
--
-- Hasta ahora la columna aceptaba cualquier texto, y por eso Identidad
-- conservaba en el código el alias histórico "administrador". Con esta
-- restricción la base deja de aceptar un rol que el servicio no reconoce, y el
-- código ya no necesita traducir alias. Identidad comprueba al arrancar que la
-- restricción exista.
--
-- Igual que la 09, va en dos pasos: el DDL con un bloqueo breve y la
-- validación (que recorre la tabla) en otra transacción, sin retener el
-- AccessExclusiveLock.

-- Paso 1/2: normalizar los datos y agregar la restricción sin validar.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

-- Repite la normalización de la 08 por si esa no se aplicó, y corrige además
-- mayúsculas y espacios en los roles vigentes.
UPDATE api.usuarios SET rol = 'admin'
WHERE LOWER(BTRIM(rol)) = 'administrador';

UPDATE api.usuarios SET rol = LOWER(BTRIM(rol))
WHERE LOWER(BTRIM(rol)) IN ('admin', 'colaborador', 'registrado')
  AND rol <> LOWER(BTRIM(rol));

-- Cualquier otro valor no tiene una traducción segura: se aborta para que un
-- operador lo revise en vez de asignar un rol a ciegas.
DO $$
DECLARE
    desconocidos BIGINT;
BEGIN
    SELECT COUNT(*) INTO desconocidos FROM api.usuarios
    WHERE rol NOT IN ('admin', 'colaborador', 'registrado');
    IF desconocidos > 0 THEN
        RAISE EXCEPTION '% filas de api.usuarios tienen un rol desconocido; corregirlas antes de aplicar 10', desconocidos;
    END IF;
END $$;

ALTER TABLE api.usuarios
    DROP CONSTRAINT IF EXISTS usuarios_rol_check;
ALTER TABLE api.usuarios
    ADD CONSTRAINT usuarios_rol_check CHECK (rol IN ('admin', 'colaborador', 'registrado')) NOT VALID;

COMMIT;

-- Paso 2/2: validar fuera de la transacción anterior. Permite seguir leyendo y
-- escribiendo durante el recorrido; sobre una restricción ya validada es un no-op.
SET lock_timeout = '5s';
SET statement_timeout = '0';

ALTER TABLE api.usuarios VALIDATE CONSTRAINT usuarios_rol_check;
