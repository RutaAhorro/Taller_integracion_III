"""Pool PostgreSQL de app_rutas. Sin migraciones: el esquema y los grants vienen de 06_propiedad_permisos.sql."""

from __future__ import annotations

import asyncio

from psycopg_pool import AsyncConnectionPool

# app_rutas tiene search_path = pg_catalog: todas las consultas califican el esquema.
_OPCIONES = "-c statement_timeout=5000 -c TimeZone=UTC"

_PERMISOS = """
SELECT has_schema_privilege(current_user, 'rutas', 'USAGE')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'SELECT')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'INSERT')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'UPDATE')
   AND has_table_privilege(current_user, 'rutas.listas_compras', 'DELETE')
   AND has_table_privilege(current_user, 'rutas.articulos_lista_compras', 'SELECT')
"""


class ErrorAlmacenamiento(RuntimeError):
    pass


async def abrir_pool(dsn: str, maximo: int) -> AsyncConnectionPool:
    pool = AsyncConnectionPool(
        dsn,
        min_size=1,
        max_size=maximo,
        max_lifetime=30 * 60,
        timeout=5,
        open=False,
        kwargs={"autocommit": True, "connect_timeout": 5, "application_name": "motor_rutas", "options": _OPCIONES},
    )
    try:
        await pool.open(wait=True, timeout=5)
    except Exception as error:
        await pool.close()
        # Sólo el tipo: el detalle de la conexión queda en el log de psycopg_pool, nunca el DSN.
        raise ErrorAlmacenamiento(f"no se pudo conectar a PostgreSQL con RUTAS_DB_URL ({type(error).__name__})") from None
    return pool


async def verificar_permisos(pool: AsyncConnectionPool) -> None:
    """Comprueba una vez al arrancar las tablas y grants que usa el servicio."""
    try:
        async with pool.connection() as conn:
            cursor = await conn.execute(_PERMISOS)
            fila = await cursor.fetchone()
    except Exception as error:
        # Conexión o timeout: no es un problema de grants. Sólo el tipo, nunca el DSN.
        raise ErrorAlmacenamiento(f"no se pudo verificar los permisos de rutas.* ({type(error).__name__})") from None
    if not fila or not fila[0]:
        raise ErrorAlmacenamiento("aplicar 06_propiedad_permisos.sql antes de iniciar el motor de rutas")


async def ping(pool: AsyncConnectionPool, plazo: float = 2.0) -> bool:
    async def _consulta() -> None:
        async with pool.connection(timeout=plazo) as conn:
            await conn.execute("SELECT 1")

    try:
        await asyncio.wait_for(_consulta(), plazo)
        return True
    except Exception:
        return False
