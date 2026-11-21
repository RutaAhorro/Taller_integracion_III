"""Limitación de solicitudes del chat por usuario autenticado mediante ventana deslizante."""

import math
import time
from collections import defaultdict, deque
from threading import Lock
from typing import Callable, Optional

from fastapi import HTTPException, status

from app.config import (
    CHAT_RATE_LIMIT_SOLICITUDES_PREDETERMINADO,
    CHAT_RATE_LIMIT_VENTANA_SEGUNDOS_PREDETERMINADA,
)

# --------------------------------------------------------------------------
# Constantes de Limitación (SUP-336)
# --------------------------------------------------------------------------
MAXIMO_SOLICITUDES_PREDETERMINADO: int = CHAT_RATE_LIMIT_SOLICITUDES_PREDETERMINADO
VENTANA_SEGUNDOS_PREDETERMINADA: float = CHAT_RATE_LIMIT_VENTANA_SEGUNDOS_PREDETERMINADA
INTERVALO_PURGA_PREDETERMINADO: int = 50


# --------------------------------------------------------------------------
# Limitador en Memoria
# --------------------------------------------------------------------------
class LimitadorSolicitudes:
    """Contador de solicitudes en memoria local mediante ventana deslizante, independiente por réplica."""

    def __init__(
        self,
        maximo_solicitudes: int = MAXIMO_SOLICITUDES_PREDETERMINADO,
        ventana_segundos: float = VENTANA_SEGUNDOS_PREDETERMINADA,
        reloj: Optional[Callable[[], float]] = None,
        intervalo_purga: int = INTERVALO_PURGA_PREDETERMINADO,
    ):
        """Inicializa el limitador por ventana deslizante (log de marcas de tiempo)."""
        if maximo_solicitudes <= 0:
            raise ValueError("maximo_solicitudes debe ser mayor a 0")
        if ventana_segundos <= 0 or not math.isfinite(ventana_segundos):
            raise ValueError("ventana_segundos debe ser mayor a 0 y finito")
        if intervalo_purga <= 0:
            raise ValueError("intervalo_purga debe ser mayor a 0")

        self.maximo_solicitudes = maximo_solicitudes
        self.ventana_segundos = ventana_segundos
        self.intervalo_purga = intervalo_purga
        self._reloj = reloj or time.monotonic
        self._solicitudes: dict[str, deque[float]] = defaultdict(deque)
        self._contador_llamadas = 0
        self._lock = Lock()

    def _purgar_inactivos_bloqueado(self, ahora: float) -> None:
        """Elimina entradas de usuarios inactivos mientras se sostiene el lock."""
        limite_ventana = ahora - self.ventana_segundos
        usuarios_inactivos = []
        for usuario_id, historial in self._solicitudes.items():
            while historial and historial[0] <= limite_ventana:
                historial.popleft()
            if not historial:
                usuarios_inactivos.append(usuario_id)
        for usuario_id in usuarios_inactivos:
            del self._solicitudes[usuario_id]

    def verificar(self, usuario_id: str) -> None:
        """Registra una solicitud o lanza HTTP 429 si la cuota del usuario está agotada."""
        ahora = self._reloj()
        with self._lock:
            self._contador_llamadas += 1
            if self._contador_llamadas >= self.intervalo_purga:
                self._contador_llamadas = 0
                self._purgar_inactivos_bloqueado(ahora)

            historial = self._solicitudes[usuario_id]
            limite_ventana = ahora - self.ventana_segundos
            while historial and historial[0] <= limite_ventana:
                historial.popleft()

            if len(historial) >= self.maximo_solicitudes:
                segundos_restantes = historial[0] + self.ventana_segundos - ahora
                raise HTTPException(
                    status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                    detail="Límite de solicitudes al asistente excedido. Por favor, espera antes de enviar otro mensaje.",
                    headers={
                        "Retry-After": str(max(1, int(math.ceil(segundos_restantes))))
                    },
                )

            historial.append(ahora)

    def purgar_inactivos(self, tiempo_limite: Optional[float] = None) -> None:
        """Elimina entradas de usuarios cuyo historial esté completamente expirado."""
        ahora = tiempo_limite if tiempo_limite is not None else self._reloj()
        with self._lock:
            self._purgar_inactivos_bloqueado(ahora)

    def reiniciar(self) -> None:
        """Limpia el almacenamiento local en memoria; destinado principalmente a pruebas."""
        with self._lock:
            self._solicitudes.clear()
            self._contador_llamadas = 0

