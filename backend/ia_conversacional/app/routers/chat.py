"""
Endpoints que reciben el mensaje del usuario y ejecutan el flujo de tres
etapas deterministas (extraccion -> filtrado real -> explicacion).

El endpoint unificado POST /api/v1/chat valida la sesión JWT del usuario
y utiliza el proveedor fallback (Gemini -> Groq) para garantizar resiliencia
y evitar alucinaciones.
"""

import logging
from typing import Optional
import anyio
from fastapi import APIRouter, Depends, HTTPException, status

from app.auth import UsuarioAutenticado, obtener_usuario_actual
from app.explicacion import generar_explicacion_con_tiempo
from app.filtros import filtrar_productos
from app.productos import ProductoRepositorio
from app.proveedores.base import ProveedorIA
from app.schemas import ChatRequest

logger = logging.getLogger(__name__)


def crear_router_chat(
    repositorio: ProductoRepositorio,
    proveedor_fallback: ProveedorIA,
    proveedor_groq: Optional[ProveedorIA] = None,
    proveedor_gemini: Optional[ProveedorIA] = None,
) -> APIRouter:
    router = APIRouter(tags=["chat"])

    async def _procesar_chat(proveedor: ProveedorIA, mensaje: str) -> dict:
        # Etapa 1: Extraer filtros estructurados mediante function calling
        resultado = await proveedor.extraer_filtros(mensaje)

        if resultado.error:
            logger.error("Error del proveedor IA en extracción: %s", resultado.error)
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                detail="El asistente no está disponible en este momento. Por favor, intenta en unos minutos.",
            )

        if resultado.filtros is None:
            explicacion = (
                resultado.texto_libre
                or "No se identificaron criterios de búsqueda específicos en tu mensaje."
            )
            return {
                "respuesta": explicacion,
                "filtros": None,
                "filtros_aplicados": None,
                "productos": [],
                "productos_recomendados": [],
                "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            }

        # Etapa 2: Obtener productos y filtrar de forma determinista
        productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
        if not productos:
            return {
                "respuesta": "El catálogo de productos no se encuentra disponible en este momento. Por favor, intenta más tarde.",
                "filtros": resultado.filtros,
                "filtros_aplicados": resultado.filtros,
                "productos": [],
                "productos_recomendados": [],
            }

        productos_filtrados = filtrar_productos(productos, resultado.filtros)

        # Etapa 3: Generar explicación final con el LLM únicamente con los productos validados de la etapa 2
        explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
            proveedor.generar_explicacion, mensaje, productos_filtrados
        )

        return {
            "respuesta": explicacion,
            "filtros": resultado.filtros,
            "filtros_aplicados": resultado.filtros,
            "productos": productos_filtrados,
            "productos_recomendados": productos_filtrados,
            "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            "tiempo_explicacion_ms": tiempo_explicacion_ms,
        }

    @router.post("/api/v1/chat")
    async def chat_unificado(
        req: ChatRequest,
        usuario: UsuarioAutenticado = Depends(obtener_usuario_actual),
    ):
        """
        Endpoint unificado protegido con JWT de sesión (SUP-247).
        Ejecuta las 3 etapas deterministas con proveedor_fallback.
        """
        return await _procesar_chat(proveedor_fallback, req.mensaje)

    return router