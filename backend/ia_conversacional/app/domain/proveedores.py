"""
Contrato común de los proveedores de IA.

El chat solo conoce esta interfaz, así que sumar un proveedor es escribir
una clase que la implemente y conectarla en `app/main.py`. Los
clientes concretos viven en `app/repositories` y el respaldo en `app/services`.
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass

# Los tiempos de cada etapa se informan en milisegundos.
MILISEGUNDOS_POR_SEGUNDO = 1000


@dataclass
class ResultadoExtraccion:
    """
    Resultado de la etapa 1.

    Trae `filtros` si el modelo llamó la función, `texto_libre` si respondió
    sin llamarla (el chat lo descarta) o `error` si el proveedor falló.
    """

    filtros: dict | None = None
    texto_libre: str | None = None
    error: str | None = None
    tiempo_extraccion_ms: int = 0


class ProveedorIA(ABC):
    """Interfaz que cumple cada proveedor de IA conectado al chat."""

    nombre: str

    @abstractmethod
    async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
        """Etapa 1: envía el mensaje al LLM y extrae filtros estructurados."""

    @abstractmethod
    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        """Etapa 3: redacta la explicación a partir de productos ya filtrados."""
