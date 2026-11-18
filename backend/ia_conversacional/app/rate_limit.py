"""Limitación de solicitudes del chat por usuario autenticado mediante ventana deslizante."""

import math
import time
from collections import defaultdict, deque
from threading import Lock
from typing import Callable, Optional

from fastapi import HTTPException, status


class LimitadorSolicitudes:
    """Contador de solicitudes en memoria local mediante ventana deslizante, independiente por réplica."""

    def __init__(
        self,
        maximo_solicitudes: int = 10,
        ventana_segundos: float = 60.0,
        reloj: Optional[Callable[[], float]] = None,
    ):
        """Inicializa el limitador por ventana deslizante (log de marcas de tiempo)."""
        if maximo_solicitudes <= 0:
            raise ValueError("maximo_solicitudes debe ser mayor a 0")
        if ventana_segundos <= 0 or not math.isfinite(ventana_segundos):
            raise ValueError("ventana_segundos debe ser mayor a 0 y finito")

        self.maximo_solicitudes = maximo_solicitudes
        self.ventana_segundos = ventana_segundos
        self._reloj = reloj or time.monotonic
        self._solicitudes: dict[str, deque[float]] = defaultdict(deque)
        self._lock = Lock()

    def verificar(self, usuario_id: str) -> None:
        """Registra una solicitud o lanza HTTP 429 si la cuota del usuario está agotada."""
        ahora = self._reloj()
        with self._lock:
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
        limite_ventana = ahora - self.ventana_segundos
        with self._lock:
            usuarios_inactivos = []
            for usuario_id, historial in self._solicitudes.items():
                while historial and historial[0] <= limite_ventana:
                    historial.popleft()
                if not historial:
                    usuarios_inactivos.append(usuario_id)
            for usuario_id in usuarios_inactivos:
                del self._solicitudes[usuario_id]

    def reiniciar(self) -> None:
        """Limpia el almacenamiento local en memoria; destinado principalmente a pruebas."""
        with self._lock:
            self._solicitudes.clear()


# Alias retrocompatible por si algún import externo lo referencia
RateLimiter = LimitadorSolicitudes

