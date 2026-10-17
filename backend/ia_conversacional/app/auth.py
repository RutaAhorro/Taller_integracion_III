"""
Autenticación y validación de tokens JWT emitidos por el servicio de Identidad.
Valida tokens usando algoritmo HS256 y extrae los claims del usuario.
"""

from typing import Optional
import jwt
from fastapi import Header, HTTPException, status
from pydantic import BaseModel

from app import config


class UsuarioAutenticado(BaseModel):
    user_id: str
    rol: str
    provider: Optional[str] = None


async def obtener_usuario_actual(
    authorization: Optional[str] = Header(None, alias="Authorization"),
) -> UsuarioAutenticado:
    """
    Dependency de FastAPI para validar el header Authorization: Bearer <token>.
    Valida firma HS256, expiración y presencia de claims.
    """
    if not authorization:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Header Authorization requerido",
            headers={"WWW-Authenticate": "Bearer"},
        )

    partes = authorization.strip().split()
    if len(partes) != 2 or partes[0].lower() != "bearer":
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Header Authorization debe tener formato 'Bearer <token>'",
            headers={"WWW-Authenticate": "Bearer"},
        )

    token = partes[1]
    secret = config.JWT_SECRET
    if not secret:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Clave de verificación JWT no configurada en el servidor",
            headers={"WWW-Authenticate": "Bearer"},
        )

    try:
        payload = jwt.decode(
            token,
            secret,
            algorithms=["HS256"],
        )
    except jwt.ExpiredSignatureError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token expirado",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except jwt.InvalidTokenError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token inválido o firma incorrecta",
            headers={"WWW-Authenticate": "Bearer"},
        )

    user_id = payload.get("user_id")
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="El token no contiene el claim 'user_id'",
            headers={"WWW-Authenticate": "Bearer"},
        )

    rol = payload.get("rol", "")
    provider = payload.get("provider")

    return UsuarioAutenticado(
        user_id=str(user_id),
        rol=str(rol),
        provider=str(provider) if provider else None,
    )
