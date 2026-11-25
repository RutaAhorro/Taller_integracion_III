"""Prueba la preparación y el aprovisionamiento con dobles; nunca escribe en el clúster."""

# ===== Importaciones y carga del script =====

import base64
import importlib.util
import io
import json
import shutil
import subprocess
import tempfile
import unittest
from contextlib import redirect_stderr
from decimal import Decimal
from pathlib import Path
from unittest.mock import Mock, patch

RUTA_SCRIPT = Path(__file__).resolve().parents[1] / "habilitar_motor_listas.py"
ESPECIFICACION = importlib.util.spec_from_file_location(
    "habilitar_motor_listas", RUTA_SCRIPT
)
MODULO = importlib.util.module_from_spec(ESPECIFICACION)
ESPECIFICACION.loader.exec_module(MODULO)
DIRECTORIO_TEMPORAL = "/tmp/opencode" if Path("/tmp/opencode").is_dir() else None
IMAGEN_PRUEBA = "ghcr.io/ejemplo/proyecto-motor-rutas@sha256:" + "a" * 64


def secreto(nombre, valores):
    """Codifica solo fixtures de prueba, nunca credenciales del entorno."""
    return {
        "metadata": {"name": nombre, "resourceVersion": "1"},
        "data": {
            clave: base64.b64encode(valor.encode()).decode()
            for clave, valor in valores.items()
        },
    }


class ClusterFalso:
    """Registra operaciones sin ejecutar kubectl ni conectar a PostgreSQL."""

    def __init__(self, secretos=None):
        """Prepara los dos Secrets previos a la primera habilitación."""
        self.contexto = "prueba"
        self.namespace = "aislado"
        self.secretos = secretos or {
            "db-secret": secreto("db-secret", {"JWT_SECRET": "j" * 64}),
            "catalogo-internal-secret": secreto(
                "catalogo-internal-secret", {"token": "i" * 64}
            ),
        }
        self.llamadas = []
        self.consultas = []

    def objeto(self, recurso, nombre):
        """Lee únicamente el diccionario de fixtures."""
        return self.secretos.get(nombre, {})

    def comando(self, *argumentos, entrada=None):
        """Guarda el payload para comprobar su forma sin emitir su contenido."""
        self.llamadas.append((argumentos, entrada))
        return ""

    def sql(self, consulta, escritura=False):
        """Registra la transacción propuesta, sin ejecutarla."""
        self.consultas.append((consulta, escritura))
        return ""


class PruebasHabilitacion(unittest.TestCase):
    """Comprueba límites, confidencialidad, guardas e idempotencia del aprovisionamiento."""

    def informe(self, login=False):
        """Devuelve los requisitos de una base de prueba lista salvo LOGIN y lotes."""
        return {"bloqueos": [], "postgresql": {"base": "prueba", "rol_login": login}}

    def test_cantidades_no_confunden_gigabytes_y_gibibytes(self):
        """Respeta los valores SI reales de la cuota en vez de asumir Gi."""
        self.assertEqual(MODULO.cantidad("1800m"), Decimal("1.8"))
        self.assertEqual(MODULO.cantidad("2G"), Decimal(2_000_000_000))
        self.assertEqual(MODULO.cantidad("2Gi"), Decimal(2 * 1024**3))
        with self.assertRaises(MODULO.ErrorHabilitacion):
            MODULO.cantidad("sin-limite")

    def test_cuota_de_cpu_admite_100m_pero_no_sobrepasa_el_limite(self):
        """Verifica el incremento de una réplica sin tocar recursos de otros servicios."""
        cuotas = {
            "items": [
                {
                    "metadata": {"name": "prueba"},
                    "status": {
                        "hard": {"limits.cpu": "2", "services": "10"},
                        "used": {"limits.cpu": "1800m", "services": "8"},
                    },
                }
            ]
        }
        self.assertEqual(MODULO.comprobar_cuotas(cuotas), [])
        cuotas["items"][0]["status"]["used"]["limits.cpu"] = "1950m"
        self.assertIn("limits.cpu", MODULO.comprobar_cuotas(cuotas)[0])
        cuotas["items"][0]["status"]["used"]["services"] = "10"
        self.assertTrue(
            any("services" in mensaje for mensaje in MODULO.comprobar_cuotas(cuotas))
        )

    def test_respeta_todas_las_cuotas(self):
        """Una cuota adicional más estricta también bloquea el plan."""
        cuotas = {
            "items": [
                {
                    "metadata": {"name": "amplia"},
                    "status": {"hard": {"pods": "12"}, "used": {"pods": "9"}},
                },
                {
                    "metadata": {"name": "estricta"},
                    "status": {"hard": {"pods": "9"}, "used": {"pods": "9"}},
                },
            ]
        }
        self.assertIn("estricta", MODULO.comprobar_cuotas(cuotas)[0])

    def test_limite_por_contenedor_exige_64mi_y_respeta_maximos(self):
        """La cuota total libre no evita que un LimitRange rechace el pod."""
        rangos = {
            "items": [
                {
                    "metadata": {"name": "prueba"},
                    "spec": {
                        "limits": [
                            {
                                "type": "Container",
                                "min": {"cpu": "25m", "memory": "64Mi"},
                                "max": {"cpu": "500m", "memory": "512Mi"},
                            }
                        ]
                    },
                }
            ]
        }
        self.assertEqual(MODULO.comprobar_limites(rangos), [])
        rangos["items"][0]["spec"]["limits"][0]["min"]["memory"] = "128Mi"
        self.assertTrue(
            any(
                "request de memory" in mensaje
                for mensaje in MODULO.comprobar_limites(rangos)
            )
        )
        rangos["items"][0]["spec"]["limits"][0]["max"]["cpu"] = "50m"
        self.assertTrue(
            any(
                "limit de cpu" in mensaje
                for mensaje in MODULO.comprobar_limites(rangos)
            )
        )

    def test_limite_detecta_proporcion_excesiva(self):
        """Una relación limit/request restringida también puede impedir admisión."""
        rangos = {
            "items": [
                {
                    "metadata": {"name": "proporcion"},
                    "spec": {
                        "limits": [
                            {"type": "Container", "maxLimitRequestRatio": {"cpu": "1"}}
                        ]
                    },
                }
            ]
        }
        self.assertTrue(
            any("proporción" in mensaje for mensaje in MODULO.comprobar_limites(rangos))
        )

    def test_salida_de_subproceso_fallido_no_revela_payload(self):
        """Ni stdout ni stderr del cliente se incluyen en un error operativo."""
        resultado = subprocess.CompletedProcess(
            [], 1, "secreto-stdout", "secreto-stderr"
        )
        with (
            patch.object(MODULO.subprocess, "run", return_value=resultado),
            self.assertRaises(MODULO.ErrorHabilitacion) as capturado,
        ):
            MODULO.ejecutar(["kubectl"], "secreto-stdin")
        self.assertNotIn("secreto", str(capturado.exception))

    def test_no_aprovisiona_sin_confirmaciones(self):
        """La CLI detiene la ejecución antes de consultar el entorno si faltan confirmaciones."""
        salida = io.StringIO()
        with (
            patch(
                "sys.argv",
                [
                    "script",
                    "aprovisionar",
                    "--contexto",
                    "prueba",
                    "--namespace",
                    "aislado",
                ],
            ),
            patch.object(MODULO, "revisar") as revision,
            redirect_stderr(salida),
        ):
            self.assertEqual(MODULO.principal(), 1)
            revision.assert_not_called()
        self.assertIn("--confirmar-cambios", salida.getvalue())

    def test_revision_solo_lee_y_no_procesa_valores_de_secrets(self):
        """El diagnóstico informa claves y pendientes sin recibir valores en su salida."""
        cluster = Mock(spec=MODULO.Cluster)
        cluster.contexto = "prueba"
        cluster.namespace = "aislado"
        cluster.sql.return_value = json.dumps(
            {
                "base": "prueba",
                "rol_existe": True,
                "rol_login": False,
                "rol_limitado": True,
                "tablas": True,
                "producto_id": True,
                "indice": True,
                "permisos_base": True,
                "permisos_lotes": False,
            }
        )
        cluster.comando.return_value = "db-secret:JWT_SECRET,\ncatalogo-internal-secret:token,\nghcr-secret:.dockerconfigjson,\n"
        recursos = {
            ("resourcequota", None): {"items": []},
            ("deployment", "catalogo"): {
                "metadata": {"resourceVersion": "1"},
                "spec": {
                    "strategy": {"type": "Recreate"},
                    "template": {
                        "spec": {"containers": [{"name": "catalogo"}]},
                    },
                },
            },
            ("deployment", "api-gateway"): {
                "spec": {
                    "template": {
                        "spec": {
                            "containers": [
                                {
                                    "env": [
                                        {
                                            "name": "RUTAS_URL",
                                            "value": "http://motor-rutas:8001",
                                        }
                                    ],
                                }
                            ]
                        }
                    }
                }
            },
        }
        cluster.objeto.side_effect = lambda recurso, nombre=None: recursos.get(
            (recurso, nombre), {}
        )
        with patch.object(MODULO, "ejecutar", return_value="prueba"):
            informe = MODULO.revisar(cluster)
        self.assertEqual(informe["bloqueos"], [])
        self.assertEqual(len(informe["pendientes_aprovisionamiento"]), 4)
        cluster.sql.assert_called_once_with(MODULO.SQL_REVISION)
        self.assertNotIn(
            "secrets", [llamada.args[0] for llamada in cluster.objeto.call_args_list]
        )
        self.assertIn("go-template=", " ".join(cluster.comando.call_args.args))
        recursos[("deployment", "motor-rutas")] = {"metadata": {"name": "motor-rutas"}}
        with patch.object(MODULO, "ejecutar", return_value="prueba"):
            self.assertTrue(MODULO.revisar(cluster)["bloqueos"])

    def test_contexto_distinto_impide_incluso_la_consulta_de_bd(self):
        """Una confirmación de destino equivocada no opera sobre el contexto activo."""
        cluster = Mock(spec=MODULO.Cluster)
        cluster.contexto = "prueba"
        with (
            patch.object(MODULO, "ejecutar", return_value="otro"),
            self.assertRaises(MODULO.ErrorHabilitacion),
        ):
            MODULO.revisar(cluster)
        cluster.sql.assert_not_called()

    def test_primera_habilitacion_conserva_ingesta_y_solo_otorga_grants_de_lotes(self):
        """Los Secrets viajan por stdin y no se otorgan permisos fuera del contrato."""
        cluster = ClusterFalso()
        with patch.object(
            MODULO.secrets, "token_hex", side_effect=["c" * 64, "d" * 64]
        ):
            MODULO.aprovisionar(cluster, self.informe())
        creacion = json.loads(cluster.llamadas[0][1])
        self.assertEqual(creacion["stringData"]["DB_PASSWORD"], "d" * 64)
        self.assertEqual(creacion["metadata"]["name"], "rutas-db-secret")
        parche = json.loads(cluster.llamadas[1][1])
        self.assertEqual(parche[0]["path"], "/metadata/resourceVersion")
        self.assertEqual(parche[1]["path"], "/data/consulta-token")
        self.assertNotIn("/data/token", json.dumps(parche))
        self.assertEqual(len(cluster.consultas), 1)
        consulta, escritura = cluster.consultas[0]
        self.assertTrue(escritura)
        self.assertIn("ALTER ROLE app_rutas LOGIN", consulta)
        self.assertIn(
            "GRANT SELECT, INSERT ON rutas.lotes_articulos_aplicados", consulta
        )
        for texto_prohibido in (
            "CREATE TABLE",
            "DROP ",
            "DELETE FROM",
            "app_identity",
            "GRANT ALL",
        ):
            self.assertNotIn(texto_prohibido, consulta)
        for argumentos, _entrada in cluster.llamadas:
            self.assertNotIn("d" * 64, " ".join(argumentos))
            self.assertNotIn("c" * 64, " ".join(argumentos))
        self.assertIn("-h 127.0.0.1", cluster.llamadas[-1][1])

    def test_reintento_reutiliza_secrets_y_no_rota_rol_activo(self):
        """Repetir antes de desplegar no genera otras claves ni cambia una contraseña activa."""
        cluster = ClusterFalso()
        cluster.secretos["catalogo-internal-secret"]["data"]["consulta-token"] = (
            base64.b64encode(b"c" * 64).decode()
        )
        cluster.secretos["rutas-db-secret"] = secreto(
            "rutas-db-secret",
            {
                "DB_PASSWORD": "d" * 64,
                "DB_URL": f"postgres://app_rutas:{'d' * 64}@db:5432/prueba?sslmode=disable",
            },
        )
        with patch.object(MODULO.secrets, "token_hex") as generar:
            MODULO.aprovisionar(cluster, self.informe(login=True))
            generar.assert_not_called()
        self.assertNotIn("ALTER ROLE", cluster.consultas[0][0])
        self.assertEqual(len(cluster.llamadas), 1)  # Solo comprobación TCP.

    def test_contrato_invalido_falla_antes_de_cualquier_escritura(self):
        """No modifica un token de consulta igual al de ingesta."""
        cluster = ClusterFalso()
        cluster.secretos["catalogo-internal-secret"]["data"]["consulta-token"] = (
            base64.b64encode(b"i" * 64).decode()
        )
        with self.assertRaises(MODULO.ErrorHabilitacion):
            MODULO.aprovisionar(cluster, self.informe())
        self.assertEqual(cluster.llamadas, [])
        self.assertEqual(cluster.consultas, [])

    def test_rechaza_imagen_mutable_sin_crear_archivos(self):
        """La imagen por defecto del repositorio no basta para preparar un despliegue real."""
        with tempfile.TemporaryDirectory(dir=DIRECTORIO_TEMPORAL) as temporal:
            destino = Path(temporal, "motor.yaml")
            with self.assertRaises(MODULO.ErrorHabilitacion):
                MODULO.preparar(
                    "ghcr.io/ejemplo/proyecto-motor-rutas:develop",
                    destino,
                    "aislado",
                    "1",
                )
            self.assertFalse(destino.exists())

    @unittest.skipUnless(shutil.which("kubectl"), "kubectl no disponible")
    def test_kustomize_genera_tres_recursos_sin_hpa_y_parche_acotado(self):
        """Renderizar es local: no aplica recursos ni pide credenciales al API."""
        with tempfile.TemporaryDirectory(dir=DIRECTORIO_TEMPORAL) as temporal:
            destino = Path(temporal, "motor.yaml")
            MODULO.preparar(IMAGEN_PRUEBA, destino, "aislado", "version-prueba")
            contenido = destino.read_text()
            self.assertEqual(contenido.count("kind:"), 3)
            self.assertNotIn("HorizontalPodAutoscaler", contenido)
            self.assertNotIn("kind: Secret", contenido)
            self.assertIn("cpu: 100m", contenido)
            self.assertIn("memory: 64Mi", contenido)
            self.assertIn(IMAGEN_PRUEBA, contenido)
            self.assertEqual(contenido.count("namespace: aislado"), 3)
            parche = json.loads(
                destino.with_name("motor.catalogo-patch.json").read_text()
            )
            self.assertEqual(parche["metadata"]["resourceVersion"], "version-prueba")
            contenedor = parche["spec"]["template"]["spec"]["containers"][0]
            self.assertNotIn("image", contenedor)
            self.assertEqual(
                contenedor["env"][0]["valueFrom"]["secretKeyRef"]["key"],
                "consulta-token",
            )
            with self.assertRaises(MODULO.ErrorHabilitacion):
                MODULO.preparar(IMAGEN_PRUEBA, destino, "aislado", "1")


if __name__ == "__main__":
    unittest.main()
