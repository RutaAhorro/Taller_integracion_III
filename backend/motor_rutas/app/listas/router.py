"""CRUD de listas de compra del usuario autenticado (SUP-240)."""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Depends, Request, Response

from app.auth import Sesion, sesion_actual
from app.errores import ErrorAPI
from app.listas.modelos import Lista, ListaActualizar, ListaCrear, ListaDetalle, RespuestaListas
from app.listas.repositorio import RepositorioListas

# Tope defensivo por usuario: el servicio no tiene límite de peticiones propio.
MAXIMO_LISTAS = 100

router = APIRouter(prefix="/listas", tags=["listas"])


def repositorio(request: Request) -> RepositorioListas:
    return request.app.state.repositorio


def _no_existe() -> ErrorAPI:
    # Un id mal formado, inexistente o de otro usuario responde igual: no revela qué listas existen.
    return ErrorAPI(404, "La lista no existe.")


def _id_o_404(lista_id: str) -> str:
    try:
        return str(uuid.UUID(lista_id))
    except ValueError:
        raise _no_existe() from None


@router.get("", response_model=RespuestaListas)
@router.get("/", response_model=RespuestaListas, include_in_schema=False)
async def listar(sesion: Sesion = Depends(sesion_actual), repo: RepositorioListas = Depends(repositorio)) -> RespuestaListas:
    return RespuestaListas(listas=await repo.listar(sesion.usuario_id, MAXIMO_LISTAS))


@router.post("", response_model=Lista, status_code=201)
@router.post("/", response_model=Lista, status_code=201, include_in_schema=False)
async def crear(datos: ListaCrear, sesion: Sesion = Depends(sesion_actual), repo: RepositorioListas = Depends(repositorio)) -> Lista:
    if await repo.contar(sesion.usuario_id) >= MAXIMO_LISTAS:
        raise ErrorAPI(409, f"Alcanzaste el máximo de {MAXIMO_LISTAS} listas.")
    return await repo.crear(sesion.usuario_id, datos.nombre)


@router.get("/{lista_id}", response_model=ListaDetalle)
async def obtener(lista_id: str, sesion: Sesion = Depends(sesion_actual), repo: RepositorioListas = Depends(repositorio)) -> ListaDetalle:
    lista = await repo.obtener(sesion.usuario_id, _id_o_404(lista_id))
    if lista is None:
        raise _no_existe()
    return lista


@router.put("/{lista_id}", response_model=Lista)
async def actualizar(lista_id: str, datos: ListaActualizar, sesion: Sesion = Depends(sesion_actual), repo: RepositorioListas = Depends(repositorio)) -> Lista:
    lista = await repo.actualizar(sesion.usuario_id, _id_o_404(lista_id), datos.nombre, datos.estado)
    if lista is None:
        raise _no_existe()
    return lista


@router.delete("/{lista_id}", status_code=204)
async def eliminar(lista_id: str, sesion: Sesion = Depends(sesion_actual), repo: RepositorioListas = Depends(repositorio)) -> Response:
    if not await repo.eliminar(sesion.usuario_id, _id_o_404(lista_id)):
        raise _no_existe()
    return Response(status_code=204)
