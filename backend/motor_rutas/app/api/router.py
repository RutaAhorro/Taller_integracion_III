"""Rutas públicas del motor de rutas; el Gateway enruta /api/v1/listas hacia este servicio."""

from fastapi import APIRouter

from app.listas.router import router as listas

api_router = APIRouter(prefix="/api/v1")
api_router.include_router(listas)
