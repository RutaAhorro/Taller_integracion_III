"""
Endpoints que reciben el mensaje del usuario y ejecutan el flujo de tres
etapas (extraccion -> filtrado real -> explicacion)la logica es
identica para Groq y Gemini porque ambos cumplen la interfaz proveedorIA
(ver app/proveedores/base.py), asi que esta ahi solo una vez en
_procesar_chat()
"""

import anyio
from fastapi import APIRouter

from app.explicacion import generar_explicacion_con_tiempo
from app.filtros import filtrar_productos
from app.productos import ProductoRepositorio
from app.proveedores.base import ProveedorIA
from app.schemas import ChatRequest


def crear_router_chat(
    repositorio: ProductoRepositorio,
    proveedor_groq: ProveedorIA,
    proveedor_gemini: ProveedorIA,
) -> APIRouter:
    router = APIRouter(prefix="/chat", tags=["chat"])

    async def _procesar_chat(proveedor: ProveedorIA, mensaje: str) -> dict:
        resultado = await proveedor.extraer_filtros(mensaje)

        if resultado.error:
            respuesta = {"error": resultado.error}
            if resultado.error_estado:
                respuesta["estado_error"] = resultado.error_estado
            if resultado.nota:
                respuesta["nota"] = resultado.nota
            if resultado.extra:
                respuesta.update(resultado.extra)
            return respuesta

        if resultado.filtros is None:
            return {
                "filtros": None,
                "texto_libre": resultado.texto_libre,
                "nota": resultado.nota or "El modelo no uso function calling esta vez",
                "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            }

        todos_los_productos = await anyio.to_thread.run_sync(repositorio.obtener_todos)
        productos_filtrados = filtrar_productos(todos_los_productos, resultado.filtros)
        print(f"Productos filtrados: {len(productos_filtrados)}")

        explicacion, tiempo_explicacion_ms = await generar_explicacion_con_tiempo(
            proveedor.generar_explicacion, mensaje, productos_filtrados
        )

        respuesta = {
            "filtros": resultado.filtros,
            "productos_filtrados": productos_filtrados,
            "explicacion": explicacion,
            "tiempo_extraccion_ms": resultado.tiempo_extraccion_ms,
            "tiempo_explicacion_ms": tiempo_explicacion_ms,
        }
        if resultado.nota:  # ej. caso de rescate de schema de Groq
            respuesta["nota"] = resultado.nota
        return respuesta

    @router.post("/groq")
    async def chat_groq(req: ChatRequest):
        if not proveedor_groq.api_key:
            return {"error": "Falta GROQ_API_KEY en el archivo .env"}
        return await _procesar_chat(proveedor_groq, req.mensaje)

    @router.post("/gemini")
    async def chat_gemini(req: ChatRequest):
        if not proveedor_gemini.api_key:
            return {"error": "Falta GEMINI_API_KEY en el archivo .env"}
        return await _procesar_chat(proveedor_gemini, req.mensaje)

    return router