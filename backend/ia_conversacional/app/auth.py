"""
Autenticación y validación de tokens JWT emitidos por el servicio de Identidad.
Valida tokens usando algoritmo HS256 y extrae los claims del usuario.
"""

from typing import Optional
import jwt
from fastapi import Cookie, Header, HTTPException, status
from pydantic import BaseModel

from app import config


class UsuarioAutenticado(BaseModel):
    user_id: str
    rol: str
    provider: Optional[str] = None


def _extraer_bearer(authorization: Optional[str]) -> Optional[str]:
    if not authorization:
        return None
    partes = authorization.strip().split()
    if len(partes) == 2 and partes[0].lower() == "bearer":
        return partes[1]
    return None


async def obtener_usuario_actual(
    jwt_cookie: Optional[str] = Cookie(None, alias="jwt"),
    authorization: Optional[str] = Header(None, alias="Authorization"),
) -> UsuarioAutenticado:
    """
    Dependency de FastAPI para validar la sesión del usuario.
    Soporta primero la cookie 'jwt' (emitida como HttpOnly por Identidad)
    y como fallback el header 'Authorization: Bearer <token>'.
    Exige expiración obligatoria (exp) y claim user_id con firma HS256.
    """
    token = jwt_cookie or _extraer_bearer(authorization)

    if not token:
        if authorization and not _extraer_bearer(authorization):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Header Authorization debe tener formato 'Bearer <token>'",
                headers={"WWW-Authenticate": "Bearer"},
            )
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Autenticación requerida: falta cookie 'jwt' o header 'Authorization'",
            headers={"WWW-Authenticate": "Bearer"},
        )

    secret = config.JWT_SECRET
    if not secret or len(secret) < 32:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Clave de verificación JWT no configurada o insegura en el servidor",
            headers={"WWW-Authenticate": "Bearer"},
        )

    try:
        payload = jwt.decode(
            token,
            secret,
            algorithms=["HS256"],
            options={"require": ["exp", "user_id"]},
        )
    except jwt.ExpiredSignatureError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token expirado",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.MissingRequiredClaimError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Token inválido: falta claim requerido ({exc})",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.InvalidTokenError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token inválido o firma incorrecta",
            headers={"WWW-Authenticate": "Bearer"},
        )

    user_id = payload.get("user_id")
    rol = payload.get("rol", "")
    provider = payload.get("provider")

    return UsuarioAutenticado(
        user_id=str(user_id),
        rol=str(rol),
        provider=str(provider) if provider else None,
    )
