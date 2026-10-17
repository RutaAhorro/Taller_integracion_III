"""
Endpoints que reciben el mensaje del usuario y ejecutan el flujo de tres
etapas deterministas (extraccion -> filtrado real -> explicacion).

El endpoint unificado POST /api/v1/chat valida la sesión JWT del usuario
y utiliza el proveedor fallback (Gemini -> Groq) para garantizar resiliencia
y evitar alucinaciones.
"""

from typing import Optional
import anyio
from fastapi import APIRouter, Depends

from app.auth import UsuarioAutenticado, obtener_usuario_actual
from app.explicacion import generar_explicacion_con_tiempo
from app.filtros import filtrar_productos
from app.productos import ProductoRepositorio
from app.proveedores.base import ProveedorIA
from app.schemas import ChatRequest


def crear_router_chat(
    repositorio: ProductoRepositorio,
    proveedor_fallback: ProveedorIA,
    proveedor_groq: Optional[ProveedorIA] = None,
    proveedor_gemini: Optional[ProveedorIA] = None,
) -> APIRouter:
    router = APIRouter(tags=["chat"])

    async def _procesar_chat(proveedor: ProveedorIA, mensaje: str, user_id: Optional[str] = None) -> dict:
        # Etapa 1: Extraer filtros estructurados mediante function calling
        resultado = await proveedor.extraer_filtros(mensaje)

        if resultado.error:
            explicacion = f"Error al procesar la consulta: {resultado.error}"
            respuesta = {
                "respuesta": explicacion,
                "explicacion": explicacion,
                "filtros_aplicados": None,
                "filtros": None,
                "productos_recomendados": [],
                "productos_filtrados": [],
                "error": resultado.error,
            }
            if resultado.error_estado:
                respuesta["estado_error"] = resultado.error_estado
            if resultado.nota:
                respuesta["nota"] = resultado.nota
            if resultado.extra:
                respuesta.update(resultado.extra)
            if user_id:
                respuesta["user_id"] = user_id
            return respuesta

        if resultado.filtros is None:
            explicacion = (
                resultado.texto_libre
                or "No se identificaron criterios de búsqueda específicos en tu mensaje."
            )
            respuesta = {
                "respuesta": explicacion,
                "explicacion": explicacion,
                "filtros_aplicados": None,
                "filtros": None,
                "productos_recomendados": [],
                "productos_filtrados": [],
                "texto_libre": resultado.texto_libre,
                "nota": resultado.nota or "El modelo no uso function calling esta vez",
                "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            }
            if user_id:
                respuesta["user_id"] = user_id
            return respuesta

        # Etapa 2: Obtener productos y filtrar de forma determinista
        productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
        productos_filtrados = filtrar_productos(productos, resultado.filtros)

        # Etapa 3: Generar explicación final con el LLM únicamente con los productos validados de la etapa 2
        explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
            proveedor.generar_explicacion, mensaje, productos_filtrados
        )

        respuesta = {
            "respuesta": explicacion,
            "explicacion": explicacion,
            "filtros_aplicados": resultado.filtros,
            "filtros": resultado.filtros,
            "productos_recomendados": productos_filtrados,
            "productos_filtrados": productos_filtrados,
            "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            "tiempo_explicacion_ms": tiempo_explicacion_ms,
        }
        if resultado.nota:
            respuesta["nota"] = resultado.nota
        if user_id:
            respuesta["user_id"] = user_id
        return respuesta

    @router.post("/api/v1/chat")
    async def chat_unificado(
        req: ChatRequest,
        usuario: UsuarioAutenticado = Depends(obtener_usuario_actual),
    ):
        """
        Endpoint unificado protegido con JWT de sesión (SUP-247).
        Ejecuta las 3 etapas deterministas con proveedor_fallback.
        """
        return await _procesar_chat(proveedor_fallback, req.mensaje, user_id=usuario.user_id)

    if proveedor_groq is not None:
        @router.post("/chat/groq")
        async def chat_groq(req: ChatRequest):
            if not proveedor_groq.api_key:
                return {"error": "Falta GROQ_API_KEY en el archivo .env"}
            return await _procesar_chat(proveedor_groq, req.mensaje)

    if proveedor_gemini is not None:
        @router.post("/chat/gemini")
        async def chat_gemini(req: ChatRequest):
            if not proveedor_gemini.api_key:
                return {"error": "Falta GEMINI_API_KEY en el archivo .env"}
            return await _procesar_chat(proveedor_gemini, req.mensaje)

    return router