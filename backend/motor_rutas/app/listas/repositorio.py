"""Persistencia de listas en rutas.*. Toda consulta filtra por el dueño de la sesión."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Protocol

from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool

from app.listas.modelos import Articulo, Lista, ListaDetalle

_COLUMNAS = """l.id::text AS id, l.nombre, l.estado, l.creada_el, l.actualizada_el,
    (SELECT count(*) FROM rutas.articulos_lista_compras a WHERE a.lista_id = l.id) AS total_articulos"""


class RepositorioListas(Protocol):
    async def contar(self, usuario_id: str) -> int: ...
    async def listar(self, usuario_id: str, limite: int) -> list[Lista]: ...
    async def crear(self, usuario_id: str, nombre: str) -> Lista: ...
    async def obtener(self, usuario_id: str, lista_id: str) -> ListaDetalle | None: ...
    async def actualizar(self, usuario_id: str, lista_id: str, nombre: str | None, estado: str | None) -> Lista | None: ...
    async def eliminar(self, usuario_id: str, lista_id: str) -> bool: ...


def _utc(valor: datetime) -> datetime:
    # Las columnas son TIMESTAMP sin zona y el pool fija TimeZone=UTC.
    return valor.replace(tzinfo=timezone.utc) if valor.tzinfo is None else valor


def _lista(fila: dict) -> Lista:
    return Lista(
        id=fila["id"],
        nombre=fila["nombre"],
        estado=fila["estado"] or "activa",
        total_articulos=fila["total_articulos"],
        creada_el=_utc(fila["creada_el"]),
        actualizada_el=_utc(fila["actualizada_el"]),
    )


class RepositorioListasPostgres:
    def __init__(self, pool: AsyncConnectionPool) -> None:
        self._pool = pool

    async def _una(self, consulta: str, parametros: tuple) -> dict | None:
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cursor:
            await cursor.execute(consulta, parametros)
            return await cursor.fetchone()

    async def _todas(self, consulta: str, parametros: tuple) -> list[dict]:
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cursor:
            await cursor.execute(consulta, parametros)
            return await cursor.fetchall()

    async def contar(self, usuario_id: str) -> int:
        fila = await self._una("SELECT count(*) AS total FROM rutas.listas_compras WHERE usuario_id = %s", (usuario_id,))
        return int(fila["total"])

    async def listar(self, usuario_id: str, limite: int) -> list[Lista]:
        filas = await self._todas(
            f"SELECT {_COLUMNAS} FROM rutas.listas_compras l WHERE l.usuario_id = %s ORDER BY l.actualizada_el DESC, l.id LIMIT %s",
            (usuario_id, limite),
        )
        return [_lista(fila) for fila in filas]

    async def crear(self, usuario_id: str, nombre: str) -> Lista:
        fila = await self._una(
            "INSERT INTO rutas.listas_compras (usuario_id, nombre) VALUES (%s, %s) "
            "RETURNING id::text AS id, nombre, estado, creada_el, actualizada_el, 0 AS total_articulos",
            (usuario_id, nombre),
        )
        return _lista(fila)

    async def obtener(self, usuario_id: str, lista_id: str) -> ListaDetalle | None:
        # Ambas consultas en la misma conexión: los artículos sólo se leen si la lista es del usuario.
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cursor:
            await cursor.execute(f"SELECT {_COLUMNAS} FROM rutas.listas_compras l WHERE l.id = %s AND l.usuario_id = %s", (lista_id, usuario_id))
            fila = await cursor.fetchone()
            if fila is None:
                return None
            await cursor.execute(
                "SELECT id, producto_normalizado_id::text AS producto_normalizado_id, cantidad, COALESCE(esta_comprado, false) AS esta_comprado "
                "FROM rutas.articulos_lista_compras WHERE lista_id = %s ORDER BY id",
                (lista_id,),
            )
            articulos = [Articulo(**articulo) for articulo in await cursor.fetchall()]
        return ListaDetalle(**_lista(fila).model_dump(), articulos=articulos)

    async def actualizar(self, usuario_id: str, lista_id: str, nombre: str | None, estado: str | None) -> Lista | None:
        fila = await self._una(
            "UPDATE rutas.listas_compras l SET nombre = COALESCE(%s, l.nombre), estado = COALESCE(%s, l.estado), actualizada_el = now() "
            f"WHERE l.id = %s AND l.usuario_id = %s RETURNING {_COLUMNAS}",
            (nombre, estado, lista_id, usuario_id),
        )
        return None if fila is None else _lista(fila)

    async def eliminar(self, usuario_id: str, lista_id: str) -> bool:
        async with self._pool.connection() as conn:
            cursor = await conn.execute("DELETE FROM rutas.listas_compras WHERE id = %s AND usuario_id = %s", (lista_id, usuario_id))
            return cursor.rowcount == 1
