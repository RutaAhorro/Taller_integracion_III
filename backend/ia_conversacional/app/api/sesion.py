"""
Sesión del usuario en el chat: valida el JWT emitido por Identidad y limita
la frecuencia de mensajes por usuario.

La autorización pertenece a cada servicio, así que IA verifica por sí misma
la firma HS256 con el `JWT_SECRET` compartido; Identidad solo emite la cookie.
"""

import math
import time
from collections import defaultdict, deque
from collections.abc import Callable
from threading import Lock

import jwt
from fastapi import Cookie, Header, HTTPException, status
from pydantic import BaseModel

from app import config

# ===== Constantes =====

NOMBRE_COOKIE_SESION = "jwt"
ALGORITMO_JWT = "HS256"
CLAIMS_OBLIGATORIOS = ["exp", "user_id"]
ESQUEMA_BEARER = "bearer"
ENCABEZADO_DESAFIO = {"WWW-Authenticate": "Bearer"}

# Cada cuántas verificaciones se purgan los usuarios inactivos del limitador.
INTERVALO_PURGA_PREDETERMINADO = 50
# Retry-After nunca baja de un segundo, aunque la ventana libere antes.
SEGUNDOS_MINIMOS_REINTENTO = 1


# ===== Autenticación JWT =====


class UsuarioAutenticado(BaseModel):
    """Claims de la sesión que el chat usa: hoy solo el identificador para limitar frecuencia."""

    user_id: str


def _error_no_autorizado(detalle: str) -> HTTPException:
    """Construye un 401 con el desafío Bearer que exige el estándar HTTP."""
    return HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail=detalle,
        headers=ENCABEZADO_DESAFIO,
    )


def _extraer_bearer(encabezado_autorizacion: str | None) -> str | None:
    """Devuelve el token de un encabezado `Authorization: Bearer <token>` o None si el formato no calza."""
    if not encabezado_autorizacion:
        return None
    partes = encabezado_autorizacion.strip().split()
    if len(partes) == 2 and partes[0].lower() == ESQUEMA_BEARER:
        return partes[1]
    return None


async def obtener_usuario_actual(
    cookie_jwt: str | None = Cookie(None, alias=NOMBRE_COOKIE_SESION),
    encabezado_autorizacion: str | None = Header(None, alias="Authorization"),
) -> UsuarioAutenticado:
    """
    Dependencia de FastAPI que valida la sesión.

    Prefiere la cookie HttpOnly `jwt` del navegador y acepta `Authorization:
    Bearer` para clientes de API. Exige firma HS256 y los claims `exp` y
    `user_id`; `validar_configuracion` garantiza el secreto al arrancar.
    """
    token = cookie_jwt or _extraer_bearer(encabezado_autorizacion)
    if not token:
        if encabezado_autorizacion:
            raise _error_no_autorizado("Header Authorization debe tener formato 'Bearer <token>'")
        raise _error_no_autorizado("Autenticación requerida: falta cookie 'jwt' o header 'Authorization'")

    try:
        claims = jwt.decode(
            token,
            config.JWT_SECRET,
            algorithms=[ALGORITMO_JWT],
            options={"require": CLAIMS_OBLIGATORIOS},
        )
    except jwt.ExpiredSignatureError:
        raise _error_no_autorizado("Token expirado") from None
    except jwt.MissingRequiredClaimError as error_claim:
        raise _error_no_autorizado(f"Token inválido: falta claim requerido ({error_claim})") from None
    except jwt.InvalidTokenError:
        raise _error_no_autorizado("Token inválido o firma incorrecta") from None

    return UsuarioAutenticado(user_id=str(claims["user_id"]))


# ===== Limitación de frecuencia (SUP-336) =====


class LimitadorSolicitudes:
    """
    Ventana deslizante en memoria por usuario.

    Es local a cada réplica, sin Redis: con N réplicas el límite efectivo es
    N veces la cuota. Basta para proteger cada instancia de una ráfaga.
    """

    def __init__(
        self,
        maximo_solicitudes: int,
        ventana_segundos: float,
        reloj: Callable[[], float] | None = None,
        intervalo_purga: int = INTERVALO_PURGA_PREDETERMINADO,
    ):
        """Valida los parámetros y prepara el historial; `reloj` permite controlar el tiempo en pruebas."""
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
        self._candado = Lock()

    def _descartar_expiradas(self, historial: deque[float], ahora: float) -> None:
        """Quita del historial las marcas que ya salieron de la ventana."""
        limite_ventana = ahora - self.ventana_segundos
        while historial and historial[0] <= limite_ventana:
            historial.popleft()

    def _purgar_inactivos(self, ahora: float) -> None:
        """Elimina usuarios sin marcas vigentes para que la memoria no crezca sin límite. Requiere el candado."""
        usuarios_inactivos = []
        for usuario_id, historial in self._solicitudes.items():
            self._descartar_expiradas(historial, ahora)
            if not historial:
                usuarios_inactivos.append(usuario_id)
        for usuario_id in usuarios_inactivos:
            del self._solicitudes[usuario_id]

    def verificar(self, usuario_id: str) -> None:
        """Registra una solicitud o lanza HTTP 429 con Retry-After si el usuario agotó su cuota."""
        ahora = self._reloj()
        with self._candado:
            self._contador_llamadas += 1
            if self._contador_llamadas >= self.intervalo_purga:
                self._contador_llamadas = 0
                self._purgar_inactivos(ahora)

            historial = self._solicitudes[usuario_id]
            self._descartar_expiradas(historial, ahora)

            if len(historial) >= self.maximo_solicitudes:
                segundos_restantes = historial[0] + self.ventana_segundos - ahora
                raise HTTPException(
                    status_code=status.HTTP_429_TOO_MANY_REQUESTS,
                    detail="Límite de solicitudes al asistente excedido. Por favor, espera antes de enviar otro mensaje.",
                    headers={"Retry-After": str(max(SEGUNDOS_MINIMOS_REINTENTO, math.ceil(segundos_restantes)))},
                )

            historial.append(ahora)
