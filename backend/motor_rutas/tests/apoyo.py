"""Utilidades de prueba: repositorio en memoria y JWT firmados como los emite Identidad."""

from __future__ import annotations

import time
import uuid
from datetime import datetime, timezone

import jwt

from app.config import Configuracion
from app.listas.modelos import Articulo, Lista, ListaDetalle

SECRETO = "sup240-solo-pruebas-clave-de-al-menos-32-bytes"
ORIGEN = "https://frontend.example"
USUARIO_A = "11111111-1111-4111-8111-111111111111"
USUARIO_B = "22222222-2222-4222-8222-222222222222"
CONFIG = Configuracion(db_url="postgres://app_rutas:x@db/test", jwt_secret=SECRETO, origenes_cors=(ORIGEN,))


def token(usuario_id: str | None = USUARIO_A, secreto: str = SECRETO, algoritmo: str = "HS256", **cambios) -> str:
    ahora = int(time.time())
    claims = {"user_id": usuario_id, "rol": "usuario", "provider": "local", "iat": ahora, "nbf": ahora, "exp": ahora + 3600}
    claims.update(cambios)
    return jwt.encode({k: v for k, v in claims.items() if v is not None}, secreto, algorithm=algoritmo)


class RepositorioMemoria:
    def __init__(self) -> None:
        self.filas: dict[str, dict] = {}

    def _lista(self, fila: dict) -> Lista:
        return Lista(id=fila["id"], nombre=fila["nombre"], estado=fila["estado"], total_articulos=len(fila["articulos"]),
                     creada_el=fila["creada_el"], actualizada_el=fila["actualizada_el"])

    def _propia(self, usuario_id: str, lista_id: str) -> dict | None:
        fila = self.filas.get(lista_id)
        return fila if fila and fila["usuario_id"] == usuario_id else None

    async def contar(self, usuario_id: str) -> int:
        return sum(1 for fila in self.filas.values() if fila["usuario_id"] == usuario_id)

    async def listar(self, usuario_id: str, limite: int) -> list[Lista]:
        propias = [f for f in self.filas.values() if f["usuario_id"] == usuario_id]
        propias.sort(key=lambda f: f["actualizada_el"], reverse=True)
        return [self._lista(f) for f in propias[:limite]]

    async def crear(self, usuario_id: str, nombre: str) -> Lista:
        ahora = datetime.now(timezone.utc)
        fila = {"id": str(uuid.uuid4()), "usuario_id": usuario_id, "nombre": nombre, "estado": "activa",
                "creada_el": ahora, "actualizada_el": ahora, "articulos": []}
        self.filas[fila["id"]] = fila
        return self._lista(fila)

    async def obtener(self, usuario_id: str, lista_id: str) -> ListaDetalle | None:
        fila = self._propia(usuario_id, lista_id)
        if fila is None:
            return None
        return ListaDetalle(**self._lista(fila).model_dump(), articulos=[Articulo(**a) for a in fila["articulos"]])

    async def actualizar(self, usuario_id: str, lista_id: str, nombre: str | None, estado: str | None) -> Lista | None:
        fila = self._propia(usuario_id, lista_id)
        if fila is None:
            return None
        fila["nombre"] = nombre or fila["nombre"]
        fila["estado"] = estado or fila["estado"]
        fila["actualizada_el"] = datetime.now(timezone.utc)
        return self._lista(fila)

    async def eliminar(self, usuario_id: str, lista_id: str) -> bool:
        return self._propia(usuario_id, lista_id) is not None and self.filas.pop(lista_id) is not None
