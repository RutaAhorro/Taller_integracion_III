"""Configuración del motor de rutas. Falla al arrancar antes que servir sin aislamiento."""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass
from urllib.parse import urlsplit


class ErrorConfiguracion(ValueError):
    """Error de arranque: nunca incluye el DSN ni secretos."""


@dataclass(frozen=True)
class Configuracion:
    db_url: str
    jwt_secret: str
    origenes_cors: tuple[str, ...] = ()
    db_pool_max: int = 5


def cargar(entorno: Mapping[str, str] | None = None) -> Configuracion:
    entorno = os.environ if entorno is None else entorno
    db_url = entorno.get("RUTAS_DB_URL", "")
    try:
        dsn = urlsplit(db_url)
        _ = dsn.port  # urlsplit sólo valida el puerto al leerlo
        valido = dsn.scheme in ("postgres", "postgresql") and dsn.username == "app_rutas" and bool(dsn.password) and bool(dsn.hostname) and dsn.path not in ("", "/")
    except ValueError:
        valido = False
    if not valido:
        raise ErrorConfiguracion("RUTAS_DB_URL requiere rol app_rutas, base y contraseña explícitos")

    secreto = entorno.get("JWT_SECRET", "")
    if len(secreto.encode()) < 32 or secreto.strip() != secreto:
        raise ErrorConfiguracion("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")

    origenes = []
    for origen in (parte.strip() for parte in entorno.get("CORS_ALLOWED_ORIGINS", "").split(",")):
        if not origen:
            continue
        url = urlsplit(origen)
        if url.scheme not in ("http", "https") or not url.hostname or url.username or url.password or url.path or url.query or url.fragment or any(c in origen for c in "*?#\\ \t\r\n"):
            raise ErrorConfiguracion("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
        origenes.append(origen)

    crudo = entorno.get("RUTAS_DB_POOL_MAX", "5")
    if not crudo.isdigit() or not 1 <= int(crudo) <= 20:
        raise ErrorConfiguracion("RUTAS_DB_POOL_MAX debe estar entre 1 y 20")
    return Configuracion(db_url=db_url, jwt_secret=secreto, origenes_cors=tuple(origenes), db_pool_max=int(crudo))
