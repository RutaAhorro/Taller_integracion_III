-- SUP-239: direcciones guardadas del usuario (CRUD en Identidad).
-- Aplicar después de 06 (crea el rol app_identity) y 10. Es idempotente.
--
-- La tabla api.direcciones_usuario existe desde 01_schema_api.sql pero nunca
-- tuvo código ni permisos. Este script la adapta al contrato del CRUD:
--   * comuna: columna nueva. Queda NULLABLE en la base porque una fila previa
--     no tiene de dónde obtenerla; la API la exige al crear y al actualizar.
--   * lat/lon: pasan a ser opcionales (una dirección puede guardarse sin
--     geocodificar), pero se exigen ambas o ninguna y dentro de rango.
--   * creado_el: columna nueva (NOT NULL DEFAULT now(): sólo metadatos en
--     PostgreSQL 11+). Da un orden estable y permite promover la dirección más
--     antigua al borrar la principal. Las filas previas quedan con la fecha de
--     esta migración y se desempatan por id.
--   * es_principal: NOT NULL DEFAULT false e índice único parcial que garantiza
--     una sola principal por usuario, incluso ante peticiones concurrentes. Los
--     datos previos se normalizan: duplicadas se desmarcan (queda la más antigua)
--     y un usuario con direcciones y sin principal recibe la más antigua.
--   * Permisos: app_identity recibe exactamente SELECT, INSERT, UPDATE y DELETE
--     sobre esta tabla. Ningún otro rol runtime tiene ni necesita acceso: el
--     motor de rutas obtendrá el origen por HTTP, no por SQL cruzado.
--
-- Va en dos pasos, igual que 09 y 10: el DDL con bloqueos breves y la validación
-- de los CHECK (que recorre la tabla) en otra transacción, sin retener el
-- AccessExclusiveLock durante el escaneo.

-- Paso 1/2: DDL de metadatos y permisos.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

ALTER TABLE api.direcciones_usuario
    ADD COLUMN IF NOT EXISTS comuna VARCHAR(100);
ALTER TABLE api.direcciones_usuario
    ADD COLUMN IF NOT EXISTS creado_el TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE api.direcciones_usuario ALTER COLUMN lat DROP NOT NULL;
ALTER TABLE api.direcciones_usuario ALTER COLUMN lon DROP NOT NULL;

-- Una fila previa con es_principal NULL no es principal. Se fija NOT NULL sólo
-- después de normalizarla, para que la migración no falle con datos existentes.
UPDATE api.direcciones_usuario SET es_principal = false WHERE es_principal IS NULL;
ALTER TABLE api.direcciones_usuario ALTER COLUMN es_principal SET DEFAULT false;
ALTER TABLE api.direcciones_usuario ALTER COLUMN es_principal SET NOT NULL;

-- Si un usuario ya tuviera varias principales (el esquema original no lo
-- impedía), se conserva la más antigua y se desmarcan las demás; de lo
-- contrario el índice único parcial no podría crearse.
WITH ordenadas AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY usuario_id ORDER BY creado_el, id) AS posicion
    FROM api.direcciones_usuario
    WHERE es_principal
)
UPDATE api.direcciones_usuario AS direccion
SET es_principal = false
FROM ordenadas
WHERE direccion.id = ordenadas.id AND ordenadas.posicion > 1;

-- Un usuario con direcciones previas pero ninguna principal (todas NULL/false)
-- recibe como principal la más antigua, para cumplir la invariante "si tiene
-- direcciones, tiene una principal" que mantiene la API. Idempotente: tras la
-- primera pasada ya no hay usuarios sin principal.
WITH candidatas AS (
    SELECT DISTINCT ON (usuario_id) id
    FROM api.direcciones_usuario AS direccion
    WHERE NOT EXISTS (
        SELECT 1 FROM api.direcciones_usuario AS otra
        WHERE otra.usuario_id = direccion.usuario_id AND otra.es_principal)
    ORDER BY usuario_id, creado_el, id
)
UPDATE api.direcciones_usuario AS direccion
SET es_principal = true
FROM candidatas
WHERE direccion.id = candidatas.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_direcciones_usuario_principal
    ON api.direcciones_usuario (usuario_id) WHERE es_principal;
CREATE INDEX IF NOT EXISTS idx_direcciones_usuario_usuario
    ON api.direcciones_usuario (usuario_id);

-- NOT VALID evita validar toda la tabla bajo AccessExclusiveLock; la validación
-- real ocurre en el paso 2.
ALTER TABLE api.direcciones_usuario DROP CONSTRAINT IF EXISTS direcciones_usuario_coordenadas_par_check;
ALTER TABLE api.direcciones_usuario
    ADD CONSTRAINT direcciones_usuario_coordenadas_par_check
    CHECK ((lat IS NULL) = (lon IS NULL)) NOT VALID;

ALTER TABLE api.direcciones_usuario DROP CONSTRAINT IF EXISTS direcciones_usuario_coordenadas_rango_check;
ALTER TABLE api.direcciones_usuario
    ADD CONSTRAINT direcciones_usuario_coordenadas_rango_check
    CHECK ((lat IS NULL OR lat BETWEEN -90 AND 90) AND (lon IS NULL OR lon BETWEEN -180 AND 180)) NOT VALID;

-- Una comuna presente no puede ser sólo espacios. Se permite NULL por las filas previas.
ALTER TABLE api.direcciones_usuario DROP CONSTRAINT IF EXISTS direcciones_usuario_comuna_check;
ALTER TABLE api.direcciones_usuario
    ADD CONSTRAINT direcciones_usuario_comuna_check
    CHECK (comuna IS NULL OR BTRIM(comuna) <> '') NOT VALID;

-- Permisos mínimos. Se exige el rol creado por 06 en vez de crearlo aquí: este
-- script no define roles ni sus atributos.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_identity') THEN
        RAISE EXCEPTION 'No existe el rol app_identity; aplicar 06_propiedad_permisos.sql antes de 11';
    END IF;
END $$;
GRANT SELECT, INSERT, UPDATE, DELETE ON api.direcciones_usuario TO app_identity;

COMMIT;

-- Paso 2/2: validar fuera de la transacción anterior. Sobre restricciones ya
-- validadas es un no-op rápido.
SET lock_timeout = '5s';
SET statement_timeout = '0';

ALTER TABLE api.direcciones_usuario VALIDATE CONSTRAINT direcciones_usuario_coordenadas_par_check;
ALTER TABLE api.direcciones_usuario VALIDATE CONSTRAINT direcciones_usuario_coordenadas_rango_check;
ALTER TABLE api.direcciones_usuario VALIDATE CONSTRAINT direcciones_usuario_comuna_check;
