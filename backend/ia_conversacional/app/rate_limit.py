"""Limitación de solicitudes del chat por usuario autenticado."""

import math
import time
from collections import defaultdict, deque
from threading import Lock
from typing import Callable, Optional

from fastapi import HTTPException, status


class RateLimiter:
    """Contador de solicitudes en memoria local, independiente por réplica."""

    def __init__(
        self,
        max_requests: int = 10,
        window_seconds: float = 60.0,
        clock: Optional[Callable[[], float]] = None,
    ):
        """Inicializa una ventana fija deslizante por usuario autenticado."""
        if max_requests <= 0:
            raise ValueError("max_requests debe ser mayor a 0")
        if window_seconds <= 0 or not math.isfinite(window_seconds):
            raise ValueError("window_seconds debe ser mayor a 0 y finito")

        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self._clock = clock or time.monotonic
        self._solicitudes: dict[str, deque[float]] = defaultdict(deque)
        self._lock = Lock()

    def verificar(self, user_id: str) -> None:
        """Registra una solicitud o lanza HTTP 429 si la cuota está agotada."""
        ahora = self._clock()
        with self._lock:
            historial = self._solicitudes[user_id]
            limite_ventana = ahora - self.window_seconds
            while historial and historial[0] <= limite_ventana:
                historial.popleft()

            if len(historial) >= self.max_requests:
                segundos_restantes = historial[0] + self.window_seconds - ahora
                raise HTTPException(
                    status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                    detail="Límite de solicitudes al asistente excedido. Por favor, espera antes de enviar otro mensaje.",
                    headers={
                        "Retry-After": str(max(1, int(math.ceil(segundos_restantes))))
                    },
                )

            historial.append(ahora)

    def reset(self) -> None:
        """Limpia el almacenamiento local; está destinado principalmente a pruebas."""
        with self._lock:
            self._solicitudes.clear()
