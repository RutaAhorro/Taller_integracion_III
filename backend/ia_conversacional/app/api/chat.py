"""
Ruta HTTP `POST /api/v1/chat`.

Valida el cuerpo y la sesión JWT de Identidad, aplica el límite por usuario
y delega el pipeline en `app/services/chat.py`.
"""

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel, Field, field_validator

from app import config
from app.api.sesion import LimitadorSolicitudes, UsuarioAutenticado, obtener_usuario_actual
from app.domain.proveedores import ProveedorIA
from app.repositories.catalogo import ProductoRepositorio
from app.services.chat import AsistenteNoDisponible, procesar_mensaje

# ===== Constantes =====

RUTA_CHAT = "/api/v1/chat"
TEXTO_ASISTENTE_NO_DISPONIBLE = "El asistente no está disponible en este momento. Por favor, intenta en unos minutos."


# ===== Contrato HTTP =====


class PeticionChat(BaseModel):
    """Cuerpo de `POST /api/v1/chat`: el mensaje en lenguaje natural del usuario."""

    mensaje: str = Field(
        ...,
        min_length=1,
        max_length=config.CHAT_MAX_MENSAJE_LEN,
        description="Mensaje del usuario",
    )

    @field_validator("mensaje", mode="before")
    @classmethod
    def _validar_mensaje_no_solo_espacios(cls, valor: object) -> str:
        """Recorta espacios externos y rechaza mensajes vacíos sin convertir otros tipos a texto."""
        if not isinstance(valor, str):
            raise ValueError("El mensaje debe ser una cadena de texto")
        texto_limpio = valor.strip()
        if not texto_limpio:
            raise ValueError("El mensaje no puede ser solo espacios en blanco")
        return texto_limpio


# ===== Router =====


def crear_router_chat(
    repositorio: ProductoRepositorio,
    proveedor: ProveedorIA,
    limitador: LimitadorSolicitudes | None = None,
) -> APIRouter:
    """Construye el router del chat con sus dependencias inyectadas (permite dobles en las pruebas)."""
    router = APIRouter(tags=["chat"])
    limitador = limitador or LimitadorSolicitudes(
        maximo_solicitudes=config.CHAT_RATE_LIMIT_SOLICITUDES,
        ventana_segundos=config.CHAT_RATE_LIMIT_VENTANA_SEGUNDOS,
    )

    @router.post(RUTA_CHAT)
    async def chatear(
        peticion: PeticionChat,
        usuario: UsuarioAutenticado = Depends(obtener_usuario_actual),
    ):
        """Valida la sesión, aplica el límite por usuario y ejecuta el pipeline (SUP-247)."""
        limitador.verificar(usuario.user_id)
        try:
            return await procesar_mensaje(proveedor, repositorio, peticion.mensaje)
        except AsistenteNoDisponible:
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail=TEXTO_ASISTENTE_NO_DISPONIBLE
            ) from None

    return router
