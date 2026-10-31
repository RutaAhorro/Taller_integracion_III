"""Valida el JWT de Identidad (HS256, cookie HttpOnly `jwt`). No emite tokens ni consulta usuarios."""

from __future__ import annotations

import uuid
from dataclasses import dataclass

import jwt
from fastapi import Request

from app.errores import ErrorAPI

LARGO_MAXIMO_TOKEN = 4096


@dataclass(frozen=True)
class Sesion:
    usuario_id: str
    rol: str
    proveedor: str


def validar_token(token: str, secreto: str) -> Sesion:
    if not token or len(token) > LARGO_MAXIMO_TOKEN:
        raise ErrorAPI(401, "Token inválido o expirado.")
    try:
        claims = jwt.decode(token, secreto, algorithms=["HS256"], options={"require": ["exp"]})
    except jwt.PyJWTError:
        raise ErrorAPI(401, "Token inválido o expirado.") from None
    usuario_id, rol, proveedor = claims.get("user_id"), claims.get("rol"), claims.get("provider")
    if not all(isinstance(valor, str) and valor.strip() for valor in (usuario_id, rol, proveedor)):
        raise ErrorAPI(401, "Token inválido o expirado.")
    try:
        usuario_id = str(uuid.UUID(usuario_id))
    except ValueError:
        raise ErrorAPI(401, "Token inválido o expirado.") from None
    return Sesion(usuario_id=usuario_id, rol=rol, proveedor=proveedor)


def sesion_actual(request: Request) -> Sesion:
    """Dependencia de FastAPI: el usuario siempre sale del JWT, nunca de headers ni del cuerpo."""
    token = request.cookies.get("jwt", "").strip()
    if not token:
        raise ErrorAPI(401, "Cookie de autenticación requerida.")
    return validar_token(token, request.app.state.config.jwt_secret)
