"""
Pruebas unitarias para limpieza de explicaciones y logs diferenciados por etapa.

Verifica:
1. `limpiar_explicacion` elimina Markdown crudo y recorta respuestas truncadas en la
   última oración completa (ej. `**Yogh`), sin cortar precios chilenos como `$1.290`.
2. `generar_explicacion_con_tiempo` aplica la limpieza y registra "Falló la explicación"
   sin exponer claves ni cabeceras.
3. `procesar_mensaje` registra "Falló la extracción" cuando falla la etapa 1.
4. El límite por defecto de tokens para explicación en Groq es suficiente para evitar cortes.
"""

import logging
from unittest.mock import MagicMock

import anyio
import pytest

from app import config
from app.domain.filtros import formatear_precio_clp
from app.domain.prompts import construir_prompt_explicacion
from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.services.chat import (
    TEXTO_EXPLICACION_RESPALDO,
    AsistenteNoDisponible,
    construir_explicacion_respaldo,
    generar_explicacion_con_tiempo,
    limpiar_explicacion,
    procesar_mensaje,
)

# ===== Limpieza de Markdown y recorte de oraciones =====


def test_limpiar_explicacion_recorta_markdown_truncado_en_ultima_oracion_completa():
    """Una respuesta cortada con markdown abierto como `**Yogh` se recorta al último punto válido."""
    texto_truncado = "Encontré leche a $1.290. Además **Yogh"

    resultado = limpiar_explicacion(texto_truncado)

    assert resultado == "Encontré leche a $1.290."
    assert "**" not in resultado
    assert "Yogh" not in resultado


def test_limpiar_explicacion_conserva_lista_numerada_completa_sin_punto_final():
    """Una explicación completa sin punto final ni delimitadores abiertos no se recorta."""
    texto_lista = "Opciones:\n1. Leche a $1.190\n2. Pan a $1.290"

    assert limpiar_explicacion(texto_lista) == texto_lista


def test_limpiar_explicacion_ignora_listas_numeradas_y_abreviaturas_al_recortar_truncado():
    """Ante delimitadores sin cerrar (backtick o paréntesis), el corte ignora `1.`, `2.`, `Av.` y `Sr.`."""
    texto_parentesis_abierto = "Opciones:\n1. Leche a $1.190.\n2. Pan en Av. Providencia (sucursal Sr. Juan"
    assert limpiar_explicacion(texto_parentesis_abierto) == "Opciones:\n1. Leche a $1.190."

    texto_backtick_abierto = "Encontré leche a $1.290. Disponible en Av. Italia `cod"
    assert limpiar_explicacion(texto_backtick_abierto) == "Encontré leche a $1.290."


def test_limpiar_explicacion_quita_markdown_crudo_de_respuesta_completa():
    """Elimina encabezados, negritas, cursivas y código en línea preservando precios y contenido."""
    texto_con_markdown = "### Opciones\n* **Leche Entera** a `$1.190`.\n* *Yoghurt Vainilla* (pack 2*1) a $250."

    resultado = limpiar_explicacion(texto_con_markdown)

    assert resultado == "Opciones\n- Leche Entera a $1.190.\n- Yoghurt Vainilla (pack 2*1) a $250."


def test_limpiar_explicacion_no_corta_precio_chileno_sin_punto_final():
    """El punto separador de miles en `$1.290` no se confunde con fin de oración."""
    texto_sin_punto_final = "Leche Entera a $1.290 en Lider"

    assert limpiar_explicacion(texto_sin_punto_final) == "Leche Entera a $1.290 en Lider"


def test_groq_max_tokens_explicacion_por_defecto_evita_recortes_prematuros():
    """El límite por defecto de Groq para explicación queda fijado en 600 tokens."""
    assert config.GROQ_MAX_TOKENS_EXPLICACION == 600


# ===== Logs diferenciados entre extracción y explicación =====


@pytest.mark.anyio
async def test_generar_explicacion_con_tiempo_limpia_y_respalda_si_queda_vacia():
    """Si la explicación solo contenía marcas Markdown, se usa el texto de respaldo."""

    async def generador_solo_markdown(_mensaje: str, _productos: list) -> str:
        return "  **  "

    explicacion, _tiempo_ms = await generar_explicacion_con_tiempo(
        generador_solo_markdown, "leche", [{"nombre": "Leche"}]
    )

    assert explicacion == TEXTO_EXPLICACION_RESPALDO


@pytest.mark.anyio
async def test_generar_explicacion_con_tiempo_loguea_fallo_explicacion_sin_secretos(caplog):
    """Ante error en etapa 3, el log indica 'Falló la explicación' y oculta cabeceras/claves."""

    async def generador_falla(_mensaje: str, _productos: list) -> str:
        raise RuntimeError("Fallo con Authorization: Bearer secreto-super-privado y ?key=clave-gemini")

    with caplog.at_level(logging.WARNING):
        explicacion, _tiempo_ms = await generar_explicacion_con_tiempo(generador_falla, "leche", [{"nombre": "Leche"}])

    assert explicacion == TEXTO_EXPLICACION_RESPALDO
    assert "Falló la explicación" in caplog.text
    assert "Falló la extracción" not in caplog.text
    assert "secreto-super-privado" not in caplog.text
    assert "clave-gemini" not in caplog.text


@pytest.mark.anyio
async def test_generar_explicacion_con_tiempo_loguea_fallo_explicacion_por_plazo(caplog):
    """Si la etapa 3 vence su plazo, el log indica 'Falló la explicación'."""

    async def generador_lento(_mensaje: str, _productos: list) -> str:
        await anyio.sleep(0.5)
        return "Tarde."

    with caplog.at_level(logging.WARNING):
        explicacion, _tiempo_ms = await generar_explicacion_con_tiempo(
            generador_lento, "leche", [{"nombre": "Leche"}], tiempo_limite=0.02
        )

    assert explicacion == TEXTO_EXPLICACION_RESPALDO
    assert "Falló la explicación" in caplog.text


@pytest.mark.anyio
async def test_procesar_mensaje_loguea_fallo_extraccion_cuando_etapa_1_falla(caplog):
    """Cuando la etapa 1 retorna error, el log indica 'Falló la extracción' y no 'Falló la explicación'."""

    class ProveedorConErrorExtraccion(ProveedorIA):
        nombre = "fallo"

        async def extraer_filtros(self, mensaje: str) -> ResultadoExtraccion:
            return ResultadoExtraccion(error="Timeout en proveedor primario")

        async def generar_explicacion(self, mensaje: str, productos: list) -> str:
            return "No debe llamarse."

    with caplog.at_level(logging.ERROR):
        with pytest.raises(AsistenteNoDisponible):
            await procesar_mensaje(ProveedorConErrorExtraccion(), MagicMock(), "busco leche")

    assert "Falló la extracción" in caplog.text
    assert "Falló la explicación" not in caplog.text


# ===== Formato de precios en pesos chilenos =====


def test_formatear_precio_clp_y_prompt_explicacion():
    """Los precios se envían al modelo ya formateados en pesos chilenos con punto de miles ($10.600)."""
    assert formatear_precio_clp(10600.0) == "$10.600"
    assert formatear_precio_clp(2000) == "$2.000"
    assert formatear_precio_clp(890.0) == "$890"

    prompt = construir_prompt_explicacion(
        "Busco harina de almendras",
        [{"nombre": "Harina de Almendras", "precio": 10600.0, "categoria": "despensa", "distancia_km": 0.0}],
    )
    assert "$10.600" in prompt
    assert "10600.0" not in prompt


def test_limpiar_explicacion_corrige_precios_sin_formato_o_con_espacio():
    """Si el modelo escribe '$10600.0', '$890.0' o '$2 000', la limpieza los normaliza sin tocar '$2.000'."""
    texto = "Encontré Harina de Almendras a $10600.0, Zanahoria a $890.0 y Leche Entera a $2 000."
    assert (
        limpiar_explicacion(texto)
        == "Encontré Harina de Almendras a $10.600, Zanahoria a $890 y Leche Entera a $2.000."
    )
    assert limpiar_explicacion("Leche a $2.000 y Queso a $10.000.") == "Leche a $2.000 y Queso a $10.000."


def test_construir_explicacion_respaldo_formatea_precio_clp():
    """La frase fija de respaldo usa el término buscado y el precio mínimo formateado en CLP."""
    productos = [{"nombre": "Harina de Almendras", "precio": 10600.0}]
    assert (
        construir_explicacion_respaldo(productos, "harina de almendras")
        == "Estas son las opciones que encontré para «harina de almendras», desde $10.600."
    )


def test_prompt_explicacion_omite_distancia_y_categoria_e_incluye_datos_verificables():
    """El prompt solo entrega nombre, marca, precio CLP y tienda; jamás pasa distancia_km ni slugs de categoría."""
    productos = [
        {
            "nombre": "Aceite de Oliva Extra Virgen 500 ml",
            "marca": "Chef",
            "precio": 5490.0,
            "supermercado": "Jumbo",
            "categoria": "despensa",
            "distancia_km": 0.0,
        },
        {
            "nombre": "Harina de Almendras 500 g",
            "marca": "",
            "precio": 10600.0,
            "supermercado": "",
            "categoria": "abarrotes-gourmet",
            "distancia_km": 2.5,
        },
    ]

    prompt = construir_prompt_explicacion("Busco harina almendras", productos)

    assert "Aceite de Oliva Extra Virgen 500 ml" in prompt
    assert '"marca": "Chef"' in prompt
    assert '"tienda": "Jumbo"' in prompt
    assert "$5.490" in prompt
    assert "$10.600" in prompt
    assert "distancia_km" not in prompt
    assert "0.0" not in prompt
    assert "2.5" not in prompt
    assert "despensa" not in prompt
    assert "abarrotes-gourmet" not in prompt
    assert "No encontré coincidencias exactas" in prompt
    assert "sin gluten" in prompt
    assert "sustituto" in prompt
    assert "No repitas la lista completa de tarjetas" in prompt
