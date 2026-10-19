"""Configuración: el servicio no arranca con credenciales ajenas, secretos débiles ni CORS abierto."""

import sys
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.config import ErrorConfiguracion, cargar  # noqa: E402

VALIDO = {
    "RUTAS_DB_URL": "postgres://app_rutas:fixture-private@db:5432/supermercados?sslmode=disable",
    "JWT_SECRET": "j" * 32,
    "CORS_ALLOWED_ORIGINS": "https://frontend.example, http://localhost:5173",
}


class ConfiguracionTest(unittest.TestCase):
    def test_valida(self):
        config = cargar(VALIDO)
        self.assertEqual(config.origenes_cors, ("https://frontend.example", "http://localhost:5173"))
        self.assertEqual(config.db_pool_max, 5)

    def test_rechaza_sin_exponer_secretos(self):
        for clave, valor in (
            ("RUTAS_DB_URL", ""),
            ("RUTAS_DB_URL", "postgres://postgres:fixture-private@db/supermercados"),
            ("RUTAS_DB_URL", "postgres://app_catalogo:fixture-private@db/supermercados"),
            ("RUTAS_DB_URL", "postgres://app_rutas@db/supermercados"),
            ("RUTAS_DB_URL", "postgres://app_rutas:fixture-private@db"),
            ("RUTAS_DB_URL", "mysql://app_rutas:fixture-private@db/supermercados"),
            ("RUTAS_DB_URL", "postgres://app_rutas:fixture-private@db:99999999/supermercados"),
            ("JWT_SECRET", "corta"),
            ("JWT_SECRET", " " + "j" * 32),
            ("CORS_ALLOWED_ORIGINS", "*"),
            ("CORS_ALLOWED_ORIGINS", "null"),
            ("CORS_ALLOWED_ORIGINS", "https://frontend.example/ruta"),
            ("CORS_ALLOWED_ORIGINS", "https://user:fixture-private@frontend.example"),
            ("RUTAS_DB_POOL_MAX", "0"),
            ("RUTAS_DB_POOL_MAX", "muchos"),
        ):
            with self.subTest(clave=clave, valor=valor):
                with self.assertRaises(ErrorConfiguracion) as error:
                    cargar({**VALIDO, clave: valor})
                self.assertNotIn("fixture-private", str(error.exception))


if __name__ == "__main__":
    unittest.main()
