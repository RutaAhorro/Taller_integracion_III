from pydantic import BaseModel, Field

from app import config


class ChatRequest(BaseModel):
    mensaje: str = Field(
        ...,
        min_length=1,
        max_length=config.CHAT_MAX_MENSAJE_LEN,
        description="Mensaje del usuario",
    )
