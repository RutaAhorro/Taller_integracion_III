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


def _dsn_valido(db_url: str) -> bool:
    """Exige el rol app_rutas con contraseña, host y base explícitos."""
    try:
        dsn = urlsplit(db_url)
        _ = dsn.port  # urlsplit sólo valida el puerto al leerlo
    except ValueError:
        return False
    return (
        dsn.scheme in ("postgres", "postgresql")
        and dsn.username == "app_rutas"
        and bool(dsn.password)
        and bool(dsn.hostname)
        and dsn.path not in ("", "/")
    )


def _origen_exacto(origen: str) -> bool:
    """Un origen CORS es esquema + host (+ puerto), sin rutas, comodines ni credenciales."""
    url = urlsplit(origen)
    return (
        url.scheme in ("http", "https")
        and bool(url.hostname)
        and not (url.username or url.password or url.path or url.query or url.fragment)
        and not any(c in origen for c in "*?#\\ \t\r\n")
    )


def cargar(entorno: Mapping[str, str] | None = None) -> Configuracion:
    entorno = os.environ if entorno is None else entorno
    db_url = entorno.get("RUTAS_DB_URL", "")
    if not _dsn_valido(db_url):
        raise ErrorConfiguracion("RUTAS_DB_URL requiere rol app_rutas, base y contraseña explícitos")

    secreto = entorno.get("JWT_SECRET", "")
    if len(secreto.encode()) < 32 or secreto.strip() != secreto:
        raise ErrorConfiguracion("JWT_SECRET requiere al menos 32 bytes y no admite espacios externos")

    origenes = []
    for origen in (parte.strip() for parte in entorno.get("CORS_ALLOWED_ORIGINS", "").split(",")):
        if not origen:
            continue
        if not _origen_exacto(origen):
            raise ErrorConfiguracion("CORS_ALLOWED_ORIGINS requiere orígenes HTTP(S) exactos")
        origenes.append(origen)

    crudo = entorno.get("RUTAS_DB_POOL_MAX", "5")
    if not crudo.isdigit() or not 1 <= int(crudo) <= 20:
        raise ErrorConfiguracion("RUTAS_DB_POOL_MAX debe estar entre 1 y 20")
    return Configuracion(db_url=db_url, jwt_secret=secreto, origenes_cors=tuple(origenes), db_pool_max=int(crudo))
