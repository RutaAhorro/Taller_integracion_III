#!/usr/bin/env python3
"""Revisa, prepara o aprovisiona la primera habilitación de Motor/Listas.

No despliega servicios ni modifica el Gateway. ``revisar`` y ``preparar`` solo
leen el clúster. ``aprovisionar`` requiere confirmaciones explícitas y modifica
únicamente los Secrets y el rol de Motor. Nunca muestra credenciales ni salidas
crudas de kubectl/psql; no usar con trazas de depuración.
"""

# ===== Importaciones y constantes =====

import argparse
import base64
import binascii
import json
import os
import re
import secrets
import subprocess
import sys
import tempfile
from decimal import Decimal, InvalidOperation
from pathlib import Path
from urllib.parse import quote

RAIZ = Path(__file__).resolve().parent.parent
OVERLAY = RAIZ / "infrastructure/k8s/overlays/motor-listas-ajustado"
CPU_SOLICITADA = Decimal("0.05")
CPU_LIMITE = Decimal("0.1")
MEMORIA_SOLICITADA = Decimal(32 * 1024**2)
MEMORIA_LIMITE = Decimal(128 * 1024**2)
PLAZO_COMANDO = 45
PATRON_IMAGEN = re.compile(
    r"ghcr\.io/[a-z0-9_.-]+/[a-z0-9_.-]+-motor-rutas@sha256:[a-f0-9]{64}"
)

# Los OID NULL permiten informar objetos ausentes sin ejecutar migraciones.
SQL_REVISION = """
SELECT json_build_object(
  'base', current_database(),
  'rol_existe', EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rutas'),
  'rol_login', COALESCE((SELECT rolcanlogin FROM pg_roles WHERE rolname = 'app_rutas'), false),
  'rol_limitado', COALESCE((SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole
    AND NOT rolreplication AND NOT rolbypassrls FROM pg_roles WHERE rolname = 'app_rutas'), false),
  'auditoria_adicional', COALESCE(current_setting('pgaudit.log', true) NOT IN ('', 'none'), false),
  'tablas', to_regclass('rutas.listas_compras') IS NOT NULL
    AND to_regclass('rutas.articulos_lista_compras') IS NOT NULL
    AND to_regclass('rutas.lotes_articulos_aplicados') IS NOT NULL,
  'producto_id', EXISTS (SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'rutas' AND table_name = 'articulos_lista_compras' AND column_name = 'producto_id'),
  'indice', EXISTS (SELECT 1 FROM pg_indexes
    WHERE schemaname = 'rutas' AND indexname = 'uq_articulos_lista_producto'),
  'permisos_base', COALESCE((SELECT
    has_schema_privilege(oid, 'rutas', 'USAGE')
    AND has_sequence_privilege(oid, to_regclass('rutas.articulos_lista_compras_id_seq'), 'USAGE')
    AND (SELECT bool_and(has_table_privilege(oid, to_regclass(tabla), permiso))
      FROM (VALUES ('rutas.listas_compras'), ('rutas.articulos_lista_compras')) AS tablas(tabla)
      CROSS JOIN (VALUES ('SELECT'), ('INSERT'), ('UPDATE'), ('DELETE')) AS permisos(permiso))
    FROM pg_roles WHERE rolname = 'app_rutas'), false),
  'permisos_lotes', COALESCE((SELECT
    has_table_privilege(oid, to_regclass('rutas.lotes_articulos_aplicados'), 'SELECT')
    AND has_table_privilege(oid, to_regclass('rutas.lotes_articulos_aplicados'), 'INSERT')
    FROM pg_roles WHERE rolname = 'app_rutas'), false)
);
"""


# ===== Acceso al clúster sin salida sensible =====


class ErrorHabilitacion(Exception):
    """Fallo operativo con un mensaje apto para mostrar sin secretos."""


def ejecutar(argumentos: list[str], entrada: str | None = None) -> str:
    """Captura las salidas en memoria; un fallo no imprime comandos ni datos recibidos."""
    try:
        resultado = subprocess.run(
            argumentos,
            input=entrada,
            text=True,
            capture_output=True,
            check=False,
            timeout=PLAZO_COMANDO,
        )
    except (OSError, subprocess.TimeoutExpired) as error_obtenido:
        raise ErrorHabilitacion(
            "No se pudo ejecutar una comprobación; revisar herramientas y conectividad."
        ) from error_obtenido
    if resultado.returncode:
        raise ErrorHabilitacion(
            "Falló una operación de kubectl/psql. Se omitió su salida para no revelar credenciales. "
            "Revisar permisos y estado antes de reintentar."
        )
    return resultado.stdout


class Cluster:
    """Fija contexto y namespace en cada operación para no depender de cambios del contexto global."""

    def __init__(self, contexto: str, namespace: str):
        """Guarda únicamente el destino, no credenciales locales."""
        self.contexto = contexto
        self.namespace = namespace

    def comando(self, *argumentos: str, entrada: str | None = None) -> str:
        """Ejecuta kubectl sin incluir credenciales en sus argumentos."""
        return ejecutar(
            [
                "kubectl",
                "--context",
                self.contexto,
                "--namespace",
                self.namespace,
                *argumentos,
            ],
            entrada,
        )

    def objeto(self, recurso: str, nombre: str | None = None) -> dict:
        """Lee JSON en memoria; los recursos ausentes devuelven un objeto vacío."""
        argumentos = ["get", recurso]
        if nombre:
            argumentos.extend([nombre, "--ignore-not-found"])
        salida = self.comando(*argumentos, "-o", "json")
        try:
            return json.loads(salida) if salida.strip() else {}
        except json.JSONDecodeError as error_obtenido:
            raise ErrorHabilitacion(
                "El clúster devolvió un contrato JSON inválido."
            ) from error_obtenido

    def sql(self, consulta: str, escritura: bool = False) -> str:
        """Envía SQL por stdin, nunca por argv; lectura es el modo predeterminado."""
        opciones = (
            "" if escritura else 'PGOPTIONS="-c default_transaction_read_only=on" '
        )
        return self.comando(
            "exec",
            "-i",
            "deployment/db",
            "--",
            "sh",
            "-c",
            opciones
            + 'psql -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
            entrada=consulta,
        )


# ===== Cantidades y plan de recursos =====


def cantidad(valor: str) -> Decimal:
    """Convierte cantidades de quota SI/binarias sin redondear CPU millicores hacia cero."""
    coincidencia = re.fullmatch(r"([0-9]+(?:\.[0-9]+)?)([a-zA-Z]*)", str(valor))
    if not coincidencia:
        raise ErrorHabilitacion("Formato de cantidad de quota no admitido.")
    factores = {
        "": 1,
        "m": Decimal("0.001"),
        "u": Decimal("0.000001"),
        "n": Decimal("0.000000001"),
        "k": 1000,
        "M": 1000**2,
        "G": 1000**3,
        "T": 1000**4,
        "Ki": 1024,
        "Mi": 1024**2,
        "Gi": 1024**3,
        "Ti": 1024**4,
    }
    try:
        return Decimal(coincidencia[1]) * factores[coincidencia[2]]
    except (KeyError, InvalidOperation) as error_obtenido:
        raise ErrorHabilitacion(
            "Unidad de cantidad de quota no admitida."
        ) from error_obtenido


def comprobar_cuotas(cuotas: dict) -> list[str]:
    """Comprueba cada cuota del namespace; no confunde consumo observado con límites asignados."""
    incrementos = {
        "requests.cpu": CPU_SOLICITADA,
        "limits.cpu": CPU_LIMITE,
        "requests.memory": MEMORIA_SOLICITADA,
        "limits.memory": MEMORIA_LIMITE,
        "pods": Decimal(1),
        "services": Decimal(1),
    }
    bloqueos = []
    for cuota in cuotas.get("items", []):
        estado = cuota.get("status", {})
        for recurso, incremento in incrementos.items():
            if recurso not in estado.get("hard", {}):
                continue
            limite = cantidad(estado["hard"][recurso])
            usado = cantidad(estado.get("used", {}).get(recurso, "0"))
            if usado + incremento > limite:
                bloqueos.append(
                    f"La quota {cuota['metadata']['name']} no admite {recurso}."
                )
    return bloqueos


def revisar(cluster: Cluster) -> dict:
    """Recoge solo el estado necesario y devuelve un informe sin valores de Secrets."""
    actual = ejecutar(["kubectl", "config", "current-context"]).strip()
    if actual != cluster.contexto:
        raise ErrorHabilitacion(
            "El contexto activo no coincide con --contexto; confirmar el destino antes de continuar."
        )
    datos_bd = json.loads(cluster.sql(SQL_REVISION))
    cuotas = cluster.objeto("resourcequota")
    # Solo se recibe la salida filtrada de kubectl, no valores de Secrets en Python.
    resumen_secretos = cluster.comando(
        "get",
        "secrets",
        "-o",
        'go-template={{range .items}}{{.metadata.name}}:{{range $clave, $valor := .data}}{{$clave}},{{end}}{{"\\n"}}{{end}}',
    )
    claves = {}
    for linea in resumen_secretos.splitlines():
        nombre, separador, entradas = linea.partition(":")
        if separador:
            claves[nombre] = sorted(filter(None, entradas.split(",")))
    deployment_motor = cluster.objeto("deployment", "motor-rutas")
    servicio_motor = cluster.objeto("service", "motor-rutas")
    bloqueos = comprobar_cuotas(cuotas)
    if deployment_motor or servicio_motor:
        bloqueos.append(
            "Motor/Listas ya tiene recursos: este asistente es solo para su primera habilitación."
        )
    if cluster.objeto("hpa", "motor-rutas-hpa"):
        bloqueos.append(
            "Existe un HPA de Motor: coordinar su presupuesto antes de continuar."
        )
    requisitos_bd = (
        "rol_existe",
        "rol_limitado",
        "tablas",
        "producto_id",
        "indice",
        "permisos_base",
    )
    for requisito in requisitos_bd:
        if not datos_bd.get(requisito):
            bloqueos.append(
                f"Falta el requisito PostgreSQL {requisito}; no se reparará automáticamente."
            )
    if datos_bd.get("auditoria_adicional"):
        bloqueos.append(
            "PostgreSQL tiene auditoría adicional: revisar el manejo de contraseñas con su operador."
        )
    for nombre, clave in (
        ("db-secret", "JWT_SECRET"),
        ("catalogo-internal-secret", "token"),
        ("ghcr-secret", ".dockerconfigjson"),
    ):
        if clave not in claves.get(nombre, []):
            bloqueos.append(f"Falta la referencia {nombre}/{clave}.")
    if datos_bd.get("rol_login") and "rutas-db-secret" not in claves:
        bloqueos.append(
            "app_rutas ya admite LOGIN sin rutas-db-secret: no se rotará una credencial desconocida."
        )
    catalogo = cluster.objeto("deployment", "catalogo")
    contenedores = (
        catalogo.get("spec", {})
        .get("template", {})
        .get("spec", {})
        .get("containers", [])
    )
    if not any(contenedor.get("name") == "catalogo" for contenedor in contenedores):
        bloqueos.append(
            "No se encontró el contenedor catalogo; revisar el despliegue antes de preparar su parche."
        )
    if catalogo.get("spec", {}).get("strategy", {}).get("type") != "Recreate":
        bloqueos.append(
            "Catálogo no usa Recreate: revisar el margen de cuota durante su rollout."
        )
    gateway = cluster.objeto("deployment", "api-gateway")
    destinos_rutas = [
        variable.get("value")
        for contenedor in gateway.get("spec", {})
        .get("template", {})
        .get("spec", {})
        .get("containers", [])
        for variable in contenedor.get("env", [])
        if variable.get("name") == "RUTAS_URL"
    ]
    if destinos_rutas != ["http://motor-rutas:8001"]:
        bloqueos.append(
            "Confirmar RUTAS_URL del Gateway; este asistente no modifica su enrutamiento."
        )
    pendientes = []
    if not datos_bd.get("rol_login"):
        pendientes.append("Habilitar LOGIN y contraseña exclusiva de app_rutas.")
    if not datos_bd.get("permisos_lotes"):
        pendientes.append("Reconciliar SELECT/INSERT de la tabla de lotes.")
    if "rutas-db-secret" not in claves:
        pendientes.append("Crear rutas-db-secret.")
    if "consulta-token" not in claves.get("catalogo-internal-secret", []):
        pendientes.append("Agregar consulta-token sin reemplazar token de ingesta.")
    return {
        "contexto": cluster.contexto,
        "namespace": cluster.namespace,
        "propuesta": {
            "replicas": 1,
            "cpu_request": "50m",
            "cpu_limit": "100m",
            "memoria_request": "32Mi",
            "memoria_limit": "128Mi",
            "hpa": False,
        },
        "postgresql": datos_bd,
        "claves_secrets": claves,
        "cuotas": [
            {"nombre": cuota["metadata"]["name"], "estado": cuota.get("status", {})}
            for cuota in cuotas.get("items", [])
        ],
        "catalogo_resource_version": catalogo.get("metadata", {}).get(
            "resourceVersion"
        ),
        "pendientes_aprovisionamiento": pendientes,
        "bloqueos": bloqueos,
    }


# ===== Preparación de manifiestos sin Secrets =====


def preparar(imagen: str, destino: Path, namespace: str, version_catalogo: str) -> None:
    """Genera solo ConfigMap, Service y Deployment; exige imagen inmutable y no sobrescribe archivos."""
    if not PATRON_IMAGEN.fullmatch(imagen):
        raise ErrorHabilitacion(
            "Indicar una imagen de Motor confirmada en GHCR con @sha256 y digest de 64 caracteres."
        )
    ruta_parche = destino.with_name(destino.stem + ".catalogo-patch.json")
    if destino.exists() or ruta_parche.exists():
        raise ErrorHabilitacion(
            "El archivo de salida ya existe; usar otro nombre para preservar el plan anterior."
        )
    destino.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(
        prefix=".motor-listas-", dir=destino.parent
    ) as temporal:
        configuracion = {
            "apiVersion": "kustomize.config.k8s.io/v1beta1",
            "kind": "Kustomization",
            "namespace": namespace,
            "resources": [os.path.relpath(OVERLAY, temporal)],
            "patches": [
                {
                    "patch": json.dumps(
                        {
                            "apiVersion": "apps/v1",
                            "kind": "Deployment",
                            "metadata": {"name": "motor-rutas"},
                            "spec": {
                                "template": {
                                    "spec": {
                                        "containers": [
                                            {"name": "motor-rutas", "image": imagen}
                                        ]
                                    }
                                }
                            },
                        }
                    ),
                }
            ],
        }
        Path(temporal, "kustomization.yaml").write_text(
            json.dumps(configuracion), encoding="utf-8"
        )
        manifiesto = ejecutar(["kubectl", "kustomize", temporal])
    with destino.open("x", encoding="utf-8") as archivo:
        archivo.write(manifiesto)
    # No reemplaza el Deployment completo ni su imagen. Una versión desactualizada
    # falla por conflicto y exige repetir la revisión junto con el operador.
    parche = {
        "metadata": {"resourceVersion": version_catalogo},
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "name": "catalogo",
                            "env": [
                                {
                                    "name": "CATALOGO_CONSULTA_TOKEN",
                                    "value": None,
                                    "valueFrom": {
                                        "secretKeyRef": {
                                            "name": "catalogo-internal-secret",
                                            "key": "consulta-token",
                                            "optional": False,
                                        }
                                    },
                                }
                            ],
                        }
                    ]
                }
            }
        },
    }
    with ruta_parche.open("x", encoding="utf-8") as archivo:
        json.dump(parche, archivo, ensure_ascii=False, indent=2)
        archivo.write("\n")


# ===== Aprovisionamiento explícito, sin despliegue =====


def leer_clave(secreto: dict, clave: str) -> str:
    """Decodifica una clave únicamente en memoria; nunca incluye su contenido en errores."""
    try:
        return base64.b64decode(secreto["data"][clave], validate=True).decode("utf-8")
    except (KeyError, ValueError, UnicodeDecodeError, binascii.Error) as error_obtenido:
        raise ErrorHabilitacion(
            "Falta una clave requerida o su codificación es inválida."
        ) from error_obtenido


def agregar_clave(cluster: Cluster, secreto: dict, clave: str, valor: str) -> None:
    """Agrega una clave con control de versión, sin reemplazar tokens ni otras entradas del Secret."""
    if clave in secreto.get("data", {}):
        raise ErrorHabilitacion(
            "La clave ya existe; se preserva y debe revisarse antes de modificarla."
        )
    parche = [
        {
            "op": "test",
            "path": "/metadata/resourceVersion",
            "value": secreto["metadata"]["resourceVersion"],
        },
        {
            "op": "add",
            "path": f"/data/{clave}",
            "value": base64.b64encode(valor.encode()).decode(),
        },
    ]
    cluster.comando(
        "patch",
        "secret",
        secreto["metadata"]["name"],
        "--type=json",
        "--patch-file=/dev/stdin",
        entrada=json.dumps(parche),
    )


def aprovisionar(cluster: Cluster, informe: dict) -> None:
    """Habilita solo app_rutas y lectura de lotes; preserva credenciales existentes y no crea tablas."""
    if informe["bloqueos"]:
        raise ErrorHabilitacion(
            "Resolver los bloqueos del informe antes de aprovisionar."
        )
    secreto_catalogo = cluster.objeto("secret", "catalogo-internal-secret")
    secreto_jwt = cluster.objeto("secret", "db-secret")
    secreto_rutas = cluster.objeto("secret", "rutas-db-secret")
    jwt = leer_clave(secreto_jwt, "JWT_SECRET")
    token_ingesta = leer_clave(secreto_catalogo, "token")
    token_consulta = (
        leer_clave(secreto_catalogo, "consulta-token")
        if "consulta-token" in secreto_catalogo.get("data", {})
        else secrets.token_hex(32)
    )
    if (
        len(token_consulta) < 32
        or any(caracter.isspace() for caracter in token_consulta)
        or token_consulta in (jwt, token_ingesta)
    ):
        raise ErrorHabilitacion(
            "El token de consulta existente no cumple el contrato; no se rotará automáticamente."
        )
    contrasena = (
        leer_clave(secreto_rutas, "DB_PASSWORD")
        if secreto_rutas
        else secrets.token_hex(32)
    )
    if not re.fullmatch(r"[a-zA-Z0-9]{32,}", contrasena) or contrasena in (
        jwt,
        token_ingesta,
        token_consulta,
    ):
        raise ErrorHabilitacion(
            "La contraseña de Motor no cumple el formato seguro esperado; no se rotará automáticamente."
        )
    nombre_bd = informe["postgresql"]["base"]
    dsn = f"postgres://app_rutas:{contrasena}@db:5432/{quote(nombre_bd, safe='')}?sslmode=disable"
    if secreto_rutas and leer_clave(secreto_rutas, "DB_URL") != dsn:
        raise ErrorHabilitacion(
            "DB_URL y DB_PASSWORD no coinciden con el destino esperado; revisar sin rotar credenciales."
        )

    # Persistir primero permite reanudar una interrupción sin generar otra contraseña.
    if not secreto_rutas:
        cluster.comando(
            "create",
            "-f",
            "-",
            entrada=json.dumps(
                {
                    "apiVersion": "v1",
                    "kind": "Secret",
                    "metadata": {
                        "name": "rutas-db-secret",
                        "namespace": cluster.namespace,
                    },
                    "type": "Opaque",
                    "stringData": {"DB_URL": dsn, "DB_PASSWORD": contrasena},
                }
            ),
        )
    if "consulta-token" not in secreto_catalogo.get("data", {}):
        agregar_clave(cluster, secreto_catalogo, "consulta-token", token_consulta)

    # Solo la primera habilitación de LOGIN asigna contraseña. No se rota un rol activo.
    alterar = (
        f"ALTER ROLE app_rutas LOGIN PASSWORD '{contrasena}';"
        if not informe["postgresql"]["rol_login"]
        else ""
    )
    cluster.sql(
        # Suprimir el registro de sentencias sensibles antes de transmitir la contraseña.
        # Un PostgreSQL con auditoría adicional requiere revisión manual previa del operador.
        "SET log_statement = 'none'; SET log_min_duration_statement = -1; "
        "SET log_min_duration_sample = -1; SET log_transaction_sample_rate = 0; "
        "SET log_min_error_statement = 'panic'; SET log_duration = off;\n"
        "BEGIN;\nSET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '15s';\n"
        "SET LOCAL password_encryption = 'scram-sha-256';\n"
        + alterar
        + "\nGRANT SELECT, INSERT ON rutas.lotes_articulos_aplicados TO app_rutas;\nCOMMIT;\n",
        escritura=True,
    )
    # Probar contraseña por TCP evita el falso positivo de una conexión local trust.
    cluster.comando(
        "exec",
        "-i",
        "deployment/db",
        "--",
        "sh",
        entrada=(
            "set +x\n"
            f"export PGPASSWORD='{contrasena}'\n"
            'export PGOPTIONS="-c default_transaction_read_only=on"\n'
            'psql -X -w -qAt -h 127.0.0.1 -U app_rutas -d "$POSTGRES_DB" -c "SELECT 1"\n'
        ),
    )


# ===== Interfaz de operador =====


def principal() -> int:
    """Solicita destino y confirmaciones antes de toda escritura; nunca ejecuta kubectl apply."""
    analizador = argparse.ArgumentParser(description=__doc__)
    analizador.add_argument("modo", choices=("revisar", "preparar", "aprovisionar"))
    analizador.add_argument("--contexto", required=True)
    analizador.add_argument("--namespace", required=True)
    analizador.add_argument("--imagen", default="")
    analizador.add_argument("--salida", type=Path)
    analizador.add_argument("--confirmar-cambios", action="store_true")
    analizador.add_argument("--respaldo-confirmado", action="store_true")
    argumentos = analizador.parse_args()
    try:
        if argumentos.modo == "aprovisionar" and not (
            argumentos.confirmar_cambios and argumentos.respaldo_confirmado
        ):
            raise ErrorHabilitacion(
                "Aprovisionar requiere --confirmar-cambios y --respaldo-confirmado, además de coordinar con el operador."
            )
        if argumentos.modo == "preparar" and not argumentos.salida:
            raise ErrorHabilitacion(
                "Preparar requiere --salida y una imagen por digest confirmada por el operador."
            )
        cluster = Cluster(argumentos.contexto, argumentos.namespace)
        informe = revisar(cluster)
        print(json.dumps(informe, ensure_ascii=False, indent=2))
        if informe["bloqueos"]:
            return 1
        if argumentos.modo == "preparar":
            preparar(
                argumentos.imagen,
                argumentos.salida,
                argumentos.namespace,
                informe["catalogo_resource_version"],
            )
            print("Manifiesto no sensible generado. No se aplicó ningún recurso.")
        elif argumentos.modo == "aprovisionar":
            aprovisionar(cluster, informe)
            print(
                "Credenciales y permisos preparados; conexión de app_rutas comprobada por TCP. No se desplegó ni reinició ningún servicio."
            )
        return 0
    except ErrorHabilitacion as error_obtenido:
        print(str(error_obtenido), file=sys.stderr)
        return 1
    except (OSError, ValueError):
        # No mostrar traceback ni excepciones de bibliotecas que puedan contener secretos.
        print(
            "No se completó la operación. Revisar requisitos y destino; no se muestran salidas sensibles. Si empezó el aprovisionamiento, conservar los Secrets y revisar el estado antes de reintentar.",
            file=sys.stderr,
        )
        return 1


if __name__ == "__main__":
    sys.exit(principal())
