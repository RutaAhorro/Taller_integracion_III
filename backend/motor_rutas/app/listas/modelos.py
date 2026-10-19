"""Contratos JSON de listas de compra (SUP-240), en snake_case como Catálogo."""

from __future__ import annotations

import unicodedata
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, field_validator, model_validator

LARGO_MAXIMO_NOMBRE = 150  # rutas.listas_compras.nombre VARCHAR(150)
Estado = Literal["activa", "completada", "archivada"]


def normalizar_nombre(valor: str) -> str:
    """Quita caracteres de control y espacios sobrantes sin cortar tildes ni ñ."""
    limpio = "".join(" " if c.isspace() else c for c in valor if c.isspace() or unicodedata.category(c) not in ("Cc", "Cf"))
    limpio = " ".join(limpio.split())
    if not 1 <= len(limpio) <= LARGO_MAXIMO_NOMBRE:
        raise ValueError(f"debe tener entre 1 y {LARGO_MAXIMO_NOMBRE} caracteres")
    return limpio


class ListaCrear(BaseModel):
    model_config = ConfigDict(extra="forbid")
    nombre: str

    @field_validator("nombre")
    @classmethod
    def _nombre(cls, valor: str) -> str:
        return normalizar_nombre(valor)


class ListaActualizar(BaseModel):
    model_config = ConfigDict(extra="forbid")
    nombre: str | None = None
    estado: Estado | None = None

    @field_validator("nombre")
    @classmethod
    def _nombre(cls, valor: str | None) -> str | None:
        return None if valor is None else normalizar_nombre(valor)

    @model_validator(mode="after")
    def _algun_cambio(self) -> ListaActualizar:
        if self.nombre is None and self.estado is None:
            raise ValueError("indicar nombre o estado")
        return self


class Articulo(BaseModel):
    id: int
    producto_normalizado_id: str
    cantidad: int
    esta_comprado: bool


class Lista(BaseModel):
    id: str
    nombre: str
    estado: str
    total_articulos: int
    creada_el: datetime
    actualizada_el: datetime


class ListaDetalle(Lista):
    articulos: list[Articulo]


class RespuestaListas(BaseModel):
    listas: list[Lista]
