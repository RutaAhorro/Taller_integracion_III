-- SUP-271. Ejecutar como operador de migraciones, nunca desde un servicio.
-- No mueve tablas ni borra filas. Los roles quedan sin LOGIN hasta provisionar
-- sus contraseñas por separado (07_runtime_credentials.sh o \password en psql).
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';

ALTER TABLE scraper.trabajos_scraper
    ADD COLUMN IF NOT EXISTS cadena_nombre VARCHAR(100) NOT NULL DEFAULT '';
UPDATE scraper.trabajos_scraper AS t SET cadena_nombre = c.nombre
    FROM scraper.cadenas_supermercado AS c
    WHERE t.cadena_id = c.id AND t.cadena_nombre = '';

-- Las referencias entre propietarios son valores de auditoría. Su validación
-- de cadena ocurre por HTTP; ninguna eliminación externa debe borrar trabajos.
DO $$
DECLARE fk RECORD;
BEGIN
    FOR fk IN SELECT conname FROM pg_constraint
        WHERE contype = 'f' AND conrelid = 'scraper.trabajos_scraper'::regclass
        AND confrelid IN ('scraper.cadenas_supermercado'::regclass, 'api.usuarios'::regclass)
    LOOP
        EXECUTE format('ALTER TABLE scraper.trabajos_scraper DROP CONSTRAINT %I', fk.conname);
    END LOOP;
END $$;
COMMENT ON COLUMN scraper.trabajos_scraper.cadena_id IS 'ID externo de Catálogo, validado por HTTP al crear el trabajo; sin cascada entre dominios';
COMMENT ON COLUMN scraper.trabajos_scraper.cadena_nombre IS 'Snapshot de Catálogo al crear el trabajo; conserva la auditoría sin consultas cruzadas';
COMMENT ON COLUMN scraper.trabajos_scraper.disparado_por_usuario_id IS 'UUID externo de auditoría; no acredita identidad ni autorización y no tiene FK entre dominios';

-- SUP-240: el motor de rutas es dueño de rutas.*. El dueño de una lista es el
-- user_id del JWT de Identidad: sin FK hacia api.usuarios ni cascada entre dominios.
DO $$
DECLARE fk RECORD;
BEGIN
    FOR fk IN SELECT conname FROM pg_constraint
        WHERE contype = 'f' AND conrelid = 'rutas.listas_compras'::regclass
        AND confrelid = 'api.usuarios'::regclass
    LOOP
        EXECUTE format('ALTER TABLE rutas.listas_compras DROP CONSTRAINT %I', fk.conname);
    END LOOP;
END $$;
COMMENT ON COLUMN rutas.listas_compras.usuario_id IS 'UUID externo de Identidad (claim user_id del JWT); sin FK ni cascada entre dominios';
CREATE INDEX IF NOT EXISTS idx_listas_compras_usuario ON rutas.listas_compras (usuario_id, actualizada_el DESC);
CREATE INDEX IF NOT EXISTS idx_articulos_lista_compras_lista ON rutas.articulos_lista_compras (lista_id);

-- Estos nombres son exclusivos del runtime. No se reutilizan administradores,
-- propietarios de objetos o roles con membresías que permitan SET ROLE.
DO $$
DECLARE role_name TEXT; role_id OID;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['app_identity','app_scraping','app_catalogo','app_rutas'] LOOP
        SELECT oid INTO role_id FROM pg_roles WHERE rolname = role_name;
        IF role_id IS NULL THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', role_name);
            SELECT oid INTO role_id FROM pg_roles WHERE rolname = role_name;
        END IF;
        IF EXISTS (SELECT 1 FROM pg_roles WHERE oid = role_id AND
            (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit))
            OR EXISTS (SELECT 1 FROM pg_auth_members WHERE member = role_id)
            OR EXISTS (SELECT 1 FROM pg_class WHERE relowner = role_id)
            OR EXISTS (SELECT 1 FROM pg_proc WHERE proowner = role_id)
            OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspowner = role_id)
            OR EXISTS (SELECT 1 FROM pg_database WHERE datdba = role_id) THEN
            RAISE EXCEPTION 'El rol % no es un rol runtime aislado; revisar con el operador', role_name;
        END IF;
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', current_database(), role_name);
        EXECUTE format('REVOKE CREATE ON DATABASE %I FROM %I', current_database(), role_name);
        EXECUTE format('ALTER ROLE %I SET search_path = pg_catalog', role_name);
    END LOOP;
END $$;

REVOKE ALL PRIVILEGES ON SCHEMA api, scraper, rutas FROM app_identity, app_scraping, app_catalogo, app_rutas;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA api, scraper, rutas FROM app_identity, app_scraping, app_catalogo, app_rutas;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA api, scraper, rutas FROM app_identity, app_scraping, app_catalogo, app_rutas;
ALTER DEFAULT PRIVILEGES IN SCHEMA api, scraper, rutas REVOKE ALL ON TABLES FROM app_identity, app_scraping, app_catalogo, app_rutas;
ALTER DEFAULT PRIVILEGES IN SCHEMA api, scraper, rutas REVOKE ALL ON SEQUENCES FROM app_identity, app_scraping, app_catalogo, app_rutas;

GRANT USAGE ON SCHEMA api TO app_identity;
GRANT SELECT, INSERT, UPDATE ON api.usuarios TO app_identity;
-- SUP-184: requiere 05_tokens_recuperacion.sql. Sin DELETE: la limpieza es del operador.
GRANT SELECT, INSERT, UPDATE ON api.tokens_recuperacion TO app_identity;
-- SUP-242: 08 crea api.auditoria_admin_usuarios después de este script. Si ya
-- existe (re-ejecución de 06 en una base migrada), se restauran sus grants para
-- que la auditoría administrativa siga operativa. Sin DELETE ni UPDATE.
DO $$
BEGIN
    IF to_regclass('api.auditoria_admin_usuarios') IS NOT NULL THEN
        EXECUTE 'GRANT SELECT, INSERT ON api.auditoria_admin_usuarios TO app_identity';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE api.auditoria_admin_usuarios_id_seq TO app_identity';
    END IF;
END $$;

GRANT USAGE ON SCHEMA scraper TO app_scraping;
GRANT SELECT, INSERT, UPDATE ON scraper.trabajos_scraper,
    scraper.entregas_ingesta, scraper.entregas_trabajos TO app_scraping;
-- SUP-270 purga sólo ejecuciones de trabajos terminales, no el ledger de ingesta.
GRANT DELETE ON scraper.entregas_trabajos TO app_scraping;

GRANT USAGE ON SCHEMA scraper TO app_catalogo;
GRANT SELECT, INSERT, UPDATE ON scraper.productos_crudos TO app_catalogo;
GRANT SELECT, INSERT ON scraper.cadenas_supermercado TO app_catalogo;
-- SUP-243: la administración de fuentes sólo modifica el flag de activación.
GRANT UPDATE (esta_activa) ON scraper.cadenas_supermercado TO app_catalogo;
GRANT SELECT, INSERT ON scraper.sucursales_supermercado, scraper.capturas_precios,
    scraper.lotes_catalogo TO app_catalogo;
GRANT USAGE ON SEQUENCE scraper.cadenas_supermercado_id_seq,
    scraper.sucursales_supermercado_id_seq, scraper.capturas_precios_id_seq TO app_catalogo;

-- SUP-240: CRUD de listas. Los artículos sólo se leen para el detalle; agregarlos
-- y quitarlos llega con SUP-241. Borrar una lista elimina sus artículos por la FK
-- interna, que PostgreSQL ejecuta como dueño de la tabla, no como app_rutas.
GRANT USAGE ON SCHEMA rutas TO app_rutas;
GRANT SELECT, INSERT, UPDATE, DELETE ON rutas.listas_compras TO app_rutas;
GRANT SELECT ON rutas.articulos_lista_compras TO app_rutas;

-- Fallar si concesiones públicas o defaults de otros operadores abren tablas
-- fuera de la matriz. No revocar permisos de IA/rutas u otros usuarios existentes.
DO $$
DECLARE role_name TEXT; relation RECORD; privilege_name TEXT; permitted TEXT[];
BEGIN
    FOREACH role_name IN ARRAY ARRAY['app_identity','app_scraping','app_catalogo','app_rutas'] LOOP
        permitted := CASE role_name
            WHEN 'app_identity' THEN ARRAY['api.usuarios','api.tokens_recuperacion','api.auditoria_admin_usuarios']
            WHEN 'app_scraping' THEN ARRAY['scraper.trabajos_scraper','scraper.entregas_ingesta','scraper.entregas_trabajos']
            WHEN 'app_rutas' THEN ARRAY['rutas.listas_compras','rutas.articulos_lista_compras']
            ELSE ARRAY['scraper.cadenas_supermercado','scraper.sucursales_supermercado','scraper.productos_crudos','scraper.capturas_precios','scraper.lotes_catalogo'] END;
        FOR relation IN SELECT format('%I.%I', n.nspname, c.relname) AS name, c.oid
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname IN ('api','scraper','rutas') AND c.relkind IN ('r','p','v','m','f')
        LOOP
            FOREACH privilege_name IN ARRAY ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER'] LOOP
                IF (has_table_privilege(role_name, relation.oid, privilege_name)
                    OR (privilege_name IN ('SELECT','INSERT','UPDATE','REFERENCES')
                        AND has_any_column_privilege(role_name, relation.oid, privilege_name)))
                    AND (NOT relation.name = ANY(permitted) OR privilege_name IN ('TRUNCATE','REFERENCES','TRIGGER')
                        OR (privilege_name = 'DELETE' AND NOT ((role_name = 'app_scraping' AND relation.name = 'scraper.entregas_trabajos')
                            OR (role_name = 'app_rutas' AND relation.name = 'rutas.listas_compras')))
                        OR (role_name = 'app_catalogo' AND privilege_name = 'UPDATE' AND relation.name NOT IN ('scraper.productos_crudos', 'scraper.cadenas_supermercado'))
                        OR (role_name = 'app_catalogo' AND privilege_name = 'UPDATE'
                            AND relation.name = 'scraper.cadenas_supermercado'
                            AND has_table_privilege(role_name, relation.oid, 'UPDATE'))
                        OR (role_name = 'app_rutas' AND privilege_name <> 'SELECT' AND relation.name = 'rutas.articulos_lista_compras')) THEN
                    RAISE EXCEPTION 'Privilegio efectivo inesperado: % % %; revisar PUBLIC/defaults', role_name, privilege_name, relation.name;
                END IF;
            END LOOP;
        END LOOP;
        FOR relation IN SELECT format('%I.%I', n.nspname, c.relname) AS name, c.oid
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname IN ('api','scraper','rutas') AND c.relkind = 'S'
        LOOP
            FOREACH privilege_name IN ARRAY ARRAY['USAGE','SELECT','UPDATE'] LOOP
                IF has_sequence_privilege(role_name, relation.oid, privilege_name)
                    AND NOT (
                        (role_name = 'app_catalogo' AND privilege_name = 'USAGE'
                            AND relation.name IN ('scraper.cadenas_supermercado_id_seq','scraper.sucursales_supermercado_id_seq','scraper.capturas_precios_id_seq'))
                        OR (role_name = 'app_identity' AND privilege_name IN ('USAGE','SELECT')
                            AND relation.name = 'api.auditoria_admin_usuarios_id_seq')
                    ) THEN
                    RAISE EXCEPTION 'Privilegio efectivo inesperado: % % %; revisar secuencias', role_name, privilege_name, relation.name;
                END IF;
            END LOOP;
        END LOOP;
        IF has_schema_privilege(role_name, 'api', 'CREATE') OR has_schema_privilege(role_name, 'scraper', 'CREATE')
            OR has_schema_privilege(role_name, 'rutas', 'CREATE') OR has_schema_privilege(role_name, 'public', 'CREATE')
            OR has_database_privilege(role_name, current_database(), 'CREATE') THEN
            RAISE EXCEPTION 'El rol % puede crear objetos; revisar privilegios públicos', role_name;
        END IF;
    END LOOP;
    IF EXISTS (SELECT 1 FROM pg_default_acl d, LATERAL aclexplode(d.defaclacl) a
        WHERE d.defaclobjtype IN ('r','S') AND (a.grantee = 0 OR a.grantee IN
            (SELECT oid FROM pg_roles WHERE rolname IN ('app_identity','app_scraping','app_catalogo','app_rutas')))) THEN
        RAISE EXCEPTION 'Hay defaults de tablas/secuencias para PUBLIC o roles runtime; revisar con el operador';
    END IF;
END $$;
COMMIT;
