-- =======================================================
-- Desactivar JIT en roles de consulta (app_catalogo, app_rutas)
-- =======================================================
-- Ejecutar como operador (POSTGRES_USER) después de 06_propiedad_permisos.sql.
-- Idempotente: segura en bases nuevas y existentes.
-- Motivo: con límite de CPU acotado en PostgreSQL, la compilación JIT sumaba
-- ~1 s por consulta en Catálogo y Motor de rutas.

-- ===== Configuración de roles runtime =====
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_catalogo') THEN
        ALTER ROLE app_catalogo SET jit = off;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rutas') THEN
        ALTER ROLE app_rutas SET jit = off;
    END IF;
END
$$;
