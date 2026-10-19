"""Integración con PostgreSQL real tras 06_propiedad_permisos.sql y 07_runtime_credentials.sh.

Se omite salvo que existan RUTAS_TEST_DB_URL (app_rutas) y RUTAS_TEST_ADMIN_URL (operador)
apuntando a una base desechable inicializada con infrastructure/db.
"""

import os
import sys
import unittest
import uuid
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import psycopg  # noqa: E402

import app.main  # noqa: E402,F401  (fija el event loop compatible con psycopg en Windows)
from app import db  # noqa: E402
from app.listas.repositorio import RepositorioListasPostgres  # noqa: E402

RUTAS_URL = os.environ.get("RUTAS_TEST_DB_URL", "")
ADMIN_URL = os.environ.get("RUTAS_TEST_ADMIN_URL", "")


@unittest.skipUnless(RUTAS_URL and ADMIN_URL, "requiere RUTAS_TEST_DB_URL y RUTAS_TEST_ADMIN_URL")
class RepositorioPostgresTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.pool = await db.abrir_pool(RUTAS_URL, 2)
        self.addAsyncCleanup(self.pool.close)
        self.repo = RepositorioListasPostgres(self.pool)
        self.usuario_a, self.usuario_b = str(uuid.uuid4()), str(uuid.uuid4())

    async def test_permisos_y_crud_aislado_por_usuario(self):
        await db.verificar_permisos(self.pool)
        self.assertTrue(await db.ping(self.pool))
        lista = await self.repo.crear(self.usuario_a, "Semana ñandú")
        self.assertEqual((lista.estado, lista.total_articulos, lista.creada_el.tzinfo is not None), ("activa", 0, True))
        self.assertEqual(await self.repo.contar(self.usuario_a), 1)
        self.assertIsNone(await self.repo.obtener(self.usuario_b, lista.id))
        self.assertIsNone(await self.repo.actualizar(self.usuario_b, lista.id, "robada", None))
        self.assertFalse(await self.repo.eliminar(self.usuario_b, lista.id))
        self.assertEqual(await self.repo.listar(self.usuario_b, 100), [])

        actualizada = await self.repo.actualizar(self.usuario_a, lista.id, None, "completada")
        self.assertEqual((actualizada.nombre, actualizada.estado), ("Semana ñandú", "completada"))
        self.assertGreaterEqual(actualizada.actualizada_el, lista.actualizada_el)
        self.assertEqual([l.id for l in await self.repo.listar(self.usuario_a, 100)], [lista.id])
        self.assertTrue(await self.repo.eliminar(self.usuario_a, lista.id))
        self.assertIsNone(await self.repo.obtener(self.usuario_a, lista.id))

    async def test_detalle_con_articulos_y_borrado_en_cascada(self):
        lista = await self.repo.crear(self.usuario_a, "Con artículos")
        # Los artículos los escribe el operador: app_rutas aún no tiene INSERT (SUP-241).
        async with await psycopg.AsyncConnection.connect(ADMIN_URL, autocommit=True) as admin:
            producto = await (await admin.execute(
                "INSERT INTO api.productos_normalizados (nombre_estandar) VALUES ('Leche prueba SUP-240') RETURNING id::text")).fetchone()
            await admin.execute(
                "INSERT INTO rutas.articulos_lista_compras (lista_id, producto_normalizado_id, cantidad) VALUES (%s, %s, 2)",
                (lista.id, producto[0]))
            detalle = await self.repo.obtener(self.usuario_a, lista.id)
            self.assertEqual((detalle.total_articulos, detalle.articulos[0].cantidad, detalle.articulos[0].producto_normalizado_id), (1, 2, producto[0]))
            self.assertTrue(await self.repo.eliminar(self.usuario_a, lista.id))
            restantes = await (await admin.execute("SELECT count(*) FROM rutas.articulos_lista_compras WHERE lista_id = %s", (lista.id,))).fetchone()
            self.assertEqual(restantes[0], 0)
            await admin.execute("DELETE FROM api.productos_normalizados WHERE id = %s", (producto[0],))

    async def test_app_rutas_no_sale_de_su_matriz(self):
        async with self.pool.connection() as conn:
            for sentencia in (
                "SELECT 1 FROM api.usuarios LIMIT 1",
                "SELECT 1 FROM scraper.productos_crudos LIMIT 1",
                f"INSERT INTO rutas.articulos_lista_compras (lista_id, producto_normalizado_id) VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}')",
                "TRUNCATE rutas.listas_compras",
                "CREATE TABLE rutas.intrusa (id int)",
            ):
                with self.subTest(sentencia=sentencia.split(" (")[0]):
                    with self.assertRaises(psycopg.errors.InsufficientPrivilege):
                        await conn.execute(sentencia)


if __name__ == "__main__":
    unittest.main()
