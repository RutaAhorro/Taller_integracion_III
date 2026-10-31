"""CRUD de listas tras JWT de Identidad: aislamiento por usuario, validación y contrato de errores."""

import sys
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from fastapi.testclient import TestClient  # noqa: E402

from app.main import create_app  # noqa: E402
from tests.apoyo import CONFIG, ORIGEN, USUARIO_A, USUARIO_B, RepositorioMemoria, token  # noqa: E402

API = "/api/v1/listas"


class ListasTest(unittest.TestCase):
    def setUp(self) -> None:
        self.repo = RepositorioMemoria()
        self.cliente = TestClient(create_app(CONFIG, self.repo))
        self.cliente.__enter__()
        self.addCleanup(self.cliente.__exit__, None, None, None)
        self.como(USUARIO_A)

    def como(self, usuario_id: str) -> None:
        self.cliente.cookies.set("jwt", token(usuario_id))

    def crear(self, nombre: str = "Semana") -> dict:
        respuesta = self.cliente.post(API, json={"nombre": nombre})
        self.assertEqual(respuesta.status_code, 201, respuesta.text)
        return respuesta.json()

    def test_crud_completo_del_dueno(self):
        creada = self.crear("  Compras   de la semana  ")
        self.assertEqual((creada["nombre"], creada["estado"], creada["total_articulos"]), ("Compras de la semana", "activa", 0))
        self.assertTrue(creada["creada_el"].endswith("Z"))

        detalle = self.cliente.get(f"{API}/{creada['id']}").json()
        self.assertEqual(detalle["articulos"], [])

        actualizada = self.cliente.put(f"{API}/{creada['id']}", json={"nombre": "Asado ñandú", "estado": "completada"})
        self.assertEqual(actualizada.status_code, 200, actualizada.text)
        self.assertEqual((actualizada.json()["nombre"], actualizada.json()["estado"]), ("Asado ñandú", "completada"))

        self.assertEqual([l["id"] for l in self.cliente.get(API).json()["listas"]], [creada["id"]])
        self.assertEqual(self.cliente.get(API + "/").status_code, 200)
        self.assertEqual(self.cliente.delete(f"{API}/{creada['id']}").status_code, 204)
        self.assertEqual(self.cliente.get(f"{API}/{creada['id']}").status_code, 404)

    def test_listas_ajenas_responden_404_sin_modificarse(self):
        ajena = self.crear("De A")
        self.como(USUARIO_B)
        self.assertEqual(self.cliente.get(API).json()["listas"], [])
        for metodo, cuerpo in (("GET", None), ("PUT", {"nombre": "robada"}), ("DELETE", None)):
            respuesta = self.cliente.request(metodo, f"{API}/{ajena['id']}", json=cuerpo)
            self.assertEqual(respuesta.status_code, 404, metodo)
            self.assertEqual(respuesta.json(), {"estado": 404, "mensaje": "La lista no existe."})
        self.assertEqual(self.repo.filas[ajena["id"]]["nombre"], "De A")

    def test_id_mal_formado_es_404(self):
        for ruta in ("no-es-uuid", "1", "' OR 1=1 --"):
            self.assertEqual(self.cliente.get(f"{API}/{ruta}").status_code, 404, ruta)

    def test_validacion_con_envoltorio_y_sin_eco_del_valor(self):
        for cuerpo in ({}, {"nombre": ""}, {"nombre": "   "}, {"nombre": "x" * 151}, {"nombre": "ok", "usuario_id": USUARIO_B}, {"nombre": 5}):
            respuesta = self.cliente.post(API, json=cuerpo)
            self.assertEqual(respuesta.status_code, 400, cuerpo)
            self.assertEqual((respuesta.json()["estado"], respuesta.json()["mensaje"]), (400, "Solicitud inválida."))
            self.assertNotIn(USUARIO_B, respuesta.text)
        lista = self.crear()
        for cuerpo in ({}, {"estado": "borrada"}, {"nombre": None, "estado": None}):
            self.assertEqual(self.cliente.put(f"{API}/{lista['id']}", json=cuerpo).status_code, 400, cuerpo)

    def test_json_mal_formado_usa_envoltorio(self):
        respuesta = self.cliente.post(API, content=b'{"nombre": "s\xe1bado"}', headers={"Content-Type": "application/json"})
        self.assertEqual(respuesta.status_code, 400)
        self.assertEqual(respuesta.json()["mensaje"], "Solicitud inválida.")

    def test_nombre_sin_caracteres_de_control(self):
        self.assertEqual(self.crear("Feria\u0000​ del\tsábado")["nombre"], "Feria del sábado")

    def test_tope_de_listas_por_usuario(self):
        for i in range(100):
            self.repo.filas[str(i)] = {"id": str(i), "usuario_id": USUARIO_A}
        respuesta = self.cliente.post(API, json={"nombre": "una más"})
        self.assertEqual(respuesta.status_code, 409)
        self.como(USUARIO_B)
        self.crear("otro usuario no se ve afectado")

    def test_rutas_y_metodos_desconocidos_usan_envoltorio(self):
        self.assertEqual(self.cliente.get("/api/v1/otra").json()["estado"], 404)
        respuesta = self.cliente.patch(API, json={})
        self.assertEqual((respuesta.status_code, respuesta.json()["estado"]), (405, 405))


class AutenticacionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.cliente = TestClient(create_app(CONFIG, RepositorioMemoria()))
        self.cliente.__enter__()
        self.addCleanup(self.cliente.__exit__, None, None, None)

    def estado_con(self, valor: str | None) -> int:
        self.cliente.cookies.clear()
        if valor is not None:
            self.cliente.cookies.set("jwt", valor)
        return self.cliente.get(API).status_code

    def test_token_valido(self):
        self.assertEqual(self.estado_con(token()), 200)

    def test_rechaza_tokens_inseguros(self):
        casos = {
            "sin cookie": None,
            "firma ajena": token(secreto="otra-clave-de-al-menos-treinta-y-dos-bytes"),
            "algoritmo distinto": token(algoritmo="HS512"),
            "alg none": token(secreto=None, algoritmo="none"),
            "expirado": token(exp=1),
            "sin expiración": token(exp=None),
            "nbf futuro": token(nbf=2**31),
            "sin rol": token(rol=None),
            "sin provider": token(provider=None),
            "user_id no uuid": token(usuario_id="admin"),
            "demasiado largo": token(relleno="x" * 5000),
        }
        for nombre, valor in casos.items():
            self.assertEqual(self.estado_con(valor), 401, nombre)

    def test_ignora_identidad_en_headers(self):
        self.cliente.cookies.clear()
        respuesta = self.cliente.get(API, headers={"X-User-ID": USUARIO_A, "Authorization": "Bearer " + token()})
        self.assertEqual(respuesta.status_code, 401)
        self.assertEqual(respuesta.json()["mensaje"], "Cookie de autenticación requerida.")


class InfraestructuraHTTPTest(unittest.TestCase):
    def setUp(self) -> None:
        self.cliente = TestClient(create_app(CONFIG, RepositorioMemoria()))
        self.cliente.__enter__()
        self.addCleanup(self.cliente.__exit__, None, None, None)
        self.cliente.cookies.set("jwt", token())

    def test_cors_lista_blanca(self):
        permitido = self.cliente.get(API, headers={"Origin": ORIGEN})
        self.assertEqual(permitido.headers["access-control-allow-origin"], ORIGEN)
        self.assertEqual(permitido.headers["access-control-allow-credentials"], "true")
        self.assertEqual(self.cliente.get(API, headers={"Origin": "https://ajeno.example"}).status_code, 403)
        preflight = self.cliente.options(API, headers={"Origin": ORIGEN, "Access-Control-Request-Method": "DELETE"})
        self.assertEqual(preflight.status_code, 204)
        self.assertIn("DELETE", preflight.headers["access-control-allow-methods"])

    def test_request_id_se_propaga_o_genera(self):
        self.assertEqual(self.cliente.get(API, headers={"X-Request-ID": "abc-123"}).headers["x-request-id"], "abc-123")
        generado = self.cliente.get(API, headers={"X-Request-ID": "malo id;<script>"}).headers["x-request-id"]
        self.assertRegex(generado, r"^[0-9a-f]{32}$")

    def test_probes(self):
        self.assertEqual(self.cliente.get("/_rutas/live").json()["status"], "ok")
        listo = self.cliente.get("/_rutas/ready")
        self.assertEqual((listo.status_code, listo.json()["checks"]["database"]), (200, "ok"))


if __name__ == "__main__":
    unittest.main()
